//go:build !linux

package managedstate

import "os"

func rootIdentity(*os.Root) (string, error) {
	return "", code("state_layout_unsupported")
}
