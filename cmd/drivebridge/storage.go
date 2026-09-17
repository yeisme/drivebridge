package main

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"github.com/yeisme/drivebridge/internal/bridge"
)

// newStorageCommand wires the onboarding surface: kinds (catalog), probe
// (read-only listing) and add (guided space creation for generic backends).
func newStorageCommand(c *cli) *cobra.Command {
	root := &cobra.Command{
		Use:   "storage",
		Short: "Discover, probe and add storage backends (guides generic backends; OneDrive uses auth + setup).",
	}
	root.AddCommand(storageKindsCommand(c), storageProbeCommand(c), storageAddCommand(c))
	return root
}

func storageKindsCommand(c *cli) *cobra.Command {
	return &cobra.Command{
		Use:   "kinds",
		Short: "List supported storage kinds with required flags and honest capabilities.",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c.command = "storage.kinds"
			kinds := bridge.StorageKinds()
			if !(c.mode.JSON || c.mode.Agent || c.mode.Events || c.mode.Explain) {
				for _, k := range kinds {
					fmt.Printf("%-10s %-38s requires: %s\n", k.Kind, k.Display, strings.Join(k.Requires, " "))
				}
			}
			return c.mode.write("storage.kinds", "success", fmt.Sprintf("%d storage kinds; OneDrive onboards via auth + setup.", len(kinds)), map[string]any{"count": len(kinds), "kinds": kinds})
		},
	}
}

func storageProbeCommand(c *cli) *cobra.Command {
	var kind, remote, remotePath string
	cmd := &cobra.Command{
		Use:   "probe",
		Short: "Read-only check of a generic rclone remote root (reachability and a listing sample; never a write test; the full root is listed, so avoid huge roots).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c.command = "storage.probe"
			if err := c.installRclone(); err != nil {
				return err
			}
			if remote == "" {
				return fail("storage_missing_flag", "storage probe requires --remote (and optionally --remote-path). Run 'drivebridge storage kinds' for details.")
			}
			res, err := bridge.ProbeStorage(cmd.Context(), kind, remote, remotePath)
			if err != nil {
				return fail(err.Error(), hintEN(err.Error()))
			}
			if !(c.mode.JSON || c.mode.Agent || c.mode.Events || c.mode.Explain) {
				fmt.Printf("remote %s:%s reachable; %d root entries; pin: %s; writability: %s\n", res.Remote, res.Root, res.Entries, res.Pin, res.Writability)
			}
			return c.mode.write("storage.probe", "success", fmt.Sprintf("Remote %s is reachable; %d root entries.", res.Remote, res.Entries), map[string]any{"probe": res})
		},
	}
	cmd.Flags().StringVar(&kind, "kind", "", "Informational kind label for the probe output")
	cmd.Flags().StringVar(&remote, "remote", "", "Existing rclone remote name")
	cmd.Flags().StringVar(&remotePath, "remote-path", "", "Root path inside the remote (empty = remote root)")
	return cmd
}

// readPassFile mirrors the house credential-file discipline: regular file,
// no group/other access, bounded size, trimmed content.
func readPassFile(path string) (string, error) {
	info, err := os.Lstat(path)
	if err != nil || !info.Mode().IsRegular() || info.Mode().Perm()&0077 != 0 {
		return "", fail("credential_file_not_private", "The --webdav-pass-file must exist, be a regular file, and disallow group/other access; it was not read.")
	}
	raw, err := os.ReadFile(path)
	if err != nil || len(raw) > 8192 {
		return "", fail("credential_file_not_private", "The --webdav-pass-file could not be read or exceeds 8192 bytes.")
	}
	return strings.TrimSpace(string(raw)), nil
}

func storageAddCommand(c *cli) *cobra.Command {
	var kind, space, remote, remoteName, remotePath, local, url, user, passFile, project, actor string
	var maxBytes int64
	var follow bool
	cmd := &cobra.Command{
		Use:   "add",
		Short: "Guided add of a generic storage backend (creates alias/webdav remotes for local/openlist kinds, probes, then registers the space).",
		RunE: func(cmd *cobra.Command, _ []string) error {
			c.command = "storage-add"
			if err := c.installRclone(); err != nil {
				return err
			}
			var catalog bridge.KindDef
			found := false
			for _, k := range bridge.StorageKinds() {
				if k.Kind == kind {
					catalog, found = k, true
				}
			}
			if !found {
				return fail("storage_kind_unknown", "Unknown --kind "+kind+"; run 'drivebridge storage kinds' for the supported list.")
			}
			if kind == "onedrive" {
				return fail("unsupported", "OneDrive onboards via 'drivebridge auth --auto' then 'drivebridge setup'; storage add only handles generic backends.")
			}
			missing := missingFlags(map[string]string{
				"--space": space, "--local-root": local,
				"--remote": remote, "--remote-path": remotePath,
				"--url": url, "--webdav-user": user, "--webdav-pass-file": passFile,
			}, catalog)
			if len(missing) > 0 {
				return fail("storage_missing_flag", "storage add --kind "+kind+" is missing: "+strings.Join(missing, ", ")+". Run 'drivebridge storage kinds' for details.")
			}
			if kind == "local" && !filepath.IsAbs(remotePath) {
				return fail("storage_missing_flag", "--remote-path for kind local must be an absolute directory.")
			}
			// Pre-validate everything the delegate will check BEFORE any
			// rclone section is created, so a rejected add never leaves an
			// orphaned remote behind.
			defaulted := false
			if maxBytes == 0 {
				maxBytes = 1 << 30
				defaulted = true
			}
			if maxBytes < 0 {
				return fail("invalid_space", "--max-bytes must be a positive byte limit.")
			}
			if root, err := filepath.EvalSymlinks(local); err != nil || !filepath.IsAbs(root) {
				return fail("invalid_space", "--local-root must be an existing absolute directory.")
			} else if info, err := os.Stat(root); err != nil || !info.IsDir() {
				return fail("invalid_space", "--local-root must be an existing absolute directory.")
			}
			pass := ""
			if passFile != "" {
				var err error
				if pass, err = readPassFile(passFile); err != nil {
					return err
				}
			}
			name := remoteName
			if name == "" {
				name = "db-" + kind + "-" + sanitizeName(space)
			}
			created := false
			rollback := func() {
				if created {
					bridge.DeleteRemote(name)
				}
			}
			switch kind {
			case "local":
				if bridge.AliasRemoteMatches(name, remotePath) {
					// Same directory already configured: reuse it.
				} else if err := bridge.EnsureAliasRemote(name, remotePath); err != nil {
					return fail(err.Error(), hintEN(err.Error()))
				} else {
					created = true
				}
				remote = name
			case "openlist":
				if bridge.WebDAVRemoteMatches(name, url, user, pass) {
					// Same endpoint and credentials already configured: reuse it.
				} else if err := bridge.EnsureWebDAVRemote(name, url, user, pass); err != nil {
					return fail(err.Error(), hintEN(err.Error()))
				} else {
					created = true
				}
				remote = name
			}
			// For kind local the absolute directory IS the alias target; the
			// space root and the probe root both sit at the remote root.
			probeRoot := remotePath
			spaceRoot := remotePath
			if kind == "local" {
				probeRoot = ""
				spaceRoot = ""
			}
			res, err := bridge.ProbeStorage(cmd.Context(), kind, remote, probeRoot)
			if err != nil {
				rollback()
				return fail(err.Error(), hintEN(err.Error()))
			}
			if !(c.mode.JSON || c.mode.Agent || c.mode.Events || c.mode.Explain) {
				fmt.Printf("probe %s:%s reachable; %d root entries; writability: %s\n", res.Remote, res.Root, res.Entries, res.Writability)
				if defaulted {
					fmt.Println("max-bytes defaulted to 1073741824; pass --max-bytes to change it.")
				}
			}
			delegate := spaceAddCommand(c, "storage-add")
			delegate.SetContext(cmd.Context())
			for flag, value := range map[string]string{
				"space": space, "remote": remote, "backend": "generic", "remote-path": spaceRoot,
				"local-root": local, "max-bytes": strconv.FormatInt(maxBytes, 10),
				"follow-symlinks": strconv.FormatBool(follow), "project": project, "actor": actor,
			} {
				if err := delegate.Flags().Set(flag, value); err != nil {
					rollback()
					return err
				}
			}
			if err := delegate.RunE(delegate, nil); err != nil {
				rollback()
				return err
			}
			return nil
		},
	}
	f := cmd.Flags()
	f.StringVar(&kind, "kind", "", "Storage kind from 'drivebridge storage kinds' (e.g. local, openlist, gdrive, s3, smb, sftp, webdav, alias)")
	f.StringVar(&space, "space", "", "Space identifier")
	f.StringVar(&remote, "remote", "", "Existing rclone remote name (required unless the kind creates one)")
	f.StringVar(&remoteName, "remote-name", "", "Override the auto-created rclone remote name (local/openlist kinds)")
	f.StringVar(&remotePath, "remote-path", "", "Root path inside the remote (local kind: absolute directory; optional for remote kinds = remote root)")
	f.StringVar(&local, "local-root", "", "Allowed absolute local directory")
	f.StringVar(&url, "url", "", "OpenList WebDAV endpoint, e.g. http://host:5244/dav")
	f.StringVar(&user, "webdav-user", "", "WebDAV user for the openlist kind")
	f.StringVar(&passFile, "webdav-pass-file", "", "0600 regular file holding the WebDAV password (max 8192 bytes); never printed")
	f.Int64Var(&maxBytes, "max-bytes", 0, "Space byte limit (default 1073741824)")
	f.BoolVar(&follow, "follow-symlinks", false, "Materialize symbolic links on upload for this space")
	f.StringVar(&project, "project", "", "Authorized Scaena project reference")
	f.StringVar(&actor, "actor", "", "Authorized Scaena actor reference")
	return cmd
}

// missingFlags returns the required-but-empty flags for a kind, failing
// closed on any future catalog flag that is absent from the values map.
// Required flag names are derived from the catalog's -- prefixed entries.
func missingFlags(values map[string]string, def bridge.KindDef) []string {
	var missing []string
	for _, req := range def.Requires {
		if !strings.HasPrefix(req, "--") {
			continue
		}
		name := req
		if i := strings.IndexByte(req, ' '); i >= 0 {
			name = req[:i]
		}
		if v, ok := values[name]; !ok || v == "" {
			missing = append(missing, name)
		}
	}
	return missing
}

// sanitizeName maps a space id onto the safe remote-name charset. When the
// id carries no alphanumeric byte at all, a short stable hash keeps the name
// unique instead of collapsing every such id onto dashes.
func sanitizeName(space string) string {
	out := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9', r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, space)
	if strings.ContainsAny(out, "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ") {
		return out
	}
	sum := sha256.Sum256([]byte(space))
	return "space-" + hex.EncodeToString(sum[:])[:8]
}
