package main

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

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
