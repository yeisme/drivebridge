//go:build !windows

package main

import (
	"os"
	"syscall"
)

// Lock modes and the writability probe keep the single-process state lock and
// doctor checks working on Unix; Windows maps them in platform_windows.go.
const (
	lockExclusive   = syscall.LOCK_EX
	lockNonBlocking = syscall.LOCK_NB
	lockRelease     = syscall.LOCK_UN
)

func lockFile(f *os.File, mode int) error {
	return syscall.Flock(int(f.Fd()), mode)
}

func writable(path string) error {
	// 2 is W_OK, identical on linux and darwin.
	return syscall.Access(path, 2)
}
