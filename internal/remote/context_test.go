package remote

import (
	"context"
	"github.com/yeisme/drivebridge-server/sdk"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestConcurrentTenantContextsDoNotMutateAdapter(t *testing.T) {
	seen := make(chan [2]string, 33)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen <- [2]string{r.Header.Get("X-DriveBridge-Tenant"), r.Header.Get("X-DriveBridge-Space")}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"ok":true}}`))
	}))
	defer server.Close()
	client, err := sdk.New(server.URL, "fixture-only")
	if err != nil {
		t.Fatal(err)
	}
	adapter := &Adapter{Client: client}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tenant := "alpha"
			if i%2 == 1 {
				tenant = "beta"
			}
			if _, e := adapter.Call(context.Background(), "capabilities", map[string]any{"tenant_context": tenant, "space_context": tenant + "-space"}); e != nil {
				t.Error(e)
			}
		}(i)
	}
	wg.Wait()
	if _, e := adapter.Call(context.Background(), "capabilities", nil); e != nil {
		t.Fatal(e)
	}
	close(seen)
	empty := 0
	for pair := range seen {
		if pair[0] == "" {
			empty++
			if pair[1] != "" {
				t.Fatal("unscoped caller inherited a space")
			}
		} else if pair[1] != pair[0]+"-space" {
			t.Fatal("concurrent scope mixed")
		}
	}
	if empty != 1 {
		t.Fatalf("unscoped requests %d", empty)
	}
}
