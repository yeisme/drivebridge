package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResourceEventsParameterErrorUsesNDJSONFrames(t *testing.T) {
	state := filepath.Join(t.TempDir(), "absent")
	var runErr error
	out, _ := capture(t, func() {
		root := newRootCommand(&cli{})
		root.SetArgs([]string{"connection", "get", "--state", state, "--events"})
		runErr = root.Execute()
	})
	var inputErr resourceInputError
	if !errors.As(runErr, &inputErr) || inputErr.code != "missing_argument" {
		t.Fatalf("expected missing_argument resource input error, got %v", runErr)
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected exactly two NDJSON lines, got %d: %q", len(lines), out)
	}
	var singleEnvelope map[string]any
	if err := json.Unmarshal([]byte(strings.TrimSpace(out)), &singleEnvelope); err == nil {
		t.Fatalf("stdout must not be one JSON envelope: %q", out)
	}
	var start, failure map[string]any
	if err := json.Unmarshal([]byte(lines[0]), &start); err != nil {
		t.Fatalf("start line is not JSON: %v (%q)", err, lines[0])
	}
	if err := json.Unmarshal([]byte(lines[1]), &failure); err != nil {
		t.Fatalf("error line is not JSON: %v (%q)", err, lines[1])
	}
	if start["type"] != "start" || int(start["seq"].(float64)) != 1 {
		t.Fatalf("unexpected start event: %+v", start)
	}
	if failure["type"] != "error" || int(failure["seq"].(float64)) != 2 {
		t.Fatalf("unexpected error event: %+v", failure)
	}
	if start["run_id"] == nil || start["run_id"] != failure["run_id"] {
		t.Fatalf("run_id must be present and consistent: start=%v error=%v", start["run_id"], failure["run_id"])
	}
	errorObject, ok := failure["error"].(map[string]any)
	if !ok || errorObject["code"] != "missing_argument" || errorObject["field"] != "connection-id" || errorObject["suggestion"] == nil {
		t.Fatalf("error details were not preserved: %+v", failure)
	}
	if strings.Contains(out, `"status":"failed"`) && strings.Contains(out, `"type":"error"`) {
		if strings.Count(out, "\n") != 2 {
			t.Fatalf("events output must not include a single-object envelope: %q", out)
		}
	}
}

func TestResourceEmptyDirectoryDoesNotInitialize(t *testing.T) {
	state := filepath.Join(t.TempDir(), "absent")
	out, _ := capture(t, func() {
		root := newRootCommand(&cli{})
		root.SetArgs([]string{"connection", "list", "--state", state, "--json"})
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, `"result_count":0`) {
		t.Fatal(out)
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatal("read initialized state")
	}
	root := newRootCommand(&cli{})
	var completion, stderr bytes.Buffer
	root.SetOut(&completion)
	root.SetErr(&stderr)
	root.SetArgs([]string{"__complete", "connection", "describe", "--state", state, ""})
	if err := root.Execute(); err != nil || stderr.Len() != 0 {
		t.Fatalf("%v %s", err, stderr.String())
	}
	if _, err := os.Stat(state); !os.IsNotExist(err) {
		t.Fatal("completion initialized state")
	}
}
