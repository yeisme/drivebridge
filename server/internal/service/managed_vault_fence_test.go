package service

import (
	"crypto/sha256"
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/yeisme/drivebridge-server/pkg/managedstate"
)

// C2 in managed-vault-guard-protocol.md: the Server's real constructor must
// refuse a fenced state instead of creating an empty database at the marker
// path. The fenced layout is authored by the owner migration protocol on a
// disposable state; nothing here touches a real deployment state.
func TestManagedVaultServerOpenRejectsFencedState(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("managed state layout is Linux-only until platform acceptance")
	}
	state := filepath.Join(t.TempDir(), "state")
	svc, err := Open(Config{State: state, Development: true})
	if err != nil {
		t.Fatalf("create legacy server state: %v", err)
	}
	svc.Close()
	if err = os.WriteFile(filepath.Join(state, "process.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := managedstate.Prepare(state, "server", "migration-server-original")
	if err != nil {
		t.Fatalf("prepare: %v", err)
	}
	installed, err := managedstate.Install(state, plan)
	if err != nil {
		t.Fatalf("install: %v", err)
	}
	before, err := os.ReadFile(installed.DatabasePath)
	if err != nil {
		t.Fatal(err)
	}
	again, reopenErr := Open(Config{State: state, Development: true})
	if again != nil {
		again.Close()
		t.Fatal("the Server constructor opened a fenced state")
	}
	if reopenErr == nil {
		t.Fatal("the Server constructor accepted a fenced state without an error")
	}
	after, err := os.ReadFile(installed.DatabasePath)
	if err != nil || sha256.Sum256(before) != sha256.Sum256(after) {
		t.Fatal("the rejected constructor modified the preserved database")
	}
	marker, err := os.ReadFile(filepath.Join(state, "metadata.sqlite"))
	if err != nil {
		t.Fatalf("fenced marker missing: %v", err)
	}
	if len(marker) < 16 || string(marker[:16]) == "SQLite format 3\x00" {
		t.Fatal("the fenced path became a database after the rejected open")
	}
	if replay, err := managedstate.Install(state, plan); err != nil || replay != installed {
		t.Fatalf("original migration identity could not reconcile: %#v %v", replay, err)
	}
}

// D08 on the Server module: there is no binding surface yet, so the state fence
// is the whole protection. A fenced state must deny every entry at construction
// time — HTTP, SDK, MCP and worker handlers cannot exist without a service, and
// the constructor must not hand back a partially open one.
func TestManagedVaultIntegrationServerFencedStateDeniesAllEntries(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("managed state layout is Linux-only until platform acceptance")
	}
	state := filepath.Join(t.TempDir(), "state")
	svc, err := Open(Config{State: state, Development: true})
	if err != nil {
		t.Fatalf("create legacy server state: %v", err)
	}
	svc.Close()
	if err = os.WriteFile(filepath.Join(state, "process.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := managedstate.Prepare(state, "server", "migration-server-integration")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = managedstate.Install(state, plan); err != nil {
		t.Fatal(err)
	}
	denied, denyErr := Open(Config{State: state, Development: true})
	if denied != nil || denyErr == nil {
		t.Fatalf("a fenced state produced a usable service: %v %v", denied, denyErr)
	}
	// The process lock must be released again, otherwise a legitimate retry
	// after an approved migration would deadlock on `state_in_use`.
	retried, retryErr := Open(Config{State: state, Development: true})
	if retried != nil {
		retried.Close()
	}
	if retryErr != nil && retryErr.Error() == "state_in_use" {
		t.Fatal("a rejected open leaked the process lock")
	}
}
