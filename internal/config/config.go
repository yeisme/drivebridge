// Package config resolves the CLI configuration surface: an optional config
// file (viper), per-key environment overrides, and explicit flags. Security
// switches (--enabled/--handoff) and secrets never come from files or env.
package config

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/viper"
)

// Keys are the complete set of file-configurable settings. Anything else in a
// config file is rejected: a typo must fail loudly, not silently do nothing.
var Keys = []string{"state", "rclone_config", "scaena_url", "scaena_token_file", "remote_url", "remote_token_file", "local_root"}

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
func EnvName(key string) string { return "DRIVEBRIDGE_" + strings.ToUpper(key) }

// Discover returns the config file path: an explicit path, then the
// DRIVEBRIDGE_CONFIG variable, then ./drivebridge.{yaml,yml,json,toml}, then
// the OS user config dir. Empty means "no file, defaults only" — identical to
// the flag-only behavior that existed before configuration files.
func Discover(explicit string) string {
	if explicit != "" {
		return explicit
	}
	if fromEnv := os.Getenv("DRIVEBRIDGE_CONFIG"); fromEnv != "" {
		return fromEnv
	}
	if cwd, err := os.Getwd(); err == nil {
		for _, ext := range []string{".yaml", ".yml", ".json", ".toml"} {
			candidate := filepath.Join(cwd, "drivebridge"+ext)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	if dir, err := os.UserConfigDir(); err == nil && dir != "" {
		for _, ext := range []string{".yaml", ".yml", ".json", ".toml"} {
			candidate := filepath.Join(dir, "drivebridge", "config"+ext)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	return ""
}

// Load reads a config file with viper and rejects unknown keys.
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
		return File{}, fmt.Errorf("unknown_config_key")
	}
	values := map[string]string{}
	for _, key := range Keys {
		if v.IsSet(key) {
			values[key] = strings.TrimSpace(v.GetString(key))
		}
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

// Effective is the resolved configuration handed to commands; Sources carries
// the per-key origin for `config show` and doctor.
type Effective struct {
	Values  map[string]string
	Sources map[string]Source
	Path    string
	HasFile bool
}

// Flag carries an explicitly provided command-line value.
type Flag struct {
	Value string
	Set   bool
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

// Template renders a commented starter config for `config init`.
func Template() string {
	var b strings.Builder
	b.WriteString("# DriveBridge CLI configuration. Delete or comment out anything unused.\n")
	b.WriteString("# Secrets never live here: token file paths are fine, tokens are not.\n")
	b.WriteString("# Write switches (--enabled/--handoff) stay on the command line by design.\n\n")
	home := ""
	if dir, err := os.UserConfigDir(); err == nil {
		home = dir
	}
	fmt.Fprintf(&b, "state: %s\n", filepath.Join(home, "drivebridge"))
	fmt.Fprintf(&b, "rclone_config: %s\n", filepath.Join(home, "rclone", "rclone.conf"))
	b.WriteString("scaena_url: \"\"\n")
	b.WriteString("scaena_token_file: \"\"\n")
	b.WriteString("remote_url: \"\"\n")
	b.WriteString("remote_token_file: \"\"\n")
	b.WriteString("local_root: \"\"\n")
	return b.String()
}
