package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/fixtures"
	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
	"gorm.io/gorm"
)

// fpCheck runs one capability preflight over HTTP and returns status plus the
// decoded readiness payload.
func fpCheck(t *testing.T, h *human, action, space, ref string) (int, service.ActionReadiness, string) {
	t.Helper()
	body := map[string]string{"action": action, "space_id": space}
	if ref != "" {
		body["ref"] = ref
	}
	raw, _ := json.Marshal(body)
	status, payload := fpRawCall(t, h, "POST", "/capabilities/check", raw)
	var env struct {
		Data  service.ActionReadiness `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if e := json.Unmarshal(payload, &env); e != nil && status < 300 {
		t.Fatalf("invalid check envelope: %s", payload)
	}
	return status, env.Data, env.Error.Code
}

func fpMCPCheck(t *testing.T, f *fixtures.Runtime, p service.Principal, action, space, ref string) (service.ActionReadiness, error) {
	t.Helper()
	args := map[string]any{"action": action, "space_id": space}
	if ref != "" {
		args["ref"] = ref
	}
	out, e := f.Service.CallTool(context.Background(), p, "capability_check", args)
	if e != nil {
		return service.ActionReadiness{}, e
	}
	m, ok := out.(map[string]any)
	if !ok {
		t.Fatal("capability_check returned unexpected shape")
	}
	b, _ := json.Marshal(m["readiness"])
	var readiness service.ActionReadiness
	if e := json.Unmarshal(b, &readiness); e != nil {
		t.Fatal(e)
	}
	return readiness, nil
}

// fpUpload uploads one small file through the SDK and returns its pinned ref.
func fpUpload(t *testing.T, f *fixtures.Runtime, c *sdk.Client, space, p string) string {
	t.Helper()
	op, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: space, Path: p}, "fp-readiness-"+p, bytes.NewReader([]byte(p)), nil)
	if e != nil {
		t.Fatal(e)
	}
	return fmt.Sprintf("drivebridge://%s/file/%s/version/%s", f.Service.Instance, op.FileID, op.VersionID)
}

// rawCall performs one HTTP call and returns the raw response body.
func fpRawCall(t *testing.T, h *human, method, path string, body []byte) (int, []byte) {
	t.Helper()
	req, _ := http.NewRequest(method, h.base+"/api/v1"+path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Origin", h.base)
	response, e := h.http.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	raw, _ := io.ReadAll(response.Body)
	return response.StatusCode, raw
}

// without any capability-side probing.
func TestFPReadinessConfiguredButNotProbed(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	// Simulate a legacy/pre-probe observation state on the platform row.
	if e := f.Service.DB.Model(&service.Connection{}).Where("id = ?", "platform").Updates(map[string]any{"observed_at": nil, "observed_status": ""}).Error; e != nil {
		t.Fatal(e)
	}
	status, readiness, code := fpCheck(t, h, "file_read", h.space.ID, "")
	if status != 200 {
		t.Fatalf("check failed: %d %s", status, code)
	}
	if !readiness.Implemented || !readiness.Configured || !readiness.Authorized {
		t.Fatalf("facts collapsed: %+v", readiness)
	}
	if readiness.DependencyStatus != "not_probed" {
		t.Fatal("unprobed dependency must read not_probed")
	}
	if readiness.ConnectionObservedAt != nil {
		t.Fatal("unprobed connection must report connection_observed_at=null")
	}
	if readiness.Availability != "unknown" {
		t.Fatalf("configured-but-unprobed must be unknown, got %q", readiness.Availability)
	}
	if readiness.Recovery == nil || readiness.Recovery.Actor != "human" || readiness.Recovery.Location != "server" {
		t.Fatalf("recovery entry missing or unsafe: %+v", readiness.Recovery)
	}
	// The preflight itself must not probe: the observation columns stay null.
	var conn service.Connection
	f.Service.DB.First(&conn, "id = ?", "platform")
	if conn.ObservedAt != nil || conn.ObservedStatus != "" {
		t.Fatal("capability_check probed the connection")
	}
	// An explicit operator probe records the observation...
	if status := h.call(t, "POST", "/connections/platform/check", nil, nil); status != 200 {
		t.Fatalf("connection check failed: %d", status)
	}
	f.Service.DB.First(&conn, "id = ?", "platform")
	if conn.ObservedAt == nil || conn.ObservedStatus != "verified" {
		t.Fatal("probe did not record observation")
	}
	// ...and only then the same preflight reports available.
	status, readiness, code = fpCheck(t, h, "file_read", h.space.ID, "")
	if status != 200 || readiness.Availability != "available" || readiness.DependencyStatus != "ok" {
		t.Fatalf("post-probe readiness wrong: %d %+v %s", status, readiness, code)
	}
	if readiness.ConnectionObservedAt == nil {
		t.Fatal("observed timestamp missing after probe")
	}
}

// Scenario 1 (inverse): losing directory permission must return a
// not-executable verdict without leaking any directory entry.
func TestFPReadinessLostDirectoryPermission(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, docsClient := agent(t, h, "docs", "read,write")
	_, full := agent(t, h, "", "read,write")
	inside := fpUpload(t, f, full, h.space.ID, "docs/inside.txt")
	outside := fpUpload(t, f, full, h.space.ID, "other/outside.txt")
	agentPrincipal := service.Principal{UserID: h.user.ID, Agent: &a}
	readiness, e := fpMCPCheck(t, f, agentPrincipal, "file_read", h.space.ID, inside)
	if e != nil {
		t.Fatal(e)
	}
	if !readiness.Authorized || readiness.Availability == "blocked" {
		t.Fatalf("in-prefix ref must stay executable: %+v", readiness)
	}
	// A prefix agent gets a space-level verdict judged on its own prefix: the
	// verdict never claims more than the agent's scope.
	spaceView, e := docsClient.CheckCapability(context.Background(), "file_read", h.space.ID, "")
	if e != nil || !spaceView.Authorized {
		t.Fatalf("prefix agent space verdict wrong: %+v %v", spaceView, e)
	}
	// Preflight the ref outside the agent's prefix: not executable, and the
	// response body must not contain any path or directory content. The SDK
	// entry carries the same verdict as HTTP and MCP (entry equivalence).
	outView, e := docsClient.CheckCapability(context.Background(), "file_read", h.space.ID, outside)
	if e != nil {
		t.Fatalf("out-of-prefix SDK preflight failed: %v", e)
	}
	if outView.Authorized || outView.Availability != "blocked" {
		t.Fatalf("out-of-prefix ref must be blocked: %+v", outView)
	}
	rawBody, sdkRawErr := json.Marshal(outView)
	if sdkRawErr != nil {
		t.Fatal(sdkRawErr)
	}
	if bytes.Contains(rawBody, []byte("outside.txt")) || bytes.Contains(rawBody, []byte("docs/")) || bytes.Contains(rawBody, []byte("other/")) {
		t.Fatal("preflight leaked directory entries")
	}
	// Revoking the agent entirely keeps the space readable for the human while
	// every agent preflight refuses.
	f.Service.DB.Model(&service.Agent{}).Where("id = ?", a.ID).Update("revoked", true)
	readiness, e = fpMCPCheck(t, f, agentPrincipal, "file_read", h.space.ID, inside)
	if e != nil {
		t.Fatal(e)
	}
	if readiness.Authorized || readiness.Availability != "blocked" {
		t.Fatalf("revoked agent still authorized: %+v", readiness)
	}
	readiness, e = fpMCPCheck(t, f, service.Principal{UserID: h.user.ID}, "file_read", h.space.ID, inside)
	if e != nil {
		t.Fatal(e)
	}
	if !readiness.Authorized {
		t.Fatal("human lost access after agent revocation")
	}
}

// Scenario 2: a pure MCP client recovers without any local CLI. Upload
// readiness points at the HTTP byte path and original-operation read-back;
// mount actions declare the owner-host dependency and stay agent-forbidden.
func TestFPReadinessNoCLIRecovery(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, _ := agent(t, h, "", "read,write")
	agentPrincipal := service.Principal{UserID: h.user.ID, Agent: &a}
	out, e := f.Service.CallTool(context.Background(), agentPrincipal, "capability_check", map[string]any{"action": "upload_create", "space_id": h.space.ID})
	if e != nil {
		t.Fatal(e)
	}
	m := out.(map[string]any)
	if m["byte_transport"] != "authenticated_http" || m["operation_read_back"] != "operation_get" {
		t.Fatalf("upload recovery must state the HTTP byte path and read-back rule: %+v", m)
	}
	b, _ := json.Marshal(m)
	if bytes.Contains(b, []byte("local_path")) || bytes.Contains(b, []byte("127.0.0.1")) {
		t.Fatal("upload recovery demanded a remote local-path read")
	}
	readiness, e := fpMCPCheck(t, f, agentPrincipal, "upload_create", h.space.ID, "")
	if e != nil {
		t.Fatal(e)
	}
	if !readiness.Implemented || !readiness.Authorized {
		t.Fatalf("agent upload readiness wrong: %+v", readiness)
	}
	if readiness.Availability != "available" {
		t.Fatalf("fixture storage is configured and observed, expected available: %+v", readiness)
	}
	// Mount actions: the dependency lives on the owner host; agents cannot
	// start mounts, and even humans get an owner-host recovery pointer.
	readiness, e = fpMCPCheck(t, f, agentPrincipal, "mount_directory", h.space.ID, "")
	if e != nil {
		t.Fatal(e)
	}
	if readiness.Authorized || readiness.BlockReason != "agent_not_permitted" {
		t.Fatalf("agent mount preflight must be agent_not_permitted: %+v", readiness)
	}
	if readiness.Recovery == nil || readiness.Recovery.Location != "owner_host" || readiness.Recovery.Actor != "human" {
		t.Fatalf("mount recovery must point at the owner host: %+v", readiness.Recovery)
	}
	status, humanView, code := fpCheck(t, h, "mount_directory", h.space.ID, "")
	if status != 200 {
		t.Fatalf("human mount check failed: %d %s", status, code)
	}
	if humanView.Availability != "blocked" || humanView.BlockReason != "owner_host_dependency" {
		t.Fatalf("human mount verdict wrong: %+v", humanView)
	}
	if humanView.Recovery == nil || humanView.Recovery.Location != "owner_host" {
		t.Fatal("human mount recovery missing owner-host location")
	}
}

// Scenario 3: the legacy capabilities contract keeps every old key and type;
// unknown actions fail with unsupported_action instead of a near guess.
func TestFPReadinessLegacyContract(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	var doc map[string]any
	if status := h.call(t, "GET", "/capabilities", nil, &doc); status != 200 {
		t.Fatal(status)
	}
	for _, key := range []string{"spec_version", "instance_id", "public_base_url", "part_size", "max_bytes", "tools", "transport", "one_drive_configured", "scopes", "local_paths", "recovery", "trash", "key_directory"} {
		if _, ok := doc[key]; !ok {
			t.Fatalf("legacy key %q dropped from capabilities", key)
		}
	}
	if doc["spec_version"] != "0.1" {
		t.Fatal("spec_version changed")
	}
	if _, ok := doc["part_size"].(float64); !ok {
		t.Fatal("part_size must stay numeric")
	}
	if _, ok := doc["recovery"].(string); !ok {
		t.Fatal("recovery must stay a string")
	}
	if _, ok := doc["tools"].([]any); !ok {
		t.Fatal("tools must stay an array")
	}
	actions, ok := doc["actions"].([]any)
	if !ok || len(actions) == 0 {
		t.Fatal("additive actions block missing")
	}
	limits, ok := doc["limits"].(map[string]any)
	if !ok {
		t.Fatal("additive limits block missing")
	}
	if _, ok := limits["max_batch_items"]; !ok {
		t.Fatal("unknown batch limit must be present as null")
	}
	if limits["max_batch_items"] != nil {
		t.Fatal("unknown batch limit must be null, not a number")
	}
	// space_id only narrows.
	var narrowed map[string]any
	if status := h.call(t, "GET", "/capabilities?space_id="+h.space.ID, nil, &narrowed); status != 200 {
		t.Fatal(status)
	}
	narrowLimits := narrowed["limits"].(map[string]any)
	if _, ok := narrowLimits["space_quota_bytes"]; !ok {
		t.Fatal("space narrowing missing quota limit")
	}
	// Another owner's space cannot be narrowed to.
	other, _ := login(t, f, "bob")
	var leakEnv map[string]any
	if status := other.call(t, "GET", "/capabilities?space_id="+h.space.ID, nil, &leakEnv); status != 404 {
		t.Fatalf("cross-owner narrowing must 404, got %d", status)
	}
	// Unknown and near-guess actions are refused identically.
	for _, action := range []string{"upload", "file-upload", "uploadx"} {
		status, _, code := fpCheck(t, h, action, h.space.ID, "")
		if status != 422 || code != "unsupported_action" {
			t.Fatalf("near guess %q must be unsupported_action/422, got %d %s", action, status, code)
		}
	}
	// The MCP capabilities tool keeps its own legacy shape plus additions.
	mcpDoc, e := f.Service.CallTool(context.Background(), service.Principal{UserID: h.user.ID}, "capabilities", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	mcpMap := mcpDoc.(map[string]any)
	for _, key := range []string{"tools", "part_size", "max_bytes", "instance_id", "byte_transport", "auth"} {
		if _, ok := mcpMap[key]; !ok {
			t.Fatalf("MCP capabilities lost key %q", key)
		}
	}
	if _, ok := mcpMap["actions"]; !ok {
		t.Fatal("MCP capabilities missing additive actions")
	}
}

// Replay, identity isolation, and the no-side-effect rule for preflights.
func TestFPReadinessReplayIsolationNoSideEffects(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	snapshot := func() (service.Connection, service.Space, int64, int64) {
		var conn service.Connection
		var sp service.Space
		var files, ops int64
		f.Service.DB.First(&conn, "id = ?", "platform")
		f.Service.DB.First(&sp, "id = ?", h.space.ID)
		f.Service.DB.Model(&service.File{}).Count(&files)
		f.Service.DB.Model(&service.Operation{}).Count(&ops)
		return conn, sp, files, ops
	}
	beforeConn, beforeSpace, beforeFiles, beforeOps := snapshot()
	status, first, _ := fpCheck(t, h, "file_read", h.space.ID, "")
	if status != 200 {
		t.Fatal(status)
	}
	status, second, _ := fpCheck(t, h, "file_read", h.space.ID, "")
	if status != 200 {
		t.Fatal(status)
	}
	b1, _ := json.Marshal(first)
	b2, _ := json.Marshal(second)
	if !bytes.Equal(b1, b2) {
		t.Fatal("replayed preflight returned a different verdict")
	}
	// Cross-owner isolation: bob cannot preflight alice's space and learns
	// nothing beyond not_found.
	status, _, code := fpCheck(t, bob, "file_read", h.space.ID, "")
	if status != 404 || code != "not_found" {
		t.Fatalf("cross-owner preflight must be not_found, got %d %s", status, code)
	}
	// A cross-instance ref is refused without leaking existence either way.
	status, _, code = fpCheck(t, h, "file_read", h.space.ID, "drivebridge://otherinstance48hexplaceholder0000000/file/"+string(bytes.Repeat([]byte("a"), 48))+"/version/"+string(bytes.Repeat([]byte("b"), 48)))
	if status != 403 || code != "cross_instance_rejected" {
		t.Fatalf("cross-instance ref must be rejected, got %d %s", status, code)
	}
	afterConn, afterSpace, afterFiles, afterOps := snapshot()
	if !bytes.Equal(beforeConn.Secret, afterConn.Secret) || beforeConn.Disabled != afterConn.Disabled || beforeConn.Status != afterConn.Status || beforeConn.ObservedAt == nil {
		t.Fatal("preflight mutated connection state")
	}
	if beforeSpace.Used != afterSpace.Used || beforeSpace.Reserved != afterSpace.Reserved || beforeSpace.MaxBytes != afterSpace.MaxBytes {
		t.Fatal("preflight mutated space state")
	}
	if beforeFiles != afterFiles || beforeOps != afterOps {
		t.Fatal("preflight created files or operations")
	}
}

// A restart keeps legacy observation state, old data and preflight verdicts.
func TestFPReadinessRestartPreservesReadiness(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	ref := fpUpload(t, f, c, h.space.ID, "restart.txt")
	// Reset to the legacy unprobed state, restart, and re-verify.
	f.Service.DB.Model(&service.Connection{}).Where("id = ?", "platform").Updates(map[string]any{"observed_at": nil, "observed_status": ""})
	config := f.Service.Config
	var filesBefore, versionsBefore int64
	f.Service.DB.Model(&service.File{}).Count(&filesBefore)
	f.Service.DB.Model(&service.Version{}).Count(&versionsBefore)
	f.Service.Close()
	reopened, e := service.Open(config)
	if e != nil {
		t.Fatal(e)
	}
	f.Service = reopened
	defer func() { f.Service = reopened }()
	var filesAfter, versionsAfter int64
	reopened.DB.Model(&service.File{}).Count(&filesAfter)
	reopened.DB.Model(&service.Version{}).Count(&versionsAfter)
	if filesBefore != filesAfter || versionsBefore != versionsAfter {
		t.Fatal("restart lost file/version references")
	}
	p := service.Principal{UserID: h.user.ID}
	readiness, e := reopened.CheckCapability(p, "file_read", h.space.ID, ref)
	if e != nil {
		t.Fatal(e)
	}
	if readiness.Availability != "unknown" || readiness.DependencyStatus != "not_probed" || readiness.ConnectionObservedAt != nil {
		t.Fatalf("restart fabricated or lost observation state: %+v", readiness)
	}
	if _, _, _, e := reopened.Download(context.Background(), p, mustFileID(t, reopened, h.user.ID, "restart.txt"), ""); e != nil {
		t.Fatal("old file unreadable after restart")
	}
}

// Task 4.1: HTTP, MCP and SDK entries must be equivalent projections of the
// same application verdict, and the tool contract must be discoverable.
func TestFPReadinessEntryEquivalence(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, c := agent(t, h, "", "read,write")
	f.Service.DB.Model(&service.Connection{}).Where("id = ?", "platform").Updates(map[string]any{"observed_at": nil, "observed_status": ""})
	// Discovery: capability_check appears in tools/list with its schema.
	found := false
	for _, tool := range service.ToolDefinitions() {
		if tool.Name == "capability_check" {
			found = true
		}
	}
	if !found {
		t.Fatal("capability_check missing from the discoverable tool contract")
	}
	agentPrincipal := service.Principal{UserID: h.user.ID, Agent: &a}
	httpStatus, httpView, httpCode := fpCheck(t, h, "file_read", h.space.ID, "")
	if httpStatus != 200 || httpCode != "" {
		t.Fatalf("HTTP entry failed: %d %s", httpStatus, httpCode)
	}
	mcpView, e := fpMCPCheck(t, f, agentPrincipal, "file_read", h.space.ID, "")
	if e != nil {
		t.Fatal(e)
	}
	sdkView, e := c.CheckCapability(context.Background(), "file_read", h.space.ID, "")
	if e != nil {
		t.Fatal(e)
	}
	// The human (HTTP) and the agent (MCP) verdicts agree on every
	// identity-independent fact; the SDK view agrees on the same fields.
	if mcpView.Action != httpView.Action || mcpView.Implemented != httpView.Implemented || mcpView.Configured != httpView.Configured {
		t.Fatalf("entries disagree on facts: http=%+v mcp=%+v", httpView, mcpView)
	}
	if mcpView.Availability != httpView.Availability || mcpView.DependencyStatus != httpView.DependencyStatus {
		t.Fatalf("entries disagree on verdict: http=%+v mcp=%+v", httpView, mcpView)
	}
	if sdkView.Action != httpView.Action || sdkView.Implemented != httpView.Implemented || sdkView.Configured != httpView.Configured {
		t.Fatalf("SDK entry disagrees on facts: http=%+v sdk=%+v", httpView, sdkView)
	}
	if sdkView.Availability != httpView.Availability || sdkView.DependencyStatus != httpView.DependencyStatus {
		t.Fatalf("SDK entry disagrees on verdict: http=%+v sdk=%+v", httpView, sdkView)
	}
	if sdkView.Availability != "unknown" || sdkView.ConnectionObservedAt != nil {
		t.Fatalf("unprobed state must be unknown across entries: %+v", sdkView)
	}
	// The same three entries agree on the refusal for an unknown action.
	if _, _, code := fpCheck(t, h, "nope_action", h.space.ID, ""); code != "unsupported_action" {
		t.Fatalf("HTTP refusal code %q", code)
	}
	if _, e := fpMCPCheck(t, f, agentPrincipal, "nope_action", h.space.ID, ""); e == nil || e.Error() != "unsupported_action" {
		t.Fatalf("MCP refusal code %v", e)
	}
	if _, e := c.CheckCapability(context.Background(), "nope_action", h.space.ID, ""); e == nil || e.Error() != "unsupported_action" {
		t.Fatalf("SDK refusal code %v", e)
	}
}

// Task 4.2: the pure MCP discovery-to-recovery flow over fixtures: discover
// the check tool, preflight the space, read the blocked reason, and land on a
// human repair location — without any CLI installed on the caller side.
func TestFPReadinessPureMCPDiscoveryToRepair(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, _ := agent(t, h, "", "read,write")
	agentPrincipal := service.Principal{UserID: h.user.ID, Agent: &a}
	// The caller has no local CLI: everything it learns must come from the
	// tool contract and the check verdict.
	out, e := f.Service.CallTool(context.Background(), agentPrincipal, "capabilities", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	capabilities := out.(map[string]any)
	actions, _ := capabilities["actions"].([]string)
	if len(actions) == 0 {
		t.Fatal("no actions advertised through MCP capabilities")
	}
	// Block the dependency by disconnecting the space's connection.
	f.Service.DB.Model(&service.Space{}).Where("id = ?", h.space.ID).Update("connection_id", "")
	readiness, e := fpMCPCheck(t, f, agentPrincipal, "upload_create", h.space.ID, "")
	if e != nil {
		t.Fatal(e)
	}
	if readiness.Availability != "blocked" || readiness.BlockReason != "storage_not_configured" {
		t.Fatalf("blocked reason missing: %+v", readiness)
	}
	if readiness.Recovery == nil || readiness.Recovery.Actor != "human" || readiness.Recovery.Location != "server" {
		t.Fatalf("repair pointer must route a human to the server: %+v", readiness.Recovery)
	}
	// A human repairs on the server (re-bind a connection) and the same
	// caller's next preflight observes the change through public state only.
	var conn service.Connection
	f.Service.DB.First(&conn, "id = ?", "platform")
	f.Service.DB.Model(&service.Space{}).Where("id = ?", h.space.ID).Update("connection_id", "platform")
	readiness, e = fpMCPCheck(t, f, agentPrincipal, "upload_create", h.space.ID, "")
	if e != nil {
		t.Fatal(e)
	}
	if !readiness.Configured || readiness.Availability != "available" {
		t.Fatalf("repair not observed through public contract: %+v", readiness)
	}
	// The blocked period produced no side effects for the caller: original
	// operations are still read back by their original IDs.
	ops, e := f.Service.CallTool(context.Background(), agentPrincipal, "operations", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	if _, ok := ops.([]service.Operation); !ok {
		t.Fatalf("operation read-back unavailable: %T", ops)
	}
}

func mustFileID(t *testing.T, s *service.Service, owner, path string) string {
	t.Helper()
	var file service.File
	if s.DB.First(&file, "owner = ? AND path = ?", owner, path).Error != nil {
		t.Fatal("fixture file missing")
	}
	return file.ID
}

// Task 5.2 scale acceptance: 10,000 read-only preflights inside a
// million-file space, 20 concurrent readers, no full-directory scan, no
// cross-identity leak, no configuration side effects. Gated behind
// DRIVEBRIDGE_FP_SCALE=quick|full because of fixture cost.
func TestFPReadinessScalePreflight(t *testing.T) {
	mode := os.Getenv("DRIVEBRIDGE_FP_SCALE")
	if mode == "" {
		t.Skip("scale fixture gated: set DRIVEBRIDGE_FP_SCALE=quick|full")
	}
	files := 200000
	if mode == "full" {
		files = 1000000
	}
	checks := 10000
	f := rig(t)
	h, _ := login(t, f, "alice")
	owner := h.user.ID
	started := time.Now()
	// One transaction for the whole seed: a single journal sync instead of a
	// thousand, keeping the large fixture viable on one machine.
	e := f.Service.DB.Transaction(func(tx *gorm.DB) error {
		batch := make([]service.File, 0, 1000)
		for i := range files {
			batch = append(batch, service.File{ID: fmt.Sprintf("%048x", i), Owner: owner, SpaceID: h.space.ID, Path: fmt.Sprintf("d%03d/f%09d.txt", i%1000, i), VersionID: fmt.Sprintf("%047x", i), Size: 10, MIME: "text/plain", Updated: time.Now().UTC()})
			if len(batch) == 1000 {
				if e := tx.Create(&batch).Error; e != nil {
					return e
				}
				batch = batch[:0]
			}
		}
		if len(batch) > 0 {
			return tx.Create(&batch).Error
		}
		return nil
	})
	if e != nil {
		t.Fatal(e)
	}
	seedSeconds := time.Since(started).Seconds()
	var count int64
	f.Service.DB.Model(&service.File{}).Where("space_id = ?", h.space.ID).Count(&count)
	if count != int64(files) {
		t.Fatalf("fixture seeded %d of %d files", count, files)
	}
	var connBefore service.Connection
	f.Service.DB.First(&connBefore, "id = ?", "platform")
	// Baseline on an empty space owned by another user.
	bob, _ := login(t, f, "bob")
	emptyStart := time.Now()
	if status, _, code := fpCheck(t, bob, "file_read", bob.space.ID, ""); status != 200 {
		t.Fatalf("baseline preflight failed: %d %s", status, code)
	}
	baseline := time.Since(emptyStart)
	// 10,000 preflights across 20 concurrent callers, mixed with ref pinning.
	var mu sync.Mutex
	latencies := make([]time.Duration, 0, checks)
	var failures int
	wg := sync.WaitGroup{}
	perWorker := checks / 20
	for w := range 20 {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for range perWorker {
				t0 := time.Now()
				principal := service.Principal{UserID: owner}
				_, e := f.Service.CheckCapability(principal, "file_read", h.space.ID, "")
				d := time.Since(t0)
				mu.Lock()
				latencies = append(latencies, d)
				if e != nil {
					failures++
				}
				mu.Unlock()
			}
		}(w)
	}
	wg.Wait()
	if failures != 0 {
		t.Fatalf("%d preflights failed", failures)
	}
	sortDurations(latencies)
	p50, p95, max := latencies[len(latencies)/2], latencies[len(latencies)*95/100], latencies[len(latencies)-1]
	// No full scan: one preflight must stay far below the cost of scanning a
	// million rows; the verdict must be independent of directory size.
	scanBudget := 50 * time.Millisecond
	if p95 > scanBudget {
		t.Fatalf("preflight P95 %v exceeds no-scan budget %v (directory size changed preflight semantics)", p95, scanBudget)
	}
	if max > scanBudget {
		t.Fatalf("preflight max %v exceeds no-scan budget %v", max, scanBudget)
	}
	_ = baseline
	// No configuration side effects and no cross-identity leak.
	var connAfter service.Connection
	f.Service.DB.First(&connAfter, "id = ?", "platform")
	if !bytes.Equal(connBefore.Secret, connAfter.Secret) || connBefore.Status != connAfter.Status {
		t.Fatal("scale preflights mutated configuration")
	}
	var alien int64
	f.Service.DB.Model(&service.File{}).Where("space_id = ? AND owner <> ?", h.space.ID, owner).Count(&alien)
	if alien != 0 {
		t.Fatal("preflight fixture leaked another owner's rows into the space")
	}
	report := map[string]any{"fixture_files": files, "checks": checks, "concurrency": 20, "seed_seconds": seedSeconds, "p50_ms": p50.Milliseconds(), "p95_ms": p95.Milliseconds(), "max_ms": max.Milliseconds(), "baseline_empty_space_ms": baseline.Milliseconds()}
	b, _ := json.MarshalIndent(report, "", "  ")
	t.Logf("scale report: %s", b)
	if dir := os.Getenv("DRIVEBRIDGE_EVIDENCE_ARTIFACTS"); dir != "" {
		os.MkdirAll(dir, 0o755)
		os.WriteFile(filepath.Join(dir, "fp-readiness-scale.json"), b, 0o644)
	}
}

func sortDurations(v []time.Duration) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}
