package service

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestMetadataWirePreservesPrivateIdentityAndLargeRevision(t *testing.T) {
	original := []Operation{{ID: "operation", Owner: "owner", Key: "old\x00key", KeyEncodingVersion: 1, Fingerprint: "original", Size: 9007199254740993, Status: "completed"}}
	wire, err := metadataWireRows(original)
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(wire)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []Operation
	if err := decodeMetadataRows(raw, &decoded); err != nil || !reflect.DeepEqual(original, decoded) {
		t.Fatal("metadata identity changed")
	}
	before := metadataFingerprints(&metadataSnapshot{Operations: original})["operations"]
	decoded[0].Owner = "other"
	after := metadataFingerprints(&metadataSnapshot{Operations: decoded})["operations"]
	if before == after {
		t.Fatal("private ownership absent from verification digest")
	}
	for _, broken := range []string{`[{"id":"operation"}]`, `[{"id":"operation","_storage_version":2,"_storage":{}}]`, `[{"id":"operation","_storage_version":1,"_storage":{}}]`} {
		if err := decodeMetadataRows(json.RawMessage(broken), &decoded); err == nil {
			t.Fatal("incomplete snapshot admitted")
		}
	}
}
