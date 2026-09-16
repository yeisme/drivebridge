package main

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/mark3labs/mcp-go/server"
	"github.com/pelletier/go-toml/v2"
	rcloneconfig "github.com/rclone/rclone/fs/config"
	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge/internal/bridge"
	dbconfig "github.com/yeisme/drivebridge/internal/config"
	remoteclient "github.com/yeisme/drivebridge/internal/remote"
)

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
	var enabled, handoff, usage bool
	var scaenaBase, scaenaTokenFile, remoteURL, remoteToken, localRoot string
	cmd := &cobra.Command{
		Use:   "mcp",
		Short: "Serve the local stdio MCP (register this with an MCP client).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if c.mode.JSON || c.mode.Agent || c.mode.Events || c.mode.Explain {
				return fail("mcp_owns_output", "The stdio MCP server owns the process output; drop the output-mode flag.")
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
			if !enabled {
				note := "write tools (upload/download/mkdir/trash/restore/purge/sync/save_output) will return disabled"
				if handoff {
					note += "; handoff also requires --enabled"
				}
				fmt.Fprintln(os.Stderr, "drivebridge: --enabled not set; "+note+".")
			}
			defer release()
			if base := c.effective.Values["scaena_url"]; base != "" {
				tokenFile := c.effective.Values["scaena_token_file"]
				if s.Output, err = bridge.NewScaenaOutput(base, scaenaTokenReader(tokenFile)); err != nil {
					return err
				}
			}
			if usage {
				s.StartInsights()
				srv, e := s.ListenInsights(filepath.Join(c.effective.Values["state"], "insights.sock"))
				if e != nil {
					return e
				}
				defer func() {
					ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
					defer cancel()
					srv.Shutdown(ctx)
				}()
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
	cmd.Flags().BoolVar(&usage, "insights", false, "Enable the private insights control socket and opted-in daily scans")
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
				return fail("remote_url_required", "Remote mode needs --remote-url or the remote_url configuration key.")
			}
			var args map[string]any
			if action != "capabilities" {
				args = map[string]any{}
				if json.NewDecoder(io.LimitReader(os.Stdin, 1<<20)).Decode(&args) != nil {
					return fail("invalid_remote_arguments", "Remote tool arguments must be one JSON object on stdin.")
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
			return "", fail("credential_unavailable", "The credential file must exist, be a regular file, and disallow group/other access.")
		}
		b, e := os.ReadFile(path)
		if e != nil || len(b) > 8192 {
			return "", fail("credential_unavailable", "The credential file must exist, be a regular file, and disallow group/other access.")
		}
		return strings.TrimSpace(string(b)), nil
	}
}

func spaceAddCommand(c *cli, use string) *cobra.Command {
	var space, remote, backend, drive, rootID, rootPath, remotePath, local string
	var limit int64
	var project, actor string
	var followSymlinks, auto bool
	var rootName string
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
			if backend != "generic" {
				drive, root, err = discoverSpaceRoot(cmd.Context(), remote, rootPath, drive, rootID)
				if err != nil {
					return err
				}
			}
			if backend != "generic" && (auto || rootName != "") {
				if rootPath != "" || rootID != "" || (auto && rootName != "") {
					return fmt.Errorf("root_selection_conflict")
				}
				drive = rcloneconfig.GetValue(remote, "drive_id")
				if drive == "" {
					return fail("authentication_required", "Run drivebridge auth --auto first so the drive and root IDs are discoverable.")
				}
				if auto {
					if root = rcloneconfig.GetValue(remote, "root_folder_id"); root == "" {
						return fail("authentication_required", "Run drivebridge auth --auto first so the drive and root IDs are discoverable.")
					}
				} else if root, err = bridge.NewOneDrive().SelectRootFolderOnRemote(cmd.Context(), remote, rootName); err != nil {
					return err
				}
			}
			if backend == "generic" {
				root = remotePath
			}
			if err = s.AddSpace(bridge.Space{ID: space, Remote: remote, Backend: backend, DriveID: drive, RootID: root, LocalRoot: local, MaxBytes: limit, Project: project, Actor: actor, FollowSymlinks: followSymlinks}); err != nil {
				return err
			}
			action := "space-add"
			if use == "setup" {
				action = "setup"
			}
			return c.mode.write(action, "success", "Space configured; file roundtrip has not been verified.", map[string]any{"configured": true, "roundtrip_status": "not_verified", "space_id": space, "local_root": local, "max_bytes": limit})
		},
	}
	f := cmd.Flags()
	f.StringVar(&space, "space", "", "Space identifier")
	f.StringVar(&remote, "remote", "", "Existing rclone remote name")
	f.StringVar(&backend, "backend", "", "Backend kind: empty for OneDrive, or 'generic' for any transfer-only rclone remote")
	f.StringVar(&drive, "drive-id", "", "OneDrive drive ID")
	f.StringVar(&rootPath, "root-path", "", "Existing OneDrive folder path; discovers drive and folder IDs")
	f.BoolVar(&auto, "auto", false, "Use the drive and root discovered by 'drivebridge auth --auto'")
	f.StringVar(&rootName, "root-name", "", "Select the allowed root folder by exact name under the drive root")
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
	root.AddCommand(newSpaceTTLCommand(c))
	list := &cobra.Command{
		Use:   "list",
		Short: "List configured spaces and their backend kinds.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if url := c.effective.Values["remote_url"]; url != "" {
				return runRemoteCall(c, url, c.effective.Values["remote_token_file"], c.effective.Values["local_root"], "spaces", nil)
			}
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
				return fail("config_exists", "A configuration file already exists at the target path; edit it or choose another --config.")
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
	root.AddCommand(show, init, configSetCommand(c))
	return root
}

func newTransferCommands(c *cli) []*cobra.Command {
	upload := &cobra.Command{
		Use:   "upload",
		Short: "Upload a local file into a space (user terminal; no --enabled gate).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			space, _ := cmd.Flags().GetString("space")
			file, _ := cmd.Flags().GetString("file")
			dir, _ := cmd.Flags().GetString("dir")
			name, _ := cmd.Flags().GetString("name")
			key, _ := cmd.Flags().GetString("idempotency-key")
			if key == "" {
				key = "cli-" + stableKey(space+"\x00"+file+"\x00"+name)
			}
			if name == "" {
				name = filepath.Base(file)
			}
			s, release, err := c.openService(true)
			if err != nil {
				return err
			}
			defer release()
			caller, _ := cmd.Flags().GetString("caller")
			taskRef, _ := cmd.Flags().GetString("task-ref")
			var op bridge.Operation
			if caller == "" && taskRef == "" {
				// Without declared provenance the upload may reuse content
				// that is already stored in the space (content-addressed).
				op, err = s.ReuseUpload(context.Background(), space, dir, file, name, key)
			} else {
				op, err = s.Upload(context.Background(), space, dir, file, name, key, bridge.Provenance{Caller: caller, TaskRef: taskRef})
			}
			if err != nil {
				return err
			}
			// The CLI is synchronous for the user: wait for the terminal state
			// instead of leaving the transfer behind in a dying process.
			stream := c.stream("upload")
			op = awaitTerminalProgress(s, op.ID, stream.progress)
			if op.Status == "completed" {
				return stream.end("success", fmt.Sprintf("Uploaded %s -> %s.", op.ID, op.FileRef), op)
			}
			return stream.fail(terminalCode(op), fmt.Sprintf("Operation %s stopped: %s (%d/%d bytes).", op.ID, op.Status, op.Transferred, op.Bytes), map[string]any{"operation": op})
		},
	}
	upload.Flags().String("caller", "", "Caller label (opaque identifier, not authentication)")
	upload.Flags().String("task-ref", "", "External task identifier; no prompts, URLs or credentials")
	upload.Flags().String("space", "", "Space identifier")
	upload.Flags().String("file", "", "Local file path inside the space's local root or absolute")
	upload.Flags().String("dir", "", "Remote directory inside the space (default root)")
	upload.Flags().String("name", "", "Destination name (default: file base name)")
	upload.Flags().String("idempotency-key", "", "Stable idempotency key (default: generated)")

	download := &cobra.Command{
		Use:   "download",
		Short: "Download a stable reference into the allowed local root.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ref, _ := cmd.Flags().GetString("ref")
			out, _ := cmd.Flags().GetString("out")
			key, _ := cmd.Flags().GetString("idempotency-key")
			if key == "" {
				key = "cli-" + stableKey(ref+"\x00"+out)
			}
			s, release, err := c.openService(true)
			if err != nil {
				return err
			}
			defer release()
			op, err := s.Download(context.Background(), ref, out, key)
			if err != nil {
				return err
			}
			stream := c.stream("download")
			op = awaitTerminalProgress(s, op.ID, stream.progress)
			if op.Status == "completed" {
				return stream.end("success", fmt.Sprintf("Downloaded via %s.", op.ID), op)
			}
			return stream.fail(terminalCode(op), fmt.Sprintf("Operation %s stopped: %s (%d/%d bytes).", op.ID, op.Status, op.Transferred, op.Bytes), map[string]any{"operation": op})
		},
	}
	download.Flags().String("ref", "", "Stable file reference")
	download.Flags().String("out", "", "Local target path inside the space's local root")
	download.Flags().String("idempotency-key", "", "Stable idempotency key (default: generated)")

	list := &cobra.Command{
		Use:   "ls",
		Short: "List files in a space root or workspace directory.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			space, _ := cmd.Flags().GetString("space")
			path, _ := cmd.Flags().GetString("path")
			s, release, err := c.openService(true)
			if err != nil {
				return err
			}
			defer release()
			files, err := s.List(context.Background(), space, path)
			if err != nil {
				return err
			}
			if !(c.mode.JSON || c.mode.Agent || c.mode.Events || c.mode.Explain) {
				for _, f := range files {
					fmt.Printf("%-40s %10d  %s\n", f.Name, f.Size, f.Ref)
				}
			}
			return c.mode.write("ls", "success", fmt.Sprintf("%d file(s).", len(files)), map[string]any{"count": len(files), "files": files})
		},
	}
	list.Flags().String("space", "", "Space identifier")
	list.Flags().String("path", "", "Workspace directory (default root)")

	stat := &cobra.Command{
		Use:   "stat",
		Short: "Verify a stable reference and report identity, version and digest.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ref, _ := cmd.Flags().GetString("ref")
			s, release, err := c.openService(true)
			if err != nil {
				return err
			}
			defer release()
			f, err := s.Stat(context.Background(), ref)
			if err != nil {
				return err
			}
			return c.mode.write("stat", "success", fmt.Sprintf("%s %d bytes version=%s", f.Name, f.Size, f.Version), f)
		},
	}
	stat.Flags().String("ref", "", "Stable file reference")

	op := &cobra.Command{
		Use:   "operation",
		Short: "Report an operation's status and received bytes.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, _ := cmd.Flags().GetString("id")
			o, err := func() (bridge.Operation, error) {
				s, release, err := c.openService(true)
				if err != nil {
					// Inspection falls back to the read-only snapshot when a
					// registered MCP client holds the state.
					var f *cliFailure
					if errors.As(err, &f) && f.code == "state_locked" {
						snap, e := openSnapshot(c.effective.Values["state"])
						if e != nil {
							return bridge.Operation{}, err
						}
						defer snap.close()
						return snap.operation(id)
					}
					return bridge.Operation{}, err
				}
				defer release()
				return s.Get(id)
			}()
			if err != nil {
				return err
			}
			// The inspection itself succeeded; the operation's own state is
			// data, never the envelope status.
			status := "partial"
			if o.Status == "completed" {
				status = "success"
			}
			return c.mode.write("operation", status, fmt.Sprintf("%s %s %d/%d bytes.", o.ID, o.Status, o.Transferred, o.Bytes), o)
		},
	}
	op.Flags().String("id", "", "Operation ID")

	mkdir := &cobra.Command{
		Use:   "mkdir",
		Short: "Create a workspace directory (idempotent).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			space, _ := cmd.Flags().GetString("space")
			path, _ := cmd.Flags().GetString("path")
			s, release, err := c.openService(true)
			if err != nil {
				return err
			}
			defer release()
			if err = s.Mkdir(context.Background(), space, path); err != nil {
				return err
			}
			return c.mode.write("mkdir", "success", "Directory ready.", map[string]any{"space": space, "path": path})
		},
	}
	mkdir.Flags().String("space", "", "Space identifier")
	mkdir.Flags().String("path", "", "Workspace directory path")

	find := &cobra.Command{
		Use:   "find",
		Short: "Search the space index by name; verify hits with stat.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			space, _ := cmd.Flags().GetString("space")
			query, _ := cmd.Flags().GetString("query")
			path, _ := cmd.Flags().GetString("path")
			mime, _ := cmd.Flags().GetString("mime-type")
			afterText, _ := cmd.Flags().GetString("after")
			after := time.Time{}
			if afterText != "" {
				var e error
				after, e = time.Parse(time.RFC3339, afterText)
				if e != nil {
					return fmt.Errorf("invalid_time")
				}
			}
			// The index is a database projection: while a registered MCP
			// client holds the state, serve the same search from the
			// read-only snapshot instead of dying on the lock. Snapshot hits
			// stay advisory — stat is the verifier.
			readOnly := false
			files, err := func() ([]bridge.File, error) {
				s, release, err := c.openService(true)
				if err != nil {
					if isStateLocked(err) {
						db, e := openReadonlyModel(c.effective.Values["state"])
						if e != nil {
							return nil, err
						}
						sqlDB, _ := db.DB()
						defer sqlDB.Close()
						readOnly = true
						return snapshotSearch(db, space, query, path, mime, after)
					}
					return nil, err
				}
				defer release()
				return s.Search(space, query, path, bridge.SearchOptions{MIME: mime, After: after})
			}()
			if err != nil {
				return err
			}
			if !(c.mode.JSON || c.mode.Agent || c.mode.Events || c.mode.Explain) {
				for _, f := range files {
					fmt.Printf("%-40s %10d  %s\n", f.Name, f.Size, f.Ref)
				}
			}
			summary := fmt.Sprintf("%d hit(s).", len(files))
			data := map[string]any{"count": len(files), "files": files}
			if readOnly {
				summary = fmt.Sprintf("%d hit(s) from the read-only index snapshot (an MCP client holds the state); verify hits with stat.", len(files))
				data["read_only"] = true
			}
			return c.mode.write("find", "success", summary, data)
		},
	}
	find.Flags().String("mime-type", "", "Exact MIME type filter")
	find.Flags().String("after", "", "Observed since RFC3339 time (index observation, not remote modification)")
	find.Flags().String("space", "", "Space identifier")
	find.Flags().String("query", "", "Case-insensitive name substring")
	find.Flags().String("path", "", "Restrict to one workspace directory")

	feed := &cobra.Command{
		Use:   "changes",
		Short: "Poll incremental events since a cursor (0 = current position).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			space, _ := cmd.Flags().GetString("space")
			since, _ := cmd.Flags().GetUint64("since")
			// The events feed is a database projection: serve it from the
			// read-only snapshot while a registered MCP client holds the
			// state, with cursor semantics preserved.
			readOnly := false
			events, cursor, err := func() ([]bridge.Event, uint64, error) {
				s, release, err := c.openService(true)
				if err != nil {
					if isStateLocked(err) {
						db, e := openReadonlyModel(c.effective.Values["state"])
						if e != nil {
							return nil, 0, err
						}
						sqlDB, _ := db.DB()
						defer sqlDB.Close()
						readOnly = true
						return snapshotChanges(db, space, since)
					}
					return nil, 0, err
				}
				defer release()
				return s.Changes(space, since)
			}()
			if err != nil {
				return err
			}
			if !(c.mode.JSON || c.mode.Agent || c.mode.Events || c.mode.Explain) {
				for _, ev := range events {
					fmt.Printf("%d %s %s %s\n", ev.ID, ev.Kind, ev.Ref, ev.Detail)
				}
			}
			summary := fmt.Sprintf("%d event(s); cursor=%d.", len(events), cursor)
			data := map[string]any{"events": events, "cursor": cursor}
			if readOnly {
				summary = fmt.Sprintf("%d event(s); cursor=%d (read-only snapshot; an MCP client holds the state).", len(events), cursor)
				data["read_only"] = true
			}
			return c.mode.write("changes", "success", summary, data)
		},
	}
	feed.Flags().String("space", "", "Space identifier")
	feed.Flags().Uint64("since", 0, "Cursor from a previous changes call")

	trashCmd := &cobra.Command{
		Use:   "trash",
		Short: "Soft-delete a reference inside its space (restorable).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ref, _ := cmd.Flags().GetString("ref")
			s, release, err := c.openService(true)
			if err != nil {
				return err
			}
			defer release()
			f, err := s.Trash(context.Background(), ref)
			if err != nil {
				return err
			}
			return c.mode.write("trash", "success", "Reference trashed; restore with 'drivebridge restore'.", f)
		},
	}
	trashCmd.Flags().String("ref", "", "Stable file reference")

	restore := &cobra.Command{
		Use:   "restore",
		Short: "Move a trashed reference back (same-name conflicts get a suffix).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ref, _ := cmd.Flags().GetString("ref")
			s, release, err := c.openService(true)
			if err != nil {
				return err
			}
			defer release()
			f, err := s.Restore(context.Background(), ref)
			if err != nil {
				return err
			}
			return c.mode.write("restore", "success", "Reference restored.", f)
		},
	}
	restore.Flags().String("ref", "", "Stable file reference")

	purge := &cobra.Command{
		Use:   "purge",
		Short: "Permanently delete an already-trashed reference (owner-only).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			ref, _ := cmd.Flags().GetString("ref")
			owner, _ := cmd.Flags().GetBool("i-am-the-owner")
			if !owner {
				return fail("owner_confirmation_required", "Permanent deletion requires --i-am-the-owner; trash first, purge stays owner-terminal only.")
			}
			s, release, err := c.openService(true)
			if err != nil {
				return err
			}
			defer release()
			if err = s.Purge(context.Background(), ref); err != nil {
				return err
			}
			return c.mode.write("purge", "success", "Reference permanently deleted.", nil)
		},
	}
	purge.Flags().String("ref", "", "Already-trashed stable reference")
	purge.Flags().Bool("i-am-the-owner", false, "Explicit human-owner confirmation; permanent deletion never runs through the MCP surface")

	commands := []*cobra.Command{upload, download, list, stat, op, mkdir, find, feed, trashCmd, restore, purge}
	for _, cmd := range commands {
		routeTransfer(c, cmd)
	}
	return commands
}

// newInstallCommand registers the stdio MCP server with a local client. It
// writes only the registration entry; secrets are never involved.
func newInstallCommand(c *cli) *cobra.Command {
	var client string
	var readOnly bool
	cmd := &cobra.Command{
		Use:   "install",
		Short: "Register the local stdio MCP server with an MCP client.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			binary, err := os.Executable()
			if err != nil {
				return err
			}
			binary = mustAbs(binary)
			if readOnly && c.effective.Values["remote_url"] != "" {
				return fmt.Errorf("remote_read_only_requires_scoped_token")
			}
			args := []string{"mcp"}
			if !readOnly {
				args = append(args, "--enabled", "--handoff")
			}
			// Persist resolved locations, not process-local environment assumptions.
			for _, key := range []string{"state", "rclone_config", "remote_url", "remote_token_file", "local_root", "scaena_url", "scaena_token_file"} {
				if value := c.effective.Values[key]; value != "" {
					if key != "remote_url" && key != "scaena_url" {
						absolute, err := filepath.Abs(value)
						if err != nil {
							return fmt.Errorf("invalid_config_path")
						}
						value = absolute
					}
					args = append(args, "--"+flagNames[key], value)
				}
			}
			switch client {
			case "claude-code":
				if _, err := exec.LookPath("claude"); err != nil {
					return c.mode.write("install", "manual", "Client CLI unavailable; register the command and args from --json in your MCP client.", map[string]any{"client": client, "command": binary, "args": args})
				}
				_, err := exec.Command("claude", append([]string{"mcp", "add", "-s", "user", "drivebridge", "--", binary}, args...)...).CombinedOutput()
				if err != nil {
					return fail("registration_failed", "The claude CLI rejected the registration; register the printed command manually.")
				}
				return c.mode.write("install", "success", "Registered drivebridge with Claude Code (user scope).", map[string]any{"client": client, "command": binary})
			case "codex":
				home, err := os.UserHomeDir()
				if err != nil {
					return err
				}
				target := filepath.Join(home, ".codex", "config.toml")
				if body, err := os.ReadFile(target); err == nil {
					var conf struct {
						Servers map[string]struct {
							Command string   `toml:"command"`
							Args    []string `toml:"args"`
						} `toml:"mcp_servers"`
					}
					if toml.Unmarshal(body, &conf) != nil {
						return fmt.Errorf("invalid_client_config")
					}
					if existing, ok := conf.Servers["drivebridge"]; ok {
						if existing.Command != binary || !reflect.DeepEqual(existing.Args, args) {
							return fmt.Errorf("registration_conflict: existing entry differs; review client configuration")
						}
						return c.mode.write("install", "success", "Existing registration matches.", map[string]any{"client": client, "path": target})
					}
				} else if !os.IsNotExist(err) {
					return fmt.Errorf("client_config_unavailable")
				}
				quoted := make([]string, len(args))
				for i, arg := range args {
					quoted[i] = strconv.Quote(arg)
				}
				entry := "\n[mcp_servers.drivebridge]\ncommand = " + strconv.Quote(binary) + "\nargs = [" + strings.Join(quoted, ", ") + "]\n"
				if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
					return err
				}
				f, err := os.OpenFile(target, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
				if err != nil {
					return err
				}
				defer f.Close()
				if _, err = f.WriteString(entry); err != nil {
					return err
				}
				return c.mode.write("install", "success", "Appended drivebridge to "+target+".", map[string]any{"client": client, "path": target})
			case "generic":
				return c.mode.write("install", "manual", "Register this stdio command with your MCP client:", map[string]any{"command": binary, "args": args})
			}
			return fail("unknown_client", "Unknown client; choose claude-code, codex or generic.")
		},
	}
	cmd.Flags().BoolVar(&readOnly, "read-only", false, "Register without enabling local file operations or the owner socket")
	cmd.Flags().StringVar(&client, "client", "claude-code", "MCP client: claude-code, codex or generic")
	return cmd
}

// awaitTerminal polls an operation until it leaves the queued/transferring
// states; interrupted CLI runs surface the state for an explicit retry.
func awaitTerminal(s *bridge.Service, id string) bridge.Operation {
	return awaitTerminalProgress(s, id, nil)
}

// awaitTerminalProgress polls an operation until a terminal state. Progress
// is reported through onProgress at a bounded cadence (at most once per
// second, or when at least 5% more bytes moved) so --events streams and the
// human terminal show transfer life instead of silence.
func awaitTerminalProgress(s *bridge.Service, id string, onProgress func(bridge.Operation)) bridge.Operation {
	var last time.Time
	var lastBytes int64
	for {
		op, err := s.Get(id)
		if err != nil {
			return bridge.Operation{ID: id, Status: "operation_not_found"}
		}
		switch op.Status {
		case "completed", "failed", "cancelled", "interrupted":
			return op
		}
		if onProgress != nil {
			now := time.Now()
			moved := op.Bytes > 0 && op.Transferred-lastBytes >= op.Bytes/20
			if now.Sub(last) >= time.Second && (last.IsZero() || moved) {
				onProgress(op)
				last, lastBytes = now, op.Transferred
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// stableKey derives a deterministic CLI idempotency key so retries of the
// same command reuse the original operation instead of creating a new one.
func stableKey(v string) string {
	sum := sha256.Sum256([]byte(v))
	return hex.EncodeToString(sum[:8])
}
func mustAbs(p string) string {
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}
func firstLine(b []byte) string {
	if i := bytes.IndexByte(b, '\n'); i >= 0 {
		return string(b[:i])
	}
	return string(b)
}
