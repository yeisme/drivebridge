package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge-server/internal/service"
	"io"
)

// captureStdout redirects stdout around fn and returns what was printed.
func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		b, _ := io.ReadAll(r)
		done <- string(b)
	}()
	fn()
	w.Close()
	os.Stdout = old
	return <-done
}

// seedAgentState creates a state directory with one space owned by user u1,
// as a running deployment would have.
func seedAgentState(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if e := os.Chmod(dir, 0700); e != nil {
		t.Fatal(e)
	}
	s, e := service.Open(service.Config{State: dir, MaxBytes: 1 << 20})
	if e != nil {
		t.Fatal(e)
	}
	if e = s.DB.Exec("INSERT INTO spaces (id, owner, name, connection_id, max_bytes) VALUES ('sp-1', 'u1', 'personal', 'platform', 1048576)").Error; e != nil {
		t.Fatal(e)
	}
	s.Close()
	return dir
}

func operatorCLI(t *testing.T, dir string) *cli {
	t.Helper()
	c := &cli{mode: outputMode{json: true}}
	c.effective.Values = map[string]string{"state": dir}
	return c
}

func TestAgentsCreateAndListRoundtrip(t *testing.T) {
	dir := seedAgentState(t)
	cmd := newAgentsCommand(operatorCLI(t, dir))
	create := findSub(t, cmd, "create")
	if e := create.Flags().Set("space", "sp-1"); e != nil {
		t.Fatal(e)
	}
	if e := create.Flags().Set("name", "robo"); e != nil {
		t.Fatal(e)
	}
	if e := create.Flags().Set("scopes", "read"); e != nil {
		t.Fatal(e)
	}
	out := captureStdout(t, func() {
		if err := create.RunE(create, nil); err != nil {
			t.Fatalf("agents create: %v", err)
		}
	})
	var envelope struct {
		Status string `json:"status"`
		Data   struct {
			Token string `json:"token"`
			Agent struct {
				SpaceID string `json:"space_id"`
			} `json:"agent"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(out), &envelope) != nil || envelope.Status != "success" || !strings.HasPrefix(envelope.Data.Token, "db_") || envelope.Data.Agent.SpaceID != "sp-1" {
		t.Fatalf("agents create envelope: %s", out)
	}

	list := findSub(t, cmd, "list")
	out = captureStdout(t, func() {
		if err := list.RunE(list, nil); err != nil {
			t.Fatalf("agents list: %v", err)
		}
	})
	var listed struct {
		Data struct {
			Count  int `json:"count"`
			Agents []struct {
				Name    string `json:"name"`
				Revoked bool   `json:"revoked"`
			} `json:"agents"`
		} `json:"data"`
	}
	if json.Unmarshal([]byte(out), &listed) != nil || listed.Data.Count != 1 || !strings.HasSuffix(listed.Data.Agents[0].Name, "robo") {
		// createAgent prefixes names with the host-cookie marker; that
		// behavior predates this surface and is preserved verbatim.
		t.Fatalf("agents list envelope: %s", out)
	}
	if strings.Contains(out, "db_") {
		t.Fatal("list must never surface token material")
	}
}

func TestAgentsCreateTokenFile(t *testing.T) {
	dir := seedAgentState(t)
	cmd := newAgentsCommand(operatorCLI(t, dir))
	create := findSub(t, cmd, "create")
	tokenPath := filepath.Join(dir, "agent.token")
	for k, v := range map[string]string{"space": "sp-1", "name": "file-agent", "out": tokenPath} {
		if e := create.Flags().Set(k, v); e != nil {
			t.Fatal(e)
		}
	}
	out := captureStdout(t, func() {
		if err := create.RunE(create, nil); err != nil {
			t.Fatal(err)
		}
	})
	if strings.Contains(out, "db_") {
		t.Fatal("with --out the token must not appear in the envelope")
	}
	info, e := os.Stat(tokenPath)
	if e != nil || info.Mode().Perm()&0077 != 0 {
		t.Fatalf("token file must be 0600: %v", e)
	}
}

func findSub(t *testing.T, root *cobra.Command, name string) *cobra.Command {
	t.Helper()
	for _, sub := range root.Commands() {
		if sub.Name() == name {
			return sub
		}
	}
	t.Fatalf("subcommand %s not found", name)
	return nil
}
