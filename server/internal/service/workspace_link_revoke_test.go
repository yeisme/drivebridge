package service

import (
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

func TestWorkspaceLinksDownloadRevocationCommitsNothing(t *testing.T) {
	s, store, p := blobRig(t)
	var space Space
	if err := s.DB.Where("owner = ?", p.UserID).First(&space).Error; err != nil {
		t.Fatal(err)
	}
	file := uploadBytes(t, s, p, space.ID, "revoked.md", []byte("[self](#anchor)"))
	adapter := &workspaceAfterGetStore{Storage: store, after: func() {
		if err := s.DB.Model(&File{}).Where("id = ?", file.FileID).Update("trashed", true).Error; err != nil {
			t.Fatal(err)
		}
	}}
	s.storage = func(S3Config, bool) (Storage, error) { return adapter, nil }
	if _, err := s.RebuildWorkspaceLinks(t.Context(), p, file.FileID, contract.FileLinkRebuild{VersionID: file.VersionID}, "links-revoked"); err == nil {
		t.Fatal("download revocation ignored")
	}
	for _, model := range []any{&WorkspaceFileLinkIndex{}, &WorkspaceFileLinkProjection{}, &WorkspaceRequest{}} {
		var count int64
		if err := s.DB.Model(model).Count(&count).Error; err != nil || count != 0 {
			t.Fatal("revocation committed projection/receipt", count, err)
		}
	}
	if adapter.body == nil || !adapter.body.closed {
		t.Fatal("revoked body not closed")
	}
}
