package desktopcontract

import "testing"

func TestDesktopRangeBudgetsAndOverflow(t *testing.T) {
	for _, limit := range []uint64{SourceRangeMaxBytes, AssetRangeMaxBytes} {
		for _, r := range []ByteRange{{0, 0}, {0, limit - 1}, {maxSafeRevision, maxSafeRevision}} {
			header, err := r.Header(limit)
			if err != nil || header == "" {
				t.Fatal("valid range rejected", err)
			}
		}
		for _, r := range []ByteRange{{0, limit}, {2, 1}, {maxSafeRevision + 1, maxSafeRevision + 1}, {0, ^uint64(0)}} {
			if _, err := r.Header(limit); err == nil {
				t.Fatal("invalid range accepted")
			}
		}
	}
	if _, err := (ByteRange{}).Header(0); err == nil {
		t.Fatal("zero budget")
	}
	source := fixtureAs[SourceRangeRequest](t, "SourceRangeRequest")
	if source.Validate() != nil {
		t.Fatal("source fixture")
	}
	source.Range.End++
	if source.Validate() == nil {
		t.Fatal("source over budget")
	}
	asset := fixtureAs[AssetRangeRequest](t, "AssetRangeRequest")
	if asset.Validate() != nil {
		t.Fatal("asset fixture")
	}
	asset.Handle = "../file"
	if asset.Validate() == nil {
		t.Fatal("path handle")
	}
}
func TestDesktopExportRecoveryAndProgress(t *testing.T) {
	req := fixtureAs[ExportRequest](t, "ExportRequest")
	if req.Validate() != nil {
		t.Fatal("export fixture")
	}
	req.NativeSelectionID = "/private/path"
	if req.Validate() == nil {
		t.Fatal("path selection")
	}
	op := fixtureAs[ExportOperation](t, "ExportOperation")
	if op.Validate() != nil {
		t.Fatal("receipt fixture")
	}
	for _, change := range []func(*ExportOperation){func(o *ExportOperation) { o.TotalBytes = nil }, func(o *ExportOperation) { o.BytesTransferred++ }, func(o *ExportOperation) { o.BytesTransferred-- }, func(o *ExportOperation) { o.UIState = "future" }, func(o *ExportOperation) { o.OperationID = "" }} {
		bad := op
		change(&bad)
		if bad.Validate() == nil {
			t.Fatal("invalid operation projection")
		}
	}
	lookup := ExportLookupResult{Found: true, Operation: &op}
	if lookup.Validate() != nil {
		t.Fatal("lookup fixture")
	}
	lookup.Found = false
	if lookup.Validate() == nil {
		t.Fatal("not found leaked result")
	}
	op.UIState = "unknown"
	op.TotalBytes = nil
	if op.Validate() != nil {
		t.Fatal("unknown outcome must remain representable")
	}
	key := fixtureAs[ExportLookup](t, "ExportLookup")
	if key.Validate() != nil {
		t.Fatal("lookup key")
	}
	key.IdempotencyKey = ""
	if key.Validate() == nil {
		t.Fatal("missing original key")
	}
}
