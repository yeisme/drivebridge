package main

import "github.com/yeisme/drivebridge/internal/recovery"

// codeRegistry is the CLI alias over the shared recovery registry; the doc
// sync test and human-mode output read through it unchanged.
var codeRegistry = recovery.Registry

// hintEN returns the recovery hint for a code, falling back to the generic
// guidance that matches the tool's core discipline (inspect before retry).
func hintEN(code string) string {
	return recovery.Hint(code)
}
