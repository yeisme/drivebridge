package bridge

import (
	"errors"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

// AdoptRecord is the CLI-authored zero-copy attach of an owner local directory
// or S3 prefix onto a DriveBridge space. Bytes are never copied; Copied is
// always false.
type AdoptRecord struct {
	ID             string    `json:"id" gorm:"primaryKey"`
	Consumer       string    `json:"consumer"`
	Kind           string    `json:"kind"`
	SpaceID        string    `json:"space" gorm:"column:space_id;uniqueIndex"`
	Location       string    `json:"location"`
	ConnectionID   string    `json:"connection_id,omitempty"`
	IdempotencyKey string    `json:"idempotency_key,omitempty" gorm:"uniqueIndex"`
	Copied         bool      `json:"copied"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

// AdoptRequest is the application-service input for storage adopt.
type AdoptRequest struct {
	Consumer       string
	Kind           string
	Space          string
	Root           string
	Remote         string
	RemotePath     string
	LocalRoot      string
	ConnectionID   string
	IdempotencyKey string
	MaxBytes       int64
}

// OwnerProtectedRel reports whether a space-relative path is owner metadata
// that DriveBridge must not mutate on an adopted space. List/stat stay allowed.
func OwnerProtectedRel(rel string) bool {
	rel = strings.TrimSpace(rel)
	if rel == "" {
		return false
	}
	rel = path.Clean(filepath.ToSlash(rel))
	rel = strings.TrimPrefix(rel, "/")
	if rel == "." || rel == "" {
		return false
	}
	parts := strings.Split(rel, "/")
	for i, part := range parts {
		if part == ".pinax" {
			return true
		}
		if part == ".scaena" {
			if i+2 < len(parts) && parts[i+1] == "objects" && parts[i+2] == "sha256" {
				continue
			}
			return true
		}
	}
	base := strings.ToLower(parts[len(parts)-1])
	if base == "token" || strings.HasSuffix(base, ".token") {
		return true
	}
	for _, suf := range []string{".sqlite", ".sqlite-wal", ".sqlite-shm", ".db", ".db-wal", ".db-shm"} {
		if strings.HasSuffix(base, suf) {
			return true
		}
	}
	return false
}

func joinRel(dir, name string) string {
	dir = path.Clean(filepath.ToSlash(dir))
	name = strings.Trim(filepath.ToSlash(name), "/")
	if dir == "." || dir == "" {
		return name
	}
	if name == "" || name == "." {
		return dir
	}
	return dir + "/" + name
}

func (s *Service) isAdopted(spaceID string) bool {
	if s == nil || s.DB == nil || spaceID == "" {
		return false
	}
	var n int64
	s.DB.Model(&AdoptRecord{}).Where("space_id = ?", spaceID).Limit(1).Count(&n)
	return n > 0
}

func (s *Service) rejectOwnerProtected(spaceID string, rels ...string) error {
	// Unlike advisory inventory, a mutation may not treat a registry read
	// failure as proof that the space is unprotected.
	if s == nil || s.DB == nil || spaceID == "" {
		return errors.New("guard_unavailable")
	}
	var n int64
	if err := s.DB.Model(&AdoptRecord{}).Where("space_id = ?", spaceID).Limit(1).Count(&n).Error; err != nil {
		return errors.New("guard_unavailable")
	}
	if n == 0 {
		return nil
	}
	for _, rel := range rels {
		if OwnerProtectedRel(rel) {
			return errors.New("owner_protected")
		}
	}
	return nil
}

// rejectOperationMutation revalidates the persisted operation, not a caller's
// replacement request. Recovery must keep the original operation and key.
func (s *Service) rejectOperationMutation(op Operation) error {
	switch op.Kind {
	case "upload", "save_output":
		if err := s.rejectOwnerProtected(op.SpaceID, op.Target); err != nil {
			return err
		}
		if op.Kind == "save_output" {
			return s.rejectLocalOwnerProtected(op.SpaceID, op.LocalPath)
		}
	case "download":
		return s.rejectLocalOwnerProtected(op.SpaceID, op.LocalPath)
	}
	return nil
}

// Local writes have a different root from provider writes. Compare physical
// locations against every local adoption rather than assuming space equality.
func (s *Service) rejectLocalOwnerProtected(spaceID, rel string) error {
	if s == nil || s.DB == nil {
		return errors.New("guard_unavailable")
	}
	var records []AdoptRecord
	if err := s.DB.Where("kind = ?", "local").Find(&records).Error; err != nil {
		return errors.New("guard_unavailable")
	}
	if len(records) == 0 {
		return nil
	}
	p, err := s.Space(spaceID)
	if err != nil {
		return err
	}
	target, err := canonicalMutationPath(filepath.Join(p.LocalRoot, rel))
	if err != nil {
		return err
	}
	for _, record := range records {
		root, err := canonicalMutationPath(record.Location)
		if err != nil {
			return err
		}
		// Keep both names: a metadata directory symlink must not turn its
		// original owner-protected name into an ordinary external path.
		for _, candidate := range []string{target, filepath.Join(p.LocalRoot, rel)} {
			same, targetInRoot, rootInTarget := sameOrNested(candidate, root)
			if same || rootInTarget {
				return errors.New("owner_protected")
			}
			if targetInRoot {
				relative, err := filepath.Rel(root, candidate)
				if err != nil {
					return errors.New("guard_unavailable")
				}
				if OwnerProtectedRel(relative) {
					return errors.New("owner_protected")
				}
			}
		}
	}
	return nil
}

// Resolve aliases even for a destination that does not exist yet. Broken
// links and inaccessible ancestors are not evidence of an unprotected path.
func canonicalMutationPath(name string) (string, error) {
	if !filepath.IsAbs(name) {
		return "", errors.New("guard_unavailable")
	}
	current := filepath.Clean(name)
	var suffix []string
	for {
		_, err := os.Lstat(current)
		if err == nil {
			resolved, err := filepath.EvalSymlinks(current)
			if err != nil {
				return "", errors.New("guard_unavailable")
			}
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved, nil
		}
		if !os.IsNotExist(err) || filepath.Dir(current) == current {
			return "", errors.New("guard_unavailable")
		}
		suffix = append(suffix, filepath.Base(current))
		current = filepath.Dir(current)
	}
}

// Whole-tree engines cannot enforce per-entry metadata exclusions. Refuse
// overlapping local/provider roots, including a different space's alias.
func (s *Service) rejectSyncMutation(p Space) error {
	if err := s.rejectOwnerProtected(p.ID, ".pinax"); err != nil {
		return err
	}
	var records []AdoptRecord
	if err := s.DB.Where("kind = ?", "local").Find(&records).Error; err != nil {
		return errors.New("guard_unavailable")
	}
	if len(records) == 0 {
		return nil
	}
	roots := []string{p.LocalRoot}
	if p.Kind() == "generic" {
		remote, err := AliasContentRoot(p.Remote)
		if err != nil {
			return errors.New("managed_scope_unverifiable")
		}
		roots = append(roots, filepath.Join(remote, p.RootID))
	}
	for _, candidate := range roots {
		root, err := canonicalMutationPath(candidate)
		if err != nil {
			return err
		}
		for _, record := range records {
			owner, err := canonicalMutationPath(record.Location)
			if err != nil {
				return err
			}
			same, inside, ancestor := sameOrNested(root, owner)
			if same || inside || ancestor {
				return errors.New("owner_protected")
			}
		}
	}
	return nil
}

// Adopts returns every adopt record, oldest space first.
func (s *Service) Adopts() []AdoptRecord {
	var out []AdoptRecord
	s.DB.Order("space_id").Find(&out)
	return out
}

// AdoptBySpace returns the adopt record for a space, if any.
func (s *Service) AdoptBySpace(spaceID string) (AdoptRecord, bool) {
	var rec AdoptRecord
	if s.DB.Where("space_id = ?", spaceID).First(&rec).Error != nil {
		return rec, false
	}
	return rec, true
}

// AdoptStorage registers a zero-copy view of an owner local directory or S3
// prefix. Same idempotency key and location reuses; same key and a different
// location is adopt_mismatch.
func (s *Service) AdoptStorage(req AdoptRequest) (*AdoptRecord, bool, error) {
	s.guardMu.Lock()
	defer s.guardMu.Unlock()
	if req.Kind != "local" && req.Kind != "s3" {
		return nil, false, errors.New("unsupported")
	}
	if strings.TrimSpace(req.Consumer) == "" || strings.TrimSpace(req.Space) == "" {
		return nil, false, errors.New("storage_missing_flag")
	}
	if strings.ContainsAny(req.Consumer, ":/\\") || strings.ContainsAny(req.Space, ":/\\") {
		return nil, false, errors.New("invalid_space")
	}
	if req.Remote == "" || strings.ContainsAny(req.Remote, ":/\\") {
		return nil, false, errors.New("invalid_space")
	}
	if req.MaxBytes <= 0 {
		req.MaxBytes = 1 << 30
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = "adopt:" + req.Consumer + ":" + req.Kind + ":" + req.Space
	}
	loc, localRoot, err := normalizeAdoptRequest(&req)
	if err != nil {
		return nil, false, err
	}
	req.LocalRoot = localRoot
	if req.ConnectionID != "" {
		if _, err = s.Connection(req.ConnectionID); err != nil {
			return nil, false, err
		}
	}

	var byKey AdoptRecord
	if s.DB.Where("idempotency_key = ?", req.IdempotencyKey).First(&byKey).Error == nil {
		if byKey.Location != loc || byKey.Kind != req.Kind || byKey.Consumer != req.Consumer || byKey.SpaceID != req.Space {
			return nil, false, errors.New("adopt_mismatch")
		}
		if err = ensureAdoptedSpace(s, req, loc); err != nil {
			return nil, false, err
		}
		return &byKey, true, nil
	}

	var bySpace AdoptRecord
	if s.DB.Where("space_id = ?", req.Space).First(&bySpace).Error == nil {
		if bySpace.Location != loc || bySpace.Kind != req.Kind {
			return nil, false, errors.New("adopt_mismatch")
		}
		return &bySpace, true, nil
	}

	if err = ensureAdoptedSpace(s, req, loc); err != nil {
		return nil, false, err
	}
	rec := AdoptRecord{
		ID: newID(), Consumer: req.Consumer, Kind: req.Kind, SpaceID: req.Space,
		Location: loc, ConnectionID: req.ConnectionID, IdempotencyKey: req.IdempotencyKey,
	}
	if err = s.DB.Create(&rec).Error; err != nil {
		return nil, false, errors.New("state_unavailable")
	}
	return &rec, false, nil
}

func normalizeAdoptRequest(req *AdoptRequest) (location, localRoot string, err error) {
	switch req.Kind {
	case "local":
		if req.Root == "" {
			return "", "", errors.New("storage_missing_flag")
		}
		root, e := filepath.EvalSymlinks(req.Root)
		if e != nil || !filepath.IsAbs(root) {
			return "", "", errors.New("invalid_space")
		}
		if info, e := os.Stat(root); e != nil || !info.IsDir() {
			return "", "", errors.New("invalid_space")
		}
		localRoot = req.LocalRoot
		if localRoot == "" {
			localRoot = root
		}
		if e := requireSameOrParent(localRoot, root); e != nil {
			return "", "", e
		}
		req.Root = root
		return filepath.Clean(root), localRoot, nil
	case "s3":
		if req.RemotePath == "" || req.LocalRoot == "" {
			return "", "", errors.New("storage_missing_flag")
		}
		loc, e := normalizeS3Location(req.RemotePath)
		if e != nil {
			return "", "", e
		}
		root, e := filepath.EvalSymlinks(req.LocalRoot)
		if e != nil || !filepath.IsAbs(root) {
			return "", "", errors.New("invalid_space")
		}
		if info, e := os.Stat(root); e != nil || !info.IsDir() {
			return "", "", errors.New("invalid_space")
		}
		return loc, root, nil
	default:
		return "", "", errors.New("unsupported")
	}
}

func normalizeS3Location(remotePath string) (string, error) {
	p := filepath.ToSlash(strings.TrimSpace(remotePath))
	p = strings.Trim(p, "/")
	if p == "" || strings.ContainsAny(p, ":\\") || !filepath.IsLocal(filepath.FromSlash(p)) {
		return "", errors.New("invalid_space")
	}
	return p, nil
}

func requireSameOrParent(localRoot, root string) error {
	localRoot, err := filepath.EvalSymlinks(localRoot)
	if err != nil || !filepath.IsAbs(localRoot) {
		return errors.New("invalid_space")
	}
	root, err = filepath.EvalSymlinks(root)
	if err != nil || !filepath.IsAbs(root) {
		return errors.New("invalid_space")
	}
	if info, err := os.Stat(localRoot); err != nil || !info.IsDir() {
		return errors.New("invalid_space")
	}
	rel, err := filepath.Rel(localRoot, root)
	if err != nil || strings.HasPrefix(rel, "..") {
		return errors.New("invalid_space")
	}
	return nil
}

func ensureAdoptedSpace(s *Service, req AdoptRequest, loc string) error {
	rootID := ""
	if req.Kind == "s3" {
		rootID = loc
	}
	existing, err := s.Space(req.Space)
	if err != nil {
		if err = s.AddSpace(Space{
			ID: req.Space, Remote: req.Remote, Backend: "generic",
			RootID: rootID, LocalRoot: req.LocalRoot, MaxBytes: req.MaxBytes,
		}); err == nil {
			return nil
		}
		if err.Error() != "space_conflict" {
			return err
		}
		existing, err = s.Space(req.Space)
		if err != nil {
			return errors.New("space_conflict")
		}
	}
	if !spaceMatchesAdopt(existing, req, loc) {
		return errors.New("adopt_mismatch")
	}
	return nil
}

func spaceMatchesAdopt(p Space, req AdoptRequest, loc string) bool {
	if p.Kind() != "generic" {
		return false
	}
	switch req.Kind {
	case "local":
		return AliasRemoteMatches(p.Remote, req.Root) || (p.Remote == req.Remote && p.RootID == "")
	case "s3":
		return p.Remote == req.Remote && filepath.ToSlash(strings.Trim(p.RootID, "/")) == loc
	}
	return false
}
