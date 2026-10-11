// Package config resolves the server CLI configuration surface: an optional
// config file (viper), per-key environment overrides, and explicit flags.
// Deployment write switches (--enabled/--development) and secret values never
// come from files or the environment — only paths and limits do.
package config

import (
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

// Keys are the complete set of file-configurable settings. Anything else in a
// config file is rejected: a typo must fail loudly, not silently do nothing.
var Keys = []string{"state", "listen", "origin", "issuer", "client_id", "client_secret_file", "onedrive_client_id", "onedrive_secret_file", "web_dir", "max_bytes", "tenant_name", "tenant_origin", "tenant_contact", "service_identity_issuer", "service_identity_endpoint", "service_access_reporter_credential_file", "document_editing", "property_editing", "automation_management", "share_receivers", "hold_management", "label_management"}

var known = func() map[string]bool {
	m := map[string]bool{}
	for _, k := range Keys {
		m[k] = true
	}
	return m
}()

// File is a strictly validated config file.
type File struct {
	Path   string
	Values map[string]string
}

// Source reports where a resolved value came from.
type Source string

const (
	SourceFlag    Source = "flag"
	SourceEnv     Source = "env"
	SourceFile    Source = "file"
	SourceDefault Source = "default"
)

// EnvName maps a key to its environment variable.
func EnvName(key string) string { return "DRIVEBRIDGE_SERVER_" + strings.ToUpper(key) }

// Discover returns the config file path: an explicit path, then the
// DRIVEBRIDGE_SERVER_CONFIG variable, then ./drivebridge-server.* in the
// working directory, then the OS user config dir, then /etc/drivebridge-server.
// Empty means "no file, defaults only" — identical to flag-only behavior.
func Discover(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if fromEnv := os.Getenv("DRIVEBRIDGE_SERVER_CONFIG"); fromEnv != "" {
		return fromEnv
	}
	if cwd, err := os.Getwd(); err == nil {
		for _, ext := range []string{".yaml", ".yml", ".json", ".toml"} {
			candidate := filepath.Join(cwd, "drivebridge-server"+ext)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		for _, ext := range []string{".yaml", ".yml", ".json", ".toml"} {
			candidate := filepath.Join(dir, "drivebridge-server", "config"+ext)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	for _, ext := range []string{".yaml", ".yml", ".json", ".toml"} {
		candidate := filepath.Join("/etc", "drivebridge-server", "config"+ext)
		if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// Load reads a config file with viper, rejects unknown keys and validates shapes.
func Load(path string) (File, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return File{}, fmt.Errorf("read config %q: %w", path, err)
	}
	var unknown []string
	for _, key := range v.AllKeys() {
		if !known[key] {
			unknown = append(unknown, key)
		}
	}
	if len(unknown) > 0 {
		sort.Strings(unknown)
		return File{}, fmt.Errorf("config %q: unknown keys: %s", path, strings.Join(unknown, ", "))
	}
	values := map[string]string{}
	for _, key := range Keys {
		if v.IsSet(key) {
			values[key] = strings.TrimSpace(v.GetString(key))
		}
	}
	for _, key := range []string{"origin", "issuer"} {
		if values[key] != "" {
			if u, err := url.Parse(values[key]); err != nil || u.Host == "" || u.Path != "" || u.RawQuery != "" || u.Fragment != "" {
				return File{}, fmt.Errorf("config %q: %s must be an origin URL without a path", path, key)
			}
		}
	}
	if values["max_bytes"] != "" {
		if n, err := strconv.ParseInt(values["max_bytes"], 10, 64); err != nil || n <= 0 {
			return File{}, fmt.Errorf("config %q: max_bytes must be a positive integer", path)
		}
	}
	for _, key := range []string{"document_editing", "property_editing", "automation_management", "share_receivers", "hold_management", "label_management"} {
		if values[key] != "" && values[key] != "true" && values[key] != "false" { return File{}, fmt.Errorf("config %q: %s must be true or false", path, key) }
	}
	return File{Path: path, Values: values}, nil
}

// Resolve picks the winning value for one key and reports its source:
// explicit flag, environment, file, then the built-in default.
func Resolve(file File, key, flagValue string, flagSet bool, def string) (string, Source) {
	if flagSet && flagValue != "" {
		return flagValue, SourceFlag
	}
	if env := strings.TrimSpace(os.Getenv(EnvName(key))); env != "" {
		return env, SourceEnv
	}
	if v, ok := file.Values[key]; ok && v != "" {
		return v, SourceFile
	}
	return def, SourceDefault
}

// Flag carries an explicitly provided command-line value.
type Flag struct {
	Value string
	Set   bool
}

// Effective is the resolved configuration handed to commands.
type Effective struct {
	Values  map[string]string
	Sources map[string]Source
	Path    string
	HasFile bool
}

// ResolveAll resolves every known key.
func ResolveAll(file File, flags map[string]Flag, defaults map[string]string) Effective {
	values := map[string]string{}
	sources := map[string]Source{}
	for _, key := range Keys {
		flag := flags[key]
		value, source := Resolve(file, key, flag.Value, flag.Set, defaults[key])
		values[key] = value
		sources[key] = source
	}
	return Effective{Values: values, Sources: sources, Path: file.Path, HasFile: file.Path != ""}
}

// Int parses an integer key with a fallback.
func (e Effective) Int(key string, fallback int64) int64 {
	if n, err := strconv.ParseInt(e.Values[key], 10, 64); err == nil && n > 0 {
		return n
	}
	return fallback
}

// Template renders a commented starter config for `config init`.
func Template() string {
	var b strings.Builder
	b.WriteString("# DriveBridge server deployment configuration. Comment out anything unused.\n")
	b.WriteString("# Secrets never live here: only file paths to secrets held elsewhere.\n")
	b.WriteString("# Write switches (--enabled/--development) stay on the command line by design.\n\n")
	home := ""
	if dir, err := os.UserConfigDir(); err == nil {
		home = dir
	}
	fmt.Fprintf(&b, "state: %s\n", filepath.Join(home, "drivebridge-server"))
	b.WriteString("listen: 127.0.0.1:8787\n")
	b.WriteString("origin: \"\"\n")
	b.WriteString("issuer: \"\"\n")
	b.WriteString("# Optional unified identity; issuer and fixed backend endpoint.\n")
	b.WriteString("service_identity_issuer: \"\"\n")
	b.WriteString("service_identity_endpoint: \"\"\n")
	b.WriteString("# Hosted business-operation admission reporter credential file; empty rejects hosted writes.\n")
	b.WriteString("service_access_reporter_credential_file: \"\"\n")
	b.WriteString("client_id: \"\"\n")
	b.WriteString("client_secret_file: \"\"\n")
	b.WriteString("onedrive_client_id: \"\"\n")
	b.WriteString("onedrive_secret_file: \"\"\n")
	b.WriteString("web_dir: \"\"\n")
	b.WriteString("max_bytes: 1073741824\n")
	b.WriteString("tenant_name: \"\"\n")
	b.WriteString("tenant_origin: \"\"\n")
	b.WriteString("tenant_contact: \"\"\n")
	b.WriteString("# Human-confirmed personal features stay disabled unless explicitly set true.\n")
	b.WriteString("document_editing: false\nproperty_editing: false\nautomation_management: false\nshare_receivers: false\nhold_management: false\nlabel_management: false\n")
	return b.String()
}
