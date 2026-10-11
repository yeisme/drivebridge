package main

import (
	"bytes"
	"encoding/json"
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
