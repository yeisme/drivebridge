package main

import "errors"

// errQuiet suppresses generic failure handling in main: the command already
// produced its complete projection before returning it.
var errQuiet = errors.New("quiet")

// cliFailure carries a typed error code plus a safe message through the
// output machinery; the message never interpolates third-party error text.
type cliFailure struct{ code, message string }

func (e *cliFailure) Error() string { return e.code + ": " + e.message }

func fail(code, message string) error { return &cliFailure{code, message} }

// safeCode reports whether an error string is one of our own typed codes.
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
// output; third-party text collapses to command_failed.
func codify(err error) (code, message string) {
	if err == nil {
		return "", ""
	}
	var f *cliFailure
	if errors.As(err, &f) {
		return f.code, f.message
	}
	if v := err.Error(); safeCode(v) {
		return v, "The command failed; inspect the error code and the operator runbook."
	}
	return "command_failed", "The command failed before producing a projection; no third-party diagnostics are surfaced."
}
