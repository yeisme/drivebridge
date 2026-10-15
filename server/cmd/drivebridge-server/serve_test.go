package main

import (
	"bufio"
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"
)

func TestDeploymentCapabilityFlagsDefaultOff(t *testing.T) {
	cmd := newServeCommand(&cli{})
	for _, name := range []string{"admin-direct-write", "cloud-connectors", "transfer-plans"} {
		flag := cmd.Flags().Lookup(name)
		if flag == nil || flag.DefValue != "false" {
			t.Fatalf("%s must be explicit", name)
		}
		if err := cmd.Flags().Set(name, "true"); err != nil {
			t.Fatal(err)
		}
	}
}

func TestAutomationManagementFlagDefaultsOff(t *testing.T) {
	cmd := newServeCommand(&cli{})
	flag := cmd.Flags().Lookup("automation-management")
	if flag == nil || flag.DefValue != "false" {
		t.Fatal("automation management must be opt-in")
	}
	if err := cmd.Flags().Set("automation-management", "true"); err != nil {
		t.Fatal(err)
	}
	enabled, err := cmd.Flags().GetBool("automation-management")
	if err != nil || !enabled {
		t.Fatal("automation opt-in failed", err)
	}
}

func TestPropertyEditingFlagDefaultsOff(t *testing.T) {
	cmd := newServeCommand(&cli{})
	flag := cmd.Flags().Lookup("property-editing")
	if flag == nil || flag.DefValue != "false" {
		t.Fatal("property edits must be opt-in")
	}
	if err := cmd.Flags().Set("property-editing", "true"); err != nil {
		t.Fatal(err)
	}
	enabled, err := cmd.Flags().GetBool("property-editing")
	if err != nil || !enabled {
		t.Fatal("explicit property editing opt-in failed", err)
	}
}

// TestServeRejectsNonEventModes keeps the process-output boundary: serve
// owns stdout except for the --events supervision stream.
func TestServeRejectsNonEventModes(t *testing.T) {
	dir := seedAgentState(t)
	c := operatorCLI(t, dir)
	c.mode.json = true
	cmd := newServeCommand(c)
	err := cmd.RunE(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "serve owns the process lifecycle") {
		t.Fatalf("--json must stay rejected, got %v", err)
	}
}

// TestServeEventsStreamRunsStartToReady boots serve in --events mode on an
// ephemeral listener and asserts the supervision stream shape: start with
// spec_version, then a ready progress event once the socket is bound.
func TestServeEventsStreamRunsStartToReady(t *testing.T) {
	dir := seedAgentState(t)
	c := operatorCLI(t, dir)
	c.mode = outputMode{events: true}
	cmd := newServeCommand(c)
	if e := cmd.Flags().Set("listen", "127.0.0.1:0"); e != nil {
		t.Fatal(e)
	}

	r, w, e := os.Pipe()
	if e != nil {
		t.Fatal(e)
	}
	old := os.Stdout
	os.Stdout = w
	go func() {
		_ = cmd.RunE(cmd, nil)
	}()
	defer func() { os.Stdout = old; w.Close() }()

	lines := make(chan string, 4)
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			lines <- scanner.Text()
		}
	}()
	readLine := func() string {
		select {
		case line := <-lines:
			return line
		case <-time.After(5 * time.Second):
			t.Fatal("serve events stream stalled before ready")
			return ""
		}
	}
	var start, ready map[string]any
	if json.Unmarshal([]byte(readLine()), &start) != nil || start["type"] != "start" || start["spec_version"] != "1.0" || start["command"] != "drivebridge-server.serve" {
		t.Fatalf("start event: %v", start)
	}
	if json.Unmarshal([]byte(readLine()), &ready) != nil || ready["type"] != "progress" || ready["stage"] != "ready" {
		t.Fatalf("ready event: %v", ready)
	}
	if seq := int(ready["seq"].(float64)); seq <= int(start["seq"].(float64)) {
		t.Fatalf("seq must be monotonic: %v %v", start, ready)
	}
	// The listener answered; the process ends with the test binary.
}
