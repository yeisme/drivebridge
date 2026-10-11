package service

import (
	"bytes"
	"io"
	"testing"
	"time"
)

func TestWorkspaceDownloadUsesStoredBytesAndCurrentAuthority(t *testing.T) {
	s, _, human := blobRig(t)
	var sp Space
	if err := s.DB.Where("owner = ?", human.UserID).First(&sp).Error; err != nil {
		t.Fatal(err)
	}
	payload := []byte("fixed version fixture")
	uploaded := uploadBytes(t, s, human, sp.ID, "refs/file.txt", payload)
	a := Agent{ID: ID(), Owner: human.UserID, SpaceID: sp.ID, ConnectionID: sp.ConnectionID, Prefix: "refs", RestrictionProfile: "personal_workspace_v1", TokenHash: "fixture"}
	g := WorkspaceGrant{ID: ID(), AgentID: a.ID, Owner: a.Owner, SpaceID: a.SpaceID, ConnectionID: a.ConnectionID, Prefix: a.Prefix, Actions: "files.read", RestrictionProfile: a.RestrictionProfile, ExpiryMode: "never", IssuedAt: time.Now().UTC(), Revision: 1, CredentialGeneration: 1}
	for _, row := range []any{&a, &g} {
		if err := s.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	p := Principal{UserID: a.Owner, Agent: &a}
	_, v, body, err := s.downloadWorkspaceVersion(t.Context(), p, uploaded.FileID, uploaded.VersionID)
	if err != nil {
		t.Fatal(err)
	}
	actual, err := io.ReadAll(body)
	body.Close()
	if err != nil || !bytes.Equal(actual, payload) || v.ID != uploaded.VersionID {
		t.Fatal("fixed bytes not returned")
	}
	if _, _, body, err := s.Download(t.Context(), p, uploaded.FileID, uploaded.VersionID); err == nil {
		body.Close()
		t.Fatal("legacy download admitted workspace identity")
	}
	_, _, opened, err := s.downloadWorkspaceVersion(t.Context(), p, uploaded.FileID, uploaded.VersionID)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	if err := s.DB.Model(&WorkspaceGrant{}).Where("id = ?", g.ID).Update("revoked", true).Error; err != nil {
		t.Fatal(err)
	}
	if n, err := opened.Read(make([]byte, 4)); err == nil || n != 0 {
		t.Fatal("opened body survived revocation")
	}
}
