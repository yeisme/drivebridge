package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	mcpclient "github.com/mark3labs/mcp-go/client"
	"github.com/mark3labs/mcp-go/client/transport"
	"github.com/mark3labs/mcp-go/mcp"
	"github.com/yeisme/drivebridge-server/internal/fixtures"
	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
	"github.com/yeisme/drivebridge-server/sdk/insights"
	"gorm.io/gorm"
)

type human struct {
	http  *http.Client
	base  string
	csrf  string
	user  service.User
	space service.Space
}

func rig(t *testing.T) *fixtures.Runtime {
	t.Helper()
	f, e := fixtures.Start(t.TempDir(), "", "")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(f.Close)
	return f
}
func login(t *testing.T, f *fixtures.Runtime, name string) (*human, int) {
	t.Helper()
	jar, _ := cookiejar.New(nil)
	h := &human{http: &http.Client{Jar: jar}, base: f.App.URL}
	response, e := h.http.Get(h.base + "/auth/login")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := io.ReadAll(response.Body)
	response.Body.Close()
	match := regexp.MustCompile(`/approve\?id=([^&]+)&amp;user=`).FindSubmatch(b)
	if len(match) != 2 {
		t.Fatal("fixture login form missing")
	}
	response, e = h.http.Get(f.OIDC.URL + "/approve?id=" + string(match[1]) + "&user=" + name)
	if e != nil {
		t.Fatal(e)
	}
	status := response.StatusCode
	if response.Request.URL.Path == "/" {
		status = 200
	}
	response.Body.Close()
	if status >= 400 {
		return h, status
	}
	var me struct {
		User service.User `json:"user"`
		CSRF string       `json:"csrf_token"`
	}
	h.call(t, "GET", "/me", nil, &me)
	h.user = me.User
	h.csrf = me.CSRF
	var spaces []service.Space
	h.call(t, "GET", "/spaces", nil, &spaces)
	if len(spaces) != 1 {
		t.Fatal("personal space missing")
	}
	h.space = spaces[0]
	return h, 200
}
func (h *human) call(t *testing.T, method, path string, body, out any) int {
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
	response, e := h.http.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer response.Body.Close()
	var env struct {
		Data  json.RawMessage `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if e = json.NewDecoder(response.Body).Decode(&env); e != nil {
		t.Fatal("invalid API response")
	}
	if out != nil && response.StatusCode < 300 {
		if e = json.Unmarshal(env.Data, out); e != nil {
			t.Fatal(e)
		}
	}
	return response.StatusCode
}
func agent(t *testing.T, h *human, prefix, scopes string) (service.Agent, *sdk.Client) {
	t.Helper()
	var out struct {
		Agent service.Agent `json:"agent"`
		Token string        `json:"token"`
	}
	if status := h.call(t, "POST", "/agents", map[string]any{"name": "test-agent", "space_id": h.space.ID, "prefix": prefix, "scopes": scopes, "days": 7}, &out); status != 200 {
		t.Fatalf("agent creation status %d", status)
	}
	client, e := sdk.New(h.base, out.Token)
	if e != nil {
		t.Fatal(e)
	}
	return out.Agent, client
}

func TestOIDCIsolationTransferAndRevocation(t *testing.T) {
	f := rig(t)
	alice, status := login(t, f, "alice")
	if status != 200 {
		t.Fatal(status)
	}
	bob, status := login(t, f, "bob")
	if status != 200 {
		t.Fatal(status)
	}
	if alice.user.ID == bob.user.ID {
		t.Fatal("same email merged users")
	}
	a, c := agent(t, alice, "project", "read,write")
	payload := bytes.Repeat([]byte("hello "), (9<<20)/6)
	op, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: alice.space.ID, Path: "project/reference.txt"}, "one-upload", bytes.NewReader(payload), nil)
	if e != nil {
		t.Fatal(e)
	}
	if op.Status != "completed" {
		t.Fatal(op.Status)
	}
	again, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: alice.space.ID, Path: "project/reference.txt"}, "one-upload", bytes.NewReader(payload), nil)
	if e != nil || again.FileID != op.FileID || f.ObjectCount() != 1 {
		t.Fatal("replay duplicated file")
	}
	var out bytes.Buffer
	if e = c.Download(context.Background(), op.FileID, "", &out); e != nil || !bytes.Equal(payload, out.Bytes()) {
		t.Fatal("download differs")
	}
	for _, path := range []string{"/files/" + op.FileID, "/files/" + op.FileID + "/versions", "/operations/" + op.ID, "/files?space_id=" + alice.space.ID} {
		if got := bob.call(t, "GET", path, nil, nil); got != 404 {
			t.Fatalf("cross-user path returned %d", got)
		}
	}
	if _, e = c.Upload(context.Background(), sdk.UploadRequest{SpaceID: alice.space.ID, Path: "project-other/private.txt"}, "escape", bytes.NewReader([]byte("x")), nil); e == nil {
		t.Fatal("prefix escape")
	}
	if got := alice.call(t, "POST", "/agents/"+a.ID+"/revoke", map[string]any{}, nil); got != 200 {
		t.Fatal(got)
	}
	if e = c.JSON(context.Background(), "GET", "/api/v1/spaces", "", nil, &[]service.Space{}); e == nil {
		t.Fatal("revoked token accepted")
	}
}
func TestOIDCRejectsInvalidClaims(t *testing.T) {
	for _, claim := range []string{"issuer", "audience", "nonce", "expired"} {
		t.Run(claim, func(t *testing.T) {
			f := rig(t)
			f.SetBadClaim(claim)
			_, status := login(t, f, "alice")
			if status < 400 {
				t.Fatal("invalid ID token accepted")
			}
		})
	}
}
func TestAgentReadOnlyVersionTrashAndConnectionIsolation(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	b, _ := login(t, f, "bob")
	_, writer := agent(t, h, "", "read,write,delete")
	_, reader := agent(t, h, "", "read")
	op, e := writer.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "notes/readme.md"}, "initial", bytes.NewReader([]byte("one")), nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = reader.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "other"}, "readonly", bytes.NewReader([]byte("x")), nil); e == nil {
		t.Fatal("read-only agent wrote")
	}
	v2, e := writer.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "notes/readme.md", FileID: op.FileID, ExpectedVersion: op.VersionID}, "v2", bytes.NewReader([]byte("two")), nil)
	if e != nil || v2.FileID != op.FileID || v2.VersionID == op.VersionID {
		t.Fatal("new version not published")
	}
	var old bytes.Buffer
	if e = writer.Download(context.Background(), op.FileID, op.VersionID, &old); e != nil || old.String() != "one" {
		t.Fatal("old version changed")
	}
	h.call(t, "POST", "/files/"+op.FileID+"/trash", map[string]any{}, nil)
	if e = writer.Download(context.Background(), op.FileID, "", io.Discard); e == nil {
		t.Fatal("trashed file served")
	}
	h.call(t, "POST", "/files/"+op.FileID+"/restore", map[string]any{}, nil)
	var connection service.Connection
	status := h.call(t, "POST", "/connections/s3", map[string]any{"name": "personal", "config": service.S3Config{Endpoint: f.S3.URL, Bucket: "private", Region: "us-east-1", AccessKey: "fixture-byos", SecretKey: "fixture-secret", Prefix: "dedicated"}}, &connection)
	if status != 200 {
		t.Fatal(status)
	}
	if got := b.call(t, "POST", "/connections/"+connection.ID+"/disconnect", map[string]any{}, nil); got != 404 {
		t.Fatal("connection crossed users")
	}
	var sp service.Space
	h.call(t, "POST", "/spaces", map[string]any{"name": "BYOS", "connection_id": connection.ID}, &sp)
	if sp.ID == "" {
		t.Fatal("BYOS space missing")
	}
	h.call(t, "POST", "/connections/"+connection.ID+"/disconnect", map[string]any{}, nil)
	var stored service.Connection
	f.Service.DB.First(&stored, "id = ?", connection.ID)
	if !stored.Disabled || bytes.Contains(stored.Secret, []byte("fixture-secret")) {
		t.Fatal("credential encryption or disconnect failed")
	}
}
func TestCSRFStateReplayAndTokenExpiry(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	saved := h.csrf
	h.csrf = "wrong"
	if got := h.call(t, "POST", "/agents", map[string]any{}, nil); got != 403 {
		t.Fatal("CSRF accepted")
	}
	h.csrf = saved
	a, c := agent(t, h, "", "read")
	f.Service.DB.Model(&a).Update("expires", time.Now().Add(-time.Minute))
	if c.JSON(context.Background(), "GET", "/api/v1/spaces", "", nil, &[]service.Space{}) == nil {
		t.Fatal("expired credential accepted")
	}
	response, e := h.http.Get(f.App.URL + "/auth/callback?state=replayed&code=old")
	if e != nil {
		t.Fatal(e)
	}
	response.Body.Close()
	if response.StatusCode < 400 {
		t.Fatal("replayed callback accepted")
	}
}
func TestMetadataRestoreAndMissingKey(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	op, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "backup.txt"}, "backup", bytes.NewReader([]byte("retain")), nil)
	if e != nil {
		t.Fatal(e)
	}
	backup := t.TempDir()
	os.Chmod(backup, 0700)
	for _, name := range []string{"metadata.sqlite", "instance.key", "instance.id"} {
		b, e := os.ReadFile(filepath.Join(f.Service.Config.State, name))
		if e != nil {
			t.Fatal(e)
		}
		os.WriteFile(filepath.Join(backup, name), b, 0600)
	}
	restored, e := service.Open(service.Config{State: backup, Enabled: true, Development: true})
	if e != nil {
		t.Fatal(e)
	}
	defer restored.Close()
	var file service.File
	if restored.DB.First(&file, "id = ? AND owner = ?", op.FileID, h.user.ID).Error != nil {
		t.Fatal("restored ownership missing")
	}
	_, _, body, e := restored.Download(context.Background(), service.Principal{UserID: h.user.ID}, op.FileID, "")
	if e != nil {
		t.Fatal(e)
	}
	b, _ := io.ReadAll(body)
	body.Close()
	if string(b) != "retain" {
		t.Fatal("restored object differs")
	}
	_ = url.PathEscape("fixture")
}

func TestNativeMCPAndPrivateAccessDoNotLeakAuthority(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, c := agent(t, h, "", "read,write")
	op, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "reference.txt"}, "native-mcp", bytes.NewReader([]byte("reference")), nil)
	if e != nil {
		t.Fatal(e)
	}
	access, e := c.Acquire(context.Background(), op.FileID, op.VersionID)
	if e != nil || access.URL == "" {
		t.Fatal("private HTTP access unavailable")
	}
	b, _ := json.Marshal(access)
	if bytes.Contains(b, []byte("X-Amz-")) {
		t.Fatal("SDK serialized private URL")
	}
	response, e := http.Get(access.URL)
	if e != nil {
		t.Fatal("S3 fixture access failed")
	}
	data, _ := io.ReadAll(response.Body)
	response.Body.Close()
	if string(data) != "reference" {
		t.Fatal("private access points at wrong file")
	}
	projection, e := f.Service.CallTool(context.Background(), service.Principal{UserID: h.user.ID, Agent: &a}, "prepare_reference", map[string]any{"file_id": op.FileID, "version_id": op.VersionID})
	if e != nil {
		t.Fatal(e)
	}
	b, _ = json.Marshal(projection)
	if (bytes.Contains(b, []byte("http://")) || bytes.Contains(b, []byte("https://"))) || bytes.Contains(b, []byte("X-Amz-")) {
		t.Fatal("MCP reference leaked a URL")
	}
}
func TestMissingInstanceKeyDoesNotInventNewCredentials(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, e := service.Open(service.Config{State: dir})
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	os.Remove(filepath.Join(dir, "instance.key"))
	if _, e = service.Open(service.Config{State: dir}); e == nil {
		t.Fatal("missing restore key was silently replaced")
	}
}
func TestRestartPartRecoveryCancellationAndConflictingContent(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	_, c := agent(t, h, "", "read,write")
	payload := []byte("recoverable")
	q := sdk.UploadRequest{SpaceID: h.space.ID, Path: "resume.txt", Size: int64(len(payload)), SHA256: service.Hash(string(payload))}
	var op sdk.Operation
	if e := c.JSON(context.Background(), "POST", "/api/v1/uploads", "resume-operation", q, &op); e != nil {
		t.Fatal(e)
	}
	p := service.Principal{UserID: h.user.ID}
	if _, e := f.Service.PutPart(p, op.ID, 0, bytes.NewReader([]byte("wrong"))); e == nil {
		t.Fatal("short part accepted")
	}
	if _, e := f.Service.PutPart(p, op.ID, 0, bytes.NewReader(payload)); e != nil {
		t.Fatal(e)
	}
	if _, e := f.Service.PutPart(p, op.ID, 0, bytes.NewReader(bytes.Repeat([]byte("x"), len(payload)))); e == nil {
		t.Fatal("part replacement accepted")
	}
	original := f.Service.Config
	f.Service.Close()
	reopened, e := service.Open(original)
	if e != nil {
		t.Fatal(e)
	}
	f.Service = reopened
	recovered, e := reopened.Complete(context.Background(), p, op.ID)
	if e != nil || recovered.Status != "completed" {
		t.Fatal("restart could not recover pinned parts")
	}
	again, e := reopened.Complete(context.Background(), p, op.ID)
	if e != nil || again.FileID != recovered.FileID {
		t.Fatal("complete replay duplicated file")
	}
	cancel, e := reopened.CreateUpload(p, service.UploadRequest{SpaceID: h.space.ID, Path: "cancel.txt", Size: 1, SHA256: service.Hash("x")}, "cancel")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = reopened.Cancel(p, cancel.ID); e != nil {
		t.Fatal(e)
	}
	if _, e = reopened.PutPart(p, cancel.ID, 0, bytes.NewReader([]byte("x"))); e == nil {
		t.Fatal("cancelled operation resumed")
	}
	var sp service.Space
	reopened.DB.First(&sp, "id = ?", h.space.ID)
	if sp.Reserved != 0 {
		t.Fatal("cancel quota was not released")
	}
}

func TestQuotaReservationAndUnreachableStorageRemainRecoverable(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	f.Service.DB.Model(&service.Space{}).Where("id = ?", h.space.ID).Update("max_bytes", 1)
	first, e := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: h.space.ID, Path: "a", Size: 1, SHA256: service.Hash("a")}, "reserve")
	if e != nil {
		t.Fatal(e)
	}
	if _, e = f.Service.CreateUpload(p, service.UploadRequest{SpaceID: h.space.ID, Path: "b", Size: 1, SHA256: service.Hash("b")}, "full"); e == nil || e.Error() != "quota_exceeded" {
		t.Fatal("quota reservation bypassed")
	}
	f.Service.PutPart(p, first.ID, 0, bytes.NewReader([]byte("a")))
	f.Service.DB.Model(&service.Connection{}).Where("id = ?", "platform").Update("disabled", true)
	if _, e = f.Service.Complete(context.Background(), p, first.ID); e == nil || e.Error() != "storage_disconnected" {
		t.Fatal("disconnected store consumed")
	}
	f.Service.DB.Model(&service.Connection{}).Where("id = ?", "platform").Update("disabled", false)
	if op, e := f.Service.Complete(context.Background(), p, first.ID); e != nil || op.Status != "completed" {
		t.Fatal("original operation could not recover")
	}
}

func TestNativeHTTPMCPUsesTheSamePrincipal(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	var grant struct {
		Agent service.Agent `json:"agent"`
		Token string        `json:"token"`
	}
	h.call(t, "POST", "/agents", map[string]any{"name": "HTTP MCP", "space_id": h.space.ID, "scopes": "read"}, &grant)
	c, e := mcpclient.NewStreamableHttpClient(f.App.URL+"/mcp", transport.WithHTTPHeaders(map[string]string{"Authorization": "Bearer " + grant.Token}))
	if e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if e = c.Start(ctx); e != nil {
		t.Fatal(e)
	}
	defer c.Close()
	init := mcp.InitializeRequest{}
	init.Params.ProtocolVersion = "2024-11-05"
	init.Params.ClientInfo = mcp.Implementation{Name: "fixture", Version: "1"}
	if _, e = c.Initialize(ctx, init); e != nil {
		t.Fatal(e)
	}
	tools, e := c.ListTools(ctx, mcp.ListToolsRequest{})
	if e != nil || len(tools.Tools) < 8 {
		t.Fatal("native MCP discovery failed")
	}
	q := mcp.CallToolRequest{}
	q.Params.Name = "spaces"
	q.Params.Arguments = map[string]any{}
	out, e := c.CallTool(ctx, q)
	if e != nil || out.IsError {
		t.Fatal("native MCP lost principal")
	}
	encoded, _ := json.Marshal(out)
	if !bytes.Contains(encoded, []byte(h.space.ID)) || bytes.Contains(encoded, []byte(grant.Token)) {
		t.Fatal("native MCP authority mismatch")
	}
	h.call(t, "POST", "/agents/"+grant.Agent.ID+"/revoke", map[string]any{}, nil)
	if _, e = c.CallTool(ctx, q); e == nil {
		t.Fatal("native MCP accepted revoked principal")
	}
}

func TestMissingInstanceIdentityCannotRewriteStableReferences(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, e := service.Open(service.Config{State: dir})
	if e != nil {
		t.Fatal(e)
	}
	s.Close()
	os.Remove(filepath.Join(dir, "instance.id"))
	if _, e = service.Open(service.Config{State: dir}); e == nil {
		t.Fatal("missing instance identity silently regenerated")
	}
}

func TestPersonalLoopProvenancePinnedVersionsAndSearch(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	_, client := agent(t, alice, "outputs", "read,write,delete")
	ctx := context.Background()
	q := sdk.UploadRequest{SpaceID: alice.space.ID, Path: "outputs/report.md", Caller: "report-agent", TaskRef: "task:report"}
	first, err := client.Upload(ctx, q, "report-original", bytes.NewReader([]byte("original report")), nil)
	if err != nil {
		t.Fatal(err)
	}
	q.FileID, q.ExpectedVersion = first.FileID, first.VersionID
	q.TaskRef = "task:revision"
	second, err := client.Upload(ctx, q, "report-revision", bytes.NewReader([]byte("revised report")), nil)
	if err != nil {
		t.Fatal(err)
	}
	ref := "drivebridge://" + f.Service.Instance + "/file/" + first.FileID + "/version/" + first.VersionID
	var pinned struct {
		File sdk.File `json:"file"`
		Ref  string   `json:"ref"`
	}
	if err = client.JSON(ctx, "GET", "/api/v1/references?ref="+url.QueryEscape(ref), "", nil, &pinned); err != nil {
		t.Fatal(err)
	}
	if pinned.File.VersionID != first.VersionID || pinned.File.TaskRef != "task:report" || pinned.Ref != ref || second.VersionID == first.VersionID {
		t.Fatal("pinned version silently changed")
	}
	if status := bob.call(t, "GET", "/references?ref="+url.QueryEscape(ref), nil, nil); status != 404 {
		t.Fatalf("cross-user reference: %d", status)
	}
	var hits []sdk.File
	if err = client.JSON(ctx, "GET", "/api/v1/files?space_id="+alice.space.ID+"&q=REPORT&prefix=outputs", "", nil, &hits); err != nil || len(hits) != 1 || hits[0].TaskRef != "task:revision" {
		t.Fatal("search lost provenance")
	}
	bad := q
	bad.Caller = "https://secret.example/token"
	if _, err = client.Upload(ctx, bad, "bad-label", bytes.NewReader([]byte("data")), nil); err == nil {
		t.Fatal("URL accepted as provenance")
	}
	if err = client.JSON(ctx, "POST", "/api/v1/files/"+first.FileID+"/trash", "", nil, nil); err != nil {
		t.Fatal(err)
	}
	if err = client.JSON(ctx, "POST", "/api/v1/files/"+first.FileID+"/restore", "", nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestUsageAuditScopesScansAndFailurePolicy(t *testing.T) {
	fixture := rig(t)
	alice, _ := login(t, fixture, "alice")
	bob, _ := login(t, fixture, "bob")
	a, client := agent(t, alice, "docs", "read,write,delete")
	ctx := context.Background()
	q := sdk.UploadRequest{SpaceID: alice.space.ID, Path: "docs/usage.txt", Caller: "declared-agent"}
	op, e := client.Upload(ctx, q, "usage-upload", bytes.NewReader([]byte("usage file")), nil)
	if e != nil {
		t.Fatal(e)
	}
	if _, e = client.Upload(ctx, q, "usage-upload", bytes.NewReader([]byte("usage file")), nil); e != nil {
		t.Fatal(e)
	}
	var summary insights.Summary
	if alice.call(t, "GET", "/usage/summary", nil, &summary) != 200 || summary.Totals.Operations != 1 || summary.Totals.Effective != 10 || summary.Totals.Observed < 20 {
		t.Fatalf("incorrect accounting %+v", summary.Totals)
	}
	if len(summary.Capacities) != 1 || summary.Capacities[0].History == nil || summary.Capacities[0].Current != 10 {
		t.Fatal("capacity categories missing")
	}
	if bob.call(t, "GET", "/usage/summary?space_id="+alice.space.ID, nil, nil) != 404 {
		t.Fatal("cross-user space metrics leaked")
	}
	var limited insights.Summary
	if e = client.JSON(ctx, "GET", "/api/v1/usage/summary", "", nil, &limited); e != nil || len(limited.Capacities) != 0 || limited.Totals.Effective != 10 {
		t.Fatal("agent scope projection invalid")
	}
	if e = client.JSON(ctx, "POST", "/api/v1/usage/scans", "", map[string]any{"space_id": alice.space.ID, "enabled": true}, nil); e == nil {
		t.Fatal("Agent changed scan policy")
	}
	var scan insights.ScanRun
	if status := alice.call(t, "POST", "/usage/scans", map[string]any{"space_id": alice.space.ID, "enabled": true, "max_entries": 100, "max_seconds": 5}, &scan); status != 200 || scan.Status != "complete" || scan.Bytes != 10 {
		t.Fatalf("inventory failed: %d %+v", status, scan)
	}
	if _, e = fixture.Service.CallTool(ctx, service.Principal{UserID: alice.user.ID, Agent: &a}, "usage_scan", map[string]any{"space_id": alice.space.ID}); e == nil {
		t.Fatal("directory Agent scanned entire space")
	}
	full, _ := agent(t, alice, "", "read")
	if _, e = fixture.Service.CallTool(ctx, service.Principal{UserID: alice.user.ID, Agent: &full}, "usage_scan", map[string]any{"space_id": alice.space.ID}); e != nil {
		t.Fatal(e)
	}
	var before int64
	fixture.Service.DB.Model(&insights.AuditEvent{}).Where("owner = ?", alice.user.ID).Count(&before)
	for i := 0; i < 3; i++ {
		alice.call(t, "GET", "/audit/events", nil, nil)
		alice.call(t, "GET", "/usage/summary", nil, nil)
	}
	var after int64
	fixture.Service.DB.Model(&insights.AuditEvent{}).Where("owner = ?", alice.user.ID).Count(&after)
	if before != after {
		t.Fatal("query polling created audit events")
	}
	req, _ := http.NewRequest("GET", fixture.App.URL+"/api/v1/usage/summary", nil)
	req.Header.Set("Authorization", "Bearer fixture-invalid")
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	resp.Body.Close()
	var anonymous int64
	fixture.Service.DB.Model(&insights.AuditEvent{}).Where("owner = ? AND actor_kind = ?", "", "anonymous").Count(&anonymous)
	if anonymous != 1 {
		t.Fatal("anonymous denial not captured separately")
	}
	var page insights.AuditPage
	alice.call(t, "GET", "/audit/events", nil, &page)
	for _, ev := range page.Events {
		if ev.ActorKind == "anonymous" {
			t.Fatal("instance events leaked to user")
		}
	}
	fail := true
	fixture.Service.DB.Callback().Create().Before("gorm:create").Register("audit-fault", func(tx *gorm.DB) {
		if fail && tx.Statement.Table == "insight_audit" {
			tx.AddError(errors.New("fixture audit unavailable"))
		}
	})
	if status := alice.call(t, "POST", "/agents", map[string]any{"name": "blocked", "space_id": alice.space.ID, "scopes": "read", "days": 7}, nil); status != 503 {
		t.Fatalf("unaudited new authorization allowed: %d", status)
	}
	if status := alice.call(t, "POST", "/files/"+op.FileID+"/trash", map[string]any{}, nil); status != 503 {
		t.Fatalf("unaudited deletion allowed: %d", status)
	}
	if status := alice.call(t, "GET", "/files/"+op.FileID, nil, nil); status != 200 {
		t.Fatal("ordinary reads not degraded gracefully")
	}
	if status := alice.call(t, "POST", "/agents/"+a.ID+"/revoke", map[string]any{}, nil); status != 200 {
		t.Fatal("emergency revocation blocked by audit")
	}
	if e = client.JSON(ctx, "GET", "/api/v1/usage/summary", "", nil, nil); e == nil {
		t.Fatal("revoked token still accepted")
	}
	fail = false
	alice.call(t, "POST", "/files/"+op.FileID+"/trash", map[string]any{}, nil)
	alice.call(t, "GET", "/usage/summary", nil, &summary)
	if !summary.Collection.Gap {
		t.Fatal("audit gap hidden")
	}
}
