package bridge

import (
	"errors"
	"io"
	"os"
)

// casRoot is the content-addressed download cache inside each space's local
// root. It is an implementation detail: never listed in user trees, never
// treated as authority over the pinned remote version.
const casRoot = ".drivebridge-cas"

// casPath maps a digest to its fan-out location: sha256/<aa>/<bb>/<digest>.
func casPath(sha string) string {
	if len(sha) != 64 {
		return ""
	}
	for _, r := range sha {
		if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') {
			return ""
		}
	}
	return casRoot + "/sha256/" + sha[:2] + "/" + sha[2:4] + "/" + sha
}

// casOpen returns the cached bytes only when the stored file still hashes to
// the expected digest; a corrupted or foreign entry is invisible.
func casOpen(root *os.Root, sha string, maxBytes int64) (*os.File, error) {
	rel := casPath(sha)
	if rel == "" {
		return nil, errors.New("cache_miss")
	}
	f, err := root.Open(rel)
	if err != nil {
		return nil, errors.New("cache_miss")
	}
	info, err := f.Stat()
	if err != nil || !info.Mode().IsRegular() || info.Size() <= 0 || info.Size() > maxBytes {
		f.Close()
		return nil, errors.New("cache_miss")
	}
	h, _, err := digestReader(io.LimitReader(f, maxBytes+1))
	if err != nil || h != sha {
		f.Close()
		return nil, errors.New("cache_miss")
	}
	if _, err = f.Seek(0, 0); err != nil {
		f.Close()
		return nil, errors.New("cache_miss")
	}
	return f, nil
}

// casStore publishes an already-verified temp file into the cache by hard
// link; cache population is best-effort and never fails the download.
func casStore(root *os.Root, sha, temp string) {
	rel := casPath(sha)
	if rel == "" {
		return
	}
	if err := root.MkdirAll(rel[:len(rel)-len(sha)-1], 0700); err != nil {
		return
	}
	// Link keeps the existing entry on collision; identical content needs no
	// replacement and a digest collision is refused rather than overwritten.
	_ = root.Link(temp, rel)
}
