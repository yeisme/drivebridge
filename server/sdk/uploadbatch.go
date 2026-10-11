package sdk

import (
	"context"
	"net/url"
	"strconv"
)

// is the fingerprint basis: a lost creation response replays with the same
// key and the same request onto the same batch.
type UploadBatchRequest struct {
	SpaceID        string `json:"space_id"`
	ConflictPolicy string `json:"conflict_policy"`
}

// UploadBatchItemInput is one enumerated source entry in an items page.
type UploadBatchItemInput struct {
	RelativePath    string `json:"relative_path"`
	IsDir           bool   `json:"is_dir,omitempty"`
	Size            int64  `json:"size_bytes"`
	SHA256          string `json:"sha256,omitempty"`
	FileID          string `json:"file_id,omitempty"`
	ExpectedVersion string `json:"expected_version,omitempty"`
}

// UploadBatchItemView is the per-item read model: source fingerprint, the
// original operation (JSON null before scheduling), status and result
// certainty (unknown until the original operation confirms).
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

// UploadBatchView is the batch read model including one keyset item page.
// ItemsTotal stays JSON null until the batch is sealed.
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

// CreateUploadBatch opens one enumeration under a durable idempotency key.
func (c *Client) CreateUploadBatch(ctx context.Context, q UploadBatchRequest, key string) (UploadBatchView, error) {
	var out UploadBatchView
	e := c.JSON(ctx, "POST", "/api/v1/upload-batches", key, q, &out)
	return out, e
}
func (c *Client) AddUploadBatchItems(ctx context.Context, batchID string, items []UploadBatchItemInput) (UploadBatchView, error) {
	var out UploadBatchView
	e := c.JSON(ctx, "POST", "/api/v1/upload-batches/"+url.PathEscape(batchID)+"/items", "", map[string]any{"items": items}, &out)
	return out, e
}

// SealUploadBatch ends the enumeration and freezes the batch fingerprint.
func (c *Client) SealUploadBatch(ctx context.Context, batchID string) (UploadBatchView, error) {
	var out UploadBatchView
	e := c.JSON(ctx, "POST", "/api/v1/upload-batches/"+url.PathEscape(batchID)+"/seal", "", nil, &out)
	return out, e
}

// GetUploadBatch reads the batch with one item page (limit 1-200).
func (c *Client) GetUploadBatch(ctx context.Context, batchID, cursor string, limit int) (UploadBatchView, error) {
	path := "/api/v1/upload-batches/" + url.PathEscape(batchID) + "/items?limit=" + url.QueryEscape(strconv.Itoa(limit))
	if cursor != "" {
		path += "&cursor=" + url.QueryEscape(cursor)
	}
	var out UploadBatchView
	e := c.JSON(ctx, "GET", path, "", nil, &out)
	return out, e
}

// ControlUploadBatch applies pause, resume or cancel to the original batch.
func (c *Client) ControlUploadBatch(ctx context.Context, batchID, action string) (UploadBatchView, error) {
	var out UploadBatchView
	e := c.JSON(ctx, "POST", "/api/v1/upload-batches/"+url.PathEscape(batchID)+"/"+url.PathEscape(action), "", nil, &out)
	return out, e
}

// OperationLookupView is the recovery answer for a lost creation response:
// the original identity behind an idempotency key, plus the certainty and
// the safe recovery action. Found=false never invents a replacement task.
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

// LookupOperation recovers the original operation behind an idempotency key
// after a lost creation response. The server recomputes the fingerprint with
// the same creation-function normalization, so the caller only needs the key
// and the original business request.
func (c *Client) LookupOperation(ctx context.Context, action, key string, originalRequest any) (OperationLookupView, error) {
	var out OperationLookupView
	body := map[string]any{"action": action, "idempotency_key": key, "original_request": originalRequest}
	e := c.JSON(ctx, "POST", "/api/v1/operations/lookup", "", body, &out)
	return out, e
}

// UploadSessionView reports the durable transfer protocol of one upload
// operation: agreed part size, verified received parts and whether the
// original source must be re-selected.
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

// UploadSession reads back the session of the original operation.
func (c *Client) UploadSession(ctx context.Context, operationID string) (UploadSessionView, error) {
	var out UploadSessionView
	e := c.JSON(ctx, "GET", "/api/v1/operations/"+url.PathEscape(operationID)+"/session", "", nil, &out)
	return out, e
}

// RetentionHoldRequest mirrors the server's hold creation input.
type RetentionHoldRequest struct {
	FileID    string `json:"file_id"`
	VersionID string `json:"version_id"`
	Purpose   string `json:"purpose"`
	SourceRef string `json:"source_ref"`
	ExpiresAt string `json:"expires_at"`
}

// RetentionHoldView is the hold read model: current state and safe source
// facts, never credentials.
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

// CreateRetentionHold protects one version under a durable idempotency key.
func (c *Client) CreateRetentionHold(ctx context.Context, q RetentionHoldRequest, key string) (RetentionHoldView, error) {
	var out RetentionHoldView
	e := c.JSON(ctx, "POST", "/api/v1/retention/holds", key, q, &out)
	return out, e
}

// GetRetentionHold reads one hold's current protection verdict.
func (c *Client) GetRetentionHold(ctx context.Context, holdID string) (RetentionHoldView, error) {
	var out RetentionHoldView
	e := c.JSON(ctx, "GET", "/api/v1/retention/holds/"+url.PathEscape(holdID), "", nil, &out)
	return out, e
}

// ReleaseRetentionHold ends the protection explicitly (human sessions only).
func (c *Client) ReleaseRetentionHold(ctx context.Context, holdID string) (RetentionHoldView, error) {
	var out RetentionHoldView
	e := c.JSON(ctx, "POST", "/api/v1/retention/holds/"+url.PathEscape(holdID)+"/release", "", nil, &out)
	return out, e
}

// PreviewRetention builds a read-only cleanup plan for one category.
func (c *Client) PreviewRetention(ctx context.Context, category string) (map[string]any, error) {
	var out map[string]any
	e := c.JSON(ctx, "POST", "/api/v1/retention/plans", "", map[string]string{"category": category}, &out)
	return out, e
}

// ExecuteRetention executes a previewed plan after re-validation
// (human sessions only; agents never purge).
func (c *Client) ExecuteRetention(ctx context.Context, planID string) (map[string]any, error) {
	var out map[string]any
	e := c.JSON(ctx, "POST", "/api/v1/retention/plans/"+url.PathEscape(planID)+"/execute", "", nil, &out)
	return out, e
}

// FileThreadCreate opens one thread; the response carries thread identity,
// anchor state and the first message.
func (c *Client) FileThreadCreate(ctx context.Context, fileID, versionID, body, key string) (map[string]any, error) {
	var out map[string]any
	e := c.JSON(ctx, "POST", "/api/v1/files/"+url.PathEscape(fileID)+"/threads", key, map[string]string{"version_id": versionID, "body": body, "idempotency_key": key}, &out)
	return out, e
}

// FileThreadsList lists threads with one keyset page.
func (c *Client) FileThreadsList(ctx context.Context, fileID, cursor string, limit int) (map[string]any, error) {
	path := "/api/v1/files/" + url.PathEscape(fileID) + "/threads?limit=" + url.QueryEscape(strconv.Itoa(limit))
	if cursor != "" {
		path += "&cursor=" + url.QueryEscape(cursor)
	}
	var out map[string]any
	e := c.JSON(ctx, "GET", path, "", nil, &out)
	return out, e
}

// ThreadReply appends one message under a durable idempotency key.
func (c *Client) ThreadReply(ctx context.Context, threadID, body, key string) (map[string]any, error) {
	var out map[string]any
	e := c.JSON(ctx, "POST", "/api/v1/threads/"+url.PathEscape(threadID)+"/replies", key, map[string]string{"body": body, "idempotency_key": key}, &out)
	return out, e
}

// ThreadMessageEdit edits the caller's own message with an expected revision.
func (c *Client) ThreadMessageEdit(ctx context.Context, messageID, body string, expectedRevision uint64) (map[string]any, error) {
	var out map[string]any
	e := c.JSON(ctx, "PATCH", "/api/v1/thread-messages/"+url.PathEscape(messageID), "", map[string]any{"body": body, "expected_revision": expectedRevision}, &out)
	return out, e
}

// ThreadMessageDelete replaces the caller's own message with a placeholder.
func (c *Client) ThreadMessageDelete(ctx context.Context, messageID string, expectedRevision uint64) (map[string]any, error) {
	var out map[string]any
	e := c.JSON(ctx, "DELETE", "/api/v1/thread-messages/"+url.PathEscape(messageID)+"?expected_revision="+url.QueryEscape(strconv.FormatUint(expectedRevision, 10)), "", nil, &out)
	return out, e
}

// ThreadResolve resolves a thread (discussion state only).
func (c *Client) ThreadResolve(ctx context.Context, threadID string) (map[string]any, error) {
	var out map[string]any
	e := c.JSON(ctx, "POST", "/api/v1/threads/"+url.PathEscape(threadID)+"/resolve", "", nil, &out)
	return out, e
}

// ThreadReopen reopens a resolved thread.
func (c *Client) ThreadReopen(ctx context.Context, threadID string) (map[string]any, error) {
	var out map[string]any
	e := c.JSON(ctx, "POST", "/api/v1/threads/"+url.PathEscape(threadID)+"/reopen", "", nil, &out)
	return out, e
}

// PrivateNoteGet reads the caller's own private note.
func (c *Client) PrivateNoteGet(ctx context.Context, fileID string) (map[string]any, error) {
	var out map[string]any
	e := c.JSON(ctx, "GET", "/api/v1/files/"+url.PathEscape(fileID)+"/private-note", "", nil, &out)
	return out, e
}

// PrivateNoteSet writes the caller's own private note with an expected
// revision (0 for the first write).
func (c *Client) PrivateNoteSet(ctx context.Context, fileID, body string, expectedRevision uint64) (map[string]any, error) {
	var out map[string]any
	e := c.JSON(ctx, "PUT", "/api/v1/files/"+url.PathEscape(fileID)+"/private-note", "", map[string]any{"body": body, "expected_revision": expectedRevision}, &out)
	return out, e
}

// PrivateNotePublishPreview previews the team publication of the note.
func (c *Client) PrivateNotePublishPreview(ctx context.Context, fileID string) (map[string]any, error) {
	var out map[string]any
	e := c.JSON(ctx, "POST", "/api/v1/files/"+url.PathEscape(fileID)+"/private-note/publish-preview", "", nil, &out)
	return out, e
}
