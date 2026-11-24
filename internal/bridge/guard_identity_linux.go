//go:build linux

package bridge

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"os"
	"syscall"
)

// guardRootIdentity returns a stable digest of the physical directory identity.
// A path string alone is not identity: renaming or replacing the directory keeps
// the name but changes the inode. Only platforms that expose device/inode are
// implemented; elsewhere guard admission stays unsupported instead of guessing.
func guardRootIdentity(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil || !info.IsDir() {
		return "", errors.New("guard_unavailable")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", errors.New("guard_identity_unsupported")
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("drivebridge-guard-root:%d:%d", stat.Dev, stat.Ino)))
	return fmt.Sprintf("sha256:%x", sum), nil
}
