//go:build !linux

package bridge

import "errors"

// guardRootIdentity is unimplemented outside Linux. Managed-vault guard
// admission therefore refuses to install rather than treating a path name as a
// physical identity. macOS/Windows stay unsupported until each platform has its
// own evidence-backed identity check.
func guardRootIdentity(string) (string, error) {
	return "", errors.New("guard_identity_unsupported")
}
