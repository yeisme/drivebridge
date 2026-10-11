package service

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"path"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const sidecarBudget = 256 * 1024

// This first slice is binary-only. No filename test grants access to Pinax or
// permits modifying a structured asset; sidecars never modify provider bytes.
func sidecarBinary(file File) bool {
	switch strings.ToLower(path.Ext(file.Path)) {
	case ".pdf", ".png", ".jpg", ".jpeg", ".gif", ".webp", ".mp3", ".wav", ".mp4", ".webm":
		return true
	}
	return false
}

func (s *Service) sidecarFileOn(tx *gorm.DB, p Principal, id, version string, write bool) (File, error) {
	if !collectionHumanPrincipal(p) {
		return File{}, code("forbidden")
	}
	if !propertyHexID(id) || !propertyHexID(version) {
		return File{}, code("invalid_request")
	}
	if err := savedQueryActorOn(tx, p); err != nil {
		return File{}, err
	}
	var file File
	query := tx
	if write {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.Where("id = ? AND owner = ?", id, p.UserID).First(&file).Error; err != nil {
		return File{}, resourceReadError(err)
	}
	if file.Trashed || file.PurgeStartedAt != nil || workspaceStagingPath(file.Path) {
		return File{}, code("not_found")
	}
	if !sidecarBinary(file) {
		return File{}, code("unsupported_action")
	}
	if err := s.collectionReadSpaceOn(tx, p, file.SpaceID); err != nil {
		return File{}, err
	}
	if err := s.allowOn(tx, p, file.SpaceID, file.Path, "read"); err != nil {
		return File{}, err
	}
	if write {
		if err := s.allowOn(tx, p, file.SpaceID, file.Path, "write"); err != nil {
			return File{}, err
		}
		if file.VersionID != version {
			return File{}, code("version_conflict")
		}
	}
	var fixed Version
	if err := tx.Where("id = ? AND owner = ? AND file_id = ?", version, p.UserID, id).First(&fixed).Error; err != nil {
		return File{}, resourceReadError(err)
	}
	return file, nil
}

func decodeSidecarValues(raw string) (map[string]json.RawMessage, error) {
	if len(raw) > sidecarBudget || !utf8.ValidString(raw) || !propertyJSONSurrogates([]byte(raw)) {
		return nil, code("dependency_unavailable")
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return nil, code("dependency_unavailable")
	}
	values := map[string]json.RawMessage{}
	for decoder.More() {
		token, err = decoder.Token()
		key, ok := token.(string)
		if err != nil || !ok || !propertyKey.MatchString(key) {
			return nil, code("dependency_unavailable")
		}
		if _, exists := values[key]; exists || len(values) >= 200 {
			return nil, code("dependency_unavailable")
		}
		var value json.RawMessage
		if err = decoder.Decode(&value); err != nil || len(value) > propertyValueBudget {
			return nil, code("dependency_unavailable")
		}
		values[key] = value
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return nil, code("dependency_unavailable")
	}
	if decoder.Decode(new(any)) != io.EOF {
		return nil, code("dependency_unavailable")
	}
	return values, nil
}

func (s *Service) sidecarOn(tx *gorm.DB, p Principal, file File, version string) (WorkspacePropertySidecar, map[string]json.RawMessage, error) {
	var row WorkspacePropertySidecar
	err := tx.Where("file_id = ? AND version_id = ?", file.ID, version).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Old projections are not canonical sidecars. Require explicit migration
		// rather than silently opening an empty store over an existing projection.
		var count int64
		if err = tx.Model(&WorkspaceFilePropertyProjection{}).Where("file_id = ? AND version_id = ?", file.ID, version).Count(&count).Error; err != nil {
			return row, nil, code("database_unavailable")
		}
		if count != 0 {
			return row, nil, code("dependency_unavailable")
		}
		return row, map[string]json.RawMessage{}, nil
	}
	if err != nil {
		return row, nil, code("database_unavailable")
	}
	if row.Owner != p.UserID || row.TenantID != p.TenantID || row.SpaceID != file.SpaceID || row.Revision == 0 || row.Revision > 9007199254740991 || row.DefinitionRevision == 0 {
		return row, nil, code("dependency_unavailable")
	}
	values, err := decodeSidecarValues(row.ValuesJSON)
	return row, values, err
}

func (s *Service) sidecarReferenceOn(tx *gorm.DB, p Principal, raw json.RawMessage) error {
	var ref string
	if json.Unmarshal(raw, &ref) != nil {
		return code("invalid_request")
	}
	id, version, err := s.resolveRef(ref)
	if err != nil {
		return err
	}
	_, err = s.resourceFileOn(tx, p, id, version)
	return err
}

// Only currently defined values are exposed; unknown raw values stay stored
// intact but are not disclosed as untyped data that could contain hidden refs.
func (s *Service) sidecarVisibleOn(tx *gorm.DB, p Principal, values map[string]json.RawMessage, catalog contract.PropertyDefinitionView) (map[string]json.RawMessage, []string, error) {
	types := map[string]string{}
	for _, d := range catalog.Definitions {
		types[d.Key] = d.Type
	}
	visible := map[string]json.RawMessage{}
	unknown := []string{}
	for key, raw := range values {
		kind, ok := types[key]
		if !ok {
			unknown = append(unknown, key)
			continue
		}
		if err := validatePropertyValue(kind, raw); err != nil {
			return nil, nil, code("dependency_unavailable")
		}
		if kind == "reference" {
			if err := s.sidecarReferenceOn(tx, p, raw); err != nil {
				return nil, nil, err
			}
		}
		visible[key] = raw
	}
	sort.Strings(unknown)
	return visible, unknown, nil
}

func (s *Service) FilePropertySidecar(ctx context.Context, p Principal, id, version string) (contract.PropertySidecarView, error) {
	var out contract.PropertySidecarView
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		file, err := s.sidecarFileOn(tx, p, id, version, false)
		if err != nil {
			return err
		}
		catalog, err := s.propertyDefinitionsOn(tx, p, file.SpaceID)
		if err != nil {
			return err
		}
		row, values, err := s.sidecarOn(tx, p, file, version)
		if err != nil {
			return err
		}
		visible, unknown, err := s.sidecarVisibleOn(tx, p, values, catalog)
		if err != nil {
			return err
		}
		latestFile, err := s.sidecarFileOn(tx, p, id, version, false)
		if err != nil {
			return err
		}
		if latestFile.SpaceID != file.SpaceID || latestFile.Path != file.Path {
			return code("query_changed")
		}
		latestCatalog, err := s.propertyDefinitionsOn(tx, p, file.SpaceID)
		if err != nil {
			return err
		}
		latest, _, err := s.sidecarOn(tx, p, file, version)
		if err != nil {
			return err
		}
		if latestCatalog.Revision != catalog.Revision || latest.Revision != row.Revision {
			return code("query_changed")
		}
		out = contract.PropertySidecarView{FileID: id, VersionID: version, DefinitionRevision: catalog.Revision, Revision: row.Revision, StorageKind: "sidecar", Values: visible, UnknownKeys: unknown}
		return nil
	})
	if err != nil {
		return contract.PropertySidecarView{}, err
	}
	return out, nil
}

func (s *Service) sidecarReceiptOn(tx *gorm.DB, p Principal, id, version, key string) (contract.PropertySidecarReceipt, bool, error) {
	var out contract.PropertySidecarReceipt
	var receipt WorkspaceRequest
	err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "property.sidecar", key).First(&receipt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return out, false, nil
	}
	if err != nil {
		return out, false, code("database_unavailable")
	}
	var row WorkspacePropertySidecar
	if err = tx.Where("id = ? AND owner = ? AND tenant_id = ?", receipt.ResourceID, p.UserID, p.TenantID).First(&row).Error; err != nil {
		return out, true, resourceReadError(err)
	}
	if row.FileID != id || row.VersionID != version {
		return out, true, code("idempotency_conflict")
	}
	digest, digestErr := hex.DecodeString(receipt.RequestDigest)
	if digestErr != nil || len(digest) != 32 || receipt.ResultRevision == 0 || receipt.ResultRevision > 9007199254740991 || receipt.CreatedAt.IsZero() {
		return out, true, code("dependency_unavailable")
	}
	out = contract.PropertySidecarReceipt{FileID: id, VersionID: version, RequestDigest: receipt.RequestDigest, ResultRevision: receipt.ResultRevision, CommittedAt: receipt.CreatedAt}
	return out, true, nil
}

func (s *Service) PatchFilePropertySidecar(ctx context.Context, p Principal, id string, q contract.PropertySidecarPatch, key string) (contract.PropertySidecarReceipt, error) {
	var out contract.PropertySidecarReceipt
	if !s.Config.PropertyEditing {
		return out, code("unsupported_action")
	}
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	if !propertyHexID(id) || !propertyHexID(q.BaseVersionID) || q.DefinitionRevision == 0 || q.DefinitionRevision > 9007199254740991 || q.ExpectedRevision >= 9007199254740991 || q.Updates == nil || q.RemoveKeys == nil || len(q.Updates)+len(q.RemoveKeys) == 0 || len(q.Updates)+len(q.RemoveKeys) > 200 {
		return out, code("invalid_request")
	}
	seen := map[string]bool{}
	for k, raw := range q.Updates {
		if !propertyKey.MatchString(k) || len(raw) > propertyValueBudget || !json.Valid(raw) || !utf8.Valid(raw) || !propertyJSONSurrogates(raw) {
			return out, code("invalid_request")
		}
		seen[k] = true
	}
	for _, k := range q.RemoveKeys {
		if !propertyKey.MatchString(k) || seen[k] {
			return out, code("invalid_request")
		}
		seen[k] = true
	}
	raw, err := json.Marshal(struct {
		FileID  string                        `json:"file_id"`
		Request contract.PropertySidecarPatch `json:"request"`
	}{id, q})
	if err != nil || len(raw) > sidecarBudget {
		return out, code("invalid_request")
	}
	digest := Hash(string(raw))
	s.mu.Lock()
	defer s.mu.Unlock()
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// A successful replay remains queryable after head changes, but must not
		// grant access after revocation. It returns no old values or references.
		file, err := s.sidecarFileOn(tx, p, id, q.BaseVersionID, false)
		if err != nil {
			return err
		}
		previous, found, err := s.sidecarReceiptOn(tx, p, id, q.BaseVersionID, key)
		if err != nil {
			return err
		}
		if found {
			if previous.RequestDigest != digest {
				return code("idempotency_conflict")
			}
			out = previous
			return nil
		}
		// Match migration's catalog-before-file lock order. The initial unlocked
		// read only selects the catalog; reauthorize the locked file below.
		var head WorkspacePropertyCatalog
		if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("space_id = ?", file.SpaceID).First(&head).Error; err != nil {
			return resourceReadError(err)
		}
		file, err = s.sidecarFileOn(tx, p, id, q.BaseVersionID, true)
		if err != nil {
			return err
		}
		if file.SpaceID != head.SpaceID {
			return code("revision_conflict")
		}
		catalog, err := s.propertyDefinitionsOn(tx, p, file.SpaceID)
		if err != nil {
			return err
		}
		if catalog.Revision != q.DefinitionRevision {
			return code("revision_conflict")
		}
		row, values, err := s.sidecarOn(tx, p, file, q.BaseVersionID)
		if err != nil {
			return err
		}
		if row.Revision != q.ExpectedRevision {
			return code("revision_conflict")
		}
		types := map[string]string{}
		for _, d := range catalog.Definitions {
			types[d.Key] = d.Type
		}
		for k, raw := range q.Updates {
			kind, ok := types[k]
			if !ok {
				return code("invalid_request")
			}
			if err = validatePropertyValue(kind, raw); err != nil {
				return err
			}
			values[k] = append(json.RawMessage{}, raw...)
		}
		for _, k := range q.RemoveKeys {
			if _, ok := types[k]; !ok {
				return code("unsupported_action")
			}
			delete(values, k)
		}
		if _, _, err = s.sidecarVisibleOn(tx, p, values, catalog); err != nil {
			return err
		}
		if len(values) > 200 {
			return code("query_budget_exceeded")
		}
		// Assemble without normalizing untouched RawMessage bytes.
		keys := make([]string, 0, len(values))
		for k := range values {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		var encoded bytes.Buffer
		encoded.WriteByte('{')
		for i, k := range keys {
			if i > 0 {
				encoded.WriteByte(',')
			}
			name, _ := json.Marshal(k)
			encoded.Write(name)
			encoded.WriteByte(':')
			encoded.Write(values[k])
		}
		encoded.WriteByte('}')
		if encoded.Len() > sidecarBudget {
			return code("query_budget_exceeded")
		}
		now := time.Now().UTC()
		revision := row.Revision + 1
		if row.ID == "" {
			row = WorkspacePropertySidecar{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, SpaceID: file.SpaceID, FileID: id, VersionID: q.BaseVersionID, Revision: revision, DefinitionRevision: catalog.Revision, ValuesJSON: encoded.String(), CreatedAt: now, UpdatedAt: now}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
			if result.Error != nil {
				return code("database_unavailable")
			}
			if result.RowsAffected != 1 {
				return code("revision_conflict")
			}
		} else {
			result := tx.Model(&WorkspacePropertySidecar{}).Where("id = ? AND revision = ?", row.ID, q.ExpectedRevision).Updates(map[string]any{"revision": revision, "definition_revision": catalog.Revision, "values_json": encoded.String(), "updated_at": now})
			if result.Error != nil {
				return code("database_unavailable")
			}
			if result.RowsAffected != 1 {
				return code("revision_conflict")
			}
		}
		receipt := WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: "property.sidecar", RequestKey: key, RequestDigest: digest, ResourceID: row.ID, ResultRevision: revision, CreatedAt: now}
		if err = tx.Create(&receipt).Error; err != nil {
			return code("database_unavailable")
		}
		out = contract.PropertySidecarReceipt{FileID: id, VersionID: q.BaseVersionID, RequestDigest: digest, ResultRevision: revision, CommittedAt: now}
		return nil
	})
	if err != nil {
		return contract.PropertySidecarReceipt{}, err
	}
	return out, nil
}

func (s *Service) LookupFilePropertySidecar(ctx context.Context, p Principal, id, version, key string) (contract.PropertySidecarReceipt, error) {
	var out contract.PropertySidecarReceipt
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := s.sidecarFileOn(tx, p, id, version, false); err != nil {
			return err
		}
		var found bool
		var err error
		out, found, err = s.sidecarReceiptOn(tx, p, id, version, key)
		if err != nil {
			return err
		}
		if !found {
			return code("not_found")
		}
		return nil
	})
	if err != nil {
		return contract.PropertySidecarReceipt{}, err
	}
	return out, nil
}
