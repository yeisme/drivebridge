package service_test

import (
	"bytes"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

func TestWorkspaceFixedDownloadHTTPRequiresReadAndVersion(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	a, client := agent(t, h, "refs", "read,write")
	payload := []byte("fixed HTTP fixture")
	uploaded, err := client.Upload(t.Context(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "refs/file.txt"}, "upload", bytes.NewReader(payload), nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Service.DB.Model(&service.Agent{}).Where("id = ?", a.ID).Updates(map[string]any{"restriction_profile": "personal_workspace_v1", "connection_id": h.space.ConnectionID, "scopes": ""}).Error; err != nil {
		t.Fatal(err)
	}
	grant := service.WorkspaceGrant{ID: service.ID(), Owner: h.user.ID, AgentID: a.ID, SpaceID: h.space.ID, ConnectionID: h.space.ConnectionID, Prefix: "refs", Actions: "files.read", RestrictionProfile: "personal_workspace_v1", Protocol: "http", ExpiryMode: "never", IssuedAt: time.Now().UTC(), Revision: 1, CredentialGeneration: 1}
	if err := f.Service.DB.Create(&grant).Error; err != nil {
		t.Fatal(err)
	}
	var metadata struct {
		File service.File `json:"file"`
		Ref  string       `json:"ref"`
	}
	if err := client.JSON(t.Context(), "GET", "/api/v1/files/"+uploaded.FileID, "", nil, &metadata); err != nil || metadata.File.VersionID != uploaded.VersionID || metadata.Ref == "" {
		t.Fatalf("workspace stat failed: %v", err)
	}
	var listed []service.File
	if err := client.JSON(t.Context(), "GET", "/api/v1/files?space_id="+h.space.ID+"&q=file", "", nil, &listed); err != nil || len(listed) != 1 || listed[0].ID != uploaded.FileID {
		t.Fatalf("HTTP scoped list failed: %v", err)
	}
	if err := client.JSON(t.Context(), "GET", "/api/v1/files?space_id="+h.space.ID+"&prefix=private", "", nil, nil); err == nil {
		t.Fatal("HTTP list accepted outside prefix")
	}
	for _, query := range []string{"&space_id=other", "&prefix=refs&prefix=private", "&trash=maybe", "&unknown_filter=value"} {
		if err := client.JSON(t.Context(), "GET", "/api/v1/files?space_id="+h.space.ID+query, "", nil, nil); err == nil || err.Error() != "invalid_request" {
			t.Fatalf("invalid list query not rejected: %v", err)
		}
	}
	// Use a dedicated synthetic fixture credential for the transport check.
	const mcpToken = "workspace-read-mcp-fixture-only"
	if err := f.Service.DB.Model(&service.Agent{}).Where("id = ?", a.ID).Update("token_hash", service.Hash(mcpToken)).Error; err != nil {
		t.Fatal(err)
	}
	client, err = sdk.New(h.base, mcpToken)
	if err != nil {
		t.Fatal(err)
	}
	native := workspaceMCP(t, h.base, mcpToken, "capabilities", "stat", "files", "search")
	for _, name := range []string{"files", "search"} {
		result := workspaceMCPCall(t, native, name, map[string]any{"space_id": h.space.ID, "query": "file"}, false)
		if !bytes.Contains(result, []byte(uploaded.FileID)) {
			t.Fatalf("MCP %s omitted readable file", name)
		}
	}
	workspaceMCPCall(t, native, "search", map[string]any{"space_id": h.space.ID, "query": "file", "path": "private"}, true)
	stat := workspaceMCPCall(t, native, "stat", map[string]any{"file_id": uploaded.FileID}, false)
	if !bytes.Contains(stat, []byte(uploaded.VersionID)) || bytes.Contains(stat, []byte(mcpToken)) {
		t.Fatal("MCP stat lost version or leaked credential")
	}
	historical := service.Version{ID: service.ID(), Owner: h.user.ID, FileID: uploaded.FileID, ConnectionID: h.space.ConnectionID, Size: 3, MIME: "text/markdown", Created: time.Now().UTC().Add(-time.Hour)}
	if err := f.Service.DB.Create(&historical).Error; err != nil {
		t.Fatal(err)
	}
	pinnedRef := f.Service.FileRef(service.File{ID: uploaded.FileID, VersionID: historical.ID})
	for _, selector := range []map[string]any{{"ref": pinnedRef}, {"file_id": uploaded.FileID, "version_id": historical.ID}} {
		result := workspaceMCPCall(t, native, "stat", selector, false)
		if !bytes.Contains(result, []byte(historical.ID)) || bytes.Contains(result, []byte(uploaded.VersionID)) {
			t.Fatal("pinned stat substituted current version")
		}
	}
	workspaceMCPCall(t, native, "stat", map[string]any{"ref": pinnedRef, "file_id": uploaded.FileID}, true)
	workspaceMCPCall(t, native, "stat", map[string]any{"ref": "drivebridge://foreign/file/" + uploaded.FileID + "/version/" + historical.ID}, true)
	var out bytes.Buffer
	if err := client.Download(t.Context(), uploaded.FileID, uploaded.VersionID, &out); err != nil || !bytes.Equal(out.Bytes(), payload) {
		t.Fatalf("fixed download failed: %v", err)
	}
	if err := client.Download(t.Context(), uploaded.FileID, "", io.Discard); err == nil {
		t.Fatal("unpinned download accepted")
	}
	if err := client.JSON(t.Context(), "POST", "/api/v1/uploads", "new", map[string]any{}, nil); err == nil {
		t.Fatal("read grant inherited upload")
	}
	if err := f.Service.DB.Model(&service.File{}).Where("id = ?", uploaded.FileID).Update("path", "refs/.inbox-staging/file.txt").Error; err != nil {
		t.Fatal(err)
	}
	if err := client.Download(t.Context(), uploaded.FileID, uploaded.VersionID, io.Discard); err == nil {
		t.Fatal("read grant read staging")
	}
	if err := client.JSON(t.Context(), "GET", "/api/v1/files/"+uploaded.FileID, "", nil, nil); err == nil {
		t.Fatal("workspace stat exposed staging metadata")
	}
	if err := client.JSON(t.Context(), "GET", "/api/v1/files?space_id="+h.space.ID, "", nil, &listed); err != nil || len(listed) != 0 {
		t.Fatal("HTTP list exposed staging")
	}
	for _, name := range []string{"files", "search"} {
		result := workspaceMCPCall(t, native, name, map[string]any{"space_id": h.space.ID, "query": "file"}, false)
		if bytes.Contains(result, []byte(uploaded.FileID)) {
			t.Fatalf("MCP %s exposed staging", name)
		}
	}
	workspaceMCPCall(t, native, "stat", map[string]any{"file_id": uploaded.FileID}, true)
	if err := f.Service.DB.Model(&service.File{}).Where("id = ?", uploaded.FileID).Update("path", "refs/file.txt").Error; err != nil {
		t.Fatal(err)
	}
	if err := f.Service.DB.Model(&service.WorkspaceGrant{}).Where("id = ?", grant.ID).Update("actions", "labels.suggest").Error; err != nil {
		t.Fatal(err)
	}
	if err := client.Download(t.Context(), uploaded.FileID, uploaded.VersionID, io.Discard); err == nil {
		t.Fatal("suggest-only read content")
	}
	if err := client.JSON(t.Context(), "GET", "/api/v1/files/"+uploaded.FileID, "", nil, nil); err == nil {
		t.Fatal("suggest-only read file metadata")
	}
	workspaceMCPCall(t, native, "stat", map[string]any{"file_id": uploaded.FileID}, true)
	// The human path retains its existing unpinned download contract.
	res, err := h.http.Get(h.base + "/api/v1/files/" + uploaded.FileID + "/content")
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	got, err := io.ReadAll(res.Body)
	if err != nil || res.StatusCode != http.StatusOK || !bytes.Equal(got, payload) {
		t.Fatal("legacy human download regressed")
	}
}
