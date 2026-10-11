package service

import (
	"testing"
	"time"
)

func TestWorkspaceStatPreservesCurrentMetadataAndPinsHistoricalMetadata(t *testing.T) {
	s, _, human := blobRig(t)
	var sp Space
	if err := s.DB.Where("owner = ?", human.UserID).First(&sp).Error; err != nil {
		t.Fatal(err)
	}
	op := uploadBytes(t, s, human, sp.ID, "refs/file.txt", []byte("fixture"))
	a := Agent{ID: ID(), Owner: human.UserID, SpaceID: sp.ID, ConnectionID: sp.ConnectionID, Prefix: "refs", RestrictionProfile: "personal_workspace_v1", TokenHash: "fixture-stat"}
	g := WorkspaceGrant{ID: ID(), Owner: a.Owner, AgentID: a.ID, SpaceID: a.SpaceID, ConnectionID: a.ConnectionID, Prefix: a.Prefix, Actions: "files.read", RestrictionProfile: a.RestrictionProfile, ExpiryMode: "never", IssuedAt: time.Now().UTC(), Revision: 1, CredentialGeneration: 1}
	for _, row := range []any{&a, &g} {
		if err := s.DB.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	p := Principal{UserID: a.Owner, Agent: &a}
	changed := time.Now().UTC().Add(time.Hour).Truncate(time.Millisecond)
	if err := s.DB.Model(&File{}).Where("id = ?", op.FileID).Update("updated", changed).Error; err != nil {
		t.Fatal(err)
	}
	current, err := s.workspaceFileStat(p, op.FileID)
	if err != nil {
		t.Fatal(err)
	}
	if !current["file"].(File).Updated.Equal(changed) {
		t.Fatal("current stat replaced file metadata with version creation time")
	}
	var version Version
	if err := s.DB.First(&version, "id = ?", op.VersionID).Error; err != nil {
		t.Fatal(err)
	}
	fixed, err := s.workspaceFileVersionStat(p, op.FileID, op.VersionID)
	if err != nil {
		t.Fatal(err)
	}
	file := fixed["file"].(File)
	if !file.Updated.Equal(version.Created) || file.Size != version.Size || file.MIME != version.MIME || file.VersionID != version.ID {
		t.Fatal("pinned stat did not use fixed-version metadata")
	}
}
