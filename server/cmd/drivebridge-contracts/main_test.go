package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"
)

func TestResponseSchemasCommand(t *testing.T) {
	var out bytes.Buffer
	if err := run([]string{"response-schemas"}, &out); err != nil {
		t.Fatal(err)
	}
	var schemas map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &schemas); err != nil {
		t.Fatal(err)
	}
	if len(schemas) != 3 || schemas["GrantView"] == nil {
		t.Fatalf("unexpected schemas: %v", schemas)
	}
}

func TestInvalidCommandDoesNotEmit(t *testing.T) {
	for _, args := range [][]string{nil, {"unknown"}, {"response-schemas", "extra"}} {
		var out bytes.Buffer
		if err := run(args, &out); err == nil || out.Len() != 0 {
			t.Fatalf("args=%v err=%v", args, err)
		}
	}
}

func TestDesktopProfileAndLegacyOutput(t *testing.T) {
	for _, kind := range []string{"openapi", "fixtures", "response-schemas"} {
		var original, explicit, desktop bytes.Buffer
		if err := run([]string{kind}, &original); err != nil {
			t.Fatal(err)
		}
		if err := run([]string{kind, "--profile", "personal-workspace"}, &explicit); err != nil {
			t.Fatal(err)
		}
		if original.String() != explicit.String() {
			t.Fatal("default output changed")
		}
		if err := run([]string{kind, "--profile", "desktop"}, &desktop); err != nil {
			t.Fatal(err)
		}
		if !json.Valid(desktop.Bytes()) || desktop.String() == original.String() {
			t.Fatal("desktop profile missing")
		}
	}
	var out bytes.Buffer
	if err := run([]string{"openapi", "--profile", "unknown"}, &out); err == nil || out.Len() != 0 {
		t.Fatal("invalid profile emitted output")
	}
}

type failedWriter struct{}

func (failedWriter) Write([]byte) (int, error) { return 0, errors.New("write failed") }
func TestOutputFailure(t *testing.T) {
	if err := run([]string{"response-schemas"}, failedWriter{}); err == nil {
		t.Fatal("write failure ignored")
	}
}
