package desktopcontract

import (
	"strings"
	"time"

	"github.com/invopop/jsonschema"
)

func exportSchemas() map[string]*jsonschema.Schema {
	r := jsonschema.Reflector{Anonymous: true, DoNotReference: true}
	operation := r.Reflect(ExportOperation{})
	operation.If = condition("ui_state", "completed")
	operation.Then = &jsonschema.Schema{Not: condition("total_bytes", nil)}
	lookup := lookupSchema(r.Reflect(ExportLookupResult{}), "operation")
	lookup.Properties.Set("operation", &jsonschema.Schema{OneOf: []*jsonschema.Schema{operation, {Type: "null"}}})
	return map[string]*jsonschema.Schema{
		"ExportRequest": r.Reflect(ExportRequest{}), "ExportLookup": r.Reflect(ExportLookup{}), "ExportOperation": operation, "ExportLookupResult": lookup,
		"ByteRange": r.Reflect(ByteRange{}), "SourceRangeRequest": r.Reflect(SourceRangeRequest{}), "AssetRangeRequest": r.Reflect(AssetRangeRequest{}),
	}
}
func exportFixtures() map[string]any {
	revision := "sha256:" + strings.Repeat("a", 64)
	ref := ReadingRef{Kind: "pinax", InstanceID: "instance-fixture", SourceID: "source-fixture", ObjectID: "note_fixture", Revision: revision, URI: "pinax://note/note_fixture?v=" + revision}
	total := uint64(42)
	op := ExportOperation{OperationID: "operation-fixture", TargetRef: ref, OwnerStatus: "completed", UIState: "completed", BytesTransferred: total, TotalBytes: &total, ObservedAt: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)}
	return map[string]any{
		"ExportRequest": ExportRequest{TargetRef: ref, NativeSelectionID: "selection-fixture", ExpectedDestinationRevision: "absent"},
		"ExportLookup":  ExportLookup{IdempotencyKey: "original-fixture"}, "ExportOperation": op, "ExportLookupResult": ExportLookupResult{},
		"ByteRange":          ByteRange{Start: 0, End: 41},
		"SourceRangeRequest": SourceRangeRequest{TargetRef: PinaxRef{Kind: ref.Kind, InstanceID: ref.InstanceID, SourceID: ref.SourceID, ObjectID: ref.ObjectID, Revision: ref.Revision, URI: ref.URI}, Range: ByteRange{End: SourceRangeMaxBytes - 1}},
		"AssetRangeRequest":  AssetRangeRequest{Handle: "asset-fixture", Range: ByteRange{End: AssetRangeMaxBytes - 1}},
	}
}
