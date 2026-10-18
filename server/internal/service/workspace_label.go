package service

import (
	"context"
	"encoding/json"
	"errors"
	"sort"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func validLabelDefinition(row contract.LabelDefinitionInput) bool {
	return validPropertyDefinition(contract.PropertyDefinition{Key: row.Key, DisplayName: row.DisplayName, Group: row.Group, Type: "text"})
}

func (s *Service) labelDefinitionsOn(tx *gorm.DB, p Principal, id string) (contract.LabelDefinitionPage, error) {
	out := contract.LabelDefinitionPage{SpaceID: id, Namespace: "manual", ManagementEnabled: s.Config.LabelManagement, Definitions: []contract.LabelDefinition{}}
	var head WorkspaceLabelCatalog
	err := tx.Where("space_id = ? AND namespace = ?", id, "manual").First(&head).Error
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
	var rows []WorkspaceLabelDefinition
	if err = tx.Where("space_id = ? AND namespace = ?", id, "manual").Order("key ASC").Limit(201).Find(&rows).Error; err != nil {
		return out, code("database_unavailable")
	}
	if len(rows) > 200 {
		return out, code("query_budget_exceeded")
	}
	for _, row := range rows {
		if row.Owner != p.UserID || row.TenantID != p.TenantID || row.Revision == 0 || row.Revision > out.Revision || !validLabelDefinition(contract.LabelDefinitionInput{Key: row.Key, DisplayName: row.DisplayName, Group: row.Group}) {
			return out, code("dependency_unavailable")
		}
		out.Definitions = append(out.Definitions, contract.LabelDefinition{Key: row.Key, DisplayName: row.DisplayName, Group: row.Group, Revision: row.Revision})
	}
	var latest WorkspaceLabelCatalog
	err = tx.Where("space_id = ? AND namespace = ?", id, "manual").First(&latest).Error
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

func (s *Service) LabelDefinitions(ctx context.Context, p Principal, id, namespace, cursor string, limit int) (contract.LabelDefinitionPage, error) {
	var out contract.LabelDefinitionPage
	if namespace != "manual" || limit < 0 || limit > 200 {
		return out, code("invalid_request")
	}
	limit = orgPageLimit(limit)
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.propertySpaceOn(tx, p, id, false); err != nil {
			return err
		}
		var err error
		out, err = s.labelDefinitionsOn(tx, p, id)
		if err != nil {
			return err
		}
		binding, _ := json.Marshal([]string{"label-definitions", p.UserID, p.TenantID, p.ScopeSpace, id, namespace, revisionString(uint64(limit))})
		last := ""
		if cursor != "" {
			page, err := s.openOrgCursor(cursor, p.UserID, string(binding), revisionString(out.Revision))
			if err != nil {
				return err
			}
			last = page.LastKey
		}
		start := sort.Search(len(out.Definitions), func(i int) bool { return out.Definitions[i].Key > last })
		out.Definitions = out.Definitions[start:]
		if len(out.Definitions) > limit {
			out.Definitions = out.Definitions[:limit]
			out.NextCursor, err = s.sealOrgCursor(orgCursorPayload{Owner: p.UserID, Query: string(binding), CatalogueRevision: revisionString(out.Revision), LastKey: out.Definitions[limit-1].Key, Kind: "label-definitions"})
		}
		return err
	})
	if err != nil {
		return contract.LabelDefinitionPage{}, err
	}
	return out, nil
}

func labelDefinitionReceipt(row WorkspaceRequest, id string) (contract.LabelDefinitionReceipt, error) {
	base, err := propertyDefinitionReceipt(row, id)
	if err != nil {
		return contract.LabelDefinitionReceipt{}, err
	}
	return contract.LabelDefinitionReceipt{SpaceID: id, Namespace: "manual", RequestDigest: base.RequestDigest, ResultRevision: base.ResultRevision, CommittedAt: base.CommittedAt}, nil
}

func (s *Service) UpdateLabelDefinitions(ctx context.Context, p Principal, id string, q contract.LabelDefinitionUpdate, key string) (contract.LabelDefinitionReceipt, error) {
	var out contract.LabelDefinitionReceipt
	if !s.Config.LabelManagement {
		return out, code("unsupported_action")
	}
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	if q.ExpectedRevision >= 9007199254740991 || q.Definitions == nil || len(q.Definitions) > 200 {
		return out, code("invalid_request")
	}
	incoming := map[string]contract.LabelDefinitionInput{}
	for _, row := range q.Definitions {
		if _, ok := incoming[row.Key]; ok || !validLabelDefinition(row) {
			return out, code("invalid_request")
		}
		incoming[row.Key] = row
	}
	raw, _ := json.Marshal(struct {
		SpaceID string                         `json:"space_id"`
		Request contract.LabelDefinitionUpdate `json:"request"`
	}{id, q})
	digest := Hash(string(raw))
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.propertySpaceOn(tx, p, id, true); err != nil {
			return err
		}
		var receipt WorkspaceRequest
		err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "label.definitions", key).First(&receipt).Error
		if err == nil {
			if receipt.RequestDigest != digest {
				return code("idempotency_conflict")
			}
			out, err = labelDefinitionReceipt(receipt, id)
			return err
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		current, err := s.labelDefinitionsOn(tx, p, id)
		if err != nil {
			return err
		}
		if current.Revision != q.ExpectedRevision {
			return code("revision_conflict")
		}
		for _, old := range current.Definitions {
			if _, ok := incoming[old.Key]; !ok {
				return code("unsupported_action")
			}
		}
		now := time.Now().UTC()
		revision := current.Revision + 1
		if current.Revision == 0 {
			inserted := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&WorkspaceLabelCatalog{SpaceID: id, Namespace: "manual", Owner: p.UserID, TenantID: p.TenantID, Revision: revision, UpdatedAt: now})
			if inserted.Error != nil {
				return code("database_unavailable")
			}
			if inserted.RowsAffected != 1 {
				return code("revision_conflict")
			}
		} else {
			updated := tx.Model(&WorkspaceLabelCatalog{}).Where("space_id = ? AND namespace = ? AND owner = ? AND tenant_id = ? AND revision = ?", id, "manual", p.UserID, p.TenantID, current.Revision).Updates(map[string]any{"revision": revision, "updated_at": now})
			if updated.Error != nil {
				return code("database_unavailable")
			}
			if updated.RowsAffected != 1 {
				return code("revision_conflict")
			}
		}
		existing := map[string]contract.LabelDefinition{}
		for _, row := range current.Definitions {
			existing[row.Key] = row
		}
		for _, row := range q.Definitions {
			if old, ok := existing[row.Key]; ok {
				if old.DisplayName == row.DisplayName && old.Group == row.Group {
					continue
				}
				updated := tx.Model(&WorkspaceLabelDefinition{}).Where("space_id = ? AND namespace = ? AND key = ? AND revision = ?", id, "manual", row.Key, old.Revision).Updates(map[string]any{"display_name": row.DisplayName, "group": row.Group, "revision": revision, "updated_at": now})
				if updated.Error != nil {
					return code("database_unavailable")
				}
				if updated.RowsAffected != 1 {
					return code("revision_conflict")
				}
			} else if err := tx.Create(&WorkspaceLabelDefinition{ID: ID(), SpaceID: id, Namespace: "manual", Key: row.Key, Owner: p.UserID, TenantID: p.TenantID, DisplayName: row.DisplayName, Group: row.Group, Revision: revision, CreatedAt: now, UpdatedAt: now}).Error; err != nil {
				return code("database_unavailable")
			}
		}
		receipt = WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: "label.definitions", RequestKey: key, RequestDigest: digest, ResourceID: id, ResultRevision: revision, CreatedAt: now}
		if err := tx.Create(&receipt).Error; err != nil {
			return code("database_unavailable")
		}
		out, err = labelDefinitionReceipt(receipt, id)
		return err
	})
	if err != nil {
		return contract.LabelDefinitionReceipt{}, err
	}
	return out, nil
}

func (s *Service) LookupLabelDefinitionUpdate(ctx context.Context, p Principal, id, key string) (contract.LabelDefinitionReceipt, error) {
	var out contract.LabelDefinitionReceipt
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.propertySpaceOn(tx, p, id, false); err != nil {
			return err
		}
		var row WorkspaceRequest
		if err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "label.definitions", key).First(&row).Error; err != nil {
			return resourceReadError(err)
		}
		var err error
		out, err = labelDefinitionReceipt(row, id)
		return err
	})
	if err != nil {
		return contract.LabelDefinitionReceipt{}, err
	}
	return out, nil
}
