package service

import (
	"bytes"
	"encoding/json"
	"sort"
	"strings"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const propertyMigrationAction = "migrate_property"
const propertyMigrationLimit = 200

type propertyMigrationFrozen struct {
	SidecarID string `json:"sidecar_id"`
	Revision uint64 `json:"revision"`
	ValuesDigest string `json:"values_digest"`
	HeadVersion string `json:"head_version"`
	SourceType string `json:"source_type"`
	PreviewError string `json:"preview_error"`
}

func IsPropertyMigrationPlan(plan OrganizationPlan) bool {
	for _, action := range decodePlanActions(plan.Actions) {
		if action.Kind == propertyMigrationAction { return true }
	}
	return false
}

func propertyMigrationRequest(plan OrganizationPlan) (contract.PropertyMigrationPreview, error) {
	var q contract.PropertyMigrationPreview
	actions := decodePlanActions(plan.Actions)
	if len(actions) != 1 || actions[0].Kind != propertyMigrationAction || json.Unmarshal([]byte(actions[0].Value), &q) != nil || !validPropertyMigration(q) {
		return q, code("dependency_unavailable")
	}
	return q, nil
}

func validPropertyMigration(q contract.PropertyMigrationPreview) bool {
	if q.ExpectedDefinitionRevision == 0 || q.ExpectedDefinitionRevision >= 9007199254740991 || !propertyKey.MatchString(q.SourceKey) || !validPropertyDefinition(contract.PropertyDefinition{Key: q.TargetKey, Type: q.TargetType, DisplayName: "Migration"}) { return false }
	switch q.ConversionPolicy { case "preserve", "parse_text", "scalar_text", "singleton_list": return true }
	return false
}

// Conversion is explicit, exact and local: no trimming, float round-trips,
// date guessing, list joining, or implicit reference acquisition.
func convertPropertyMigration(raw json.RawMessage, source string, q contract.PropertyMigrationPreview) (json.RawMessage, error) {
	if validatePropertyValue(source, raw) != nil { return nil, code("incompatible_value") }
	var out json.RawMessage
	switch q.ConversionPolicy {
	case "preserve":
		out = raw
	case "parse_text":
		if source != "text" { return nil, code("incompatible_value") }
		var text string
		if json.Unmarshal(raw, &text) != nil { return nil, code("incompatible_value") }
		switch q.TargetType {
		case "number", "checkbox":
			if strings.TrimSpace(text) != text { return nil, code("incompatible_value") }
			out = json.RawMessage(text)
		case "date", "reference": out = raw
		default: return nil, code("incompatible_value")
		}
	case "scalar_text":
		if q.TargetType != "text" { return nil, code("incompatible_value") }
		switch source {
		case "number", "checkbox": out, _ = json.Marshal(string(bytes.TrimSpace(raw)))
		case "text", "date", "reference": out = raw
		default: return nil, code("incompatible_value")
		}
	case "singleton_list":
		if source != "text" || q.TargetType != "list" { return nil, code("incompatible_value") }
		out = append(append(json.RawMessage{'['}, raw...), ']')
	}
	if validatePropertyValue(q.TargetType, out) != nil { return nil, code("incompatible_value") }
	return out, nil
}

func encodeMigrationValues(values map[string]json.RawMessage) (string, error) {
	keys := make([]string, 0, len(values))
	for key := range values { keys = append(keys, key) }
	sort.Strings(keys)
	var out bytes.Buffer
	out.WriteByte('{')
	for i, key := range keys {
		if i > 0 { out.WriteByte(',') }
		name, _ := json.Marshal(key)
		out.Write(name)
		out.WriteByte(':')
		out.Write(values[key])
	}
	out.WriteByte('}')
	if out.Len() > sidecarBudget { return "", code("query_budget_exceeded") }
	return out.String(), nil
}

func (s *Service) propertyMigrationCatalogOn(tx *gorm.DB, p Principal, space string, q contract.PropertyMigrationPreview) (contract.PropertyDefinitionView, string, error) {
	var head WorkspacePropertyCatalog
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id = ?", space).First(&head).Error; err != nil { return contract.PropertyDefinitionView{}, "", resourceReadError(err) }
	catalog, err := s.propertyDefinitionsOn(tx, p, space)
	if err != nil { return catalog, "", err }
	if catalog.Revision != q.ExpectedDefinitionRevision { return catalog, "", code("revision_conflict") }
	source := ""
	for _, d := range catalog.Definitions {
		if d.Key == q.SourceKey { source = d.Type }
		if q.SourceKey != q.TargetKey && d.Key == q.TargetKey { return catalog, "", code("revision_conflict") }
	}
	if source == "" { return catalog, "", code("not_found") }
	if q.SourceKey == q.TargetKey && source == q.TargetType { return catalog, "", code("invalid_request") }
	return catalog, source, nil
}

func (s *Service) propertyMigrationRowsOn(tx *gorm.DB, p Principal, space string) ([]WorkspacePropertySidecar, error) {
	// Legacy projections and frontmatter have no lossless migration contract.
	// Never silently claim a space-wide dictionary cutover over those values.
	var projections []WorkspaceFilePropertyProjection
	if err := tx.Where("space_id = ?", space).Limit(1).Find(&projections).Error; err != nil { return nil, code("database_unavailable") }
	if len(projections) != 0 { return nil, code("unsupported_action") }
	var rows []WorkspacePropertySidecar
	if err := tx.Where("space_id = ?", space).Order("id ASC").Limit(propertyMigrationLimit+1).Find(&rows).Error; err != nil { return nil, code("database_unavailable") }
	if len(rows) > propertyMigrationLimit { return nil, code("query_budget_exceeded") }
	for _, row := range rows {
		if row.Owner != p.UserID || row.TenantID != p.TenantID { return nil, code("dependency_unavailable") }
	}
	return rows, nil
}

func propertyMigrationImpact(q contract.PropertyMigrationPreview, items []OrganizationPlanItem) string {
	type frozenItem struct { FileID, VersionID, Path, Detail, Before, After string }
	frozen := make([]frozenItem, 0, len(items))
	for _, item := range items { frozen = append(frozen, frozenItem{item.FileID, item.VersionID, item.Path, item.ActionDetail, item.BeforeValue, item.AfterValue}) }
	raw, _ := json.Marshal(struct { Request contract.PropertyMigrationPreview; Items []frozenItem }{q, frozen})
	return Hash(string(raw))
}
