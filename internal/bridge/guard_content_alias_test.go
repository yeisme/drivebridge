package bridge

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configfile"
	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"github.com/yeisme/drivebridge-server/pkg/managedstate"
)

// A generic (rclone alias) space has two physical roots: the local root and the
// alias content root (`AliasContentRoot(remote) + RootID`). Guard admission now
// refuses an owner-state directory inside an adopted local root, but the content
// root is not an `AdoptRecord`, so the same hole stays open through the alias.
//
// The fenced database, its inert marker and every `.managed-state-v1` snapshot
// are written by this application, so a state inside the content root would
// mutate the protected domain through a path that is not an admitted space
// target. `guardDomain` must compare the state against both roots it claims.
func TestManagedVaultGuardRefusesStateInsideAliasContentRoot(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("managed guard identity is Linux-only until platform acceptance")
	}
	dir := t.TempDir()
	content := filepath.Join(dir, "remote-files")
	if err := os.Mkdir(content, 0700); err != nil {
		t.Fatal(err)
	}
	conf := filepath.Join(dir, "rclone.conf")
	// The alias backend pins the remote root to a temp directory, standing in for
	// any configured remote exactly like genericFixture.
	body := "[nas]\ntype = alias\nremote = " + content + "\n"
	if err := os.WriteFile(conf, []byte(body), 0600); err != nil {
		t.Fatal(err)
	}
	configfile.Install()
	if err := config.SetConfigPath(conf); err != nil {
		t.Fatal(err)
	}
	local := filepath.Join(dir, "local")
	if err := os.Mkdir(local, 0700); err != nil {
		t.Fatal(err)
	}

	// The state sits inside the alias content root, not inside the adopted local
	// root, so the adopt-record predicate alone cannot see it.
	state := filepath.Join(content, "state")
	first, err := New(state, NewOneDrive(), false)
	if err != nil {
		t.Fatal(err)
	}
	if err = first.AddSpace(Space{ID: "nas", Remote: "nas", Backend: "generic", LocalRoot: local, MaxBytes: 1 << 20}); err != nil {
		first.Close()
		t.Fatal(err)
	}
	if err = first.DB.Create(&AdoptRecord{ID: "guard-owner", Consumer: "pinax", Kind: "local", SpaceID: "nas", Location: local, IdempotencyKey: "guard-owner"}).Error; err != nil {
		first.Close()
		t.Fatal(err)
	}
	first.Close()
	if err = os.WriteFile(filepath.Join(state, "process.lock"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	plan, err := managedstate.Prepare(state, "cli", "migration-original")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = managedstate.Install(state, plan); err != nil {
		t.Fatal(err)
	}
	svc, err := New(state, NewOneDrive(), false)
	if err != nil {
		t.Fatal(err)
	}
	defer svc.Close()

	request := GuardPreviewRequest{InstanceID: "instance-1", SourceID: "source-1", SpaceID: "nas", Scopes: []desktopcontract.ReaderScope{desktopcontract.ScopeMetadata}}
	if _, err := svc.PreviewGuard(request); err == nil || err.Error() != "owner_protected" {
		t.Fatalf("preview accepted a state inside the alias content root: %v", err)
	}
	if _, err := svc.InstallGuard(GuardInstallRequest{PreviewID: "guard-alias", Digest: "sha256:unused", BindingID: "guard-alias", InstanceID: "instance-1", SourceID: "source-1", SpaceID: "nas", Scopes: request.Scopes, Confirmed: true, IdempotencyKey: "confirm-alias"}); err == nil || err.Error() != "owner_protected" {
		t.Fatalf("confirm accepted a state inside the alias content root: %v", err)
	}
	var persisted int64
	if err := svc.DB.Model(&ManagedGuard{}).Count(&persisted).Error; err != nil {
		t.Fatal(err)
	}
	if persisted != 0 {
		t.Fatalf("a refused confirm persisted %d guard row(s)", persisted)
	}
}
