package bridge

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
)

func TestRelayUsesOnlyOriginalInputAndTransientGrant(t *testing.T) {
	s, provider, local, state := setup(t)
	upload := uploadFixture(t, s, local)
	var complete atomic.Bool
	var loseCompletion atomic.Bool
	loseCompletion.Store(true)
	var puts atomic.Int64
	var digest string
	var owner *httptest.Server
	owner = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		respond := func(v any) { json.NewEncoder(w).Encode(map[string]any{"data": v}) }
		if strings.HasPrefix(r.URL.Path, "/input-requests/") {
			if r.Header.Get("Authorization") != "" || r.Header.Get("X-Input-Grant") != "transient-only" {
				t.Error("incorrect transfer authority")
			}
			if strings.HasSuffix(r.URL.Path, "/content") {
				puts.Add(1)
				data, _ := io.ReadAll(r.Body)
				sum, _, _ := digestReader(strings.NewReader(string(data)))
				if sum != digest {
					t.Error("source changed")
				}
			}
			if strings.HasSuffix(r.URL.Path, "/complete") {
				complete.Store(true)
				if loseCompletion.Swap(false) {
					w.WriteHeader(503)
					return
				}
			}
			w.WriteHeader(204)
			return
		}
		if r.Header.Get("Authorization") != "Bearer fixture-relay-key" {
			t.Error("missing owner identity")
		}
		switch {
		case r.URL.Path == "/api/v1/drivebridge/imports":
			var q struct {
				SHA256 string `json:"sha256"`
			}
			json.NewDecoder(r.Body).Decode(&q)
			digest = q.SHA256
			respond(map[string]any{"import_id": "imp_original", "input_request_id": "inp_original", "state": "awaiting_transfer"})
		case strings.HasSuffix(r.URL.Path, "/transfer"):
			respond(map[string]any{"request": map[string]string{"input_request_id": "inp_original"}, "transfer_url": owner.URL + "/input-requests/inp_original#grant=transient-only"})
		case r.Method == "GET":
			st := "awaiting_transfer"
			if complete.Load() {
				st = "ready"
			}
			respond(map[string]any{"import_id": "imp_original", "input_request_id": "inp_original", "state": st, "asset_uri": "eikona://asset/fixed", "sha256": digest, "domain_state": "unreviewed"})
		default:
			t.Error("unexpected remote operation", r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	defer owner.Close()
	key := filepath.Join(t.TempDir(), "key")
	if err := os.WriteFile(key, []byte("fixture-relay-key"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.ConfigureRelay(RelayTarget{ID: "eikona", SpaceID: "creative", Project: "p", ConnectionID: "local-source", Origin: owner.URL, KeyFile: key}); err != nil {
		t.Fatal(err)
	}
	op, err := s.StartRelay(context.Background(), upload.FileRef, "eikona", "relay-one")
	if err != nil {
		t.Fatal(err)
	}
	op = waitOp(t, s, op.ID)
	if op.Status != "failed" || op.ImportID != "imp_original" {
		t.Fatalf("lost completion not recoverable: %+v", op)
	}
	s.Close()
	reopened, err := New(state, provider, true)
	if err != nil {
		t.Fatal(err)
	}
	s = reopened
	t.Cleanup(s.Close)
	if _, err := s.Resume(op.ID); err != nil {
		t.Fatal(err)
	}
	op = waitOp(t, s, op.ID)

	if op.Status != "completed" || op.ResultRef != "eikona://asset/fixed" {
		t.Fatalf("relay: %+v", op)
	}
	again, err := s.StartRelay(context.Background(), upload.FileRef, "eikona", "relay-one")
	if err != nil || again.ID != op.ID || puts.Load() != 1 {
		t.Fatal("duplicate transfer", err)
	}
	raw, _ := json.Marshal(op)
	if strings.Contains(string(raw), "transient-only") || strings.Contains(string(raw), key) {
		t.Fatal("capability leaked")
	}
	contents, err := os.ReadFile(filepath.Join(state, "state.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents), "transient-only") {
		t.Fatal("grant persisted")
	}
}

func TestRelayCancellationDoesNotCancelOtherOperations(t *testing.T) {
	s, _, _, _ := setup(t)
	op := Operation{ID: "ordinary", Kind: "upload", Status: "failed", Key: "ordinary"}
	if err := s.DB.Create(&op).Error; err != nil {
		t.Fatal(err)
	}
	if _, err := s.CancelRelay(context.Background(), op.ID); err == nil {
		t.Fatal("non-relay accepted")
	}
	got, err := s.Get(op.ID)
	if err != nil || got.Status != "failed" {
		t.Fatalf("other operation changed: %+v %v", got, err)
	}
}

func TestRelayDiscoveryIsSafeAndNonMutating(t *testing.T) {
	s, _, _, _ := setup(t)
	target := RelayTarget{ID: "known", SpaceID: "creative", Project: "p", ConnectionID: "eikona", Origin: "https://private.example", KeyFile: "/private/client.key", Disabled: true}
	if err := s.DB.Create(&target).Error; err != nil {
		t.Fatal(err)
	}
	got, err := s.Call(context.Background(), "relay_targets", nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(got)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, "known") || !strings.Contains(text, "not_probed") || !strings.Contains(text, "disabled") || strings.Contains(text, "private.example") || strings.Contains(text, "client.key") {
		t.Fatal(text)
	}
	stored, _ := s.RelayTargets()
	if len(stored) != 1 || !stored[0].Disabled {
		t.Fatal("discovery changed binding")
	}
}
