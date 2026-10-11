package fixtures

import (
	"net/http"
	"sync/atomic"
	"testing"
)

func TestRecoveryReadinessBlocksRequests(t *testing.T) {
	var ready atomic.Bool
	f, e := StartWithOptions(t.TempDir(), "", "127.0.0.1:0", Options{Demo: true, Ready: &ready})
	if e != nil {
		t.Fatal(e)
	}
	defer f.Close()
	r, e := http.Get(f.App.URL + "/healthz")
	if e != nil {
		t.Fatal(e)
	}
	r.Body.Close()
	if r.StatusCode != 503 {
		t.Fatal(r.StatusCode)
	}
	ready.Store(true)
	r, e = http.Get(f.App.URL + "/healthz")
	if e != nil {
		t.Fatal(e)
	}
	r.Body.Close()
	if r.StatusCode != 200 {
		t.Fatal(r.StatusCode)
	}
}
