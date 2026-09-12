package bridge

// Generic spaces rely on rclone's registry; backends register themselves via
// init() when their package is imported. OneDrive is imported by onedrive.go
// and keeps its privileged handoff path; the curated set below covers common
// personal cloud drives, NAS protocols and object storage for transfer-only
// spaces. Adding another backend is an import line plus a regression note —
// no other wiring is needed.
import (
	_ "github.com/rclone/rclone/backend/alias"
	_ "github.com/rclone/rclone/backend/box"
	_ "github.com/rclone/rclone/backend/drive"
	_ "github.com/rclone/rclone/backend/dropbox"
	_ "github.com/rclone/rclone/backend/local"
	_ "github.com/rclone/rclone/backend/pcloud"
	_ "github.com/rclone/rclone/backend/s3"
	_ "github.com/rclone/rclone/backend/sftp"
	_ "github.com/rclone/rclone/backend/smb"
	_ "github.com/rclone/rclone/backend/webdav"
)
