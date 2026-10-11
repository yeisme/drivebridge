package service_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

type prober interface {
	ProbeIsolation(context.Context) (bool, error)
}

func newProbeStore(t *testing.T, handler http.Handler) service.Storage {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	st, e := service.NewS3(service.S3Config{Endpoint: srv.URL, Region: "us-east-1", Bucket: "fixture", Prefix: "tenants", AccessKey: "k", SecretKey: "s"}, true)
	if e != nil {
		t.Fatalf("NewS3: %v", e)
	}
	return st
}

func TestProbeIsolationBothDenied(t *testing.T) {
	var puts, deletes int
	st := newProbeStore(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			puts++
			w.WriteHeader(http.StatusForbidden)
		case http.MethodDelete:
			deletes++
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	p := st.(prober)
	isolated, e := p.ProbeIsolation(context.Background())
	if e != nil || !isolated {
		t.Fatalf("expected isolated, got %v err %v", isolated, e)
	}
	if puts != 2 || deletes != 0 {
		t.Fatalf("expected 2 denied puts and 0 deletes, got %d/%d", puts, deletes)
	}
}

func TestProbeIsolationOverScopedCleansUp(t *testing.T) {
	var deleted []string
	var mu sync.Mutex
	st := newProbeStore(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			mu.Lock()
			deleted = append(deleted, r.URL.Path)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	p := st.(prober)
	isolated, e := p.ProbeIsolation(context.Background())
	if e != nil || isolated {
		t.Fatalf("expected over-scoped, got isolated=%v err %v", isolated, e)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(deleted) != 1 || !strings.Contains(deleted[0], "/_isolation-probe/") {
		t.Fatalf("expected cleanup of the root probe object only, got %v", deleted)
	}
}

func TestProbeIsolationSiblingScopeDetected(t *testing.T) {
	// Root probe denied but a sibling-shaped path under the prefix is
	// writable: the credential covers more than this deployment.
	var deleted []string
	var mu sync.Mutex
	st := newProbeStore(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodPut:
			if strings.HasPrefix(r.URL.Path, "/fixture/_isolation-probe/") {
				w.WriteHeader(http.StatusForbidden)
				return
			}
			w.WriteHeader(http.StatusOK)
		case http.MethodDelete:
			mu.Lock()
			deleted = append(deleted, r.URL.Path)
			mu.Unlock()
			w.WriteHeader(http.StatusNoContent)
		}
	}))
	p := st.(prober)
	isolated, e := p.ProbeIsolation(context.Background())
	if e != nil || isolated {
		t.Fatalf("expected over-scoped, got isolated=%v err %v", isolated, e)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(deleted) != 1 || !strings.HasPrefix(deleted[0], "/fixture/tenants/") {
		t.Fatalf("expected cleanup of the sibling probe object under the prefix, got %v", deleted)
	}
}

func TestProbeIsolationServerErrorPropagates(t *testing.T) {
	st := newProbeStore(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	p := st.(prober)
	if _, e := p.ProbeIsolation(context.Background()); e == nil {
		t.Fatal("expected transport-level error to propagate, not be counted as denial")
	}
}

func TestS3ConfigVirtualHostedZeroMigration(t *testing.T) {
	var legacy service.S3Config
	if e := json.Unmarshal([]byte(`{"endpoint":"https://cos.ap-guangzhou.myqcloud.com","bucket":"b","access_key":"k","secret_key":"s"}`), &legacy); e != nil || legacy.VirtualHosted {
		t.Fatalf("legacy sealed config must decode to path-style, got %+v err %v", legacy, e)
	}
	var hosted service.S3Config
	if e := json.Unmarshal([]byte(`{"endpoint":"https://cos.ap-guangzhou.myqcloud.com","bucket":"b","access_key":"k","secret_key":"s","virtual_hosted":true}`), &hosted); e != nil || !hosted.VirtualHosted {
		t.Fatalf("virtual_hosted must round-trip, got %+v err %v", hosted, e)
	}
}

// A shared bucket relies on every deployment writing only under its own
// instance namespace; this locks the objectKey shape end to end.
func TestSharedBucketInstanceNamespace(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	c, _ := vaultAgent(t, h, h.space.ID)
	op, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "shared/ns.txt"}, "ns", strings.NewReader("namespace"), nil)
	if e != nil || op.Status != "completed" {
		t.Fatalf("upload: %v status=%s", e, op.Status)
	}
	v := versionRow(t, f, op.VersionID)
	if !strings.HasPrefix(v.ObjectKey, f.Service.Instance+"/") {
		t.Fatalf("object key %q must start with the deployment instance namespace", v.ObjectKey)
	}
	storedObject(t, f, v.ObjectKey)
}
