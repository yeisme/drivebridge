package bridge

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// fakeOwnerInput stands in for one creative owner's original input contract:
// owner-scoped imports, transient transfer grants and owner-native completion
// receipts. It never shares a path with the local adapter.
type fakeOwnerInput struct {
	name         string
	server       *httptest.Server
	imports      atomic.Int64
	puts         atomic.Int64
	completes    atomic.Int64
	expired      atomic.Bool
	failPut      atomic.Bool
	loseComplete atomic.Bool
	digest       atomic.Value
}

func newFakeOwnerInput(t *testing.T, owner string) *fakeOwnerInput {
	t.Helper()
	name, profile, ok := relayOwner(owner)
	if !ok {
		t.Fatalf("unknown owner fixture: %s", owner)
	}
	grant := "grant-" + name + "-transient"
	o := &fakeOwnerInput{name: name}
	o.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/input-requests/") {
			// Transfer endpoints accept the transient grant and nothing else.
			if r.Header.Get("Authorization") != "" || r.Header.Get("X-Input-Grant") != grant {
				w.WriteHeader(403)
				return
			}
			switch {
			case strings.HasSuffix(r.URL.Path, "/content"):
				o.puts.Add(1)
				data, _ := io.ReadAll(r.Body)
				sum, _, _ := digestReader(bytes.NewReader(data))
				o.digest.Store(sum)
				if o.failPut.Swap(false) {
					w.WriteHeader(503)
					return
				}
				w.WriteHeader(204)
			case strings.HasSuffix(r.URL.Path, "/complete"):
				o.completes.Add(1)
				if o.loseComplete.Swap(false) {
					w.WriteHeader(503)
					return
				}
				w.WriteHeader(204)
			default:
				w.WriteHeader(404)
			}
			return
		}
		if r.Header.Get("Authorization") != "Bearer fixture-relay-key" {
			w.WriteHeader(401)
			return
		}
		respond := func(v any) {
			w.Header().Set("Content-Type", "application/json")
			_ = json.NewEncoder(w).Encode(map[string]any{"data": v})
		}
		switch {
		case r.URL.Path == "/api/v1/drivebridge/imports" && r.Method == "POST":
			o.imports.Add(1)
			var q struct {
				SHA256 string `json:"sha256"`
			}
			_ = json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&q)
			o.digest.Store(q.SHA256)
			respond(map[string]any{"import_id": "imp_" + name, "input_request_id": "inp_" + name, "state": "awaiting_transfer"})
		case strings.HasSuffix(r.URL.Path, "/transfer") && r.Method == "POST":
			respond(map[string]any{"request": map[string]string{"input_request_id": "inp_" + name}, "transfer_url": "http://" + r.Host + "/input-requests/inp_" + name + "#grant=" + grant})
		case r.Method == "GET":
			state := "awaiting_transfer"
			if o.completes.Load() > 0 {
				state = "ready"
			}
			if o.expired.Load() {
				state = "expired"
			}
			sum, _ := o.digest.Load().(string)
			respond(map[string]any{"import_id": "imp_" + name, "input_request_id": "inp_" + name, "state": state, "asset_uri": profile.refPrefix + "fixed", "sha256": sum, "domain_state": profile.domainState})
		default:
			t.Errorf("unexpected owner call %s %s", r.Method, r.URL.Path)
			w.WriteHeader(404)
		}
	}))
	t.Cleanup(o.server.Close)
	return o
}

func relayKeyFixture(t *testing.T, dir string) string {
	t.Helper()
	key := filepath.Join(dir, "relay.key")
	if err := os.WriteFile(key, []byte("fixture-relay-key"), 0600); err != nil {
		t.Fatal(err)
	}
	return key
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var b bytes.Buffer
	if err := png.Encode(&b, image.NewRGBA(image.Rect(0, 0, 4, 4))); err != nil {
		t.Fatal(err)
	}
	return b.Bytes()
}

func mp4Bytes() []byte {
	// Minimal ftyp box with a real box size: DetectContentType resolves it
	// to video/mp4.
	box := make([]byte, 32)
	binary.BigEndian.PutUint32(box[0:4], uint32(len(box)))
	copy(box[4:], "ftypisom")
	copy(box[16:], "avc1mp41")
	return box
}

func mp3Bytes() []byte {
	// ID3 header: DetectContentType resolves it to audio/mpeg.
	frame := make([]byte, 4096)
	copy(frame, "ID3\x04\x00\x00\x00\x00\x00\x00")
	return frame
}

// TestRelayCompletesEachOwnerOriginalInput relays one pinned source per owner
// through separate owner origins: each import finishes with that owner's own
// domain pending state and reference scheme, and replaying the same
// idempotency key never transfers or imports a second time.
func TestRelayCompletesEachOwnerOriginalInput(t *testing.T) {
	s, _, local, state := setup(t)
	key := relayKeyFixture(t, t.TempDir())
	sources := map[string]string{
		"eikona": "reference.png",
		"scaena": "reference.mp4",
		"sonora": "reference.mp3",
	}
	contents := map[string][]byte{
		"reference.png": pngBytes(t),
		"reference.mp4": mp4Bytes(),
		"reference.mp3": mp3Bytes(),
	}
	owners := map[string]*fakeOwnerInput{}
	uploaded := map[string]Operation{}
	for owner, name := range sources {
		path := filepath.Join(local, name)
		if err := os.WriteFile(path, contents[name], 0600); err != nil {
			t.Fatal(err)
		}
		op, err := s.Upload(context.Background(), "creative", "", path, name, "upload-"+owner)
		if err != nil {
			t.Fatal(err)
		}
		op = waitOp(t, s, op.ID)
		if op.Status != "completed" {
			t.Fatalf("upload %s: %s %s", owner, op.Status, op.Error)
		}
		uploaded[owner] = op
		owners[owner] = newFakeOwnerInput(t, owner)
		if err := s.ConfigureRelay(RelayTarget{ID: owner, SpaceID: "creative", Project: "p", ConnectionID: "laptop", Owner: owner, Origin: owners[owner].server.URL, KeyFile: key}); err != nil {
			t.Fatal(err)
		}
	}
	for owner, upload := range uploaded {
		op, err := s.StartRelay(context.Background(), upload.FileRef, owner, "key-"+owner)
		if err != nil {
			t.Fatal(owner, err)
		}
		op = waitOp(t, s, op.ID)
		_, profile, _ := relayOwner(owner)
		if op.Status != "completed" || !strings.HasPrefix(op.ResultRef, profile.refPrefix) {
			t.Fatalf("%s relay: %+v", owner, op)
		}
		if o := owners[owner]; o.imports.Load() != 1 || o.puts.Load() != 1 {
			t.Fatalf("%s owner work: imports=%d puts=%d", owner, o.imports.Load(), o.puts.Load())
		}
	}
	// Replay every key: same operation, no second import or transfer.
	for owner, upload := range uploaded {
		op, err := s.StartRelay(context.Background(), upload.FileRef, owner, "key-"+owner)
		if err != nil || op.Status != "completed" {
			t.Fatalf("replay %s: %+v %v", owner, op, err)
		}
		if o := owners[owner]; o.imports.Load() != 1 || o.puts.Load() != 1 {
			t.Fatalf("%s replay duplicated work: imports=%d puts=%d", owner, o.imports.Load(), o.puts.Load())
		}
	}
	contents2, err := os.ReadFile(filepath.Join(state, "state.sqlite"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(contents2), "grant-") {
		t.Fatal("transient grant persisted")
	}
}

func TestRelaySourceAndTargetBoundaries(t *testing.T) {
	s, provider, local, _ := setup(t)
	key := relayKeyFixture(t, t.TempDir())
	owner := newFakeOwnerInput(t, "eikona")
	// Unknown owners never reach registration.
	if err := s.ConfigureRelay(RelayTarget{ID: "auctra", SpaceID: "creative", Project: "p", ConnectionID: "c", Owner: "auctra", Origin: owner.server.URL, KeyFile: key}); err == nil || err.Error() != "invalid_relay_target" {
		t.Fatalf("unknown owner accepted: %v", err)
	}
	// Non-loopback plain http origins are refused before any network use.
	if err := s.ConfigureRelay(RelayTarget{ID: "remote", SpaceID: "creative", Project: "p", ConnectionID: "c", Owner: "eikona", Origin: "http://storage.example.com", KeyFile: key}); err == nil || err.Error() != "invalid_relay_target" {
		t.Fatalf("insecure origin accepted: %v", err)
	}
	// https origins register without probing the endpoint.
	if err := s.ConfigureRelay(RelayTarget{ID: "trusted", SpaceID: "creative", Project: "p", ConnectionID: "c", Owner: "sonora", Origin: "https://private.example.com", KeyFile: key}); err != nil {
		t.Fatal(err)
	}
	// Duplicate identity with a different binding is a conflict.
	if err := s.ConfigureRelay(RelayTarget{ID: "trusted", SpaceID: "creative", Project: "other", ConnectionID: "c2", Owner: "eikona", Origin: "https://private.example.com", KeyFile: key}); err == nil || err.Error() != "relay_target_conflict" {
		t.Fatalf("conflict accepted: %v", err)
	}
	if err := s.ConfigureRelay(RelayTarget{ID: "images", SpaceID: "creative", Project: "p", ConnectionID: "laptop", Owner: "eikona", Origin: owner.server.URL, KeyFile: key}); err != nil {
		t.Fatal(err)
	}
	upload := uploadFixture(t, s, local)
	inject := func(ref, item string, size int64, trashed bool) {
		f := File{Ref: ref, ItemID: item, DriveID: "drive", Version: "version1", Size: size, SpaceID: "creative", MIME: "image/png"}
		if trashed {
			now := time.Now().UTC()
			f.Trashed = &now
		}
		if err := s.DB.Create(&f).Error; err != nil {
			t.Fatal(err)
		}
		provider.files[item] = File{ItemID: item, DriveID: "drive", Version: "version1", Size: size}
		provider.data[item] = pngBytes(t)
	}
	// Empty, oversize and trashed pinned sources never enqueue a relay.
	inject("drivebridge://file/empty", "empty", 0, false)
	if _, err := s.StartRelay(context.Background(), "drivebridge://file/empty", "images", "x2"); err == nil || err.Error() != "source_not_allowed" {
		t.Fatalf("empty source accepted: %v", err)
	}
	inject("drivebridge://file/oversize", "oversize", (128<<20)+1, false)
	if _, err := s.StartRelay(context.Background(), "drivebridge://file/oversize", "images", "x3"); err == nil || err.Error() != "source_not_allowed" {
		t.Fatalf("oversize source accepted: %v", err)
	}
	inject("drivebridge://file/trashed", "trashed", 64, true)
	if _, err := s.StartRelay(context.Background(), "drivebridge://file/trashed", "images", "x4"); err == nil || err.Error() != "source_not_allowed" {
		t.Fatalf("trashed source accepted: %v", err)
	}
	// A pinned identity from another drive is outside the bound space.
	inject2 := File{Ref: "drivebridge://file/foreign", ItemID: "foreign-item", DriveID: "other-drive", Version: "version1", Size: 64, SpaceID: "creative", MIME: "image/png"}
	if err := s.DB.Create(&inject2).Error; err != nil {
		t.Fatal(err)
	}
	provider.files["foreign-item"] = File{ItemID: "foreign-item", DriveID: "other-drive", Version: "version1", Size: 64}
	provider.data["foreign-item"] = pngBytes(t)
	if _, err := s.StartRelay(context.Background(), "drivebridge://file/foreign", "images", "x1"); err == nil {
		t.Fatal("foreign drive accepted")
	}
	// An unknown target id never reveals existence; disable names it.
	if _, err := s.StartRelay(context.Background(), upload.FileRef, "unknown", "x5"); err == nil || err.Error() != "relay_target_unavailable" {
		t.Fatalf("unknown target accepted: %v", err)
	}
	if err := s.DisableRelay("missing"); err == nil || err.Error() != "relay_target_not_found" {
		t.Fatalf("missing disable target: %v", err)
	}
	if err := s.DisableRelay("images"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.StartRelay(context.Background(), upload.FileRef, "images", "x6"); err == nil || err.Error() != "relay_target_unavailable" {
		t.Fatalf("disabled target accepted: %v", err)
	}
	// Owner media adapters: an image pinned for Eikona does not satisfy a
	// Sonora audio input even when the target is otherwise valid.
	sonora := newFakeOwnerInput(t, "sonora")
	if err := s.ConfigureRelay(RelayTarget{ID: "tracks", SpaceID: "creative", Project: "p", ConnectionID: "laptop", Owner: "sonora", Origin: sonora.server.URL, KeyFile: key}); err != nil {
		t.Fatal(err)
	}
	op, err := s.StartRelay(context.Background(), upload.FileRef, "tracks", "x7")
	if err != nil {
		t.Fatal(err)
	}
	op = waitOp(t, s, op.ID)
	if op.Status != "failed" || op.Error != "unsupported_media" {
		t.Fatalf("owner media adapter skipped: %+v", op)
	}
	if sonora.imports.Load() != 0 {
		t.Fatal("media rejection still created an import")
	}
}

func TestRelayGenericBackendSource(t *testing.T) {
	s, local, _ := genericFixture(t)
	if err := s.AddSpace(Space{ID: "nas", Backend: "generic", Remote: "nas", LocalRoot: local, MaxBytes: 1 << 20}); err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(local, "reference.png")
	if err := os.WriteFile(source, pngBytes(t), 0600); err != nil {
		t.Fatal(err)
	}
	upload, err := s.Upload(context.Background(), "nas", "", source, "reference.png", "generic-upload")
	if err != nil {
		t.Fatal(err)
	}
	upload = waitCompleted(t, s, upload.ID)
	owner := newFakeOwnerInput(t, "eikona")
	key := relayKeyFixture(t, t.TempDir())
	if err := s.ConfigureRelay(RelayTarget{ID: "images", SpaceID: "nas", Project: "p", ConnectionID: "laptop", Owner: "eikona", Origin: owner.server.URL, KeyFile: key}); err != nil {
		t.Fatal(err)
	}
	op, err := s.StartRelay(context.Background(), upload.FileRef, "images", "generic-key")
	if err != nil {
		t.Fatal(err)
	}
	op = waitOp(t, s, op.ID)
	if op.Status != "completed" || !strings.HasPrefix(op.ResultRef, "eikona://asset/") || owner.imports.Load() != 1 || owner.puts.Load() != 1 {
		t.Fatalf("generic backend relay: %+v imports=%d puts=%d", op, owner.imports.Load(), owner.puts.Load())
	}
}

func TestRelayExpiredInputIsNotResurrected(t *testing.T) {
	s, _, local, _ := setup(t)
	upload := uploadFixture(t, s, local)
	owner := newFakeOwnerInput(t, "scaena")
	key := relayKeyFixture(t, t.TempDir())
	if err := s.ConfigureRelay(RelayTarget{ID: "shots", SpaceID: "creative", Project: "p", ConnectionID: "laptop", Owner: "scaena", Origin: owner.server.URL, KeyFile: key}); err != nil {
		t.Fatal(err)
	}
	owner.loseComplete.Store(true)
	op, err := s.StartRelay(context.Background(), upload.FileRef, "shots", "expire-key")
	if err != nil {
		t.Fatal(err)
	}
	op = waitOp(t, s, op.ID)
	if op.Status != "failed" || op.ImportID == "" {
		t.Fatalf("setup run: %+v", op)
	}
	owner.expired.Store(true)
	if _, err := s.Resume(op.ID); err != nil {
		t.Fatal(err)
	}
	op = waitOp(t, s, op.ID)
	if op.Status != "failed" || op.Error != "relay_input_terminal" {
		t.Fatalf("expired input resurrected: %+v", op)
	}
	if owner.imports.Load() != 1 || owner.puts.Load() != 1 {
		t.Fatalf("expired resume recreated work: imports=%d puts=%d", owner.imports.Load(), owner.puts.Load())
	}
}

func TestRelaySourceChangeBlocksRecovery(t *testing.T) {
	s, provider, local, _ := setup(t)
	source := filepath.Join(local, "track.mp3")
	if err := os.WriteFile(source, mp3Bytes(), 0600); err != nil {
		t.Fatal(err)
	}
	upload, err := s.Upload(context.Background(), "creative", "", source, "track.mp3", "track-upload")
	if err != nil {
		t.Fatal(err)
	}
	upload = waitOp(t, s, upload.ID)
	if upload.Status != "completed" {
		t.Fatalf("audio upload: %+v", upload)
	}
	owner := newFakeOwnerInput(t, "sonora")
	key := relayKeyFixture(t, t.TempDir())
	if err := s.ConfigureRelay(RelayTarget{ID: "tracks", SpaceID: "creative", Project: "p", ConnectionID: "laptop", Owner: "sonora", Origin: owner.server.URL, KeyFile: key}); err != nil {
		t.Fatal(err)
	}
	owner.failPut.Store(true)
	relay, err := s.StartRelay(context.Background(), upload.FileRef, "tracks", "change-key")
	if err != nil {
		t.Fatal(err)
	}
	relay = waitOp(t, s, relay.ID)
	if relay.Status != "failed" || relay.ImportID == "" {
		t.Fatalf("setup run: %+v", relay)
	}
	// Same size, different bytes: the pinned digest no longer matches, so
	// recovery is rejected instead of uploading replacement content.
	var row File
	if err := s.DB.First(&row, "ref = ?", upload.FileRef).Error; err != nil {
		t.Fatal(err)
	}
	changed := mp3Bytes()
	changed[len(changed)-1] ^= 0xff
	provider.data[row.ItemID] = changed
	if _, err := s.Resume(relay.ID); err != nil {
		t.Fatal(err)
	}
	relay = waitOp(t, s, relay.ID)
	if relay.Status != "failed" || relay.Error != "source_changed" {
		t.Fatalf("changed source resumed: %+v", relay)
	}
	if owner.imports.Load() != 1 || owner.puts.Load() != 1 {
		t.Fatalf("changed source re-transferred: imports=%d puts=%d", owner.imports.Load(), owner.puts.Load())
	}
}
