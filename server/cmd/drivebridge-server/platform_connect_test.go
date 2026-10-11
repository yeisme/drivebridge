package main

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
)

func feedStdin(t *testing.T, body string) {
	t.Helper()
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.WriteString(body); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	old := os.Stdin
	os.Stdin = r
	t.Cleanup(func() { os.Stdin = old; r.Close() })
}

func loopbackS3Config(t *testing.T) string {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	t.Cleanup(srv.Close)
	return `{"endpoint":"` + srv.URL + `","region":"us-east-1","bucket":"fixture","access_key":"k","secret_key":"s"}`
}

func TestPlatformConnectRejectsLoopbackWithoutDevelopment(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	feedStdin(t, loopbackS3Config(t))
	if _, err := capture(t, func() error { return runCLI("platform-connect", "--state", dir) }); err == nil {
		t.Fatal("loopback HTTP endpoint must be rejected without --development")
	}
}

func TestPlatformConnectDevelopmentAllowsLoopback(t *testing.T) {
	dir := t.TempDir()
	if err := os.Chmod(dir, 0700); err != nil {
		t.Fatal(err)
	}
	feedStdin(t, loopbackS3Config(t))
	if _, err := capture(t, func() error { return runCLI("platform-connect", "--state", dir, "--development") }); err != nil {
		t.Fatalf("loopback HTTP endpoint with --development should succeed: %v", err)
	}
}
