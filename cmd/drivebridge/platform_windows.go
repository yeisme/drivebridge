//go:build windows

package main

import (
	"os"
	"path/filepath"
	"syscall"
	"unsafe"
)

// LockFileEx maps the flock-style single-process state lock onto Windows byte
// ranges. Compiled but not yet accepted on a real Windows machine; the on-host
// gates are listed in client/drivebridge-client/docs/cross-platform-client.md.
const (
	lockExclusive   = 0x2 // LOCKFILE_EXCLUSIVE_LOCK
	lockNonBlocking = 0x1 // LOCKFILE_FAIL_IMMEDIATELY
	lockRelease     = 0x8 // sentinel handled before flag mapping
)

var (
	kernel32       = syscall.NewLazyDLL("kernel32.dll")
	lockFileEx     = kernel32.NewProc("LockFileEx")
	unlockFileEx   = kernel32.NewProc("UnlockFileEx")
	maxLockRangeLo = uint32(0xFFFFFFFF)
)

func lockFile(f *os.File, mode int) error {
	handle := syscall.Handle(f.Fd())
	var overlapped syscall.Overlapped
	if mode == lockRelease {
		result, _, callErr := unlockFileEx.Call(uintptr(handle), 0, uintptr(maxLockRangeLo), 0, uintptr(unsafe.Pointer(&overlapped)))
		if result == 0 {
			return callErr
		}
		return nil
	}
	result, _, callErr := lockFileEx.Call(uintptr(handle), uintptr(mode), 0, uintptr(maxLockRangeLo), 0, uintptr(unsafe.Pointer(&overlapped)))
	if result == 0 {
		return callErr
	}
	return nil
}

func writable(path string) error {
	// syscall.Access does not exist on Windows; probe with a transient file
	// that is removed immediately.
	probe := filepath.Join(path, ".drivebridge-write-probe")
	if err := os.WriteFile(probe, nil, 0600); err != nil {
		return err
	}
	return os.Remove(probe)
}
