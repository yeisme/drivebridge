package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
}

func TestLoadStrictUnknownKeysAndShapes(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "c.yaml")
	write(t, bad, "state: /s\nenabled: true\n")
	if _, err := Load(bad); err == nil || !strings.Contains(err.Error(), "unknown keys: enabled") {
		t.Fatalf("unknown key must name the key: %v", err)
	}
	badOrigin := filepath.Join(dir, "o.yaml")
	write(t, badOrigin, "origin: https://x.example.com/path\n")
	if _, err := Load(badOrigin); err == nil || !strings.Contains(err.Error(), "origin") {
		t.Fatalf("origin with path must be rejected: %v", err)
	}
	badBytes := filepath.Join(dir, "m.yaml")
	write(t, badBytes, "max_bytes: -5\n")
	if _, err := Load(badBytes); err == nil || !strings.Contains(err.Error(), "max_bytes") {
		t.Fatalf("negative max_bytes must be rejected: %v", err)
	}
	badFeature := filepath.Join(dir, "f.yaml")
	write(t, badFeature, "label_management: maybe\n")
	if _, err := Load(badFeature); err == nil || !strings.Contains(err.Error(), "label_management") { t.Fatalf("invalid feature value must be rejected: %v", err) }
	good := filepath.Join(dir, "g.json")
	write(t, good, `{"state": "/s", "listen": "127.0.0.1:9000", "origin": "https://files.example.com"}`)
	file, err := Load(good)
	if err != nil {
		t.Fatal(err)
	}
	if file.Values["listen"] != "127.0.0.1:9000" {
		t.Fatalf("values: %v", file.Values)
	}
}

func TestResolveAllIntAndPrecedence(t *testing.T) {
	t.Setenv("DRIVEBRIDGE_SERVER_ISSUER", "https://env-issuer.example.com")
	file := File{Path: "/c", Values: map[string]string{"issuer": "https://file-issuer.example.com", "max_bytes": "2048"}}
	e := ResolveAll(file, map[string]Flag{"state": {Value: "/flag-state", Set: true}}, map[string]string{"state": "/def", "max_bytes": "1073741824"})
	if e.Values["state"] != "/flag-state" || e.Sources["state"] != SourceFlag {
		t.Fatalf("flag precedence: %v %v", e.Values["state"], e.Sources["state"])
	}
	if e.Values["issuer"] != "https://env-issuer.example.com" || e.Sources["issuer"] != SourceEnv {
		t.Fatalf("env beats file: %v", e.Values["issuer"])
	}
	if e.Int("max_bytes", 1) != 2048 {
		t.Fatal("file max_bytes should parse")
	}
	if e.Int("tenant_name", 1) != 1 || e.Int("listen", 7) != 7 {
		t.Fatal("non-integer keys fall back")
	}
}

func TestTemplateCoversEveryKey(t *testing.T) {
	tpl := Template()
	for _, key := range Keys {
		if !strings.Contains(tpl, key+":") {
			t.Fatalf("template missing %s", key)
		}
	}
	if strings.Contains(tpl, "secret_value") {
		t.Fatal("template must not carry secret values")
	}
}
