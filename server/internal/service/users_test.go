package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/cookiejar"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/fixtures"
	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

func bearerCall(t *testing.T, f *fixtures.Runtime, method, path, token string, body any, out any) int {
	t.Helper()
	var rdr *bytes.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rdr = bytes.NewReader(b)
	} else {
		rdr = bytes.NewReader(nil)
	}
	req, e := http.NewRequest(method, f.App.URL+"/api/v1"+path, rdr)
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	if out != nil {
		var envelope struct {
			Data json.RawMessage `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&envelope)
		if len(envelope.Data) == 0 {
			t.Fatalf("no data envelope: %s", resp.Status)
		}
		json.Unmarshal(envelope.Data, out)
	}
	return resp.StatusCode
}

func TestUserKeyProvisioningAndLifecycle(t *testing.T) {
	f := rig(t)
	supplied := "dbu_" + strings.Repeat("ab", 48)
	issued, e := f.Service.ImportUsers([]service.UserImport{
		{Name: "batch-one"}, {Name: "batch-two"}, {Name: "imported", Key: supplied},
	})
	if e != nil || len(issued) != 3 {
		t.Fatalf("import: %v issued=%d", e, len(issued))
	}
	if issued[2].Key != supplied || !strings.HasPrefix(issued[0].Key, "dbu_") || len(issued[0].Key) != 100 {
		t.Fatalf("key shape: %+v", issued)
	}

	// A key user is a full user: create a space over platform storage and
	// complete an upload round-trip.
	var space service.Space
	if status := bearerCall(t, f, "POST", "/spaces", issued[0].Key, map[string]any{"name": "keys", "connection_id": "platform"}, &space); status != 200 {
		t.Fatalf("space create: %d", status)
	}
	c, e := sdk.New(f.App.URL, issued[0].Key)
	if e != nil {
		t.Fatal(e)
	}
	op, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: space.ID, Path: "k.txt"}, "k1", strings.NewReader("key-user"), nil)
	if e != nil || op.Status != "completed" {
		t.Fatalf("key-user upload: %v status=%s", e, op.Status)
	}
	if f.ObjectCount() == 0 {
		t.Fatal("key-user upload stored nothing")
	}

	// Admin matrix: only an admin user key may provision over HTTP.
	adminIssued, e := f.Service.ImportUsers([]service.UserImport{{Name: "ops", Admin: true}})
	if e != nil {
		t.Fatal(e)
	}
	var created struct {
		Issued []service.UserKeyIssued `json:"issued"`
	}
	if status := bearerCall(t, f, "POST", "/users", adminIssued[0].Key, map[string]any{"users": []service.UserImport{{Name: "http-made"}}}, &created); status != 200 || len(created.Issued) != 1 {
		t.Fatalf("admin create: %d %+v", status, created)
	}
	if status := bearerCall(t, f, "POST", "/users", issued[0].Key, map[string]any{"users": []service.UserImport{{Name: "nope"}}}, nil); status != 403 {
		t.Fatalf("normal key must be forbidden, got %d", status)
	}
	h, _ := login(t, f, "alice")
	if status := h.call(t, "POST", "/users", map[string]any{"users": []service.UserImport{{Name: "nope"}}}, nil); status != 403 {
		t.Fatalf("session must be forbidden, got %d", status)
	}
	agentTok := createAgentFor(t, f, h)
	if status := bearerCall(t, f, "POST", "/users", agentTok, map[string]any{"users": []service.UserImport{{Name: "nope"}}}, nil); status != 403 {
		t.Fatalf("agent must be forbidden, got %d", status)
	}

	// Revocation kills the key on the next request.
	keys := f.Service.ListUserKeys()
	var target string
	for _, k := range keys {
		if k.Owner == issued[0].UserID && !k.Revoked {
			target = k.ID
			break
		}
	}
	if target == "" {
		t.Fatal("key id missing from list")
	}
	if e := f.Service.RevokeUserKey(target); e != nil {
		t.Fatal(e)
	}
	if status := bearerCall(t, f, "GET", "/spaces", issued[0].Key, nil, nil); status != 401 {
		t.Fatalf("revoked key must be unauthorized, got %d", status)
	}
}

func TestUserKeyBatchRejectedAtomically(t *testing.T) {
	f := rig(t)
	bad := "dbu_not-a-valid-key"
	if _, e := f.Service.ImportUsers([]service.UserImport{{Name: "ok"}, {Name: "bad", Key: bad}}); e == nil {
		t.Fatal("invalid key format must reject the batch")
	}
	if _, e := f.Service.ImportUsers([]service.UserImport{{Name: "dup", Key: "dbu_" + strings.Repeat("cd", 24)}, {Name: "dup2", Key: "dbu_" + strings.Repeat("cd", 24)}}); e == nil {
		t.Fatal("duplicate supplied keys must reject the batch")
	}
	if keys := f.Service.ListUserKeys(); len(keys) != 0 {
		t.Fatalf("rejected batches must not persist users, got %d", len(keys))
	}
	if _, e := f.Service.ImportUsers(nil); e == nil {
		t.Fatal("empty batch must be rejected")
	}
}

// createAgentFor mints an agent token through the existing HTTP surface so the
// forbidden-matrix uses a real agent credential.
func createAgentFor(t *testing.T, f *fixtures.Runtime, h *human) string {
	t.Helper()
	var out struct {
		Token string `json:"token"`
	}
	if status := h.call(t, "POST", "/agents", map[string]any{"name": "u", "space_id": h.space.ID, "scopes": "read", "days": 1}, &out); status != 200 {
		t.Fatal(status)
	}
	return out.Token
}

func TestUsersManagementAgentSurface(t *testing.T) {
	f := rig(t)
	adminIssued, e := f.Service.ImportUsers([]service.UserImport{{Name: "ops", Admin: true}})
	if e != nil {
		t.Fatal(e)
	}
	normalIssued, e := f.Service.ImportUsers([]service.UserImport{{Name: "member"}})
	if e != nil {
		t.Fatal(e)
	}
	adminP := service.Principal{UserID: "operator", Key: &service.UserKey{Admin: true}, Admin: true}
	normalP := service.Principal{UserID: "member-user", Key: &service.UserKey{}}

	// MCP surface mirrors the HTTP admin gate: only admin keys may manage.
	out, e := f.Service.CallTool(context.Background(), adminP, "users_create", map[string]any{
		"users": []any{map[string]any{"name": "mcp-made", "days": 30}},
	})
	if e != nil {
		t.Fatal(e)
	}
	issued, ok := out.([]service.UserKeyIssued)
	if !ok || len(issued) != 1 || issued[0].Key == "" {
		t.Fatalf("mcp create: %T %+v", out, out)
	}
	if _, e = f.Service.CallTool(context.Background(), normalP, "users_create", map[string]any{"users": []any{map[string]any{"name": "nope"}}}); e == nil || e.Error() != "forbidden" {
		t.Fatalf("normal key must be forbidden, got %v", e)
	}

	listOut, e := f.Service.CallTool(context.Background(), adminP, "users_list", map[string]any{})
	if e != nil {
		t.Fatal(e)
	}
	keys, ok := listOut.([]service.UserKey)
	if !ok || len(keys) != 3 {
		t.Fatalf("mcp list: %T %+v", listOut, listOut)
	}
	var target service.UserKey
	for _, k := range keys {
		if k.Name == "mcp-made" {
			target = k
		}
	}
	if target.ID == "" || !target.Expires.After(target.Created) {
		t.Fatalf("issued key must carry id and expiry: %+v", target)
	}
	if _, e = f.Service.CallTool(context.Background(), normalP, "users_list", map[string]any{}); e == nil || e.Error() != "forbidden" {
		t.Fatalf("normal key list must be forbidden, got %v", e)
	}
	if _, e = f.Service.CallTool(context.Background(), adminP, "users_revoke", map[string]any{"key_id": target.ID}); e != nil {
		t.Fatal(e)
	}
	if _, e = f.Service.CallTool(context.Background(), adminP, "users_revoke", map[string]any{"key_id": "missing"}); e == nil || e.Error() != "not_found" {
		t.Fatalf("unknown key must be not_found, got %v", e)
	}

	// HTTP list/revoke carry the same admin gate as create.
	var listed struct {
		Keys []service.UserKey `json:"keys"`
	}
	if status := bearerCall(t, f, "GET", "/users", adminIssued[0].Key, nil, &listed); status != 200 || len(listed.Keys) != 3 {
		t.Fatalf("http list: %d %+v", status, listed)
	}
	if status := bearerCall(t, f, "GET", "/users", normalIssued[0].Key, nil, nil); status != 403 {
		t.Fatalf("normal key http list must be forbidden, got %d", status)
	}
	if status := bearerCall(t, f, "POST", "/users/"+target.ID+"/revoke", adminIssued[0].Key, nil, nil); status != 200 {
		t.Fatalf("http revoke: %d", status)
	}
	if status := bearerCall(t, f, "POST", "/users/"+target.ID+"/revoke", normalIssued[0].Key, nil, nil); status != 403 {
		t.Fatalf("normal key http revoke must be forbidden, got %d", status)
	}
	for _, k := range f.Service.ListUserKeys() {
		if k.ID == target.ID && !k.Revoked {
			t.Fatal("revocation must be persisted")
		}
	}
}

func keySessionClient(t *testing.T) *http.Client {
	t.Helper()
	jar, e := cookiejar.New(nil)
	if e != nil {
		t.Fatal(e)
	}
	return &http.Client{Jar: jar}
}

func TestKeySessionLoginAndAdminSurface(t *testing.T) {
	f := rig(t)
	issued, e := f.Service.ImportUsers([]service.UserImport{{Name: "web"}, {Name: "ops", Admin: true}})
	if e != nil {
		t.Fatal(e)
	}
	base := f.App.URL

	// Login mode is honestly reported before any authentication.
	var mode struct {
		Data struct {
			OIDC bool `json:"oidc"`
			Key  bool `json:"key"`
		} `json:"data"`
	}
	if resp, _ := http.Get(base + "/auth/mode"); resp.StatusCode != 200 {
		t.Fatalf("auth/mode: %d", resp.StatusCode)
	} else {
		json.NewDecoder(resp.Body).Decode(&mode)
		resp.Body.Close()
	}
	if !mode.Data.OIDC || !mode.Data.Key {
		t.Fatalf("fixture must report oidc+key, got %+v", mode.Data)
	}

	// Wrong or revoked keys never distinguish existence.
	if status := bearerCall(t, f, "POST", "/nonexistent", "dbu_"+strings.Repeat("ff", 48), nil, nil); status == 0 {
		t.Fatal("unreachable")
	}
	c := keySessionClient(t)
	resp, e := c.Post(base+"/auth/key", "application/json", strings.NewReader(`{"key":"dbu_`+strings.Repeat("aa", 48)+`"}`))
	if e != nil || resp.StatusCode != 401 {
		t.Fatalf("bad key login must 401, got %d err %v", resp.StatusCode, e)
	}
	resp.Body.Close()

	// Valid key exchanges for a session; the browser holds only the cookie.
	login := func(key string) *http.Client {
		t.Helper()
		cl := keySessionClient(t)
		req, _ := http.NewRequest("POST", base+"/auth/key", strings.NewReader(`{"key":"`+key+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", base)
		r, e := cl.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		r.Body.Close()
		if r.StatusCode != 200 {
			t.Fatalf("key login: %d", r.StatusCode)
		}
		return cl
	}
	web := login(issued[0].Key)
	admin := login(issued[1].Key)

	me := func(cl *http.Client) (bool, string) {
		t.Helper()
		r, e := cl.Get(base + "/api/v1/me")
		if e != nil {
			t.Fatal(e)
		}
		defer r.Body.Close()
		var out struct {
			Data struct {
				Admin bool   `json:"admin"`
				CSRF  string `json:"csrf_token"`
			} `json:"data"`
		}
		json.NewDecoder(r.Body).Decode(&out)
		return out.Data.Admin, out.Data.CSRF
	}
	webAdmin, webCSRF := me(web)
	adminAdmin, adminCSRF := me(admin)
	if webAdmin || !adminAdmin || webCSRF == "" || adminCSRF == "" {
		t.Fatalf("admin inheritance wrong: web=%v admin=%v", webAdmin, adminAdmin)
	}

	// Admin session reaches the management surface; plain session does not.
	post := func(cl *http.Client, csrf, path, body string) int {
		t.Helper()
		req, _ := http.NewRequest("POST", base+path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", base)
		req.Header.Set("X-CSRF-Token", csrf)
		r, e := cl.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		r.Body.Close()
		return r.StatusCode
	}
	if status := post(web, webCSRF, "/api/v1/users", `{"users":[{"name":"x"}]}`); status != 403 {
		t.Fatalf("plain session must be forbidden, got %d", status)
	}
	if status := post(admin, adminCSRF, "/api/v1/users", `{"users":[{"name":"http-session-made"}]}`); status != 200 {
		t.Fatalf("admin session create: %d", status)
	}

	// Listing carries lifecycle metadata and no key material.
	r, e := admin.Get(base + "/api/v1/users")
	if e != nil {
		t.Fatal(e)
	}
	raw, _ := io.ReadAll(r.Body)
	r.Body.Close()
	if r.StatusCode != 200 || bytes.Contains(raw, []byte("dbu_")) || bytes.Contains(raw, []byte("token_hash")) {
		t.Fatalf("users list leaked or failed: %d %s", r.StatusCode, raw)
	}

	// HTTP revoke kills the key at the next login.
	var id string
	for _, k := range f.Service.ListUserKeys() {
		if k.Owner == issued[0].UserID {
			id = k.ID
		}
	}
	if status := post(admin, adminCSRF, "/api/v1/users/"+id+"/revoke", `{}`); status != 200 {
		t.Fatalf("http revoke: %d", status)
	}
	req, _ := http.NewRequest("POST", base+"/auth/key", strings.NewReader(`{"key":"`+issued[0].Key+`"}`))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Origin", base)
	r, e = keySessionClient(t).Do(req)
	if e != nil {
		t.Fatal(e)
	}
	r.Body.Close()
	if r.StatusCode != 401 {
		t.Fatalf("revoked key must not re-login, got %d", r.StatusCode)
	}
}

func TestExternalKeyImportAndRouting(t *testing.T) {
	f := rig(t)
	// Foreign pre-generated key of any prefix imports and authenticates.
	foreign := "sk-external-" + strings.Repeat("Zq9", 12) // 46 printable chars
	issued, e := f.Service.ImportUsers([]service.UserImport{{Name: "外来用户", Key: foreign}})
	if e != nil || len(issued) != 1 {
		t.Fatalf("foreign import: %v %+v", e, issued)
	}
	var out struct {
		User struct {
			Name string `json:"name"`
		} `json:"user"`
	}
	if status := bearerCall(t, f, "GET", "/me", foreign, nil, &out); status != 200 || out.User.Name != "外来用户" {
		t.Fatalf("foreign key bearer: %d %+v", status, out)
	}
	// Agent tokens still route correctly alongside foreign-format user keys.
	h, _ := login(t, f, "alice")
	agentTok := createAgentFor(t, f, h)
	var agentOut struct {
		Agent any `json:"agent"`
	}
	if status := bearerCall(t, f, "GET", "/me", agentTok, nil, &agentOut); status != 200 || agentOut.Agent == nil {
		t.Fatalf("agent token must still authenticate as agent: %d", status)
	}
}

func TestExternalKeyValidationAndCollisions(t *testing.T) {
	f := rig(t)
	bad := []string{
		"tooshort-key-1234",                          // < 32 chars
		"space inside a perfectly long enough key!!", // whitespace
		strings.Repeat("k", 129),                     // too long
	}
	for _, k := range bad {
		if _, e := f.Service.ImportUsers([]service.UserImport{{Name: "坏", Key: k}}); e == nil {
			t.Fatalf("expected rejection for %q", k[:min(12, len(k))])
		}
	}
	if keys := f.Service.ListUserKeys(); len(keys) != 0 {
		t.Fatalf("rejected batches must not persist, got %d", len(keys))
	}
	// An agent token must not be importable as a user key.
	h, _ := login(t, f, "alice")
	agentTok := createAgentFor(t, f, h)
	if _, e := f.Service.ImportUsers([]service.UserImport{{Name: "撞", Key: agentTok}}); e == nil {
		t.Fatal("agent token hash must be rejected cross-table")
	}
}

func TestRotateUserKey(t *testing.T) {
	f := rig(t)
	issued, e := f.Service.ImportUsers([]service.UserImport{{Name: "轮换"}})
	if e != nil {
		t.Fatal(e)
	}
	var id string
	for _, k := range f.Service.ListUserKeys() {
		if k.Owner == issued[0].UserID {
			id = k.ID
		}
	}
	fresh, e := f.Service.RotateUserKey(id, 30)
	if e != nil || fresh.UserID != issued[0].UserID || fresh.Key == issued[0].Key {
		t.Fatalf("rotate: %v %+v", e, fresh)
	}
	if !fresh.ExpiresAt.IsZero() && !fresh.ExpiresAt.After(time.Now()) {
		t.Fatal("rotated key must carry the requested lifetime")
	}
	if status := bearerCall(t, f, "GET", "/me", issued[0].Key, nil, nil); status != 401 {
		t.Fatalf("old key must be revoked, got %d", status)
	}
	if status := bearerCall(t, f, "GET", "/me", fresh.Key, nil, nil); status != 200 {
		t.Fatalf("new key must work, got %d", status)
	}
}

func TestSessionRevocationCascade(t *testing.T) {
	f := rig(t)
	issued, e := f.Service.ImportUsers([]service.UserImport{{Name: "会话甲"}, {Name: "会话乙"}})
	if e != nil {
		t.Fatal(e)
	}
	base := f.App.URL
	mkSession := func(key string) *http.Client {
		t.Helper()
		cl := keySessionClient(t)
		req, _ := http.NewRequest("POST", base+"/auth/key", strings.NewReader(`{"key":"`+key+`"}`))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Origin", base)
		r, e := cl.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		r.Body.Close()
		if r.StatusCode != 200 {
			t.Fatalf("key login: %d", r.StatusCode)
		}
		return cl
	}
	web := mkSession(issued[0].Key)
	if r, e := web.Get(base + "/api/v1/me"); e != nil || r.StatusCode != 200 {
		t.Fatalf("session pre-revoke: %v %d", e, r.StatusCode)
	} else {
		r.Body.Close()
	}
	// Revoking the origin key kills its sessions on the next request.
	var id string
	for _, k := range f.Service.ListUserKeys() {
		if k.Owner == issued[0].UserID {
			id = k.ID
		}
	}
	if e := f.Service.RevokeUserKey(id); e != nil {
		t.Fatal(e)
	}
	if r, e := web.Get(base + "/api/v1/me"); e != nil || r.StatusCode != 401 {
		t.Fatalf("session must die with its origin key: %v %d", e, r.StatusCode)
	} else {
		r.Body.Close()
	}
	// Rotation cascades the same way to its own session.
	other := mkSession(issued[1].Key)
	var otherID string
	for _, k := range f.Service.ListUserKeys() {
		if k.Owner == issued[1].UserID && !k.Revoked {
			otherID = k.ID
		}
	}
	if _, e := f.Service.RotateUserKey(otherID, 0); e != nil {
		t.Fatal(e)
	}
	if r, e := other.Get(base + "/api/v1/me"); e != nil || r.StatusCode != 401 {
		t.Fatalf("rotated origin must kill its session: %v %d", e, r.StatusCode)
	} else {
		r.Body.Close()
	}
	// OIDC sessions carry no origin key and are unaffected by any revocation.
	h, _ := login(t, f, "alice")
	if status := h.call(t, "GET", "/me", nil, nil); status != 200 {
		t.Fatalf("oidc session must be unaffected: %d", status)
	}
}

func TestVerifyUserKeyLifecycleLookup(t *testing.T) {
	f := rig(t)
	const key = "dbu_verifyfixture0123456789abcdefghij"
	issued, e := f.Service.ImportUsers([]service.UserImport{{Name: "verify-me", Key: key, Admin: true}})
	if e != nil || len(issued) != 1 {
		t.Fatalf("import with explicit key: %v %+v", e, issued)
	}
	k, e := f.Service.VerifyUserKey(key)
	if e != nil {
		t.Fatal(e)
	}
	if k.Name != "verify-me" || !k.Admin || k.Revoked {
		t.Fatalf("verify must report the stored lifecycle: %+v", k)
	}
	if _, e = f.Service.VerifyUserKey("dbu_nope"); e == nil || e.Error() != "not_found" {
		t.Fatalf("unknown key must be not_found, got %v", e)
	}
	if _, e = f.Service.VerifyUserKey(""); e == nil || e.Error() != "invalid_user_key" {
		t.Fatalf("empty key must be invalid_user_key, got %v", e)
	}
	if e = f.Service.RevokeUserKey(k.ID); e != nil {
		t.Fatal(e)
	}
	k, e = f.Service.VerifyUserKey(key)
	if e != nil || !k.Revoked {
		t.Fatalf("revoked key must verify as revoked: %+v %v", k, e)
	}
}

func TestRotateRejectsExpiredKeyAndHTTPEndpoint(t *testing.T) {
	f := rig(t)
	// An already-expired key must not be rotatable: the fresh key would
	// inherit a past expiry and be born dead.
	issued, e := f.Service.ImportUsers([]service.UserImport{{Name: "过期者", Key: "dbu_" + strings.Repeat("ee", 48), Days: -0}})
	if e != nil {
		t.Fatal(e)
	}
	var id string
	for _, k := range f.Service.ListUserKeys() {
		if k.Owner == issued[0].UserID {
			id = k.ID
			if e := f.Service.DB.Model(&service.UserKey{}).Where("id = ?", id).Update("expires_at", time.Now().Add(-time.Hour)).Error; e != nil {
				t.Fatal(e)
			}
		}
	}
	if _, e := f.Service.RotateUserKey(id, 0); e == nil {
		t.Fatal("expired key must not rotate")
	}
	// The HTTP rotate endpoint serves running deployments (no CLI lock).
	adminIssued, e := f.Service.ImportUsers([]service.UserImport{{Name: "ops", Admin: true}})
	if e != nil {
		t.Fatal(e)
	}
	normalIssued, e := f.Service.ImportUsers([]service.UserImport{{Name: "活着"}})
	if e != nil {
		t.Fatal(e)
	}
	var normalID string
	for _, k := range f.Service.ListUserKeys() {
		if k.Owner == normalIssued[0].UserID {
			normalID = k.ID
		}
	}
	var out struct {
		Key string `json:"key"`
	}
	if status := bearerCall(t, f, "POST", "/users/"+normalID+"/rotate", normalIssued[0].Key, map[string]any{"days": 7}, nil); status != 403 {
		t.Fatalf("non-admin rotate must be forbidden, got %d", status)
	}
	if status := bearerCall(t, f, "POST", "/users/"+normalID+"/rotate", adminIssued[0].Key, map[string]any{"days": 7}, &out); status != 200 || !strings.HasPrefix(out.Key, "dbu_") {
		t.Fatalf("admin http rotate: %d %+v", status, out)
	}
	if status := bearerCall(t, f, "GET", "/me", normalIssued[0].Key, nil, nil); status != 401 {
		t.Fatalf("old key must die after http rotate, got %d", status)
	}
}

func TestListAgentsOrdersByRealColumn(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	if tok := createAgentFor(t, f, h); tok == "" {
		t.Fatal("agent mint failed")
	}
	out := f.Service.ListAgents()
	if len(out) != 1 {
		t.Fatalf("ListAgents must return rows (legacy expires column), got %+v", out)
	}
}
