package bridge

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	rconfig "github.com/rclone/rclone/fs/config"
)

func TestBindPrefixContains(t *testing.T) {
	if !BindPrefixContains("projects/drama-1/", "projects/drama-1") {
		t.Fatal("prefix directory itself must be in range")
	}
	if !BindPrefixContains("projects/drama-1", "projects/drama-1/ref.png") {
		t.Fatal("descendant must be in range")
	}
	if BindPrefixContains("projects/drama-1/", "projects/drama-2/ref.png") {
		t.Fatal("sibling prefix must be denied")
	}
	if BindPrefixContains("projects/drama-1", "projects/drama-10/x") {
		t.Fatal("prefix must not match a longer sibling name")
	}
	if BindPrefixContains("projects/drama-1", "") {
		t.Fatal("space root is outside a non-empty prefix")
	}
	if !BindPrefixContains("", "anywhere/file") {
		t.Fatal("empty prefix is the whole space")
	}
}

func TestBindDiscoverDefaultKindsAndAdopted(t *testing.T) {
	s, local, _ := genericFixture(t)
	if err := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	rconfig.Data().SetValue("web", "type", "webdav")
	rconfig.Data().SetValue("web", "url", "http://127.0.0.1:9")
	rconfig.Data().SetValue("web", "vendor", "other")
	rconfig.Data().SetValue("web", "user", "u")
	if err := s.AddSpace(Space{ID: "web", Remote: "web", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	rows := s.DiscoverBindSpaces(false)
	if !discoverHas(rows, "nas") || discoverHas(rows, "web") {
		t.Fatalf("default discover=%+v want nas, not web", rows)
	}
	generic := s.DiscoverBindSpaces(true)
	if !discoverHas(generic, "web") {
		t.Fatalf("allow-generic missing web: %+v", generic)
	}

	vault := filepath.Join(t.TempDir(), "vault")
	if err := os.MkdirAll(vault, 0700); err != nil {
		t.Fatal(err)
	}
	if err := EnsureAliasRemote("db-local-vault", vault); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.AdoptStorage(AdoptRequest{
		Consumer: "pinax", Kind: "local", Space: "pinax-vault",
		Root: vault, Remote: "db-local-vault", LocalRoot: vault,
		IdempotencyKey: "k-local", MaxBytes: 1 << 20,
	}); err != nil {
		t.Fatal(err)
	}
	rows = s.DiscoverBindSpaces(false)
	found := false
	for _, row := range rows {
		if row.Space == "pinax-vault" {
			found = true
			if !row.Adopted || row.Kind != "local" {
				t.Fatalf("adopted row: %+v", row)
			}
		}
		if row.Space == "web" {
			t.Fatalf("escape leaked into default discover: %+v", rows)
		}
	}
	if !found {
		t.Fatalf("adopted space missing: %+v", rows)
	}
}

func discoverHas(rows []BindDiscoverSpace, id string) bool {
	for _, row := range rows {
		if row.Space == id {
			return true
		}
	}
	return false
}

func TestBindAddListRevokeAndEscape(t *testing.T) {
	s, local, _ := genericFixture(t)
	if err := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	rconfig.Data().SetValue("web", "type", "webdav")
	rconfig.Data().SetValue("web", "url", "http://127.0.0.1:9")
	if err := s.AddSpace(Space{ID: "web", Remote: "web", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	if _, _, err := s.AddBind(BindRequest{Consumer: "pinax", ProjectRef: "drama-1", Space: "web", Prefix: "x", Purpose: "materials"}); err == nil || err.Error() != "bind_escape_denied" {
		t.Fatalf("escape bind: %v", err)
	}
	if _, _, err := s.AddBind(BindRequest{Consumer: "pinax", ProjectRef: "drama-1", Space: "nas", Prefix: "projects/drama-1", Purpose: "photos"}); err == nil || err.Error() != "bind_purpose_unknown" {
		t.Fatalf("purpose: %v", err)
	}
	rec, reused, err := s.AddBind(BindRequest{
		Consumer: "pinax", ProjectRef: "drama-1", Space: "nas",
		Prefix: "projects/drama-1/", Purpose: "materials", IdempotencyKey: "k-bind",
	})
	if err != nil || reused || rec.Prefix != "projects/drama-1" {
		t.Fatalf("add: %+v reused=%v err=%v", rec, reused, err)
	}
	again, reused, err := s.AddBind(BindRequest{
		Consumer: "pinax", ProjectRef: "drama-1", Space: "nas",
		Prefix: "projects/drama-1", Purpose: "materials", IdempotencyKey: "k-bind",
	})
	if err != nil || !reused || again.ID != rec.ID {
		t.Fatalf("reuse: %+v reused=%v err=%v", again, reused, err)
	}
	if _, _, err = s.AddBind(BindRequest{
		Consumer: "pinax", ProjectRef: "drama-1", Space: "nas",
		Prefix: "projects/drama-2", Purpose: "materials", IdempotencyKey: "k-bind",
	}); err == nil || err.Error() != "bind_mismatch" {
		t.Fatalf("mismatch: %v", err)
	}
	if len(s.Binds()) != 1 {
		t.Fatalf("list: %+v", s.Binds())
	}
	shown, err := s.Bind(rec.ID)
	if err != nil || shown.Consumer != "pinax" {
		t.Fatalf("show: %+v %v", shown, err)
	}
	if _, err = s.RevokeBind(rec.ID); err != nil {
		t.Fatal(err)
	}
	revoked, _ := s.Bind(rec.ID)
	if revoked.RevokedAt == nil {
		t.Fatal("revoke must keep the record and set revoked_at")
	}
}

func TestBindPrefixDeniedAndRevokeStopsReads(t *testing.T) {
	s, local, remoteRoot := genericFixture(t)
	if err := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"projects/drama-1", "projects/drama-2"} {
		if err := os.MkdirAll(filepath.Join(remoteRoot, dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(remoteRoot, "projects/drama-1/ref.png"), []byte("in"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(remoteRoot, "projects/drama-2/other.png"), []byte("out"), 0600); err != nil {
		t.Fatal(err)
	}
	rec, _, err := s.AddBind(BindRequest{
		Consumer: "pinax", ProjectRef: "drama-1", Space: "nas",
		Prefix: "projects/drama-1", Purpose: "materials", IdempotencyKey: "k-fence",
	})
	if err != nil {
		t.Fatal(err)
	}
	scope := WithBindScope(context.Background(), BindScope{Consumer: "pinax", ProjectRef: "drama-1"})
	if _, err = s.List(context.Background(), "nas", "projects/drama-2"); err != nil {
		t.Fatalf("unbound list must stay compatible: %v", err)
	}
	if _, err = s.List(scope, "nas", "projects/drama-2"); err == nil || err.Error() != "bind_prefix_denied" {
		t.Fatalf("outside list: %v", err)
	}
	files, err := s.List(scope, "nas", "projects/drama-1")
	if err != nil || len(files) != 1 || files[0].Name != "ref.png" {
		t.Fatalf("inside list: %v %+v", err, files)
	}
	if err = s.Mkdir(scope, "nas", "projects/drama-2/nope"); err == nil || err.Error() != "bind_prefix_denied" {
		t.Fatalf("outside mkdir: %v", err)
	}
	src := filepath.Join(local, "up.bin")
	if err = os.WriteFile(src, []byte("payload-bytes"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = s.Upload(scope, "nas", "projects/drama-2", src, "up.bin", "up-out"); err == nil || err.Error() != "bind_prefix_denied" {
		t.Fatalf("outside upload: %v", err)
	}
	outside, err := s.List(context.Background(), "nas", "projects/drama-2")
	if err != nil || len(outside) != 1 {
		t.Fatalf("index outside: %v %+v", err, outside)
	}
	if _, err = s.Stat(scope, outside[0].Ref); err == nil || err.Error() != "bind_prefix_denied" {
		t.Fatalf("outside stat: %v", err)
	}
	if _, err = s.Trash(scope, outside[0].Ref); err == nil || err.Error() != "bind_prefix_denied" {
		t.Fatalf("outside trash: %v", err)
	}
	if _, err = s.RevokeBind(rec.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = s.List(scope, "nas", "projects/drama-1"); err == nil || err.Error() != "bind_revoked" {
		t.Fatalf("revoked list: %v", err)
	}
	if _, err = s.List(context.Background(), "nas", "projects/drama-1"); err != nil {
		t.Fatalf("unbound list after revoke: %v", err)
	}
}

func TestBindMCPDiscoverList(t *testing.T) {
	s, local, _ := genericFixture(t)
	if err := s.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	caps, _ := json.Marshal(s.Capabilities())
	if !strings.Contains(string(caps), `"bind_discover"`) || !strings.Contains(string(caps), `"bind_list"`) {
		t.Fatalf("capabilities missing bind tools: %s", caps)
	}
	out, err := s.Call(context.Background(), "bind_discover", nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(out)
	if !strings.Contains(string(raw), `"space":"nas"`) {
		t.Fatalf("discover: %s", raw)
	}
	if _, _, err = s.AddBind(BindRequest{Consumer: "pinax", ProjectRef: "p", Space: "nas", Prefix: "notes", Purpose: "vault"}); err != nil {
		t.Fatal(err)
	}
	listed, err := s.Call(context.Background(), "bind_list", nil)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = json.Marshal(listed)
	if !strings.Contains(string(raw), `"consumer":"pinax"`) || strings.Contains(string(raw), "token") {
		t.Fatalf("bind_list: %s", raw)
	}
	names := map[string]bool{}
	for _, tool := range s.Tools() {
		names[tool.Name] = true
	}
	if !names["bind_discover"] || !names["bind_list"] {
		t.Fatalf("tools: %v", names)
	}
}
