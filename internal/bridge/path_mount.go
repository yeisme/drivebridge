package bridge

import (
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	rconfig "github.com/rclone/rclone/fs/config"
)

const rcloneMountWait = 15 * time.Second

// rcloneExec runs the rclone CLI. Tests replace this; production never
// interpolates stdout/stderr into DriveBridge envelopes.
var rcloneExec = runRcloneCLI

func runRcloneCLI(args ...string) error {
	bin, err := exec.LookPath("rclone")
	if err != nil {
		return errors.New("mount_path_adapter_missing")
	}
	cmd := exec.Command(bin, args...)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		return errors.New("mount_path_adapter_missing")
	}
	return nil
}

// RcloneMountSource is remote:path for `rclone mount`. remote-path is the
// space RootID (S3: bucket/prefix). Traversal is rejected.
func RcloneMountSource(remote, remotePath string) (string, error) {
	if remote == "" || strings.ContainsAny(remote, ":/\\") {
		return "", errors.New("invalid_space")
	}
	p := strings.Trim(filepath.ToSlash(remotePath), "/")
	if p != "" && !filepath.IsLocal(p) {
		return "", errors.New("invalid_space")
	}
	return remote + ":" + p, nil
}

func s3RemoteType(remote string) bool {
	return strings.EqualFold(rconfig.GetValue(remote, "type"), "s3")
}

func rcloneMountDir(stateDir, spaceID string) (string, error) {
	if spaceID == "" || spaceID != filepath.Base(spaceID) || spaceID == "." || spaceID == ".." {
		return "", errors.New("invalid_space")
	}
	return filepath.Join(stateDir, "mnt", spaceID), nil
}

// ContentRoot is a prepared directory DriveBridge can overlay into vault-root.
type ContentRoot struct {
	Path    string
	Adapter string
	unmount func()
}

// Close unmounts an rclone path adapter. Alias roots are a no-op.
func (c ContentRoot) Close() {
	if c.unmount != nil {
		c.unmount()
	}
}

// PrepareSpaceContentRoot 解析空间的本机内容根：alias 直接用目录；
// rclone type=s3 则 exec 本机 rclone CLI 挂到 state/mnt/<space>（不链 libfuse）。
func PrepareSpaceContentRoot(space Space, stateDir, rcloneConfig string) (ContentRoot, error) {
	if dir, err := AliasContentRoot(space.Remote); err == nil {
		return ContentRoot{Path: dir, Adapter: "pinax-vault"}, nil
	}
	if !s3RemoteType(space.Remote) {
		return ContentRoot{}, errors.New("mount_path_adapter_missing")
	}
	return mountS3WithRclone(space, stateDir, rcloneConfig)
}

func mountS3WithRclone(space Space, stateDir, rcloneConfig string) (ContentRoot, error) {
	src, err := RcloneMountSource(space.Remote, space.RootID)
	if err != nil {
		return ContentRoot{}, err
	}
	target, err := rcloneMountDir(stateDir, space.ID)
	if err != nil {
		return ContentRoot{}, err
	}
	if err := os.MkdirAll(target, 0o700); err != nil {
		return ContentRoot{}, errors.New("state_unavailable")
	}
	logFile := filepath.Join(stateDir, "mnt", space.ID+".rclone.log")
	args := []string{
		"mount", src, target,
		"--vfs-cache-mode", "writes",
		"--dir-cache-time", "5s",
		"--umask", "077",
		"--daemon",
		"--log-level", "ERROR",
		"--log-file", logFile,
	}
	if rcloneConfig != "" {
		args = append(args, "--config", rcloneConfig)
	}
	if err := rcloneExec(args...); err != nil {
		_ = os.Remove(target)
		return ContentRoot{}, err
	}
	if err := waitDirReady(target, rcloneMountWait); err != nil {
		unmountRclonePath(target, rcloneConfig)
		return ContentRoot{}, errors.New("mount_path_adapter_missing")
	}
	var once sync.Once
	return ContentRoot{
		Path:    target,
		Adapter: "pinax-vault-rclone",
		unmount: func() {
			once.Do(func() { unmountRclonePath(target, rcloneConfig) })
		},
	}, nil
}

func waitDirReady(dir string, d time.Duration) error {
	deadline := time.Now().Add(d)
	for {
		if entries, err := os.ReadDir(dir); err == nil {
			_ = entries
			return nil
		}
		if time.Now().After(deadline) {
			return errors.New("mount_path_adapter_missing")
		}
		time.Sleep(50 * time.Millisecond)
	}
}

// UnmountRclonePath releases an rclone FUSE mount. Failures stay local; stop
// still removes the Pinax overlay so the vault path is not left half-mounted.
func UnmountRclonePath(target, rcloneConfig string) {
	unmountRclonePath(target, rcloneConfig)
}

func unmountRclonePath(target, rcloneConfig string) {
	if target == "" || !filepath.IsAbs(target) {
		return
	}
	args := []string{"umount", target}
	if rcloneConfig != "" {
		args = append(args, "--config", rcloneConfig)
	}
	_ = rcloneExec(args...)
}
