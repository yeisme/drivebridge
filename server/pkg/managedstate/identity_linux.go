package managedstate

import (
	"crypto/sha256"
	"fmt"
	"os"
	"syscall"
)

func rootIdentity(root *os.Root) (string, error) {
	info, err := root.Lstat(".")
	if err != nil {
		return "", code("state_unavailable")
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !ok {
		return "", code("state_layout_unsupported")
	}
	sum := sha256.Sum256([]byte(fmt.Sprintf("drivebridge-state:%d:%d", stat.Dev, stat.Ino)))
	return fmt.Sprintf("sha256:%x", sum), nil
}
