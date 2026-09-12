package bridge

import (
	"context"
	"net/url"
	"os"
	"path/filepath"
	"testing"

	"github.com/rclone/rclone/fs/config"
	"github.com/yeisme/drivebridge-server/sdk/insights"
)

func TestLocalUsageAndBoundedDirectoryObservation(t *testing.T) {
	state, local, remote := t.TempDir(), t.TempDir(), t.TempDir()
	os.Chmod(state, 0700)
	const name = "insights-fixture"
	config.Data().SetValue(name, "type", "alias")
	config.Data().SetValue(name, "remote", remote)
	defer config.Data().DeleteSection(name)
	s, e := New(state, fake(), true)
	if e != nil {
		t.Fatal(e)
	}
	defer s.Close()
	if e = s.AddSpace(Space{ID: "scope", Remote: name, Backend: "generic", LocalRoot: local, MaxBytes: 1024}); e != nil {
		t.Fatal(e)
	}
	os.WriteFile(filepath.Join(local, "file.txt"), []byte("payload"), 0600)
	op, e := s.Upload(context.Background(), "scope", "", "file.txt", "file.txt", "key", Provenance{Caller: "declared-agent"})
	if e != nil {
		t.Fatal(e)
	}
	op = waitOp(t, s, op.ID)
	if op.Status != "completed" {
		t.Fatal(op.Error)
	}
	raw, e := s.Usage("summary", url.Values{"space_id": {"scope"}})
	if e != nil {
		t.Fatal(e)
	}
	summary := raw.(insights.Summary)
	if summary.Totals.Effective != 7 || summary.Totals.Observed != 14 || len(summary.Capacities) != 1 || summary.Capacities[0].Quota != nil || summary.Capacities[0].History != nil {
		t.Fatalf("local metric semantics invalid: %+v", summary)
	}
	// Inventory must include externally written and oversized files hidden by List.
	os.WriteFile(filepath.Join(remote, "outside-upload-limit.bin"), make([]byte, 2048), 0600)
	s.ConfigureScan("scope", true, 100, 5)
	scan, e := s.Scan(context.Background(), "scope", "")
	if e != nil || scan.Status != "complete" || scan.Bytes != 2055 {
		t.Fatalf("observation incomplete: %+v %v", scan, e)
	}
	s.ConfigureScan("scope", true, 1, 5)
	partial, e := s.Scan(context.Background(), "scope", "")
	if e != nil || partial.Status != "partial" {
		t.Fatal("scan budget ignored")
	}
	raw, e = s.Usage("summary", url.Values{"space_id": {"scope"}})
	if e != nil {
		t.Fatal(e)
	}
	summary = raw.(insights.Summary)
	if summary.ObservedTotal == nil || *summary.ObservedTotal != 2055 {
		t.Fatal("partial scan replaced last complete")
	}
}
