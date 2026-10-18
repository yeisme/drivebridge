package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RetentionHold protects one file version against purge for a stated
// purpose until a fixed expiry. A hold never promises permanent storage: it
// only blocks release while it is active, and activity requires an
// unexpired, unrevoked source the holder could manage at creation time.
type RetentionHold struct {
	ID          string     `json:"hold_id" gorm:"primaryKey"`
	Owner       string     `json:"-" gorm:"index"`
	TenantID    string     `json:"tenant_id,omitempty"`
	FileID      string     `json:"file_id" gorm:"index"`
	VersionID   string     `json:"version_id" gorm:"index"`
	Purpose     string     `json:"purpose"`
	SourceKind  string     `json:"source_kind"` // operation|share
	SourceRef   string     `json:"source_ref"`
	Key         string     `json:"-" gorm:"uniqueIndex:retention_hold_key"`
	Fingerprint string     `json:"-"`
	Revision    uint64     `json:"revision"`
	State       string     `json:"state"` // active|released
	ExpiresAt   time.Time  `json:"expires_at"`
	ReleasedAt  *time.Time `json:"released_at,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

func (RetentionHold) TableName() string { return "retention_holds" }

// RetentionPolicy is one cleanup category's configuration. New categories
// default to disabled and only a human identity may change them; agents get
// no purge tool at all.
type RetentionPolicy struct {
	ID        string    `json:"policy_id" gorm:"primaryKey"`
	Owner     string    `json:"-" gorm:"uniqueIndex:retention_policy_scope"`
	Category  string    `json:"category" gorm:"uniqueIndex:retention_policy_scope"`
	Enabled   bool      `json:"enabled" gorm:"not null;default:false"`
	KeepDays  int       `json:"keep_days"`
	Revision  uint64    `json:"revision"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (RetentionPolicy) TableName() string { return "retention_policies" }

// RetentionPlan is one preview of cleanup candidates under a frozen policy
// revision. Plans are read-only until an explicit human execution
// re-validates every item against current holds, references and permission.
type RetentionPlan struct {
	ID             string    `json:"plan_id" gorm:"primaryKey"`
	Owner          string    `json:"-" gorm:"index"`
	Category       string    `json:"category"`
	PolicyRevision uint64    `json:"policy_revision"`
	Status         string    `json:"status"` // preview|executing|completed|failed|cancelled
	ItemsTotal     int64     `json:"items_total"`
	ItemsDeleted   int64     `json:"items_deleted"`
	ItemsSkipped   int64     `json:"items_skipped"`
	ConfirmedBy    string    `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (RetentionPlan) TableName() string { return "retention_plans" }

// RetentionItem is one plan candidate with its re-validated outcome. A
// deleted item never comes back: cancelling a plan cannot resurrect a
// version the execution already removed.
type RetentionItem struct {
	ID        string    `gorm:"primaryKey"`
	PlanID    string    `json:"plan_id" gorm:"index;uniqueIndex:retention_plan_item"`
	FileID    string    `json:"file_id" gorm:"uniqueIndex:retention_plan_item"`
	VersionID string    `json:"version_id,omitempty"`
	Kind      string    `json:"kind"`  // file|version
	State     string    `json:"state"` // candidate|deleted|skipped_protected|skipped_changed|failed
	Detail    string    `json:"detail,omitempty"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

func (RetentionItem) TableName() string { return "retention_items" }

const maxRetentionHoldsPerOwner = 10000

var validRetentionPurposes = map[string]bool{"fixed_delivery": true, "legal_hold": true, "transfer_window": true}

// holdSourceValidTx resolves source validity on the caller's connection:
// inside a deletion transaction the single SQLite connection is already
// held, so the check must never open a second one.
func holdSourceValidTx(tx *gorm.DB, h RetentionHold) (bool, error) {
	switch h.SourceKind {
	case "share":
		var grant Share
		if err := tx.First(&grant, "id = ?", h.SourceRef).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, nil
			}
			return false, code("database_unavailable")
		}
		return !grant.Revoked && shareActive(grant), nil
	case "operation":
		var op Operation
		if err := tx.First(&op, "id = ?", h.SourceRef).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return false, nil
			}
			return false, code("database_unavailable")
		}
		return op.Status != "cancelled", nil
	}
	return false, code("dependency_unavailable")
}

// holdActive is the transaction-truth verdict used by every deletion path.
func holdActiveAt(h RetentionHold, now time.Time) bool {
	if h.State == "released" {
		return false
	}
	if !h.ExpiresAt.IsZero() && !now.Before(h.ExpiresAt) {
		return false
	}
	return true
}

// activeHoldForVersion returns the protecting hold for one version inside
// the caller's transaction, checking the transaction truth (not any
// projection). Source validity is part of the verdict: a protection whose
// authorizing delivery was revoked no longer blocks cleanup.
func activeHoldForVersion(tx *gorm.DB, fileID, versionID string, now time.Time) (*RetentionHold, error) {
	var holds []RetentionHold
	if e := tx.Where("file_id = ? AND (version_id = ? OR version_id = '')", fileID, versionID).Find(&holds).Error; e != nil {
		return nil, code("database_unavailable")
	}
	for i := range holds {
		if !holdActiveAt(holds[i], now) {
			continue
		}
		valid, err := holdSourceValidTx(tx, holds[i])
		if err != nil {
			return nil, err
		}
		if valid {
			return &holds[i], nil
		}
	}
	return nil, nil
}

// RetentionHoldRequest is the creation input; its canonical JSON form is the
// fingerprint basis shared with the idempotent replay path.
type RetentionHoldRequest struct {
	FileID    string `json:"file_id"`
	VersionID string `json:"version_id"`
	Purpose   string `json:"purpose"`
	SourceRef string `json:"source_ref"`
	ExpiresAt string `json:"expires_at"`
}

// RetentionHoldView is the read model shared by HTTP, MCP and SDK.
type RetentionHoldView struct {
	HoldID     string `json:"hold_id"`
	Revision   uint64 `json:"revision"`
	State      string `json:"state"`
	FileID     string `json:"file_id"`
	VersionID  string `json:"version_id"`
	Purpose    string `json:"purpose"`
	SourceKind string `json:"source_kind"`
	SourceRef  string `json:"source_ref"`
	ExpiresAt  string `json:"expires_at"`
}

func retentionHoldView(h RetentionHold, active bool) RetentionHoldView {
	state := h.State
	if state == "active" && !active {
		state = "inactive_source_or_expiry"
	}
	return RetentionHoldView{HoldID: h.ID, Revision: h.Revision, State: state, FileID: h.FileID, VersionID: h.VersionID, Purpose: h.Purpose, SourceKind: h.SourceKind, SourceRef: h.SourceRef, ExpiresAt: h.ExpiresAt.UTC().Format(time.RFC3339)}
}

// CreateRetentionHold protects one version under a durable idempotency key.
// Creation and purge share the writer serialization and the transaction
// truth, so a concurrent pair always resolves to exactly one winner.
func (s *Service) CreateRetentionHold(p Principal, q RetentionHoldRequest, key string) (RetentionHoldView, error) {
	if key == "" || len(key) > 200 || !validRetentionPurposes[q.Purpose] || q.FileID == "" || q.SourceRef == "" {
		return RetentionHoldView{}, code("invalid_request")
	}
	expiresAt, err := holdExpiry(q.ExpiresAt)
	if err != nil {
		return RetentionHoldView{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out RetentionHoldView
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := s.retentionActorOn(tx, p); err != nil {
			return err
		}
		var h RetentionHold
		err := tx.Where("owner = ? AND key = ?", p.dataOwner(), key).First(&h).Error
		if err == nil {
			// Omission means the version pinned by this original request, not
			// whatever head happens to exist when its response is recovered.
			if h.TenantID != p.TenantID {
				return code("not_found")
			}
			f, err := s.holdFileOn(tx, p, h.FileID, h.VersionID, false)
			if err != nil {
				return err
			}
			if _, err = s.holdSourceOn(tx, p, f, h.VersionID, h.SourceKind, h.SourceRef, false); err != nil {
				return err
			}
			if q.VersionID == "" {
				q.VersionID = h.VersionID
			}
			body, _ := json.Marshal(q)
			if h.Fingerprint != Hash(string(body)) {
				return code("idempotency_conflict")
			}
		} else {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return code("database_unavailable")
			}
			f, err := s.holdFileOn(tx, p, q.FileID, q.VersionID, false)
			if err != nil {
				return err
			}
			if q.VersionID == "" {
				q.VersionID = f.VersionID
			}
			var v Version
			if err = tx.Where("id = ? AND file_id = ? AND owner = ?", q.VersionID, f.ID, p.dataOwner()).First(&v).Error; err != nil {
				return resourceReadError(err)
			}
			if f.PurgeStartedAt != nil {
				return code("purge_in_progress")
			}
			sourceKind, err := s.holdSourceOn(tx, p, f, q.VersionID, "", q.SourceRef, true)
			if err != nil {
				return err
			}
			body, _ := json.Marshal(q)
			var count int64
			if err = tx.Model(&RetentionHold{}).Where("owner = ?", p.dataOwner()).Count(&count).Error; err != nil {
				return code("database_unavailable")
			}
			if count >= maxRetentionHoldsPerOwner {
				return code("quota_exceeded")
			}
			now := time.Now().UTC()
			h = RetentionHold{ID: ID(), Owner: p.dataOwner(), TenantID: p.TenantID, FileID: f.ID, VersionID: q.VersionID, Purpose: q.Purpose, SourceKind: sourceKind, SourceRef: q.SourceRef, Key: key, Fingerprint: Hash(string(body)), Revision: 1, State: "active", ExpiresAt: expiresAt, CreatedAt: now, UpdatedAt: now}
			if err = tx.Create(&h).Error; err != nil {
				return code("database_unavailable")
			}
		}
		active := holdActiveAt(h, time.Now().UTC())
		if active {
			active, err = holdSourceValidTx(tx, h)
			if err != nil {
				return err
			}
		}
		out = retentionHoldView(h, active)
		return nil
	})
	if err != nil {
		return RetentionHoldView{}, err
	}
	return out, nil
}

// GetRetentionHold reads one hold with its current protection verdict.
func (s *Service) GetRetentionHold(p Principal, holdID string) (RetentionHoldView, error) {
	var out RetentionHoldView
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		h, f, err := s.holdRowOn(tx, p, holdID, false)
		if err != nil {
			return err
		}
		if _, err = s.holdSourceOn(tx, p, f, h.VersionID, h.SourceKind, h.SourceRef, false); err != nil {
			return err
		}
		active := holdActiveAt(h, time.Now().UTC())
		if active {
			active, err = holdSourceValidTx(tx, h)
			if err != nil {
				return err
			}
		}
		out = retentionHoldView(h, active)
		return nil
	})
	if err != nil {
		return RetentionHoldView{}, err
	}
	return out, nil
}

// ReleaseRetentionHold retains the legacy idempotent signature. New callers
// use ReleaseWorkspaceHold for expected-revision CAS and immutable receipts.
func (s *Service) ReleaseRetentionHold(p Principal, holdID string) (RetentionHoldView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out RetentionHoldView
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		h, f, err := s.holdRowOn(tx, p, holdID, true)
		if err != nil {
			return err
		}
		if _, err = s.holdSourceOn(tx, p, f, h.VersionID, h.SourceKind, h.SourceRef, false); err != nil {
			return err
		}
		// Workspace provenance is atomic with creation. Its holds must never
		// fall back to an ungated, non-CAS legacy release entry point.
		var managed int64
		if err = tx.Model(&WorkspaceRequest{}).Where("owner = ? AND tenant_id = ? AND resource_id = ? AND action = ?", h.Owner, h.TenantID, h.ID, "hold.create").Count(&managed).Error; err != nil {
			return code("database_unavailable")
		}
		if managed != 0 {
			return code("unsupported_action")
		}
		if h.State != "released" {
			now := time.Now().UTC()
			result := tx.Model(&RetentionHold{}).Where("id = ? AND revision = ? AND state = ?", h.ID, h.Revision, h.State).Updates(map[string]any{"state": "released", "revision": h.Revision + 1, "released_at": now, "updated_at": now})
			if result.Error != nil {
				return code("database_unavailable")
			}
			if result.RowsAffected != 1 {
				return code("revision_conflict")
			}
			h.State, h.Revision = "released", h.Revision+1
		}
		out = retentionHoldView(h, false)
		return nil
	})
	if err != nil {
		return RetentionHoldView{}, err
	}
	return out, nil
}

// PreviewRetention persists a plan without mutating cleanup candidates for one
// category under the current policy revision. Trash candidates are trashed
// files past their retention window; historical versions are superseded
// versions older than the keep window that are not the live version.
func (s *Service) PreviewRetention(p Principal, category string) (map[string]any, error) {
	// Preview persists metadata and is agent-visible; only execution purges
	// and stays human-only.
	if category != "trash" && category != "historical_versions" {
		return nil, code("invalid_request")
	}
	var result map[string]any
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		result, err = s.previewRetentionOn(tx, p, category)
		return err
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (s *Service) previewRetentionOn(tx *gorm.DB, p Principal, category string) (map[string]any, error) {
	if err := s.retentionActorOn(tx, p); err != nil {
		return nil, err
	}
	var policy RetentionPolicy
	if e := tx.First(&policy, "owner = ? AND category = ?", p.dataOwner(), category).Error; e != nil {
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return nil, code("database_unavailable")
		}
		policy = RetentionPolicyPolicyDefaults(category)
	}
	now := time.Now().UTC()
	type candidate struct {
		FileID    string
		VersionID string
		Reason    string
	}
	candidates := []candidate{}
	if category == "trash" {
		var files []File
		if err := tx.Where("owner = ? AND trashed = ?", p.dataOwner(), true).Find(&files).Error; err != nil {
			return nil, code("database_unavailable")
		}
		for _, f := range files {
			days := policy.KeepDays
			if days <= 0 {
				var err error
				days, err = trashDaysFor(tx, f)
				if err != nil {
					return nil, err
				}
			}
			if days <= 0 || now.After(f.TrashedAt.Add(time.Duration(days)*24*time.Hour)) {
				if err := s.retentionFileAccessOn(tx, p, f, "read"); err != nil {
					return nil, err
				}
				candidates = append(candidates, candidate{FileID: f.ID, Reason: "trash_retention_elapsed"})
			}
		}
	} else {
		keep := policy.KeepDays
		if keep <= 0 {
			keep = 90
		}
		var files []File
		if err := tx.Where("owner = ?", p.dataOwner()).Find(&files).Error; err != nil {
			return nil, code("database_unavailable")
		}
		for _, f := range files {
			var versions []Version
			if err := tx.Where("file_id = ? AND owner = ?", f.ID, p.dataOwner()).Order("created DESC").Find(&versions).Error; err != nil {
				return nil, code("database_unavailable")
			}
			for i, v := range versions {
				if i == 0 {
					continue // the live version is never a historical candidate
				}
				if now.After(v.Created.Add(time.Duration(keep) * 24 * time.Hour)) {
					if err := s.retentionFileAccessOn(tx, p, f, "read"); err != nil {
						return nil, err
					}
					candidates = append(candidates, candidate{FileID: f.ID, VersionID: v.ID, Reason: "historical_keep_window_elapsed"})
				}
			}
		}
	}
	// The plan and its items are one preview snapshot; execution re-validates.
	plan := RetentionPlan{ID: ID(), Owner: p.dataOwner(), Category: category, PolicyRevision: policy.Revision, Status: "preview", ItemsTotal: int64(len(candidates)), CreatedAt: now, UpdatedAt: now}
	if e := tx.Create(&plan).Error; e != nil {
		return nil, code("database_unavailable")
	}
	items := []map[string]any{}
	for _, c := range candidates {
		item := RetentionItem{ID: ID(), PlanID: plan.ID, FileID: c.FileID, VersionID: c.VersionID, Kind: "version", State: "candidate", Detail: c.Reason, CreatedAt: now, UpdatedAt: now}
		if c.VersionID == "" {
			item.Kind = "file"
		}
		if e := tx.Create(&item).Error; e != nil {
			return nil, code("database_unavailable")
		}
		items = append(items, map[string]any{"file_id": item.FileID, "version_id": item.VersionID, "kind": item.Kind, "state": item.State, "detail": item.Detail})
	}
	return map[string]any{"plan_id": plan.ID, "category": category, "policy_revision": policy.Revision, "status": plan.Status, "items_total": plan.ItemsTotal, "items": items, "note": "preview only; execution re-validates holds, references and permissions"}, nil
}

func RetentionPolicyPolicyDefaults(category string) RetentionPolicy {
	return RetentionPolicy{Owner: "", Category: category, Enabled: false, KeepDays: 0, Revision: 0}
}

// ExecuteRetention runs a previewed plan after re-validating every item
// against the transaction truth. Human identity only: agents never purge.
func (s *Service) ExecuteRetention(ctx context.Context, p Principal, planID string) (map[string]any, error) {
	if p.Agent != nil {
		return nil, code("forbidden")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var plan RetentionPlan
	var items []RetentionItem
	// Admission is a read-only snapshot. No candidate is dispatched until
	// the complete plan and current policy have both been read successfully.
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.retentionHumanOn(tx, p); err != nil {
			return err
		}
		if err := tx.First(&plan, "id = ? AND owner = ?", planID, p.dataOwner()).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return code("not_found")
			}
			return code("database_unavailable")
		}
		if err := retentionPlanCurrentOn(tx, plan); err != nil {
			return err
		}
		if err := tx.Where("plan_id = ?", plan.ID).Order("id").Find(&items).Error; err != nil {
			return code("database_unavailable")
		}
		if plan.ItemsTotal < 0 || int64(len(items)) != plan.ItemsTotal {
			return code("dependency_unavailable")
		}
		return nil
	})
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}
	if err != nil {
		return nil, err
	}
	for _, item := range items {
		if err := ctx.Err(); err != nil {
			return nil, err
		}
		if err := s.executeRetentionItem(ctx, p, item); err != nil {
			return nil, err
		}
	}
	var result map[string]any
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.retentionHumanOn(tx, p); err != nil {
			return err
		}
		var fresh RetentionPlan
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&fresh, "id = ? AND owner = ?", plan.ID, p.dataOwner()).Error; err != nil {
			return code("database_unavailable")
		}
		if err := retentionPlanCurrentOn(tx, fresh); err != nil {
			return err
		}
		var stored []RetentionItem
		if err := tx.Where("plan_id = ?", plan.ID).Find(&stored).Error; err != nil {
			return code("database_unavailable")
		}
		if int64(len(stored)) != fresh.ItemsTotal {
			return code("dependency_unavailable")
		}
		deleted, skipped := int64(0), int64(0)
		status := "completed"
		for _, item := range stored {
			switch item.State {
			case "deleted":
				deleted++
			case "skipped_protected", "skipped_changed":
				skipped++
			case "failed":
				skipped++
				status = "failed"
			default:
				return code("dependency_unavailable")
			}
		}
		if fresh.Status != status || fresh.ItemsDeleted != deleted || fresh.ItemsSkipped != skipped {
			out := tx.Model(&RetentionPlan{}).Where("id = ? AND owner = ?", plan.ID, p.dataOwner()).Updates(map[string]any{"status": status, "items_deleted": deleted, "items_skipped": skipped, "updated_at": time.Now().UTC()})
			if out.Error != nil {
				return code("database_unavailable")
			}
			if out.RowsAffected != 1 {
				return code("dependency_unavailable")
			}
		}
		result = map[string]any{"plan_id": plan.ID, "status": status, "items_deleted": deleted, "items_skipped": skipped}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return result, nil
}

// trashDaysFor resolves the effective trash retention window of one file
// from its space policy; zero means the default window.
func trashDaysFor(tx *gorm.DB, f File) (int, error) {
	var sp Space
	if err := tx.First(&sp, "id = ? AND owner = ?", f.SpaceID, f.Owner).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return 0, code("dependency_unavailable")
		}
		return 0, code("database_unavailable")
	}
	if sp.TrashRetentionDays > 0 {
		return sp.TrashRetentionDays, nil
	}
	return defaultTrashRetentionDays, nil
}
