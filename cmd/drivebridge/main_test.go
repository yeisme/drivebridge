package main

import (
	"github.com/rogpeppe/go-internal/testscript"
	"testing"
)

func TestMain(m *testing.M) { testscript.Main(m, map[string]func(){"drivebridge": main}) }
func TestCLI(t *testing.T)  { testscript.Run(t, testscript.Params{Dir: "testdata"}) }
