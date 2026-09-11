package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/yeisme/drivebridge/internal/bridge"
	remoteclient "github.com/yeisme/drivebridge/internal/remote"
)

func main() {
	if err := run(); err != nil {
		if !errors.Is(err, errDoctorNotReady) {
			fmt.Fprintln(os.Stderr, "DriveBridge failed. Check local configuration and the documented recovery steps.")
		}
		os.Exit(1)
	}
}
func run() (err error) {
	if len(os.Args) < 2 || os.Args[1] == "--help" || os.Args[1] == "-h" {
		fmt.Println("Usage: drivebridge <auth|space-add|mcp|capabilities|doctor|remote> [options]")
		return nil
	}
	flags := flag.NewFlagSet(os.Args[1], flag.ContinueOnError)
	mode := outputMode{}
	flags.BoolVar(&mode.JSON, "json", false, "Print the machine envelope")
	flags.BoolVar(&mode.Agent, "agent", false, "Print stable key=value status")
	flags.BoolVar(&mode.Events, "events", false, "Print start/end NDJSON events")
	flags.BoolVar(&mode.Explain, "explain", false, "Explain the local result")
	defer func() {
		if err != nil && !errors.Is(err, errDoctorNotReady) && (mode.JSON || mode.Agent || mode.Events || mode.Explain) {
			_ = mode.write(os.Args[1], "failed", "Command failed; check configuration and recovery instructions.", nil)
		}
	}()

	home, err := os.UserConfigDir()
	if err != nil {
		return err
	}
	remoteURL := flags.String("remote-url", "", "Remote DriveBridge service origin")
	remoteToken := flags.String("remote-token-file", "", "Private scoped Agent token file")
	remoteAction := flags.String("action", "capabilities", "Remote CLI action; JSON arguments are read from stdin")
	state := flags.String("state", filepath.Join(home, "drivebridge"), "Private local state directory")
	rcloneConfig := flags.String("rclone-config", filepath.Join(home, "rclone", "rclone.conf"), "Existing rclone configuration; credentials are never printed")
	scaenaBase := flags.String("scaena-url", "", "Scaena owner URL for output recovery")
	scaenaTokenFile := flags.String("scaena-token-file", "", "Private file containing the existing Scaena API credential")
	enabled := flags.Bool("enabled", false, "Explicitly enable file operations")
	handoff := flags.Bool("handoff", false, "Enable a private local owner socket; requires --enabled")
	space := flags.String("space", "", "Space identifier")
	remote := flags.String("remote", "", "Existing OneDrive remote name")
	drive := flags.String("drive-id", "", "OneDrive drive ID")
	root := flags.String("root-id", "", "Allowed root item ID")
	local := flags.String("local-root", "", "Allowed absolute local directory")
	limit := flags.Int64("max-bytes", 0, "Explicit byte limit")
	project := flags.String("project", "", "Authorized Scaena project reference")
	actor := flags.String("actor", "", "Authorized Scaena actor reference")
	if err = flags.Parse(os.Args[2:]); err != nil {
		return err
	}
	// doctor is read-only: it must not create state, take the process lock, or
	// open the database for writes, so it branches before both.
	if os.Args[1] == "doctor" {
		return runDoctor(mode, doctorConfig{
			State: *state, RcloneConfig: *rcloneConfig, ScaenaURL: *scaenaBase, ScaenaTokenFile: *scaenaTokenFile,
			Handoff: *handoff, RemoteURL: *remoteURL, RemoteTokenFile: *remoteToken, LocalRoot: *local,
		})
	}
	if *remoteURL != "" {
		adapter, e := remoteclient.New(*remoteURL, *remoteToken, *local)
		if e != nil {
			return e
		}
		defer adapter.Close()
		if os.Args[1] == "mcp" {
			return server.ServeStdio(adapter.MCP())
		}
		action := *remoteAction
		if os.Args[1] == "capabilities" {
			action = "capabilities"
		} else if os.Args[1] != "remote" {
			return fmt.Errorf("unsupported remote command")
		}
		args := map[string]any{}
		if action != "capabilities" {
			if json.NewDecoder(io.LimitReader(os.Stdin, 1<<20)).Decode(&args) != nil {
				return fmt.Errorf("invalid remote arguments")
			}
		}
		out, e := adapter.Call(context.Background(), action, args)
		if e != nil {
			return e
		}
		return mode.write("remote."+action, "success", "Remote operation completed.", out)
	}
	if err = os.MkdirAll(*state, 0700); err != nil {
		return err
	}
	lock, err := os.OpenFile(filepath.Join(*state, "process.lock"), os.O_CREATE|os.O_RDWR, 0600)
	if err != nil {
		return err
	}
	defer lock.Close()
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		return err
	}
	defer syscall.Flock(int(lock.Fd()), syscall.LOCK_UN)
	configfile.Install()
	if err = config.SetConfigPath(*rcloneConfig); err != nil {
		return err
	}
	if os.Args[1] == "auth" {
		return config.EditConfig(context.Background())
	}
	// Third-party raw errors can contain bearer URLs. Only our typed error codes
	// are rendered; SDK diagnostics never enter stdout/stderr.
	log.SetOutput(io.Discard)
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	s, err := bridge.New(*state, bridge.NewOneDrive(), *enabled)
	if err != nil {
		return err
	}
	defer s.Close()
	if *scaenaBase != "" {
		s.Output, err = bridge.NewScaenaOutput(*scaenaBase, func() (string, error) {
			if *scaenaTokenFile == "" {
				return os.Getenv("SCAENA_API_TOKEN"), nil
			}
			info, e := os.Lstat(*scaenaTokenFile)
			if e != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
				return "", fmt.Errorf("credential file unavailable")
			}
			b, e := os.ReadFile(*scaenaTokenFile)
			if e != nil || len(b) > 8192 {
				return "", fmt.Errorf("credential file unavailable")
			}
			return strings.TrimSpace(string(b)), nil
		})
		if err != nil {
			return err
		}
	}
	switch os.Args[1] {
	case "space-add":
		if err = s.AddSpace(bridge.Space{ID: *space, Remote: *remote, DriveID: *drive, RootID: *root, LocalRoot: *local, MaxBytes: *limit, Project: *project, Actor: *actor}); err != nil {
			return err
		}
		return mode.write("space-add", "success", "Space configured.", nil)
	case "capabilities":
		return mode.write("capabilities", "success", fmt.Sprintf("DriveBridge enabled=%t; %d configured spaces. Use --json for schemas and limits.", s.Enabled, len(s.Spaces())), s.Capabilities())
	case "mcp":
		if mode.JSON || mode.Agent || mode.Events || mode.Explain {
			return fmt.Errorf("stdio MCP owns output")
		}
		s.Handoff = *handoff
		if *handoff {
			socket := filepath.Join(*state, "handoff.sock")
			srv, e := s.Listen(socket)
			if e != nil {
				return e
			}
			defer func() {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				_ = srv.Shutdown(ctx)
			}()
		}
		return server.ServeStdio(s.MCP())
	default:
		return fmt.Errorf("unknown command")
	}
}
