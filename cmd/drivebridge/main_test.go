package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rogpeppe/go-internal/testscript"
)

func TestMain(m *testing.M) { testscript.Main(m, map[string]func(){"drivebridge": main}) }
func TestCLI(t *testing.T)  { testscript.Run(t, testscript.Params{Dir: "testdata"}) }

func TestBindCLI(t *testing.T) {
	src, err := os.ReadFile(filepath.Join("testdata", "bind.txt"))
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	if err = os.WriteFile(filepath.Join(dir, "bind.txt"), src, 0600); err != nil {
		t.Fatal(err)
	}
	testscript.Run(t, testscript.Params{Dir: dir})
}
