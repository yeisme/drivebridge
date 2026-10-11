package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"gorm.io/gorm"
)

// Content retrieval: pinned bindings to a content owner (Manju-shaped
// contract), human-configured policies and typed queries. Processing is
// never automatic; queries without the content owner report the
// dependency honestly instead of fabricating inference.

// ContentPolicyRequest is the human policy upsert.
type ContentPolicyRequest struct {
	Prefixes         []string `json:"prefixes,omitempty"`
	MimeAllowlist    []string `json:"mime_allowlist,omitempty"`
	RequestedLayers  []string `json:"requested_layers,omitempty"`
	MaxItems         int64    `json:"max_items,omitempty"`
	MaxBytes         int64    `json:"max_bytes,omitempty"`
	BudgetRef        string   `json:"budget_ref,omitempty"`
	EgressScope      string   `json:"egress_scope,omitempty"`
	Enabled          bool     `json:"enabled"`
	ExpectedRevision uint64   `json:"expected_revision,omitempty"`
}

// ContentPolicyView is the policy read model.
type ContentPolicyView struct {
	SpaceID         string    `json:"space_id"`
	PolicyRevision  uint64    `json:"policy_revision"`
	Prefixes        []string  `json:"prefixes,omitempty"`
	MimeAllowlist   []string  `json:"mime_allowlist,omitempty"`
	RequestedLayers []string  `json:"requested_layers,omitempty"`
	MaxItems        int64     `json:"max_items,omitempty"`
	MaxBytes        int64     `json:"max_bytes,omitempty"`
	BudgetRef       string    `json:"budget_ref,omitempty"`
	EgressScope     string    `json:"egress_scope,omitempty"`
	Enabled         bool      `json:"enabled"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// SetContentPolicy writes the human-only processing policy under an
// optimistic revision check. Agents never widen policies.
func (s *Service) SetContentPolicy(p Principal, spaceID string, q ContentPolicyRequest) (ContentPolicyView, error) {
	if p.Agent != nil {
		return ContentPolicyView{}, code("forbidden")
	}
	if _, e := s.space(p, spaceID); e != nil {
		return ContentPolicyView{}, e
	}
	if len(q.Prefixes) > 32 || len(q.MimeAllowlist) > 64 || len(q.RequestedLayers) > 16 {
		return ContentPolicyView{}, code("invalid_request")
	}
	var policy ContentPolicy
	s.DB.First(&policy, "owner = ? AND space_id = ?", p.dataOwner(), spaceID)
	if policy.ID != "" && q.ExpectedRevision != policy.PolicyRevision {
		return ContentPolicyView{}, code("version_conflict")
	}
	if policy.ID == "" && q.ExpectedRevision != 0 {
		return ContentPolicyView{}, code("version_conflict")
	}
	layers, _ := json.Marshal(q.RequestedLayers)
	now := time.Now().UTC()
	if policy.ID == "" {
		policy = ContentPolicy{ID: ID(), Owner: p.dataOwner(), SpaceID: spaceID, PolicyRevision: 1, Prefixes: strings.Join(q.Prefixes, ","), MimeAllowlist: strings.Join(q.MimeAllowlist, ","), RequestedLayers: layers, MaxItems: q.MaxItems, MaxBytes: q.MaxBytes, BudgetRef: q.BudgetRef, EgressScope: q.EgressScope, Enabled: q.Enabled, CreatedAt: now, UpdatedAt: now}
		if e := s.DB.Create(&policy).Error; e != nil {
			return ContentPolicyView{}, code("database_unavailable")
		}
		return s.contentPolicyView(spaceID), nil
	}
	updates := map[string]any{"prefixes": strings.Join(q.Prefixes, ","), "mime_allowlist": strings.Join(q.MimeAllowlist, ","), "requested_layers": layers, "max_items": q.MaxItems, "max_bytes": q.MaxBytes, "budget_ref": q.BudgetRef, "egress_scope": q.EgressScope, "enabled": q.Enabled, "policy_revision": policy.PolicyRevision + 1, "updated_at": now}
	res := s.DB.Model(&ContentPolicy{}).Where("id = ? AND policy_revision = ?", policy.ID, policy.PolicyRevision).Updates(updates)
	if res.Error != nil {
		return ContentPolicyView{}, code("database_unavailable")
	}
	if res.RowsAffected == 0 {
		// A concurrent revision won the write: the caller re-reads.
		return ContentPolicyView{}, code("version_conflict")
	}
	return s.contentPolicyView(spaceID), nil
}

// GetContentPolicy reads one space's policy.
func (s *Service) GetContentPolicy(p Principal, spaceID string) (ContentPolicyView, error) {
	if _, e := s.space(p, spaceID); e != nil {
		return ContentPolicyView{}, e
	}
	var policy ContentPolicy
	if s.DB.First(&policy, "owner = ? AND space_id = ?", p.dataOwner(), spaceID).Error != nil {
		return ContentPolicyView{}, code("not_found")
	}
	return s.contentPolicyView(spaceID), nil
}

func (s *Service) contentPolicyView(spaceID string) ContentPolicyView {
	var policy ContentPolicy
	s.DB.First(&policy, "space_id = ?", spaceID)
	split := func(v string) []string {
		if v == "" {
			return nil
		}
		return strings.Split(v, ",")
	}
	var layers []string
	json.Unmarshal(policy.RequestedLayers, &layers)
	return ContentPolicyView{SpaceID: spaceID, PolicyRevision: policy.PolicyRevision, Prefixes: split(policy.Prefixes), MimeAllowlist: split(policy.MimeAllowlist), RequestedLayers: layers, MaxItems: policy.MaxItems, MaxBytes: policy.MaxBytes, BudgetRef: policy.BudgetRef, EgressScope: policy.EgressScope, Enabled: policy.Enabled, UpdatedAt: policy.UpdatedAt}
}

// CreateContentBindingRequest pins one version to a content-owner project.
type CreateContentBindingRequest struct {
	Ref               string `json:"ref"`
	ManjuConnectionID string `json:"manju_connection_id"`
	ProjectRef        string `json:"project_ref"`
	Purpose           string `json:"purpose"`
}

// ContentBindingView is the binding read model with the original owner
// recovery link.
type ContentBindingView struct {
	BindingID         string    `json:"binding_id"`
	FileRef           string    `json:"file_ref"`
	ManjuConnectionID string    `json:"manju_connection_id"`
	ProjectRef        string    `json:"project_ref"`
	Purpose           string    `json:"purpose"`
	State             string    `json:"state"`
	PolicyRevision    uint64    `json:"policy_revision"`
	IndexGeneration   string    `json:"index_generation,omitempty"`
	OperationID       string    `json:"operation_id,omitempty"`
	Error             string    `json:"error,omitempty"`
	RecoveryRef       string    `json:"recovery_ref,omitempty"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// CreateContentBinding pins one fixed version for a content-owner project.
// The policy must exist and be enabled for the file's scope; nothing is
// processed at binding time.
func (s *Service) CreateContentBinding(p Principal, q CreateContentBindingRequest, key string) (ContentBindingView, error) {
	if key == "" || len(key) > 200 || q.ProjectRef == "" || q.Purpose == "" || q.ManjuConnectionID == "" {
		return ContentBindingView{}, code("invalid_request")
	}
	fileID, versionID, e := s.resolveRef(q.Ref)
	if e != nil {
		return ContentBindingView{}, e
	}
	f, e := s.file(p, fileID, "read")
	if e != nil {
		return ContentBindingView{}, e
	}
	var policy ContentPolicy
	if s.DB.First(&policy, "owner = ? AND space_id = ?", p.dataOwner(), f.SpaceID).Error != nil || !policy.Enabled {
		return ContentBindingView{}, code("content_policy_required")
	}
	if len(policy.Prefixes) > 0 {
		ok := false
		for _, prefix := range strings.Split(policy.Prefixes, ",") {
			if contains(prefix, f.Path) {
				ok = true
			}
		}
		if !ok {
			return ContentBindingView{}, code("content_policy_required")
		}
	}
	if len(policy.MimeAllowlist) > 0 && !strings.Contains(","+policy.MimeAllowlist+",", ","+f.MIME+",") {
		return ContentBindingView{}, code("content_policy_required")
	}
	fingerprintSource := struct {
		Ref     string `json:"ref"`
		Manju   string `json:"manju"`
		Project string `json:"project"`
		Purpose string `json:"purpose"`
	}{q.Ref, q.ManjuConnectionID, q.ProjectRef, q.Purpose}
	body, _ := json.Marshal(fingerprintSource)
	fingerprint := Hash(string(body))
	s.mu.Lock()
	defer s.mu.Unlock()
	var binding ContentBinding
	e = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&binding, "owner = ? AND key = ?", p.dataOwner(), key).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		now := time.Now().UTC()
		binding = ContentBinding{ID: ID(), Owner: p.dataOwner(), TenantID: p.TenantID, FileID: fileID, VersionID: versionID, ManjuConnectionID: q.ManjuConnectionID, ProjectRef: q.ProjectRef, Purpose: q.Purpose, State: "not_processed", PolicyRevision: policy.PolicyRevision, Key: key, Fingerprint: fingerprint, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&binding).Error; err != nil {
			return code("database_unavailable")
		}
		return nil
	})
	if e != nil {
		return ContentBindingView{}, e
	}
	if binding.Fingerprint != fingerprint {
		return ContentBindingView{}, code("idempotency_conflict")
	}
	return s.contentBindingView(p, binding), nil
}

func (s *Service) contentBindingView(p Principal, b ContentBinding) ContentBindingView {
	return ContentBindingView{BindingID: b.ID, FileRef: "drivebridge://" + s.Instance + "/file/" + b.FileID + "/version/" + b.VersionID, ManjuConnectionID: b.ManjuConnectionID, ProjectRef: b.ProjectRef, Purpose: b.Purpose, State: b.State, PolicyRevision: b.PolicyRevision, IndexGeneration: b.IndexGeneration, OperationID: b.OperationID, Error: b.Error, RecoveryRef: "drivebridge://" + s.Instance + "/file/" + b.FileID + "/version/" + b.VersionID, CreatedAt: b.CreatedAt, UpdatedAt: b.UpdatedAt}
}

// ListContentBindings lists the caller's bindings in one space.
func (s *Service) ListContentBindings(p Principal, spaceID string) ([]ContentBindingView, error) {
	if _, e := s.space(p, spaceID); e != nil {
		return nil, e
	}
	var bindings []ContentBinding
	if e := s.DB.Where("owner = ?", p.dataOwner()).Order("created_at").Find(&bindings).Error; e != nil {
		return nil, code("database_unavailable")
	}
	out := make([]ContentBindingView, 0, len(bindings))
	for _, b := range bindings {
		var f File
		if s.DB.First(&f, "id = ?", b.FileID).Error != nil || f.SpaceID != spaceID {
			continue
		}
		out = append(out, s.contentBindingView(p, b))
	}
	return out, nil
}

// GetContentBinding reads one binding.
func (s *Service) GetContentBinding(p Principal, bindingID string) (ContentBindingView, error) {
	var b ContentBinding
	if s.DB.First(&b, "id = ? AND owner = ?", bindingID, p.dataOwner()).Error != nil {
		return ContentBindingView{}, code("not_found")
	}
	return s.contentBindingView(p, b), nil
}

// manjuCall performs one typed call to the content owner endpoint
// configured on the creative-style connection. No credential or URL is
// ever echoed into responses or logs.
func (s *Service) manjuCall(ctx context.Context, p Principal, connectionID, method, apiPath string, payload any) ([]byte, error) {
	var conn CreativeConnection
	if s.DB.First(&conn, "id = ? AND owner = ? AND disabled = ?", connectionID, p.dataOwner(), false).Error != nil {
		return nil, code("identity_mapping_required")
	}
	if conn.Kind != "manju" {
		return nil, code("identity_mapping_required")
	}
	// Consuming an existing human-configured connection is allowed for
	// scoped agents; only connection management is human-only.
	var secret creativeSecret
	if e := s.unseal(p.dataOwner(), conn.Secret, &secret); e != nil {
		return nil, code("identity_mapping_required")
	}
	body, _ := json.Marshal(payload)
	req, e := http.NewRequestWithContext(ctx, method, strings.TrimSuffix(secret.URL, "/")+apiPath, bytes.NewReader(body))
	if e != nil {
		return nil, code("dependency_unavailable")
	}
	req.Header.Set("Content-Type", "application/json")
	if secret.Token != "" {
		req.Header.Set("Authorization", "Bearer "+secret.Token)
	}
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		return nil, code("dependency_unavailable")
	}
	defer resp.Body.Close()
	if resp.StatusCode == 403 {
		return nil, code("identity_mapping_required")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, code("dependency_unavailable")
	}
	return io.ReadAll(io.LimitReader(resp.Body, 4<<20))
}

// ProcessContentRequest executes the bound processing intent.
type ProcessContentRequest struct {
	BindingID      string `json:"binding_id"`
	PolicyRevision uint64 `json:"policy_revision"`
}

// ProcessContent submits one bound version to the content owner under an
// approved policy revision. Processing failure never affects the original
// file's readability; late results after a version change never attach to
// the new version.
func (s *Service) ProcessContent(ctx context.Context, p Principal, q ProcessContentRequest, key string) (ContentBindingView, error) {
	if key == "" || len(key) > 200 {
		return ContentBindingView{}, code("invalid_request")
	}
	var b ContentBinding
	if s.DB.First(&b, "id = ? AND owner = ?", q.BindingID, p.dataOwner()).Error != nil {
		return ContentBindingView{}, code("not_found")
	}
	var f File
	if s.DB.First(&f, "id = ?", b.FileID).Error != nil || f.VersionID != b.VersionID {
		// The pinned version moved: the binding goes stale and the owner
		// must clean derived indexes; the new version needs a new intent.
		s.DB.Model(&ContentBinding{}).Where("id = ?", b.ID).Updates(map[string]any{"state": "stale", "error": "content_index_stale", "updated_at": time.Now().UTC()})
		return ContentBindingView{}, code("content_index_stale")
	}
	var policy ContentPolicy
	if s.DB.First(&policy, "owner = ? AND space_id = ?", p.dataOwner(), f.SpaceID).Error != nil || !policy.Enabled {
		return ContentBindingView{}, code("content_policy_required")
	}
	if q.PolicyRevision != policy.PolicyRevision {
		return ContentBindingView{}, code("version_conflict")
	}
	now := time.Now().UTC()
	// Persisted idempotency: the key plus the binding/policy intent is
	// the durable identity of this processing run.
	fingerprint := Hash(b.ID + "|" + fmt.Sprint(policy.PolicyRevision) + "|" + b.VersionID)
	if b.ProcessKey == key {
		if b.ProcessFingerprint != fingerprint {
			return ContentBindingView{}, code("idempotency_conflict")
		}
		if b.State != "failed" && b.OperationID != "" {
			// Replay reads the original operation back.
			var original Operation
			if s.DB.First(&original, "id = ?", b.OperationID).Error == nil && original.Status != "failed" {
				return s.contentBindingView(p, b), nil
			}
		}
		// A failed prior attempt under this key leaves room for one
		// explicit retry with the same key.
	} else if b.ProcessKey != "" && b.State != "failed" {
		return ContentBindingView{}, code("idempotency_conflict")
	}
	s.DB.Model(&ContentBinding{}).Where("id = ?", b.ID).Updates(map[string]any{"process_key": key, "process_fingerprint": fingerprint, "updated_at": now})
	op := Operation{ActorID: p.UserID, ID: ID(), Owner: p.dataOwner(), Kind: "content_process", SpaceID: f.SpaceID, Path: f.Path, Key: "content-process:" + b.ID + ":" + key, Status: "running", Updated: now}
	if s.DB.Create(&op).Error != nil {
		return ContentBindingView{}, code("database_unavailable")
	}
	s.DB.Model(&ContentBinding{}).Where("id = ?", b.ID).Updates(map[string]any{"state": "processing", "operation_id": op.ID, "updated_at": now})
	response, e := s.manjuCall(ctx, p, b.ManjuConnectionID, "POST", "/api/v1/process", map[string]any{"project_ref": b.ProjectRef, "purpose": b.Purpose, "layers": policy.RequestedLayers, "file": map[string]any{"file_id": b.FileID, "version_id": b.VersionID}})
	if e != nil {
		s.DB.Model(&ContentBinding{}).Where("id = ?", b.ID).Updates(map[string]any{"state": "failed", "error": safeCode(e), "updated_at": time.Now().UTC()})
		s.DB.Model(&Operation{}).Where("id = ?", op.ID).Updates(map[string]any{"status": "failed", "error": safeCode(e), "updated": time.Now().UTC()})
		return ContentBindingView{}, e
	}
	var processed struct {
		IndexGeneration string   `json:"index_generation"`
		Unsupported     []string `json:"unsupported"`
	}
	json.Unmarshal(response, &processed)
	state := "searchable"
	errText := ""
	if len(processed.Unsupported) > 0 {
		state = "unsupported"
		errText = strings.Join(processed.Unsupported, ",")
	}
	// Late-result guard: the source version must still be current when
	// the result lands.
	var current File
	s.DB.First(&current, "id = ?", b.FileID)
	if current.VersionID != b.VersionID {
		s.DB.Model(&ContentBinding{}).Where("id = ?", b.ID).Updates(map[string]any{"state": "stale", "error": "content_index_stale", "updated_at": time.Now().UTC()})
		s.DB.Model(&Operation{}).Where("id = ?", op.ID).Updates(map[string]any{"status": "failed", "error": "content_index_stale", "updated": time.Now().UTC()})
		return ContentBindingView{}, code("content_index_stale")
	}
	s.DB.Model(&ContentBinding{}).Where("id = ?", b.ID).Updates(map[string]any{"state": state, "index_generation": processed.IndexGeneration, "error": errText, "updated_at": time.Now().UTC()})
	s.DB.Model(&Operation{}).Where("id = ?", op.ID).Updates(map[string]any{"status": "completed", "updated": time.Now().UTC()})
	var fresh ContentBinding
	s.DB.First(&fresh, "id = ?", b.ID)
	return s.contentBindingView(p, fresh), nil
}

// ContentQueryRequest is the typed retrieval query.
type ContentQueryRequest struct {
	Mode              string            `json:"mode"` // exact|discovery
	Text              string            `json:"text,omitempty"`
	ImageRef          string            `json:"image_ref,omitempty"`
	SpaceIDs          []string          `json:"space_ids"`
	Modalities        []string          `json:"modalities,omitempty"`
	Filters           []FileQueryFilter `json:"filters,omitempty"`
	EvidenceRequired  bool              `json:"evidence_required"`
	Limit             int               `json:"limit,omitempty"`
	ManjuConnectionID string            `json:"manju_connection_id,omitempty"`
}

// ContentQueryItem is one retrieval result row.
type ContentQueryItem struct {
	FileRef          string         `json:"file_ref"`
	AssetRevisionRef string         `json:"asset_revision_ref,omitempty"`
	EvidenceRef      string         `json:"evidence_ref,omitempty"`
	Locator          map[string]any `json:"locator,omitempty"`
	TruthLevel       string         `json:"truth_level,omitempty"`
	IndexGeneration  string         `json:"index_generation,omitempty"`
}

// ContentQueryView is one bounded retrieval page.
type ContentQueryView struct {
	Mode       string             `json:"mode"`
	Coverage   string             `json:"coverage"`
	Items      []ContentQueryItem `json:"items"`
	Missing    []string           `json:"missing,omitempty"`
	NextCursor string             `json:"next_cursor,omitempty"`
}

// QueryContent answers one typed retrieval query. exact mode reads the
// managed catalogue facts only; discovery mode requires the content owner
// and reports its unavailability honestly. Pure queries never trigger new
// analysis.
func (s *Service) QueryContent(ctx context.Context, p Principal, q ContentQueryRequest) (ContentQueryView, error) {
	if q.Mode != "exact" && q.Mode != "discovery" {
		return ContentQueryView{}, code("invalid_query")
	}
	if len(q.SpaceIDs) == 0 {
		return ContentQueryView{}, code("invalid_query")
	}
	for _, m := range q.Modalities {
		switch m {
		case "document", "image", "video", "audio":
		default:
			return ContentQueryView{}, code("modality_unsupported")
		}
	}
	limit := orgPageLimit(q.Limit)
	if q.Mode == "exact" {
		view, e := s.QueryFiles(p, FileQueryRequest{Mode: "exact", SpaceIDs: q.SpaceIDs, Filters: q.Filters, Limit: limit})
		if e != nil {
			return ContentQueryView{}, e
		}
		out := ContentQueryView{Mode: "exact", Coverage: view.Coverage, Items: []ContentQueryItem{}}
		for _, it := range view.Items {
			out.Items = append(out.Items, ContentQueryItem{FileRef: "drivebridge://" + s.Instance + "/file/" + it.FileID + "/version/" + it.VersionID})
		}
		return out, nil
	}
	// Discovery: the content owner answers with evidence and locators.
	connectionID := q.ManjuConnectionID
	if connectionID == "" {
		var conn CreativeConnection
		if s.DB.First(&conn, "owner = ? AND kind = ?", p.dataOwner(), "manju").Error != nil {
			return ContentQueryView{}, code("dependency_unavailable")
		}
		connectionID = conn.ID
	}
	scope, e := s.analyticsAuthorizationScope(p)
	if e != nil {
		return ContentQueryView{}, e
	}
	payload := map[string]any{"mode": "discovery", "text": q.Text, "space_ids": scope, "modalities": q.Modalities, "evidence_required": q.EvidenceRequired, "limit": limit}
	if q.ImageRef != "" {
		payload["image_ref"] = q.ImageRef
	}
	response, e := s.manjuCall(ctx, p, connectionID, "POST", "/api/v1/query", payload)
	if e != nil {
		return ContentQueryView{}, e
	}
	var decoded struct {
		Coverage string             `json:"coverage"`
		Items    []ContentQueryItem `json:"items"`
		Missing  []string           `json:"missing"`
	}
	if json.Unmarshal(response, &decoded) != nil {
		return ContentQueryView{}, code("dependency_unavailable")
	}
	// Every returned item re-authorizes against the current managed
	// permission before it leaves this service.
	out := ContentQueryView{Mode: "discovery", Coverage: decoded.Coverage, Items: []ContentQueryItem{}, Missing: decoded.Missing}
	for _, it := range decoded.Items {
		fileID, _, e := s.resolveRef(it.FileRef)
		if e != nil {
			continue
		}
		if _, e := s.file(p, fileID, "read"); e != nil {
			continue
		}
		out.Items = append(out.Items, it)
	}
	return out, nil
}

var _ = fmt.Sprintf
