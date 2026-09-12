package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/server"
	rcloneconfig "github.com/rclone/rclone/fs/config"
	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge/internal/bridge"
	dbconfig "github.com/yeisme/drivebridge/internal/config"
	remoteclient "github.com/yeisme/drivebridge/internal/remote"
)

func newAuthCommand(c *cli) *cobra.Command {
	return &cobra.Command{
		Use:   "auth",
		Short: "Open the embedded rclone configurator to sign in to a backend.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := c.installRclone(); err != nil {
				return err
			}
			// The configurator is interactive by design; it runs in the user
			// terminal only and never prints through the output contract.
			return rcloneconfig.EditConfig(context.Background())
		},
	}
}

func newCapabilitiesCommand(c *cli) *cobra.Command {
	var remoteURL, remoteToken, localRoot string
	cmd := &cobra.Command{
		Use:   "capabilities",
		Short: "Report enabled state, spaces, tool schemas and limits.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if url := c.effective.Values["remote_url"]; url != "" {
				return runRemoteCall(c, url, c.effective.Values["remote_token_file"], c.effective.Values["local_root"], "capabilities", nil)
			}
			s, release, err := c.openService(false)
			if err != nil {
				return err
			}
			defer release()
			return c.mode.write("capabilities", "success", fmt.Sprintf("DriveBridge enabled=%t; %d configured spaces. Use --json for schemas and limits.", s.Enabled, len(s.Spaces())), s.Capabilities())
		},
	}
	cmd.Flags().StringVar(&remoteURL, "remote-url", "", "Remote DriveBridge service origin")
	cmd.Flags().StringVar(&remoteToken, "remote-token-file", "", "Private scoped Agent token file")
	cmd.Flags().StringVar(&localRoot, "local-root", "", "Allowed absolute local directory")
	return cmd
}

func newMCPCommand(c *cli) *cobra.Command {
	var enabled, handoff bool
	var scaenaBase, scaenaTokenFile, remoteURL, remoteToken, localRoot string
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Serve the local stdio MCP (register this with an MCP client).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if c.mode.JSON || c.mode.Agent || c.mode.Events || c.mode.Explain {
				return fmt.Errorf("stdio MCP owns output")
			}
			if url := c.effective.Values["remote_url"]; url != "" {
				adapter, err := remoteclient.New(url, c.effective.Values["remote_token_file"], c.effective.Values["local_root"])
				if err != nil {
					return err
				}
				defer adapter.Close()
				return server.ServeStdio(adapter.MCP())
			}
			s, release, err := c.openService(enabled)
			if err != nil {
				return err
			}
			defer release()
			if base := c.effective.Values["scaena_url"]; base != "" {
				tokenFile := c.effective.Values["scaena_token_file"]
				if s.Output, err = bridge.NewScaenaOutput(base, scaenaTokenReader(tokenFile)); err != nil {
					return err
				}
			}
			s.Handoff = handoff
			if handoff {
				socket := filepath.Join(c.effective.Values["state"], "handoff.sock")
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
		},
	}
	cmd.Flags().BoolVar(&enabled, "enabled", false, "Explicitly enable file operations")
	cmd.Flags().BoolVar(&handoff, "handoff", false, "Enable a private local owner socket; requires --enabled")
	cmd.Flags().StringVar(&scaenaBase, "scaena-url", "", "Scaena owner URL for output recovery")
	cmd.Flags().StringVar(&scaenaTokenFile, "scaena-token-file", "", "Private file containing the existing Scaena API credential")
	cmd.Flags().StringVar(&remoteURL, "remote-url", "", "Remote DriveBridge service origin")
	cmd.Flags().StringVar(&remoteToken, "remote-token-file", "", "Private scoped Agent token file")
	cmd.Flags().StringVar(&localRoot, "local-root", "", "Allowed absolute local directory")
	return cmd
}

func newRemoteCommand(c *cli) *cobra.Command {
	var remoteURL, remoteToken, localRoot, action string
	cmd := &cobra.Command{
		Use:   "remote",
		Short: "Call a remote personal-space tool; JSON arguments are read from stdin.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			url := c.effective.Values["remote_url"]
			if url == "" {
				return fmt.Errorf("remote-url required")
			}
			var args map[string]any
			if action != "capabilities" {
				args = map[string]any{}
				if json.NewDecoder(io.LimitReader(os.Stdin, 1<<20)).Decode(&args) != nil {
					return fmt.Errorf("invalid remote arguments")
				}
			}
			return runRemoteCall(c, url, c.effective.Values["remote_token_file"], c.effective.Values["local_root"], action, args)
		},
	}
	cmd.Flags().StringVar(&remoteURL, "remote-url", "", "Remote DriveBridge service origin")
	cmd.Flags().StringVar(&remoteToken, "remote-token-file", "", "Private scoped Agent token file")
	cmd.Flags().StringVar(&localRoot, "local-root", "", "Allowed absolute local directory")
	cmd.Flags().StringVar(&action, "action", "capabilities", "Remote CLI action; JSON arguments are read from stdin")
	return cmd
}

func runRemoteCall(c *cli, url, tokenFile, localRoot, action string, args map[string]any) error {
	adapter, err := remoteclient.New(url, tokenFile, localRoot)
	if err != nil {
		return err
	}
	defer adapter.Close()
	out, err := adapter.Call(context.Background(), action, args)
	if err != nil {
		return err
	}
	return c.mode.write("remote."+action, "success", "Remote operation completed.", out)
}

func scaenaTokenReader(path string) func() (string, error) {
	return func() (string, error) {
		if path == "" {
			return os.Getenv("SCAENA_API_TOKEN"), nil
		}
		info, e := os.Lstat(path)
		if e != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
			return "", fmt.Errorf("credential file unavailable")
		}
		b, e := os.ReadFile(path)
		if e != nil || len(b) > 8192 {
			return "", fmt.Errorf("credential file unavailable")
		}
		return strings.TrimSpace(string(b)), nil
	}
}

func spaceAddCommand(c *cli, use string) *cobra.Command {
	var space, remote, backend, drive, rootID, remotePath, local string
	var limit int64
	var project, actor string
	var followSymlinks bool
	cmd := &cobra.Command{
		Use:   use,
		Short: "Register a space bound to an allowed local directory and backend root.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, release, err := c.openService(false)
			if err != nil {
				return err
			}
			defer release()
			root := rootID
			if backend == "generic" {
				root = remotePath
			}
			if err = s.AddSpace(bridge.Space{ID: space, Remote: remote, Backend: backend, DriveID: drive, RootID: root, LocalRoot: local, MaxBytes: limit, Project: project, Actor: actor, FollowSymlinks: followSymlinks}); err != nil {
				return err
			}
			return c.mode.write("space-add", "success", "Space configured.", nil)
		},
	}
	f := cmd.Flags()
	f.StringVar(&space, "space", "", "Space identifier")
	f.StringVar(&remote, "remote", "", "Existing rclone remote name")
	f.StringVar(&backend, "backend", "", "Backend kind: empty for OneDrive, or 'generic' for any transfer-only rclone remote")
	f.StringVar(&drive, "drive-id", "", "OneDrive drive ID")
	f.StringVar(&rootID, "root-id", "", "Allowed root item ID")
	f.StringVar(&remotePath, "remote-path", "", "Generic backend root path inside the remote (empty = remote root)")
	f.BoolVar(&followSymlinks, "follow-symlinks", false, "Materialize symbolic links on upload for this space")
	f.StringVar(&local, "local-root", "", "Allowed absolute local directory")
	f.Int64Var(&limit, "max-bytes", 0, "Explicit byte limit")
	f.StringVar(&project, "project", "", "Authorized Scaena project reference")
	f.StringVar(&actor, "actor", "", "Authorized Scaena actor reference")
	return cmd
}

func newSpaceAddCommand(c *cli) *cobra.Command {
	cmd := spaceAddCommand(c, "space-add")
	cmd.Hidden = true
	cmd.Deprecated = "use 'space add'"
	return cmd
}

func newSpaceCommand(c *cli) *cobra.Command {
	root := &cobra.Command{Use: "space", Short: "Manage configured spaces."}
	root.AddCommand(spaceAddCommand(c, "add"))
	list := &cobra.Command{
		Use:   "list",
		Short: "List configured spaces and their backend kinds.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, release, err := c.openService(false)
			if err != nil {
				return err
			}
			defer release()
			spaces := s.Spaces()
			data := make([]map[string]any, 0, len(spaces))
			for _, p := range spaces {
				data = append(data, map[string]any{"id": p.ID, "backend": p.Kind(), "local_root": p.LocalRoot, "max_bytes": p.MaxBytes, "follow_symlinks": p.FollowSymlinks, "save_output": p.Project != "" && p.Actor != ""})
			}
			summary := fmt.Sprintf("%d configured space(s).", len(spaces))
			if !(c.mode.JSON || c.mode.Agent || c.mode.Events || c.mode.Explain) {
				for _, row := range data {
					fmt.Printf("%-16s %-10s %s\n", row["id"], row["backend"], row["local_root"])
				}
			}
			return c.mode.write("space.list", "success", summary, map[string]any{"count": len(spaces), "spaces": data})
		},
	}
	root.AddCommand(list)
	return root
}

func newConfigCommand(c *cli) *cobra.Command {
	root := &cobra.Command{Use: "config", Short: "Inspect or create the CLI configuration file."}
	show := &cobra.Command{
		Use:   "show",
		Short: "Print the resolved configuration and each key's source.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			values := map[string]any{"config_file": c.effective.Path}
			for _, key := range dbconfig.Keys {
				values[key] = c.effective.Values[key]
				values["source."+key] = string(c.effective.Sources[key])
			}
			if !(c.mode.JSON || c.mode.Agent || c.mode.Events || c.mode.Explain) {
				fmt.Println("config_file:", c.effective.Path)
				for _, key := range dbconfig.Keys {
					fmt.Printf("%-20s %s  (%s)\n", key, c.effective.Values[key], c.effective.Sources[key])
				}
				return nil
			}
			return c.mode.write("config.show", "success", "Resolved configuration; secrets never appear here.", values)
		},
	}
	init := &cobra.Command{
		Use:   "init",
		Short: "Write a commented starter configuration file (refuses to overwrite).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			home, err := os.UserConfigDir()
			if err != nil {
				return err
			}
			target := c.configPath
			if target == "" {
				target = filepath.Join(home, "drivebridge", "config.yaml")
			}
			if _, err := os.Stat(target); err == nil {
				return fmt.Errorf("config exists: %s", target)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return err
			}
			if err := os.WriteFile(target, []byte(dbconfig.Template()), 0600); err != nil {
				return err
			}
			return c.mode.write("config.init", "success", fmt.Sprintf("Wrote %s.", target), map[string]any{"path": target})
		},
	}
	show.Flags().String("local-root", "", "Resolve this key from a flag to preview precedence")
	root.AddCommand(show, init)
	return root
}
