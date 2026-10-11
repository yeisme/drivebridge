package service

import (
	"io"
	"strings"
	"testing"
	"time"
)

func TestWorkspaceFixedVersionReaderRechecksAuthority(t *testing.T) {
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&User{}, &Space{}, &Agent{}, &File{}, &Version{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	a := Agent{ID: "agent", Owner: "human", SpaceID: "space", ConnectionID: "conn", Prefix: "refs", TokenHash: "fixture", RestrictionProfile: "personal_workspace_v1"}
	grant := WorkspaceGrant{ID: "grant", AgentID: a.ID, Owner: a.Owner, TenantID: "tenant", SpaceID: a.SpaceID, ConnectionID: a.ConnectionID, Prefix: a.Prefix, Actions: "files.read", RestrictionProfile: a.RestrictionProfile, ExpiryMode: "never", IssuedAt: time.Now().UTC(), Revision: 1, CredentialGeneration: 1}
	file := File{ID: "file", Owner: a.Owner, SpaceID: a.SpaceID, Path: "refs/file", VersionID: "current"}
	version := Version{ID: "fixed", Owner: a.Owner, FileID: file.ID, ConnectionID: a.ConnectionID}
	for _, row := range []any{&User{ID: a.Owner, Issuer: "fixture", Subject: a.Owner}, &Space{ID: a.SpaceID, Owner: a.Owner, TenantID: grant.TenantID, ConnectionID: a.ConnectionID}, &a, &grant, &file, &version} {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{DB: db}
	p := Principal{UserID: a.Owner, TenantID: grant.TenantID, Agent: &a}
	newReader := func() *workspaceVersionReader {
		return &workspaceVersionReader{ReadCloser: io.NopCloser(strings.NewReader("abcdef")), service: s, principal: p, fileID: file.ID, versionID: version.ID}
	}
	reader := newReader()
	defer reader.Close()
	buf := make([]byte, 2)
	if n, err := reader.Read(buf); err != nil || n != 2 || string(buf) != "ab" {
		t.Fatalf("fixed historical version read failed: %v", err)
	}
	if err := db.Model(&WorkspaceGrant{}).Where("id = ?", grant.ID).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	if n, err := reader.Read(buf); err == nil || n != 0 {
		t.Fatal("open stream survived revocation")
	}
	if err := db.Model(&WorkspaceGrant{}).Where("id = ?", grant.ID).Update("revoked", false).Error; err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"private/file", "refs/.inbox-staging/file"} {
		if err := db.Model(&File{}).Where("id = ?", file.ID).Update("path", target).Error; err != nil {
			t.Fatal(err)
		}
		r := newReader()
		n, err := r.Read(buf)
		r.Close()
		if err == nil || n != 0 {
			t.Fatal("moved or staging file read allowed")
		}
	}
	if _, _, err := s.workspaceReadVersionOn(db, p, file.ID, ""); err == nil {
		t.Fatal("unpinned read accepted")
	}
}
