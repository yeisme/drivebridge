package desktop

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
)

func facadeRequest(t *testing.T, f *facade, method, path, body string, headers map[string]string) (int, []byte) {
	t.Helper()
	request, err := http.NewRequest(method, f.address+path, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+f.token)
	request.Header.Set("Content-Type", "application/json")
	for key, value := range headers {
		if key == "Host" {
			request.Host = value
		} else {
			request.Header.Set(key, value)
		}
	}
	transport := &http.Transport{Proxy: nil}
	defer transport.CloseIdleConnections()
	client := &http.Client{Transport: transport, Timeout: 5 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal("facade request failed")
	}
	defer response.Body.Close()
	if response.Header.Get("Access-Control-Allow-Origin") != "" || response.Header.Get("Cache-Control") != "private, no-store" {
		t.Fatal("unsafe response headers")
	}
	data, err := io.ReadAll(io.LimitReader(response.Body, 65537))
	if err != nil || len(data) > 65536 {
		t.Fatal("response budget")
	}
	return response.StatusCode, data
}
func TestDesktopSessionFacadeBoundaries(t *testing.T) {
	f, err := startFacade(context.Background(), testBinary(t))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	status, data := facadeRequest(t, f, "GET", "/v1/desktop/capabilities", "", nil)
	if status != 200 || strings.Contains(string(data), "confirmBinding") {
		t.Fatal("capabilities imply writable binding")
	}
	for _, headers := range []map[string]string{{"Authorization": ""}, {"Authorization": "Bearer wrong"}, {"Origin": "http://example.invalid"}, {"Sec-Fetch-Site": "same-origin"}, {"Host": "example.invalid"}} {
		status, _ := facadeRequest(t, f, "GET", "/v1/desktop/capabilities", "", headers)
		if status != 401 {
			t.Fatal("host boundary accepted")
		}
	}
	for _, body := range []string{`{}`, `{"native_selection_id":null}`, `{"Native_selection_id":"x"}`, `{"native_selection_id":"x","native_selection_id":"y"}`, `{"native_selection_id":"x","path":"/private"}`, `{"native_selection_id":"x"} {}`, strings.Repeat("x", 128*1024+1)} {
		status, _ := facadeRequest(t, f, "POST", "/v1/desktop/bindings/preview", body, nil)
		if status != 400 {
			t.Fatal("bad JSON accepted")
		}
	}
	for _, path := range []string{"/v1/desktop/capabilities?x=1", "/v1/desktop/capabilities?"} {
		status, _ := facadeRequest(t, f, "GET", path, "", nil)
		if status != 400 {
			t.Fatal("unknown query accepted")
		}
	}
	for _, test := range []string{"duplicate-auth", "non-loopback", "empty-origin"} {
		r := httptest.NewRequest("GET", f.address+"/v1/desktop/capabilities", nil)
		r.RemoteAddr = "127.0.0.1:34567"
		r.Header.Set("Authorization", "Bearer "+f.token)
		switch test {
		case "duplicate-auth":
			r.Header.Add("Authorization", "Bearer "+f.token)
		case "non-loopback":
			r.RemoteAddr = "10.0.0.1:34567"
		case "empty-origin":
			r.Header["Origin"] = []string{""}
		}
		w := httptest.NewRecorder()
		f.handle(w, r)
		if w.Code != 401 {
			t.Fatal("ambiguous authentication accepted")
		}
	}
	status, _ = facadeRequest(t, f, "GET", "/v1/desktop/sources", "", nil)
	if status != 422 {
		t.Fatal("unimplemented source route appeared available")
	}
}
func TestDesktopSessionFacadeTicketsAndDeniedConfirm(t *testing.T) {
	f, err := startFacade(context.Background(), testBinary(t))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	ticket := selected(t, f.preflight, processVault(t, "bad"))
	input, _ := json.Marshal(ticket)
	f.slot <- struct{}{}
	busy, _ := facadeRequest(t, f, "POST", "/v1/desktop/bindings/preview", string(input), nil)
	<-f.slot
	if busy != 409 {
		t.Fatal("probe saturation did not refuse")
	}
	f.preflight.mu.Lock()
	_, retained := f.preflight.selections[ticket.NativeSelectionID]
	f.preflight.mu.Unlock()
	if !retained {
		t.Fatal("busy probe consumed selection")
	}
	status, data := facadeRequest(t, f, "POST", "/v1/desktop/bindings/preview", string(input), nil)
	var response struct {
		Data desktopcontract.BindingPreview `json:"data"`
	}
	if status != 200 || json.Unmarshal(data, &response) != nil || response.Data.Validate() != nil || response.Data.Decision != "blocked" || response.Data.Reason != "owner_unavailable" {
		t.Fatal("failed owner preview not blocked")
	}
	input, _ = json.Marshal(confirmation(response.Data))
	status, _ = facadeRequest(t, f, "POST", "/v1/desktop/bindings", string(input), nil)
	if status != 400 {
		t.Fatal("missing original key accepted")
	}
	status, data = facadeRequest(t, f, "POST", "/v1/desktop/bindings", string(input), map[string]string{"Idempotency-Key": "original-key"})
	if status != 422 || !strings.Contains(string(data), "managed_scope_unverifiable") {
		t.Fatal("confirmation bypassed guard")
	}
	f.Close()
	select {
	case <-f.done:
	case <-time.After(time.Second):
		t.Fatal("facade listener survived close")
	}
}
