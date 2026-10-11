package sdk

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestAigoraKeyOnlySentToGatewayAndScopedCacheShared(t *testing.T) {
	const key = "fixture-aigora-key"
	const proof = "header.payload.signature"
	var exchanges atomic.Int64
	var leaks atomic.Int64
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+key {
			w.WriteHeader(401)
			return
		}
		exchanges.Add(1)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"principal_context": proof, "expires_at": time.Now().Add(5 * time.Minute).Unix()}})
	}))
	defer gateway.Close()
	owner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+proof {
			leaks.Add(1)
			w.WriteHeader(401)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]bool{"ok": true}})
	}))
	defer owner.Close()
	client, err := NewAigora(owner.URL, gateway.URL, key)
	if err != nil {
		t.Fatal(err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for range 8 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var result any
			errs <- client.Scoped("tenant", "space").JSON(context.Background(), "GET", "/api/v1/me", "", nil, &result)
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	if leaks.Load() != 0 || exchanges.Load() != 1 {
		t.Fatalf("leaks=%d exchanges=%d", leaks.Load(), exchanges.Load())
	}
}

func TestAigoraFailureDoesNotSendLongKeyToOwner(t *testing.T) {
	var calls atomic.Int64
	owner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { calls.Add(1) }))
	defer owner.Close()
	gateway := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.WriteHeader(503) }))
	defer gateway.Close()
	client, err := NewAigora(owner.URL, gateway.URL, "fixture-key")
	if err != nil {
		t.Fatal(err)
	}
	var out any
	if err := client.JSON(context.Background(), "GET", "/api/v1/me", "", nil, &out); err == nil {
		t.Fatal("gateway failure ignored")
	}
	if calls.Load() != 0 {
		t.Fatal("owner contacted without a service context")
	}
}
