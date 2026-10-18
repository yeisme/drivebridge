package bridge

import (
	"bytes"
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

// This controlled owner serves only existing task/read-grant contracts. Any
// generation, acceptance, redirect or unrecognized endpoint fails the test.
func semanticOutputOwner(t *testing.T, raw []byte, status, imported, digest string, grantStatus int) (*ScaenaOutput, *atomic.Int64) {
	t.Helper()
	grants := &atomic.Int64{}
	owner := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+"semantic-owner-secret" {
			t.Error("owner authentication missing")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		encode := func(data any) { _ = json.NewEncoder(w).Encode(map[string]any{"data": data}) }
		switch r.Method + " " + r.URL.Path {
		case "GET /api/v1/projects/project/video-tasks/video-provider-task:one":
			encode(map[string]any{"status": status, "project_ref": "project", "submission_ref": "video-provider-task:one", "asset_ref": "asset:fixed", "import_status": imported})
		case "POST /api/v1/projects/project/artifacts/asset:fixed/access-grants":
			grants.Add(1)
			if r.Header.Get("Idempotency-Key") == "" {
				t.Error("read grant lacks identity")
			}
			encode(map[string]any{"grant_ref": "grant:fixed", "size_bytes": len(raw), "digest": digest, "access_kind": "resource_link", "resource_link": "https://untrusted.invalid/secret"})
		case "GET /api/v1/projects/project/artifacts/asset:fixed/access-grants/grant:fixed":
			w.WriteHeader(grantStatus)
			if grantStatus == http.StatusOK {
				_, _ = w.Write(raw)
			}
		default:
			t.Errorf("unexpected owner side effect: %s %s", r.Method, r.URL.Path)
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	t.Cleanup(owner.Close)
	client, err := NewScaenaOutput(owner.URL, func() (string, error) { return "semantic-owner-secret", nil })
	if err != nil {
		t.Fatal(err)
	}
	return client, grants
}

func TestSemanticHandoffSaveOutputRejectsUnreadyAndInvalidGrant(t *testing.T) {
	for _, tc := range []struct {
		name, status, imported, digest, want string
		grantStatus                          int
	}{
		{"pending", "running", "imported", "valid", "output_not_ready", 200},
		{"failed", "failed", "imported", "valid", "output_not_ready", 200},
		{"not_imported", "succeeded", "pending", "valid", "output_not_ready", 200},
		{"etag_is_not_digest", "succeeded", "imported", "sha256:etag-17", "output_grant_invalid", 200},
		{"expired", "succeeded", "imported", "valid", "scaena_unavailable", 410},
		{"revoked", "succeeded", "imported", "valid", "scaena_unavailable", 403},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, p, _, _ := setup(t)
			raw := pngBytes(t)
			digest, _, _ := digestReader(bytes.NewReader(raw))
			if tc.digest != "valid" {
				digest = strings.TrimPrefix(tc.digest, "sha256:")
			}
			owner, grants := semanticOutputOwner(t, raw, tc.status, tc.imported, "sha256:"+digest, tc.grantStatus)
			s.Output = owner
			body, failed := semanticHandoffTool(t, s, "save_output", map[string]any{"space_id": "creative", "submission_ref": "video-provider-task:one", "idempotency_key": "output"})
			var queued Operation
			if err := json.Unmarshal(body, &queued); err != nil || failed {
				t.Fatalf("save enqueue: %s (%v)", body, err)
			}
			op := waitOp(t, s, queued.ID)
			if op.Status != "failed" || op.Error != tc.want || p.uploads != 0 || op.FileRef != "" {
				t.Fatalf("unready output published: %+v uploads=%d", op, p.uploads)
			}
			if tc.want == "output_not_ready" && grants.Load() != 0 {
				t.Fatal("unready task acquired bytes")
			}
		})
	}
}

func TestSemanticHandoffSaveOutputResumePreservesTargetAndDigest(t *testing.T) {
	s, p, local, state := setup(t)
	raw := pngBytes(t)
	digest, _, _ := digestReader(bytes.NewReader(raw))
	owner, grants := semanticOutputOwner(t, raw, "succeeded", "imported", "sha256:"+digest, http.StatusOK)
	s.Output = owner
	p.failAfterPut = true
	op, err := s.SaveOutput("creative", "video-provider-task:one", "lost-response")
	if err != nil {
		t.Fatal(err)
	}
	op = waitOp(t, s, op.ID)
	if op.Status != "failed" || p.uploads != 1 {
		t.Fatalf("response loss not injected: %+v", op)
	}
	s.Close()
	reopened, err := New(state, p, true)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(reopened.Close)
	reopened.Output = owner
	resumed, err := reopened.Resume(op.ID)
	if err != nil || resumed.ID != op.ID || resumed.Target != op.Target {
		t.Fatalf("resume changed identity: %+v %v", resumed, err)
	}
	resumed = waitOp(t, reopened, op.ID)
	if resumed.Status != "completed" || resumed.ContentSHA256 != digest || resumed.SourceRef != op.SourceRef || resumed.Target != op.Target || p.uploads != 1 || grants.Load() != 2 {
		t.Fatalf("recovery duplicated or lost evidence: %+v uploads=%d grants=%d", resumed, p.uploads, grants.Load())
	}
	// Target version belongs to File; it must never be claimed as source ETag.
	stored, err := reopened.Stat(context.Background(), resumed.FileRef)
	if err != nil || stored.SHA256 != digest || stored.Version == digest || resumed.SourceVersion != "" {
		t.Fatal("source/target versions conflated", err)
	}
	got, err := os.ReadFile(filepath.Join(local, op.LocalPath))
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatal("verified recovery source lost", err)
	}
	if replay, err := reopened.SaveOutput("creative", op.SourceRef, "lost-response"); err != nil || replay.ID != op.ID || replay.Target != op.Target {
		t.Fatal("replay changed original target", err)
	}
	if _, err := reopened.SaveOutput("creative", "video-provider-task:different", "lost-response"); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("same key changed output source", err)
	}
	body, failed := semanticHandoffTool(t, reopened, "operation_get", map[string]any{"operation_id": op.ID})
	if failed || bytes.Contains(body, []byte("semantic-owner-secret")) || bytes.Contains(body, []byte("accepted")) {
		t.Fatal("save receipt leaked credentials or invented acceptance")
	}
}

func TestSemanticHandoffSaveOutputPreservesExistingLocalTarget(t *testing.T) {
	s, p, local, _ := setup(t)
	raw := pngBytes(t)
	digest, _, _ := digestReader(bytes.NewReader(raw))
	s.Output, _ = semanticOutputOwner(t, raw, "succeeded", "imported", "sha256:"+digest, http.StatusOK)
	op := Operation{ID: newID(), SpaceID: "creative", Kind: "save_output", SourceRef: "video-provider-task:one", LocalPath: ".drivebridge-output/existing.mp4"}
	op.Target = op.ID + "-output.mp4"
	if err := os.Mkdir(filepath.Join(local, ".drivebridge-output"), 0700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(local, op.LocalPath)
	if err := os.WriteFile(path, []byte("original-user-file"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := s.saveOutput(context.Background(), &op); err == nil || err.Error() != "target_conflict" {
		t.Fatal("existing output target replaced", err)
	}
	got, err := os.ReadFile(path)
	if err != nil || string(got) != "original-user-file" || p.uploads != 0 {
		t.Fatal("target conflict lost user bytes", err)
	}
}

type semanticLateCommitProvider struct {
	*fakeProvider
	committed chan struct{}
}

func (p semanticLateCommitProvider) Upload(ctx context.Context, space Space, name string, source io.Reader, size int64) (File, error) {
	file, err := p.fakeProvider.Upload(context.Background(), space, name, source, size)
	close(p.committed)
	<-ctx.Done()
	return file, err
}

func TestSemanticHandoffCancelLateCommitRetainsOriginalRecoveryIdentity(t *testing.T) {
	s, p, _, _ := setup(t)
	raw := pngBytes(t)
	digest, _, _ := digestReader(bytes.NewReader(raw))
	s.Output, _ = semanticOutputOwner(t, raw, "succeeded", "imported", "sha256:"+digest, http.StatusOK)
	committed := make(chan struct{})
	s.Provider = semanticLateCommitProvider{fakeProvider: p, committed: committed}
	op, err := s.SaveOutput("creative", "video-provider-task:one", "late-cancel")
	if err != nil {
		t.Fatal(err)
	}
	<-committed
	if _, err := s.Cancel(op.ID); err != nil {
		t.Fatal(err)
	}
	terminal := waitOp(t, s, op.ID)
	if terminal.Status != "interrupted" || terminal.Error != "resume_required" || terminal.FileRef == "" || terminal.ContentSHA256 != "" || terminal.ID != op.ID || terminal.Target != op.Target {
		t.Fatalf("late cancellation lost identity: %+v", terminal)
	}
	body, failed := semanticHandoffTool(t, s, "operation_get", map[string]any{"operation_id": op.ID})
	if failed || !bytes.Contains(body, []byte(terminal.FileRef)) || !bytes.Contains(body, []byte("resume_required")) {
		t.Fatal("late committed side effect hidden from public recovery")
	}
	s.wg.Wait()
	if _, err := s.Resume(op.ID); err != nil {
		t.Fatal(err)
	}
	recovered := waitOp(t, s, op.ID)
	if recovered.Status != "completed" || recovered.FileRef != terminal.FileRef || recovered.ContentSHA256 != digest {
		t.Fatalf("late commit did not verify original target: %+v", recovered)
	}
	// Cancellation is not proof of rollback: inspect the controlled storage.
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(p.files) != 1 || p.uploads != 1 {
		t.Fatal("late remote side effect was removed or duplicated")
	}
	for _, file := range p.files {
		if file.Name != terminal.Target || !bytes.Equal(p.data[file.ItemID], raw) {
			t.Fatal("original operation cannot identify actual side effect")
		}
	}
}

type semanticUnknownCommitProvider struct {
	*fakeProvider
	started      chan struct{}
	refuseLookup bool
}

func (p semanticUnknownCommitProvider) Upload(ctx context.Context, space Space, target string, source io.Reader, size int64) (File, error) {
	_, err := p.fakeProvider.Upload(context.Background(), space, target, source, size)
	close(p.started)
	<-ctx.Done()
	if err != nil {
		return File{}, err
	}
	return File{}, ctx.Err()
}

func (p semanticUnknownCommitProvider) List(ctx context.Context, space Space, dir string) ([]File, error) {
	select {
	case <-p.started:
		if p.refuseLookup {
			return nil, context.DeadlineExceeded
		}
	default:
	}
	return p.fakeProvider.List(ctx, space, dir)
}

func TestSemanticHandoffCancelLostResponseReconcilesWithoutResubmission(t *testing.T) {
	for _, refuse := range []bool{false, true} {
		t.Run(map[bool]string{false: "commit_found", true: "lookup_unavailable"}[refuse], func(t *testing.T) {
			s, p, local, _ := setup(t)
			started := make(chan struct{})
			s.Provider = semanticUnknownCommitProvider{fakeProvider: p, started: started, refuseLookup: refuse}
			op, err := s.Upload(context.Background(), "creative", "", fixture(t, local), "reference.png", "unknown-cancel")
			if err != nil {
				t.Fatal(err)
			}
			<-started
			if _, err := s.Cancel(op.ID); err != nil {
				t.Fatal(err)
			}
			op = waitOp(t, s, op.ID)
			if op.Status != "interrupted" || op.Error != "resume_required" || (!refuse && op.FileRef == "") || p.uploads != 1 {
				t.Fatalf("unknown commit claimed rollback: %+v uploads=%d", op, p.uploads)
			}
			s.wg.Wait()
			s.Provider = p
			if _, err := s.Resume(op.ID); err != nil {
				t.Fatal(err)
			}
			recovered := waitOp(t, s, op.ID)
			if recovered.Status != "completed" || recovered.ID != op.ID || recovered.Target != op.Target || p.uploads != 1 {
				t.Fatalf("reconciliation reuploaded: %+v", recovered)
			}
		})
	}
}

func TestSemanticHandoffSaveOutputCannotSubstituteChangedArtifactOnResume(t *testing.T) {
	s, p, _, _ := setup(t)
	raw := pngBytes(t)
	digest, _, _ := digestReader(bytes.NewReader(raw))
	s.Output, _ = semanticOutputOwner(t, raw, "succeeded", "imported", "sha256:"+digest, http.StatusOK)
	p.failAfterPut = true
	op, err := s.SaveOutput("creative", "video-provider-task:one", "fixed-artifact")
	if err != nil {
		t.Fatal(err)
	}
	op = waitOp(t, s, op.ID)
	if op.Status != "failed" {
		t.Fatalf("missing response loss: %+v", op)
	}
	s.wg.Wait()
	changed := append(bytes.Clone(raw), byte(1))
	changedDigest, _, _ := digestReader(bytes.NewReader(changed))
	s.Output, _ = semanticOutputOwner(t, changed, "succeeded", "imported", "sha256:"+changedDigest, http.StatusOK)
	if _, err := s.Resume(op.ID); err != nil {
		t.Fatal(err)
	}
	resumed := waitOp(t, s, op.ID)
	if resumed.Status != "failed" || resumed.Error != "target_conflict" || resumed.Target != op.Target || p.uploads != 1 {
		t.Fatalf("resume substituted new artifact: %+v", resumed)
	}
}
