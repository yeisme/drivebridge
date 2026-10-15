package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	serverconfig "github.com/yeisme/drivebridge-server/internal/config"
	"github.com/yeisme/drivebridge-server/internal/service"
)

func (c *cli) openService(enabled, development bool) (*service.Service, error) {
	cfg, err := c.serviceConfig(enabled, development)
	if err != nil {
		return nil, err
	}
	s, err := service.Open(cfg)
	if err != nil {
		return nil, err
	}
	log.SetOutput(io.Discard)
	slog.SetDefault(slog.New(slog.NewTextHandler(io.Discard, nil)))
	return s, nil
}

func newServeCommand(c *cli) *cobra.Command {
	var enabled, development, documentEditing, propertyEditing, automationManagement, shareReceivers, holdManagement, labelManagement, adminDirectWrite, cloudConnectors, transferPlans bool
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Run the HTTP server (application endpoints stay off without --enabled).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if c.mode.json || c.mode.agent || c.mode.explain {
				return fmt.Errorf("serve owns the process lifecycle; --json/--agent/--explain do not apply")
			}
			// The team DSN is read from the flag or environment only: it
			// never enters the resolved config, so no output surface can
			// echo it.
			c.teamDSN, _ = cmd.Flags().GetString("team-dsn")
			if c.teamDSN == "" {
				c.teamDSN = os.Getenv("DRIVEBRIDGE_TEAM_DSN")
			}
			// --events is the supervision stream: start, ready (after the
			// listener is bound), and end/error keep machine stdout clean
			// while logs stay on stderr.
			emitEvent := func(kind string, fields map[string]any) {}
			if c.mode.events {
				seq := 0
				emitEvent = func(kind string, fields map[string]any) {
					seq++
					event := map[string]any{"type": kind, "seq": seq, "spec_version": "1.0", "command": "drivebridge-server.serve"}
					for k, v := range fields {
						event[k] = v
					}
					_ = json.NewEncoder(os.Stdout).Encode(event)
				}
				emitEvent("start", nil)
			}
			s, err := c.openService(enabled, development)
			if err != nil {
				return err
			}
			defer s.Close()

			s.Config.AdminDirectWrite = adminDirectWrite
			s.Config.CloudConnectors = cloudConnectors
			s.Config.TransferPlans = transferPlans
			s.Config.DocumentEditing = documentEditing || s.Config.DocumentEditing
			s.Config.PropertyEditing = propertyEditing || s.Config.PropertyEditing
			s.Config.AutomationManagement = automationManagement || s.Config.AutomationManagement
			s.Config.ShareReceivers = shareReceivers || s.Config.ShareReceivers
			s.Config.HoldManagement = holdManagement || s.Config.HoldManagement
			s.Config.LabelManagement = labelManagement || s.Config.LabelManagement
			if enabled && c.effective.Values["issuer"] != "" {
				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				err = s.ConfigureLogin(ctx)
				cancel()
				if err != nil {
					return err
				}
			}
			srv := &http.Server{
				Handler:           s.Handler(),
				ReadHeaderTimeout: 10 * time.Second,
				ReadTimeout:       15 * time.Minute,
				WriteTimeout:      15 * time.Minute,
				IdleTimeout:       60 * time.Second,
				MaxHeaderBytes:    1 << 20,
			}
			listener, err := net.Listen("tcp", c.effective.Values["listen"])
			if err != nil {
				emitEvent("error", map[string]any{"error": map[string]string{"code": "listener_unavailable", "message": err.Error()}})
				return err
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			done := make(chan struct{})
			go func() {
				defer close(done)
				<-ctx.Done()
				shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
				defer cancel()
				srv.Shutdown(shutdown)
			}()
			emitEvent("progress", map[string]any{"stage": "ready", "listen": listener.Addr().String()})
			fmt.Fprintln(os.Stderr, "DriveBridge server ready.")
			err = srv.Serve(listener)
			if err == http.ErrServerClosed {
				<-done
				emitEvent("end", map[string]any{"status": "success"})
				return nil
			}
			if err != nil {
				emitEvent("error", map[string]any{"error": map[string]string{"code": "listener_unavailable", "message": "The listener failed while serving."}})
			}
			return err
		},
	}
	// Values are resolved through c.effective; the flag targets only exist so
	// cobra can parse and mark them.
	var origin, listen, issuer, clientID, clientSecret, msClient, msSecret, tenantName, tenantOrigin, tenantContact, webDir string
	var maxBytes int64
	f := cmd.Flags()
	f.String("service-identity-issuer", "", "Optional unified identity issuer; disabled when unset")
	f.String("service-identity-endpoint", "", "Optional trusted unified identity backend URL")
	f.StringVar(&origin, "origin", "", "Exact public origin")
	f.StringVar(&listen, "listen", "", "HTTP listener")
	f.StringVar(&issuer, "issuer", "", "OIDC issuer")
	f.StringVar(&clientID, "client-id", "", "OIDC client ID")
	f.StringVar(&clientSecret, "client-secret-file", "", "Private OIDC client secret file")
	f.StringVar(&msClient, "onedrive-client-id", "", "Microsoft connector client ID")
	f.StringVar(&msSecret, "onedrive-secret-file", "", "Private Microsoft connector secret file")
	f.StringVar(&tenantName, "tenant-name", "", "Deployment tenant display name (written at startup, read-only online)")
	f.StringVar(&tenantOrigin, "tenant-origin", "", "Deployment tenant public origin")
	f.StringVar(&tenantContact, "tenant-contact", "", "Deployment tenant contact channel")
	f.StringVar(&webDir, "web-dir", "", "Built web client directory")
	f.Int64Var(&maxBytes, "max-bytes", 0, "Per-user-space and file byte limit")
	f.String("team-dsn", "", "Team PostgreSQL metadata DSN (alternative: DRIVEBRIDGE_TEAM_DSN); never logged or echoed")
	f.BoolVar(&adminDirectWrite, "admin-direct-write", false, "Allow human administrators to write within their existing managed data scope")
	f.BoolVar(&cloudConnectors, "cloud-connectors", false, "Enable configured cloud connector operations")
	f.BoolVar(&transferPlans, "transfer-plans", false, "Enable human-confirmed cross-storage transfer plans")
	f.BoolVar(&enabled, "enabled", false, "Enable application endpoints")
	f.BoolVar(&development, "development", false, "Allow loopback HTTP for offline development only")
	f.BoolVar(&documentEditing, "document-editing", false, "Enable human-confirmed text and Markdown version saves")
	f.BoolVar(&propertyEditing, "property-editing", false, "Enable human-confirmed fixed binary version sidecar property edits")
	f.BoolVar(&automationManagement, "automation-management", false, "Enable human-confirmed automation previews, changes and execution receipts")
	f.BoolVar(&shareReceivers, "share-receivers", false, "Enable proof-based share sessions, receiver devices and download quotas")
	f.BoolVar(&holdManagement, "hold-management", false, "Enable human-confirmed retention hold creation and revision-checked release")
	f.BoolVar(&labelManagement, "label-management", false, "Enable human-confirmed label dictionary changes and bounded merge plans")
	return cmd
}

func newUsersCommand(c *cli) *cobra.Command {
	root := &cobra.Command{Use: "users", Short: "Manage operator-issued key users (no OAuth; keys appear once at issuance)."}
	importCmd := &cobra.Command{
		Use:   "import",
		Short: "Batch-create key users: JSON on stdin, or --csv for a name[,days[,admin[,key]]] table.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := c.openService(false, false)
			if err != nil {
				return err
			}
			defer s.Close()
			var entries []service.UserImport
			if csvMode, _ := cmd.Flags().GetBool("csv"); csvMode {
				raw, e := io.ReadAll(io.LimitReader(os.Stdin, 1<<20))
				if e != nil {
					return fmt.Errorf("invalid user batch")
				}
				entries, e = parseUserCSV(string(raw))
				if e != nil {
					return e
				}
			} else if json.NewDecoder(io.LimitReader(os.Stdin, 1<<20)).Decode(&entries) != nil {
				return fmt.Errorf("invalid user batch")
			}
			issued, err := s.ImportUsers(entries)
			if err != nil {
				return err
			}
			rows := make([]map[string]any, 0, len(issued))
			for _, v := range issued {
				rows = append(rows, map[string]any{"user_id": v.UserID, "name": v.Name, "key": v.Key, "admin": v.Admin, "expires_at": v.ExpiresAt})
			}
			out, _ := cmd.Flags().GetString("out")
			if out != "" {
				if e := writeIssuedCSV(out, issued); e != nil {
					return e
				}
			}
			if !(c.mode.json || c.mode.agent || c.mode.events || c.mode.explain) {
				for _, v := range issued {
					fmt.Printf("%s  %-20s %s%s\n", v.UserID, v.Name, v.Key, map[bool]string{true: "  (admin)"}[v.Admin])
				}
			}
			data := map[string]any{"count": len(issued), "issued": rows}
			if out != "" {
				data["out"] = out
			}
			return c.mode.emitData("users.import", "success", "Key users issued; plaintext keys appear only in this output. Store them now.", data)
		},
	}
	importCmd.Flags().Bool("csv", false, "Read a CSV table on stdin: header name,days,admin,key or positional name[,days[,admin[,key]]]; a single column is a plain name list")
	importCmd.Flags().String("out", "", "Write issued keys to a distribution CSV (0600): name,user_id,key,admin,expires_at")
	list := &cobra.Command{
		Use:   "list",
		Short: "List key users (lifecycle metadata only).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := c.openService(false, false)
			if err != nil {
				return err
			}
			defer s.Close()
			keys := s.ListUserKeys()
			rows := make([]map[string]any, 0, len(keys))
			for _, k := range keys {
				rows = append(rows, map[string]any{"id": k.ID, "user": k.Owner, "name": k.Name, "admin": k.Admin, "expires_at": k.Expires, "revoked": k.Revoked, "disabled": k.Disabled, "archived": k.Archived})
			}
			if !(c.mode.json || c.mode.agent || c.mode.events || c.mode.explain) {
				for _, k := range keys {
					expires := "never"
					if !k.Expires.IsZero() {
						expires = k.Expires.Format("2006-01-02")
					}
					fmt.Printf("%s  %-20s %-20s admin=%-5v revoked=%-5v disabled=%-5v expires=%s\n", k.ID, k.Owner, k.Name, k.Admin, k.Revoked, k.Disabled, expires)
				}
			}
			return c.mode.emitData("users.list", "success", fmt.Sprintf("%d key user(s).", len(keys)), map[string]any{"count": len(keys), "users": rows})
		},
	}
	revoke := &cobra.Command{
		Use:   "revoke",
		Short: "Revoke a key user by key id (next request is unauthorized).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, _ := cmd.Flags().GetString("id")
			if id == "" {
				return fmt.Errorf("missing --id")
			}
			s, err := c.openService(false, false)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.RevokeUserKey(id); err != nil {
				return err
			}
			return c.mode.emit("users.revoke", "success", "Key revoked.")
		},
	}
	revoke.Flags().String("id", "", "Key id from 'users list'")
	disable := &cobra.Command{
		Use:   "disable",
		Short: "Disable a login key without destroying it (sessions end; the key can be enabled later).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, _ := cmd.Flags().GetString("id")
			if id == "" {
				return fmt.Errorf("missing --id")
			}
			s, err := c.openService(false, false)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.SetUserKeyDisabled(id, true); err != nil {
				return err
			}
			return c.mode.emit("users.disable", "success", "Key disabled.")
		},
	}
	disable.Flags().String("id", "", "Key id from 'users list'")
	enable := &cobra.Command{
		Use:   "enable",
		Short: "Re-enable a previously disabled login key.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, _ := cmd.Flags().GetString("id")
			if id == "" {
				return fmt.Errorf("missing --id")
			}
			s, err := c.openService(false, false)
			if err != nil {
				return err
			}
			defer s.Close()
			if err := s.SetUserKeyDisabled(id, false); err != nil {
				return err
			}
			return c.mode.emit("users.enable", "success", "Key enabled.")
		},
	}
	enable.Flags().String("id", "", "Key id from 'users list'")
	rotate := &cobra.Command{
		Use:   "rotate",
		Short: "Atomically re-arm one user: mint a fresh key and revoke the old one.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			id, _ := cmd.Flags().GetString("id")
			if id == "" {
				return fmt.Errorf("missing --id")
			}
			days, _ := cmd.Flags().GetInt("days")
			s, err := c.openService(false, false)
			if err != nil {
				return err
			}
			defer s.Close()
			issued, err := s.RotateUserKey(id, days)
			if err != nil {
				return err
			}
			if !(c.mode.json || c.mode.agent || c.mode.events || c.mode.explain) {
				fmt.Printf("%s  %-20s %s%s\n", issued.UserID, issued.Name, issued.Key, map[bool]string{true: "  (admin)"}[issued.Admin])
			}
			return c.mode.emitData("users.rotate", "success", "Key rotated; the old key is revoked and the plaintext appears only in this output.", map[string]any{"user_id": issued.UserID, "name": issued.Name, "key": issued.Key, "admin": issued.Admin, "expires_at": issued.ExpiresAt})
		},
	}
	rotate.Flags().String("id", "", "Key id from 'users list'")
	rotate.Flags().Int("days", 0, "New key lifetime in days (0 keeps the remaining lifetime)")
	verify := &cobra.Command{
		Use:   "verify",
		Short: "Confirm one distributed key by its plaintext (local hash lookup, no network).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			key, _ := cmd.Flags().GetString("key")
			if key == "" {
				return fmt.Errorf("missing --key")
			}
			s, err := c.openService(false, false)
			if err != nil {
				return err
			}
			defer s.Close()
			k, err := s.VerifyUserKey(key)
			if err != nil {
				return err
			}
			state := "live"
			if k.Archived {
				state = "archived"
			} else if k.Revoked {
				state = "revoked"
			} else if k.Disabled {
				state = "disabled"
			}
			if !k.Expires.IsZero() && time.Now().After(k.Expires) && state == "live" {
				state = "expired"
			}
			if !(c.mode.json || c.mode.agent || c.mode.events || c.mode.explain) {
				fmt.Printf("%s  %-20s state=%s admin=%v expires=%s\n", k.ID, k.Name, state, k.Admin, k.Expires.Format("2006-01-02"))
			}
			return c.mode.emitData("users.verify", "success", fmt.Sprintf("Key %s is %s.", k.ID, state), map[string]any{"id": k.ID, "name": k.Name, "state": state, "admin": k.Admin, "expires_at": k.Expires, "revoked": k.Revoked, "disabled": k.Disabled, "archived": k.Archived})
		},
	}
	verify.Flags().String("key", "", "The plaintext key exactly as distributed")
	root.AddCommand(importCmd, list, revoke, disable, enable, rotate, verify)
	return root
}

func newPlatformConnectCommand(c *cli) *cobra.Command {
	var verifyIsolation, development bool
	cmd := &cobra.Command{
		Use:   "platform-connect",
		Short: "Configure platform S3 storage from JSON on stdin.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := c.openService(false, development)
			if err != nil {
				return err
			}
			defer s.Close()
			var cfg service.S3Config
			if json.NewDecoder(io.LimitReader(os.Stdin, 65536)).Decode(&cfg) != nil {
				return fmt.Errorf("invalid storage configuration")
			}
			// One envelope per command: the isolation probe result rides in
			// the same projection instead of a second JSON object.
			data := map[string]any{}
			status, message := "success", "Platform storage configured."
			if verifyIsolation {
				isolated, err := s.VerifyStorageIsolation(context.Background(), cfg)
				if err != nil {
					return err
				}
				isolation := "isolated"
				if !isolated {
					isolation = "over_scoped"
					status = "partial"
					message = "Platform storage configured; credential is over-scoped, prefer a prefix-scoped credential or rely on space encryption."
				}
				data["storage_isolation"] = isolation
			}
			if err = s.SetPlatform(context.Background(), cfg); err != nil {
				return err
			}
			return c.mode.emitData("platform-connect", status, message, data)
		},
	}
	cmd.Flags().BoolVar(&verifyIsolation, "verify-isolation", false, "Probe whether the storage credential is narrowed to this deployment namespace (writes only inside reserved probe namespaces)")
	cmd.Flags().BoolVar(&development, "development", false, "Allow loopback HTTP storage endpoints for offline rehearsal only")
	return cmd
}

func newBackupCommand(c *cli) *cobra.Command {
	var output, verify, decryptBundle string
	var verifySample int
	var encryptBundle bool
	cmd := &cobra.Command{
		Use:   "backup",
		Short: "Create, verify or decrypt a metadata backup with object manifest.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := c.openService(false, false)
			if err != nil {
				return err
			}
			defer s.Close()
			state := c.effective.Values["state"]
			if verify != "" {
				result, err := s.VerifyBackup(context.Background(), verify, verifySample)
				if err != nil {
					return err
				}
				status, message := "success", fmt.Sprintf("Backup verified: %d manifest entries, %d objects sampled.", result.ManifestEntries, result.Sampled)
				if !result.Ready {
					status = "not_ready"
					message = fmt.Sprintf("Backup NOT verifiable: %d failure(s); treat as unrestorable until fixed.", len(result.Failures))
				}
				payload, _ := json.Marshal(result)
				data := map[string]any{}
				json.Unmarshal(payload, &data)
				if err := c.mode.emitData("backup-verify", status, message, data); err != nil {
					return err
				}
				if !result.Ready {
					return errDoctorNotReady
				}
				return nil
			}
			if decryptBundle != "" {
				if output == "" {
					return fmt.Errorf("output directory required")
				}
				blob, err := os.ReadFile(decryptBundle)
				if err != nil {
					return err
				}
				files, err := s.OpenBundle(blob)
				if err != nil {
					return err
				}
				if err = os.Mkdir(output, 0700); err != nil {
					return err
				}
				names := make([]string, 0, len(files))
				for name := range files {
					names = append(names, name)
				}
				sort.Strings(names)
				for _, name := range names {
					out, err := os.OpenFile(filepath.Join(output, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
					if err != nil {
						return err
					}
					_, writeErr := out.Write(files[name])
					syncErr := out.Sync()
					closeErr := out.Close()
					if writeErr != nil || syncErr != nil || closeErr != nil {
						return fmt.Errorf("extraction failed")
					}
				}
				return c.mode.emitData("backup-decrypt", "success", fmt.Sprintf("Sealed bundle decrypted: %d files extracted.", len(names)), map[string]any{"files": len(names)})
			}
			if output == "" {
				return fmt.Errorf("backup directory required")
			}
			if err = os.Mkdir(output, 0700); err != nil {
				return err
			}
			if err = s.CheckpointMetadata(); err != nil {
				return err
			}
			staging := output
			if encryptBundle {
				if staging, err = os.MkdirTemp("", "drivebridge-backup-"); err != nil {
					return err
				}
				defer os.RemoveAll(staging)
			}
			for _, name := range []string{"metadata.sqlite", "instance.id"} {
				in, err := os.Open(filepath.Join(state, name))
				if err != nil {
					return err
				}
				out, err := os.OpenFile(filepath.Join(staging, name), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if err != nil {
					in.Close()
					return err
				}
				_, err = io.Copy(out, in)
				in.Close()
				out.Sync()
				out.Close()
				if err != nil {
					return err
				}
			}
			if err = service.WriteBackupManifest(staging); err != nil {
				return err
			}
			if encryptBundle {
				files := map[string][]byte{}
				for _, name := range []string{"metadata.sqlite", "instance.id", "manifest.json"} {
					body, err := os.ReadFile(filepath.Join(staging, name))
					if err != nil {
						return err
					}
					files[name] = body
				}
				blob, err := s.SealBundle(files)
				if err != nil {
					return err
				}
				out, err := os.OpenFile(filepath.Join(output, "backup-bundle.enc"), os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
				if err != nil {
					return err
				}
				_, writeErr := out.Write(blob)
				syncErr := out.Sync()
				closeErr := out.Close()
				if writeErr != nil || syncErr != nil || closeErr != nil {
					return fmt.Errorf("sealed bundle write failed")
				}
				return c.mode.emitData("backup", "success", "Encrypted metadata bundle written. instance.key stays in the separate secret channel; without it the bundle is unrecoverable.", map[string]any{"bundle_files": len(files), "schema_version": 1})
			}
			return c.mode.emit("backup", "success", "Metadata and object manifest backed up. Preserve the instance key separately; unfinished upload parts require the original state directory.")
		},
	}
	f := cmd.Flags()
	f.StringVar(&output, "output", "", "New backup directory (instance key excluded); also the extraction target for --decrypt and the export target for space-export")
	f.StringVar(&verify, "verify", "", "Verify an existing backup directory (read-only) instead of creating one")
	f.IntVar(&verifySample, "verify-sample", 50, "Object existence sample size for backup --verify")
	f.BoolVar(&encryptBundle, "encrypt", false, "Write the metadata backup as an instance-key sealed bundle")
	f.StringVar(&decryptBundle, "decrypt", "", "Decrypt a sealed backup bundle into --output")
	return cmd
}

func newKeyRotateCommand(c *cli) *cobra.Command {
	return &cobra.Command{
		Use:   "key-rotate",
		Short: "Rotate the instance KEK: re-wrap space DEKs, rewrite no objects.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			s, err := c.openService(false, false)
			if err != nil {
				return err
			}
			defer s.Close()
			report, err := s.RotateKEK()
			if err != nil {
				return err
			}
			return c.mode.emitData("key-rotate", "success", fmt.Sprintf("Instance KEK rotated to version %v; %v space(s) re-wrapped, 0 objects rewritten.", report["key_version"], report["spaces_rewrapped"]), report)
		},
	}
}

func newSpaceExportCommand(c *cli) *cobra.Command {
	var spaceID, output string
	cmd := &cobra.Command{
		Use:   "space-export",
		Short: "Export an encrypted space's current versions as plaintext files.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			if spaceID == "" || output == "" {
				return fmt.Errorf("space and output directory required")
			}
			s, err := c.openService(false, false)
			if err != nil {
				return err
			}
			defer s.Close()
			report, err := s.ExportSpacePlaintext(context.Background(), spaceID, output)
			if err != nil {
				return err
			}
			return c.mode.emitData("space-export", "success", fmt.Sprintf("Encrypted space exported as plaintext: %v file(s), %v byte(s).", report["files_exported"], report["bytes_written"]), report)
		},
	}
	cmd.Flags().StringVar(&spaceID, "space", "", "Space id for space-export")
	cmd.Flags().StringVar(&output, "output", "", "Plaintext export target directory")
	return cmd
}

func newConfigCommand(c *cli) *cobra.Command {
	root := &cobra.Command{Use: "config", Short: "Inspect or create the server configuration file."}
	show := &cobra.Command{
		Use:   "show",
		Short: "Print the resolved configuration and each key's source.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			values := map[string]any{"config_file": c.effective.Path}
			for _, key := range serverconfig.Keys {
				values[key] = c.effective.Values[key]
				values["source."+key] = string(c.effective.Sources[key])
			}
			if !(c.mode.json || c.mode.agent || c.mode.events || c.mode.explain) {
				fmt.Println("config_file:", c.effective.Path)
				for _, key := range serverconfig.Keys {
					fmt.Printf("%-22s %s  (%s)\n", key, c.effective.Values[key], c.effective.Sources[key])
				}
				return nil
			}
			return c.mode.emitData("config.show", "success", "Resolved server configuration; secrets never appear here.", values)
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
				target = filepath.Join(home, "drivebridge-server", "config.yaml")
			}
			if _, err := os.Stat(target); err == nil {
				return fmt.Errorf("config exists: %s", target)
			}
			if err := os.MkdirAll(filepath.Dir(target), 0700); err != nil {
				return err
			}
			if err := os.WriteFile(target, []byte(serverconfig.Template()), 0600); err != nil {
				return err
			}
			return c.mode.emitData("config.init", "success", fmt.Sprintf("Wrote %s.", target), map[string]any{"path": target})
		},
	}
	root.AddCommand(show, init)
	return root
}

func readSecret(path string) (string, error) {
	if path == "" {
		return "", nil
	}
	i, e := os.Lstat(path)
	if e != nil || !i.Mode().IsRegular() || i.Mode().Perm()&0077 != 0 {
		return "", fmt.Errorf("private credential file required")
	}
	f, e := os.Open(path)
	if e != nil {
		return "", e
	}
	defer f.Close()
	b, e := io.ReadAll(io.LimitReader(f, 8193))
	if e != nil || len(b) > 8192 {
		return "", fmt.Errorf("invalid credential file")
	}
	return strings.TrimSpace(string(b)), nil
}
