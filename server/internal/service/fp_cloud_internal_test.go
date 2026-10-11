package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"golang.org/x/oauth2"
)

// gdriveStub serves a minimal Google Drive v3 shape over loopback: one
// folder with two files (one binary, one native document), a changes feed
// and revision-pinned media.
type gdriveStub struct {
	mu        chan struct{}
	pageSize  int32
	revisions map[string]string
}

func (s *gdriveStub) handler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer fixture-gdrive" {
			w.WriteHeader(401)
			return
		}
		switch {
		case strings.Contains(r.URL.Path, "/revisions/") && r.URL.Query().Get("alt") == "media":
			// Per-revision media: distinct bytes per revision id so tests
			// can prove a pinned historical fetch never serves head. This
			// arm must precede the generic file-media arm.
			parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/drive/v3/files/"), "/revisions/")
			io.WriteString(w, "revision-bytes-"+parts[0]+"@"+parts[1])
		case strings.HasPrefix(r.URL.Path, "/drive/v3/files") && r.URL.Query().Get("alt") == "media":
			id := strings.TrimPrefix(r.URL.Path, "/drive/v3/files/")
			if id == "native-1" {
				w.WriteHeader(403)
				return
			}
			io.WriteString(w, "binary-bytes-"+id)
		case strings.HasPrefix(r.URL.Path, "/drive/v3/files/") && strings.Contains(r.URL.Path, "/revisions/"):
			parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/drive/v3/files/"), "/revisions/")
			if rev, ok := s.revisions[parts[0]+"&"+parts[1]]; ok && rev != "" {
				io.WriteString(w, `{"id":"`+parts[1]+`"}`)
				return
			}
			w.WriteHeader(404)
		case strings.HasPrefix(r.URL.Path, "/drive/v3/files/"):
			id := strings.TrimPrefix(r.URL.Path, "/drive/v3/files/")
			switch id {
			case "binary-1":
				json.NewEncoder(w).Encode(map[string]any{"id": id, "name": "report.bin", "mimeType": "application/octet-stream", "size": "13", "headRevision": "rev-1", "parents": []string{"root-1"}, "modifiedTime": "2026-09-25T00:00:00Z"})
			case "native-1":
				json.NewEncoder(w).Encode(map[string]any{"id": id, "name": "Sheet", "mimeType": "application/vnd.google-apps.spreadsheet", "size": "0", "headRevision": "rev-9", "parents": []string{"root-1"}, "modifiedTime": "2026-09-25T00:00:00Z"})
			default:
				w.WriteHeader(404)
			}
		case r.URL.Path == "/drive/v3/files":
			q := r.URL.Query().Get("q")
			if !strings.Contains(q, "'root-1' in parents") {
				json.NewEncoder(w).Encode(map[string]any{"files": []any{}})
				return
			}
			files := []any{
				map[string]any{"id": "binary-1", "name": "report.bin", "mimeType": "application/octet-stream", "size": "13", "headRevision": "rev-1", "parents": []string{"root-1"}, "modifiedTime": "2026-09-25T00:00:00Z"},
				map[string]any{"id": "native-1", "name": "Sheet", "mimeType": "application/vnd.google-apps.spreadsheet", "size": "0", "headRevision": "rev-9", "parents": []string{"root-1"}, "modifiedTime": "2026-09-25T00:00:00Z"},
			}
			out := map[string]any{"files": files}
			if atomic.LoadInt32(&s.pageSize) > 0 {
				// One item per page so pagination is observable.
				if r.URL.Query().Get("pageToken") == "" {
					out["files"] = files[:1]
					out["nextPageToken"] = "page-2"
				} else {
					out["files"] = files[1:]
				}
			}
			json.NewEncoder(w).Encode(out)
		case r.URL.Path == "/drive/v3/changes":
			token := r.URL.Query().Get("pageToken")
			if token == "" || token == "1" {
				json.NewEncoder(w).Encode(map[string]any{"nextPageToken": "page-2", "changes": []any{
					map[string]any{"fileId": "binary-1", "file": map[string]any{"id": "binary-1", "name": "report.bin", "mimeType": "application/octet-stream", "size": "13", "headRevision": "rev-2", "parents": []string{"root-1"}, "modifiedTime": "2026-09-25T00:00:00Z"}},
				}})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"newStartPageToken": "page-final", "changes": []any{
				map[string]any{"fileId": "native-1", "removed": true},
			}})
		default:
			w.WriteHeader(404)
		}
	})
}

// Task 5.1 spec scenario 1: a Google Drive connection lists the allowed
// root from the server side — the local machine being offline never blocks
// cloud reads — with bounded pages; native documents list as
// native_document and refuse downloads with export_required; historical
// revisions resolve through the revisions endpoint, and the changes feed
// tombstones only provider-confirmed removals.
func TestFPCloudConnectorsGDriveAdapterScenarios(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, e := Open(Config{State: dir, Enabled: true, Development: true, MaxBytes: 1 << 20, CloudConnectors: true})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	stub := &gdriveStub{revisions: map[string]string{"binary-1&rev-1": "ok"}}
	server := httptest.NewServer(stub.handler())
	defer server.Close()
	// The endpoint override is honored in development mode only; the stub
	// client is installed so requests land on the loopback server.
	s.gdriveHTTP = server.Client()
	secret, _ := s.seal("owner-1", gdriveCredential{Token: oauth2.Token{AccessToken: "fixture-gdrive", TokenType: "Bearer"}, Endpoint: server.URL})
	conn := Connection{ID: ID(), Owner: "owner-1", Kind: "gdrive", Name: "gd", Secret: secret, Status: "verified"}
	if s.DB.Create(&conn).Error != nil {
		t.Fatal("connection create failed")
	}
	p := Principal{UserID: "owner-1"}
	root := CloudRoot{ID: ID(), Owner: "owner-1", ConnectionID: conn.ID, Provider: "gdrive", RootRef: "root-1", Status: "bound", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	s.DB.Create(&root)
	adapter, _ := s.cloudAdapterFor("gdrive")
	// Paginated listing: page tokens continue the enumeration.
	atomic.StoreInt32(&stub.pageSize, 1)
	page, e := adapter.ListChildren(context.Background(), p, conn, root, "", "", 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(page.Items) != 1 || page.NextToken == "" || page.Complete {
		t.Fatalf("first page wrong: %+v", page)
	}
	second, e := adapter.ListChildren(context.Background(), p, conn, root, "", page.NextToken, 1)
	if e != nil {
		t.Fatal(e)
	}
	if len(second.Items) != 1 || !second.Complete {
		t.Fatalf("second page wrong: %+v", second)
	}
	kinds := map[string]string{}
	for _, pg := range []cloudPage{page, second} {
		for _, it := range pg.Items {
			kinds[it.ProviderItemID] = it.Kind
		}
	}
	if kinds["binary-1"] != "file" || kinds["native-1"] != "native_document" {
		t.Fatalf("kinds wrong: %+v", kinds)
	}
	// Native documents refuse downloads with export_required.
	if _, e := adapter.FetchItem(context.Background(), p, conn, root, "native-1", "rev-9"); e == nil || e.Error() != "export_required" {
		t.Fatalf("native doc must be export_required, got %v", e)
	}
	// Binary fetch serves the pinned revision bytes.
	fetch, e := adapter.FetchItem(context.Background(), p, conn, root, "binary-1", "rev-1")
	if e != nil {
		t.Fatal(e)
	}
	body, _ := io.ReadAll(fetch.Body)
	fetch.Body.Close()
	if string(body) != "binary-bytes-binary-1" {
		t.Fatalf("fetch bytes wrong: %q", body)
	}
	// The changes feed: first page carries an update, the final page a
	// provider-confirmed removal; only removals may tombstone.
	changes, e := adapter.ReadChanges(context.Background(), p, conn, root, "", 10)
	if e != nil {
		t.Fatal(e)
	}
	if len(changes.Items) != 1 || changes.Items[0].Revision != "rev-2" || !changes.Delta || changes.NextCursor == "" {
		t.Fatalf("first change page wrong: %+v", changes)
	}
	final, e := adapter.ReadChanges(context.Background(), p, conn, root, changes.NextCursor, 10)
	if e != nil {
		t.Fatal(e)
	}
	if len(final.Removed) != 1 || final.Removed[0] != "native-1" || final.Delta {
		t.Fatalf("final change page wrong: %+v", final)
	}
}

// Task 5.1 checkpoint item: the OneDrive adapter path through the Graph
// stub — children listing with skip tokens, stat, the item-versions
// endpoint for historical bytes and the delta feed with deleted facets.
func TestFPCloudConnectorsOneDriveAdapterPath(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, e := Open(Config{State: dir, Enabled: true, Development: true, MaxBytes: 1 << 20, CloudConnectors: true})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	secret, _ := s.seal("owner-1", oneDriveCredential{Token: oauth2.Token{AccessToken: "fixture-private", TokenType: "Bearer"}, DriveID: "drv-1"})
	conn := Connection{ID: ID(), Owner: "owner-1", Kind: "onedrive", Name: "od", Secret: secret, Status: "verified"}
	if s.DB.Create(&conn).Error != nil {
		t.Fatal("connection create failed")
	}
	s.graphHTTP = &http.Client{Transport: graphRoundTrip(func(r *http.Request) (*http.Response, error) {
		if r.Header.Get("Authorization") != "Bearer fixture-private" || r.URL.Host != "graph.microsoft.com" {
			return nil, code("forbidden")
		}
		var out []byte
		switch {
		case strings.Contains(r.URL.Path, "/versions/") && strings.HasSuffix(r.URL.Path, "/content"):
			if strings.Contains(r.URL.Path, "rev-old") {
				out = []byte("historical-bytes")
			} else {
				return graphResponse(404, nil)
			}
		case strings.Contains(r.URL.Path, "/delta"):
			out, _ = json.Marshal(map[string]any{"value": []any{
				map[string]any{"id": "item-2", "name": "gone.txt", "size": 4, "eTag": "e2", "file": map[string]string{"mimeType": "text/plain"}, "deleted": map[string]any{}},
			}, "@odata.deltaLink": "https://graph.microsoft.com/v1.0/delta?$skiptoken=final"})
		case strings.Contains(r.URL.Path, "/children"):
			if r.URL.Query().Get("$skiptoken") == "" {
				out, _ = json.Marshal(map[string]any{"value": []any{
					map[string]any{"id": "item-1", "name": "one.txt", "size": 3, "eTag": "rev-old", "file": map[string]string{"mimeType": "text/plain"}, "parentReference": map[string]string{"id": "root"}},
				}, "@odata.nextLink": "https://graph.microsoft.com/v1.0/children?$skiptoken=2"})
			} else {
				out, _ = json.Marshal(map[string]any{"value": []any{
					map[string]any{"id": "item-2", "name": "gone.txt", "size": 4, "eTag": "e2", "file": map[string]string{"mimeType": "text/plain"}},
				}})
			}
		case strings.Contains(r.URL.Path, "/items/item-1"):
			out, _ = json.Marshal(map[string]any{"id": "item-1", "name": "one.txt", "size": 3, "eTag": "rev-new", "file": map[string]string{"mimeType": "text/plain"}})
		default:
			return graphResponse(404, nil)
		}
		return graphResponse(200, out)
	})}
	p := Principal{UserID: "owner-1"}
	root := CloudRoot{ID: ID(), Owner: "owner-1", ConnectionID: conn.ID, Provider: "onedrive", RootRef: "root", Status: "bound", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	s.DB.Create(&root)
	adapter, _ := s.cloudAdapterFor("onedrive")
	page, e := adapter.ListChildren(context.Background(), p, conn, root, "", "", 10)
	if e != nil {
		t.Fatal(e)
	}
	if len(page.Items) != 1 || page.Items[0].Revision != "rev-old" || page.Complete {
		t.Fatalf("first onedrive page wrong: %+v", page)
	}
	second, e := adapter.ListChildren(context.Background(), p, conn, root, "", page.NextToken, 10)
	if e != nil || len(second.Items) != 1 || !second.Complete {
		t.Fatalf("second onedrive page wrong: %+v %v", second, e)
	}
	// Historical bytes resolve through the item versions endpoint.
	fetch, e := adapter.FetchItem(context.Background(), p, conn, root, "item-1", "rev-old")
	if e != nil {
		t.Fatal(e)
	}
	body, _ := io.ReadAll(fetch.Body)
	fetch.Body.Close()
	if string(body) != "historical-bytes" {
		t.Fatalf("historical bytes wrong: %q", body)
	}
	// The delta feed reports the provider-confirmed removal only.
	changes, e := adapter.ReadChanges(context.Background(), p, conn, root, "", 10)
	if e != nil {
		t.Fatal(e)
	}
	if len(changes.Removed) != 1 || changes.Removed[0] != "item-2" || changes.Delta || changes.NextCursor != "final" {
		t.Fatalf("onedrive delta wrong: %+v", changes)
	}
}

func graphResponse(status int, body []byte) (*http.Response, error) {
	return &http.Response{StatusCode: status, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytesReader(body))}, nil
}

func bytesReader(b []byte) io.Reader { return strings.NewReader(string(b)) }

// Rework M3: a pinned historical revision streams from the revision media
// endpoint — the returned bytes are the revision's own, never the head's.
func TestFPCloudConnectorsGDriveHistoricalBytesArePinned(t *testing.T) {
	dir := t.TempDir()
	os.Chmod(dir, 0700)
	s, e := Open(Config{State: dir, Enabled: true, Development: true, MaxBytes: 1 << 20, CloudConnectors: true})
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	stub := &gdriveStub{revisions: map[string]string{"binary-1&rev-1": "ok"}}
	server := httptest.NewServer(stub.handler())
	defer server.Close()
	secret, _ := s.seal("owner-1", gdriveCredential{Token: oauth2.Token{AccessToken: "fixture-gdrive", TokenType: "Bearer"}, Endpoint: server.URL})
	conn := Connection{ID: ID(), Owner: "owner-1", Kind: "gdrive", Name: "gd", Secret: secret, Status: "verified"}
	s.DB.Create(&conn)
	p := Principal{UserID: "owner-1"}
	root := CloudRoot{ID: ID(), Owner: "owner-1", ConnectionID: conn.ID, Provider: "gdrive", RootRef: "root-1", Status: "bound", CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
	s.DB.Create(&root)
	adapter, _ := s.cloudAdapterFor("gdrive")
	// The head sits on rev-1; a pinned older revision must return its own
	// revision bytes, not the head's file media.
	fetch, e := adapter.FetchItem(context.Background(), p, conn, root, "binary-1", "rev-older")
	if e != nil {
		t.Fatal(e)
	}
	body, _ := io.ReadAll(fetch.Body)
	fetch.Body.Close()
	if string(body) != "revision-bytes-binary-1@rev-older" {
		t.Fatalf("historical fetch served non-pinned bytes: %q", body)
	}
	// The current head still streams from the file media endpoint.
	head, e := adapter.FetchItem(context.Background(), p, conn, root, "binary-1", "rev-1")
	if e != nil {
		t.Fatal(e)
	}
	headBody, _ := io.ReadAll(head.Body)
	head.Body.Close()
	if string(headBody) != "binary-bytes-binary-1" {
		t.Fatalf("head fetch wrong: %q", headBody)
	}
}
