package service

import (
	"fmt"
	"testing"
	"time"
)

func TestAuthorizationExpiryComparesInstantsAcrossOffsets(t *testing.T) {
	s, _, owner := blobRig(t)
	root, err := s.ensureUser("fixture", "time-root", "Root")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.BootstrapGovernance(root.ID); err != nil {
		t.Fatal(err)
	}
	var sp Space
	if err := s.DB.Where("owner = ?", owner.UserID).First(&sp).Error; err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	for _, offset := range []int{-8, 0, 8} {
		for _, live := range []bool{false, true} {
			t.Run(fmt.Sprintf("offset_%d_live_%t", offset, live), func(t *testing.T) {
				delta := -time.Hour
				if live {
					delta = time.Hour
				}
				expires := now.Add(delta).In(time.FixedZone("fixture", offset*3600))
				session := Session{Hash: ID(), Owner: owner.UserID, Expires: expires}
				access := AccessGrant{Hash: ID(), Owner: owner.UserID, Expires: expires}
				grant := AccessRequest{ID: ID(), TenantID: s.Instance, Requester: root.ID, Approver: owner.UserID, SpaceID: sp.ID, Kind: "range", Action: "write", Status: "approved", GrantUntil: expires}
				for _, row := range []any{&session, &access, &grant} {
					if err := s.DB.Create(row).Error; err != nil {
						t.Fatal(err)
					}
				}
				defer s.DB.Delete(&grant)
				checks := []struct {
					name string
					err  error
				}{
					{"session", s.governanceCredential(Principal{UserID: owner.UserID, SessionHash: session.Hash})},
					{"access", s.allowOn(s.DB, Principal{UserID: owner.UserID, TenantID: s.Instance, AccessGrantHash: access.Hash}, sp.ID, "file.txt", "read")},
					{"range", s.governanceAllow(s.DB, Principal{UserID: root.ID, DataOwner: owner.UserID, TenantID: s.Instance}, sp.ID, "file.txt", "write")},
				}
				for _, check := range checks {
					if (check.err == nil) != live {
						t.Fatalf("%s expiry decision wrong: live=%t err=%v", check.name, live, check.err)
					}
				}
			})
		}
	}
}
