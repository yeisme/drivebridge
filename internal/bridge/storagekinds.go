package bridge

import (
	"context"
	"errors"
	"path/filepath"
	"strings"

	"github.com/rclone/rclone/fs"
	rconfig "github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/obscure"
)

// KindDef is one entry of the storage onboarding catalog: what a kind is,
// which flags it needs and which capabilities it honestly has. Generic
// (transfer-only) kinds never claim direct URLs or reference handoff; keys
// absent from Capabilities mean "backend-dependent", never "supported".
type KindDef struct {
	Kind         string          `json:"kind"`
	Display      string          `json:"display"`
	Backend      string          `json:"backend"`
	Requires     []string        `json:"requires"`
	Notes        []string        `json:"notes,omitempty"`
	Capabilities map[string]bool `json:"capabilities"`
}

// genericCaps mirrors the mcp.go backend_capabilities truth table for the
// generic row exactly (transfer/direct_url/reference_handoff). server-side
// copy is deliberately absent: Generic.Copy uses it whenever the backend
// advertises it, so a flat false would be dishonest.
func genericCaps() map[string]bool {
	return map[string]bool{"transfer": true, "direct_url": false, "reference_handoff": false}
}

func onedriveCaps() map[string]bool {
	return map[string]bool{"transfer": true, "server_side_copy": true, "direct_url": true, "reference_handoff": true}
}

// StorageKinds is the discoverable catalog behind `drivebridge storage kinds`
// and the capabilities resource. OneDrive stays the only privileged path.
func StorageKinds() []KindDef {
	return []KindDef{
		{Kind: "onedrive", Display: "OneDrive", Backend: "onedrive",
			Requires:     []string{"drivebridge auth --auto", "drivebridge setup"},
			Capabilities: onedriveCaps(),
			Notes:        []string{"Onboard with auth + setup, not storage add; the only backend with direct URL and reference handoff."}},
		{Kind: "local", Display: "Local directory (alias remote)", Backend: "generic",
			Requires:     []string{"--space", "--remote-path (absolute directory)", "--local-root"},
			Capabilities: genericCaps(),
			Notes:        []string{"Creates an alias remote; a zero-copy view of the directory."}},
		{Kind: "openlist", Display: "OpenList (AList-fork aggregator)", Backend: "generic",
			Requires:     []string{"--space", "--url (OpenList /dav endpoint)", "--webdav-user", "--webdav-pass-file (0600)", "--local-root"},
			Capabilities: genericCaps(),
			Notes:        []string{"--remote-path is optional and defaults to the OpenList root.", "Self-host and pin a reviewed version; use a dedicated least-privilege account.", "CN drives arrive transfer-only; direct URLs stay refused."}},
		{Kind: "gdrive", Display: "Google Drive (rclone drive)", Backend: "generic",
			Requires:     []string{"--space", "--remote (rclone remote, type=drive)", "--local-root"},
			Capabilities: genericCaps(),
			Notes:        []string{"--remote-path is optional (empty = the whole drive).", "Server-side copy is used whenever the remote advertises it.", "Version pin today is size+mtime; native MD5 reuse is a retained-next promotion."}},
		{Kind: "s3", Display: "S3 compatible (MinIO/COS/OSS/R2/B2)", Backend: "generic",
			Requires:     []string{"--space", "--remote (rclone remote, type=s3)", "--remote-path '<bucket>/<prefix>'", "--local-root"},
			Capabilities: genericCaps(),
			Notes:        []string{"The first remote-path segment must be the bucket."}},
		{Kind: "webdav", Display: "WebDAV", Backend: "generic",
			Requires:     []string{"--space", "--remote (rclone remote)", "--local-root"},
			Capabilities: genericCaps(),
			Notes:        []string{"--remote-path is optional (empty = remote root)."}},
		{Kind: "smb", Display: "SMB share", Backend: "generic",
			Requires:     []string{"--space", "--remote (rclone remote)", "--local-root"},
			Capabilities: genericCaps(),
			Notes:        []string{"--remote-path is optional (empty = remote root)."}},
		{Kind: "sftp", Display: "SFTP", Backend: "generic",
			Requires:     []string{"--space", "--remote (rclone remote)", "--local-root"},
			Capabilities: genericCaps(),
			Notes:        []string{"--remote-path is optional (empty = remote root)."}},
		{Kind: "alias", Display: "Alias to another rclone remote", Backend: "generic",
			Requires:     []string{"--space", "--remote (alias remote)", "--local-root"},
			Capabilities: genericCaps(),
			Notes:        []string{"--remote-path is optional (empty = remote root)."}},
	}
}

// ProbeResult is the read-only outcome of `storage probe`: reachability plus
// a bounded listing sample. It is never proof of write access, and the full
// root is enumerated (avoid huge remote roots).
type ProbeResult struct {
	Kind         string          `json:"kind,omitempty"`
	Remote       string          `json:"remote"`
	Root         string          `json:"root"`
	Reachable    bool            `json:"reachable"`
	Entries      int             `json:"entries"`
	Sample       []string        `json:"sample,omitempty"`
	Pin          string          `json:"pin"`
	Writability  string          `json:"writability"`
	Capabilities map[string]bool `json:"capabilities"`
}

// ProbeStorage lists the remote root exactly like Generic would resolve it.
// It creates nothing and verifies nothing about writability.
func ProbeStorage(ctx context.Context, kind, remote, remotePath string) (*ProbeResult, error) {
	if remote == "" || strings.ContainsAny(remote, ":/\\") {
		return nil, errors.New("invalid_space")
	}
	if rconfig.GetValue(remote, "type") == "" {
		return nil, errors.New("probe_failed")
	}
	f, err := fs.NewFs(ctx, remote+":"+remotePath)
	if err != nil {
		return nil, errors.New("probe_failed")
	}
	entries, err := f.List(ctx, "")
	if err != nil {
		return nil, errors.New("probe_failed")
	}
	res := &ProbeResult{
		Kind: kind, Remote: remote, Root: remotePath, Reachable: true,
		Pin:         "size+mtime (drift reports file_version_changed)",
		Writability: "unverified (the first upload verifies)",
	}
	res.Capabilities = genericCaps()
	if kind == "onedrive" {
		res.Pin = "item+ETag"
		res.Capabilities = onedriveCaps()
	}
	for _, e := range entries {
		res.Entries++
		if len(res.Sample) < 5 {
			res.Sample = append(res.Sample, e.Remote())
		}
	}
	return res, nil
}

// saveConfig persists the in-memory config and surfaces write failures with
// the registered code (rclone's own SaveConfig only logs).
func saveConfig() error {
	if err := rconfig.Data().Save(); err != nil {
		return errors.New("config_write_failed")
	}
	return nil
}

// EnsureAliasRemote creates an alias remote pointing at an absolute local
// directory; an existing remote of a different shape is never rewritten. The
// target is stored Cleaned but NOT symlink-resolved: the alias stays a
// zero-copy view and rclone resolves it at access time.
func EnsureAliasRemote(name, dir string) error {
	if err := validRemoteName(name); err != nil {
		return err
	}
	dir = filepath.Clean(dir)
	if rconfig.GetValue(name, "type") != "" {
		return errors.New("remote_exists")
	}
	rconfig.Data().SetValue(name, "type", "alias")
	rconfig.Data().SetValue(name, "remote", dir)
	return saveConfig()
}

// EnsureWebDAVRemote creates a WebDAV remote for aggregators like OpenList.
// The password is stored rclone-obscured (reversible obfuscation, standard
// rclone practice) and never returned.
func EnsureWebDAVRemote(name, url, user, pass string) error {
	if err := validRemoteName(name); err != nil {
		return err
	}
	if url == "" || user == "" || pass == "" {
		return errors.New("storage_missing_flag")
	}
	if rconfig.GetValue(name, "type") != "" {
		return errors.New("remote_exists")
	}
	obscured, err := obscure.Obscure(pass)
	if err != nil {
		return errors.New("config_write_failed")
	}
	rconfig.Data().SetValue(name, "type", "webdav")
	rconfig.Data().SetValue(name, "url", url)
	rconfig.Data().SetValue(name, "vendor", "other")
	rconfig.Data().SetValue(name, "user", user)
	rconfig.Data().SetValue(name, "pass", obscured)
	return saveConfig()
}

// WebDAVRemoteMatches reports whether an existing remote already points at
// the same aggregator endpoint AND carries the same password, so credential
// rotation surfaces explicitly instead of silently reusing stale secrets.
// rclone obscuring carries a random nonce, so the stored value is revealed
// for comparison instead of re-obscuring the candidate.
func WebDAVRemoteMatches(name, url, user, pass string) bool {
	if rconfig.GetValue(name, "type") != "webdav" ||
		rconfig.GetValue(name, "url") != url ||
		rconfig.GetValue(name, "user") != user {
		return false
	}
	stored, err := obscure.Reveal(rconfig.GetValue(name, "pass"))
	if err != nil {
		return false
	}
	return stored == pass
}

// AliasRemoteMatches reports whether an existing alias remote already points
// at the same directory (Cleaned comparison; /a/b and /a/b/ reuse).
func AliasRemoteMatches(name, dir string) bool {
	return rconfig.GetValue(name, "type") == "alias" &&
		filepath.Clean(rconfig.GetValue(name, "remote")) == filepath.Clean(dir)
}

// DeleteRemote removes an auto-created section so a failed guided add leaves
// no poisoned remote behind. It never touches sections it did not create:
// callers only pass names they just created.
func DeleteRemote(name string) {
	if name == "" || strings.ContainsAny(name, ":/\\") {
		return
	}
	rconfig.Data().DeleteSection(name)
	_ = saveConfig()
}

func validRemoteName(name string) error {
	if name == "" || strings.ContainsAny(name, ":/\\") {
		return errors.New("invalid_space")
	}
	return nil
}
