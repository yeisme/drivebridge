package service

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"gorm.io/gorm"
)

// organizes original upload Operations: per-item reservation, part staging,
// verification and publication stay in the existing upload path, and every
// batch decision replays onto the original identity instead of creating a
// replacement task.

// UploadBatchRequest is the creation input. Its canonical JSON form is the
// fingerprint basis shared by CreateUploadBatch and LookupOperation, so the
// server — not the browser — defines the recovery matching rule.
type UploadBatchRequest struct {
	SpaceID        string `json:"space_id"`
	ConflictPolicy string `json:"conflict_policy"`
}

func uploadBatchFingerprint(q UploadBatchRequest) string {
	body, _ := json.Marshal(q)
	return Hash(string(body))
}

// UploadBatchItemInput is one enumerated entry in an AddUploadBatchItems page.
type UploadBatchItemInput struct {
	RelativePath    string `json:"relative_path"`
	IsDir           bool   `json:"is_dir,omitempty"`
	Size            int64  `json:"size_bytes"`
	SHA256          string `json:"sha256,omitempty"`
	FileID          string `json:"file_id,omitempty"`
	ExpectedVersion string `json:"expected_version,omitempty"`
}

// UploadBatchView is the batch read model shared by HTTP, MCP and SDK.
type UploadBatchView struct {
	BatchID        string                `json:"batch_id"`
	OperationID    string                `json:"operation_id"`
	BatchRevision  uint64                `json:"batch_revision"`
	Phase          string                `json:"phase"`
	ConflictPolicy string                `json:"conflict_policy"`
	SpaceID        string                `json:"space_id"`
	ItemsTotal     *int64                `json:"items_total"`
	ItemsCompleted int64                 `json:"items_completed"`
	ItemsFailed    int64                 `json:"items_failed"`
	ItemsWaiting   int64                 `json:"items_waiting"`
	NextCursor     string                `json:"next_cursor,omitempty"`
	Items          []UploadBatchItemView `json:"items,omitempty"`
}

type UploadBatchItemView struct {
	ItemKey           string  `json:"item_key"`
	SourceFingerprint string  `json:"source_fingerprint"`
	RelativePath      string  `json:"relative_path"`
	IsDir             bool    `json:"is_dir,omitempty"`
	SizeBytes         int64   `json:"size_bytes"`
	SHA256            string  `json:"sha256,omitempty"`
	OperationID       *string `json:"operation_id"`
	Status            string  `json:"status"`
	ResultCertainty   string  `json:"result_certainty"`
	Error             string  `json:"error,omitempty"`
}

// CreateUploadBatch opens one enumeration under a durable idempotency key.
// A lost creation response replays here and receives the same batch.
func (s *Service) CreateUploadBatch(p Principal, q UploadBatchRequest, key string) (UploadBatchView, error) {
	if key == "" || len(key) > 200 {
		return UploadBatchView{}, code("invalid_request")
	}
	if q.ConflictPolicy == "" {
		q.ConflictPolicy = "fail"
	}
	if !uploadBatchPolicyValid(q.ConflictPolicy) {
		return UploadBatchView{}, code("invalid_request")
	}
	if _, e := s.space(p, q.SpaceID); e != nil {
		return UploadBatchView{}, e
	}
	if e := s.allow(p, q.SpaceID, "", "write"); e != nil {
		return UploadBatchView{}, e
	}
	fingerprint := uploadBatchFingerprint(q)
	s.mu.Lock()
	defer s.mu.Unlock()
	var batch UploadBatch
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&batch, "owner = ? AND key = ?", p.dataOwner(), key).Error; err == nil {
			if batch.Fingerprint != fingerprint {
				return code("idempotency_conflict")
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		var sp Space
		if err := tx.First(&sp, "id = ? AND owner = ?", q.SpaceID, p.dataOwner()).Error; err != nil {
			return code("not_found")
		}
		now := time.Now().UTC()
		operationID := ID()
		agent := ""
		if p.Agent != nil {
			agent = p.Agent.ID
		}
		op := Operation{ActorID: p.UserID, ID: operationID, Owner: p.dataOwner(), AgentID: agent, Key: key, Fingerprint: fingerprint, Kind: "upload_batch", SpaceID: q.SpaceID, Path: "", Status: "running", Updated: now}
		if err := tx.Create(&op).Error; err != nil {
			return code("database_unavailable")
		}
		batch = UploadBatch{ID: ID(), Owner: p.dataOwner(), TenantID: p.TenantID, SpaceID: q.SpaceID, ConflictPolicy: q.ConflictPolicy, Key: key, Fingerprint: fingerprint, Revision: 1, Phase: "enumerating", OperationID: operationID, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&batch).Error; err != nil {
			return code("database_unavailable")
		}
		return nil
	})
	if e != nil {
		return UploadBatchView{}, e
	}
	return s.uploadBatchView(p, batch, "", 0)
}

// batch loads one batch inside the caller's identity scope.
func (s *Service) batch(p Principal, id string) (UploadBatch, error) {
	var b UploadBatch
	if s.DB.First(&b, "id = ? AND owner = ?", id, p.dataOwner()).Error != nil {
		return b, code("not_found")
	}
	return b, nil
}

// validateRelativePath enforces the batch enumeration vocabulary: relative,
// cleaned, no parent traversal, no drive/UNC shapes, no control characters.
func validateRelativePath(rel string) error {
	if rel == "" {
		return code("invalid_request")
	}
	if strings.ContainsRune(rel, 0) || strings.ContainsRune(rel, '\n') || strings.ContainsRune(rel, '\r') {
		return code("invalid_request")
	}
	if strings.HasPrefix(rel, "/") || strings.HasPrefix(rel, "\\") || strings.Contains(rel, ":\\") {
		return code("invalid_request")
	}
	cleaned := path.Clean(rel)
	if cleaned != rel || cleaned == "." || cleaned == ".." || strings.HasPrefix(cleaned, "../") {
		return code("invalid_request")
	}
	for _, seg := range strings.Split(cleaned, "/") {
		if seg == ".." || seg == "." {
			return code("invalid_request")
		}
	}
	if len(rel) > 1024 {
		return code("invalid_request")
	}
	return nil
}

// AddUploadBatchItems appends one enumeration page. Pages are atomic: any
// invalid entry refuses the whole page without writing items.
func (s *Service) AddUploadBatchItems(p Principal, batchID string, items []UploadBatchItemInput) (UploadBatchView, error) {
	if len(items) == 0 || len(items) > uploadBatchMaxPage {
		return UploadBatchView{}, code("invalid_request")
	}
	b, e := s.batch(p, batchID)
	if e != nil {
		return UploadBatchView{}, e
	}
	if e := s.allow(p, b.SpaceID, "", "write"); e != nil {
		return UploadBatchView{}, e
	}
	if b.Phase != "enumerating" {
		// Sealed or terminal batches never accept new enumeration; the
		// decision must read the original batch, not grow a second one.
		return UploadBatchView{}, code("operation_closed")
	}
	seen := map[string]bool{}
	caseFold := map[string]bool{}
	for _, it := range items {
		rel := it.RelativePath
		isDir := it.IsDir
		if isDir && !strings.HasSuffix(rel, "/") {
			rel += "/"
		}
		if !isDir && strings.HasSuffix(rel, "/") {
			isDir = true
		}
		if e := validateRelativePath(strings.TrimSuffix(rel, "/")); e != nil {
			return UploadBatchView{}, e
		}
		if !isDir {
			if it.Size < 0 || (it.SHA256 != "" && !validHash(it.SHA256)) {
				return UploadBatchView{}, code("invalid_request")
			}
			if it.SHA256 == "" && it.Size != 0 {
				return UploadBatchView{}, code("invalid_request")
			}
		}
		if b.ConflictPolicy == "new_version" && it.FileID == "" {
			return UploadBatchView{}, code("invalid_request")
		}
		key := rel + "\x00" + it.SHA256
		if seen[key] {
			return UploadBatchView{}, code("invalid_request")
		}
		seen[key] = true
		// Case-insensitive backends treat differing case as the same name:
		// collisions are refused inside the page and against stored items.
		folded := strings.ToLower(rel)
		if caseFold[folded] {
			return UploadBatchView{}, code("invalid_request")
		}
		caseFold[folded] = true
		var clash int64
		s.DB.Model(&UploadBatchItem{}).Where("batch_id = ? AND case_folded = ?", b.ID, folded).Count(&clash)
		if clash > 0 {
			return UploadBatchView{}, code("invalid_request")
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e = s.DB.Transaction(func(tx *gorm.DB) error {
		// Re-read under the writer lock: the phase may have changed since
		// the read above (seal or cancel racing this page).
		var current UploadBatch
		if err := tx.First(&current, "id = ? AND owner = ?", batchID, p.dataOwner()).Error; err != nil {
			return code("not_found")
		}
		if current.Phase != "enumerating" {
			return code("operation_closed")
		}
		now := time.Now().UTC()
		for _, it := range items {
			rel := it.RelativePath
			if it.IsDir {
				if !strings.HasSuffix(rel, "/") {
					rel += "/"
				}
			}
			item := UploadBatchItem{ID: ID(), BatchID: batchID, ItemKey: rel + "\x00" + it.SHA256, Owner: p.dataOwner(), CaseFolded: strings.ToLower(rel), RelativePath: rel, IsDir: it.IsDir, Size: it.Size, SHA256: it.SHA256, FileID: it.FileID, ExpectedVersion: it.ExpectedVersion, Status: "pending", CreatedAt: now, UpdatedAt: now}
			if err := tx.Create(&item).Error; err != nil {
				if strings.Contains(err.Error(), "UNIQUE") {
					return code("invalid_request")
				}
				return code("database_unavailable")
			}
		}
		return tx.Model(&UploadBatch{}).Where("id = ?", batchID).Updates(map[string]any{"revision": gorm.Expr("revision + 1"), "updated_at": now}).Error
	})
	if e != nil {
		return UploadBatchView{}, e
	}
	b, _ = s.batch(p, batchID)
	return s.uploadBatchView(p, b, "", 0)
}

// uploadBatchItemView projects one row. The source fingerprint is the digest
// captured at enumeration time; a not-yet-scheduled item keeps an explicit
// null operation_id and an unknown result certainty — pending facts stay
// visible facts, never silently absent.
func uploadBatchItemView(it UploadBatchItem) UploadBatchItemView {
	certainty := it.Certainty
	if certainty == "" {
		certainty = "unknown"
	}
	var operationID *string
	if it.OperationID != "" {
		operationID = &it.OperationID
	}
	return UploadBatchItemView{ItemKey: it.ItemKey, SourceFingerprint: it.SHA256, RelativePath: it.RelativePath, IsDir: it.IsDir, SizeBytes: it.Size, SHA256: it.SHA256, OperationID: operationID, Status: it.Status, ResultCertainty: certainty, Error: it.Error}
}

// uploadBatchView assembles the read model including one keyset item page.
// The cursor is opaque: bound to this batch, its identity scope and the ID
// sort order. Pages never cross owners.
func (s *Service) uploadBatchView(p Principal, b UploadBatch, cursor string, limit int) (UploadBatchView, error) {
	out := UploadBatchView{BatchID: b.ID, OperationID: b.OperationID, BatchRevision: b.Revision, Phase: b.Phase, ConflictPolicy: b.ConflictPolicy, SpaceID: b.SpaceID, ItemsTotal: b.ItemsTotal}
	if limit > 0 {
		if limit > 200 {
			limit = 200
		}
		query := s.DB.Where("batch_id = ? AND owner = ?", b.ID, p.dataOwner())
		if cursor != "" {
			query = query.Where("id > ?", cursor)
		}
		var rows []UploadBatchItem
		if e := query.Order("id").Limit(limit + 1).Find(&rows).Error; e != nil {
			return UploadBatchView{}, code("database_unavailable")
		}
		if len(rows) > limit {
			out.NextCursor = rows[limit-1].ID
			rows = rows[:limit]
		}
		out.Items = make([]UploadBatchItemView, 0, len(rows))
		for _, it := range rows {
			out.Items = append(out.Items, uploadBatchItemView(it))
		}
	}
	var completed, failed, waiting int64
	for _, count := range []struct {
		statuses []string
		value    *int64
	}{
		{[]string{"completed"}, &completed}, {[]string{"failed"}, &failed}, {[]string{"pending", "transferring"}, &waiting},
	} {
		if err := s.DB.Model(&UploadBatchItem{}).Where("batch_id = ? AND owner = ? AND status IN ?", b.ID, p.dataOwner(), count.statuses).Count(count.value).Error; err != nil {
			return UploadBatchView{}, code("database_unavailable")
		}
	}
	out.ItemsCompleted, out.ItemsFailed, out.ItemsWaiting = completed, failed, waiting
	return out, nil
}

// GetUploadBatch returns the batch read model with one item page.
func (s *Service) GetUploadBatch(p Principal, batchID, cursor string, limit int) (UploadBatchView, error) {
	b, e := s.batch(p, batchID)
	if e != nil {
		return UploadBatchView{}, e
	}
	if e := s.allow(p, b.SpaceID, "", "read"); e != nil {
		return UploadBatchView{}, e
	}
	if limit <= 0 {
		limit = 50
	}
	return s.uploadBatchView(p, b, cursor, limit)
}

// SealUploadBatch ends the enumeration, freezes the batch fingerprint over
// the ordered item keys and moves the batch to the schedulable phase.
func (s *Service) SealUploadBatch(p Principal, batchID string) (UploadBatchView, error) {
	b, e := s.batch(p, batchID)
	if e != nil {
		return UploadBatchView{}, e
	}
	if e := s.allow(p, b.SpaceID, "", "write"); e != nil {
		return UploadBatchView{}, e
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e = s.DB.Transaction(func(tx *gorm.DB) error {
		var current UploadBatch
		if err := tx.First(&current, "id = ? AND owner = ?", batchID, p.dataOwner()).Error; err != nil {
			return code("not_found")
		}
		if current.Phase == "sealed" || current.Phase == "transferring" {
			return nil // idempotent re-seal
		}
		if current.Phase != "enumerating" {
			return code("operation_closed")
		}
		var items []UploadBatchItem
		if err := tx.Where("batch_id = ? AND owner = ?", batchID, p.dataOwner()).Order("id").Find(&items).Error; err != nil {
			return code("database_unavailable")
		}
		var total, bytesTotal int64
		for _, it := range items {
			total++
			bytesTotal += it.Size
		}
		now := time.Now().UTC()
		if err := tx.Model(&UploadBatch{}).Where("id = ?", batchID).Updates(map[string]any{"phase": "sealed", "sealed_at": now, "items_total": total, "bytes_total": bytesTotal, "revision": gorm.Expr("revision + 1"), "updated_at": now}).Error; err != nil {
			return code("database_unavailable")
		}
		return nil
	})
	if e != nil {
		return UploadBatchView{}, e
	}
	b, _ = s.batch(p, batchID)
	return s.uploadBatchView(p, b, "", 0)
}

// ControlUploadBatch applies pause, resume or cancel. A pause only blocks new
// scheduling and keeps verified data; a cancel keeps committed files and
// reports them as completed side effects — it never claims a rollback.
func (s *Service) ControlUploadBatch(p Principal, batchID, action string) (UploadBatchView, error) {
	b, e := s.batch(p, batchID)
	if e != nil {
		return UploadBatchView{}, e
	}
	if e := s.allow(p, b.SpaceID, "", "write"); e != nil {
		return UploadBatchView{}, e
	}
	if action != "pause" && action != "resume" && action != "cancel" {
		return UploadBatchView{}, code("invalid_request")
	}
	if action == "cancel" {
		return s.cancelUploadBatch(p, b.ID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	e = s.DB.Transaction(func(tx *gorm.DB) error {
		var current UploadBatch
		if err := tx.First(&current, "id = ? AND owner = ?", batchID, p.dataOwner()).Error; err != nil {
			return code("not_found")
		}
		if current.Phase == "cancelled" || current.Phase == "completed" {
			return code("operation_closed")
		}
		now := time.Now().UTC()
		phase := current.Phase
		switch action {
		case "pause":
			if current.Phase == "enumerating" {
				return code("batch_not_sealed")
			}
			phase = "paused"
		case "resume":
			if current.Phase != "paused" {
				return code("invalid_request")
			}
			var transferring int64
			if err := tx.Model(&UploadBatchItem{}).Where("batch_id = ? AND status = ?", batchID, "transferring").Count(&transferring).Error; err != nil {
				return code("database_unavailable")
			}
			if transferring > 0 {
				phase = "transferring"
			} else {
				phase = "sealed"
			}
		}
		return tx.Model(&UploadBatch{}).Where("id = ?", batchID).Updates(map[string]any{"phase": phase, "revision": gorm.Expr("revision + 1"), "updated_at": now}).Error
	})
	if e != nil {
		return UploadBatchView{}, e
	}
	b, _ = s.batch(p, batchID)
	return s.uploadBatchView(p, b, "", 0)
}

// OperationLookupView is the recovery answer for a lost creation response.
// Upload-centric identities keep their dedicated fields; other keyed
// creation records surface through resource_id/resource_type.
type OperationLookupView struct {
	Found           bool   `json:"found"`
	BatchID         string `json:"batch_id,omitempty"`
	OperationID     string `json:"operation_id,omitempty"`
	ResourceID      string `json:"resource_id,omitempty"`
	ResourceType    string `json:"resource_type,omitempty"`
	Status          string `json:"status,omitempty"`
	ResultCertainty string `json:"result_certainty"`
	RecoveryAction  string `json:"recovery_action,omitempty"`
}

// LookupOperation recovers the original identity behind an idempotency key
// after a lost creation response. The fingerprint is recomputed server-side
// with the same creation-function normalization, so the client never needs
// the creation response to have arrived.
func (s *Service) LookupOperation(p Principal, action, key string, originalRequest json.RawMessage) (OperationLookupView, error) {
	if key == "" || len(key) > 200 || len(originalRequest) == 0 {
		return OperationLookupView{}, code("invalid_request")
	}
	// Actions whose creation path keeps no durable keyed record (pure
	// queries, revision-guarded writes) answer found:false: replaying the
	// same key and the same request is the safe recovery, and a fresh
	// record can never appear under that key.
	switch action {
	case "analytics_query", "content_query", "organize_apply", "private_note_set":
		return lookupNotFound(), nil
	case "upload_batch_create":
		var q UploadBatchRequest
		if e := json.Unmarshal(originalRequest, &q); e != nil {
			return OperationLookupView{}, code("invalid_request")
		}
		fingerprint := uploadBatchFingerprint(q)
		var b UploadBatch
		if s.DB.First(&b, "owner = ? AND key = ?", p.dataOwner(), key).Error != nil {
			return lookupNotFound(), nil
		}
		if b.Fingerprint != fingerprint {
			return OperationLookupView{}, code("idempotency_conflict")
		}
		view, e := s.uploadBatchView(p, b, "", 0)
		if e != nil {
			return OperationLookupView{}, e
		}
		return OperationLookupView{Found: true, BatchID: b.ID, OperationID: view.OperationID, ResultCertainty: "confirmed", RecoveryAction: "read_original_operation"}, nil
	case "upload":
		return s.lookupAnchorOperation(p, key, "upload", originalRequest, func() (any, error) {
			var q UploadRequest
			if e := strictUnmarshal(originalRequest, &q); e != nil {
				return nil, e
			}
			body, _ := json.Marshal(q)
			return Hash(string(body)), nil
		})
	case "cloud_scan":
		return s.lookupAnchorOperation(p, key, "cloud_scan", originalRequest, func() (any, error) {
			var q struct {
				ConnectionID string `json:"connection_id"`
				RootID       string `json:"root_id"`
				Budget       int64  `json:"budget"`
			}
			if e := strictUnmarshal(originalRequest, &q); e != nil {
				return nil, e
			}
			return cloudScanFingerprint(q.ConnectionID, q.RootID, q.Budget), nil
		})
	case "thread":
		return s.lookupAnchorOperation(p, key, "file_thread", originalRequest, func() (any, error) {
			var q struct {
				FileID    string `json:"file_id"`
				VersionID string `json:"version_id"`
				Body      string `json:"body"`
			}
			if e := strictUnmarshal(originalRequest, &q); e != nil {
				return nil, e
			}
			fpBody, _ := json.Marshal(map[string]string{"file_id": q.FileID, "version_id": q.VersionID, "body": q.Body})
			return Hash(string(fpBody)), nil
		})
	case "thread_reply":
		return s.lookupAnchorOperation(p, key, "thread_reply", originalRequest, func() (any, error) {
			var q struct {
				ThreadID string `json:"thread_id"`
				Body     string `json:"body"`
			}
			if e := strictUnmarshal(originalRequest, &q); e != nil {
				return nil, e
			}
			fpBody, _ := json.Marshal(map[string]string{"thread_id": q.ThreadID, "body": q.Body})
			return Hash(string(fpBody)), nil
		})
	case "organize_plan":
		var q CreateOrganizationPlanRequest
		if e := strictUnmarshal(originalRequest, &q); e != nil {
			return OperationLookupView{}, code("invalid_request")
		}
		fingerprint := organizationPlanFingerprint(q)
		var plan OrganizationPlan
		if s.DB.First(&plan, "owner = ? AND key = ?", p.dataOwner(), key).Error != nil {
			return lookupNotFound(), nil
		}
		if plan.Fingerprint != fingerprint {
			return OperationLookupView{}, code("idempotency_conflict")
		}
		return OperationLookupView{Found: true, ResourceID: plan.ID, ResourceType: "organization_plan", Status: plan.Phase, ResultCertainty: planOutcomeCertainty(plan.Phase), RecoveryAction: "read_original_operation"}, nil
	case "delivery_prepare":
		var q struct {
			ShareID               string `json:"share_id"`
			SnapshotID            string `json:"snapshot_id"`
			ExpectedShareRevision uint64 `json:"expected_share_revision"`
		}
		if e := strictUnmarshal(originalRequest, &q); e != nil {
			return OperationLookupView{}, code("invalid_request")
		}
		fingerprint := deliveryFingerprint(q.ShareID, q.SnapshotID, q.ExpectedShareRevision)
		var stored DeliveryManifest
		if s.DB.First(&stored, "owner = ? AND key = ?", p.dataOwner(), key).Error != nil {
			return lookupNotFound(), nil
		}
		if stored.Fingerprint != fingerprint {
			return OperationLookupView{}, code("idempotency_conflict")
		}
		certainty := "unknown"
		if stored.Phase == "published" {
			certainty = "confirmed"
		}
		return OperationLookupView{Found: true, ResourceID: stored.ID, ResourceType: "delivery_manifest", Status: stored.Phase, ResultCertainty: certainty, RecoveryAction: "read_original_operation"}, nil
	case "transfer_plan":
		var q CreateTransferPlanRequest
		if e := strictUnmarshal(originalRequest, &q); e != nil {
			return OperationLookupView{}, code("invalid_request")
		}
		fingerprint := transferPlanFingerprint(q)
		var plan TransferPlan
		if s.DB.First(&plan, "owner = ? AND key = ?", p.dataOwner(), key).Error != nil {
			return lookupNotFound(), nil
		}
		if plan.Fingerprint != fingerprint {
			return OperationLookupView{}, code("idempotency_conflict")
		}
		return OperationLookupView{Found: true, ResourceID: plan.ID, ResourceType: "transfer_plan", Status: plan.Phase, ResultCertainty: planOutcomeCertainty(plan.Phase), RecoveryAction: "read_original_operation"}, nil
	}
	return OperationLookupView{}, code("invalid_request")
}

// lookupNotFound is the honest miss: no record exists under this key, so
// replaying the same key with the same original request is the only safe
// recovery — never a replacement task.
func lookupNotFound() OperationLookupView {
	return OperationLookupView{Found: false, ResultCertainty: "unknown", RecoveryAction: "replay_same_key_and_request"}
}

// strictUnmarshal decodes with the same unknown-field discipline as the
// HTTP surface, so a drifted original_request is invalid_request rather
// than a silently different fingerprint.
func strictUnmarshal(body []byte, v any) error {
	dec := json.NewDecoder(strings.NewReader(string(body)))
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

// lookupAnchorOperation recovers one Operation-anchored creation (upload,
// cloud_scan, thread anchors). The fingerprint closure mirrors the creation
// function's normalization exactly.
func (s *Service) lookupAnchorOperation(p Principal, key, kind string, originalRequest json.RawMessage, fingerprint func() (any, error)) (OperationLookupView, error) {
	fp, e := fingerprint()
	if e != nil {
		return OperationLookupView{}, code("invalid_request")
	}
	hash, _ := fp.(string)
	var op Operation
	if s.DB.First(&op, "owner = ? AND key = ? AND kind = ?", p.dataOwner(), key, kind).Error != nil {
		return lookupNotFound(), nil
	}
	if op.Fingerprint != hash {
		return OperationLookupView{}, code("idempotency_conflict")
	}
	certainty := "unknown"
	if op.Status == "completed" || op.Status == "failed" || op.Status == "cancelled" {
		certainty = "confirmed"
	}
	return OperationLookupView{Found: true, BatchID: op.BatchID, OperationID: op.ID, Status: op.Status, ResultCertainty: certainty, RecoveryAction: "read_original_operation"}, nil
}

// planOutcomeCertainty maps a plan-family phase to the certainty
// vocabulary: terminal phases are confirmed facts, everything else keeps
// the outcome honestly unknown.
func planOutcomeCertainty(phase string) string {
	switch phase {
	case "completed", "partial", "cancelled":
		return "confirmed"
	}
	return "unknown"
}

// UploadSessionView reports the durable transfer protocol state of one
// upload operation: the agreed client part size, the verified received
// ranges and whether the source must be re-selected by the original actor.
type UploadSessionView struct {
	OperationID               string `json:"operation_id"`
	BatchID                   string `json:"batch_id,omitempty"`
	Protocol                  string `json:"protocol"`
	PartSize                  int64  `json:"part_size"`
	BackendPartSize           int64  `json:"backend_part_size"`
	Generation                int64  `json:"generation"`
	ReceivedParts             []int  `json:"received_parts"`
	Durability                string `json:"durability"`
	SourceReselectionRequired bool   `json:"source_reselection_required"`
	BytesReceived             int64  `json:"bytes_received"`
}

// UploadSessionFor reads back the session of the original operation. Reading
// never mutates the session; the generation only moves on explicit recovery.
func (s *Service) UploadSessionFor(p Principal, operationID string) (UploadSessionView, error) {
	op, e := s.operation(p, operationID, "read")
	if e != nil {
		return UploadSessionView{}, e
	}
	var session UploadSession
	first := s.DB.First(&session, "operation_id = ?", op.ID).Error == nil
	var parts []Part
	s.DB.Where("operation_id = ?", op.ID).Order("`index`").Find(&parts)
	received := make([]int, 0, len(parts))
	var bytes int64
	for _, part := range parts {
		received = append(received, part.Index)
		// The logical received range is plaintext-sized: the last part is
		// short and staged ciphertext frames are not the client's numbering.
		if part.Size > 0 {
			lo := int64(part.Index) * PartSize
			hi := lo + part.Size
			if op.Size > 0 && hi > op.Size {
				hi = op.Size
			}
			if staged := op.Size - lo; staged > 0 && staged < part.Size {
				hi = lo + staged
			}
			bytes += hi - lo
		}
	}
	view := UploadSessionView{OperationID: op.ID, BatchID: op.BatchID, PartSize: PartSize, ReceivedParts: received, BytesReceived: bytes}
	if first {
		view.Protocol = session.Protocol
		view.BackendPartSize = session.BackendPartSize
		view.Generation = session.Generation
		view.Durability = session.Durability
		view.SourceReselectionRequired = session.SourceReselect
	} else {
		// Legacy operations keep the implicit v1 protocol; no session row is
		// fabricated for them, the view only reports observable facts.
		view.Protocol = "v1-implicit"
		view.BackendPartSize = 0
		view.Generation = 0
		view.Durability = "durable"
	}
	if op.Error == "original_source_required" {
		view.SourceReselectionRequired = true
	}
	return view, nil
}

// ensureUploadSession creates the protocol session row for batch-bound
// operations at scheduling time. Existing rows are never recreated here.
func (s *Service) ensureUploadSession(tx *gorm.DB, op Operation) error {
	var existing UploadSession
	if tx.First(&existing, "operation_id = ?", op.ID).Error == nil {
		return nil
	}
	now := time.Now().UTC()
	// S3-kind backends reassemble eight 8 MiB client parts into one 64 MiB
	// backend part; other stores stream sequentially without a fixed backend
	// part size. The client part numbering space is never shared with the
	// backend numbering space.
	backend := int64(64 << 20)
	// The backend transfer window follows the S3 multipart constraint:
	// initiation to completion within 7 days. After it lapses, parts are
	// refused with transfer_session_expired until an explicit resume
	// rebuilds the session under this same operation.
	window := now.Add(7 * 24 * time.Hour)
	session := UploadSession{ID: ID(), Owner: op.Owner, OperationID: op.ID, BatchID: op.BatchID, Protocol: "v1-8mib-client-parts", PartSize: PartSize, BackendPartSize: backend, Generation: 1, Durability: "durable", ExpiresAt: &window, CreatedAt: now, UpdatedAt: now}
	return tx.Create(&session).Error
}

// bindBatchItem validates and binds one batch item when its original upload
// operation is scheduled, and enforces the per-batch file concurrency budget.
// Reservation itself stays inside the existing createUpload transaction.
func (s *Service) bindBatchItem(tx *gorm.DB, p Principal, q *UploadRequest) (UploadBatch, error) {
	if q.BatchID == "" {
		return UploadBatch{}, nil
	}
	if q.ItemKey == "" {
		return UploadBatch{}, code("invalid_request")
	}
	var b UploadBatch
	if tx.First(&b, "id = ? AND owner = ?", q.BatchID, p.dataOwner()).Error != nil {
		return UploadBatch{}, code("not_found")
	}
	if b.SpaceID != q.SpaceID {
		return UploadBatch{}, code("invalid_request")
	}
	switch b.Phase {
	case "sealed", "transferring":
	default:
		if b.Phase == "enumerating" {
			return UploadBatch{}, code("batch_not_sealed")
		}
		return UploadBatch{}, code("operation_closed")
	}
	var item UploadBatchItem
	if tx.First(&item, "batch_id = ? AND item_key = ?", q.BatchID, q.ItemKey).Error != nil {
		return UploadBatch{}, code("not_found")
	}
	if item.IsDir {
		return UploadBatch{}, code("invalid_request")
	}
	// Source identity check: a re-selected file with a different size or
	// digest is a different source, not a retry of the original item.
	if item.SHA256 != q.SHA256 || item.Size != q.Size {
		return UploadBatch{}, code("content_mismatch")
	}
	if item.RelativePath != q.Path {
		return UploadBatch{}, code("content_mismatch")
	}
	// new_version policy schedules an explicit version replace: the item
	// pinned file_id+expected_version at enumeration and the scheduled
	// upload must target exactly that file version.
	if b.ConflictPolicy == "new_version" && (item.FileID == "" || q.FileID != item.FileID || q.ExpectedVersion != item.ExpectedVersion) {
		return UploadBatch{}, code("version_conflict")
	}
	if item.Status == "completed" {
		return UploadBatch{}, code("operation_closed")
	}
	// Per-batch concurrency budget: at most four files transfer at once.
	// Items without a free slot wait; they never fail the whole batch.
	var inflight int64
	if e := tx.Model(&UploadBatchItem{}).Where("batch_id = ? AND status = ?", q.BatchID, "transferring").Count(&inflight).Error; e != nil {
		return UploadBatch{}, code("database_unavailable")
	}
	if inflight >= 4 {
		return UploadBatch{}, code("budget_exceeded")
	}
	now := time.Now().UTC()
	if b.Phase == "sealed" {
		if e := tx.Model(&UploadBatch{}).Where("id = ?", b.ID).Updates(map[string]any{"phase": "transferring", "revision": gorm.Expr("revision + 1"), "updated_at": now}).Error; e != nil {
			return UploadBatch{}, code("database_unavailable")
		}
		b.Phase = "transferring"
	}
	return b, nil
}

// scheduleBatchItem marks the item as transferring once its operation row
// exists in the same transaction.
func scheduleBatchItem(tx *gorm.DB, owner, batchID, itemKey, operationID string) error {
	now := time.Now().UTC()
	return tx.Model(&UploadBatchItem{}).Where("batch_id = ? AND item_key = ? AND owner = ?", batchID, itemKey, owner).Updates(map[string]any{"status": "transferring", "operation_id": operationID, "updated_at": now}).Error
}

// finishBatchItem records the item outcome after its original operation
// reached a terminal state. Successful items are never rewritten by later
// batch decisions. Callers already hold s.mu (completion and cancellation
// serialize on it), so this path must not re-lock.
func (s *Service) finishBatchItem(op Operation, status, errText string) {
	if op.BatchID == "" || op.ItemKey == "" {
		return
	}
	_ = s.DB.Transaction(func(tx *gorm.DB) error {
		now := time.Now().UTC()
		// Conditional update: a cancelled batch may have already decided the
		// item; completed facts always win over later failure replays.
		q := tx.Model(&UploadBatchItem{}).Where("batch_id = ? AND item_key = ? AND status <> ?", op.BatchID, op.ItemKey, "completed")
		updates := map[string]any{"status": status, "updated_at": now, "error": errText}
		if status == "completed" {
			updates["certainty"] = "confirmed"
		}
		if e := q.Updates(updates).Error; e != nil {
			return e
		}
		// The batch task completes when nothing is pending or transferring.
		var waiting int64
		if err := tx.Model(&UploadBatchItem{}).Where("batch_id = ? AND status IN ?", op.BatchID, []string{"pending", "transferring"}).Count(&waiting).Error; err != nil {
			return code("database_unavailable")
		}
		if waiting == 0 {
			var b UploadBatch
			if err := tx.First(&b, "id = ?", op.BatchID).Error; err != nil {
				return code("database_unavailable")
			}
			if b.Phase != "cancelled" {
				if e := tx.Model(&UploadBatch{}).Where("id = ?", b.ID).Updates(map[string]any{"phase": "completed", "revision": gorm.Expr("revision + 1"), "updated_at": now}).Error; e != nil {
					return e
				}
				// The anchor operation is keyed by OperationID: addressing
				// it by the batch id silently missed and left the anchor
				// running forever.
				return tx.Model(&Operation{}).Where("id = ?", b.OperationID).Updates(map[string]any{"status": "completed", "updated": now}).Error
			}
		}
		return nil
	})
}

// itemStatusFromCertainty maps an operation's terminal state to the item
// result vocabulary used by the batch read model.
func itemStatusFromOperation(op Operation) (string, string) {
	switch op.Status {
	case "completed":
		return "completed", ""
	case "cancelled":
		return "cancelled", "operation_cancelled"
	default:
		return "failed", safeCode_str(op.Error)
	}
}

func safeCode_str(v string) string {
	if len(v) > 80 {
		return "internal_error"
	}
	for _, r := range v {
		if r < 0x20 && r != '\t' {
			return "internal_error"
		}
	}
	return v
}

// partReader streams the staged client parts of one operation in order
// without opening every file descriptor at once and without assembling the
// whole object on local disk. Seek support exists only so S3 signing can
// rewind to compute the payload hash; seeks never buffer the object.
type partReader struct {
	dir     string
	size    int64
	next    int
	inPart  int64
	pos     int64
	current *os.File
}

func (r *partReader) Read(b []byte) (int, error) {
	for {
		if r.current == nil {
			if r.pos >= r.size {
				return 0, io.EOF
			}
			f, e := os.Open(filepath.Join(r.dir, fmt.Sprintf("%08d", r.next)))
			if e != nil {
				return 0, code("missing_parts")
			}
			if r.inPart > 0 {
				if _, e = f.Seek(r.inPart, io.SeekStart); e != nil {
					f.Close()
					return 0, e
				}
			}
			r.current = f
		}
		n, e := r.current.Read(b)
		r.pos += int64(n)
		r.inPart += int64(n)
		if n > 0 {
			return n, nil
		}
		if e == io.EOF {
			r.current.Close()
			r.current = nil
			r.next++
			r.inPart = 0
			continue
		}
		if e != nil {
			return 0, e
		}
	}
}

// Seek repositions the stream by whole parts plus an in-part offset.
func (r *partReader) Seek(offset int64, whence int) (int64, error) {
	abs := r.pos
	switch whence {
	case io.SeekStart:
		abs = offset
	case io.SeekCurrent:
		abs += offset
	case io.SeekEnd:
		abs = r.size + offset
	}
	if abs < 0 || abs > r.size {
		return 0, fmt.Errorf("invalid seek")
	}
	if r.current != nil {
		r.current.Close()
		r.current = nil
	}
	r.next = int(abs / PartSize)
	r.inPart = abs - int64(r.next)*PartSize
	r.pos = abs
	return abs, nil
}

func (r *partReader) Close() error {
	if r.current != nil {
		return r.current.Close()
	}
	return nil
}

// streamStagedParts returns one sequential reader over the staged client
// parts. The reader is the backend reassembly input: parts flow through the
// bounded buffers of the storage client, never the whole file in memory.
func (s *Service) streamStagedParts(op Operation) *partReader {
	return &partReader{dir: filepath.Join(s.Config.State, "uploads", op.ID), size: op.Size}
}
