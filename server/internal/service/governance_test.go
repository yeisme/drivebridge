package service_test

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"github.com/yeisme/drivebridge-server/internal/service"
	"io"
	"net/http"
	"testing"
	"time"
)

func govCall(t *testing.T, h *human, tenant, space, method, path string, body, out any) int {
	t.Helper()
	var data io.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		data = bytes.NewReader(b)
	}
	req, _ := http.NewRequest(method, h.base+path, data)
	req.Header.Set("Origin", h.base)
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-DriveBridge-Tenant", tenant)
	req.Header.Set("X-DriveBridge-Space", space)
	resp, e := h.http.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	var envelope struct {
		Data  json.RawMessage `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if e = json.NewDecoder(resp.Body).Decode(&envelope); e != nil {
		t.Fatal("invalid response")
	}
	if out != nil && resp.StatusCode == 200 {
		if e = json.Unmarshal(envelope.Data, out); e != nil {
			t.Fatal(e)
		}
	}
	if resp.StatusCode >= 400 {
		t.Logf("governance response: status=%d code=%s", resp.StatusCode, envelope.Error.Code)
	}
	return resp.StatusCode
}
func TestGovernanceTenantApprovalAndRevocation(t *testing.T) {
	f := rig(t)
	root, _ := login(t, f, "root")
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	legacy, _ := login(t, f, "legacy")
	if e := f.Service.BootstrapGovernance(root.user.ID); e != nil {
		t.Fatal(e)
	}
	tenant := f.Service.Instance
	call := func(h *human, target, action string, q any, out any) int {
		return govCall(t, h, target, "", "POST", "/api/v1/governance/"+action, q, out)
	}
	// Legacy admin remains unable to supervise data without an explicit role.
	f.Service.DB.Model(&service.Session{}).Where("owner = ?", legacy.user.ID).Update("admin", true)
	if st := govCall(t, legacy, tenant, alice.space.ID, "GET", "/api/v1/files?space_id="+alice.space.ID, nil, nil); st != 403 {
		t.Fatalf("legacy expanded: %d", st)
	}
	if st := govCall(t, root, tenant, alice.space.ID, "GET", "/api/v1/files?space_id="+alice.space.ID, nil, nil); st != 200 {
		t.Fatalf("admin read %d", st)
	}
	if st := govCall(t, root, tenant, alice.space.ID, "POST", "/api/v1/directories", map[string]any{"space_id": alice.space.ID, "path": "forbidden"}, nil); st != 403 {
		t.Fatalf("admin write %d", st)
	}
	var request service.AccessRequest
	if st := call(root, tenant, "requests", map[string]any{"space_id": alice.space.ID, "kind": "range", "action": "write", "reason": "support task", "hours": 1}, &request); st != 200 {
		t.Fatalf("request %d", st)
	}
	if st := call(root, tenant, "approve", map[string]string{"id": request.ID}, nil); st != 403 {
		t.Fatalf("self approve %d", st)
	}
	if st := call(bob, tenant, "approve", map[string]string{"id": request.ID}, nil); st != 403 {
		t.Fatalf("other approve %d", st)
	}
	if st := call(alice, tenant, "approve", map[string]string{"id": request.ID}, nil); st != 200 {
		t.Fatalf("owner approve %d", st)
	}
	if st := govCall(t, root, tenant, alice.space.ID, "POST", "/api/v1/directories", map[string]any{"space_id": alice.space.ID, "path": "approved"}, nil); st != 200 {
		t.Fatalf("approved write %d", st)
	}
	if st := call(alice, tenant, "revoke", map[string]string{"id": request.ID}, nil); st != 200 {
		t.Fatal(st)
	}
	if st := govCall(t, root, tenant, alice.space.ID, "POST", "/api/v1/directories", map[string]any{"space_id": alice.space.ID, "path": "after-revoke"}, nil); st != 403 {
		t.Fatalf("revoked %d", st)
	}
	// A second tenant grants no authority over the same person's first space.
	var other service.Tenant
	if st := call(root, tenant, "tenants", map[string]string{"name": "Other tenant", "user_id": bob.user.ID}, &other); st != 200 {
		t.Fatalf("tenant create %d", st)
	}
	var member service.Membership
	f.Service.DB.First(&member, "tenant_id = ? AND user_id = ?", other.ID, bob.user.ID)
	if st := call(bob, tenant, "accept", map[string]string{"id": member.ID}, nil); st != 200 {
		t.Fatalf("accept %d", st)
	}
	if st := govCall(t, bob, other.ID, alice.space.ID, "GET", "/api/v1/files?space_id="+alice.space.ID, nil, nil); st != 403 {
		t.Fatalf("cross tenant %d", st)
	}
	if st := call(bob, other.ID, "invite", map[string]string{"user_id": alice.user.ID, "role": "member"}, &member); st != 200 {
		t.Fatalf("invite %d", st)
	}
	if st := call(alice, tenant, "accept", map[string]string{"id": member.ID}, nil); st != 200 {
		t.Fatalf("accept alice %d", st)
	}
	var spaces []service.Space
	if st := govCall(t, alice, other.ID, "", "GET", "/api/v1/spaces", nil, &spaces); st != 200 || len(spaces) != 1 || spaces[0].ID == alice.space.ID {
		t.Fatalf("personal isolation %d count %d", st, len(spaces))
	}
	var removedApproverGrant service.AccessRequest
	if st := call(bob, other.ID, "requests", map[string]any{"space_id": spaces[0].ID, "kind": "range", "action": "write", "reason": "approved before leaving"}, &removedApproverGrant); st != 200 {
		t.Fatal(st)
	}
	if st := call(alice, other.ID, "approve", map[string]string{"id": removedApproverGrant.ID}, nil); st != 200 {
		t.Fatal(st)
	}
	if st := call(bob, other.ID, "remove-member", map[string]string{"user_id": alice.user.ID}, nil); st != 200 {
		t.Fatal(st)
	}
	f.Service.DB.First(&removedApproverGrant, "id = ?", removedApproverGrant.ID)
	if removedApproverGrant.Status != "revoked" {
		t.Fatal("removed approver retained grant")
	}
	var invalidationNotes int64
	f.Service.DB.Model(&service.Notification{}).Where("user_id = ? AND resource_id = ? AND action = ?", bob.user.ID, removedApproverGrant.ID, "request.revoked").Count(&invalidationNotes)
	if invalidationNotes != 1 {
		t.Fatal("grant holder was not notified atomically")
	}
	if st := govCall(t, alice, other.ID, "", "GET", "/api/v1/spaces", nil, nil); st != 403 {
		t.Fatalf("removed membership %d", st)
	}
	if st := govCall(t, alice, tenant, "", "GET", "/api/v1/spaces", nil, nil); st != 200 {
		t.Fatalf("other membership affected %d", st)
	}
	var notes []service.Notification
	f.Service.DB.Where("user_id = ? AND action = ?", alice.user.ID, "data.view").Find(&notes)
	if len(notes) != 1 {
		t.Fatalf("view notifications not grouped: %d", len(notes))
	}
	// Fail closed when mandatory governance audit cannot persist.
	if e := f.Service.DB.Migrator().DropTable(&service.GovernanceEvent{}); e != nil {
		t.Fatal(e)
	}
	if st := govCall(t, root, tenant, alice.space.ID, "GET", "/api/v1/files?space_id="+alice.space.ID, nil, nil); st != 503 {
		t.Fatalf("audit outage %d", st)
	}
}

func TestGovernanceTeamScopesAndExactExecution(t *testing.T) {
	f := rig(t)
	root, _ := login(t, f, "root")
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	eve, _ := login(t, f, "eve")
	if e := f.Service.BootstrapGovernance(root.user.ID); e != nil {
		t.Fatal(e)
	}
	tenant := f.Service.Instance
	call := func(h *human, action string, q any, out any) int {
		return govCall(t, h, tenant, "", "POST", "/api/v1/governance/"+action, q, out)
	}
	var team service.Team
	if st := call(root, "teams", map[string]string{"name": "Research", "user_id": bob.user.ID}, &team); st != 200 {
		t.Fatal(st)
	}
	for _, h := range []*human{alice, eve} {
		if st := call(bob, "team-member", map[string]string{"team_id": team.ID, "user_id": h.user.ID, "role": "member"}, nil); st != 200 {
			t.Fatal(st)
		}
	}
	var space service.Space
	if st := call(root, "spaces", map[string]string{"team_id": team.ID, "name": "Team files", "user_id": alice.user.ID}, &space); st != 200 {
		t.Fatalf("space %d", st)
	}
	if st := call(bob, "permission", map[string]string{"space_id": space.ID, "user_id": bob.user.ID, "role": "editor"}, nil); st != 403 {
		t.Fatalf("self grant %d", st)
	}
	if st := govCall(t, eve, tenant, space.ID, "GET", "/api/v1/files?space_id="+space.ID, nil, nil); st != 403 {
		t.Fatalf("ungranted member %d", st)
	}
	if st := call(alice, "permission", map[string]string{"space_id": space.ID, "user_id": eve.user.ID, "role": "viewer"}, nil); st != 200 {
		t.Fatal(st)
	}
	if st := govCall(t, eve, tenant, space.ID, "GET", "/api/v1/files?space_id="+space.ID, nil, nil); st != 200 {
		t.Fatal(st)
	}
	f.Service.DB.First(&space, "id = ?", space.ID)
	// Create a metadata fixture; moves need no external storage.
	file := service.File{ID: service.ID(), Owner: space.Owner, SpaceID: space.ID, Path: "report.txt", VersionID: service.ID(), Updated: time.Now()}
	if e := f.Service.DB.Create(&file).Error; e != nil {
		t.Fatal(e)
	}
	var req service.AccessRequest
	if st := call(bob, "requests", map[string]any{"space_id": space.ID, "kind": "exact", "reason": "organize approved file", "items": []map[string]string{{"file_id": file.ID, "version_id": file.VersionID, "action": "move", "path": "reviewed.txt"}}}, &req); st != 200 {
		t.Fatal(st)
	}
	if st := call(alice, "approve", map[string]string{"id": req.ID}, nil); st != 200 {
		t.Fatal(st)
	}
	if st := call(bob, "execute", map[string]string{"id": req.ID}, &req); st != 200 || req.Status != "completed" {
		t.Fatalf("execute %d %s", st, req.Status)
	}
	if st := call(bob, "execute", map[string]string{"id": req.ID}, nil); st != 403 {
		t.Fatalf("replay %d", st)
	}
	var result service.File
	f.Service.DB.First(&result, "id = ?", file.ID)
	if result.Path != "reviewed.txt" {
		t.Fatal("move missing")
	}
	if st := call(bob, "team-member", map[string]string{"team_id": team.ID, "user_id": eve.user.ID, "role": "remove"}, nil); st != 200 {
		t.Fatal(st)
	}
	if st := govCall(t, eve, tenant, space.ID, "GET", "/api/v1/files?space_id="+space.ID, nil, nil); st != 403 {
		t.Fatalf("team removal %d", st)
	}
	if st := govCall(t, bob, tenant, alice.space.ID, "GET", "/api/v1/files?space_id="+alice.space.ID, nil, nil); st != 403 {
		t.Fatalf("team admin private files %d", st)
	}
}

func TestGovernanceEmergencyAndManualRecovery(t *testing.T) {
	f := rig(t)
	root, _ := login(t, f, "root")
	second, _ := login(t, f, "second")
	alice, _ := login(t, f, "alice")
	if e := f.Service.BootstrapGovernance(root.user.ID); e != nil {
		t.Fatal(e)
	}
	if e := f.Service.BootstrapGovernance(second.user.ID); e != nil {
		t.Fatal(e)
	}
	// Fixture assertion of a fresh provider-authenticated session, not a re-used SSO token.
	f.Service.DB.Model(&service.Session{}).Where("owner IN ?", []string{root.user.ID, second.user.ID}).Update("authenticated_at", time.Now().UTC())
	tenant := f.Service.Instance
	call := func(h *human, action string, q any, out any) int {
		return govCall(t, h, tenant, "", "POST", "/api/v1/governance/"+action, q, out)
	}
	var grant service.AccessRequest
	if st := call(alice, "emergency", map[string]string{"space_id": root.space.ID, "reason": "urgent maintenance", "action": "all"}, nil); st != 403 {
		t.Fatalf("ordinary emergency %d", st)
	}
	if st := call(root, "emergency", map[string]string{"space_id": alice.space.ID, "reason": "urgent maintenance", "action": "all"}, &grant); st != 200 {
		t.Fatalf("emergency %d", st)
	}
	if time.Until(grant.GrantUntil) > 15*time.Minute || grant.Kind != "emergency" {
		t.Fatal("emergency expiry")
	}
	if st := govCall(t, root, tenant, alice.space.ID, "POST", "/api/v1/directories", map[string]any{"space_id": alice.space.ID, "path": "emergency"}, nil); st != 200 {
		t.Fatalf("emergency write %d", st)
	}
	f.Service.DB.Model(&service.AccessRequest{}).Where("id = ?", grant.ID).Update("grant_until", time.Now().Add(-time.Second))
	if st := govCall(t, root, tenant, alice.space.ID, "POST", "/api/v1/directories", map[string]any{"space_id": alice.space.ID, "path": "expired"}, nil); st != 403 {
		t.Fatalf("emergency expired %d", st)
	}
	// Applicant generates the future credential. Admin never sees it or the proof.
	newKey := "dbu_" + service.ID() + service.ID()
	var claim struct {
		ID    string `json:"id"`
		Proof string `json:"claim_proof"`
	}
	// Use a separate unauthenticated client to require manual verification.
	anonymous := &human{http: &http.Client{}, base: f.App.URL}
	if st := govCall(t, anonymous, "", "", "POST", "/auth/recovery/request", map[string]string{"user_id": root.user.ID, "new_key": newKey}, &claim); st != 200 {
		t.Fatal(st)
	}
	if st := call(root, "recovery-approve", map[string]string{"claim_id": claim.ID, "reason": "identity verified offline"}, nil); st != 403 {
		t.Fatalf("self recovery approval %d", st)
	}
	if st := call(second, "recovery-approve", map[string]string{"claim_id": claim.ID, "reason": "identity verified offline"}, nil); st != 200 {
		t.Fatalf("manual approve %d", st)
	}
	if st := govCall(t, anonymous, "", "", "POST", "/auth/recovery/complete", map[string]string{"id": claim.ID, "claim_proof": "incorrect"}, nil); st != 400 {
		t.Fatalf("wrong proof %d", st)
	}
	if st := govCall(t, anonymous, "", "", "POST", "/auth/recovery/complete", map[string]string{"id": claim.ID, "claim_proof": claim.Proof}, nil); st != 200 {
		t.Fatalf("recover %d", st)
	}
	if st := govCall(t, root, tenant, "", "GET", "/api/v1/me", nil, nil); st != 401 {
		t.Fatalf("old session %d", st)
	}
	var role service.PlatformRole
	f.Service.DB.First(&role, "user_id = ?", root.user.ID)
	if !role.Suspended {
		t.Fatal("manual recovery retained platform role")
	}
	if st := call(second, "restore-privileges", map[string]string{"user_id": root.user.ID}, nil); st != 200 {
		t.Fatalf("privilege restore %d", st)
	}
	f.Service.DB.First(&role, "user_id = ?", root.user.ID)
	if role.Suspended {
		t.Fatal("independent restore missing")
	}
	var k service.UserKey
	f.Service.DB.First(&k, "token_hash = ?", service.Hash(newKey))
	if k.Owner != root.user.ID || k.Admin {
		t.Fatal("new login or privilege source wrong")
	}
}

func TestGovernanceStaleVersionAndLegacyCredentialBypass(t *testing.T) {
	f := rig(t)
	root, _ := login(t, f, "root")
	alice, _ := login(t, f, "alice")
	issued, e := f.Service.ImportUsers([]service.UserImport{{Name: "legacy-admin", Admin: true}})
	if e != nil {
		t.Fatal(e)
	}
	if e = f.Service.BootstrapGovernance(root.user.ID); e != nil {
		t.Fatal(e)
	}
	tenant := f.Service.Instance
	call := func(h *human, action string, q any, out any) int {
		return govCall(t, h, tenant, "", "POST", "/api/v1/governance/"+action, q, out)
	}
	file := service.File{ID: service.ID(), Owner: alice.user.ID, SpaceID: alice.space.ID, Path: "a.txt", VersionID: service.ID()}
	f.Service.DB.Create(&file)
	var req service.AccessRequest
	if st := call(root, "requests", map[string]any{"space_id": alice.space.ID, "kind": "exact", "reason": "review change", "items": []map[string]string{{"file_id": file.ID, "version_id": file.VersionID, "action": "move", "path": "b.txt"}}}, &req); st != 200 {
		t.Fatal(st)
	}
	if st := call(alice, "approve", map[string]string{"id": req.ID}, nil); st != 200 {
		t.Fatal(st)
	}
	f.Service.DB.Model(&file).Update("version_id", service.ID())
	if st := call(root, "execute", map[string]string{"id": req.ID}, &req); st != 200 || req.Status != "partial" || req.Items[0].Error != "version_conflict" {
		t.Fatalf("stale version %d %s", st, req.Status)
	}
	var key service.UserKey
	f.Service.DB.First(&key, "owner = ?", issued[0].UserID)
	if st := govCall(t, root, tenant, "", "POST", "/api/v1/users/"+key.ID+"/rotate", map[string]any{}, nil); st != 403 {
		t.Fatalf("legacy rotation %d", st)
	}
	if st := govCall(t, root, tenant, alice.space.ID, "POST", "/api/v1/shares", map[string]any{"name": "bypass", "space_id": alice.space.ID, "mode": "live", "audience": "link", "days": 1}, nil); st != 403 {
		t.Fatalf("readonly share %d", st)
	}
}

func TestGovernanceExactOverwriteExportAndEnrollment(t *testing.T) {
	f := rig(t)
	root, _ := login(t, f, "root")
	alice, _ := login(t, f, "alice")
	if e := f.Service.BootstrapGovernance(root.user.ID); e != nil {
		t.Fatal(e)
	}
	tenant := f.Service.Instance
	call := func(h *human, action string, q any, out any) int {
		return govCall(t, h, tenant, "", "POST", "/api/v1/governance/"+action, q, out)
	}
	seed := func(path, content string) service.Operation {
		p := service.Principal{UserID: alice.user.ID, TenantID: tenant}
		op, e := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: alice.space.ID, Path: path, Size: int64(len(content)), SHA256: service.Hash(content)}, service.ID())
		if e != nil {
			t.Fatal(e)
		}
		if _, e = f.Service.PutPart(p, op.ID, 0, bytes.NewBufferString(content)); e != nil {
			t.Fatal(e)
		}
		op, e = f.Service.Complete(context.Background(), p, op.ID)
		if e != nil {
			t.Fatal(e)
		}
		return op
	}
	target := seed("target.txt", "original")
	source := seed("source.txt", "replacement")
	var approval service.AccessRequest
	if st := call(root, "requests", map[string]any{"space_id": alice.space.ID, "kind": "exact", "reason": "replace with reviewed version", "items": []map[string]string{{"file_id": target.FileID, "version_id": target.VersionID, "action": "overwrite", "source_file_id": source.FileID, "source_version_id": source.VersionID}}}, &approval); st != 200 {
		t.Fatal(st)
	}
	if st := call(alice, "approve", map[string]string{"id": approval.ID}, nil); st != 200 {
		t.Fatal(st)
	}
	if st := call(root, "execute", map[string]string{"id": approval.ID}, &approval); st != 200 || approval.Status != "completed" {
		t.Fatalf("overwrite %d %s %+v", st, approval.Status, approval.Items)
	}
	var file service.File
	f.Service.DB.First(&file, "id = ?", target.FileID)
	if file.VersionID == target.VersionID || file.Size != int64(len("replacement")) {
		t.Fatal("overwrite not committed")
	}
	req, _ := http.NewRequest("GET", f.App.URL+"/api/v1/files-export?files="+file.ID+","+source.FileID, nil)
	req.Header.Set("X-DriveBridge-Tenant", tenant)
	req.Header.Set("X-DriveBridge-Space", alice.space.ID)
	response, e := root.http.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	body, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if response.StatusCode != 200 {
		t.Fatal("export failed")
	}
	archive, e := zip.NewReader(bytes.NewReader(body), int64(len(body)))
	if e != nil || len(archive.File) != 2 {
		t.Fatal("invalid export")
	}
	var n int64
	f.Service.DB.Model(&service.Notification{}).Where("user_id = ? AND action = ?", alice.user.ID, "data.download").Count(&n)
	if n != 1 {
		t.Fatalf("export notification count %d", n)
	}
	var provision struct {
		UserID string `json:"user_id"`
		Code   string `json:"activation_code"`
	}
	if st := call(root, "provision", map[string]string{"name": "new colleague"}, &provision); st != 200 {
		t.Fatal(st)
	}
	key := "dbu_" + service.ID() + service.ID()
	anonymous := &human{http: &http.Client{}, base: f.App.URL}
	enrollment := map[string]string{"activation_code": provision.Code, "new_key": key}
	if st := govCall(t, anonymous, "", "", "POST", "/auth/enroll", enrollment, nil); st != 200 {
		t.Fatalf("enrollment %d", st)
	}
	if st := govCall(t, anonymous, "", "", "POST", "/auth/enroll", enrollment, nil); st != 400 {
		t.Fatalf("enrollment reused %d", st)
	}
	var keys int64
	f.Service.DB.Model(&service.UserKey{}).Where("owner = ? AND token_hash = ?", provision.UserID, service.Hash(key)).Count(&keys)
	if keys != 1 {
		t.Fatal("applicant key not active")
	}
}

func TestGovernanceStreamingStopsAfterCredentialRevocation(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	if e := f.Service.BootstrapGovernance(alice.user.ID); e != nil {
		t.Fatal(e)
	}
	var session service.Session
	f.Service.DB.First(&session, "owner = ?", alice.user.ID)
	p := service.Principal{UserID: alice.user.ID, TenantID: f.Service.Instance, SessionHash: session.Hash}
	content := bytes.Repeat([]byte("x"), 600<<10)
	op, e := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: alice.space.ID, Path: "large.txt", Size: int64(len(content)), SHA256: service.Hash(string(content))}, service.ID())
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.Service.PutPart(p, op.ID, 0, bytes.NewReader(content)); e != nil {
		t.Fatal(e)
	}
	op, e = f.Service.Complete(context.Background(), p, op.ID)
	if e != nil {
		t.Fatal(e)
	}
	_, _, reader, e := f.Service.Download(context.Background(), p, op.FileID, op.VersionID)
	if e != nil {
		t.Fatal(e)
	}
	defer reader.Close()
	buf := make([]byte, 256<<10)
	if _, e = io.ReadFull(reader, buf); e != nil {
		t.Fatal(e)
	}
	f.Service.DB.Delete(&service.Session{}, "hash = ?", session.Hash)
	if _, e = reader.Read(buf); e == nil {
		t.Fatal("stream survived credential revocation")
	}
}

func TestGovernanceMCPUsersListRequiresPlatformAdmin(t *testing.T) {
	f := rig(t)
	root, _ := login(t, f, "root")
	legacy, e := f.Service.ImportUsers([]service.UserImport{{Name: "legacy-admin", Admin: true}})
	if e != nil {
		t.Fatal(e)
	}
	if e := f.Service.BootstrapGovernance(root.user.ID); e != nil {
		t.Fatal(e)
	}
	// A pre-bootstrap admin key without a platform role must not list every
	// tenant's keys through MCP; the HTTP user directory is already gated.
	legacyP := service.Principal{UserID: legacy[0].UserID, Key: &service.UserKey{Admin: true}}
	if _, e := f.Service.CallTool(context.Background(), legacyP, "users_list", map[string]any{}); e == nil || e.Error() != "forbidden" {
		t.Fatalf("legacy admin key must be forbidden, got %v", e)
	}
	rootP := service.Principal{UserID: root.user.ID, Key: &service.UserKey{Admin: true}}
	if _, e := f.Service.CallTool(context.Background(), rootP, "users_list", map[string]any{}); e != nil {
		t.Fatalf("platform admin must list keys: %v", e)
	}
}
