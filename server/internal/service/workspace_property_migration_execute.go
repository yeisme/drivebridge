package service

import (
	"context"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func validMigrationDecision(q contract.PropertyMigrationDecision) bool {
	digest, err := hex.DecodeString(q.ImpactDigest)
	return q.ExpectedRevision > 0 && q.ExpectedRevision < 9007199254740991 && err == nil && len(digest) == 32
}

func (s *Service) ConfirmPropertyMigration(ctx context.Context, p Principal, id string, q contract.PropertyMigrationDecision, key string) (contract.PropertyMigrationReceipt, error) {
	return s.decidePropertyMigration(ctx, p, id, q, key, false)
}

func (s *Service) ExecutePropertyMigration(ctx context.Context, p Principal, id string, q contract.PropertyMigrationDecision, key string) (contract.PropertyMigrationReceipt, error) {
	return s.decidePropertyMigration(ctx, p, id, q, key, true)
}

func (s *Service) decidePropertyMigration(ctx context.Context, p Principal, id string, decision contract.PropertyMigrationDecision, key string, execute bool) (contract.PropertyMigrationReceipt, error) {
	var out contract.PropertyMigrationReceipt
	if !s.Config.PropertyEditing {
		return out, code("unsupported_action")
	}
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	if !validMigrationDecision(decision) {
		return out, code("invalid_request")
	}
	stage := "confirm"
	if execute {
		stage = "execute"
	}
	digest := propertyMigrationDigest(id, decision)
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		plan, items, q, err := s.propertyMigrationPlanOn(tx, p, id, true)
		if err != nil {
			return err
		}
		previous, found, err := s.propertyMigrationReceiptOn(tx, p, plan.SpaceID, stage, key)
		if err != nil {
			return err
		}
		if found {
			if previous.PlanID != id || previous.RequestDigest != digest {
				return code("idempotency_conflict")
			}
			out = previous
			return nil
		}
		if decision.ExpectedRevision != plan.Revision || decision.ImpactDigest != plan.ImpactDigest {
			return code("revision_conflict")
		}
		nextPhase := "confirmed"
		if execute {
			if plan.Phase != "confirmed" || plan.ConfirmedBy != p.UserID {
				return code("approval_required")
			}
			nextPhase, err = s.executePropertyMigrationOn(tx, p, plan, items, q)
			if err != nil {
				return err
			}
		} else {
			if plan.Phase != "previewed" {
				return code("revision_conflict")
			}
			// Confirmation records the exact preview, including exclusions; it
			// never grants authority or silently recomputes conversion results.
			if _, _, err = s.propertyMigrationCatalogOn(tx, p, plan.SpaceID, q); err != nil {
				return err
			}
		}
		now := time.Now().UTC()
		changed := tx.Model(&OrganizationPlan{}).Where("id = ? AND owner = ? AND tenant_id = ? AND revision = ? AND phase = ?", id, p.UserID, p.TenantID, plan.Revision, plan.Phase).Updates(map[string]any{"phase": nextPhase, "revision": plan.Revision + 1, "confirmed_by": p.UserID, "updated_at": now})
		if changed.Error != nil {
			return code("database_unavailable")
		}
		if changed.RowsAffected != 1 {
			return code("revision_conflict")
		}
		plan.Revision++
		plan.Phase = nextPhase
		out, err = createPropertyMigrationReceipt(tx, p, plan, stage, key, digest)
		return err
	})
	if err != nil {
		return contract.PropertyMigrationReceipt{}, err
	}
	return out, nil
}

// All rows are preflighted before any value changes. A dictionary has one type
// per key, so a partial type/key cutover would make its remaining values unreadable.
// A blocked execution therefore records per-item outcomes but changes no values.
func (s *Service) executePropertyMigrationOn(tx *gorm.DB, p Principal, plan OrganizationPlan, items []OrganizationPlanItem, q contract.PropertyMigrationPreview) (string, error) {
	catalog, source, err := s.propertyMigrationCatalogOn(tx, p, plan.SpaceID, q)
	if err != nil {
		return "", err
	}
	rows, err := s.propertyMigrationRowsOn(tx, p, plan.SpaceID)
	if err != nil {
		return "", err
	}
	if len(rows) != len(items) {
		return "", code("revision_conflict")
	}
	encoded := make([]string, len(items))
	blocked := false
	for i := range items {
		item := &items[i]
		var frozen propertyMigrationFrozen
		if json.Unmarshal([]byte(item.ActionDetail), &frozen) != nil || frozen.SourceType != source || frozen.SidecarID != item.ItemKey || rows[i].ID != frozen.SidecarID {
			return "", code("revision_conflict")
		}
		var locked File
		if err = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner = ?", item.FileID, p.UserID).First(&locked).Error; err != nil {
			return "", resourceReadError(err)
		}
		file, err := s.sidecarFileOn(tx, p, item.FileID, item.VersionID, false)
		if err != nil {
			return "", err
		}
		if err = s.allowOn(tx, p, file.SpaceID, file.Path, "write"); err != nil {
			return "", err
		}
		row, values, err := s.sidecarOn(tx, p, file, item.VersionID)
		if err != nil {
			return "", err
		}
		if file.SpaceID != plan.SpaceID || file.Path != item.Path || file.VersionID != frozen.HeadVersion || row.ID != frozen.SidecarID || row.Revision != frozen.Revision || Hash(row.ValuesJSON) != frozen.ValuesDigest {
			item.Status, item.Error = "stale", "source_changed"
			blocked = true
			continue
		}
		if frozen.PreviewError != "" {
			item.Status, item.Error = "failed", frozen.PreviewError
			blocked = true
			continue
		}
		if before, exists := values[q.SourceKey]; exists {
			converted, conversionErr := convertPropertyMigration(before, source, q)
			if conversionErr != nil || string(before) != item.BeforeValue || string(converted) != item.AfterValue {
				return "", code("dependency_unavailable")
			}
			if q.SourceKey != q.TargetKey {
				if _, conflict := values[q.TargetKey]; conflict {
					return "", code("revision_conflict")
				}
			}
			delete(values, q.SourceKey)
			values[q.TargetKey] = converted
		} else if item.BeforeValue != "" || item.AfterValue != "" {
			return "", code("dependency_unavailable")
		}
		for j := range catalog.Definitions {
			if catalog.Definitions[j].Key == q.SourceKey {
				catalog.Definitions[j].Key, catalog.Definitions[j].Type = q.TargetKey, q.TargetType
			}
		}
		if _, _, err = s.sidecarVisibleOn(tx, p, values, catalog); err != nil {
			return "", err
		}
		encoded[i], err = encodeMigrationValues(values)
		if err != nil {
			return "", err
		}
	}
	now := time.Now().UTC()
	if blocked {
		for i := range items {
			if items[i].Status == "pending" {
				items[i].Status, items[i].Error = "failed", "migration_blocked"
			}
			if err = persistMigrationItem(tx, items[i], now); err != nil {
				return "", err
			}
		}
		return "partial", nil
	}
	// Catalog CAS fences concurrent dictionary changes and sidecar PATCH creation.
	head := tx.Model(&WorkspacePropertyCatalog{}).Where("space_id = ? AND owner = ? AND tenant_id = ? AND revision = ?", plan.SpaceID, p.UserID, p.TenantID, q.ExpectedDefinitionRevision).Updates(map[string]any{"revision": q.ExpectedDefinitionRevision + 1, "updated_at": now})
	if head.Error != nil {
		return "", code("database_unavailable")
	}
	if head.RowsAffected != 1 {
		return "", code("revision_conflict")
	}
	definition := tx.Model(&WorkspacePropertyDefinition{}).Where("space_id = ? AND owner = ? AND tenant_id = ? AND key = ? AND type = ?", plan.SpaceID, p.UserID, p.TenantID, q.SourceKey, source).Updates(map[string]any{"key": q.TargetKey, "type": q.TargetType, "revision": q.ExpectedDefinitionRevision + 1, "updated_at": now})
	if definition.Error != nil {
		return "", code("database_unavailable")
	}
	if definition.RowsAffected != 1 {
		return "", code("revision_conflict")
	}
	for i := range items {
		var frozen propertyMigrationFrozen
		_ = json.Unmarshal([]byte(items[i].ActionDetail), &frozen)
		changed := tx.Model(&WorkspacePropertySidecar{}).Where("id = ? AND owner = ? AND tenant_id = ? AND revision = ? AND values_json = ?", frozen.SidecarID, p.UserID, p.TenantID, frozen.Revision, rows[i].ValuesJSON).Updates(map[string]any{"revision": frozen.Revision + 1, "definition_revision": q.ExpectedDefinitionRevision + 1, "values_json": encoded[i], "updated_at": now})
		if changed.Error != nil {
			return "", code("database_unavailable")
		}
		if changed.RowsAffected != 1 {
			return "", code("revision_conflict")
		}
		items[i].Status, items[i].Error, items[i].ResultVersion = "done", "", items[i].VersionID
		if err = persistMigrationItem(tx, items[i], now); err != nil {
			return "", err
		}
	}
	return "completed", nil
}

func persistMigrationItem(tx *gorm.DB, item OrganizationPlanItem, now time.Time) error {
	changed := tx.Model(&OrganizationPlanItem{}).Where("id = ? AND plan_id = ? AND owner = ?", item.ID, item.PlanID, item.Owner).Updates(map[string]any{"status": item.Status, "error": item.Error, "result_version": item.ResultVersion, "updated_at": now})
	if changed.Error != nil {
		return code("database_unavailable")
	}
	if changed.RowsAffected != 1 {
		return code("revision_conflict")
	}
	return nil
}
