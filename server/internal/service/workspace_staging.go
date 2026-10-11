package service

import "strings"

// The server-owned staging segment is never delegated through legacy scopes.
func workspaceStagingPath(path string) bool {
	for _, segment := range strings.Split(path, "/") {
		if segment == ".inbox-staging" {
			return true
		}
	}
	return false
}
