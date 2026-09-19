package bridge

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	rconfig "github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/obscure"
)

// Product-discovery tiers. first_class_* kinds are bind/adopt defaults;
// escape kinds stay addable but are omitted from that default surface.
const (
	KindTierFirstClassStore = "first_class_store"
	KindTierFirstClassCloud = "first_class_cloud"
	KindTierEscape          = "escape"
)

// KindDef is one entry of the storage onboarding catalog: what a kind is,
// which flags it needs and which capabilities it honestly has. Generic
// (transfer-only) kinds never claim direct URLs or reference handoff; keys
// absent from Capabilities mean "backend-dependent", never "supported".
// Tier and BindDefault are additive discovery fields and do not change
// backend capabilities.
type KindDef struct {
	Kind         string          `json:"kind"`
	Display      string          `json:"display"`
	Backend      string          `json:"backend"`
	Requires     []string        `json:"requires"`
	Notes        []string        `json:"notes,omitempty"`
	Capabilities map[string]bool `json:"capabilities"`
	Tier         string          `json:"tier,omitempty"`
	BindDefault  bool            `json:"bind_default"`
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
// First-class store/cloud labels are product discovery only; local/s3/gdrive
// remain generic transfer-only backends.
func StorageKinds() []KindDef {
	return []KindDef{
		{Kind: "onedrive", Display: "OneDrive", Backend: "onedrive",
			Requires:     []string{"drivebridge auth --auto", "drivebridge setup"},
			Capabilities: onedriveCaps(),
			Tier:         KindTierFirstClassCloud, BindDefault: true,
			Notes: []string{"Onboard with auth + setup, not storage add; the only backend with direct URL and reference handoff.", "First-class personal cloud; included in default bind/adopt discovery."}},
		{Kind: "local", Display: "Local directory (alias remote)", Backend: "generic",
			Requires:     []string{"--space", "--remote-path (absolute directory)", "--local-root"},
			Capabilities: genericCaps(),
			Tier:         KindTierFirstClassStore, BindDefault: true,
			Notes: []string{"Creates an alias remote; a zero-copy view of the directory.", "Manageable default store for owner local roots; not a new cloud vendor.", "Included in default bind/adopt discovery."}},
		{Kind: "openlist", Display: "OpenList (AList-fork aggregator)", Backend: "generic",
			Requires:     []string{"--space", "--url (OpenList /dav endpoint)", "--webdav-user", "--webdav-pass-file (0600)", "--local-root"},
			Capabilities: genericCaps(),
			Tier:         KindTierEscape, BindDefault: false,
			Notes: []string{"--remote-path is optional and defaults to the OpenList root.", "Self-host and pin a reviewed version; use a dedicated least-privilege account.", "CN drives arrive transfer-only; direct URLs stay refused.", "Escape hatch; omitted from default bind/adopt discovery."}},
		{Kind: "gdrive", Display: "Google Drive (rclone drive)", Backend: "generic",
			Requires:     []string{"--space", "--remote (rclone remote, type=drive)", "--local-root"},
			Capabilities: genericCaps(),
			Tier:         KindTierFirstClassCloud, BindDefault: true,
			Notes: []string{"--remote-path is optional (empty = the whole drive).", "Server-side copy is used whenever the remote advertises it.", "Version pin today is size+mtime; native MD5 reuse is a retained-next promotion.", "First-class personal cloud for bind/adopt discovery; transfer-only (no direct URL or reference handoff)."}},
		{Kind: "s3", Display: "S3 compatible (MinIO/COS/OSS/R2/B2)", Backend: "generic",
			Requires:     []string{"--space", "--remote (rclone remote, type=s3)", "--remote-path '<bucket>/<prefix>'", "--local-root"},
			Capabilities: genericCaps(),
			Tier:         KindTierFirstClassStore, BindDefault: true,
			Notes: []string{"The first remote-path segment must be the bucket.", "Manageable default store for owner S3 prefixes; included in default bind/adopt discovery."}},
		{Kind: "webdav", Display: "WebDAV", Backend: "generic",
			Requires:     []string{"--space", "--remote (rclone remote)", "--local-root"},
			Capabilities: genericCaps(),
			Tier:         KindTierEscape, BindDefault: false,
			Notes: []string{"--remote-path is optional (empty = remote root).", "Escape hatch; omitted from default bind/adopt discovery."}},
		{Kind: "smb", Display: "SMB share", Backend: "generic",
			Requires:     []string{"--space", "--remote (rclone remote)", "--local-root"},
			Capabilities: genericCaps(),
			Tier:         KindTierEscape, BindDefault: false,
			Notes: []string{"--remote-path is optional (empty = remote root).", "Escape hatch; omitted from default bind/adopt discovery."}},
		{Kind: "sftp", Display: "SFTP", Backend: "generic",
			Requires:     []string{"--space", "--remote (rclone remote)", "--local-root"},
			Capabilities: genericCaps(),
			Tier:         KindTierEscape, BindDefault: false,
			Notes: []string{"--remote-path is optional (empty = remote root).", "Escape hatch; omitted from default bind/adopt discovery."}},
		{Kind: "alias", Display: "Alias to another rclone remote", Backend: "generic",
			Requires:     []string{"--space", "--remote (alias remote)", "--local-root"},
			Capabilities: genericCaps(),
			Tier:         KindTierEscape, BindDefault: false,
			Notes: []string{"--remote-path is optional (empty = remote root).", "Escape hatch; omitted from default bind/adopt discovery."}},
	}
}

// BindDefaultKinds is the default bind/adopt discovery surface: kinds with
// BindDefault set. Escape hatches remain in StorageKinds.
func BindDefaultKinds() []KindDef {
	all := StorageKinds()
	out := make([]KindDef, 0, 4)
	for _, k := range all {
		if k.BindDefault {
			out = append(out, k)
		}
	}
	return out
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
	// Aggregator/AggregatorVersion are filled when the openlist kind detects a
	// live OpenList instance behind the WebDAV endpoint; absence is not an
	// error (any WebDAV server remains a valid remote).
	Aggregator        string `json:"aggregator,omitempty"`
	AggregatorVersion string `json:"aggregator_version,omitempty"`
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
		return nil, errors.New("probe_failed[list]:" + err.Error())
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
	if kind == "openlist" {
		detectOpenList(ctx, res, rconfig.GetValue(remote, "url"))
	}
	return res, nil
}

// detectOpenList probes the unauthenticated /api/public/settings endpoint on
// the origin behind a /dav WebDAV URL. A miss is silent: any WebDAV server
// remains a valid openlist-kind remote.
func detectOpenList(ctx context.Context, res *ProbeResult, davURL string) {
	origin := strings.TrimSuffix(strings.TrimSuffix(strings.TrimSuffix(davURL, "/"), "/dav"), "/")
	if origin == "" || !strings.HasPrefix(origin, "http") {
		return
	}
	cctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(cctx, http.MethodGet, origin+"/api/public/settings", nil)
	if err != nil {
		return
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return
	}
	var payload struct {
		Code int `json:"code"`
		Data struct {
			Version string `json:"version"`
		} `json:"data"`
	}
	if json.NewDecoder(io.LimitReader(resp.Body, 1<<16)).Decode(&payload) != nil {
		return
	}
	if payload.Code != 200 {
		return
	}
	res.Aggregator = "openlist"
	res.AggregatorVersion = payload.Data.Version
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
