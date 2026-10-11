package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

var propertyKey = regexp.MustCompile(`^[a-z][a-z0-9_]{0,63}$`)

func validPropertyDefinition(row contract.PropertyDefinition) bool {
	if !propertyKey.MatchString(row.Key) {
		return false
	}
	switch row.Type {
	case "text", "number", "date", "checkbox", "list", "reference":
	default:
		return false
	}
	for _, field := range []struct {
		value string
		max   int
	}{{row.DisplayName, 120}, {row.Group, 80}} {
		if !utf8.ValidString(field.value) || utf8.RuneCountInString(field.value) > field.max {
			return false
		}
		for _, r := range field.value {
			if unicode.IsControl(r) {
				return false
			}
		}
	}
	return strings.TrimSpace(row.DisplayName) != ""
}

func (s *Service) propertySpaceOn(tx *gorm.DB, p Principal, id string, write bool) error {
	if !collectionHumanPrincipal(p) {
		return code("forbidden")
	}
	if err := savedQueryActorOn(tx, p); err != nil {
		return err
	}
	if err := s.collectionReadSpaceOn(tx, p, id); err != nil {
		return err
	}
	if err := s.allowOn(tx, p, id, "", "read"); err != nil {
		return err
	}
	if write {
		return s.allowOn(tx, p, id, "", "manage")
	}
	return nil
}

func (s *Service) propertyDefinitionsOn(tx *gorm.DB, p Principal, id string) (contract.PropertyDefinitionView, error) {
	out := contract.PropertyDefinitionView{SpaceID: id, Definitions: []contract.PropertyDefinition{}}
	var head WorkspacePropertyCatalog
	err := tx.Where("space_id = ?", id).First(&head).Error
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return out, code("database_unavailable")
	}
	if err == nil {
		if head.Owner != p.UserID || head.TenantID != p.TenantID {
			return out, code("not_found")
		}
		if head.Revision == 0 || head.Revision > 9007199254740991 {
			return out, code("dependency_unavailable")
		}
		out.Revision = head.Revision
	}
	var rows []WorkspacePropertyDefinition
	if err = tx.Where("space_id = ?", id).Order("key ASC").Limit(201).Find(&rows).Error; err != nil {
		return out, code("database_unavailable")
	}
	if len(rows) > 200 {
		return out, code("query_budget_exceeded")
	}
	if out.Revision == 0 && len(rows) > 0 {
		return out, code("dependency_unavailable")
	}
	seen := map[string]bool{}
	for _, row := range rows {
		if seen[row.Key] {
			return out, code("dependency_unavailable")
		}
		seen[row.Key] = true
		item := contract.PropertyDefinition{Key: row.Key, DisplayName: row.DisplayName, Group: row.Group, Type: row.Type}
		if row.Owner != p.UserID || row.TenantID != p.TenantID || row.Revision == 0 || row.Revision > out.Revision || !validPropertyDefinition(item) {
			return out, code("dependency_unavailable")
		}
		out.Definitions = append(out.Definitions, item)
	}
	// Recheck aggregate revision on databases with statement-level snapshots.
	var latest WorkspacePropertyCatalog
	err = tx.Where("space_id = ?", id).First(&latest).Error
	if out.Revision == 0 && errors.Is(err, gorm.ErrRecordNotFound) {
		return out, nil
	}
	if err != nil {
		return out, resourceReadError(err)
	}
	if latest.Revision != out.Revision || latest.Owner != head.Owner || latest.TenantID != head.TenantID {
		return out, code("query_changed")
	}
	return out, nil
}

func (s *Service) PropertyDefinitions(ctx context.Context, p Principal, id string) (contract.PropertyDefinitionView, error) {
	var out contract.PropertyDefinitionView
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.propertySpaceOn(tx, p, id, false); err != nil {
			return err
		}
		var err error
		out, err = s.propertyDefinitionsOn(tx, p, id)
		return err
	})
	if err != nil {
		return contract.PropertyDefinitionView{}, err
	}
	return out, nil
}

func propertyDefinitionReceipt(row WorkspaceRequest, id string) (contract.PropertyDefinitionReceipt, error) {
	digest, err := hex.DecodeString(row.RequestDigest)
	if row.ResourceID != id {
		return contract.PropertyDefinitionReceipt{}, code("idempotency_conflict")
	}
	if err != nil || len(digest) != 32 || row.ResultRevision == 0 || row.ResultRevision > 9007199254740991 || row.CreatedAt.IsZero() {
		return contract.PropertyDefinitionReceipt{}, code("dependency_unavailable")
	}
	return contract.PropertyDefinitionReceipt{SpaceID: id, RequestDigest: row.RequestDigest, ResultRevision: row.ResultRevision, CommittedAt: row.CreatedAt}, nil
}

// UpdatePropertyDefinitions supports additions and presentation changes. Stable
// key/type removal or replacement requires a separate explicit migration plan.
func (s *Service) UpdatePropertyDefinitions(ctx context.Context, p Principal, id string, q contract.PropertyDefinitionUpdate, key string) (contract.PropertyDefinitionReceipt, error) {
	var out contract.PropertyDefinitionReceipt
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	if q.ExpectedRevision >= 9007199254740991 || q.Definitions == nil || len(q.Definitions) > 200 {
		return out, code("invalid_request")
	}
	incoming := map[string]contract.PropertyDefinition{}
	for _, row := range q.Definitions {
		if _, exists := incoming[row.Key]; exists || !validPropertyDefinition(row) {
			return out, code("invalid_request")
		}
		incoming[row.Key] = row
	}
	raw, _ := json.Marshal(struct {
		SpaceID string                            `json:"space_id"`
		Request contract.PropertyDefinitionUpdate `json:"request"`
	}{id, q})
	digest := Hash(string(raw))
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.propertySpaceOn(tx, p, id, true); err != nil {
			return err
		}
		var receipt WorkspaceRequest
		err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "property.definitions", key).First(&receipt).Error
		if err == nil {
			if receipt.RequestDigest != digest {
				return code("idempotency_conflict")
			}
			out, err = propertyDefinitionReceipt(receipt, id)
			return err
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		current, err := s.propertyDefinitionsOn(tx, p, id)
		if err != nil {
			return err
		}
		if current.Revision != q.ExpectedRevision {
			return code("revision_conflict")
		}
		for _, old := range current.Definitions {
			next, ok := incoming[old.Key]
			if !ok || next.Type != old.Type {
				return code("unsupported_action")
			}
		}
		now := time.Now().UTC()
		revision := current.Revision + 1
		if current.Revision == 0 {
			inserted := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&WorkspacePropertyCatalog{SpaceID: id, Owner: p.UserID, TenantID: p.TenantID, Revision: revision, UpdatedAt: now})
			if inserted.Error != nil {
				return code("database_unavailable")
			}
			if inserted.RowsAffected != 1 {
				return code("revision_conflict")
			}
		} else {
			updated := tx.Model(&WorkspacePropertyCatalog{}).Where("space_id = ? AND owner = ? AND tenant_id = ? AND revision = ?", id, p.UserID, p.TenantID, current.Revision).Updates(map[string]any{"revision": revision, "updated_at": now})
			if updated.Error != nil {
				return code("database_unavailable")
			}
			if updated.RowsAffected != 1 {
				return code("revision_conflict")
			}
		}
		existing := map[string]contract.PropertyDefinition{}
		for _, row := range current.Definitions {
			existing[row.Key] = row
		}
		for _, row := range q.Definitions {
			old, ok := existing[row.Key]
			if ok {
				if old == row {
					continue
				}
				result := tx.Model(&WorkspacePropertyDefinition{}).Where("space_id = ? AND owner = ? AND tenant_id = ? AND key = ?", id, p.UserID, p.TenantID, row.Key).Updates(map[string]any{"display_name": row.DisplayName, "group": row.Group, "revision": revision, "updated_at": now})
				if result.Error != nil {
					return code("database_unavailable")
				}
				if result.RowsAffected != 1 {
					return code("revision_conflict")
				}
			} else {
				if err = tx.Create(&WorkspacePropertyDefinition{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, SpaceID: id, Key: row.Key, DisplayName: row.DisplayName, Group: row.Group, Type: row.Type, Revision: revision, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
					return code("database_unavailable")
				}
			}
		}
		receipt = WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: "property.definitions", RequestKey: key, RequestDigest: digest, ResourceID: id, ResultRevision: revision, CreatedAt: now}
		if err = tx.Create(&receipt).Error; err != nil {
			return code("database_unavailable")
		}
		out, err = propertyDefinitionReceipt(receipt, id)
		return err
	})
	if err != nil {
		return contract.PropertyDefinitionReceipt{}, err
	}
	return out, nil
}

func (s *Service) LookupPropertyDefinitionUpdate(ctx context.Context, p Principal, id, key string) (contract.PropertyDefinitionReceipt, error) {
	var out contract.PropertyDefinitionReceipt
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.propertySpaceOn(tx, p, id, false); err != nil {
			return err
		}
		var row WorkspaceRequest
		if err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "property.definitions", key).First(&row).Error; err != nil {
			return resourceReadError(err)
		}
		var err error
		out, err = propertyDefinitionReceipt(row, id)
		return err
	})
	if err != nil {
		return contract.PropertyDefinitionReceipt{}, err
	}
	return out, nil
}
