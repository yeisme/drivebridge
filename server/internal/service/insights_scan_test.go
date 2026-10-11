package service

import (
	"context"
	"testing"

	"github.com/yeisme/drivebridge-server/sdk/insights"
)

// scriptedInventory answers each (prefix, cursor) call from a scripted table
// and records the call order.
type scriptedInventory struct {
	responses map[string]insights.InventoryPage
	calls     [][2]string
}

func (f *scriptedInventory) Inventory(_ context.Context, prefix, cursor string) (insights.InventoryPage, error) {
	f.calls = append(f.calls, [2]string{prefix, cursor})
	page, ok := f.responses[prefix+"|"+cursor]
	if !ok {
		return insights.InventoryPage{}, code("inventory_unavailable")
	}
	return page, nil
}

func TestNamespaceInventoryWalksAndResumes(t *testing.T) {
	a, b := "inst/owner/space/", "inst/blobs/"
	fresh := &scriptedInventory{responses: map[string]insights.InventoryPage{
		a + "|":   {Entries: 2, Bytes: 10, Next: "t1", Resumable: true},
		a + "|t1": {Entries: 1, Bytes: 5},
		b + "|":   {Entries: 3, Bytes: 30, Next: "u1", Resumable: true},
		b + "|u1": {Entries: 1, Bytes: 7},
	}}
	fetch := namespaceInventory([]string{a, b}, fresh)

	want := [][2]string{{a, ""}, {a, "t1"}, {b, ""}, {b, "u1"}}
	cursors := []string{"", "t1", "ns:1", "nsc:1:u1"}
	for i, cursor := range cursors {
		page, e := fetch(context.Background(), cursor)
		if e != nil {
			t.Fatalf("cursor %q: %v", cursor, e)
		}
		var next string
		if i+1 < len(cursors) {
			next = cursors[i+1]
		}
		if page.Next != next {
			t.Fatalf("cursor %q: next = %q, want %q", cursor, page.Next, next)
		}
	}
	if len(fresh.calls) != len(want) {
		t.Fatalf("calls = %v, want %v", fresh.calls, want)
	}
	for i, call := range fresh.calls {
		if call != want[i] {
			t.Fatalf("call %d = %v, want %v", i, call, want[i])
		}
	}
}

func TestNamespaceInventoryResumeSkipsEarlierNamespaces(t *testing.T) {
	a, b := "inst/owner/space/", "inst/blobs/"
	scanner := &scriptedInventory{responses: map[string]insights.InventoryPage{
		b + "|":   {Entries: 3, Next: "u1", Resumable: true},
		b + "|u1": {Entries: 1},
	}}
	fetch := namespaceInventory([]string{a, b}, scanner)

	// Resuming at a namespace boundary must not restart namespace 0 (which
	// used to re-emit the boundary cursor and stall the run) and must not
	// apply namespace-1 tokens to namespace 0.
	page, e := fetch(context.Background(), "ns:1")
	if e != nil || page.Next != "nsc:1:u1" {
		t.Fatalf("boundary resume: page=%+v err=%v", page, e)
	}
	page, e = fetch(context.Background(), "nsc:1:u1")
	if e != nil || page.Next != "" {
		t.Fatalf("chained resume: page=%+v err=%v", page, e)
	}
	for _, call := range scanner.calls {
		if call[0] != b {
			t.Fatalf("namespace 0 re-scanned on resume: %v", scanner.calls)
		}
	}
}

func TestNamespaceInventoryRejectsInvalidCursors(t *testing.T) {
	fetch := namespaceInventory([]string{"a/", "b/"}, &scriptedInventory{})
	for _, cursor := range []string{"ns:x", "ns:2", "nsc:bad:token", "nsc:"} {
		if _, e := fetch(context.Background(), cursor); safeCode(e) != "invalid_inventory" {
			t.Fatalf("cursor %q: err=%v, want invalid_inventory", cursor, e)
		}
	}
}
