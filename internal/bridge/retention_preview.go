package bridge

import (
	"context"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RetentionClass is one cleanup category. Classes are previewed separately
// and never merged: staging payloads, local trash, VFS cache state and
// remote references have different owners and different recovery rules.
type RetentionClass struct {
	Class     string `json:"class"`
	Files     int    `json:"files"`
	Bytes     int64  `json:"bytes"`
	Cleanable bool   `json:"cleanable"`
	Protected bool   `json:"protected"`
	Note      string `json:"note,omitempty"`
}

// RetentionPreviewReport is the read-only classification for one space. It
// never mutates anything: cleanup decisions stay with the user and protected
// paths stay with their domain owners.
type RetentionPreviewReport struct {
	Space              string           `json:"space_id"`
	Classes            []RetentionClass `json:"classes"`
	MountAlive         bool             `json:"mount_alive"`
	MountAliveNote     string           `json:"mount_alive_note"`
	ProtectedPathsNote string           `json:"protected_paths_note,omitempty"`
	GeneratedAt        time.Time        `json:"generated_at"`
}

// rcloneVFSRoot returns the host rclone VFS cache root (best effort). Mount
// adapters run the rclone CLI with --vfs-cache-mode writes and the default
// cache location.
// vfsCacheRootOverride lets tests pin the cache root deterministically;
// production leaves it empty.
var vfsCacheRootOverride string

func rcloneVFSRoot() (string, bool) {
	if vfsCacheRootOverride != "" {
		return vfsCacheRootOverride, true
	}
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", false
	}
	return filepath.Join(cacheDir, "rclone", "vfs"), true
}

func dirUsage(root string) (files int, bytes int64) {
	_ = filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil || path == root {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, statErr := entry.Info()
		if statErr != nil {
			return nil
		}
		files++
		bytes += info.Size()
		return nil
	})
	return files, bytes
}

// vfsClasses scans rclone VFS cache directories tied to this space's remote.
// Unflushed upload bytes are protected: a mount being alive proves nothing
// about remote persistence, and pending writes are never cleanable cache.
func vfsClasses(remote string) []RetentionClass {
	root, ok := rcloneVFSRoot()
	if !ok || remote == "" {
		return nil
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		return nil
	}
	var pending, rebuildable RetentionClass
	pending.Class, rebuildable.Class = "vfs_pending_upload", "vfs_cache_rebuildable"
	pending.Protected = true
	pending.Note = "Unflushed VFS upload bytes are protected and never listed as cleanable."
	rebuildable.Cleanable = true
	rebuildable.Note = "Rebuildable cache blocks (data/meta) can be re-fetched from the remote."
	for _, entry := range entries {
		// rclone v1.75 nests each remote's cache under its exact directory
		// name (remote name, then the mounted root path below it). Only an
		// exact name match attributes cache bytes: a prefix rule would
		// cross-account sibling remotes like nas and nas-backup.
		if !entry.IsDir() || entry.Name() != remote {
			continue
		}
		cacheDir := filepath.Join(root, entry.Name())
		// rclone nests one directory per mounted subpath; the role segment
		// (upload/data/meta) can sit at any depth under the cache dir.
		_ = filepath.WalkDir(cacheDir, func(path string, de fs.DirEntry, err error) error {
			if err != nil || path == cacheDir || de.IsDir() {
				return nil
			}
			rel, relErr := filepath.Rel(cacheDir, path)
			if relErr != nil {
				return nil
			}
			info, statErr := de.Info()
			if statErr != nil {
				return nil
			}
			segments := strings.Split(rel, string(filepath.Separator))
			role := ""
			for _, segment := range segments[:len(segments)-1] {
				if segment == "upload" || segment == "data" || segment == "meta" {
					role = segment
					break
				}
			}
			switch role {
			case "upload":
				pending.Files++
				pending.Bytes += info.Size()
			case "data", "meta":
				rebuildable.Files++
				rebuildable.Bytes += info.Size()
			}
			return nil
		})
	}
	out := []RetentionClass{}
	if pending.Files > 0 {
		out = append(out, pending)
	}
	if rebuildable.Files > 0 {
		out = append(out, rebuildable)
	}
	return out
}

// RetentionPreview classifies what a manual cleanup could touch for one
// space, without touching anything. Server-side history and versions belong
// to the backend-server retention contracts and are reported read-only.
func (s *Service) RetentionPreview(ctx context.Context, space string) (RetentionPreviewReport, error) {
	p, err := s.Space(space)
	if err != nil {
		return RetentionPreviewReport{}, err
	}
	report := RetentionPreviewReport{
		Space:          space,
		GeneratedAt:    time.Now().UTC(),
		MountAliveNote: "mount_alive reports the local mount only; it does not prove remote persistence.",
	}
	if status, ok := s.MountStatus()["alive"].(bool); ok {
		report.MountAlive = status
	}
	// Browser staging payloads wait for their upload operation.
	stagingFiles, stagingBytes := dirUsage(filepath.Join(p.LocalRoot, ".drivebridge-staging"))
	report.Classes = append(report.Classes, RetentionClass{
		Class:     "staging",
		Files:     stagingFiles,
		Bytes:     stagingBytes,
		Cleanable: true,
		Note:      "Browser upload staging payloads; verify the owning operation finished before removing.",
	})
	// Local trash: trashed references stay restorable until purged by the owner surface.
	var trashFiles, trashBytes int64
	if err := s.DB.Model(&File{}).Where("space_id = ? AND trashed_at IS NOT NULL", space).Count(&trashFiles).Error; err != nil {
		return RetentionPreviewReport{}, errors.New("state_unavailable")
	}
	if err := s.DB.Model(&File{}).Where("space_id = ? AND trashed_at IS NOT NULL", space).Select("COALESCE(SUM(size),0)").Scan(&trashBytes).Error; err != nil {
		return RetentionPreviewReport{}, errors.New("state_unavailable")
	}
	if trashFiles > 0 || trashBytes > 0 {
		report.Classes = append(report.Classes, RetentionClass{
			Class:     "local_trash",
			Files:     int(trashFiles),
			Bytes:     trashBytes,
			Cleanable: true,
			Note:      "Trashed references; restore before any purge decision.",
		})
	}
	// rclone VFS cache split by flush state.
	report.Classes = append(report.Classes, vfsClasses(p.Remote)...)
	// Remote references are outside local cleanup scope.
	var liveFiles int64
	if err := s.DB.Model(&File{}).Where("space_id = ? AND trashed_at IS NULL", space).Count(&liveFiles).Error; err != nil {
		return RetentionPreviewReport{}, errors.New("state_unavailable")
	}
	report.Classes = append(report.Classes, RetentionClass{
		Class:     "remote",
		Files:     int(liveFiles),
		Cleanable: false,
		Note:      "Remote files and their history belong to the server retention contracts; this preview never modifies them.",
	})
	// Domain-owner protection continues to outrank any local cleanup policy.
	if s.isAdopted(space) {
		report.ProtectedPathsNote = "Owner-protected paths (.pinax, databases, Capsa envelopes) reject generic cleanup writes regardless of this preview."
	}
	return report, nil
}
