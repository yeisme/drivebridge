package insights

import (
	"context"
	"errors"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
	"path/filepath"
	"testing"
	"time"
)

func rig(t *testing.T) *Store {
	t.Helper()
	db, e := gorm.Open(sqlite.Open(filepath.Join(t.TempDir(), "state.sqlite")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if e != nil {
		t.Fatal(e)
	}
	s, e := Open(db)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close(); raw, _ := db.DB(); raw.Close() })
	return s
}
func TestIdempotentUsageAndScopedQueries(t *testing.T) {
	s := rig(t)
	d := Dimensions{Owner: "alice", Space: "project", Connection: "storage", Actor: "agent1", ActorKind: "authenticated_agent", Operation: "op1", Resource: "file1"}
	f := Fact{ID: "completed-op1", Dimensions: d, Metric: "effective_bytes", Direction: "ingest", Purpose: "upload", Bytes: 1024}
	for i := 0; i < 3; i++ {
		if e := s.Record(f); e != nil {
			t.Fatal(e)
		}
	}
	for _, purpose := range []string{"transfer", "verification", "transfer"} {
		if e := s.Record(Fact{Dimensions: d, Metric: "observed_bytes", Direction: "server_to_storage", Purpose: purpose, Bytes: 1024}); e != nil {
			t.Fatal(e)
		}
	}
	out, e := s.Summary(Scope{Owner: "alice"}, Filter{})
	if e != nil || out.Totals.Effective != 1024 || out.Totals.Observed != 3072 || out.Totals.Operations != 1 {
		t.Fatalf("wrong usage: %+v %v", out.Totals, e)
	}
	other, e := s.Summary(Scope{Owner: "bob"}, Filter{})
	if e != nil || other.Totals.Observed != 0 {
		t.Fatal("cross-user usage leaked")
	}
	scoped, e := s.Summary(Scope{Owner: "alice", Actor: "agent1", RestrictOperations: true, OperationIDs: []string{}}, Filter{})
	if e != nil || scoped.Totals.Observed != 0 {
		t.Fatal("denied scope leaked")
	}
	scoped, e = s.Summary(Scope{Owner: "alice", Actor: "agent1", RestrictOperations: true, ResourceIDs: []string{"file1"}}, Filter{})
	if e != nil || scoped.Totals.Observed != 3072 {
		t.Fatal("authorized resource facts missing")
	}
	if _, e := s.Series(Scope{Owner: "alice"}, Filter{From: "bad"}); e == nil {
		t.Fatal("invalid range accepted")
	}
}
func TestPartialScanPreservesCompleteAndResumesByPage(t *testing.T) {
	s := rig(t)
	_, e := s.Configure("alice", "space", "store", true, 10, 60)
	if e != nil {
		t.Fatal(e)
	}
	full, e := s.Scan(context.Background(), "alice", "space", func(context.Context, string) (InventoryPage, error) {
		return InventoryPage{Entries: 2, Bytes: 100, Resumable: true}, nil
	}, "")
	if e != nil || full.Status != "complete" {
		t.Fatal(e)
	}
	s.Configure("alice", "space", "store", true, 2, 60)
	fetch := func(_ context.Context, cursor string) (InventoryPage, error) {
		if cursor == "" {
			return InventoryPage{Entries: 2, Bytes: 50, Next: "next", Resumable: true}, nil
		}
		return InventoryPage{Entries: 2, Bytes: 70, Resumable: true}, nil
	}
	partial, e := s.Scan(context.Background(), "alice", "space", fetch, "")
	if e != nil || partial.Status != "partial" || partial.Bytes != 50 || !partial.Resumable {
		t.Fatalf("bad partial: %+v %v", partial, e)
	}
	summary, e := s.Summary(Scope{Owner: "alice"}, Filter{})
	if e != nil || summary.ObservedTotal == nil || *summary.ObservedTotal != 100 {
		t.Fatal("partial scan replaced complete snapshot")
	}
	resumed, e := s.Scan(context.Background(), "alice", "space", fetch, partial.ID)
	if e != nil || resumed.Status != "complete" || resumed.Bytes != 120 || resumed.Entries != 4 {
		t.Fatalf("resume duplicated page: %+v %v", resumed, e)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	run, e := s.Scan(ctx, "alice", "space", fetch, "")
	if e != nil || run.Status != "cancelled" {
		t.Fatal("cancellation not preserved")
	}
	s.Configure("alice", "other", "same-store", true, 10, 60)
	s.Scan(context.Background(), "alice", "other", func(context.Context, string) (InventoryPage, error) { return InventoryPage{Entries: 1, Bytes: 20}, nil }, "")
	summary, _ = s.Summary(Scope{Owner: "alice"}, Filter{})
	if summary.ObservedTotal != nil {
		t.Fatal("potentially overlapping scopes summed")
	}
}
func TestAuditFailureGapAndRetention(t *testing.T) {
	s := rig(t)
	failure := true
	s.DB.Callback().Create().Before("gorm:create").Register("fail-audit", func(tx *gorm.DB) {
		if failure && tx.Statement.Table == "insight_audit" {
			tx.AddError(errors.New("fixture unavailable"))
		}
	})
	if e := s.Log(AuditEvent{Owner: "alice", ActorKind: "user", Action: "agents.create", Outcome: "intent", Entry: "test"}); !errors.Is(e, ErrAudit) {
		t.Fatal("audit failure ignored")
	}
	failure = false
	if e := s.Log(AuditEvent{Owner: "alice", ActorKind: "user", Action: "agents.revoke", Outcome: "success", Entry: "test"}); e != nil {
		t.Fatal(e)
	}
	page, e := s.Audit(Scope{Owner: "alice"}, AuditFilter{})
	if e != nil || len(page.Events) != 2 {
		t.Fatalf("gap not represented: %v %+v", e, page)
	}
	future := s.Now().AddDate(0, 0, 100)
	old := Fact{ID: "old", Dimensions: Dimensions{Owner: "alice"}, At: s.Now(), Metric: "observed_bytes", Bytes: 7}
	if e := s.Record(old); e != nil {
		t.Fatal(e)
	}
	s.Now = func() time.Time { return future }
	if e := s.Prune(); e != nil {
		t.Fatal(e)
	}
	var n int64
	s.DB.Model(&Fact{}).Count(&n)
	if n != 0 {
		t.Fatal("expired facts retained")
	}
	s.DB.Model(&Daily{}).Count(&n)
	if n == 0 {
		t.Fatal("yearly aggregates removed")
	}
}
