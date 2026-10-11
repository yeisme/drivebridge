package contract

import (
	"bytes"
	"encoding/json"
	"testing"
)

func TestCollectionCreatePreservesWireCompatibility(t *testing.T) {
	raw := []byte(`{"space_id":"s","name":"Manual","kind":"manual","scope":"private","rule":null}`)
	input, err := DecodeRequest[CreateFileCollectionRequest](bytes.NewReader(raw))
	if err != nil || string(input.Rule) != "null" {
		t.Fatal("legacy manual null changed", err)
	}
	// The HTTP adapter validates flat shape then decodes original bytes again.
	rule := `{ "mode" : "exact", "filters" : [] }`
	raw = []byte(`{"space_id":"s","name":"Rule","kind":"rule","scope":"private","rule":` + rule + `}`)
	if _, err = DecodeRequest[CreateFileCollectionRequest](bytes.NewReader(raw)); err != nil {
		t.Fatal(err)
	}
	if err = json.Unmarshal(raw, &input); err != nil || string(input.Rule) != rule {
		t.Fatal("original nested bytes changed", err)
	}
	op := RegisteredPaths()["/api/v1/file-collections"].(map[string]any)["post"].(map[string]any)
	if op["x-human-only"] != false || op["x-csrf-required-for-session"] != true {
		t.Fatal("legacy Agent or session policy changed")
	}
	if op["responses"].(map[string]any)["200"] == nil {
		t.Fatal("create success status changed")
	}
	if op["responses"].(map[string]any)["201"] != nil {
		t.Fatal("invented creation status")
	}
	for _, route := range Routes() {
		if route.OperationID == "createFileCollection" && route.ResponseSchema != "FileCollectionViewEnvelope" {
			t.Fatal("current view mislabeled as original receipt")
		}
	}
}
