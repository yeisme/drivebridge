// Package bridge implements the pre-1.0 local file control contract.
package bridge

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Space struct {
	ID         string `json:"id" gorm:"primaryKey"`
	Remote     string `json:"-"`
	Backend    string `json:"backend,omitempty"`
	DriveID    string `json:"drive_id"`
	RootID     string `json:"root_id"`
	LocalRoot  string `json:"local_root"`
	MaxBytes   int64  `json:"max_bytes"`
	Project    string `json:"project"`
	Actor      string `json:"actor"`
	FollowSymlinks bool `json:"follow_symlinks"`
}

// Kind reports the provider family: "" and "onedrive" are the privileged
// OneDrive path (direct URL handoff included); any other backend kind is a
// transfer-only rclone remote.
func (p Space) Kind() string {
	if p.Backend == "" || p.Backend == "onedrive" {
		return "onedrive"
	}
	return p.Backend
}
type File struct {
	Ref      string `json:"ref" gorm:"primaryKey"`
	SpaceID  string `json:"space_id"`
	DriveID  string `json:"drive_id"`
	ItemID   string `json:"item_id"`
	Version  string `json:"version"`
	Name     string `json:"name"`
	Size     int64  `json:"size_bytes"`
	SHA256   string `json:"sha256,omitempty"`
	MIME     string `json:"mime_type"`
	Duration int    `json:"duration_seconds,omitempty"`
	Verified bool   `json:"-"`
}
type Operation struct {
	SourceRef   string    `json:"source_ref,omitempty"`
	ID          string    `json:"id" gorm:"primaryKey"`
	Key         string    `json:"-" gorm:"uniqueIndex"`
	SpaceID     string    `json:"space_id"`
	Kind        string    `json:"kind"`
	Status      string    `json:"status"`
	LocalPath   string    `json:"-"`
	Target      string    `json:"-"`
	FileRef     string    `json:"file_ref,omitempty"`
	Fingerprint string    `json:"-"`
	Bytes       int64     `json:"bytes_total"`
	Transferred int64     `json:"bytes_transferred"`
	Error       string    `json:"error,omitempty"`
	Updated     time.Time `json:"updated_at"`
}

// Provider errors must be codes; raw transport errors never cross this boundary.
type Provider interface {
	List(context.Context, Space) ([]File, error)
	Stat(context.Context, Space, string) (File, error)
	Upload(context.Context, Space, string, io.Reader, int64) (File, error)
	Open(context.Context, Space, File) (io.ReadCloser, error)
	Access(context.Context, Space, File) (string, error)
}
type Service struct {
	DB       *gorm.DB
	Provider Provider
	Generic  Provider
	Output   OutputSource
	Enabled  bool
	Handoff  bool
	mu       sync.Mutex
	jobs     map[string]context.CancelFunc
	wg       sync.WaitGroup
}

// providerFor routes each space to its provider family; OneDrive keeps the
// default provider (tests inject fakes there) and every other backend kind
// uses the transfer-only generic rclone provider.
func (s *Service) providerFor(p Space) Provider {
	if p.Kind() != "onedrive" && s.Generic != nil {
		return s.Generic
	}
	return s.Provider
}

func New(state string, provider Provider, enabled bool) (*Service, error) {
	if err := os.MkdirAll(state, 0700); err != nil {
		return nil, errors.New("state_unavailable")
	}
	info, statErr := os.Lstat(state)
	if statErr != nil || !info.IsDir() || info.Mode().Perm()&0077 != 0 {
		return nil, errors.New("state_not_private")
	}
	db, err := gorm.Open(sqlite.Open(filepath.Join(state, "state.sqlite")), &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)})
	if err != nil {
		return nil, errors.New("state_unavailable")
	}
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}
	sqlDB.SetMaxOpenConns(1)
	if err = db.AutoMigrate(&Space{}, &File{}, &Operation{}); err != nil {
		return nil, errors.New("state_unavailable")
	}
	if err = os.Chmod(filepath.Join(state, "state.sqlite"), 0600); err != nil {
		return nil, err
	}
	// A process lock is held by the composition root before opening this state.
	if err = db.Model(&Operation{}).Where("status IN ?", []string{"transferring", "queued"}).Updates(map[string]any{"status": "interrupted", "error": "resume_required"}).Error; err != nil {
		return nil, err
	}
	if err = db.Model(&Operation{}).Where("status = ?", "cancelling").Updates(map[string]any{"status": "cancelled", "error": "operation_cancelled"}).Error; err != nil {
		return nil, err
	}
	return &Service{DB: db, Provider: provider, Generic: NewGeneric(), Enabled: enabled, jobs: map[string]context.CancelFunc{}}, nil
}
func (s *Service) Close() {
	s.mu.Lock()
	for _, cancel := range s.jobs {
		cancel()
	}
	s.mu.Unlock()
	s.wg.Wait()
	db, _ := s.DB.DB()
	_ = db.Close()
}
func (s *Service) Space(id string) (Space, error) {
	var p Space
	if s.DB.First(&p, "id = ?", id).Error != nil {
		return p, errors.New("space_not_allowed")
	}
	return p, nil
}
func (s *Service) Spaces() []Space { var out []Space; s.DB.Find(&out); return out }
func (s *Service) AddSpace(p Space) error {
	root, err := filepath.EvalSymlinks(p.LocalRoot)
	if err != nil || !filepath.IsAbs(root) || p.ID == "" || strings.ContainsAny(p.Remote, ":/\\") || p.Remote == "" || p.MaxBytes <= 0 {
		return errors.New("invalid_space")
	}
	switch p.Kind() {
	case "onedrive":
		if p.DriveID == "" || p.RootID == "" {
			return errors.New("invalid_space")
		}
	case "generic":
		// The remote root path is optional (empty = the remote's own root) but
		// must stay inside the remote: no absolute paths, no traversal.
		if p.RootID != "" && !filepath.IsLocal(p.RootID) {
			return errors.New("invalid_space")
		}
		// Reuse the identity fields: a stable drive label plus the root path.
		p.DriveID = "rclone:" + p.Remote
	default:
		return errors.New("invalid_space")
	}
	info, err := os.Stat(root)
	if err != nil || !info.IsDir() {
		return errors.New("invalid_local_root")
	}
	p.LocalRoot = root
	// Existing space identities cannot be silently rebound.
	if err = s.DB.Create(&p).Error; err != nil {
		return errors.New("space_conflict")
	}
	return nil
}
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func digestReader(r io.Reader) (string, int64, error) {
	h := sha256.New()
	n, err := io.Copy(h, r)
	return hex.EncodeToString(h.Sum(nil)), n, err
}
func safeName(n string) bool {
	if n == "" || n == "." || n == ".." || filepath.Base(n) != n || strings.ContainsAny(n, "/\\:\"<>|?*") || strings.HasSuffix(n, ".") || strings.HasSuffix(n, " ") || len(n) > 180 {
		return false
	}
	for _, r := range n {
		if r < 32 {
			return false
		}
	}
	return true
}
func localPath(p Space, name string) (string, error) {
	if filepath.IsAbs(name) {
		rel, err := filepath.Rel(p.LocalRoot, name)
		if err != nil {
			return "", errors.New("path_not_allowed")
		}
		name = rel
	}
	if !filepath.IsLocal(name) {
		return "", errors.New("path_not_allowed")
	}
	return name, nil
}
func (s *Service) remember(p Space, f File) (File, error) {
	if f.ItemID == "" || f.Version == "" || f.DriveID != p.DriveID || f.Size <= 0 || f.Size > p.MaxBytes {
		return File{}, errors.New("invalid_file_metadata")
	}
	sum := sha256.Sum256([]byte(p.ID + "\x00" + f.DriveID + "\x00" + f.ItemID + "\x00" + f.Version))
	f.Ref = "drivebridge://file/" + hex.EncodeToString(sum[:])
	f.SpaceID = p.ID
	// A reference pins an immutable version: re-observing it through a plain
	// listing must not drop the cached digest or the verified media probe.
	var old File
	if s.DB.First(&old, "ref = ?", f.Ref).Error == nil {
		if f.SHA256 == "" {
			f.SHA256 = old.SHA256
		}
		if f.MIME == "" {
			f.MIME, f.Duration = old.MIME, old.Duration
		}
		f.Verified = f.Verified || old.Verified
	}
	if err := s.DB.Save(&f).Error; err != nil {
		return File{}, errors.New("state_unavailable")
	}
	return f, nil
}
func (s *Service) List(ctx context.Context, space string) ([]File, error) {
	if !s.Enabled {
		return nil, errors.New("disabled")
	}
	p, e := s.Space(space)
	if e != nil {
		return nil, e
	}
	items, e := s.providerFor(p).List(ctx, p)
	if e != nil {
		return nil, e
	}
	out := []File{}
	for _, f := range items {
		if f.Size <= 0 || f.Size > p.MaxBytes {
			continue
		}
		f, e = s.remember(p, f)
		if e != nil {
			return nil, e
		}
		out = append(out, f)
	}
	return out, nil
}
func (s *Service) Stat(ctx context.Context, ref string) (File, error) {
	if !s.Enabled {
		return File{}, errors.New("disabled")
	}
	var old File
	if s.DB.First(&old, "ref = ?", ref).Error != nil {
		return File{}, errors.New("reference_not_found")
	}
	p, e := s.Space(old.SpaceID)
	if e != nil {
		return File{}, e
	}
	f, e := s.providerFor(p).Stat(ctx, p, old.ItemID)
	if e != nil {
		return File{}, e
	}
	if f.Version != old.Version || f.Size != old.Size || f.DriveID != old.DriveID {
		return File{}, errors.New("file_version_changed")
	}
	f.Ref = ref
	f.SpaceID = p.ID
	f.SHA256 = old.SHA256
	return f, nil
}
func (s *Service) Get(id string) (Operation, error) {
	var op Operation
	if s.DB.First(&op, "id = ?", id).Error != nil {
		return op, errors.New("operation_not_found")
	}
	return op, nil
}
