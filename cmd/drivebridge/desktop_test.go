package main

import (
	"bytes"
	"encoding/json"
	"github.com/yeisme/drivebridge-server/pkg/managedstate"
	"github.com/yeisme/drivebridge/internal/bridge"
	"github.com/yeisme/drivebridge/internal/desktop"
	"os"
	"path/filepath"
	"testing"
)

func TestDesktopSchemaIgnoresOrdinaryConfig(t *testing.T) {
	t.Setenv("DRIVEBRIDGE_CONFIG", "/does-not-exist/secret-config")
	command := newRootCommand(&cli{})
	var output bytes.Buffer
	command.SetOut(&output)
	command.SetArgs([]string{"desktop", "schema"})
	if err := command.Execute(); err != nil {
		t.Fatal(err)
	}
	var schemas map[string]json.RawMessage
	if json.Unmarshal(output.Bytes(), &schemas) != nil || len(schemas) != 4 {
		t.Fatal("private schemas missing")
	}
	for _, name := range []string{"HostBootstrap", "HostSelection", "HostCancel", "HostEvent"} {
		if len(schemas[name]) == 0 {
			t.Fatal("missing private schema")
		}
	}
}
func TestDesktopRejectsInheritedFlags(t *testing.T) {
	for _, args := range [][]string{{"desktop", "schema", "--json"}, {"--state", "/private", "desktop", "schema"}, {"desktop", "serve", "--events"}, {"desktop", "schema", "extra"}} {
		command := newRootCommand(&cli{})
		command.SetArgs(args)
		command.SetOut(&bytes.Buffer{})
		if err := command.Execute(); err == nil {
			t.Fatal("desktop flags accepted")
		}
	}
}

func TestDesktopSelectedStateIsolationAndRefusal(t *testing.T) {
	state := filepath.Join(t.TempDir(), "state")
	s, err := bridge.New(state, bridge.NewOneDrive(), false)
	if err != nil {
		t.Fatal(err)
	}
	s.Close()
	if err = os.WriteFile(filepath.Join(state, "process.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := managedstate.Prepare(state, "cli", "desktop-state-test")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = managedstate.Install(state, plan); err != nil {
		t.Fatal(err)
	}
	first, closeFirst, ref, err := desktopSelectedAdmission(desktop.HostBootstrap{StatePath: state})
	if err != nil || first == nil || closeFirst == nil || ref != plan.StateIdentity {
		t.Fatal("selected state not admitted")
	}
	second, closeSecond, secondRef, err := desktopSelectedAdmission(desktop.HostBootstrap{StatePath: state})
	if closeSecond != nil {
		closeSecond()
	}
	if err != nil || second != nil || secondRef != ref {
		t.Fatal("state lock or native identity lost")
	}
	closeFirst()
	reopened, closeReopened, reopenedRef, err := desktopSelectedAdmission(desktop.HostBootstrap{StatePath: state})
	if err != nil || reopened == nil || reopenedRef != ref {
		t.Fatal("state did not reopen with its original identity")
	}
	closeReopened()
	absent := filepath.Join(t.TempDir(), "missing")
	denied, release, _, _ := desktopSelectedAdmission(desktop.HostBootstrap{StatePath: absent})
	if release != nil {
		release()
	}
	if denied != nil {
		t.Fatal("missing state admitted")
	}
	if _, err = os.Stat(absent); !os.IsNotExist(err) {
		t.Fatal("missing state was created")
	}
	corrupt := t.TempDir()
	if err = os.WriteFile(filepath.Join(corrupt, "state.sqlite"), []byte("damaged"), 0600); err != nil {
		t.Fatal(err)
	}
	denied, release, _, _ = desktopSelectedAdmission(desktop.HostBootstrap{StatePath: corrupt})
	if release != nil {
		release()
	}
	if denied != nil {
		t.Fatal("damaged state admitted")
	}
	data, err := os.ReadFile(filepath.Join(corrupt, "state.sqlite"))
	if err != nil || string(data) != "damaged" {
		t.Fatal("damaged state replaced")
	}
}

func TestDesktopExplicitStatePreparation(t *testing.T) {
	state := filepath.Join(t.TempDir(), "new-state")
	if err := desktopPrepareState(state, "create"); err != nil {
		t.Fatal(err)
	}
	layout, err := managedstate.Inspect(state, "cli")
	if err != nil || layout.Layout != "fenced" {
		t.Fatal("created state not fenced")
	}
	if err = desktopPrepareState(state, "create"); err == nil {
		t.Fatal("create replaced existing state")
	}
	again, err := managedstate.Inspect(state, "cli")
	if err != nil || again.Plan.StateIdentity != layout.Plan.StateIdentity {
		t.Fatal("existing state changed")
	}
	legacy := filepath.Join(t.TempDir(), "legacy")
	owner, err := bridge.New(legacy, bridge.NewOneDrive(), false)
	if err != nil {
		t.Fatal(err)
	}
	if err = owner.DB.Create(&bridge.Space{ID: "retained"}).Error; err != nil {
		t.Fatal(err)
	}
	owner.Close()
	if err = os.WriteFile(filepath.Join(legacy, "process.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	if err = desktopPrepareState(legacy, "migrate"); err != nil {
		t.Fatal(err)
	}
	owner, err = bridge.New(legacy, bridge.NewOneDrive(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer owner.Close()
	var retained bridge.Space
	if err = owner.DB.First(&retained, "id = ?", "retained").Error; err != nil {
		t.Fatal("migration lost original metadata")
	}
	if err = desktopPrepareState(filepath.Join(t.TempDir(), "absent"), "migrate"); err == nil {
		t.Fatal("migration created replacement state")
	}
}
