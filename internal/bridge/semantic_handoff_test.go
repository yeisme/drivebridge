package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the registered JSON-RPC surface, rather than calling Service.Call.
func semanticHandoffTool(t *testing.T, s *Service, name string, args map[string]any) (json.RawMessage, bool) {
	t.Helper()
	request, err := json.Marshal(map[string]any{"jsonrpc": "2.0", "id": 1, "method": "tools/call", "params": map[string]any{"name": name, "arguments": args}})
	if err != nil {
		t.Fatal(err)
	}
	wire, err := json.Marshal(s.MCP().HandleMessage(context.Background(), request))
	if err != nil {
		t.Fatal(err)
	}
	var response struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(wire, &response); err != nil || len(response.Result.Content) == 0 {
		t.Fatalf("missing MCP result: %s (%v)", wire, err)
	}
	for _, secret := range []string{"ephemeral-test-value", "Bearer ", "https://", s.state} {
		if bytes.Contains(wire, []byte(secret)) {
			t.Fatal("MCP exposed private transport authority")
		}
	}
	return json.RawMessage(response.Result.Content[0].Text), response.Result.IsError
}

func TestSemanticHandoffMCPVerifiedBytesAndLegacyReference(t *testing.T) {
	s, p, _, _ := setup(t)
	s.Handoff = true
	raw := pngBytes(t)
	// A legacy metadata-only receipt has an opaque ETag, not a byte digest.
	file := File{Ref: "drivebridge://file/legacy", SpaceID: "creative", DriveID: "drive", ItemID: "legacy", Version: `"opaque-etag,17"`, Name: "source.png", Size: int64(len(raw)), MIME: "application/octet-stream"}
	p.files[file.ItemID], p.data[file.ItemID] = file, raw
	if err := s.DB.Create(&file).Error; err != nil {
		t.Fatal(err)
	}
	body, failed := semanticHandoffTool(t, s, "stat", map[string]any{"file_ref": file.Ref})
	var metadata File
	if err := json.Unmarshal(body, &metadata); err != nil || failed || metadata.SHA256 != "" || metadata.Version != file.Version {
		t.Fatalf("metadata invented bytes: %s (%v)", body, err)
	}
	body, failed = semanticHandoffTool(t, s, "prepare_reference", map[string]any{"file_ref": file.Ref})
	var prepared struct {
		File   File   `json:"file"`
		Status string `json:"status"`
	}
	want, _, _ := digestReader(bytes.NewReader(raw))
	if err := json.Unmarshal(body, &prepared); err != nil || failed || prepared.File.Ref != file.Ref || prepared.File.Version != file.Version || prepared.File.SHA256 != want || prepared.File.MIME != "image/png" || prepared.Status != "ready_for_owner_acquisition" {
		t.Fatalf("unverified preparation: %s (%v)", body, err)
	}
	if p.accesses != 0 || p.opens != 1 {
		t.Fatalf("prepare acquired URL or skipped bytes: accesses=%d opens=%d", p.accesses, p.opens)
	}
	body, failed = semanticHandoffTool(t, s, "prepare_reference", map[string]any{"file_ref": file.Ref})
	if failed || p.opens != 1 {
		t.Fatalf("verified exact-version cache lost: %s", body)
	}
	p.mu.Lock()
	current := p.files[file.ItemID]
	current.Version = "new-etag"
	p.files[file.ItemID] = current
	p.mu.Unlock()
	body, failed = semanticHandoffTool(t, s, "prepare_reference", map[string]any{"file_ref": file.Ref})
	if !failed || string(body) != "file_version_changed" {
		t.Fatalf("prepared version followed head: %s", body)
	}
	var retained File
	if err := s.DB.First(&retained, "ref = ?", file.Ref).Error; err != nil || retained.Version != file.Version || retained.SHA256 != want {
		t.Fatal("source drift overwrote historical receipt", err)
	}
}

type semanticHandoffProvider struct {
	*fakeProvider
	afterOpen func()
	statErr   error
}

func (p *semanticHandoffProvider) Stat(ctx context.Context, space Space, id string) (File, error) {
	if p.statErr != nil {
		return File{}, p.statErr
	}
	return p.fakeProvider.Stat(ctx, space, id)
}

func (p *semanticHandoffProvider) Open(ctx context.Context, space Space, file File) (io.ReadCloser, error) {
	body, err := p.fakeProvider.Open(ctx, space, file)
	if err == nil && p.afterOpen != nil {
		p.afterOpen()
	}
	return body, err
}

func TestSemanticHandoffRejectsEmptyDriftRevocationAndCrossSpace(t *testing.T) {
	for _, scenario := range []string{"empty", "drift_during_read", "revoked_during_read", "cross_space", "wrong_project", "missing"} {
		t.Run(scenario, func(t *testing.T) {
			s, original, _, _ := setup(t)
			s.Handoff = true
			p := &semanticHandoffProvider{fakeProvider: original}
			s.Provider = p
			raw := pngBytes(t)
			file := File{Ref: "drivebridge://file/legacy", SpaceID: "creative", DriveID: "drive", ItemID: "legacy", Version: "etag-1", Name: "reference.png", Size: int64(len(raw))}
			want := "file_version_changed"
			request := HandoffRequest{Ref: file.Ref, Project: "project", Actor: "actor"}
			switch scenario {
			case "empty":
				file.Size, raw, want = 0, nil, "file_size_invalid"
			case "drift_during_read":
				p.afterOpen = func() {
					changed := p.files[file.ItemID]
					changed.Version = "etag-2"
					p.files[file.ItemID] = changed
				}
			case "revoked_during_read":
				want = "authentication_required"
				p.afterOpen = func() { p.statErr = errors.New(want) }
			case "cross_space":
				file.DriveID, want = "other-drive", "remote_outside_space"
			case "wrong_project":
				request.Project, want = "other-project", "secure_handoff_unavailable"
			case "missing":
				request.Ref, want = "drivebridge://file/missing", "reference_not_found"
			}
			p.files[file.ItemID], p.data[file.ItemID] = file, raw
			if err := s.DB.Create(&file).Error; err != nil {
				t.Fatal(err)
			}
			if scenario == "empty" {
				body, failed := semanticHandoffTool(t, s, "prepare_reference", map[string]any{"file_ref": file.Ref})
				if !failed || string(body) != "file_size_invalid" || p.opens != 0 {
					t.Fatalf("empty media claimed ready: %s", body)
				}
			}
			payload, _ := json.Marshal(request)
			response := httptest.NewRecorder()
			s.Handler().ServeHTTP(response, httptest.NewRequest(http.MethodPost, "/v1/acquire", bytes.NewReader(payload)))
			if response.Code != http.StatusForbidden || strings.TrimSpace(response.Body.String()) != want || p.accesses != 0 {
				t.Fatalf("unsafe source acquired: status=%d body=%s accesses=%d", response.Code, response.Body.String(), p.accesses)
			}
			var retained File
			if err := s.DB.First(&retained, "ref = ?", file.Ref).Error; err != nil || retained.Verified || retained.SHA256 != "" || retained.Version != file.Version {
				t.Fatal("refusal changed original receipt", err)
			}
		})
	}
}

func TestSemanticHandoffDownloadMapsSourceVersionAndActualDigest(t *testing.T) {
	s, p, local, _ := setup(t)
	raw := pngBytes(t)
	file := File{Ref: "drivebridge://file/legacy", SpaceID: "creative", DriveID: "drive", ItemID: "legacy", Version: "opaque-etag", Name: "source.png", Size: int64(len(raw))}
	p.files[file.ItemID], p.data[file.ItemID] = file, raw
	if err := s.DB.Create(&file).Error; err != nil {
		t.Fatal(err)
	}
	op, err := s.Download(context.Background(), file.Ref, "received.png", "receive")
	if err != nil {
		t.Fatal(err)
	}
	op = waitOp(t, s, op.ID)
	want, _, _ := digestReader(bytes.NewReader(raw))
	if op.Status != "completed" || op.SourceVersion != file.Version || op.ContentSHA256 != want || op.FileRef != file.Ref {
		t.Fatalf("source/digest mapping lost: %+v", op)
	}
	got, err := os.ReadFile(filepath.Join(local, "received.png"))
	if err != nil || !bytes.Equal(got, raw) {
		t.Fatal("received bytes differ", err)
	}
	body, failed := semanticHandoffTool(t, s, "operation_get", map[string]any{"operation_id": op.ID})
	if failed || !bytes.Contains(body, []byte(want)) || !bytes.Contains(body, []byte(file.Version)) {
		t.Fatalf("public receipt omitted source facts: %s", body)
	}
}

func TestSemanticHandoffSameKeyCannotChangeUploadDirectory(t *testing.T) {
	s, p, local, _ := setup(t)
	op, err := s.Upload(context.Background(), "creative", "original", fixture(t, local), "reference.png", "fixed-target")
	if err != nil {
		t.Fatal(err)
	}
	op = waitOp(t, s, op.ID)
	if op.Status != "completed" || op.ContentSHA256 == "" || op.SourceVersion != "" {
		t.Fatalf("upload invented source version or lost bytes: %+v", op)
	}
	if _, err := s.Upload(context.Background(), "creative", "different", filepath.Join(local, "reference.png"), "reference.png", "fixed-target"); err == nil || err.Error() != "idempotency_conflict" {
		t.Fatal("same key changed upload target directory", err)
	}
	if p.uploads != 1 {
		t.Fatal("conflicting request wrote a second target")
	}
}

func TestSemanticHandoffUploadVersionDriftDuringReadbackPreservesCommit(t *testing.T) {
	s, original, local, _ := setup(t)
	p := &semanticHandoffProvider{fakeProvider: original}
	p.afterOpen = func() {
		p.mu.Lock()
		defer p.mu.Unlock()
		for id, file := range p.files {
			file.Version = "replaced-during-readback"
			p.files[id] = file
		}
	}
	s.Provider = p
	op, err := s.Upload(context.Background(), "creative", "", fixture(t, local), "reference.png", "readback-drift")
	if err != nil {
		t.Fatal(err)
	}
	op = waitOp(t, s, op.ID)
	if op.Status != "failed" || op.Error != "file_version_changed" || op.FileRef == "" || op.ContentSHA256 != "" {
		t.Fatalf("readback version drift claimed verified commit: %+v", op)
	}
	var receipt File
	if err := s.DB.First(&receipt, "ref = ?", op.FileRef).Error; err != nil || receipt.Version != "version1" || receipt.SHA256 != "" {
		t.Fatal("drift rewrote original commit metadata", err)
	}
	s.wg.Wait()
	if _, err := s.Resume(op.ID); err != nil {
		t.Fatal(err)
	}
	recovered := waitOp(t, s, op.ID)
	if recovered.Status != "failed" || recovered.Error != "file_version_changed" || recovered.FileRef != op.FileRef || p.uploads != 1 {
		t.Fatalf("resume followed replacement or uploaded again: %+v", recovered)
	}
}

func TestSemanticHandoffCachedDownloadRetainsVersionDigest(t *testing.T) {
	s, p, local, _ := setup(t)
	upload := uploadFixture(t, s, local)
	file, err := s.Stat(context.Background(), upload.FileRef)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"first.png", "cached.png", "cached.png"} {
		op, err := s.Download(context.Background(), file.Ref, target, newID())
		if err != nil {
			t.Fatal(err)
		}
		op = waitOp(t, s, op.ID)
		if op.Status != "completed" || op.ContentSHA256 != file.SHA256 || op.SourceVersion != file.Version {
			t.Fatalf("cached receipt omitted verified facts: %+v", op)
		}
		if target == "cached.png" && (!op.CacheHit || op.Transferred != 0) {
			t.Fatalf("cached receipt invented copied bytes: %+v", op)
		}
	}
	if p.opens != 2 {
		t.Fatalf("cache redownloaded bytes: opens=%d", p.opens)
	}
}
