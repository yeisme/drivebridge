package service_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/yeisme/drivebridge-server/sdk"
)

func TestFileQueryLiteralPathSearch(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	bob, _ := login(t, f, "bob")
	c := mustSDK(t, h)
	paths := []string{"search/report-one.md", "search/report-two.txt", "search/unrelated.txt", "literal/100%_done.txt", "literal/100XXdone.txt", "percent%/child.txt", "percentXX/child.txt", "中文/笔记.txt", "literal/a!b.txt"}
	for _, path := range paths {
		orgUpload(t, c, h.space.ID, path, path)
	}
	query := func(op, value string, limit int) sdk.FileQueryRequest {
		return sdk.FileQueryRequest{Mode: "exact", SpaceIDs: []string{h.space.ID}, Filters: []sdk.OrgFilter{{Field: "path_prefix", Op: op, Value: value}}, Limit: limit}
	}
	q := query("contains", "report-", 1)
	first, err := c.QueryFiles(context.Background(), q)
	if err != nil {
		t.Fatal(err)
	}
	if len(first.Items) != 1 || first.NextCursor == "" {
		t.Fatal("missing literal search page")
	}
	q.Cursor = first.NextCursor
	second, err := c.QueryFiles(context.Background(), q)
	if err != nil || len(second.Items) != 1 || second.Items[0].FileID == first.Items[0].FileID || second.NextCursor != "" {
		t.Fatal("cursor lost distinct search result", err)
	}
	q.Filters[0].Value = "unrelated"
	if _, err = c.QueryFiles(context.Background(), q); err == nil {
		t.Fatal("changed literal accepted old cursor")
	}
	for _, test := range []struct{ op, value, path string }{
		{"contains", "100%_", "literal/100%_done.txt"}, {"prefix", "percent%/", "percent%/child.txt"}, {"contains", "笔记", "中文/笔记.txt"}, {"contains", "a!b", "literal/a!b.txt"},
	} {
		view, err := c.QueryFiles(context.Background(), query(test.op, test.value, 50))
		if err != nil || len(view.Items) != 1 || view.Items[0].Path != test.path {
			t.Fatalf("literal %q mismatch: %v %v", test.value, view.Items, err)
		}
	}
	var foreign sdk.FileQueryView
	if status := bob.call(t, "POST", "/file-queries", query("contains", "report", 50), &foreign); status != 200 || len(foreign.Items) != 0 {
		t.Fatal("cross-owner search leaked")
	}
	// Saved exact conditions round-trip through the existing owner contract.
	var saved struct {
		QueryID string          `json:"query_id"`
		Query   json.RawMessage `json:"query"`
	}
	if status := h.call(t, "POST", "/saved-searches", map[string]any{"name": "Literal report query", "query": query("contains", "report", 50)}, &saved); status != 200 || saved.QueryID == "" {
		t.Fatal("save exact query failed", status)
	}
	var restored sdk.FileQueryRequest
	if err = json.Unmarshal(saved.Query, &restored); err != nil {
		t.Fatal(err)
	}
	view, err := c.QueryFiles(context.Background(), restored)
	if err != nil || len(view.Items) != 2 {
		t.Fatal("saved query changed semantics", err)
	}
	for _, filter := range []sdk.OrgFilter{{Field: "path_prefix", Op: "eq", Value: "x"}, {Field: "path_prefix", Op: "contains", Value: 12}, {Field: "path_prefix", Op: "contains", Value: ""}, {Field: "path_prefix", Op: "contains", Value: "x\x00y"}, {Field: "prefix", Op: "contains", Value: "report"}} {
		q := query("contains", "report", 50)
		q.Filters = []sdk.OrgFilter{filter}
		if _, err := c.QueryFiles(context.Background(), q); err == nil || err.Error() != "invalid_query" {
			t.Fatal("invalid path predicate ignored", err)
		}
	}
}
