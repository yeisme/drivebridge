package main

import (
	"errors"
	"io"
	"io/fs"
	"log"
	"log/slog"
	"os"
	"path/filepath"

	rcloneconfig "github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge/internal/bridge"
	"github.com/yeisme/drivebridge/internal/config"
)

// cli carries the resolved configuration shared by every subcommand.
type cli struct {
	root                              *cobra.Command
	command                           string
	mode                              outputMode
	configPath, flagState, flagRclone string
	file                              config.File
	effective                         config.Effective
}

// resolve loads the optional config file and resolves every known key:
// explicit flag > DRIVEBRIDGE_* environment > file > built-in default.
// config init tolerates a missing explicit path: that file is its target.
func (c *cli) resolve(cmd *cobra.Command) (config.File, config.Effective, error) {
	path := config.Discover(c.configPath)
	tolerateMissing := cmd.Parent() != nil && cmd.Parent().Name() == "config" && cmd.Name() == "init"
	var file config.File
	if path != "" {
		loaded, err := config.Load(path)
		if err != nil {
			if !(tolerateMissing && errors.Is(err, fs.ErrNotExist)) {
				return file, config.Effective{}, err
			}
		} else {
			file = loaded
		}
	}
	home, err := os.UserConfigDir()
	if err != nil {
		return file, config.Effective{}, err
	}
	// Any command-local flag that backs a known key participates in
	// resolution with the same precedence as the persistent ones.
	flags := map[string]config.Flag{}
	for key, name := range flagNames {
		if cmd.Flags().Lookup(name) != nil && cmd.Flags().Changed(name) {
			flags[key] = config.Flag{Value: cmd.Flags().Lookup(name).Value.String(), Set: true}
		}
	}
	effective := config.ResolveAll(file, flags, map[string]string{
		"state":         filepath.Join(home, "drivebridge"),
		"rclone_config": filepath.Join(home, "rclone", "rclone.conf"),
	})
	return file, effective, nil
}

// flagNames maps configuration keys to their command-line flag names.
var flagNames = map[string]string{
	"state":             "state",
	"rclone_config":     "rclone-config",
	"scaena_url":        "scaena-url",
	"scaena_token_file": "scaena-token-file",
	"remote_url":        "remote-url",
	"remote_token_file": "remote-token-file",
	"local_root":        "local-root",
}

// installRclone points the embedded rclone at the configured config file.
func (c *cli) installRclone() error {
	configfile.Install()
	return rcloneconfig.SetConfigPath(c.effective.Values["rclone_config"])
}

// openService takes the process lock and opens the state exactly like the
// pre-cobra flat CLI did; the returned release func drops both.
func (c *cli) openService(enabled bool) (*bridge.Service, func(), error) {
	state := c.effective.Values["state"]
	if err := os.MkdirAll(state, 0700); err != nil {
		return nil, nil, err
	}
	lock, err := os.OpenFile(filepath.Join(state, "process.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return nil, nil, err
	}
	if err = lockFile(lock, lockExclusive|lockNonBlocking); err != nil {
		lock.Close()
		return nil, nil, err
	}
	if err = c.installRclone(); err != nil {
		lock.Close()
		return nil, nil, err
	}
	release := func() {
		_ = lockFile(lock, lockRelease)
		lock.Close()
	}
	// Third-party raw errors can contain bearer URLs. Only our typed error
	// codes are rendered; SDK diagnostics never enter stdout/stderr.
	log.SetOutput(io.Discard)
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	s, err := bridge.New(state, bridge.NewOneDrive(), enabled)
	if err != nil {
		release()
		return nil, nil, err
	}
	return s, func() { s.Close(); release() }, nil
}
