package bridge

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge/internal/recovery"
)

// TestToolErrorResultCarriesRecoveryHint pins the two-block error contract:
// the first text block stays the bare machine code (unchanged wire shape for
// existing hosts); the second block is structured JSON with the shared
func TestToolErrorResultCarriesRecoveryHint(t *testing.T) {
	result := toolErrorResult(errors.New("quota_exceeded"))
	if !result.IsError {
		t.Fatal("quota_exceeded must surface as an MCP tool error")
	}
	encoded, err := json.Marshal(result.Content)
	if err != nil {
		t.Fatal(err)
	}
	var decoded []map[string]string
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	if len(decoded) != 2 {
		t.Fatalf("want code + hint blocks, got %d: %s", len(decoded), encoded)
	}
	if decoded[0]["text"] != "quota_exceeded" {
		t.Fatalf("first block must stay the bare code, got %q", decoded[0]["text"])
	}
	if !strings.Contains(decoded[1]["text"], `"code":"quota_exceeded"`) ||
		!strings.Contains(decoded[1]["text"], `"next_action"`) ||
		!strings.Contains(decoded[1]["text"], recovery.Hint("quota_exceeded")) {
		t.Fatalf("hint block malformed: %s", decoded[1]["text"])
	}
}

// TestUnknownCodeFallsBackToGenericHint pins the fallback path through the
// shared registry.
func TestUnknownCodeFallsBackToGenericHint(t *testing.T) {
	result := toolErrorResult(errors.New("totally_unknown_code"))
	encoded, err := json.Marshal(result.Content)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(encoded), "transfer_failed") {
		t.Fatalf("unknown codes flatten to transfer_failed, got %s", encoded)
	}
	if !strings.Contains(string(encoded), recovery.Hint("transfer_failed")) {
		t.Fatalf("fallback hint missing: %s", encoded)
	}
}

// TestToolAnnotationsPinHints pins the read/destructive/idempotent annotation
// set hosts use for safety heuristics.
func TestToolAnnotationsPinHints(t *testing.T) {
	s := &Service{}
	encoded, err := json.Marshal(s.Tools())
	if err != nil {
		t.Fatal(err)
	}
	var decoded []struct {
		Name        string `json:"name"`
		Annotations *struct {
			ReadOnlyHint    *bool `json:"readOnlyHint,omitempty"`
			DestructiveHint *bool `json:"destructiveHint,omitempty"`
			IdempotentHint  *bool `json:"idempotentHint,omitempty"`
		} `json:"annotations"`
	}
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatal(err)
	}
	hints := map[string]*struct {
		readOnly, destructive, idempotent *bool
	}{}
	for _, entry := range decoded {
		h := &struct {
			readOnly, destructive, idempotent *bool
		}{}
		if entry.Annotations != nil {
			h.readOnly = entry.Annotations.ReadOnlyHint
			h.destructive = entry.Annotations.DestructiveHint
			h.idempotent = entry.Annotations.IdempotentHint
		}
		hints[entry.Name] = h
	}
	assertTrue := func(name, field string, value *bool) {
		t.Helper()
		if value == nil || !*value {
			t.Fatalf("%s must set %s=true", name, field)
		}
	}
	assertTrue("stat", "readOnlyHint", hints["stat"].readOnly)
	assertTrue("operation_get", "readOnlyHint", hints["operation_get"].readOnly)
	assertTrue("audit_events", "readOnlyHint", hints["audit_events"].readOnly)
	assertTrue("trash", "destructiveHint", hints["trash"].destructive)
	assertTrue("cancel", "destructiveHint", hints["cancel"].destructive)
	assertTrue("mkdir", "idempotentHint", hints["mkdir"].idempotent)
	if hints["upload"].readOnly != nil && *hints["upload"].readOnly {
		t.Fatal("upload is a mutation and must not claim readOnlyHint")
	}
}
