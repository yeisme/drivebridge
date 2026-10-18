package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"errors"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func propertyMigrationDigest(id string, request any) string {
	raw, _ := json.Marshal(struct {
		ResourceID string `json:"resource_id"`
		Request    any    `json:"request"`
	}{id, request})
	return Hash(string(raw))
}

func (s *Service) PropertyMigrationCapability(ctx context.Context, p Principal, space string) (contract.PropertyMigrationCapability, error) {
	if err := s.propertySpaceOn(s.DB.WithContext(ctx), p, space, false); err != nil {
		return contract.PropertyMigrationCapability{}, err
	}
	return contract.PropertyMigrationCapability{SpaceID: space, EditingEnabled: s.Config.PropertyEditing, StorageKind: "sidecar", MaxSidecars: propertyMigrationLimit, MarkdownSupported: false, ConversionPolicies: []string{"preserve", "parse_text", "scalar_text", "singleton_list"}}, nil
}

func (s *Service) propertyMigrationPlanOn(tx *gorm.DB, p Principal, id string, write bool) (OrganizationPlan, []OrganizationPlanItem, contract.PropertyMigrationPreview, error) {
	var plan OrganizationPlan
	var q contract.PropertyMigrationPreview
	query := tx
	if write {
		query = query.Clauses(clause.Locking{Strength: "UPDATE"})
	}
	if err := query.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.UserID, p.TenantID).First(&plan).Error; err != nil {
		return plan, nil, q, resourceReadError(err)
	}
	if err := s.propertySpaceOn(tx, p, plan.SpaceID, write); err != nil {
		return plan, nil, q, err
	}
	q, err := propertyMigrationRequest(plan)
	if err != nil {
		return plan, nil, q, err
	}
	var items []OrganizationPlanItem
	if err = tx.Where("plan_id = ?", id).Order("item_key ASC").Limit(propertyMigrationLimit + 1).Find(&items).Error; err != nil {
		return plan, nil, q, code("database_unavailable")
	}
	if len(items) > propertyMigrationLimit || int64(len(items)) != plan.ItemsTotal || plan.Revision == 0 || plan.Revision > 9007199254740991 || propertyMigrationImpact(q, items) != plan.ImpactDigest {
		return plan, nil, q, code("dependency_unavailable")
	}
	for _, item := range items {
		if item.Owner != p.UserID || item.ActionKind != propertyMigrationAction {
			return plan, nil, q, code("dependency_unavailable")
		}
		file, err := s.sidecarFileOn(tx, p, item.FileID, item.VersionID, false)
		if err != nil {
			return plan, nil, q, err
		}
		if file.SpaceID != plan.SpaceID {
			return plan, nil, q, code("not_found")
		}
		if err = s.allowOn(tx, p, plan.SpaceID, item.Path, "read"); err != nil {
			return plan, nil, q, err
		}
		if write {
			if err = s.allowOn(tx, p, file.SpaceID, file.Path, "write"); err != nil {
				return plan, nil, q, err
			}
		}
		var frozen propertyMigrationFrozen
		if json.Unmarshal([]byte(item.ActionDetail), &frozen) != nil || frozen.SidecarID != item.ItemKey || frozen.Revision == 0 || frozen.Revision >= 9007199254740991 {
			return plan, nil, q, code("dependency_unavailable")
		}
		for _, value := range []struct{ Kind, Raw string }{{frozen.SourceType, item.BeforeValue}, {q.TargetType, item.AfterValue}} {
			if value.Raw != "" && validatePropertyValue(value.Kind, json.RawMessage(value.Raw)) != nil {
				return plan, nil, q, code("dependency_unavailable")
			}
			if value.Kind == "reference" && value.Raw != "" {
				if err = s.sidecarReferenceOn(tx, p, json.RawMessage(value.Raw)); err != nil {
					return plan, nil, q, err
				}
			}
		}
	}
	return plan, items, q, nil
}

func migrationView(plan OrganizationPlan, items []OrganizationPlanItem, q contract.PropertyMigrationPreview, enabled bool) contract.PropertyMigrationView {
	out := contract.PropertyMigrationView{PlanID: plan.ID, SpaceID: plan.SpaceID, Revision: plan.Revision, ImpactDigest: plan.ImpactDigest, Phase: plan.Phase, EditingEnabled: enabled, Request: q, Items: []contract.PropertyMigrationItem{}}
	for _, item := range items {
		var frozen propertyMigrationFrozen
		_ = json.Unmarshal([]byte(item.ActionDetail), &frozen)
		out.Items = append(out.Items, contract.PropertyMigrationItem{FileID: item.FileID, VersionID: item.VersionID, SidecarRevision: frozen.Revision, Before: item.BeforeValue, After: item.AfterValue, Status: item.Status, Error: item.Error})
	}
	return out
}

func (s *Service) GetPropertyMigration(ctx context.Context, p Principal, id string) (contract.PropertyMigrationView, error) {
	var out contract.PropertyMigrationView
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		plan, items, q, err := s.propertyMigrationPlanOn(tx, p, id, false)
		if err != nil {
			return err
		}
		out = migrationView(plan, items, q, s.Config.PropertyEditing)
		return nil
	})
	if err != nil {
		return contract.PropertyMigrationView{}, err
	}
	return out, nil
}

func (s *Service) propertyMigrationReceiptOn(tx *gorm.DB, p Principal, space, stage, key string) (contract.PropertyMigrationReceipt, bool, error) {
	var out contract.PropertyMigrationReceipt
	var row WorkspaceRequest
	err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "property.migration."+stage, key).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return out, false, nil
	}
	if err != nil {
		return out, false, code("database_unavailable")
	}
	plan, _, _, err := s.propertyMigrationPlanOn(tx, p, row.ResourceID, false)
	if err != nil {
		return out, true, err
	}
	if plan.SpaceID != space {
		return out, true, code("idempotency_conflict")
	}
	digest, err := hex.DecodeString(row.RequestDigest)
	if err != nil || len(digest) != 32 || row.ResultRevision == 0 || row.ResultRevision > 9007199254740991 || row.CreatedAt.IsZero() {
		return out, true, code("dependency_unavailable")
	}
	return contract.PropertyMigrationReceipt{SpaceID: space, PlanID: plan.ID, Stage: stage, RequestDigest: row.RequestDigest, ResultRevision: row.ResultRevision, CommittedAt: row.CreatedAt}, true, nil
}

func createPropertyMigrationReceipt(tx *gorm.DB, p Principal, plan OrganizationPlan, stage, key, digest string) (contract.PropertyMigrationReceipt, error) {
	row := WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: "property.migration." + stage, RequestKey: key, RequestDigest: digest, ResourceID: plan.ID, ResultRevision: plan.Revision, CreatedAt: time.Now().UTC()}
	if err := tx.Create(&row).Error; err != nil {
		return contract.PropertyMigrationReceipt{}, code("database_unavailable")
	}
	return contract.PropertyMigrationReceipt{SpaceID: plan.SpaceID, PlanID: plan.ID, Stage: stage, RequestDigest: digest, ResultRevision: row.ResultRevision, CommittedAt: row.CreatedAt}, nil
}

func (s *Service) LookupPropertyMigration(ctx context.Context, p Principal, space string, q contract.PropertyMigrationLookup) (contract.PropertyMigrationReceipt, error) {
	var out contract.PropertyMigrationReceipt
	if err := collectionChangeKey(q.IdempotencyKey); err != nil {
		return out, err
	}
	if q.Stage != "preview" && q.Stage != "confirm" && q.Stage != "execute" {
		return out, code("invalid_request")
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.propertySpaceOn(tx, p, space, false); err != nil {
			return err
		}
		var found bool
		var err error
		out, found, err = s.propertyMigrationReceiptOn(tx, p, space, q.Stage, q.IdempotencyKey)
		if err == nil && !found {
			return code("not_found")
		}
		return err
	})
	if err != nil {
		return contract.PropertyMigrationReceipt{}, err
	}
	return out, nil
}

func (s *Service) PreviewPropertyMigration(ctx context.Context, p Principal, space string, q contract.PropertyMigrationPreview, key string) (contract.PropertyMigrationReceipt, error) {
	var out contract.PropertyMigrationReceipt
	if !s.Config.PropertyEditing {
		return out, code("unsupported_action")
	}
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	if !validPropertyMigration(q) {
		return out, code("invalid_request")
	}
	digest := propertyMigrationDigest(space, q)
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.propertySpaceOn(tx, p, space, true); err != nil {
			return err
		}
		previous, found, err := s.propertyMigrationReceiptOn(tx, p, space, "preview", key)
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
		catalog, source, err := s.propertyMigrationCatalogOn(tx, p, space, q)
		if err != nil {
			return err
		}
		rows, err := s.propertyMigrationRowsOn(tx, p, space)
		if err != nil {
			return err
		}
		now := time.Now().UTC()
		rawRequest, _ := json.Marshal(q)
		actions, _ := json.Marshal([]OrganizationAction{{Kind: propertyMigrationAction, Value: string(rawRequest)}})
		plan := OrganizationPlan{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, SpaceID: space, Actions: string(actions), Revision: 1, Phase: "previewed", Risk: "approval_required", ItemsTotal: int64(len(rows)), Key: Hash(p.UserID + "\x00" + p.TenantID + "\x00property.migration\x00" + key), Fingerprint: digest, CreatedAt: now, UpdatedAt: now}
		items := make([]OrganizationPlanItem, 0, len(rows))
		for _, row := range rows {
			file, err := s.sidecarFileOn(tx, p, row.FileID, row.VersionID, false)
			if err != nil {
				return err
			}
			if file.SpaceID != space {
				return code("dependency_unavailable")
			}
			if err = s.allowOn(tx, p, space, file.Path, "write"); err != nil {
				return err
			}
			loaded, values, err := s.sidecarOn(tx, p, file, row.VersionID)
			if err != nil {
				return err
			}
			if loaded.ID != row.ID || loaded.Revision != row.Revision || loaded.ValuesJSON != row.ValuesJSON {
				return code("query_changed")
			}
			if row.Revision >= 9007199254740991 {
				return code("revision_conflict")
			}
			frozen := propertyMigrationFrozen{SidecarID: row.ID, Revision: row.Revision, ValuesDigest: Hash(row.ValuesJSON), HeadVersion: file.VersionID, SourceType: source}
			item := OrganizationPlanItem{ID: ID(), PlanID: plan.ID, ItemKey: row.ID, Owner: p.UserID, FileID: row.FileID, VersionID: row.VersionID, Path: file.Path, ActionKind: propertyMigrationAction, Status: "pending", CreatedAt: now, UpdatedAt: now}
			before, exists := values[q.SourceKey]
			if exists {
				if source == "reference" {
					if err = s.sidecarReferenceOn(tx, p, before); err != nil {
						return err
					}
				}
				if validatePropertyValue(source, before) == nil {
					item.BeforeValue = string(before)
				}
				converted, conversionErr := convertPropertyMigration(before, source, q)
				if conversionErr != nil {
					frozen.PreviewError = "incompatible_value"
				} else {
					item.BeforeValue = string(before)
					item.AfterValue = string(converted)
					if q.TargetType == "reference" {
						if err = s.sidecarReferenceOn(tx, p, converted); err != nil {
							return err
						}
					}
				}
			}
			if q.SourceKey != q.TargetKey {
				if _, conflict := values[q.TargetKey]; conflict {
					frozen.PreviewError = "target_key_exists"
				}
			}
			// Validate other known values without hiding a source conversion error.
			delete(values, q.SourceKey)
			if _, _, err = s.sidecarVisibleOn(tx, p, values, catalog); err != nil {
				return err
			}
			if frozen.PreviewError != "" {
				item.Status, item.Error = "failed", frozen.PreviewError
			}
			detail, _ := json.Marshal(frozen)
			item.ActionDetail = string(detail)
			items = append(items, item)
		}
		plan.ImpactDigest = propertyMigrationImpact(q, items)
		if err = tx.Create(&plan).Error; err != nil {
			return code("database_unavailable")
		}
		if len(items) > 0 {
			if err = tx.Create(&items).Error; err != nil {
				return code("database_unavailable")
			}
		}
		out, err = createPropertyMigrationReceipt(tx, p, plan, "preview", key, digest)
		return err
	})
	if err != nil {
		return contract.PropertyMigrationReceipt{}, err
	}
	return out, nil
}
