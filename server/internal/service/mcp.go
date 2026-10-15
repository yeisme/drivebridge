package service

import (
	"context"
	"encoding/json"
	"github.com/yeisme/drivebridge-server/sdk/insights"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/mcp"
	"github.com/mark3labs/mcp-go/server"
)

type principalKey struct{}

func (s *Service) MCP() http.Handler {
	srv := server.NewMCPServer("drivebridge-service", "0.1.2", server.WithToolCapabilities(false), server.WithResourceCapabilities(false, false), server.WithToolFilter(s.workspaceToolFilter))
	for _, tool := range ToolDefinitions() {
		name := tool.Name
		srv.AddTool(tool, func(ctx context.Context, q mcp.CallToolRequest) (*mcp.CallToolResult, error) {
			p, ok := ctx.Value(principalKey{}).(Principal)
			if !ok {
				return mcp.NewToolResultError("unauthorized"), nil
			}
			invocation := s.startWorkspaceInvocation(p, "mcp", name)
			result, e := s.CallTool(ctx, p, name, q.GetArguments())
			defer func() { s.finishWorkspaceInvocation(invocation, e) }()
			outcome := "success"
			if e != nil {
				outcome = safeCode(e)
			}
			resource, _ := q.GetArguments()["file_id"].(string)
			if resource == "" {
				resource, _ = q.GetArguments()["operation_id"].(string)
			}
			if !strings.HasPrefix(name, "usage_") && !strings.HasPrefix(name, "audit_") && name != "operation_get" && name != "input.handoff_status" && name != "operations" && name != "capabilities" && name != "files" && name != "stat" {
				s.audit(p, "mcp."+name, resource, outcome)
				if s.Insights != nil {
					_ = s.Insights.Log(s.auditEvent(p, "mcp."+name, resource, "", outcome, "mcp", insights.ID()))
				}
			}
			if e != nil {
				payload := map[string]any{
					"error": map[string]any{
						"reason":        safeCode(e),
						"impact":        "The requested operation was not completed.",
						"next_actions":  []string{"Use capabilities to inspect the available workflow, then retry the original operation only when retryable is true."},
						"retryable":     retryableError(e),
						"recovery_tool": "capabilities",
					},
				}
				b, _ := json.Marshal(payload)
				return mcp.NewToolResultError(string(b)), nil
			}
			b, e := json.Marshal(result)
			if e != nil {
				return mcp.NewToolResultError("serialization_failed"), nil
			}
			return mcp.NewToolResultText(string(b)), nil
		})
	}
	srv.AddResource(mcp.NewResource("drivebridge://capabilities", "DriveBridge authenticated capabilities", mcp.WithMIMEType("application/json")), func(ctx context.Context, q mcp.ReadResourceRequest) ([]mcp.ResourceContents, error) {
		p, ok := ctx.Value(principalKey{}).(Principal)
		if !ok {
			return nil, code("unauthorized")
		}
		out, e := s.CallTool(ctx, p, "capabilities", nil)
		b, _ := json.Marshal(out)
		return []mcp.ResourceContents{mcp.TextResourceContents{URI: q.Params.URI, MIMEType: "application/json", Text: string(b)}}, e
	})
	stream := server.NewStreamableHTTPServer(srv, server.WithStateLess(true))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !s.Config.Enabled {
			writeError(w, code("disabled"))
			return
		}
		p, e := s.authenticate(r)
		if e != nil {
			if s.Insights != nil {
				_ = s.Insights.Log(s.auditEvent(Principal{}, "authentication", "", "", safeCode(e), "mcp", insights.ID()))
			}
			writeError(w, e)
			return
		}
		p, e = s.bindContext(p, r)
		if e != nil {
			writeError(w, e)
			return
		}
		stream.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), principalKey{}, p)))
	})
}
func (s *Service) CallTool(ctx context.Context, p Principal, name string, args map[string]any) (any, error) {
	// Tool dispatch must not trust a cached legacy classification. Some tools
	// return metadata without passing through the per-file allowOn gate.
	if p.Agent != nil && p.Agent.RestrictionProfile == "" {
		if err := rejectWorkspaceLegacyAccess(s.DB, *p.Agent); err != nil {
			return nil, err
		}
		var current Agent
		if err := s.DB.Where("id = ? AND owner = ?", p.Agent.ID, p.UserID).First(&current).Error; err != nil || current.RestrictionProfile != "" {
			return nil, code("forbidden")
		}
	}
	if err := s.serviceIdentityGuard(p, "mcp."+name); err != nil {
		return nil, err
	}
	for _, field := range []string{"tenant_context", "space_context"} {
		if v, ok := args[field]; ok {
			value, valid := v.(string)
			if !valid {
				return nil, code("invalid_context")
			}
			if value != "" && !s.governanceActive() {
				return nil, code("context_unavailable")
			}
		}
	}
	if s.governanceActive() {
		req, _ := http.NewRequest("POST", "/mcp", nil)
		tenant, _ := args["tenant_context"].(string)
		space, _ := args["space_context"].(string)
		req.Header.Set("X-DriveBridge-Tenant", defaultString(tenant, p.TenantID))
		req.Header.Set("X-DriveBridge-Space", defaultString(space, p.ScopeSpace))
		var contextErr error
		p, contextErr = s.bindContext(p, req)
		if contextErr != nil {
			return nil, contextErr
		}
		if err := s.governanceGuard(p, "mcp."+name, req); err != nil {
			return nil, err
		}
		if p.ScopeSpace != "" && (name == "create_agent" || strings.HasPrefix(name, "users_") || name == "usage_scan" && p.Agent == nil) {
			return nil, code("forbidden")
		}
	}
	text := func(k string) string { v, _ := args[k].(string); return v }
	if p.Agent != nil && p.Agent.RestrictionProfile != "" {
		if p.Agent.RestrictionProfile != "personal_workspace_v1" {
			return nil, code("forbidden")
		}
		switch name {
		case "capabilities":
			return s.workspaceCapabilities(p)
		case "files":
			return s.searchWorkspaceFiles(p, text("space_id"), "", "", "", "", false)
		case "search":
			return s.searchWorkspaceFiles(p, text("space_id"), text("query"), text("path"), text("mime_type"), text("after"), false)
		case "stat":
			if text("ref") != "" && (text("file_id") != "" || text("version_id") != "") {
				return nil, code("invalid_request")
			}
			id, version, err := s.fileIDOrRef(text)
			if err != nil {
				return nil, err
			}
			if id == "" {
				return nil, code("invalid_request")
			}
			return s.workspaceFileVersionStat(p, id, version)
		case "operation_get":
			return s.workspaceOperationRecovery(p, text("operation_id"))
		case "complete_upload":
			op, err := s.completeAuthorized(ctx, p, text("operation_id"), true)
			if err != nil {
				return nil, err
			}
			return workspaceOperationProgress(op), nil
		default:
			return nil, code("forbidden")
		}
	}
	text2 := func(m map[string]any, k string) string { v, _ := m[k].(string); return v }
	// Ordinary shared-instance users receive references and operation receipts,
	// never an index of another user's files. Keep this guard in the application
	// service as well as the Gateway's tools/list projection: guessed tool names
	// must fail closed at the owner boundary.
	fullSpaceAgent := p.Agent != nil && p.Agent.Prefix == ""
	if !p.Admin && !fullSpaceAgent && isEnumerationTool(name) && !(s.governanceActive() && p.ScopeSpace != "") {
		return nil, code("forbidden")
	}
	if name == "usage_scan" {
		if _, e := s.insightScope(p, text("space_id"), ""); e != nil {
			return nil, e
		}
		if text("space_id") == "" {
			return nil, code("invalid_space")
		}
		if p.Agent != nil {
			var current Agent
			if s.DB.First(&current, "id = ? AND owner = ?", p.Agent.ID, p.dataOwner()).Error != nil || current.Prefix != "" {
				return nil, code("forbidden")
			}
		}
		if e := s.Insights.Log(s.auditEvent(p, "scan.start", "", "", "intent", "mcp", insights.ID())); e != nil {
			return nil, e
		}
		scanPrincipal := p
		scanPrincipal.Agent = nil
		return s.scanSpace(ctx, scanPrincipal, text("space_id"), text("resume_id"))
	}
	if strings.HasPrefix(name, "usage_") || name == "audit_events" {
		scope, e := s.insightScope(p, text("space_id"), text("connection_id"))
		if e != nil {
			return nil, e
		}
		f := insights.Filter{From: text("from"), To: text("to"), Actor: text("actor"), Caller: text("caller"), Direction: text("direction"), Purpose: text("purpose")}
		switch name {
		case "usage_summary":
			return s.Insights.Summary(scope, f)
		case "usage_series":
			return s.Insights.Series(scope, f)
		case "usage_breakdown":
			return s.Insights.Breakdown(scope, f, defaultString(text("group"), "space"))
		case "usage_scans":
			return s.Insights.Scans(scope)
		case "audit_events":
			return s.Insights.Audit(scope, insights.AuditFilter{From: f.From, To: f.To, Action: text("action"), Outcome: text("outcome")})
		}
	}
	switch name {
	case "input.bind_reference":
		for _, field := range []string{"binding_id", "consumer_request_ref"} {
			if v, present := args[field]; present {
				if _, ok := v.(string); !ok {
					return nil, code("invalid_handoff")
				}
			}
		}
		q := BindReferenceRequest{BindingID: text("binding_id"), Ref: text("ref"), Consumer: text("consumer"), ProjectRef: text("project_ref"), Purpose: text("purpose"), ExpectedSHA256: text("expected_sha256"), ConsumerRequestRef: text("consumer_request_ref")}
		if raw, present := args["request_expires_at"]; present {
			value, ok := raw.(string)
			expiry, err := time.Parse(time.RFC3339, value)
			if !ok || err != nil {
				return nil, code("invalid_handoff")
			}
			q.RequestExpiresAt = &expiry
		}
		grant, err := s.BindReference(p, q, text("idempotency_key"))
		if err != nil {
			return nil, err
		}
		return map[string]any{"grant_id": grant.ID, "ref": text("ref"), "consumer": grant.Consumer, "project_ref": grant.ProjectRef, "purpose": grant.Purpose, "sha256": grant.Digest, "expires_at": grant.Expires, "status": "ready_for_owner_acquisition", "binding_id": grant.BindingID, "cancelled": grant.Cancelled, "request_expires_at": grant.RequestExpires}, nil
	case "input.handoff_status", "input.handoff_renew", "input.handoff_cancel":
		var grant HandoffGrant
		var err error
		if name == "input.handoff_status" {
			grant, err = s.GetHandoff(p, text("grant_id"))
		} else {
			grant, err = s.ChangeHandoff(p, text("grant_id"), strings.TrimPrefix(name, "input.handoff_"))
		}
		if err != nil {
			return nil, err
		}
		state := "ready"
		if grant.Cancelled {
			state = "cancelled"
		} else if !time.Now().Before(grant.Expires) {
			state = "expired"
		}
		return map[string]any{"handoff": grant, "state": state, "recovery_tool": "input.handoff_status", "byte_transport": "authenticated HTTP by the bound owner adapter", "instruction": "Inspect the original target input before renewal; never renew a completed or cancelled input. A grant ID is not a credential."}, nil
	case "create_upload":
		size, ok := args["size_bytes"].(float64)
		if !ok || size != float64(int64(size)) {
			return nil, code("invalid_upload")
		}
		op, e := s.CreateUpload(p, UploadRequest{Caller: text("caller"), TaskRef: text("task_ref"), SpaceID: text("space_id"), Path: text("path"), Size: int64(size), SHA256: text("sha256"), FileID: text("file_id"), ExpectedVersion: text("expected_version"), EntitlementID: text("entitlement_id")}, text("idempotency_key"))
		if e != nil {
			return nil, e
		}
		return map[string]any{"operation": op, "part_size": PartSize, "part_path": "/api/v1/operations/" + op.ID + "/parts/{index}", "authorization": "same_scoped_agent_token"}, nil
	case "complete_upload":
		return s.Complete(ctx, p, text("operation_id"))
	case "runtime_status":
		// Operator-only read surface; the tool handler and HTTP route call
		// the same application verdict. Rebuild is deliberately NOT an MCP
		// tool: it stays a terminal/HTTP operator action.
		return s.RuntimeStatusView(p)
	case "capability_check":
		readiness, e := s.CheckCapability(p, text("action"), text("space_id"), text("ref"))
		if e != nil {
			return nil, e
		}
		// The byte-transport note is additive guidance for pure MCP clients:
		// upload readiness never requires reading a local path remotely.
		out := map[string]any{"readiness": readiness, "byte_transport": "authenticated_http", "operation_read_back": "operation_get"}
		return out, nil
	case "capabilities":
		ext, e := s.CapabilityExtension(p, "")
		if e != nil {
			return nil, e
		}
		doc := map[string]any{"tools": ToolDefinitions(), "part_size": PartSize, "max_bytes": s.Config.MaxBytes, "instance_id": s.Instance, "governance_enabled": s.governanceActive(), "tenant_id": p.TenantID, "context_headers": []string{"X-DriveBridge-Tenant", "X-DriveBridge-Space"}, "governance": "/api/v1/governance", "public_base_url": s.Config.Origin, "local_file_access": false, "byte_transport": "authenticated HTTP", "auth": "scoped_agent_token", "service_identity": s.serviceIdentityCapability(p)}
		for k, v := range ext {
			doc[k] = v
		}
		return doc, nil
	case "spaces":
		out := []Space{}
		q := s.DB.Where("owner = ?", p.dataOwner())
		if p.Agent != nil {
			q = q.Where("id = ?", p.Agent.SpaceID)
		}
		q.Find(&out)
		return out, nil
	case "search":
		return s.SearchFiles(p, text("space_id"), text("query"), text("path"), text("mime_type"), text("after"), false)
	case "trash", "restore":
		id := text("file_id")
		if text("ref") != "" {
			var err error
			id, _, err = s.resolveRef(text("ref"))
			if err != nil {
				return nil, err
			}
		}
		if err := s.Trash(p, id, name == "restore"); err != nil {
			return nil, err
		}
		return map[string]bool{name: true}, nil
	case "files":
		if _, e := s.space(p, text("space_id")); e != nil {
			return nil, e
		}
		all := []File{}
		s.DB.Where("owner = ? AND space_id = ? AND trashed = ?", p.dataOwner(), text("space_id"), false).Find(&all)
		out := []File{}
		for _, f := range all {
			if s.allow(p, f.SpaceID, f.Path, "read") == nil {
				out = append(out, f)
			}
		}
		return out, nil
	case "prepare_reference":
		fileID, version, e := s.fileIDOrRef(text)
		if e != nil {
			return nil, e
		}
		f, e := s.file(p, fileID, "read")
		if e != nil {
			return nil, e
		}
		if f.Trashed {
			return nil, code("file_trashed")
		}
		var v Version
		if s.DB.First(&v, "id = ? AND owner = ? AND file_id = ?", version, p.dataOwner(), f.ID).Error != nil {
			return nil, code("not_found")
		}
		if sp, ok := s.spaceFor(p, f.SpaceID); ok && sp.Encrypted {
			// Reference handoff depends on storage-side direct links, which
			// encrypted spaces no longer expose; acquisition goes through the
			// server-side decryption stream instead.
			return nil, code("encrypted_space_reference_rejected")
		}
		f.VersionID = v.ID
		return map[string]any{"ref": s.FileRef(f), "version": v, "status": "ready_for_authorized_http_acquisition"}, nil
	case "stat":
		if text("ref") != "" {
			return s.PinnedFile(p, text("ref"))
		}
		fileID, _, e := s.fileIDOrRef(text)
		if e != nil {
			return nil, e
		}
		f, e := s.file(p, fileID, "read")
		if e != nil {
			return nil, e
		}
		return map[string]any{"file": f, "ref": s.FileRef(f)}, nil
	case "operations":
		all := []Operation{}
		s.DB.Where("owner = ?", p.dataOwner()).Order("updated DESC").Limit(200).Find(&all)
		out := []Operation{}
		for _, op := range all {
			if s.allow(p, op.SpaceID, op.Path, "read") == nil {
				out = append(out, op)
			}
		}
		return out, nil
	case "inbox":
		q := s.DB.Where("owner = ? AND updated >= ?", p.dataOwner(), time.Now().UTC().Add(-30*24*time.Hour))
		if v := text("status"); v != "" {
			q = q.Where("status = ?", v)
		}
		if v := text("caller"); v != "" {
			q = q.Where("caller = ?", v)
		}
		if v := text("space_id"); v != "" {
			q = q.Where("space_id = ?", v)
		}
		var all []Operation
		if q.Order("updated DESC").Limit(200).Find(&all).Error != nil {
			return nil, code("database_unavailable")
		}
		out := []Operation{}
		for _, op := range all {
			if s.allow(p, op.SpaceID, op.Path, "read") == nil {
				out = append(out, op)
			}
		}
		return out, nil
	case "archive":
		return s.ArchiveVersion(ctx, p, ArchiveRequest{SourceFileID: text("source_file_id"), SourceVersionID: text("source_version_id"), TargetSpaceID: text("target_space_id"), TargetPath: text("target_path")}, text("idempotency_key"))
	case "operation_get":
		op, e := s.operation(p, text("operation_id"), "read")
		if e != nil {
			return nil, e
		}
		parts := []Part{}
		s.DB.Where("operation_id = ?", op.ID).Find(&parts)
		indexes := []int{}
		for _, v := range parts {
			indexes = append(indexes, v.Index)
		}
		return map[string]any{"operation": op, "received_parts": indexes}, nil
	case "cancel":
		return s.Cancel(p, text("operation_id"))
	case "users_create":
		entries := []UserImport{}
		if raw, ok := args["users"].([]any); ok {
			for _, item := range raw {
				m, _ := item.(map[string]any)
				entry := UserImport{Name: text2(m, "name"), Key: text2(m, "key")}
				if days, ok := m["days"].(float64); ok {
					entry.Days = int(days)
				} else if days, ok := m["days"].(int); ok {
					entry.Days = days
				}
				entry.Admin, _ = m["admin"].(bool)
				entries = append(entries, entry)
			}
		}
		return s.CreateUserUsers(p, entries)
	case "users_list":
		if p.Key == nil || !p.Key.Admin {
			return nil, code("forbidden")
		}
		return s.ListUserKeys(), nil
	case "users_revoke":
		if p.Key == nil || !p.Key.Admin {
			return nil, code("forbidden")
		}
		return map[string]any{"revoked": true}, s.RevokeUserKey(text("key_id"))
	case "resume":
		op, e := s.Resume(ctx, p, text("operation_id"))
		if e != nil {
			return nil, e
		}
		if op.Status == "awaiting_upload" {
			return map[string]any{"operation": op, "next_action": "send_missing_parts_over_http"}, nil
		}
		return op, nil
	case "upload_batch_create":
		return s.CreateUploadBatch(p, UploadBatchRequest{SpaceID: text("space_id"), ConflictPolicy: text("conflict_policy")}, text("idempotency_key"))
	case "upload_batch_add_items":
		items := []UploadBatchItemInput{}
		if raw, ok := args["items"].([]any); ok {
			for _, item := range raw {
				m, _ := item.(map[string]any)
				entry := UploadBatchItemInput{RelativePath: text2(m, "relative_path"), FileID: text2(m, "file_id"), ExpectedVersion: text2(m, "expected_version"), SHA256: text2(m, "sha256")}
				entry.IsDir, _ = m["is_dir"].(bool)
				if size, ok := m["size_bytes"].(float64); ok {
					entry.Size = int64(size)
				}
				items = append(items, entry)
			}
		}
		view, e := s.AddUploadBatchItems(p, text("batch_id"), items)
		if e != nil {
			return nil, e
		}
		return map[string]any{"batch": view, "byte_transport": "authenticated_http_parts"}, nil
	case "upload_batch_seal":
		return s.SealUploadBatch(p, text("batch_id"))
	case "upload_batch_get":
		limit := 50
		if v, err := strconv.Atoi(text("limit")); err == nil && v >= 1 && v <= 200 {
			limit = v
		}
		return s.GetUploadBatch(p, text("batch_id"), text("cursor"), limit)
	case "upload_batch_pause":
		return s.ControlUploadBatch(p, text("batch_id"), "pause")
	case "upload_batch_resume":
		return s.ControlUploadBatch(p, text("batch_id"), "resume")
	case "upload_batch_cancel":
		return s.ControlUploadBatch(p, text("batch_id"), "cancel")
	case "operation_lookup":
		original, _ := json.Marshal(args["original_request"])
		return s.LookupOperation(p, text("action"), text("idempotency_key"), json.RawMessage(original))
	case "upload_session":
		return s.UploadSessionFor(p, text("operation_id"))
	case "retention_hold":
		return s.CreateRetentionHold(p, RetentionHoldRequest{FileID: text("file_id"), VersionID: text("version_id"), Purpose: text("purpose"), SourceRef: text("source_ref"), ExpiresAt: text("expires_at")}, text("idempotency_key"))
	case "retention_hold_get":
		return s.GetRetentionHold(p, text("hold_id"))
	case "retention_preview":
		return s.PreviewRetention(p, text("category"))
	case "changes_v2":
		limit := 50
		if v, err := strconv.Atoi(text("limit")); err == nil && v >= 1 && v <= 200 {
			limit = v
		}
		return s.ReadChanges(p, text("space_id"), text("cursor"), limit)
	case "change_subscription_create":
		return s.CreateChangeSubscription(p, CreateChangeSubscriptionRequest{ConsumerID: text("consumer_id"), SpaceID: text("space_id")})
	case "change_subscription_ack":
		checkpoint := int64(0)
		if v, e := strconv.ParseInt(text("checkpoint"), 10, 64); e == nil {
			checkpoint = v
		}
		return s.AckChanges(p, text("subscription_id"), AckChangesRequest{Checkpoint: checkpoint})
	case "automation_intent_get":
		return s.GetAutomationIntent(p, text("intent_id"))
	case "organize_plan":
		var q CreateOrganizationPlanRequest
		q.SourceSnapshotID = text("source_snapshot_id")
		q.PolicyID = text("policy_id")
		if raw, ok := args["refs"].([]any); ok {
			for _, item := range raw {
				if ref, ok := item.(string); ok {
					q.Refs = append(q.Refs, ref)
				}
			}
		}
		if raw, ok := args["actions"].([]any); ok {
			for _, item := range raw {
				m, _ := item.(map[string]any)
				q.Actions = append(q.Actions, OrganizationAction{Kind: text2(m, "kind"), Namespace: text2(m, "namespace"), Value: text2(m, "value"), TargetPrefix: text2(m, "target_prefix")})
			}
		}
		return s.CreateOrganizationPlan(p, q, text("idempotency_key"))
	case "organize_plan_get":
		limit := 50
		if v, err := strconv.Atoi(text("limit")); err == nil && v >= 1 && v <= 200 {
			limit = v
		}
		return s.GetOrganizationPlan(p, text("plan_id"), text("cursor"), limit)
	case "organize_apply":
		rev := uint64(0)
		if v, e := strconv.ParseUint(text("expected_revision"), 10, 64); e == nil {
			rev = v
		}
		return s.ExecuteOrganizationPlan(p, text("plan_id"), ExecuteOrganizationPlanRequest{ExpectedRevision: rev})
	case "organize_compensation_plan":
		return s.CreateCompensationPlan(p, text("plan_id"), text("idempotency_key"))
	case "delivery_prepare":
		rev := uint64(0)
		if v, e := strconv.ParseUint(text("expected_share_revision"), 10, 64); e == nil {
			rev = v
		}
		return s.PrepareFileDelivery(p, text("share_id"), PrepareFileDeliveryRequest{SnapshotID: text("snapshot_id"), ExpectedShareRevision: rev}, text("idempotency_key"))
	case "delivery_get":
		rev := uint64(0)
		if v, e := strconv.ParseUint(text("manifest_revision"), 10, 64); e == nil {
			rev = v
		}
		return s.GetFileDelivery(p, text("share_id"), rev)
	case "delivery_receipt":
		rev := uint64(0)
		if v, e := strconv.ParseUint(text("manifest_revision"), 10, 64); e == nil {
			rev = v
		}
		return s.RecordDeliveryReceipt(p, text("share_id"), DeliveryReceiptRequest{ManifestRevision: rev, Kind: text("kind"), Digest: text("digest"), Note: text("note")})
	case "content_bindings_list":
		return s.ListContentBindings(p, text("space_id"))
	case "content_binding_create":
		return s.CreateContentBinding(p, CreateContentBindingRequest{Ref: text("ref"), ManjuConnectionID: text("manju_connection_id"), ProjectRef: text("project_ref"), Purpose: text("purpose")}, text("idempotency_key"))
	case "content_binding_get":
		return s.GetContentBinding(p, text("binding_id"))
	case "content_process":
		rev := uint64(0)
		if v, e := strconv.ParseUint(text("policy_revision"), 10, 64); e == nil {
			rev = v
		}
		return s.ProcessContent(ctx, p, ProcessContentRequest{BindingID: text("binding_id"), PolicyRevision: rev}, text("idempotency_key"))
	case "content_query":
		q := ContentQueryRequest{Mode: text("mode"), Text: text("text"), ImageRef: text("image_ref"), EvidenceRequired: text("evidence_required") == "true", ManjuConnectionID: text("manju_connection_id")}
		if raw, ok := args["space_ids"].([]any); ok {
			for _, item := range raw {
				if id, ok := item.(string); ok {
					q.SpaceIDs = append(q.SpaceIDs, id)
				}
			}
		}
		if raw, ok := args["modalities"].([]any); ok {
			for _, item := range raw {
				if m, ok := item.(string); ok {
					q.Modalities = append(q.Modalities, m)
				}
			}
		}
		if v, ok := args["limit"].(float64); ok {
			q.Limit = int(v)
		}
		return s.QueryContent(ctx, p, q)
	case "analytics_query":
		var q AnalyticsQueryRequest
		if raw, ok := args["filters"].([]any); ok {
			for _, item := range raw {
				m, _ := item.(map[string]any)
				value, _ := m["value"]
				q.Filters = append(q.Filters, AnalyticsFilter{Field: text2(m, "field"), Op: text2(m, "op"), Value: value})
			}
		}
		for _, pair := range []struct {
			key  string
			dest *[]string
		}{{"dimensions", &q.Dimensions}, {"metrics", &q.Metrics}} {
			if raw, ok := args[pair.key].([]any); ok {
				for _, item := range raw {
					if v, ok := item.(string); ok {
						*pair.dest = append(*pair.dest, v)
					}
				}
			}
		}
		q.Dataset = text("dataset")
		q.Consistency = text("consistency")
		q.Cursor = text("cursor")
		if v, ok := args["limit"].(float64); ok {
			q.Limit = int(v)
		}
		return s.QueryAnalytics(p, q)
	case "analytics_query_get":
		return s.GetAnalyticsQuery(p, text("query_id"))
	case "analytics_snapshot":
		var q AnalyticsSnapshotRequest
		if raw, ok := args["filters"].([]any); ok {
			for _, item := range raw {
				m, _ := item.(map[string]any)
				value, _ := m["value"]
				q.Filters = append(q.Filters, AnalyticsFilter{Field: text2(m, "field"), Op: text2(m, "op"), Value: value})
			}
		}
		if raw, ok := args["fields"].([]any); ok {
			for _, item := range raw {
				if v, ok := item.(string); ok {
					q.Fields = append(q.Fields, v)
				}
			}
		}
		q.Dataset = text("dataset")
		return s.CreateAnalyticsSnapshot(ctx, p, q, text("idempotency_key"))
	case "analytics_snapshot_query":
		var q AnalyticsQueryRequest
		if raw, ok := args["filters"].([]any); ok {
			for _, item := range raw {
				m, _ := item.(map[string]any)
				value, _ := m["value"]
				q.Filters = append(q.Filters, AnalyticsFilter{Field: text2(m, "field"), Op: text2(m, "op"), Value: value})
			}
		}
		for _, pair := range []struct {
			key  string
			dest *[]string
		}{{"dimensions", &q.Dimensions}, {"metrics", &q.Metrics}} {
			if raw, ok := args[pair.key].([]any); ok {
				for _, item := range raw {
					if v, ok := item.(string); ok {
						*pair.dest = append(*pair.dest, v)
					}
				}
			}
		}
		q.Dataset = text("dataset")
		return s.QueryAnalyticsSnapshot(p, text("snapshot_id"), q)
	case "transfer_plan":
		var q CreateTransferPlanRequest
		if raw, ok := args["sources"].([]any); ok {
			for _, item := range raw {
				m, _ := item.(map[string]any)
				q.Sources = append(q.Sources, TransferSourceInput{Ref: text2(m, "ref"), SHA256: text2(m, "sha256")})
			}
		}
		q.TargetSpaceID = text("target_space_id")
		q.TargetPath = text("target_path")
		q.Mode = text("mode")
		q.ConflictPolicy = text("conflict_policy")
		q.CrossSpaceConfirmed = text("cross_space_confirmed") == "true"
		return s.CreateTransferPlan(p, q, text("idempotency_key"))
	case "transfer_plan_get":
		limit := 50
		if v, err := strconv.Atoi(text("limit")); err == nil && v >= 1 && v <= 200 {
			limit = v
		}
		return s.GetTransferPlan(p, text("plan_id"), text("cursor"), limit)
	case "transfer_execute":
		rev := uint64(0)
		if v, e := strconv.ParseUint(text("expected_revision"), 10, 64); e == nil {
			rev = v
		}
		return s.ExecuteTransferPlan(ctx, p, text("plan_id"), ExecuteTransferPlanRequest{ExpectedRevision: rev})
	case "files_query":
		var q FileQueryRequest
		if raw, ok := args["filters"].([]any); ok {
			for _, item := range raw {
				m, _ := item.(map[string]any)
				value, _ := m["value"]
				q.Filters = append(q.Filters, FileQueryFilter{Field: text2(m, "field"), Op: text2(m, "op"), Value: value})
			}
		}
		if raw, ok := args["sort"].([]any); ok {
			for _, item := range raw {
				m, _ := item.(map[string]any)
				q.Sort = append(q.Sort, FileQuerySort{Field: text2(m, "field"), Direction: text2(m, "direction")})
			}
		}
		if raw, ok := args["space_ids"].([]any); ok {
			for _, item := range raw {
				if id, ok := item.(string); ok {
					q.SpaceIDs = append(q.SpaceIDs, id)
				}
			}
		}
		q.Mode = text("mode")
		q.Cursor = text("cursor")
		if v, ok := args["limit"].(float64); ok {
			q.Limit = int(v)
		}
		if q.Mode == "duplicates" {
			return s.QueryDuplicateCandidates(p, q.SpaceIDs)
		}
		return s.QueryFiles(p, q)
	case "file_labels_set":
		labels := []string{}
		if raw, ok := args["labels"].([]any); ok {
			for _, item := range raw {
				if l, ok := item.(string); ok {
					labels = append(labels, l)
				}
			}
		}
		return s.SetFileLabels(p, text("file_id"), SetFileLabelsRequest{Namespace: text("namespace"), Labels: labels, ExpectedRevision: text("expected_revision")})
	case "file_collections_list":
		return s.ListFileCollections(p, text("space_id"))
	case "file_collection_create":
		rule := json.RawMessage(text("rule"))
		return s.CreateFileCollection(p, CreateFileCollectionRequest{SpaceID: text("space_id"), Name: text("name"), Kind: text("kind"), Scope: text("scope"), Rule: rule}, text("idempotency_key"))
	case "collection_members_set":
		include := []string{}
		exclude := []string{}
		for _, pair := range []struct {
			key  string
			dest *[]string
		}{{"include", &include}, {"exclude", &exclude}} {
			if raw, ok := args[pair.key].([]any); ok {
				for _, item := range raw {
					if id, ok := item.(string); ok {
						*pair.dest = append(*pair.dest, id)
					}
				}
			}
		}
		rev := uint64(0)
		if v, e := strconv.ParseUint(text("expected_revision"), 10, 64); e == nil {
			rev = v
		}
		return s.SetCollectionMembers(p, text("collection_id"), SetCollectionMembersRequest{ExpectedRevision: rev, Include: include, Exclude: exclude})
	case "collection_snapshot":
		return s.CreateFileSnapshot(p, text("collection_id"))
	case "saved_searches_list":
		return s.SavedFileQueriesList(p)
	case "saved_search_create":
		var q FileQueryRequest
		if e := json.Unmarshal([]byte(text("query")), &q); e != nil {
			return nil, code("invalid_query")
		}
		name := text("name")
		if name == "" {
			if n, ok := args["name"].(string); ok {
				name = n
			}
		}
		return s.SavedFileQueryCreate(p, name, q)
	case "cloud_entries":
		limit := 50
		if v, err := strconv.Atoi(text("limit")); err == nil && v >= 1 && v <= 200 {
			limit = v
		}
		return s.ListCloudEntries(ctx, p, text("connection_id"), text("root_id"), text("parent_ref"), text("cursor"), limit)
	case "cloud_entry_stat":
		return s.StatCloudEntry(ctx, p, text("connection_id"), text("entry_ref"), text("expected_revision"))
	case "cloud_changes":
		limit := 50
		if v, err := strconv.Atoi(text("limit")); err == nil && v >= 1 && v <= 200 {
			limit = v
		}
		return s.ReadCloudChanges(ctx, p, text("connection_id"), text("root_id"), text("cursor"), limit)
	case "cloud_scan":
		budget := int64(0)
		if v, err := strconv.Atoi(text("budget")); err == nil && v > 0 {
			budget = int64(v)
		}
		return s.ScanCloudConnection(ctx, p, text("connection_id"), CloudScanRequest{RootID: text("root_id"), Budget: budget}, text("idempotency_key"))
	case "file_threads_list":
		limit := 50
		if v, err := strconv.Atoi(text("limit")); err == nil && v >= 1 && v <= 200 {
			limit = v
		}
		return s.ListFileThreads(p, text("file_id"), text("cursor"), limit)
	case "file_thread_create":
		return s.CreateFileThread(p, text("file_id"), text("version_id"), text("body"), text("idempotency_key"))
	case "thread_reply":
		return s.ReplyFileThread(p, text("thread_id"), text("body"), text("idempotency_key"))
	case "thread_message_edit":
		rev, e := strconv.ParseUint(text("expected_revision"), 10, 64)
		if e != nil {
			return nil, code("invalid_request")
		}
		return s.EditThreadMessage(p, text("message_id"), text("body"), rev)
	case "thread_message_delete":
		rev, e := strconv.ParseUint(text("expected_revision"), 10, 64)
		if e != nil {
			return nil, code("invalid_request")
		}
		return s.DeleteThreadMessage(p, text("message_id"), rev)
	case "thread_resolve":
		return s.SetThreadResolution(p, text("thread_id"), true)
	case "thread_reopen":
		return s.SetThreadResolution(p, text("thread_id"), false)
	case "private_note_get":
		return s.PrivateFileNote(p, text("file_id"), "", 0, false)
	case "private_note_set":
		rev := uint64(0)
		if raw := text("expected_revision"); raw != "" {
			parsed, e := strconv.ParseUint(raw, 10, 64)
			if e != nil {
				return nil, code("invalid_request")
			}
			rev = parsed
		}
		return s.PrivateFileNote(p, text("file_id"), text("body"), rev, true)
	case "private_note_publish_preview":
		return s.PreviewNotePublication(p, text("file_id"))
	}
	return nil, code("unknown_tool")
}

func isEnumerationTool(name string) bool {
	if name == "files" || name == "spaces" || name == "search" || name == "changes" || name == "inbox" || name == "operations" {
		return true
	}
	return strings.HasPrefix(name, "usage_") || name == "audit_events"
}

func retryableError(err error) bool {
	switch safeCode(err) {
	case "backend_unavailable", "throttled", "transfer_capacity_unavailable", "database_unavailable", "access_unavailable":
		return true
	default:
		return false
	}
}

// fileIDOrRef accepts either a local file id or a drivebridge:// reference;
// references pinned to another deployment instance are rejected.
func (s *Service) fileIDOrRef(text func(string) string) (string, string, error) {
	if id := text("file_id"); id != "" {
		return id, text("version_id"), nil
	}
	return s.resolveRef(text("ref"))
}
