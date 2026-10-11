package service_test

import (
	"errors"
	"sync"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestCollectionMembersAtomicAuthorityAndRollback(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	client := mustSDK(t, h)
	a := orgUpload(t, client, h.space.ID, "members/a.txt", "a")
	b := orgUpload(t, client, h.space.ID, "members/b.txt", "b")
	c, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Atomic", Kind: "manual", Scope: "private"}, "atomic-members")
	if err != nil {
		t.Fatal(err)
	}
	c, err = f.Service.SetCollectionMembers(p, c.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: c.Revision, Include: []string{a.FileID}})
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []service.Principal{{UserID: p.UserID, TenantID: "foreign"}, {UserID: p.UserID, ScopeSpace: "foreign"}} {
		if _, err = f.Service.SetCollectionMembers(bad, c.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: c.Revision, Include: []string{b.FileID}}); err == nil {
			t.Fatal("member replacement bypassed active scope")
		}
	}
	hits := 0
	if err = f.Service.DB.Callback().Query().Before("gorm:query").Register("member_count_fail", func(tx *gorm.DB) {
		if tx.Statement.Table == "collection_members" {
			hits++
			tx.AddError(errors.New("synthetic count failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, failure := f.Service.SetCollectionMembers(p, c.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: c.Revision, Include: []string{b.FileID}})
	f.Service.DB.Callback().Query().Remove("member_count_fail")
	if hits == 0 || failure == nil || failure.Error() != "database_unavailable" {
		t.Fatal("count failure lost", failure)
	}
	view, err := f.Service.ReadFileCollection(p, c.CollectionID)
	if err != nil || view.Revision != c.Revision || len(view.Items) != 1 || view.Items[0].FileID != a.FileID {
		t.Fatal("failed result committed members or revision", err)
	}
	fired := false
	if err = f.Service.DB.Callback().Update().Before("gorm:update").Register("member_cas_drift", func(tx *gorm.DB) {
		if tx.Statement.Table != "file_collections" || fired {
			return
		}
		fired = true
		if err := tx.Session(&gorm.Session{NewDB: true}).Model(&service.FileCollection{}).Where("id = ?", c.CollectionID).Update("revision", c.Revision+1).Error; err != nil {
			tx.AddError(err)
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, failure = f.Service.SetCollectionMembers(p, c.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: c.Revision, Include: []string{b.FileID}})
	f.Service.DB.Callback().Update().Remove("member_cas_drift")
	if !fired || failure == nil || failure.Error() != "collection_revision_conflict" {
		t.Fatal("database CAS lost its revision predicate", failure)
	}
	view, err = f.Service.ReadFileCollection(p, c.CollectionID)
	if err != nil || view.Revision != c.Revision || len(view.Items) != 1 || view.Items[0].FileID != a.FileID {
		t.Fatal("CAS loser changed members", err)
	}
	if err = f.Service.DB.Model(&service.User{}).Where("id = ?", p.UserID).Update("disabled", true).Error; err != nil {
		t.Fatal(err)
	}
	if _, err = f.Service.SetCollectionMembers(p, c.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: c.Revision}); err == nil {
		t.Fatal("disabled actor cleared collection")
	}
}

func TestCollectionMembersConcurrentRevisionSingleWinner(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	client := mustSDK(t, h)
	a := orgUpload(t, client, h.space.ID, "concurrent/a.txt", "a")
	b := orgUpload(t, client, h.space.ID, "concurrent/b.txt", "b")
	c, err := f.Service.CreateFileCollection(p, service.CreateFileCollectionRequest{SpaceID: h.space.ID, Name: "Concurrent", Kind: "manual", Scope: "private"}, "concurrent-members")
	if err != nil {
		t.Fatal(err)
	}
	type result struct {
		id   string
		view service.FileCollectionView
		err  error
	}
	results := make(chan result, 2)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for _, id := range []string{a.FileID, b.FileID} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			v, e := f.Service.SetCollectionMembers(p, c.CollectionID, service.SetCollectionMembersRequest{ExpectedRevision: c.Revision, Include: []string{id}})
			results <- result{id, v, e}
		}()
	}
	close(start)
	wg.Wait()
	close(results)
	winner := ""
	conflicts := 0
	for r := range results {
		if r.err == nil {
			if winner != "" || r.view.Revision != c.Revision+1 {
				t.Fatal("two writes won one revision")
			}
			winner = r.id
		} else {
			if r.err.Error() != "collection_revision_conflict" {
				t.Fatal(r.err)
			}
			conflicts++
		}
	}
	if winner == "" || conflicts != 1 {
		t.Fatal("missing single winner")
	}
	view, err := f.Service.ReadFileCollection(p, c.CollectionID)
	if err != nil || view.Revision != c.Revision+1 || len(view.Items) != 1 || view.Items[0].FileID != winner {
		t.Fatal("loser changed winner membership", err)
	}
}
