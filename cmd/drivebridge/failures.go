package main

import (
	"errors"

	"github.com/yeisme/drivebridge/internal/bridge"
)

// errQuiet suppresses the generic failure handling in main: the command has
// already produced its complete projection (a failed envelope or a doctor
// report) before returning it.
var errQuiet = errors.New("quiet")

// cliFailure carries a typed error code plus a safe message through the
// output machinery; the message never interpolates third-party error text.
type cliFailure struct{ code, message string }

func (e *cliFailure) Error() string { return e.code + ": " + e.message }

func fail(code, message string) error { return &cliFailure{code, message} }

// safeCode reports whether an error string is one of our own typed codes
// (lowercase snake_case within the length bound) and therefore safe to
// surface in every output mode.
func safeCode(v string) bool {
	if len(v) == 0 || len(v) > 80 {
		return false
	}
	for _, r := range v {
		if !(r >= 'a' && r <= 'z') && r != '_' {
			return false
		}
	}
	return true
}

// codify converts any error into a code+message pair safe for machine
// output: typed codes pass through with their registry hint as the message;
// anything third-party collapses to command_failed so raw text never reaches
// stdout or stderr.
func codify(err error) (code, message string) {
	if err == nil {
		return "", ""
	}
	var f *cliFailure
	if errors.As(err, &f) {
		return f.code, f.message
	}
	if v := err.Error(); safeCode(v) {
		return v, hintEN(v)
	}
	return "command_failed", "The command failed before producing a projection; no third-party diagnostics are surfaced."
}

// terminalCode maps a stopped transfer to its typed error code: the
// operation's own code when it is one of ours, otherwise a status-derived
// recovery code.
func terminalCode(op bridge.Operation) string {
	return terminalCodeValues(op.Error, op.Status)
}

// terminalCodeValues is the transport-neutral core shared by local bridge
// operations and remote SDK receipts.
func terminalCodeValues(errCode, status string) string {
	if safeCode(errCode) {
		return errCode
	}
	switch status {
	case "cancelled":
		return "operation_cancelled"
	case "interrupted":
		return "resume_required"
	}
	return "transfer_failed"
}
