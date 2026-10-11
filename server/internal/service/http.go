package service

import (
	"context"
	"encoding/json"
	"github.com/yeisme/drivebridge-server/sdk/insights"
	"io"
	"mime"
	"net/http"
	"path"
	"strconv"
	"strings"
	"time"
)

func safeCode(e error) string {
	if e == nil {
		return ""
	}
	v := e.Error()
	if len(v) > 80 {
		return "internal_error"
	}
	for _, r := range v {
		if !(r >= 'a' && r <= 'z') && r != '_' {
			return "internal_error"
		}
	}
	return v
}
func writeError(w http.ResponseWriter, e error) {
	c := safeCode(e)
	status := 400
	switch c {
	case "unauthorized", "onedrive_authorization_required", "gdrive_authorization_required":
		status = 401
	case "unified_login_required", "identity_mapping_required", "cross_tenant_admin_context_required", "content_policy_required", "receiver_identity_required":
		status = 403
	case "service_admission_required", "identity_unavailable":
		status = 503
	case "approval_required", "account_recovery_required", "reauthentication_required", "forbidden", "comment_write_forbidden", "csrf_rejected", "encrypted_space_reference_rejected", "cross_instance_rejected", "label_namespace_forbidden":
		status = 403
	case "not_found", "object_missing":
		status = 404
	case "quota_below_occupied", "account_state_conflict", "last_platform_admin", "last_tenant_admin", "last_team_admin", "last_space_custodian", "storage_in_use", "invitation_exists", "invitation_unavailable", "invitation_not_pending", "team_archived", "idempotency_conflict", "version_conflict", "name_conflict", "part_conflict", "operation_closed", "export_conflict", "user_key_duplicate", "insufficient_funds", "worker_fenced", "projection_not_caught_up", "metadata_migration_requires_quiescence", "batch_not_sealed", "source_reselection_required", "transfer_session_expired", "version_protected", "purge_in_progress", "retention_plan_stale", "revision_conflict", "anchor_version_unavailable", "source_changed", "version_unavailable", "query_changed", "collection_revision_conflict", "plan_stale", "target_conflict", "content_index_stale", "delivery_incomplete", "retention_not_ready", "policy_revoked", "compensation_conflict", "rule_paused", "ack_ahead":
		status = 409
	case "cursor_expired", "snapshot_expired":
		status = 410
	case "quota_exceeded":
		status = 413
	case "invalid_expiry", "unsupported_action", "export_required", "source_cleanup_unsupported", "modality_unsupported":
		status = 422
	case "throttled", "budget_exceeded", "query_budget_exceeded", "task_scan_budget_exceeded":
		status = 429
	case "authorization_projection_stale", "analytics_backend_unavailable", "disabled", "oidc_not_configured", "storage_disconnected", "storage_unavailable", "storage_write_failed", "storage_read_failed", "storage_delete_failed", "transfer_failed", "delivery_unavailable", "onedrive_unavailable", "blob_unavailable", "dependency_unavailable":
		status = 503
	case "audit_unavailable", "insights_unavailable", "task_source_unavailable":
		status = 503
	case "internal_error", "database_unavailable", "disk_write_failed", "encryption_unavailable":
		status = 500
	}
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": c, "message": errorMessage(c)}})
}
func respond(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	json.NewEncoder(w).Encode(map[string]any{"data": v})
}

type auditTargetKey struct{}
type auditTarget struct{ ID string }

func markAuditTarget(r *http.Request, id string) {
	if target, ok := r.Context().Value(auditTargetKey{}).(*auditTarget); ok {
		target.ID = id
	}
}

type endpoint func(http.ResponseWriter, *http.Request, Principal) error

func (s *Service) wrap(action string, handler endpoint) http.HandlerFunc {
	return s.wrapObserved(action, handler, true)
}

// Receipt lookups must not create audit/usage mutations merely because their
// closed request uses POST. Authentication, CSRF and governance are unchanged.
func (s *Service) wrapObserved(action string, handler endpoint, observe bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !s.Config.Enabled {
			writeError(w, code("disabled"))
			return
		}
		p, e := s.authenticate(r)
		if e != nil {
			if s.Insights != nil {
				_ = s.Insights.Log(s.auditEvent(Principal{}, "authentication", "", "", safeCode(e), "http", insights.ID()))
			}
			writeError(w, e)
			return
		}
		invocation := s.startWorkspaceInvocation(p, "http", action)
		returned := false
		defer func() {
			if returned {
				s.finishWorkspaceInvocation(invocation, e)
			}
		}()
		p, e = s.bindContext(p, r)
		if e == nil {
			e = s.governanceGuard(p, action, r)
		}
		if e != nil {
			returned = true
			writeError(w, e)
			return
		}
		target := &auditTarget{}
		r = r.WithContext(context.WithValue(r.Context(), auditTargetKey{}, target))
		correlation := insights.ID()
		if observe && auditCritical(action, r.Method) && s.Insights != nil {
			if err := s.Insights.Log(s.auditEvent(p, action, r.PathValue("id"), "", "intent", "http", correlation)); err != nil {
				e = err
				returned = true
				writeError(w, err)
				return
			}
		}
		r.Body = http.MaxBytesReader(w, r.Body, PartSize+1)
		e = handler(w, r, p)
		returned = true
		outcome := "success"
		if e != nil {
			outcome = safeCode(e)
			writeError(w, e)
		}
		if !observe {
			return
		}
		resource := target.ID
		if resource == "" {
			resource = r.PathValue("id")
		}
		if resource == "" {
			resource = r.PathValue("connection")
		}
		if action != "audit.list" && action != "audit.events" && action != "audit.series" && !strings.HasPrefix(action, "usage.") {
			s.audit(p, action, resource, outcome)
		}
		if s.Insights != nil && (!strings.HasPrefix(action, "usage.") && !strings.HasPrefix(action, "audit.") && action != "operations.list" && action != "operations.get" && action != "files.list" && action != "files.stat" && action != "me" && action != "spaces.list" && action != "capabilities") {
			_ = s.Insights.Log(s.auditEvent(p, action, resource, "", outcome, "http", correlation))
		}
		// Typed file queries use POST for their filter body but do not mutate capacity.
		if r.Method != "GET" && r.Method != "HEAD" && action != "org.file_queries" {
			s.CaptureUsage(p.dataOwner())
		}
	}
}
func (s *Service) Handler() http.Handler {
	s.startInsights()
	mux := http.NewServeMux()
	s.governanceRoutes(mux)
	s.insightsRoutes(mux)
	s.uploadBatchRoutes(mux)
	s.browserRoutes(mux)
	s.retentionRoutes(mux)
	s.cloudRoutes(mux)
	s.organizationRoutes(mux)
	s.transferRoutes(mux)
	s.analyticsRoutes(mux)
	s.contentRoutes(mux)
	s.deliveryRoutes(mux)
	s.agentorgRoutes(mux)
	s.automationRoutes(mux)
	s.discussionRoutes(mux)
	s.mediaRoutes(mux)
	s.shareRoutes(mux)
	s.keyArchiveRoutes(mux)
	s.keyDirectoryRoutes(mux)
	s.handoffRoutes(mux)
	s.creativeRoutes(mux)
	s.creativeSaveRoutes(mux)
	s.documentRoutes(mux)
	s.notebookRoutes(mux)
	s.resourceRoutes(mux)
	s.propertyDefinitionRoutes(mux)
	s.propertySidecarRoutes(mux)
	s.favoriteRoutes(mux)
	s.linkRoutes(mux)
	s.workspaceHoldRoutes(mux)
	s.labelRoutes(mux)
	s.propertyMigrationRoutes(mux)
	mux.Handle("/mcp", s.MCP())
	mux.HandleFunc("POST /api/v1/files/{id}/access", s.wrap("files.access", s.accessHandler))
	mux.HandleFunc("GET /api/v1/access/{token}", s.accessStreamHandler)
	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		respond(w, map[string]any{"status": "ok", "enabled": s.Config.Enabled})
	})
	mux.HandleFunc("GET /auth/login", s.beginLogin)
	mux.HandleFunc("GET /auth/callback", s.finishLogin)
	mux.HandleFunc("GET /auth/mode", s.authMode)
	mux.HandleFunc("POST /auth/key", s.keyLogin)
	mux.HandleFunc("GET /auth/onedrive/callback", s.finishOneDrive)
	mux.HandleFunc("POST /auth/logout", s.wrap("logout", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil {
			return code("forbidden")
		}
		c, e := r.Cookie(s.cookieName("db_session"))
		if e != nil {
			return code("unauthorized")
		}
		s.DB.Delete(&Session{}, "hash = ?", Hash(c.Value))
		s.cookie(w, "db_session", "", -1)
		respond(w, map[string]bool{"logged_out": true})
		return nil
	}))
	mux.HandleFunc("GET /api/v1/tenant", s.wrap("tenant", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		respond(w, map[string]any{"tenant": s.Tenant(), "instance_id": s.Instance, "instance_ref_prefix": "drivebridge://" + s.Instance + "/file/"})
		return nil
	}))
	mux.HandleFunc("GET /api/v1/me", s.wrap("me", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var u User
		if s.DB.First(&u, "id = ?", p.UserID).Error != nil {
			return code("unauthorized")
		}
		respond(w, map[string]any{"user": u, "csrf_token": p.CSRF, "agent": p.Agent, "admin": p.Admin, "instance_id": s.Instance, "tenant_id": p.TenantID, "governance_enabled": s.governanceActive()})
		return nil
	}))
	s.profileRoutes(mux)
	mux.HandleFunc("GET /api/v1/capabilities", s.wrap("capabilities", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		// The optional space_id only narrows the additive limits block; every
		// pre-existing key keeps its type and semantics for old clients.
		ext, e := s.CapabilityExtension(p, r.URL.Query().Get("space_id"))
		if e != nil {
			return e
		}
		doc := map[string]any{"spec_version": "0.1", "instance_id": s.Instance, "public_base_url": s.Config.Origin, "part_size": PartSize, "max_bytes": s.Config.MaxBytes, "tools": ToolDefinitions(), "transport": "http", "one_drive_configured": s.Config.MicrosoftClientID != "", "scopes": []string{"read", "write", "delete", "import", "export"}, "local_paths": "local_client_only", "recovery": "query_original_operation_then_resume", "trash": map[string]any{"default_retention_days": defaultTrashRetentionDays, "batch_limit": maxTrashBatch, "auto_purge": true}, "key_directory": map[string]any{"self_revoke": true, "self_delete": true, "self_disable": true, "self_enable": true}, "service_identity": s.serviceIdentityCapability(p), "creative_handoff": map[string]any{"implemented": true, "requires_receiver_binding": true, "bind_path": "/api/v1/handoffs", "content_path": "/api/v1/handoffs/{id}/content", "operations": []string{"bind", "get", "renew", "cancel", "read"}}}
		for k, v := range ext {
			doc[k] = v
		}
		respond(w, doc)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/capabilities/check", s.wrap("capabilities.check", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			Action  string `json:"action"`
			SpaceID string `json:"space_id"`
			Ref     string `json:"ref"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		readiness, e := s.CheckCapability(p, q.Action, q.SpaceID, q.Ref)
		if e != nil {
			return e
		}
		respond(w, readiness)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/runtime/status", s.wrap("runtime.status", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.RuntimeStatusView(p)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/projections/{name}/rebuild", s.wrap("projections.rebuild", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			ExpectedGeneration string `json:"expected_generation"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.StartProjectionRebuild(p, r.PathValue("name"), q.ExpectedGeneration)
		if e != nil {
			return e
		}
		markAuditTarget(r, r.PathValue("name"))
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/spaces", s.wrap("spaces.list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		list := []Space{}
		query := s.DB.Where("owner = ?", p.dataOwner())
		if p.ScopeSpace != "" {
			query = query.Where("id = ?", p.ScopeSpace)
		}
		if p.Agent != nil {
			query = query.Where("id = ?", p.Agent.SpaceID)
		}
		if query.Find(&list).Error != nil {
			return code("database_unavailable")
		}
		respond(w, list)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/spaces", s.wrap("spaces.create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil {
			return code("forbidden")
		}
		var q struct {
			Name         string `json:"name"`
			ConnectionID string `json:"connection_id"`
			Encrypted    bool   `json:"encrypted"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		if q.Name == "" || len(q.Name) > 120 {
			return code("invalid_space")
		}
		if _, e := s.storageFor(p.dataOwner(), q.ConnectionID); e != nil {
			return e
		}
		id := ID()
		v := Space{TenantID: p.TenantID, ID: id, Owner: p.dataOwner(), Name: q.Name, ConnectionID: q.ConnectionID, MaxBytes: s.Config.MaxBytes, TrashRetentionDays: defaultTrashRetentionDays}
		if q.Encrypted {
			blob, e := s.newSpaceDEK(id)
			if e != nil {
				return e
			}
			v.Encrypted = true
			v.WrappedDEK = blob
		}
		if s.DB.Create(&v).Error != nil {
			return code("database_unavailable")
		}
		markAuditTarget(r, v.ID)
		respond(w, v)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/spaces/{id}/bind", s.wrap("spaces.bind", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil {
			return code("forbidden")
		}
		var q struct {
			ConnectionID string `json:"connection_id"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		sp, e := s.space(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		if sp.ConnectionID != "" || sp.Used != 0 || sp.Reserved != 0 {
			return code("space_binding_immutable")
		}
		if _, e = s.storageFor(p.dataOwner(), q.ConnectionID); e != nil {
			return e
		}
		if s.DB.Model(&sp).Update("connection_id", q.ConnectionID).Error != nil {
			return code("database_unavailable")
		}
		sp.ConnectionID = q.ConnectionID
		respond(w, sp)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/references", s.wrap("files.resolve", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		out, err := s.PinnedFile(p, r.URL.Query().Get("ref"))
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/files", s.wrap("files.list", s.listFiles))
	mux.HandleFunc("GET /api/v1/files/{id}", s.wrap("files.stat", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil && p.Agent.RestrictionProfile == "personal_workspace_v1" {
			return s.workspaceFileGet(w, r, p)
		}
		f, e := s.file(p, r.PathValue("id"), "read")
		if e != nil {
			return e
		}
		respond(w, map[string]any{"file": f, "ref": s.FileRef(f)})
		return nil
	}))
	mux.HandleFunc("GET /api/v1/files/{id}/versions", s.wrap("files.versions", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		f, e := s.file(p, r.PathValue("id"), "read")
		if e != nil {
			return e
		}
		v := []Version{}
		s.DB.Where("file_id = ? AND owner = ?", f.ID, p.dataOwner()).Order("created DESC").Find(&v)
		respond(w, v)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/files/{id}/content", s.wrap("files.download", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		download := s.Download
		if p.Agent != nil && p.Agent.RestrictionProfile == "personal_workspace_v1" {
			versions := r.URL.Query()["version"]
			if len(versions) != 1 || versions[0] == "" {
				return code("invalid_request")
			}
			download = s.downloadWorkspaceVersion
		}
		f, v, body, e := download(r.Context(), p, r.PathValue("id"), r.URL.Query().Get("version"))
		if e != nil {
			return e
		}
		defer body.Close()
		if e = s.sendContent(w, p, f, v, body); e != nil {
			s.audit(p, "files.download_incomplete", f.ID, safeCode(e))
			panic(http.ErrAbortHandler)
		}
		return nil
	}))
	mux.HandleFunc("POST /api/v1/files/{id}/trash", s.wrap("files.trash", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if e := s.Trash(p, r.PathValue("id"), false); e != nil {
			return e
		}
		respond(w, map[string]bool{"trashed": true})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/files/{id}/restore", s.wrap("files.restore", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if e := s.Trash(p, r.PathValue("id"), true); e != nil {
			return e
		}
		respond(w, map[string]bool{"restored": true})
		return nil
	}))
	mux.HandleFunc("DELETE /api/v1/files/{id}", s.wrap("files.purge", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.Header.Get("X-Confirm-Permanent-Delete") != "yes" {
			return code("confirmation_required")
		}
		if e := s.Purge(r.Context(), p, r.PathValue("id")); e != nil {
			return e
		}
		respond(w, map[string]bool{"purged": true})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/files/batch/restore", s.wrap("files.batch_restore", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			IDs []string `json:"ids"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		ok, failed, e := s.BatchRestore(p, q.IDs)
		if e != nil {
			return e
		}
		respond(w, map[string]any{"restored": ok, "failed": failed})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/files/batch/purge", s.wrap("files.batch_purge", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.Header.Get("X-Confirm-Permanent-Delete") != "yes" {
			return code("confirmation_required")
		}
		var q struct {
			IDs []string `json:"ids"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		ok, failed, e := s.BatchPurge(r.Context(), p, q.IDs)
		if e != nil {
			return e
		}
		respond(w, map[string]any{"purged": ok, "failed": failed})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/spaces/{id}/trash-policy", s.wrap("spaces.trash_policy", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			Days int `json:"days"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		sp, e := s.SetTrashPolicy(p, r.PathValue("id"), q.Days)
		if e != nil {
			return e
		}
		respond(w, sp)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/blobs/{sha256}", s.wrap("blobs.stat", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		info, e := s.BlobInfo(p, r.PathValue("sha256"))
		if e != nil {
			return e
		}
		respond(w, info)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/uploads", s.wrap("uploads.create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q UploadRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		op, e := s.CreateUpload(p, q, r.Header.Get("Idempotency-Key"))
		if e != nil {
			return e
		}
		respond(w, op)
		return nil
	}))
	mux.HandleFunc("PUT /api/v1/operations/{id}/parts/{index}", s.wrap("uploads.part", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		i, e := strconv.Atoi(r.PathValue("index"))
		if e != nil {
			return code("invalid_part")
		}
		var op Operation
		if p.Agent != nil && p.Agent.RestrictionProfile == "personal_workspace_v1" {
			op, e = s.putPartAuthorized(p, r.PathValue("id"), i, r.Body, false, true)
		} else {
			op, e = s.PutPart(p, r.PathValue("id"), i, r.Body)
		}
		if e != nil {
			return e
		}
		if op.Purpose == "inbox_staging" {
			w.Header().Set("Cache-Control", "no-store")
			respond(w, workspaceOperationProgress(op))
			return nil
		}
		respond(w, op)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/operations/{id}/complete", s.wrap("uploads.complete", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil && p.Agent.RestrictionProfile == "personal_workspace_v1" {
			op, err := s.completeAuthorized(r.Context(), p, r.PathValue("id"), true)
			if err != nil {
				return err
			}
			w.Header().Set("Cache-Control", "no-store")
			respond(w, workspaceOperationProgress(op))
			return nil
		}
		op, e := s.Complete(r.Context(), p, r.PathValue("id"))
		if e != nil {
			return e
		}
		respond(w, op)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/operations", s.wrap("operations.list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		limit := 200
		if raw := r.URL.Query().Get("limit"); raw != "" {
			var err error
			limit, err = strconv.Atoi(raw)
			if err != nil || limit < 1 || limit > 200 {
				return code("invalid_limit")
			}
		}
		query := s.DB.Where("owner = ?", p.dataOwner())
		if space := r.URL.Query().Get("space_id"); space != "" {
			if _, err := s.space(p, space); err != nil {
				return err
			}
			query = query.Where("space_id = ?", space)
		}
		if p.Agent != nil {
			query = query.Where("space_id = ?", p.Agent.SpaceID)
		}
		if r.URL.Query().Get("pending") == "true" {
			query = query.Where("status NOT IN ?", []string{"completed", "cancelled"})
		}
		all, out := []Operation{}, []Operation{}
		if query.Order("updated DESC").Find(&all).Error != nil {
			return code("database_unavailable")
		}
		for _, op := range all {
			if s.allow(p, op.SpaceID, op.Path, "read") == nil {
				out = append(out, op)
				if len(out) == limit {
					break
				}
			}
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/inbox", s.wrap("operations.inbox", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		limit := 50
		if raw := r.URL.Query().Get("limit"); raw != "" {
			n, e := strconv.Atoi(raw)
			if e != nil || n < 1 || n > 200 {
				return code("invalid_limit")
			}
			limit = n
		}
		q := s.DB.Where("owner = ? AND updated >= ?", p.dataOwner(), time.Now().UTC().Add(-30*24*time.Hour))
		if v := r.URL.Query().Get("status"); v != "" {
			q = q.Where("status = ?", v)
		}
		if v := r.URL.Query().Get("caller"); v != "" {
			q = q.Where("caller = ?", v)
		}
		if v := r.URL.Query().Get("space_id"); v != "" {
			q = q.Where("space_id = ?", v)
		}
		var all []Operation
		if q.Order("updated DESC").Limit(limit).Find(&all).Error != nil {
			return code("database_unavailable")
		}
		out := all[:0]
		for _, op := range all {
			if s.allow(p, op.SpaceID, op.Path, "read") == nil {
				out = append(out, op)
			}
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/archive", s.wrap("files.archive", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q ArchiveRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		op, e := s.ArchiveVersion(r.Context(), p, q, r.Header.Get("Idempotency-Key"))
		if e != nil {
			return e
		}
		respond(w, op)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/operations/{id}", s.wrap("operations.get", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil && p.Agent.RestrictionProfile == "personal_workspace_v1" {
			return s.workspaceOperationGet(w, r, p)
		}
		op, e := s.operation(p, r.PathValue("id"), "read")
		if e != nil {
			return e
		}
		parts := []Part{}
		s.DB.Where("operation_id = ?", op.ID).Order("`index`").Find(&parts)
		indexes := []int{}
		for _, part := range parts {
			indexes = append(indexes, part.Index)
		}
		respond(w, map[string]any{"operation": op, "received_parts": indexes})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/operations/{id}/cancel", s.wrap("operations.cancel", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		op, e := s.Cancel(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		respond(w, op)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/operations/{id}/resume", s.wrap("operations.resume", s.resumeOperation))
	mux.HandleFunc("GET /api/v1/connections", s.wrap("connections.list", s.listConnections))
	mux.HandleFunc("POST /api/v1/connections/s3", s.wrap("connections.s3", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			Name   string   `json:"name"`
			Config S3Config `json:"config"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		v, e := s.ConnectS3(r.Context(), p, q.Name, q.Config)
		if e != nil {
			return e
		}
		markAuditTarget(r, v.ID)
		respond(w, v)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/connections/onedrive", s.wrap("connections.onedrive", s.beginOneDrive))
	mux.HandleFunc("POST /api/v1/connections/{id}/disconnect", s.wrap("connections.disconnect", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil {
			return code("forbidden")
		}
		var c Connection
		if s.DB.First(&c, "id = ? AND owner = ?", r.PathValue("id"), p.dataOwner()).Error != nil {
			return code("not_found")
		}
		// Disconnection invalidates any previous observation; readiness must
		// not present a stale probe as current liveness.
		if s.DB.Model(&c).Updates(map[string]any{"disabled": true, "status": "disconnected", "observed_status": "", "observed_at": nil}).Error != nil {
			return code("database_unavailable")
		}
		respond(w, map[string]bool{"disconnected": true})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/connections/{id}/check", s.wrap("connections.check", s.checkConnection))
	mux.HandleFunc("GET /api/v1/connections/{id}/items", s.wrap("onedrive.browse", s.browseOneDrive))
	mux.HandleFunc("POST /api/v1/connections/{id}/import", s.wrap("onedrive.import", s.importOneDrive))
	mux.HandleFunc("POST /api/v1/connections/{id}/export", s.wrap("onedrive.export", s.exportOneDrive))
	mux.HandleFunc("POST /api/v1/inbox-submissions/{id}/archive", s.wrap("files.archive", s.workspaceSubmissionArchive))
	mux.HandleFunc("POST /api/v1/inbox-submissions/{id}/decision", s.wrap("org.labels_set", s.workspaceSubmissionDecision))
	mux.HandleFunc("GET /api/v1/inbox-submissions", s.wrap("operations.get", s.workspaceSubmissionList))
	mux.HandleFunc("GET /api/v1/inbox-submissions/{id}", s.wrap("operations.get", s.workspaceSubmissionGet))
	mux.HandleFunc("POST /api/v1/inbox-submissions", s.wrap("uploads.create", s.workspaceSubmissionCreate))
	mux.HandleFunc("POST /api/v1/inbox-submissions/lookup", s.wrap("uploads.lookup", s.workspaceSubmissionLookup))
	mux.HandleFunc("GET /api/v1/agent-groups", s.wrap("agents.list", s.workspaceGroupList))
	mux.HandleFunc("GET /api/v1/agent-organizations", s.wrap("agents.list", s.workspaceOrganizationList))
	mux.HandleFunc("GET /api/v1/agent-organizations/{agent_id}", s.wrap("agents.list", s.workspaceOrganizationGet))
	mux.HandleFunc("PATCH /api/v1/agent-organizations/{agent_id}", s.wrap("agents.update", s.workspaceOrganizationPatch))
	mux.HandleFunc("POST /api/v1/agent-groups", s.wrap("agents.create", s.workspaceGroupCreate))
	mux.HandleFunc("GET /api/v1/agent-groups/{id}", s.wrap("agents.list", s.workspaceGroupGet))
	mux.HandleFunc("DELETE /api/v1/agent-groups/{id}", s.wrap("agents.update", s.workspaceGroupDelete))
	mux.HandleFunc("PATCH /api/v1/agent-groups/{id}", s.wrap("agents.update", s.workspaceGroupRename))
	mux.HandleFunc("GET /api/v1/workspace/tasks/{kind}/{id}", s.wrap("operations.get", s.workspaceTaskDetail))
	mux.HandleFunc("GET /api/v1/workspace/tasks/upload_batch/{id}/items", s.wrap("upload_batches.get", s.workspaceBatchTaskItems))
	mux.HandleFunc("GET /api/v1/workspace/home", s.wrap("files.list", s.workspaceHome))
	mux.HandleFunc("GET /api/v1/workspace/tasks", s.wrap("operations.list", s.workspaceTasks))
	mux.HandleFunc("GET /api/v1/agent-invocations", s.wrap("agents.list", s.workspaceInvocationList))
	mux.HandleFunc("GET /api/v1/agent-grants", s.wrap("agents.list", s.workspaceGrantList))
	mux.HandleFunc("POST /api/v1/agent-grants", s.wrap("agents.create", s.workspaceGrantCreate))
	mux.HandleFunc("POST /api/v1/agent-grants/lookup", s.wrap("agents.list", s.workspaceGrantLookup))
	mux.HandleFunc("GET /api/v1/agent-grants/{id}", s.wrap("agents.list", s.workspaceGrantGet))
	mux.HandleFunc("POST /api/v1/agent-grants/{id}/changes/lookup", s.wrap("agents.list", s.workspaceGrantChangeLookup))
	mux.HandleFunc("POST /api/v1/agent-grants/{id}/checks", s.wrap("agents.list", s.workspaceGrantChecks))
	mux.HandleFunc("POST /api/v1/agent-grants/{id}/revoke", s.wrap("agents.revoke", s.workspaceGrantRevoke))
	mux.HandleFunc("PATCH /api/v1/agent-grants/{id}", s.wrap("agents.update", s.workspaceGrantPatch))
	mux.HandleFunc("POST /api/v1/agent-grants/{id}/rotate/lookup", s.wrap("agents.list", s.workspaceRotationLookup))
	mux.HandleFunc("POST /api/v1/agent-grants/{id}/rotate", s.wrap("agents.update", s.workspaceGrantRotate))
	mux.HandleFunc("GET /api/v1/agents", s.wrap("agents.list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil {
			return code("forbidden")
		}
		out := []Agent{}
		query := s.DB.Where("owner = ?", p.UserID)
		if s.governanceActive() {
			query = query.Where("space_id IN (?)", s.DB.Model(&Space{}).Select("id").Where("tenant_id = ?", p.TenantID))
		}
		query.Find(&out)
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/users", s.wrap("users.list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if !p.Admin {
			return code("forbidden")
		}
		keys := s.ListUserKeys()
		rows := make([]map[string]any, 0, len(keys))
		for _, k := range keys {
			rows = append(rows, map[string]any{"id": k.ID, "user": k.Owner, "name": k.Name, "admin": k.Admin, "expires_at": k.Expires, "revoked": k.Revoked, "archived": k.Archived, "disabled": k.Disabled})
		}
		respond(w, map[string]any{"keys": rows})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/users/{id}/rotate", s.wrap("users.rotate", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if !p.Admin {
			return code("forbidden")
		}
		var q struct {
			Days int `json:"days"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		issued, e := s.RotateUserKey(r.PathValue("id"), q.Days)
		if e != nil {
			return e
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, map[string]any{"user_id": issued.UserID, "name": issued.Name, "key": issued.Key, "admin": issued.Admin, "expires_at": issued.ExpiresAt})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/users/{id}/revoke", s.wrap("users.revoke", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if !p.Admin {
			return code("forbidden")
		}
		if e := s.RevokeUserKey(r.PathValue("id")); e != nil {
			return e
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, map[string]any{"revoked": true})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/users", s.wrap("users.create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			Users []UserImport `json:"users"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		issued, e := s.CreateUserUsers(p, q.Users)
		if e != nil {
			return e
		}
		respond(w, map[string]any{"issued": issued})
		return nil
	}))
	// users.list/users.revoke live above with the Principal.Admin gate; the
	// MCP tools call the same service methods directly.
	mux.HandleFunc("POST /api/v1/agents", s.wrap("agents.create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			Name         string `json:"name"`
			SpaceID      string `json:"space_id"`
			Prefix       string `json:"prefix"`
			Scopes       string `json:"scopes"`
			ConnectionID string `json:"connection_id"`
			Days         int    `json:"days"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		a, secret, e := s.createAgent(p, q.Name, q.SpaceID, q.Prefix, q.Scopes, q.ConnectionID, q.Days)
		if e != nil {
			return e
		}
		markAuditTarget(r, a.ID)
		respond(w, map[string]any{"agent": a, "token": secret})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/agents/{id}/revoke", s.wrap("agents.revoke", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil {
			return code("forbidden")
		}
		result := s.DB.Model(&Agent{}).Where("id = ? AND owner = ?", r.PathValue("id"), p.UserID).Update("revoked", true)
		if result.Error != nil {
			return code("database_unavailable")
		}
		if result.RowsAffected == 0 {
			return code("not_found")
		}
		respond(w, map[string]bool{"revoked": true})
		return nil
	}))
	mux.HandleFunc("GET /api/v1/audit", s.wrap("audit.list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		out := []Audit{}
		q := s.DB.Where("owner = ?", p.dataOwner())
		if p.Agent != nil {
			q = q.Where("agent_id = ?", p.Agent.ID)
		}
		q.Order("id DESC").Limit(200).Find(&out)
		respond(w, out)
		return nil
	}))
	if s.Config.WebDir != "" {
		mux.Handle("/", http.FileServer(http.Dir(s.Config.WebDir)))
	} else {
		mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) { http.NotFound(w, r) })
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Referrer-Policy", "no-referrer")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; script-src 'self'; style-src 'self' 'unsafe-inline'; img-src 'self' blob: data:; media-src 'self' blob:; connect-src 'self'; object-src 'none'; frame-ancestors 'none'; base-uri 'none'")
		mux.ServeHTTP(w, r)
	})
}

// serveContent streams one decrypted/plaintext object body. For envelope
// streams the final digest check runs after the last byte; a mismatch aborts
// the connection so a truncated body can never pass for a complete one.
func serveContent(w http.ResponseWriter, f File, v Version, body io.Reader) error {
	w.Header().Set("Content-Type", v.MIME)
	w.Header().Set("Content-Length", strconv.FormatInt(v.Size, 10))
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": path.Base(f.Path)}))
	w.Header().Set("X-Content-SHA256", v.SHA256)
	w.Header().Set("Cache-Control", "private, no-store")
	if _, e := io.CopyN(w, body, v.Size); e != nil {
		return code("transfer_failed")
	}
	if verifier, ok := body.(interface{ Verify() error }); ok {
		return verifier.Verify()
	}
	return nil
}
func (s *Service) listFiles(w http.ResponseWriter, r *http.Request, p Principal) error {
	search := s.SearchFiles
	if p.Agent != nil && p.Agent.RestrictionProfile == "personal_workspace_v1" {
		for key, values := range r.URL.Query() {
			switch key {
			case "space_id", "q", "prefix", "mime_type", "after", "trash", "tenant_context", "space_context":
			default:
				return code("invalid_request")
			}
			if len(values) != 1 {
				return code("invalid_request")
			}
		}
		if values, ok := r.URL.Query()["trash"]; ok && values[0] != "true" && values[0] != "false" {
			return code("invalid_request")
		}
		search = s.searchWorkspaceFiles
	}
	out, err := search(p, r.URL.Query().Get("space_id"), r.URL.Query().Get("q"), r.URL.Query().Get("prefix"), r.URL.Query().Get("mime_type"), r.URL.Query().Get("after"), r.URL.Query().Get("trash") == "true")
	if err != nil {
		return err
	}
	respond(w, out)
	return nil
}
func (s *Service) listConnections(w http.ResponseWriter, r *http.Request, p Principal) error {
	out := []Connection{}
	if p.Agent != nil {
		if p.Agent.ConnectionID != "" {
			s.DB.Where("id = ? AND owner = ?", p.Agent.ConnectionID, p.dataOwner()).Find(&out)
		}
	} else {
		s.DB.Where("owner = ? OR id = ?", p.dataOwner(), "platform").Find(&out)
	}
	if p.Agent == nil && p.ScopeSpace != "" {
		var sp Space
		if s.DB.First(&sp, "id = ?", p.ScopeSpace).Error == nil && sp.TeamID != "" && s.scopeRole(p, sp) != "" {
			var c Connection
			if s.DB.First(&c, "id = ?", sp.ConnectionID).Error == nil {
				found := false
				for _, v := range out {
					if v.ID == c.ID {
						found = true
					}
				}
				if !found {
					out = append(out, c)
				}
			}
		}
	}
	respond(w, out)
	return nil
}
func (s *Service) checkConnection(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.Agent != nil {
		return code("forbidden")
	}
	var c Connection
	if s.DB.First(&c, "id = ? AND (owner = ? OR id = ?)", r.PathValue("id"), p.dataOwner(), "platform").Error != nil {
		return code("not_found")
	}
	if c.Disabled {
		return code("storage_disconnected")
	}
	if c.Kind == "s3" {
		st, e := s.storageFor(p.dataOwner(), c.ID)
		if e != nil {
			return e
		}
		if e = st.Check(r.Context()); e != nil {
			// A failed probe records the observed failure without leaking
			// provider detail; readiness then reports unavailable.
			s.DB.Model(&Connection{}).Where("id = ?", c.ID).Updates(map[string]any{"observed_status": "failed", "observed_at": time.Now().UTC()})
			return e
		}
	} else {
		if _, e := s.graph(r.Context(), p, c.ID, "GET", "/me/drive?$select=id", nil); e != nil {
			s.DB.Model(&Connection{}).Where("id = ?", c.ID).Updates(map[string]any{"observed_status": "failed", "observed_at": time.Now().UTC()})
			return e
		}
	}
	// The probe succeeded: persist the observation timestamp that readiness
	// separates from configuration. This is the only mutating side effect of
	// an explicit connection check; capability preflight itself never probes.
	if e := s.DB.Model(&Connection{}).Where("id = ?", c.ID).Updates(map[string]any{"observed_status": "verified", "observed_at": time.Now().UTC()}).Error; e != nil {
		return code("database_unavailable")
	}
	c.ObservedStatus, c.Status = "verified", "verified"
	respond(w, map[string]string{"status": "verified"})
	return nil
}
func (s *Service) resumeOperation(w http.ResponseWriter, r *http.Request, p Principal) error {
	op, e := s.Resume(r.Context(), p, r.PathValue("id"))
	if e != nil {
		return e
	}
	respond(w, op)
	return nil
}

// uploadBatchRoutes registers the batch upload surface: creation, paged
// enumeration, seal, paginated read-back, control decisions and the
// lost-response lookup. All entries call the same application service the
// SDK and MCP tools use.
func (s *Service) uploadBatchRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/upload-batches", s.wrap("upload_batches.create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q UploadBatchRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.CreateUploadBatch(p, q, r.Header.Get("Idempotency-Key"))
		if e != nil {
			return e
		}
		markAuditTarget(r, view.BatchID)
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/upload-batches/{id}/items", s.wrap("upload_batches.add_items", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			Items []UploadBatchItemInput `json:"items"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.AddUploadBatchItems(p, r.PathValue("id"), q.Items)
		if e != nil {
			return e
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/upload-batches/{id}/seal", s.wrap("upload_batches.seal", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.SealUploadBatch(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/upload-batches/{id}/items", s.wrap("upload_batches.get", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		limit := 50
		if raw := r.URL.Query().Get("limit"); raw != "" {
			n, e := strconv.Atoi(raw)
			if e != nil || n < 1 || n > 200 {
				return code("invalid_request")
			}
			limit = n
		}
		view, e := s.GetUploadBatch(p, r.PathValue("id"), r.URL.Query().Get("cursor"), limit)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/upload-batches/{id}/pause", s.wrap("upload_batches.pause", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.ControlUploadBatch(p, r.PathValue("id"), "pause")
		if e != nil {
			return e
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/upload-batches/{id}/resume", s.wrap("upload_batches.resume", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.ControlUploadBatch(p, r.PathValue("id"), "resume")
		if e != nil {
			return e
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/upload-batches/{id}/cancel", s.wrap("upload_batches.cancel", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.ControlUploadBatch(p, r.PathValue("id"), "cancel")
		if e != nil {
			return e
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/operations/lookup", s.wrap("operations.lookup", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			Action          string          `json:"action"`
			IdempotencyKey  string          `json:"idempotency_key"`
			OriginalRequest json.RawMessage `json:"original_request"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.LookupOperation(p, q.Action, q.IdempotencyKey, q.OriginalRequest)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/operations/{id}/session", s.wrap("operations.session", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.UploadSessionFor(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
}

// retentionRoutes registers the retention surface: hold creation/read/
// release, preview plans and the human-only execution. Execution is
// deliberately not an MCP tool: agents never purge.
func (s *Service) retentionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/retention/holds", s.wrap("retention.holds_create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q RetentionHoldRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.CreateRetentionHold(p, q, r.Header.Get("Idempotency-Key"))
		if e != nil {
			return e
		}
		markAuditTarget(r, view.HoldID)
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/retention/holds/{id}", s.wrap("retention.holds_get", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.GetRetentionHold(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/retention/holds/{id}/release", s.wrap("retention.holds_release", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.ReleaseRetentionHold(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/retention/plans", s.wrap("retention.plans_preview", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			Category string `json:"category"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.PreviewRetention(p, q.Category)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/retention/plans/{id}/execute", s.wrap("retention.plans_execute", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.ExecuteRetention(r.Context(), p, r.PathValue("id"))
		if e != nil {
			return e
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, view)
		return nil
	}))
}

// discussionRoutes registers the version-discussion surface: file threads,
// replies, message edit/delete, resolution, private notes and the
// preview-only publication.
func (s *Service) discussionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/files/{id}/threads", s.wrap("threads.list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		limit := 50
		if raw := r.URL.Query().Get("limit"); raw != "" {
			n, e := strconv.Atoi(raw)
			if e != nil || n < 1 || n > 200 {
				return code("invalid_request")
			}
			limit = n
		}
		view, e := s.ListFileThreads(p, r.PathValue("id"), r.URL.Query().Get("cursor"), limit)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/files/{id}/threads", s.wrap("threads.create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			VersionID      string `json:"version_id"`
			Body           string `json:"body"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.CreateFileThread(p, r.PathValue("id"), q.VersionID, q.Body, firstNonEmpty(q.IdempotencyKey, r.Header.Get("Idempotency-Key")))
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/threads/{id}/replies", s.wrap("threads.reply", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			Body           string `json:"body"`
			IdempotencyKey string `json:"idempotency_key"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.ReplyFileThread(p, r.PathValue("id"), q.Body, firstNonEmpty(q.IdempotencyKey, r.Header.Get("Idempotency-Key")))
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("PATCH /api/v1/thread-messages/{id}", s.wrap("messages.edit", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			Body             string `json:"body"`
			ExpectedRevision uint64 `json:"expected_revision"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.EditThreadMessage(p, r.PathValue("id"), q.Body, q.ExpectedRevision)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("DELETE /api/v1/thread-messages/{id}", s.wrap("messages.delete", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		rev, e := strconv.ParseUint(r.URL.Query().Get("expected_revision"), 10, 64)
		if e != nil {
			return code("invalid_request")
		}
		view, e := s.DeleteThreadMessage(p, r.PathValue("id"), rev)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/threads/{id}/resolve", s.wrap("threads.resolve", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.SetThreadResolution(p, r.PathValue("id"), true)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/threads/{id}/reopen", s.wrap("threads.reopen", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.SetThreadResolution(p, r.PathValue("id"), false)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/files/{id}/private-note", s.wrap("notes.get", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.PrivateFileNote(p, r.PathValue("id"), "", 0, false)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("PUT /api/v1/files/{id}/private-note", s.wrap("notes.set", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			Body             string `json:"body"`
			ExpectedRevision uint64 `json:"expected_revision"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.PrivateFileNote(p, r.PathValue("id"), q.Body, q.ExpectedRevision, true)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/files/{id}/private-note/publish-preview", s.wrap("notes.publish_preview", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.PreviewNotePublication(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
}

// firstNonEmpty returns the first non-empty string.
func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
