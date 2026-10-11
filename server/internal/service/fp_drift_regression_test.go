package service_test

// Regression lights for the five backend-arbitrated client↔backend contract
// drifts found by the integration smoke (D3/D5/D7/D10/D12). Every red light
// uses the real request shape the client sent in the drift report, carried
// over raw HTTP so the assertion covers the wire contract itself.

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

// returns the status plus the decoded envelope (data or error code).
func driftRaw(t *testing.T, h *human, method, path, key string, body any) (int, map[string]any) {
	t.Helper()
	var reader io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		reader = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, h.base+"/api/v1"+path, reader)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Origin", h.base)
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	res, e := h.http.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	var env map[string]any
	if e := json.NewDecoder(res.Body).Decode(&env); e != nil {
		t.Fatalf("invalid envelope for %s %s: %v", method, path, e)
	}
	return res.StatusCode, env
}

// D3: analytics sort must be its own {field,direction} clause. The client's
// live-shape query with sort:[{field,direction}] was rejected 400 by
// DisallowUnknownFields because sort reused the {field,op,value} filter
// shape.
func TestFPDriftAnalyticsSortClauseShape(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	status, env := driftRaw(t, h, "POST", "/analytics/queries", "", map[string]any{
		"dataset":     "file_current",
		"metrics":     []string{"file_count"},
		"sort":        []map[string]any{{"field": "mime_type", "direction": "desc"}},
		"consistency": "live",
	})
	if status != 200 {
		t.Fatalf("design sort shape {field,direction} must answer 200, got %d %v", status, env)
	}
	// The filter shape inside sort stays rejected: op/value are unknown
	// fields on a sort clause.
	status, env = driftRaw(t, h, "POST", "/analytics/queries", "", map[string]any{
		"dataset":     "file_current",
		"metrics":     []string{"file_count"},
		"sort":        []map[string]any{{"field": "mime_type", "op": "eq", "value": "x"}},
		"consistency": "live",
	})
	if status != 400 {
		t.Fatalf("filter shape in sort must stay 400, got %d %v", status, env)
	}
	// A direction outside asc|desc is invalid_query.
	status, env = driftRaw(t, h, "POST", "/analytics/queries", "", map[string]any{
		"dataset":     "file_current",
		"metrics":     []string{"file_count"},
		"sort":        []map[string]any{{"field": "mime_type", "direction": "sideways"}},
		"consistency": "live",
	})
	if status != 400 {
		t.Fatalf("unknown sort direction must be 400, got %d %v", status, env)
	}
}

// D5: operations/lookup must cover every implemented creation action. An
// unknown-to-the-old-switch action answered 400 invalid_request; the design
// contract answers found:false so the same key and original request may
// replay safely.
func TestFPDriftOperationLookupActionCoverage(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	ref := "drivebridge://" + f.Service.Instance + "/file/none/version/none"
	probes := map[string]map[string]any{
		"analytics_query":     {"dataset": "file_current", "metrics": []string{"file_count"}, "consistency": "live"},
		"content_query":       {"mode": "exact", "space_ids": []string{h.space.ID}},
		"organize_plan":       {"refs": []string{ref}, "actions": []map[string]any{{"kind": "add_label", "namespace": "rule", "value": "reference"}}},
		"organize_apply":      {"plan_id": "missing", "expected_revision": 1},
		"delivery_prepare":    {"share_id": "missing", "snapshot_id": "missing", "expected_share_revision": 1},
		"cloud_scan":          {"connection_id": "missing", "root_id": "missing", "budget": 100},
		"transfer_plan":       {"sources": []map[string]any{{"ref": ref}}, "target_space_id": h.space.ID, "target_path": "dest", "mode": "copy", "conflict_policy": "fail"},
		"private_note_set":    {"file_id": "missing", "body": "note", "expected_revision": 0},
		"thread":              {"file_id": "missing", "version_id": "", "body": "hello"},
		"thread_reply":        {"thread_id": "missing", "body": "reply"},
		"upload_batch_create": {"space_id": h.space.ID, "conflict_policy": "fail"},
		"upload":              {"space_id": h.space.ID, "path": "a.txt", "size_bytes": 3},
	}
	for action, original := range probes {
		status, env := driftRaw(t, h, "POST", "/operations/lookup", "", map[string]any{
			"action":           action,
			"idempotency_key":  "drift-probe-" + action,
			"original_request": original,
		})
		if status != 200 {
			t.Fatalf("lookup for %s must answer 200 found:false, got %d %v", action, status, env)
		}
		data, _ := env["data"].(map[string]any)
		if found, _ := data["found"].(bool); found {
			t.Fatalf("lookup for %s with an unused key must not claim a record: %v", action, data)
		}
	}
}

// D7: organization-plan actions follow the design shape {kind, namespace,
// value} with the move target in target_prefix. The old {kind, namespace,
// label, target} shape made the client's design-shaped request fail 400.
func TestFPDriftOrganizationPlanActionShape(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	ref := transferRef(t, f, c, h.space.ID, "drift-org/a.txt", "alpha")
	status, env := driftRaw(t, h, "POST", "/organization-plans", "drift-org-plan", map[string]any{
		"refs":    []string{ref},
		"actions": []map[string]any{{"kind": "add_label", "namespace": "rule", "value": "reference"}},
	})
	if status != 200 {
		t.Fatalf("design action shape {kind,namespace,value} must create a plan, got %d %v", status, env)
	}
	// Move actions carry the destination prefix in target_prefix.
	status, env = driftRaw(t, h, "POST", "/organization-plans", "drift-org-move", map[string]any{
		"refs":    []string{ref},
		"actions": []map[string]any{{"kind": "move", "target_prefix": "drift-org/archive"}},
	})
	if status != 200 {
		t.Fatalf("move action with target_prefix must create a plan, got %d %v", status, env)
	}
	// The old label/target field names are now unknown fields and stay 400.
	status, env = driftRaw(t, h, "POST", "/organization-plans", "drift-org-old", map[string]any{
		"refs":    []string{ref},
		"actions": []map[string]any{{"kind": "add_label", "namespace": "rule", "label": "reference"}},
	})
	if status != 400 {
		t.Fatalf("legacy label field must stay 400, got %d %v", status, env)
	}
}

// D10: upload-batch items carry source_fingerprint, an explicit null
// operation_id before scheduling and an explicit result_certainty. The old
// view returned only sha256 and omitted pending fields entirely.
func TestFPDriftUploadBatchItemFields(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	b := fpBatchCreate(t, c, h.space.ID, "fail", "drift-items-batch")
	sha := service.Hash("drift item")
	fpBatchAdd(t, c, b.BatchID, []sdk.UploadBatchItemInput{{RelativePath: "docs/drift.txt", Size: int64(len("drift item")), SHA256: sha}})
	status, env := driftRaw(t, h, "GET", "/upload-batches/"+b.BatchID+"/items?limit=10", "", nil)
	if status != 200 {
		t.Fatalf("items read status %d %v", status, env)
	}
	data, _ := env["data"].(map[string]any)
	items, _ := data["items"].([]any)
	if len(items) != 1 {
		t.Fatalf("one item expected: %v", data)
	}
	item, _ := items[0].(map[string]any)
	if got, _ := item["source_fingerprint"].(string); got != sha {
		t.Fatalf("item must carry source_fingerprint=%s, got %v", sha, item)
	}
	if opid, ok := item["operation_id"]; !ok || opid != nil {
		t.Fatalf("pending item operation_id must be explicit null, got %v", item)
	}
	if got, _ := item["result_certainty"].(string); got != "unknown" {
		t.Fatalf("pending item result_certainty must be unknown, got %v", item)
	}
}

// D12: expected_share_revision carries real optimistic semantics. The old
// short-circuit refused every non-zero value with unsupported; the design
// compares against the share's monotonic revision (equal continues, stale
// is version_conflict) and the revision joins the idempotency fingerprint.
func TestFPDriftDeliveryExpectedShareRevision(t *testing.T) {
	f, h, shareID, snapshotID := deliveryWorld(t, true)
	// The current revision prepares successfully.
	status, env := driftRaw(t, h, "POST", "/shares/"+shareID+"/delivery-manifests", "drift-dl-current", map[string]any{
		"snapshot_id": snapshotID, "expected_share_revision": 1,
	})
	if status != 200 {
		t.Fatalf("prepare with the current share revision must succeed, got %d %v", status, env)
	}
	// A member/permission change bumps the monotonic share revision; a
	// second share isolates the pure revision semantics from the revoked
	// state itself.
	var share2 struct {
		Share struct {
			ID string `json:"id"`
		} `json:"share"`
	}
	if status := h.call(t, "POST", "/shares", map[string]any{"name": "fixed2", "space_id": h.space.ID, "path": "dl", "mode": "fixed", "audience": "link", "days": 7}, &share2); status != 200 {
		t.Fatalf("second share status %d", status)
	}
	if e := f.Service.DB.Model(&service.Share{}).Where("id = ?", share2.Share.ID).Update("revision", 2).Error; e != nil {
		t.Fatal(e)
	}
	status, env = driftRaw(t, h, "POST", "/shares/"+share2.Share.ID+"/delivery-manifests", "drift-dl-stale", map[string]any{
		"snapshot_id": snapshotID, "expected_share_revision": 1,
	})
	if status != 409 {
		t.Fatalf("stale expected_share_revision must be 409 version_conflict, got %d %v", status, env)
	}
	if errField, _ := env["error"].(map[string]any); errField != nil {
		if codeValue, _ := errField["code"].(string); codeValue != "version_conflict" {
			t.Fatalf("stale revision code must be version_conflict, got %v", env)
		}
	}
	status, env = driftRaw(t, h, "POST", "/shares/"+share2.Share.ID+"/delivery-manifests", "drift-dl-fresh", map[string]any{
		"snapshot_id": snapshotID, "expected_share_revision": 2,
	})
	if status != 200 {
		t.Fatalf("prepare with the changed current revision must succeed, got %d %v", status, env)
	}
	// The revision joined the fingerprint: the same key with a different
	// revision is idempotency_conflict, never a silent second manifest.
	status, env = driftRaw(t, h, "POST", "/shares/"+share2.Share.ID+"/delivery-manifests", "drift-dl-fresh", map[string]any{
		"snapshot_id": snapshotID, "expected_share_revision": 1,
	})
	if status != 409 {
		t.Fatalf("same key different revision must conflict, got %d %v", status, env)
	}
	_ = fmt.Sprint
}

// D5 green coverage: keyed creation actions recover their original
// identities through lookup, and a drifted original_request under the same
// key is idempotency_conflict — never a second record.
func TestFPDriftOperationLookupRecoversKeyedActions(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	ctx := context.Background()

	// organize_plan: create through the SDK, recover through lookup with
	// the same business request.
	ref := transferRef(t, f, c, h.space.ID, "drift-keyed/a.txt", "alpha")
	planReq := sdk.CreateOrganizationPlanRequest{Refs: []string{ref}, Actions: []sdk.OrganizationAction{{Kind: "add_label", Namespace: "rule", Value: "reference"}}}
	plan, e := c.CreateOrganizationPlan(ctx, planReq, "drift-lookup-plan")
	if e != nil {
		t.Fatal(e)
	}
	original := map[string]any{"refs": []string{ref}, "actions": []map[string]any{{"kind": "add_label", "namespace": "rule", "value": "reference"}}}
	found, e := c.LookupOperation(ctx, "organize_plan", "drift-lookup-plan", original)
	if e != nil {
		t.Fatal(e)
	}
	if !found.Found || found.ResourceID != plan.PlanID || found.ResourceType != "organization_plan" || found.Status != "previewed" || found.ResultCertainty != "unknown" {
		t.Fatalf("organize_plan lookup wrong: %+v", found)
	}
	drifted := map[string]any{"refs": []string{ref}, "actions": []map[string]any{{"kind": "trash"}}}
	if _, e := c.LookupOperation(ctx, "organize_plan", "drift-lookup-plan", drifted); e == nil || e.Error() != "idempotency_conflict" {
		t.Fatalf("drifted organize_plan lookup must conflict, got %v", e)
	}

	// thread: the anchor operation identity comes back.
	upload := orgUpload(t, c, h.space.ID, "drift-keyed/b.txt", "beta")
	threadBody := "drift thread"
	thread, e := c.FileThreadCreate(ctx, upload.FileID, upload.VersionID, threadBody, "drift-lookup-thread")
	if e != nil {
		t.Fatal(e)
	}
	threadFound, e := c.LookupOperation(ctx, "thread", "drift-lookup-thread", map[string]any{"file_id": upload.FileID, "version_id": upload.VersionID, "body": threadBody})
	if e != nil {
		t.Fatal(e)
	}
	if !threadFound.Found || threadFound.OperationID != thread["thread_id"] || threadFound.ResultCertainty != "confirmed" {
		t.Fatalf("thread lookup wrong: %+v", threadFound)
	}

	_, dh, shareID, snapshotID := deliveryWorld(t, false)
	dc := mustSDK(t, dh)
	manifest := prepareDeliveryHTTP(t, dh, shareID, snapshotID, "drift-lookup-delivery")
	deliveryFound, e := dc.LookupOperation(ctx, "delivery_prepare", "drift-lookup-delivery", map[string]any{"share_id": shareID, "snapshot_id": snapshotID, "expected_share_revision": 1})
	if e != nil {
		t.Fatal(e)
	}
	if !deliveryFound.Found || deliveryFound.ResourceID != manifest.ManifestID || deliveryFound.ResourceType != "delivery_manifest" {
		t.Fatalf("delivery_prepare lookup wrong: %+v", deliveryFound)
	}
	if _, e := dc.LookupOperation(ctx, "delivery_prepare", "drift-lookup-delivery", map[string]any{"share_id": shareID, "snapshot_id": snapshotID, "expected_share_revision": 2}); e == nil || e.Error() != "idempotency_conflict" {
		t.Fatalf("delivery_prepare revision drift must conflict, got %v", e)
	}

	// transfer_plan: the surface-gated fixture carries its own world.
	tf := transferRig(t)
	th, _ := login(t, tf, "alice")
	tc := mustSDK(t, th)
	tref := transferRef(t, tf, tc, th.space.ID, "drift-keyed/c.txt", "gamma")
	transferReq := sdk.CreateTransferPlanRequest{Sources: []sdk.TransferSource{{Ref: tref}}, TargetSpaceID: th.space.ID, TargetPath: "drift-dest", Mode: "copy", ConflictPolicy: "fail"}
	transfer, e := tc.CreateTransferPlan(ctx, transferReq, "drift-lookup-transfer")
	if e != nil {
		t.Fatal(e)
	}
	transferFound, e := tc.LookupOperation(ctx, "transfer_plan", "drift-lookup-transfer", map[string]any{"sources": []map[string]any{{"ref": tref}}, "target_space_id": th.space.ID, "target_path": "drift-dest", "mode": "copy", "conflict_policy": "fail"})
	if e != nil {
		t.Fatal(e)
	}
	if !transferFound.Found || transferFound.ResourceID != transfer.PlanID || transferFound.ResourceType != "transfer_plan" {
		t.Fatalf("transfer_plan lookup wrong: %+v", transferFound)
	}
}
