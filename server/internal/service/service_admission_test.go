package service_test

import (
	"bytes"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/fixtures"
	"github.com/yeisme/drivebridge-server/internal/service"
)

// admissionAuthority simulates the Aigora identity and service-billing edge:
// JWKS + authorize for short-lived EdDSA contexts, and the trusted reporter
// reserve/report endpoints for hosted business-operation admission.
type admissionAuthority struct {
	server      *httptest.Server
	priv        ed25519.PrivateKey
	failReserve string
	mu          sync.Mutex
	reserves    []map[string]any
	reports     []map[string]any
}

func (a *admissionAuthority) recorded() ([]map[string]any, []map[string]any) {
	a.mu.Lock()
	defer a.mu.Unlock()
	reserves := make([]map[string]any, len(a.reserves))
	copy(reserves, a.reserves)
	reports := make([]map[string]any, len(a.reports))
	copy(reports, a.reports)
	return reserves, reports
}

func newAdmissionAuthority(t *testing.T) *admissionAuthority {
	t.Helper()
	pub, priv, err := ed25519.GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	a := &admissionAuthority{priv: priv}
	identity := func() map[string]any {
		return map[string]any{"sub": "central-user", "tenant_id": "central-org", "actor_type": "human", "membership_version": 1, "iat": time.Now().Unix(), "exp": time.Now().Add(300 * time.Second).Unix(), "credential_context": map[string]any{"key_id": "key", "key_family_id": "family", "tenant_id": "central-org", "principal_type": "user", "authority_version": "v1", "audiences": []string{"drivebridge"}, "scopes": []string{"drivebridge:read", "drivebridge:write"}}}
	}
	a.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/.well-known/jwks.json":
			_ = json.NewEncoder(w).Encode(map[string]any{"keys": []map[string]string{{"kty": "OKP", "crv": "Ed25519", "kid": "fixture", "use": "sig", "alg": "EdDSA", "x": base64.RawURLEncoding.EncodeToString(pub)}}})
		case "/api/v1/service-access/authorize":
			_ = json.NewEncoder(w).Encode(map[string]any{"data": identity()})
		case "/internal/v1/service-billing/drivebridge/reserve":
			if r.Header.Get("Authorization") != "Bearer reporter-secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			a.mu.Lock()
			a.reserves = append(a.reserves, in)
			fail := a.failReserve
			a.mu.Unlock()
			if fail != "" {
				w.WriteHeader(http.StatusConflict)
				_ = json.NewEncoder(w).Encode(map[string]any{"error": map[string]string{"code": fail}})
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"operation_ref": "svc-" + in["operation_id"].(string), "status": "reserved", "reserved_units": in["units"]}})
		case "/internal/v1/service-billing/drivebridge/report":
			if r.Header.Get("Authorization") != "Bearer reporter-secret" {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			var in map[string]any
			_ = json.NewDecoder(r.Body).Decode(&in)
			a.mu.Lock()
			a.reports = append(a.reports, in)
			a.mu.Unlock()
			_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"operation_ref": in["operation_ref"], "status": "settled"}})
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(a.server.Close)
	return a
}

func (a *admissionAuthority) hostedToken(t *testing.T) string {
	t.Helper()
	now := time.Now().Unix()
	credential := map[string]any{"key_id": "key", "key_family_id": "family", "tenant_id": "central-org", "principal_type": "user", "authority_version": "v1", "audiences": []string{"drivebridge"}, "scopes": []string{"drivebridge:read", "drivebridge:write"}}
	claims := map[string]any{"sub": "central-user", "tenant_id": "central-org", "actor_type": "human", "membership_version": 1, "iat": now, "exp": now + 300, "credential_context": credential, "iss": a.server.URL, "aud": "drivebridge"}
	payload, _ := json.Marshal(claims)
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"EdDSA","kid":"fixture"}`))
	input := header + "." + base64.RawURLEncoding.EncodeToString(payload)
	return input + "." + base64.RawURLEncoding.EncodeToString(ed25519.Sign(a.priv, []byte(input)))
}

func admissionRestart(t *testing.T, f *fixtures.Runtime, a *admissionAuthority, credential string) {
	t.Helper()
	original := f.Service.Config
	original.ServiceIdentityIssuer = a.server.URL
	original.ServiceAdmissionCredential = credential
	original.Development = true
	f.Service.Close()
	reopened, e := service.Open(original)
	if e != nil {
		t.Fatal(e)
	}
	f.Service = reopened
}

// admissionSetup restarts the fixture runtime with unified identity admission
// attached and seeds a local owner, space and exact identity mapping.
func admissionSetup(t *testing.T, a *admissionAuthority) (*fixtures.Runtime, service.Space, string) {
	t.Helper()
	f, e := fixtures.Start(t.TempDir(), "", "")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(f.Close)
	admissionRestart(t, f, a, "reporter-secret")
	user := service.User{ID: "local-user", Issuer: "fixture", Subject: "fixture-user", Name: "Local"}
	if e := f.Service.DB.Create(&user).Error; e != nil {
		t.Fatal(e)
	}
	space := service.Space{ID: "local-space", Owner: user.ID, Name: "Local", ConnectionID: "platform", MaxBytes: 32 << 20}
	if e := f.Service.DB.Create(&space).Error; e != nil {
		t.Fatal(e)
	}
	if e := f.Service.DB.Create(&service.ServiceIdentityBinding{ID: "admission-binding", Issuer: f.Service.Config.ServiceIdentityIssuer, Subject: "central-user", IdentityTenant: "central-org", UserID: user.ID, TenantID: "local-org", Active: true}).Error; e != nil {
		t.Fatal(e)
	}
	return f, space, a.hostedToken(t)
}

func admissionSend(t *testing.T, f *fixtures.Runtime, method, path, token string, body []byte, idempotency string) (int, map[string]any) {
	t.Helper()
	app := httptest.NewServer(f.Service.Handler())
	defer app.Close()
	req, e := http.NewRequest(method, app.URL+path, bytes.NewReader(body))
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if idempotency != "" {
		req.Header.Set("Idempotency-Key", idempotency)
	}
	res, e := app.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	payload, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	out := map[string]any{}
	_ = json.Unmarshal(payload, &out)
	return res.StatusCode, out
}

func admissionError(t *testing.T, body map[string]any) string {
	t.Helper()
	if e, ok := body["error"].(map[string]any); ok {
		if code, ok := e["code"].(string); ok {
			return code
		}
	}
	t.Fatalf("expected error envelope, got %v", body)
	return ""
}

func admissionUpload(t *testing.T, f *fixtures.Runtime, space service.Space, token, path, entitlement, key string) (int, map[string]any) {
	t.Helper()
	body := map[string]any{"space_id": space.ID, "path": path, "size_bytes": 12, "sha256": service.Hash("hosted-bytes")}
	if entitlement != "" {
		body["entitlement_id"] = entitlement
	}
	encoded, _ := json.Marshal(body)
	return admissionSend(t, f, "POST", "/api/v1/uploads", token, encoded, key)
}

func admissionPart(t *testing.T, f *fixtures.Runtime, operationID, token string) int {
	t.Helper()
	app := httptest.NewServer(f.Service.Handler())
	defer app.Close()
	req, e := http.NewRequest("PUT", app.URL+"/api/v1/operations/"+operationID+"/parts/0", strings.NewReader("hosted-bytes"))
	if e != nil {
		t.Fatal(e)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	res, e := app.Client().Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer res.Body.Close()
	io.Copy(io.Discard, res.Body)
	return res.StatusCode
}

func TestServiceAdmissionLifecycleReportsDeliveryAndCancellation(t *testing.T) {
	a := newAdmissionAuthority(t)
	f, space, token := admissionSetup(t, a)
	status, out := admissionUpload(t, f, space, token, "hosted.bin", "ent-1", "admit-1")
	if status != 200 {
		t.Fatalf("upload status=%d body=%v", status, out)
	}
	operation, _ := out["data"].(map[string]any)
	if operation == nil || operation["id"] == "" {
		t.Fatalf("operation missing: %v", out)
	}
	opID := operation["id"].(string)
	reserves, _ := a.recorded()
	if len(reserves) != 1 || reserves[0]["entitlement_id"] != "ent-1" || reserves[0]["units"] != float64(12) || reserves[0]["operation_id"] != opID || reserves[0]["principal_context"] != token {
		t.Fatalf("reserve intent %+v", reserves)
	}
	var row service.ServiceAdmission
	if e := f.Service.DB.First(&row, "id = ?", opID).Error; e != nil || row.Status != "reserved" || row.OperationRef == "" || row.TenantID != "local-org" {
		t.Fatalf("admission row %+v err=%v", row, e)
	}
	if code := admissionPart(t, f, opID, token); code != 200 {
		t.Fatalf("part status=%d", code)
	}
	status, out = admissionSend(t, f, "POST", "/api/v1/operations/"+opID+"/complete", token, []byte("{}"), "")
	if status != 200 {
		t.Fatalf("complete status=%d body=%v", status, out)
	}
	_, reports := a.recorded()
	if len(reports) != 1 || reports[0]["outcome"] != "delivered" || reports[0]["actual_units"] != float64(12) || reports[0]["operation_ref"] != row.OperationRef {
		t.Fatalf("reports %+v", reports)
	}
	if e := f.Service.DB.First(&row, "id = ?", opID).Error; e != nil || row.Status != "reported" || row.Outcome != "delivered" {
		t.Fatalf("reported row %+v err=%v", row, e)
	}
	// A cancelled operation releases its reservation with an explicit failure.
	status, out = admissionUpload(t, f, space, token, "cancelled.bin", "ent-1", "admit-2")
	if status != 200 {
		t.Fatalf("second upload status=%d body=%v", status, out)
	}
	second := out["data"].(map[string]any)["id"].(string)
	status, out = admissionSend(t, f, "POST", "/api/v1/operations/"+second+"/cancel", token, []byte("{}"), "")
	if status != 200 {
		t.Fatalf("cancel status=%d body=%v", status, out)
	}
	_, reports = a.recorded()
	if len(reports) != 2 || reports[1]["outcome"] != "failed" || reports[1]["actual_units"] != float64(0) {
		t.Fatalf("cancel reports %+v", reports)
	}
	// Idempotent replays never reserve twice.
	status, out = admissionUpload(t, f, space, token, "hosted.bin", "ent-1", "admit-1")
	if status != 200 || out["data"].(map[string]any)["id"] != opID {
		t.Fatalf("replay status=%d body=%v", status, out)
	}
	reserves, _ = a.recorded()
	if len(reserves) != 2 {
		t.Fatalf("replay reserved again: %d", len(reserves))
	}
}

func TestServiceAdmissionFailsClosedAndMapsRemoteDenials(t *testing.T) {
	a := newAdmissionAuthority(t)
	f, space, token := admissionSetup(t, a)
	// No entitlement reference: the write is unmetered and must be rejected.
	status, out := admissionUpload(t, f, space, token, "no-ent.bin", "", "no-ent")
	if status != 503 || admissionError(t, out) != "service_admission_required" {
		t.Fatalf("unmetered write status=%d body=%v", status, out)
	}
	// Unrelated hosted writes stay closed even with a write scope.
	status, out = admissionSend(t, f, "POST", "/api/v1/spaces", token, []byte(`{"name":"New"}`), "")
	if status != 503 || admissionError(t, out) != "service_admission_required" {
		t.Fatalf("unrelated write status=%d body=%v", status, out)
	}
	// Reads under the same short-lived context remain available.
	status, out = admissionSend(t, f, "GET", "/api/v1/me", token, nil, "")
	if status != 200 {
		t.Fatalf("me status=%d body=%v", status, out)
	}
	// Remote quota denial is surfaced, and no local operation appears.
	a.mu.Lock()
	a.failReserve = "quota_exceeded"
	a.mu.Unlock()
	status, out = admissionUpload(t, f, space, token, "denied.bin", "ent-1", "denied")
	if status != 413 || admissionError(t, out) != "quota_exceeded" {
		t.Fatalf("remote denial status=%d body=%v", status, out)
	}
	var count int64
	f.Service.DB.Model(&service.Operation{}).Where("path = ?", "denied.bin").Count(&count)
	if count != 0 {
		t.Fatal("denied operation persisted")
	}
	a.mu.Lock()
	a.failReserve = "insufficient_funds"
	a.mu.Unlock()
	status, out = admissionUpload(t, f, space, token, "funds.bin", "ent-1", "funds")
	if status != 409 || admissionError(t, out) != "insufficient_funds" {
		t.Fatalf("insufficient funds status=%d body=%v", status, out)
	}
	// Without a reporter credential the deployment stays fail-closed.
	a.mu.Lock()
	a.failReserve = ""
	a.mu.Unlock()
	admissionRestart(t, f, a, "")
	status, out = admissionUpload(t, f, space, token, "closed.bin", "ent-1", "closed")
	if status != 503 || admissionError(t, out) != "service_admission_required" {
		t.Fatalf("fail-closed status=%d body=%v", status, out)
	}
}

func TestServiceAdmissionRecoveryReconciliation(t *testing.T) {
	a := newAdmissionAuthority(t)
	f, space, token := admissionSetup(t, a)
	// Completed locally while the report could not be delivered: restart
	// reconciliation settles it as delivered.
	status, out := admissionUpload(t, f, space, token, "recovered.bin", "ent-1", "recover-1")
	if status != 200 {
		t.Fatalf("upload status=%d body=%v", status, out)
	}
	delivered := out["data"].(map[string]any)["id"].(string)
	if e := f.Service.DB.Model(&service.Operation{}).Where("id = ?", delivered).Update("status", "completed").Error; e != nil {
		t.Fatal(e)
	}
	// A reserved operation whose local record vanished is reported as an
	// unknown result so the authority holds it for manual settlement.
	status, out = admissionUpload(t, f, space, token, "vanished.bin", "ent-1", "recover-2")
	if status != 200 {
		t.Fatalf("upload status=%d body=%v", status, out)
	}
	vanished := out["data"].(map[string]any)["id"].(string)
	if e := f.Service.DB.Delete(&service.Operation{}, "id = ?", vanished).Error; e != nil {
		t.Fatal(e)
	}
	// A locally failed operation stays recoverable: reconciliation holds it.
	status, out = admissionUpload(t, f, space, token, "held.bin", "ent-1", "recover-3")
	if status != 200 {
		t.Fatalf("upload status=%d body=%v", status, out)
	}
	held := out["data"].(map[string]any)["id"].(string)
	if e := f.Service.DB.Model(&service.Operation{}).Where("id = ?", held).Update("status", "failed").Error; e != nil {
		t.Fatal(e)
	}
	admissionRestart(t, f, a, "reporter-secret")
	_, reports := a.recorded()
	if len(reports) != 2 {
		t.Fatalf("reconciliation reports %+v", reports)
	}
	outcomes := map[string]string{}
	for _, report := range reports {
		outcomes[report["operation_ref"].(string)] = report["outcome"].(string)
	}
	if outcomes["svc-"+delivered] != "delivered" || outcomes["svc-"+vanished] != "unknown_accept" {
		t.Fatalf("outcomes %v", outcomes)
	}
	var heldRow service.ServiceAdmission
	if e := f.Service.DB.First(&heldRow, "id = ?", held).Error; e != nil || heldRow.Status != "reserved" {
		t.Fatalf("held row %+v err=%v", heldRow, e)
	}
}

// A reservation made at the authority whose local transaction then fails
// must be settled from memory: the rolled-back journal row means neither the
// success-path report nor reconcile would ever find it.
func TestServiceAdmissionReportsFailedReservationWhenUploadTxFails(t *testing.T) {
	a := newAdmissionAuthority(t)
	f, space, token := admissionSetup(t, a)
	if e := f.Service.DB.Model(&service.Space{}).Where("id = ?", space.ID).Update("max_bytes", 4).Error; e != nil {
		t.Fatal(e)
	}
	status, out := admissionUpload(t, f, space, token, "leak.bin", "ent-1", "leak-1")
	if status == 200 || admissionError(t, out) != "quota_exceeded" {
		t.Fatalf("expected quota_exceeded, status=%d out=%v", status, out)
	}
	reserves, reports := a.recorded()
	if len(reserves) != 1 {
		t.Fatalf("reserves %+v", reserves)
	}
	if len(reports) != 1 || reports[0]["outcome"] != "failed" || reports[0]["actual_units"] != float64(0) || !strings.HasPrefix(reports[0]["operation_ref"].(string), "svc-") {
		t.Fatalf("leaked reservation: reports %+v", reports)
	}
	var rows int64
	f.Service.DB.Model(&service.ServiceAdmission{}).Count(&rows)
	if rows != 0 {
		t.Fatalf("journal row survived rollback: %d", rows)
	}
}
