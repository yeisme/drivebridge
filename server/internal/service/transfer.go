package service

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Cross-storage transfer plans: copy executes per-item against the original
// archive contract; source cleanup is a separate human authorization that
// only ever soft-deletes sources still matching the pinned version.

// transferGate is the deployment switch for the transfer surface.
func (s *Service) transferGate() error {
	if !s.Config.TransferPlans {
		return code("unsupported")
	}
	return nil
}

// TransferSourceInput is one fixed source reference with its digest.
type TransferSourceInput struct {
	Ref    string `json:"ref"`
	SHA256 string `json:"sha256,omitempty"`
}

// CreateTransferPlanRequest freezes one transfer intent.
type CreateTransferPlanRequest struct {
	Sources             []TransferSourceInput `json:"sources"`
	TargetSpaceID       string                `json:"target_space_id"`
	TargetPath          string                `json:"target_path"`
	Mode                string                `json:"mode"`
	ConflictPolicy      string                `json:"conflict_policy"`
	CrossSpaceConfirmed bool                  `json:"cross_space_confirmed,omitempty"`
}

// TransferPlanView is the plan read model shared by HTTP, MCP and SDK.
type TransferPlanView struct {
	PlanID                  string             `json:"plan_id"`
	Mode                    string             `json:"mode"`
	TargetSpaceID           string             `json:"target_space_id"`
	TargetPath              string             `json:"target_path"`
	ConflictPolicy          string             `json:"conflict_policy"`
	Revision                uint64             `json:"revision"`
	Phase                   string             `json:"phase"`
	OperationID             string             `json:"operation_id,omitempty"`
	ItemsTotal              int64              `json:"items_total"`
	ItemsCopied             int64              `json:"items_copied"`
	ItemsPending            int64              `json:"items_pending"`
	ItemsPaused             int64              `json:"items_paused"`
	ItemsFailed             int64              `json:"items_failed"`
	SourceCleanupAuthorized bool               `json:"source_cleanup_authorized"`
	NextCursor              string             `json:"next_cursor,omitempty"`
	Items                   []TransferItemView `json:"items,omitempty"`
}

// TransferItemView is one item's durable outcome.
type TransferItemView struct {
	ItemKey         string `json:"item_key"`
	SourceFileID    string `json:"source_file_id"`
	SourceVersionID string `json:"source_version_id"`
	SourceSHA256    string `json:"source_sha256,omitempty"`
	TargetPath      string `json:"target_path"`
	CopyOperationID string `json:"copy_operation_id,omitempty"`
	TargetRef       string `json:"target_ref,omitempty"`
	TargetSHA256    string `json:"target_sha256,omitempty"`
	Status          string `json:"status"`
	ResultCertainty string `json:"result_certainty,omitempty"`
	Error           string `json:"error,omitempty"`
	CleanupState    string `json:"cleanup_state,omitempty"`
}

// CreateTransferPlan validates and freezes the intent under a durable
// idempotency key. Cross-space egress needs its own confirmation: plain
// target write is not enough.
func (s *Service) CreateTransferPlan(p Principal, q CreateTransferPlanRequest, key string) (TransferPlanView, error) {
	if e := s.transferGate(); e != nil {
		return TransferPlanView{}, e
	}
	if key == "" || len(key) > 200 || len(q.Sources) == 0 || len(q.Sources) > 500 {
		return TransferPlanView{}, code("invalid_request")
	}
	if q.Mode == "" {
		q.Mode = "copy"
	}
	if q.ConflictPolicy == "" {
		q.ConflictPolicy = "fail"
	}
	// Overwrite needs a per-target expected_version plus human
	// confirmation; the plan model pins sources, not target versions, so
	// the vocabulary here is fail (default) and keep_both. Occupied
	// targets under fail surface as target_conflict.
	if (q.Mode != "copy" && q.Mode != "migrate") || (q.ConflictPolicy != "fail" && q.ConflictPolicy != "keep_both") {
		return TransferPlanView{}, code("invalid_request")
	}
	if _, e := s.space(p, q.TargetSpaceID); e != nil {
		return TransferPlanView{}, e
	}
	if e := s.allow(p, q.TargetSpaceID, q.TargetPath, "write"); e != nil {
		return TransferPlanView{}, e
	}
	// Resolve every source now: the plan pins exact versions and digests.
	type resolved struct {
		fileID, versionID, sha, space string
		itemKey, targetPath           string
	}
	var sources []resolved
	crossSpace := false
	for _, src := range q.Sources {
		fileID, versionID, e := s.resolveRef(src.Ref)
		if e != nil {
			return TransferPlanView{}, e
		}
		f, e := s.file(p, fileID, "read")
		if e != nil {
			return TransferPlanView{}, e
		}
		if f.SpaceID != q.TargetSpaceID {
			crossSpace = true
		}
		var v Version
		if s.DB.First(&v, "id = ? AND file_id = ?", versionID, fileID).Error != nil {
			return TransferPlanView{}, code("not_found")
		}
		if src.SHA256 != "" && src.SHA256 != v.SHA256 {
			return TransferPlanView{}, code("source_changed")
		}
		// Each item lands under the target directory with its own source
		// base name; one shared path would turn siblings into renames.
		base := f.Path
		if idx := strings.LastIndex(base, "/"); idx >= 0 {
			base = base[idx+1:]
		}
		target := q.TargetPath + "/" + base
		sources = append(sources, resolved{fileID: fileID, versionID: versionID, sha: v.SHA256, space: f.SpaceID, itemKey: fileID + "\x00" + versionID, targetPath: target})
	}
	if crossSpace && !q.CrossSpaceConfirmed {
		// Cross-space egress is its own authorization decision.
		return TransferPlanView{}, code("forbidden")
	}
	// The fingerprint normalization lives in one helper shared with
	// LookupOperation: recovery recomputes exactly what creation computed.
	fingerprint := transferPlanFingerprint(q)
	s.mu.Lock()
	defer s.mu.Unlock()
	var plan TransferPlan
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&plan, "owner = ? AND key = ?", p.dataOwner(), key).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		now := time.Now().UTC()
		plan = TransferPlan{ID: ID(), Owner: p.dataOwner(), TenantID: p.TenantID, Mode: q.Mode, TargetSpaceID: q.TargetSpaceID, TargetPath: q.TargetPath, ConflictPolicy: q.ConflictPolicy, Revision: 1, Phase: "planned", ItemsTotal: int64(len(sources)), CrossSpaceConfirmed: q.CrossSpaceConfirmed, Key: key, Fingerprint: fingerprint, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&plan).Error; err != nil {
			return code("database_unavailable")
		}
		for _, src := range sources {
			item := TransferItem{ID: ID(), PlanID: plan.ID, ItemKey: src.itemKey, Owner: p.dataOwner(), SourceFileID: src.fileID, SourceVersionID: src.versionID, SourceSHA256: src.sha, TargetPath: src.targetPath, Status: "pending", CleanupState: "none", CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&item).Error; err != nil {
				return code("database_unavailable")
			}
		}
		return nil
	})
	if e != nil {
		return TransferPlanView{}, e
	}
	if plan.Fingerprint != fingerprint {
		return TransferPlanView{}, code("idempotency_conflict")
	}
	return s.TransferPlanViewFor(p, plan, "", 0)
}

// transferPlanFingerprint is the canonical normalization of one transfer
// plan creation: the single definition shared by CreateTransferPlan and
// LookupOperation so a lost creation response replays onto the same plan.
func transferPlanFingerprint(q CreateTransferPlanRequest) string {
	fingerprintSource := struct {
		Mode    string                `json:"mode"`
		Target  string                `json:"target"`
		Path    string                `json:"path"`
		Policy  string                `json:"policy"`
		Sources []TransferSourceInput `json:"sources"`
		Cross   bool                  `json:"cross_space_confirmed"`
	}{q.Mode, q.TargetSpaceID, q.TargetPath, q.ConflictPolicy, q.Sources, q.CrossSpaceConfirmed}
	body, _ := json.Marshal(fingerprintSource)
	return Hash(string(body))
}

// TransferPlanViewFor builds the paged read model.
func (s *Service) TransferPlanViewFor(p Principal, plan TransferPlan, cursor string, limit int) (TransferPlanView, error) {
	limit = orgPageLimit(limit)
	var items []TransferItem
	tx := s.DB.Where("plan_id = ?", plan.ID).Order("created_at, id")
	if cursor != "" {
		tx = tx.Where("id > ?", cursor)
	}
	if limit > 0 {
		tx = tx.Limit(limit + 1)
	}
	if e := tx.Find(&items).Error; e != nil {
		return TransferPlanView{}, code("database_unavailable")
	}
	view := TransferPlanView{PlanID: plan.ID, Mode: plan.Mode, TargetSpaceID: plan.TargetSpaceID, TargetPath: plan.TargetPath, ConflictPolicy: plan.ConflictPolicy, Revision: plan.Revision, Phase: plan.Phase, OperationID: plan.OperationID, ItemsTotal: plan.ItemsTotal, SourceCleanupAuthorized: plan.CleanupAuthorized, Items: []TransferItemView{}}
	// Counters are plan-level facts: they summarize every item, not just
	// the page in view.
	s.DB.Model(&TransferItem{}).Where("plan_id = ? AND status IN ?", plan.ID, []string{"copied", "source_trashed"}).Count(&view.ItemsCopied)
	s.DB.Model(&TransferItem{}).Where("plan_id = ? AND status IN ?", plan.ID, []string{"pending", "copying"}).Count(&view.ItemsPending)
	s.DB.Model(&TransferItem{}).Where("plan_id = ? AND status = ?", plan.ID, "paused").Count(&view.ItemsPaused)
	s.DB.Model(&TransferItem{}).Where("plan_id = ? AND status = ?", plan.ID, "failed").Count(&view.ItemsFailed)
	more := limit > 0 && len(items) > limit
	if more {
		items = items[:limit]
	}
	for _, item := range items {
		iv := TransferItemView{ItemKey: item.ItemKey, SourceFileID: item.SourceFileID, SourceVersionID: item.SourceVersionID, SourceSHA256: item.SourceSHA256, TargetPath: item.TargetPath, CopyOperationID: item.CopyOperationID, TargetSHA256: item.TargetSHA256, Status: item.Status, ResultCertainty: item.ResultCertainty, Error: item.Error, CleanupState: item.CleanupState}
		if item.TargetFileID != "" && item.TargetVersionID != "" {
			iv.TargetRef = "drivebridge://" + s.Instance + "/file/" + item.TargetFileID + "/version/" + item.TargetVersionID
		}
		view.Items = append(view.Items, iv)
	}
	if more && len(items) > 0 {
		view.NextCursor = items[len(items)-1].ID
	}
	return view, nil
}

// GetTransferPlan reads one plan with a paged item impact preview.
func (s *Service) GetTransferPlan(p Principal, planID, cursor string, limit int) (TransferPlanView, error) {
	if e := s.transferGate(); e != nil {
		return TransferPlanView{}, e
	}
	var plan TransferPlan
	if s.DB.First(&plan, "id = ? AND owner = ?", planID, p.dataOwner()).Error != nil {
		return TransferPlanView{}, code("not_found")
	}
	return s.TransferPlanViewFor(p, plan, cursor, limit)
}

// ExecuteTransferPlanRequest carries the optimistic plan revision.
type ExecuteTransferPlanRequest struct {
	ExpectedRevision uint64 `json:"expected_revision"`
}

// ExecuteTransferPlan copies every pending item through the original
// archive contract under the plan anchor. Per-item idempotency replays on
// the item's original operation; a drifted source pauses only that item
// and keeps every completed result.
func (s *Service) ExecuteTransferPlan(ctx context.Context, p Principal, planID string, q ExecuteTransferPlanRequest) (TransferPlanView, error) {
	if e := s.transferGate(); e != nil {
		return TransferPlanView{}, e
	}
	var plan TransferPlan
	if s.DB.First(&plan, "id = ? AND owner = ?", planID, p.dataOwner()).Error != nil {
		return TransferPlanView{}, code("not_found")
	}
	if plan.Phase == "cancelled" {
		return TransferPlanView{}, code("operation_closed")
	}
	if q.ExpectedRevision != plan.Revision {
		return TransferPlanView{}, code("plan_stale")
	}
	if plan.OperationID == "" {
		agent := ""
		if p.Agent != nil {
			agent = p.Agent.ID
		}
		// The anchor key is plan-scoped: two plans under one owner must not
		// collide on the (owner, key) unique index.
		op := Operation{ActorID: p.UserID, ID: ID(), Owner: p.dataOwner(), AgentID: agent, Kind: "transfer_plan", SpaceID: plan.TargetSpaceID, Path: plan.TargetPath, Key: "transfer-plan:" + plan.ID, Status: "running", Updated: time.Now().UTC()}
		if s.DB.Create(&op).Error != nil {
			return TransferPlanView{}, code("database_unavailable")
		}
		plan.OperationID = op.ID
		s.DB.Model(&TransferPlan{}).Where("id = ?", plan.ID).Update("operation_id", op.ID)
	}
	var items []TransferItem
	if err := s.DB.Where("plan_id = ? AND (status IN ? OR (status = ? AND error = ?))", plan.ID, []string{"pending", "copying", "paused"}, "failed", "idempotency_conflict").Order("created_at, id").Find(&items).Error; err != nil {
		return TransferPlanView{}, code("database_unavailable")
	}
	now := time.Now().UTC()
	for _, item := range items {
		// Source still on the pinned version? A drifted source pauses this
		// item only; other results stay.
		var f File
		if s.DB.First(&f, "id = ?", item.SourceFileID).Error != nil || f.VersionID != item.SourceVersionID {
			s.DB.Model(&TransferItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "paused", "error": "source_changed", "result_certainty": "unknown", "updated_at": now})
			continue
		}
		if e := s.allow(p, plan.TargetSpaceID, item.TargetPath, "write"); e != nil {
			s.DB.Model(&TransferItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "failed", "error": safeCode(e), "updated_at": now})
			continue
		}
		key, allocated, identityErr := s.transferCopyIdentity(p, plan, item)
		if identityErr != nil {
			return TransferPlanView{}, identityErr
		}
		if plan.ConflictPolicy == "fail" && allocated == nil {
			var occupied int64
			if err := s.DB.Model(&File{}).Where("space_id = ? AND path = ?", plan.TargetSpaceID, item.TargetPath).Count(&occupied).Error; err != nil {
				return TransferPlanView{}, code("database_unavailable")
			}
			if occupied > 0 {
				s.DB.Model(&TransferItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "failed", "error": "target_conflict", "updated_at": now})
				continue
			}
		}
		var op Operation
		var e error
		if allocated != nil {
			op, e = s.Resume(ctx, p, allocated.ID)
			if e == nil && op.Status == "cancelled" {
				e = code("operation_closed")
			}
		} else {
			op, e = s.ArchiveVersion(ctx, p, ArchiveRequest{SourceFileID: item.SourceFileID, SourceVersionID: item.SourceVersionID, TargetSpaceID: plan.TargetSpaceID, TargetPath: item.TargetPath, ExpectedSourceVersion: item.SourceVersionID}, key)
		}
		if e != nil {
			mapped := safeCode(e)
			if mapped == "name_conflict" {
				mapped = "target_conflict"
			}
			s.DB.Model(&TransferItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "failed", "error": mapped, "updated_at": now})
			continue
		}
		if op.Status != "completed" {
			// The original archive operation answers recovery; the item
			// keeps its certainty unknown instead of guessing.
			s.DB.Model(&TransferItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "copying", "copy_operation_id": op.ID, "result_certainty": "unknown", "updated_at": now})
			continue
		}
		// Verify the published target digest before claiming the copy.
		if item.SourceSHA256 != "" && op.SHA256 != item.SourceSHA256 {
			s.DB.Model(&TransferItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "failed", "error": "content_mismatch", "copy_operation_id": op.ID, "result_certainty": "unknown", "updated_at": now})
			continue
		}
		s.DB.Model(&TransferItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "copied", "copy_operation_id": op.ID, "target_file_id": op.FileID, "target_version_id": op.VersionID, "target_sha256": op.SHA256, "result_certainty": "confirmed", "error": "", "updated_at": now})
	}
	// Phase follows the durable item facts.
	var pending, failed int64
	s.DB.Model(&TransferItem{}).Where("plan_id = ? AND status IN ?", plan.ID, []string{"pending", "copying", "paused"}).Count(&pending)
	s.DB.Model(&TransferItem{}).Where("plan_id = ? AND status = ?", plan.ID, "failed").Count(&failed)
	phase := "copied"
	if pending > 0 || failed > 0 {
		phase = "partial"
	}
	if plan.Mode == "migrate" && phase == "copied" {
		phase = "source_cleanup_pending"
	}
	s.DB.Model(&TransferPlan{}).Where("id = ?", plan.ID).Updates(map[string]any{"phase": phase, "revision": gorm.Expr("revision + 1"), "updated_at": now})
	if plan.OperationID != "" && phase == "copied" {
		s.DB.Model(&Operation{}).Where("id = ?", plan.OperationID).Updates(map[string]any{"status": "completed", "updated": now})
	}
	var fresh TransferPlan
	s.DB.First(&fresh, "id = ?", plan.ID)
	return s.TransferPlanViewFor(p, fresh, "", 0)
}

// CleanupTransferSource is the separate human authorization: it only soft-
// deletes sources that still match the pinned version, never rolls back a
// verified copy, and leaves copied_source_pending on failure.
func (s *Service) CleanupTransferSource(p Principal, planID string, expectedRevision uint64) (TransferPlanView, error) {
	if e := s.transferGate(); e != nil {
		return TransferPlanView{}, e
	}
	if p.Agent != nil {
		return TransferPlanView{}, code("forbidden")
	}
	var plan TransferPlan
	if s.DB.First(&plan, "id = ? AND owner = ?", planID, p.dataOwner()).Error != nil {
		return TransferPlanView{}, code("not_found")
	}
	if expectedRevision != plan.Revision {
		return TransferPlanView{}, code("plan_stale")
	}
	var items []TransferItem
	s.DB.Where("plan_id = ? AND status = ?", plan.ID, "copied").Find(&items)
	if len(items) == 0 {
		return TransferPlanView{}, code("invalid_request")
	}
	now := time.Now().UTC()
	pending := false
	for _, item := range items {
		if item.TargetSHA256 == "" || item.TargetFileID == "" {
			// Unverified copies never authorize source deletion.
			s.DB.Model(&TransferItem{}).Where("id = ?", item.ID).Updates(map[string]any{"cleanup_state": "pending", "updated_at": now})
			pending = true
			continue
		}
		var f File
		if s.DB.First(&f, "id = ?", item.SourceFileID).Error != nil || f.VersionID != item.SourceVersionID {
			// The source moved after the copy: the original version is no
			// longer the head, cleanup stays pending for re-confirmation.
			s.DB.Model(&TransferItem{}).Where("id = ?", item.ID).Updates(map[string]any{"cleanup_state": "pending", "updated_at": now})
			pending = true
			continue
		}
		if _, e := s.file(p, item.SourceFileID, "write"); e != nil {
			s.DB.Model(&TransferItem{}).Where("id = ?", item.ID).Updates(map[string]any{"cleanup_state": "pending", "updated_at": now})
			pending = true
			continue
		}
		// Soft delete only: trash the source; the verified copy stays.
		if e := s.DB.Model(&File{}).Where("id = ?", item.SourceFileID).Updates(map[string]any{"trashed": true, "trashed_at": now, "updated": now}).Error; e != nil {
			s.DB.Model(&TransferItem{}).Where("id = ?", item.ID).Updates(map[string]any{"cleanup_state": "pending", "updated_at": now})
			pending = true
			continue
		}
		s.DB.Model(&TransferItem{}).Where("id = ?", item.ID).Updates(map[string]any{"cleanup_state": "trashed", "status": "source_trashed", "updated_at": now})
	}
	phase := "completed"
	if pending {
		phase = "source_cleanup_pending"
	} else {
		// Any item not yet source_trashed (failed, paused, unverified or
		// unprocessed) keeps the cleanup gap visible: the migration is not
		// claimed done while a source still stands.
		var remaining int64
		s.DB.Model(&TransferItem{}).Where("plan_id = ? AND status NOT IN ?", plan.ID, []string{"source_trashed", "cancelled"}).Count(&remaining)
		if remaining > 0 {
			phase = "source_cleanup_pending"
		}
	}
	s.DB.Model(&TransferPlan{}).Where("id = ?", plan.ID).Updates(map[string]any{"phase": phase, "cleanup_authorized": true, "revision": gorm.Expr("revision + 1"), "updated_at": now})
	var fresh TransferPlan
	s.DB.First(&fresh, "id = ?", plan.ID)
	return s.TransferPlanViewFor(p, fresh, "", 0)
}

// CancelTransferPlan stops new items; completed copies stay as committed
// side effects and are never claimed as rolled back.
func (s *Service) CancelTransferPlan(p Principal, planID string) (TransferPlanView, error) {
	if e := s.transferGate(); e != nil {
		return TransferPlanView{}, e
	}
	var plan TransferPlan
	if s.DB.First(&plan, "id = ? AND owner = ?", planID, p.dataOwner()).Error != nil {
		return TransferPlanView{}, code("not_found")
	}
	now := time.Now().UTC()
	s.DB.Model(&TransferItem{}).Where("plan_id = ? AND status IN ?", plan.ID, []string{"pending", "paused"}).Updates(map[string]any{"status": "cancelled", "updated_at": now})
	s.DB.Model(&TransferPlan{}).Where("id = ?", plan.ID).Updates(map[string]any{"phase": "cancelled", "revision": gorm.Expr("revision + 1"), "updated_at": now})
	if plan.OperationID != "" {
		s.DB.Model(&Operation{}).Where("id = ? AND status = ?", plan.OperationID, "running").Updates(map[string]any{"status": "cancelled", "updated": now})
	}
	var fresh TransferPlan
	s.DB.First(&fresh, "id = ?", plan.ID)
	return s.TransferPlanViewFor(p, fresh, "", 0)
}
