package service_test

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func TestWorkspaceTaskDetailFixedOutput(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	p := service.Principal{UserID: alice.user.ID}
	op, err := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: alice.space.ID, Path: "task-detail.md", Size: 1, SHA256: service.Hash("a")}, "detail-original")
	if err != nil {
		t.Fatal(err)
	}
	path := "/workspace/tasks/operation/" + op.ID
	get := func() contract.OperationTaskDetail {
		t.Helper()
		var out contract.OperationTaskDetail
		if status := alice.call(t, "GET", path, nil, &out); status != 200 {
			t.Fatalf("detail status %d", status)
		}
		return out
	}
	initial := get()
	if initial.OutputState != "not_published" || len(initial.Outputs) != 0 || initial.Task.OwnerID != op.ID {
		t.Fatal("unpublished output exposed")
	}
	if _, err = f.Service.PutPart(p, op.ID, 0, bytes.NewBufferString("a")); err != nil {
		t.Fatal(err)
	}
	done, err := f.Service.Complete(t.Context(), p, op.ID)
	if err != nil {
		t.Fatal(err)
	}
	published := get()
	if published.OutputState != "available" || len(published.Outputs) != 1 || published.Outputs[0].FileID != done.FileID || published.Outputs[0].VersionID != done.VersionID || published.Outputs[0].SpaceID != done.SpaceID {
		t.Fatal("fixed output missing")
	}
	raw, _ := json.Marshal(published)
	for _, key := range []string{"object_key", "sha256\":", "connection_id", "request_key", "fingerprint", "http://", "https://"} {
		if strings.Contains(string(raw), key) {
			t.Fatal("private field exposed")
		}
	}
	if status := bob.call(t, "GET", path, nil, nil); status != 404 {
		t.Fatalf("foreign detail %d", status)
	}
	_, client := agent(t, alice, "", "read,write")
	if err := client.JSON(t.Context(), "GET", "/api/v1"+path, "", nil, nil); err == nil {
		t.Fatal("Agent used human detail")
	}
	for _, bad := range []string{path + "?version_id=latest", "/workspace/tasks/upload_batch/" + op.ID, path + "?kind=operation"} {
		if status := alice.call(t, "GET", bad, nil, nil); status != 400 {
			t.Fatalf("invalid detail query %d", status)
		}
	}
	next, err := f.Service.CreateUpload(p, service.UploadRequest{SpaceID: alice.space.ID, Path: "task-detail.md", FileID: done.FileID, ExpectedVersion: done.VersionID, Size: 1, SHA256: service.Hash("b")}, "detail-next")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.PutPart(p, next.ID, 0, bytes.NewBufferString("b")); err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.Complete(t.Context(), p, next.ID); err != nil {
		t.Fatal(err)
	}
	if after := get(); after.Outputs[0].VersionID != done.VersionID || after.Task.Revision != published.Task.Revision {
		t.Fatal("current head substituted original version")
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", done.FileID).Update("trashed", true).Error; err != nil {
		t.Fatal(err)
	}
	if after := get(); after.OutputState != "unavailable" || len(after.Outputs) != 0 || after.Task.OwnerStatus != "completed" {
		t.Fatal("trashed output visible or owner rewritten")
	}
	if err = f.Service.DB.Model(&service.File{}).Where("id = ?", done.FileID).Update("trashed", false).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Delete(&service.Version{}, "id = ?", done.VersionID).Error; err != nil {
		t.Fatal(err)
	}
	if after := get(); after.OutputState != "unavailable" || len(after.Outputs) != 0 {
		t.Fatal("retained latest version substituted missing output")
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", alice.user.ID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", path, nil, nil); status != 403 {
		t.Fatalf("disabled human detail %d", status)
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", alice.user.ID).Update("disabled", false).Error; err != nil {
		t.Fatal(err)
	}
	if err = f.Service.DB.Migrator().DropTable(&service.Version{}); err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", path, nil, nil); status != 500 {
		t.Fatalf("dependency failure status %d", status)
	}
}

func TestWorkspaceTaskDetailStagingAndScope(t *testing.T) {
	f := rig(t)
	alice, _ := login(t, f, "alice")
	op := service.Operation{ID: service.ID(), Key: "detail-staging", Owner: alice.user.ID, ActorID: alice.user.ID, AgentID: "agent-fixture", SpaceID: alice.space.ID, Path: ".inbox-staging/private", Purpose: "inbox_staging", WorkspaceGrantID: "grant-fixture", Kind: "upload", Status: "awaiting_upload"}
	if err := f.Service.DB.Create(&op).Error; err != nil {
		t.Fatal(err)
	}
	path := "/workspace/tasks/operation/" + op.ID
	if status := alice.call(t, "GET", path, nil, nil); status != 503 {
		t.Fatalf("unbound staging status %d", status)
	}
	sub := service.WorkspaceSubmission{ID: service.ID(), Owner: op.Owner, AgentID: op.AgentID, GrantID: op.WorkspaceGrantID, SpaceID: op.SpaceID, UploadOperationID: op.ID, CreationKey: "detail", RequestDigest: "fixture", State: "uploading", Revision: 1}
	if err := f.Service.DB.Create(&sub).Error; err != nil {
		t.Fatal(err)
	}
	var detail contract.OperationTaskDetail
	if status := alice.call(t, "GET", path, nil, &detail); status != 200 || detail.Task.Title != "Agent submission" || detail.OutputState != "not_published" {
		t.Fatal("authorized staging detail unavailable")
	}
	// A space reclassified into another tenant cannot be read through the ID.
	if err := f.Service.DB.Model(&service.Space{}).Where("id = ?", alice.space.ID).Update("tenant_id", "foreign-tenant").Error; err != nil {
		t.Fatal(err)
	}
	if status := alice.call(t, "GET", path, nil, nil); status != 404 {
		t.Fatalf("cross tenant detail %d", status)
	}
}
