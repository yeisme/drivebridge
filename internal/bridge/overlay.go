package bridge

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	rconfig "github.com/rclone/rclone/fs/config"
)

const pinaxVaultMarkerName = ".drivebridge-pinax-vault.json"
const pinaxVaultSchema = "drivebridge.pinax_vault.v1"

// PinaxVaultReceipt is the CLI-authored overlay marker at vault-root.
type PinaxVaultReceipt struct {
	SchemaVersion string   `json:"schema_version"`
	Preset        string   `json:"preset"`
	Space         string   `json:"space"`
	VaultRoot     string   `json:"vault_root"`
	ContentRoot   string   `json:"content_root"`
	Overlay       string   `json:"overlay"`
	Adapter       string   `json:"adapter"`
	Links         []string `json:"links"`
}

func controlPlaneName(name string) bool {
	switch name {
	case ".pinax", ".git", ".obsidian", ".pinaxignore", ".gitignore",
		".drivebridge-cas", ".drivebridge", pinaxVaultMarkerName:
		return true
	default:
		return strings.HasPrefix(name, ".")
	}
}

func looksLikeCapsaPrefix(root string) bool {
	if _, err := os.Stat(filepath.Join(root, "protocol.json")); err != nil {
		return false
	}
	info, err := os.Stat(filepath.Join(root, "workspaces"))
	return err == nil && info.IsDir()
}

// AliasContentRoot returns the local directory an rclone alias remote points at.
func AliasContentRoot(remote string) (string, error) {
	if rconfig.GetValue(remote, "type") != "alias" {
		return "", errors.New("mount_path_adapter_missing")
	}
	dir := filepath.Clean(rconfig.GetValue(remote, "remote"))
	if dir == "" || !filepath.IsAbs(dir) {
		return "", errors.New("mount_path_adapter_missing")
	}
	info, err := os.Stat(dir)
	if err != nil || !info.IsDir() {
		return "", errors.New("mount_path_adapter_missing")
	}
	return dir, nil
}

func sameOrNested(a, b string) (same bool, aInB bool, bInA bool) {
	aa, errA := filepath.Abs(a)
	bb, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		return false, false, false
	}
	aa = filepath.Clean(aa)
	bb = filepath.Clean(bb)
	if aa == bb {
		return true, false, false
	}
	rel, err := filepath.Rel(bb, aa)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false, true, false
	}
	rel, err = filepath.Rel(aa, bb)
	if err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return false, false, true
	}
	return false, false, false
}

// ApplyPinaxOverlay 把内容根的用户树叠进本机 vault 根：内容走符号链接，
// 控制面（.pinax 等）留在 vault 根真实目录，避免 SQLite 落到对象存储/VFS。
func ApplyPinaxOverlay(vaultRoot, contentRoot, space string) (PinaxVaultReceipt, error) {
	var zero PinaxVaultReceipt
	if vaultRoot == "" || !filepath.IsAbs(vaultRoot) {
		return zero, errors.New("vault_root_not_local")
	}
	if strings.Contains(filepath.ToSlash(vaultRoot), "/.drivebridge-cas/") || filepath.Base(vaultRoot) == ".drivebridge-cas" {
		return zero, errors.New("vault_root_not_local")
	}
	if looksLikeCapsaPrefix(contentRoot) {
		return zero, errors.New("preset_prefix_is_capsa")
	}
	if err := os.MkdirAll(vaultRoot, 0o755); err != nil {
		return zero, errors.New("vault_root_not_local")
	}
	info, err := os.Lstat(vaultRoot)
	if err != nil || !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return zero, errors.New("vault_root_not_local")
	}
	same, vaultInContent, contentInVault := sameOrNested(vaultRoot, contentRoot)
	if vaultInContent || contentInVault {
		return zero, errors.New("mount_overlay_conflict")
	}
	entries, err := os.ReadDir(contentRoot)
	if err != nil {
		return zero, errors.New("mount_path_adapter_missing")
	}
	var names []string
	for _, e := range entries {
		if controlPlaneName(e.Name()) {
			continue
		}
		names = append(names, e.Name())
	}
	hasNotes := false
	for _, name := range names {
		if name == "notes" {
			hasNotes = true
			break
		}
	}
	if !hasNotes {
		if err := os.MkdirAll(filepath.Join(contentRoot, "notes"), 0o755); err != nil {
			return zero, errors.New("mount_path_adapter_missing")
		}
		names = append(names, "notes")
	}
	adapter := "pinax-vault"
	var links []string
	if !same {
		for _, name := range names {
			target := filepath.Join(contentRoot, name)
			dest := filepath.Join(vaultRoot, name)
			if err := ensureContentLink(dest, target); err != nil {
				return zero, err
			}
			links = append(links, name)
		}
	} else {
		adapter = "pinax-vault-colocated"
		if _, err := os.Stat(filepath.Join(contentRoot, "notes")); err != nil {
			if err := os.MkdirAll(filepath.Join(contentRoot, "notes"), 0o755); err != nil {
				return zero, errors.New("mount_path_adapter_missing")
			}
		}
	}
	receipt := PinaxVaultReceipt{
		SchemaVersion: pinaxVaultSchema,
		Preset:        "pinax-vault",
		Space:         space,
		VaultRoot:     vaultRoot,
		ContentRoot:   contentRoot,
		Overlay:       "pinax",
		Adapter:       adapter,
		Links:         links,
	}
	if err := persistPinaxReceipt(receipt); err != nil {
		return zero, err
	}
	return receipt, nil
}

func persistPinaxReceipt(receipt PinaxVaultReceipt) error {
	body, err := json.MarshalIndent(receipt, "", "  ")
	if err != nil {
		return errors.New("state_unavailable")
	}
	if err := os.WriteFile(filepath.Join(receipt.VaultRoot, pinaxVaultMarkerName), body, 0o600); err != nil {
		return errors.New("state_unavailable")
	}
	return nil
}

// WithAdapter records the path adapter (alias overlay vs rclone S3 mount) on
// the vault-root marker after the content tree is linked.
func (r PinaxVaultReceipt) WithAdapter(adapter string) (PinaxVaultReceipt, error) {
	if adapter == "" {
		return r, nil
	}
	r.Adapter = adapter
	if err := persistPinaxReceipt(r); err != nil {
		return r, err
	}
	return r, nil
}

func ensureContentLink(dest, target string) error {
	want, err := filepath.Abs(target)
	if err != nil {
		return errors.New("mount_overlay_conflict")
	}
	info, err := os.Lstat(dest)
	if err == nil {
		if info.Mode()&os.ModeSymlink == 0 {
			return errors.New("mount_overlay_conflict")
		}
		got, readErr := os.Readlink(dest)
		if readErr != nil {
			return errors.New("mount_overlay_conflict")
		}
		if !filepath.IsAbs(got) {
			got = filepath.Join(filepath.Dir(dest), got)
		}
		gotAbs, _ := filepath.Abs(got)
		if filepath.Clean(gotAbs) != filepath.Clean(want) {
			return errors.New("mount_overlay_conflict")
		}
		return nil
	}
	if !errors.Is(err, os.ErrNotExist) {
		return errors.New("mount_overlay_conflict")
	}
	if err := os.Symlink(want, dest); err != nil {
		return errors.New("mount_overlay_conflict")
	}
	return nil
}

// RemovePinaxOverlay 只拆内容符号链接和标记文件，不动 .pinax 与内容根字节。
func RemovePinaxOverlay(receipt PinaxVaultReceipt) {
	if receipt.VaultRoot == "" {
		return
	}
	for _, name := range receipt.Links {
		if controlPlaneName(name) {
			continue
		}
		p := filepath.Join(receipt.VaultRoot, name)
		info, err := os.Lstat(p)
		if err != nil || info.Mode()&os.ModeSymlink == 0 {
			continue
		}
		_ = os.Remove(p)
	}
	_ = os.Remove(filepath.Join(receipt.VaultRoot, pinaxVaultMarkerName))
}

func overlayAlive(st MountState) bool {
	if st.VaultRoot == "" {
		return false
	}
	info, err := os.Lstat(st.VaultRoot)
	if err != nil || !info.IsDir() {
		return false
	}
	if len(st.Links) == 0 {
		_, err := os.Stat(filepath.Join(st.VaultRoot, pinaxVaultMarkerName))
		return err == nil
	}
	for _, name := range st.Links {
		p := filepath.Join(st.VaultRoot, name)
		if _, err := os.Stat(p); err != nil {
			return false
		}
	}
	return true
}
