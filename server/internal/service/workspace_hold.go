package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func workspaceHoldReceipt(row WorkspaceRequest, fileID string) contract.WorkspaceHoldReceipt {
	action := "create"
	if row.Action == "hold.release" {
		action = "release"
	}
	return contract.WorkspaceHoldReceipt{HoldID: row.ResourceID, FileID: fileID, Action: action, ResultRevision: row.ResultRevision, RequestDigest: row.RequestDigest, CommittedAt: row.CreatedAt}
}

func holdExpiry(value string) (time.Time, error) {
	if value == "" {
		return time.Time{}, nil
	}
	t, err := time.Parse(time.RFC3339, value)
	if err != nil {
		return time.Time{}, code("invalid_request")
	}
	return t.UTC(), nil
}

func holdRequestOn(tx *gorm.DB, p Principal, action, key string) (WorkspaceRequest, error) {
	var row WorkspaceRequest
	err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.dataOwner(), p.TenantID, p.UserID, action, key).First(&row).Error
	return row, err
}

func (s *Service) CreateWorkspaceHold(ctx context.Context, p Principal, fileID string, q contract.WorkspaceHoldCreate, key string) (contract.WorkspaceHoldReceipt, error) {
	var out contract.WorkspaceHoldReceipt
	if !s.Config.HoldManagement {
		return out, code("unsupported_action")
	}
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	if !propertyHexID(fileID) || !propertyHexID(q.VersionID) || !propertyHexID(q.SourceRef) || (q.SourceKind != "share" && q.SourceKind != "operation") || !validRetentionPurposes[q.Purpose] {
		return out, code("invalid_request")
	}
	expires, err := holdExpiry(q.ExpiresAt)
	if err != nil {
		return out, err
	}
	if q.ExpiresAt != "" {
		q.ExpiresAt = expires.Format(time.RFC3339)
	}
	raw, _ := json.Marshal(struct {
		FileID  string
		Request contract.WorkspaceHoldCreate
	}{fileID, q})
	digest := Hash(string(raw))
	s.mu.Lock()
	defer s.mu.Unlock()
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		f, err := s.holdFileOn(tx, p, fileID, q.VersionID, true)
		if err != nil {
			return err
		}
		if _, err = s.holdSourceOn(tx, p, f, q.VersionID, q.SourceKind, q.SourceRef, false); err != nil {
			return err
		}
		receipt, err := holdRequestOn(tx, p, "hold.create", key)
		if err == nil {
			if receipt.RequestDigest != digest {
				return code("idempotency_conflict")
			}
			h, _, err := s.holdRowOn(tx, p, receipt.ResourceID, true)
			if err != nil {
				return err
			}
			if h.FileID != fileID {
				return code("idempotency_conflict")
			}
			out = workspaceHoldReceipt(receipt, fileID)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		if f.PurgeStartedAt != nil {
			return code("purge_in_progress")
		}
		if _, err = s.holdSourceOn(tx, p, f, q.VersionID, q.SourceKind, q.SourceRef, true); err != nil {
			return err
		}
		if !expires.IsZero() && !expires.After(time.Now()) {
			return code("invalid_request")
		}
		var count int64
		if err = tx.Model(&RetentionHold{}).Where("owner = ?", p.dataOwner()).Count(&count).Error; err != nil {
			return code("database_unavailable")
		}
		if count >= maxRetentionHoldsPerOwner {
			return code("quota_exceeded")
		}
		now := time.Now().UTC()
		h := RetentionHold{ID: ID(), Owner: p.dataOwner(), TenantID: p.TenantID, FileID: fileID, VersionID: q.VersionID, Purpose: q.Purpose, SourceKind: q.SourceKind, SourceRef: q.SourceRef, Key: "workspace:" + ID(), Fingerprint: digest, Revision: 1, State: "active", ExpiresAt: expires, CreatedAt: now, UpdatedAt: now}
		if err = tx.Create(&h).Error; err != nil {
			return code("database_unavailable")
		}
		receipt = WorkspaceRequest{ID: ID(), Owner: p.dataOwner(), TenantID: p.TenantID, ActorID: p.UserID, Action: "hold.create", RequestKey: key, RequestDigest: digest, ResourceID: h.ID, ResultRevision: 1, CreatedAt: now}
		if err = tx.Create(&receipt).Error; err != nil {
			return code("database_unavailable")
		}
		out = workspaceHoldReceipt(receipt, fileID)
		return nil
	})
	if err != nil {
		return contract.WorkspaceHoldReceipt{}, err
	}
	return out, nil
}

func (s *Service) ReleaseWorkspaceHold(ctx context.Context, p Principal, id string, q contract.WorkspaceHoldRelease, key string) (contract.WorkspaceHoldReceipt, error) {
	var out contract.WorkspaceHoldReceipt
	if !s.Config.HoldManagement {
		return out, code("unsupported_action")
	}
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	if !propertyHexID(id) || q.ExpectedRevision == 0 || q.ExpectedRevision >= 9007199254740991 {
		return out, code("invalid_request")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		h, f, err := s.holdRowOn(tx, p, id, true)
		if err != nil {
			return err
		}
		if _, err = s.holdSourceOn(tx, p, f, h.VersionID, h.SourceKind, h.SourceRef, false); err != nil {
			return err
		}
		raw, _ := json.Marshal(struct {
			FileID  string
			HoldID  string
			Request contract.WorkspaceHoldRelease
		}{f.ID, id, q})
		digest := Hash(string(raw))
		receipt, err := holdRequestOn(tx, p, "hold.release", key)
		if err == nil {
			if receipt.RequestDigest != digest || receipt.ResourceID != id {
				return code("idempotency_conflict")
			}
			out = workspaceHoldReceipt(receipt, f.ID)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		if h.Revision != q.ExpectedRevision || h.State != "active" {
			return code("revision_conflict")
		}
		now := time.Now().UTC()
		result := tx.Model(&RetentionHold{}).Where("id = ? AND owner = ? AND tenant_id = ? AND revision = ? AND state = ?", id, p.dataOwner(), p.TenantID, q.ExpectedRevision, "active").Updates(map[string]any{"state": "released", "revision": q.ExpectedRevision + 1, "released_at": now, "updated_at": now})
		if result.Error != nil {
			return code("database_unavailable")
		}
		if result.RowsAffected != 1 {
			return code("revision_conflict")
		}
		receipt = WorkspaceRequest{ID: ID(), Owner: p.dataOwner(), TenantID: p.TenantID, ActorID: p.UserID, Action: "hold.release", RequestKey: key, RequestDigest: digest, ResourceID: id, ResultRevision: q.ExpectedRevision + 1, CreatedAt: now}
		if err = tx.Create(&receipt).Error; err != nil {
			return code("database_unavailable")
		}
		out = workspaceHoldReceipt(receipt, f.ID)
		return nil
	})
	if err != nil {
		return contract.WorkspaceHoldReceipt{}, err
	}
	return out, nil
}

func (s *Service) LookupWorkspaceHold(ctx context.Context, p Principal, fileID string, q contract.WorkspaceHoldLookup) (contract.WorkspaceHoldReceipt, error) {
	var out contract.WorkspaceHoldReceipt
	if err := collectionChangeKey(q.IdempotencyKey); err != nil {
		return out, err
	}
	if q.Action != "create" && q.Action != "release" {
		return out, code("invalid_request")
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := s.holdFileOn(tx, p, fileID, "", true); err != nil {
			return err
		}
		receipt, err := holdRequestOn(tx, p, "hold."+q.Action, q.IdempotencyKey)
		if err != nil {
			return resourceReadError(err)
		}
		h, f, err := s.holdRowOn(tx, p, receipt.ResourceID, true)
		if err != nil {
			return err
		}
		if h.FileID != fileID {
			return code("not_found")
		}
		if _, err = s.holdSourceOn(tx, p, f, h.VersionID, h.SourceKind, h.SourceRef, false); err != nil {
			return err
		}
		out = workspaceHoldReceipt(receipt, fileID)
		return nil
	})
	if err != nil {
		return contract.WorkspaceHoldReceipt{}, err
	}
	return out, nil
}

func (s *Service) WorkspaceHolds(ctx context.Context, p Principal, fileID, versionID string, page contract.PageQuery) (contract.WorkspaceHoldList, error) {
	out := contract.WorkspaceHoldList{Items: []contract.WorkspaceHoldView{}, ObservedAt: time.Now().UTC(), TotalKind: "unknown", ManagementEnabled: s.Config.HoldManagement}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		f, err := s.holdFileOn(tx, p, fileID, versionID, true)
		if err != nil {
			return err
		}
		query := tx.Where("owner = ? AND tenant_id = ? AND file_id = ?", p.dataOwner(), p.TenantID, fileID)
		if versionID != "" {
			query = query.Where("version_id = ?", versionID)
		}
		var rows []RetentionHold
		if err = query.Order("id").Limit(maxRetentionHoldsPerOwner + 1).Find(&rows).Error; err != nil {
			return code("database_unavailable")
		}
		if len(rows) > maxRetentionHoldsPerOwner {
			return code("query_budget_exceeded")
		}
		keys := make([]string, 0, len(rows))
		for _, row := range rows {
			var version Version
			if err = tx.Where("id = ? AND file_id = ? AND owner = ?", row.VersionID, f.ID, f.Owner).First(&version).Error; err != nil {
				return resourceReadError(err)
			}
			if _, err = s.holdSourceOn(tx, p, f, row.VersionID, row.SourceKind, row.SourceRef, false); err != nil {
				return err
			}
			active := holdActiveAt(row, out.ObservedAt)
			if active {
				active, err = holdSourceValidTx(tx, row)
				if err != nil {
					return err
				}
			}
			view := retentionHoldView(row, active)
			actions := []string{}
			if out.ManagementEnabled && row.State == "active" && row.Revision > 0 && row.Revision < 9007199254740991 {
				actions = append(actions, "release")
			}
			out.Items = append(out.Items, contract.WorkspaceHoldView{HoldID: view.HoldID, Revision: view.Revision, State: view.State, FileID: view.FileID, VersionID: view.VersionID, Purpose: view.Purpose, SourceKind: view.SourceKind, SourceRef: view.SourceRef, ExpiresAt: view.ExpiresAt, AllowedActions: actions})
			keys = append(keys, row.ID)
		}
		// Bind current file identity/path/head as well as visible hold states.
		start, end, next, err := s.workspaceMetadataPage(p, "workspace.holds", []string{fileID, versionID, f.SpaceID, f.Path, f.VersionID}, page, out.Items, keys)
		if err != nil {
			return err
		}
		out.Items, out.NextCursor = out.Items[start:end], next
		return nil
	})
	if err != nil {
		return contract.WorkspaceHoldList{}, err
	}
	return out, nil
}
