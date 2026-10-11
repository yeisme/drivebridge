package service_test

// Rework regression lights (ReworkVerify batch 2). Each test pins the
// verdict's stated behavior with the real surface contract; run them
// red first, then fix, then green.

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

// F1: every closed-vocabulary analytics path must answer 200 — dimensions,
// metrics and filters all map to real physical columns.
func TestFPReworkAnalyticsVocabularyColumns(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	for i := range 3 {
		if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: fmt.Sprintf("vocab/f%d.bin", i)}, "vocab-"+fmt.Sprint(i)+time.Now().Format("150405.000000000"), sizeReader(100+i), nil); e != nil {
			t.Fatal(e)
		}
	}
	paths := []sdk.AnalyticsQueryRequest{
		{Dataset: "file_current", Dimensions: []string{"size_bytes"}, Metrics: []string{"file_count"}, Consistency: "live"},
		{Dataset: "file_current", Dimensions: []string{"updated_at"}, Metrics: []string{"file_count"}, Consistency: "live"},
		{Dataset: "transfer_facts", Dimensions: []string{"created_at"}, Metrics: []string{"transferred_bytes"}, Consistency: "live"},
		{Dataset: "audit_facts", Dimensions: []string{"created_at"}, Metrics: []string{"event_count"}, Consistency: "live"},
		{Dataset: "transfer_facts", Filters: []sdk.AnalyticsFilter{{Field: "bytes_transferred", Op: "gte", Value: 0}}, Metrics: []string{"transfer_count"}, Consistency: "live"},
	}
	for _, q := range paths {
		view, e := c.QueryAnalytics(context.Background(), q)
		if e != nil {
			t.Fatalf("vocabulary path %+v must answer, got %v", q, e)
		}
		// Vocabulary names ride the rows: clients read the documented keys.
		if len(view.Rows) > 0 {
			row := view.Rows[0]
			switch q.Dataset {
			case "file_current":
				if q.Dimensions[0] == "size_bytes" {
					if _, ok := row["size_bytes"]; !ok {
						t.Fatalf("size_bytes dimension key missing: %v", row)
					}
				}
			case "transfer_facts":
				if len(q.Dimensions) > 0 && q.Dimensions[0] == "created_at" {
					if _, ok := row["created_at"]; !ok {
						t.Fatalf("created_at dimension key missing: %v", row)
					}
					if _, ok := row["transferred_bytes"]; !ok {
						t.Fatalf("transferred_bytes metric key missing: %v", row)
					}
				}
			case "audit_facts":
				if _, ok := row["created_at"]; !ok {
					t.Fatalf("audit created_at dimension key missing: %v", row)
				}
			}
		}
	}
}

func TestFPReworkAnalyticsPaginationStable(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	paths := map[string]bool{"paged/a.txt": false, "paged/b.txt": false, "paged/c.txt": false, "paged/d.txt": false, "paged/e.txt": false}
	i := 0
	for p := range paths {
		if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: p}, "paged-"+fmt.Sprint(i)+time.Now().Format("150405.000000000"), sizeReader(10), nil); e != nil {
			t.Fatal(e)
		}
		i++
	}
	seen := map[string]bool{}
	order := []string{}
	cursor := ""
	pages := 0
	for {
		q := sdk.AnalyticsQueryRequest{Dataset: "file_current", Dimensions: []string{"path"}, Metrics: []string{"file_count"}, Limit: 2, Consistency: "live", Cursor: cursor}
		view, e := c.QueryAnalytics(context.Background(), q)
		if e != nil {
			t.Fatal(e)
		}
		for _, row := range view.Rows {
			key, _ := row["path"].(string)
			if key == "" {
				t.Fatalf("row missing path: %v", row)
			}
			if seen[key] {
				t.Fatalf("group %s repeated across pages", key)
			}
			seen[key] = true
			order = append(order, key)
		}
		pages++
		if view.NextCursor == "" {
			break
		}
		cursor = view.NextCursor
		if pages > 10 {
			t.Fatal("pagination did not terminate")
		}
	}
	if len(seen) != len(paths) {
		t.Fatalf("pagination lost groups: got %d want %d", len(seen), len(paths))
	}
	for i := 1; i < len(order); i++ {
		if order[i-1] > order[i] {
			t.Fatalf("page order unstable: %v", order)
		}
	}
}

// F2: an agent's exact files query never leaves its bound space and prefix,
// and every returned row passes the current authorization.
func TestFPReworkFilesQueryAgentPrefixBoundary(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	owner, oc := agent(t, h, "", "read,write")
	_ = owner
	ctx := context.Background()
	if _, e := oc.Upload(ctx, sdk.UploadRequest{SpaceID: h.space.ID, Path: "in/visible.txt"}, "fq-in", sizeReader(4), nil); e != nil {
		t.Fatal(e)
	}
	if _, e := oc.Upload(ctx, sdk.UploadRequest{SpaceID: h.space.ID, Path: "out/hidden.txt"}, "fq-out", sizeReader(4), nil); e != nil {
		t.Fatal(e)
	}
	_, scoped := agent(t, h, "in", "read")
	view, e := scoped.QueryFiles(ctx, sdk.FileQueryRequest{Mode: "exact", SpaceIDs: []string{h.space.ID}, Limit: 200})
	if e != nil {
		t.Fatal(e)
	}
	for _, it := range view.Items {
		if it.Path == "out/hidden.txt" {
			t.Fatalf("prefix-scoped agent must not see outside its prefix: %+v", view.Items)
		}
	}
	if len(view.Items) != 1 || view.Items[0].Path != "in/visible.txt" {
		t.Fatalf("scoped query must return exactly the in-prefix file: %+v", view.Items)
	}
	// A scoped agent naming a foreign space gets nothing, not a leak.
	view, e = scoped.QueryFiles(ctx, sdk.FileQueryRequest{Mode: "exact", SpaceIDs: []string{"no-such-space"}, Limit: 200})
	if e != nil || len(view.Items) != 0 {
		t.Fatalf("foreign space query must stay empty: %+v %v", view, e)
	}
}

// F5: agents read changes inside their authorized scope only: their own
// space under their prefix, never the unfiltered owner stream.
func TestFPReworkChangesAgentAuthorization(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	owner, oc := agent(t, h, "", "read,write")
	_ = owner
	ctx := context.Background()
	if _, e := oc.Upload(ctx, sdk.UploadRequest{SpaceID: h.space.ID, Path: "in/a.txt"}, "chg-in", sizeReader(4), nil); e != nil {
		t.Fatal(e)
	}
	if _, e := oc.Upload(ctx, sdk.UploadRequest{SpaceID: h.space.ID, Path: "out/b.txt"}, "chg-out", sizeReader(4), nil); e != nil {
		t.Fatal(e)
	}
	_, scoped := agent(t, h, "in", "read")
	// The agent's own space is readable (previously the empty-prefix allow
	// check refused every prefixed agent).
	view, e := scoped.ReadChanges(ctx, h.space.ID, "", 100)
	if e != nil {
		t.Fatalf("agent reading its own space changes must pass, got %v", e)
	}
	for _, ev := range view.Events {
		if ev.ResourceRef == "" {
			t.Fatalf("event missing resource ref: %+v", ev)
		}
	}
	// Omitting the space never widens an agent to the owner-wide stream.
	if _, e := scoped.ReadChanges(ctx, "", "", 100); e == nil || e.Error() != "forbidden" {
		t.Fatalf("agent without space_id must be forbidden, got %v", e)
	}
	// The delivered stream stays inside the agent's prefix: the out/ file
	// event must not flow to the in/-scoped agent.
	scopedView, e := scoped.ReadChanges(ctx, h.space.ID, "", 100)
	if e != nil {
		t.Fatal(e)
	}
	if len(scopedView.Events) == 0 {
		t.Fatal("scoped agent must receive its own prefix events")
	}
	var out service.File
	if f.Service.DB.First(&out, "owner = ? AND path = ?", h.user.ID, "out/b.txt").Error != nil {
		t.Fatal("out file missing")
	}
	for _, ev := range scopedView.Events {
		if ev.ResourceRef == "drivebridge://"+f.Service.Instance+"/file/"+out.ID {
			t.Fatalf("out-of-prefix event leaked to scoped agent: %+v", scopedView.Events)
		}
	}
}

// F3: a rule collection evaluates its whole result set across pages — a
// rule matching more than one page of files still returns every member.
func TestFPReworkRuleCollectionFullEvaluation(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	now := time.Now().UTC()
	for i := range 205 {
		file := service.File{ID: service.ID(), Owner: h.user.ID, SpaceID: h.space.ID, Path: fmt.Sprintf("bulk/f%03d.txt", i), VersionID: service.ID(), Size: 10, Updated: now.Add(time.Duration(i) * time.Millisecond)}
		if e := f.Service.DB.Create(&file).Error; e != nil {
			t.Fatal(e)
		}
	}
	c := mustSDK(t, h)
	coll, e := c.CreateFileCollection(context.Background(), h.space.ID, "bulk-rule", "rule", "private", `{"mode":"exact"}`, "rework-bulk-rule-key")
	if e != nil {
		t.Fatal(e)
	}
	view, e := c.ReadFileCollection(context.Background(), coll.CollectionID)
	if e != nil {
		t.Fatal(e)
	}
	if len(view.Items) != 205 {
		t.Fatalf("rule evaluation truncated: got %d items, coverage %s", len(view.Items), view.Coverage)
	}
	if view.Coverage != "complete" {
		t.Fatalf("full evaluation must report complete, got %s", view.Coverage)
	}
}

// F6: non-matching events advance the rule checkpoint — a long run of
// out-of-scope events cannot starve the first matching event forever.
func TestFPReworkAutomationCheckpointStarvation(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	now := time.Now().UTC()
	var lastSeq int64 = 0
	for i := range 205 {
		file := service.File{ID: service.ID(), Owner: h.user.ID, SpaceID: h.space.ID, Path: fmt.Sprintf("noise/n%03d.txt", i), VersionID: service.ID(), Size: 5, Updated: now}
		if e := f.Service.DB.Create(&file).Error; e != nil {
			t.Fatal(e)
		}
		lastSeq++
		event := service.OutboxEvent{EventID: service.ID(), Kind: "file_catalogue.upsert", ResourceID: file.ID, ResourceRevision: now.UnixNano(), SourceSequence: lastSeq, OwnerScope: h.user.ID}
		if e := f.Service.DB.Create(&event).Error; e != nil {
			t.Fatal(e)
		}
	}
	target := service.File{ID: service.ID(), Owner: h.user.ID, SpaceID: h.space.ID, Path: "target/t.txt", VersionID: service.ID(), Size: 5, Updated: now}
	if e := f.Service.DB.Create(&target).Error; e != nil {
		t.Fatal(e)
	}
	lastSeq++
	targetEvent := service.OutboxEvent{EventID: service.ID(), Kind: "file_catalogue.upsert", ResourceID: target.ID, ResourceRevision: now.UnixNano(), SourceSequence: lastSeq, OwnerScope: h.user.ID}
	if e := f.Service.DB.Create(&targetEvent).Error; e != nil {
		t.Fatal(e)
	}
	var rules []sdk.AutomationRule
	if status := h.call(t, "PUT", "/spaces/"+h.space.ID+"/automation-rules", map[string]any{"action": "add_label", "action_detail": "rule:auto", "prefix": "target", "enabled": true}, &rules); status != 200 || len(rules) != 1 {
		t.Fatalf("rule create status %d rules %d", status, len(rules))
	}
	c := mustSDK(t, h)
	var intents []sdk.AutomationIntent
	for range 5 {
		var e error
		intents, e = c.RunAutomationRule(context.Background(), rules[0].ID)
		if e != nil {
			t.Fatal(e)
		}
		if len(intents) > 0 {
			break
		}
	}
	if len(intents) != 1 || intents[0].FileID != target.ID {
		t.Fatalf("matching event starved behind non-matching window: %+v", intents)
	}
	var rule service.AutomationRule
	if f.Service.DB.First(&rule, "id = ?", rules[0].ID).Error != nil {
		t.Fatal("rule load failed")
	}
	if rule.Checkpoint < targetEvent.SourceSequence {
		t.Fatalf("checkpoint must advance past consumed non-matching events: %d < %d", rule.Checkpoint, targetEvent.SourceSequence)
	}
}
