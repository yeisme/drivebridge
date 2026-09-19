package bridge

import (
	"context"
	"errors"
	"path"
	"path/filepath"
	"sort"
	"strings"
	"time"

	rconfig "github.com/rclone/rclone/fs/config"
)

// AllowedBindPurposes is the first-batch purpose whitelist.
var AllowedBindPurposes = map[string]bool{
	"vault": true, "notes-attachments": true, "materials": true,
	"saveback": true, "owner-store": true,
}

// BindRecord is a CLI-authored project binding onto a space prefix.
// Consumer and project_ref are stored facts, not authentication.
type BindRecord struct {
	ID             string     `json:"bind_id" gorm:"primaryKey"`
	Consumer       string     `json:"consumer"`
	ProjectRef     string     `json:"project_ref"`
	SpaceID        string     `json:"space" gorm:"column:space_id"`
	Prefix         string     `json:"prefix"`
	Purpose        string     `json:"purpose"`
	ConnectionID   string     `json:"connection_id,omitempty"`
	IdempotencyKey string     `json:"idempotency_key,omitempty" gorm:"uniqueIndex"`
	RevokedAt      *time.Time `json:"revoked_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
	UpdatedAt      time.Time  `json:"updated_at"`
}

// BindRequest is the application-service input for bind add.
type BindRequest struct {
	Consumer       string
	ProjectRef     string
	Space          string
	Prefix         string
	Purpose        string
	ConnectionID   string
	IdempotencyKey string
	AllowGeneric   bool
}

// BindScope selects a bind for prefix fencing. Labels are not proof of identity.
type BindScope struct {
	ID         string
	Consumer   string
	ProjectRef string
}

type bindScopeKey struct{}

// WithBindScope attaches a caller-declared bind scope to file operations.
func WithBindScope(ctx context.Context, scope BindScope) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if scope.ID == "" && scope.Consumer == "" && scope.ProjectRef == "" {
		return ctx
	}
	return context.WithValue(ctx, bindScopeKey{}, scope)
}

func bindScopeFrom(ctx context.Context) (BindScope, bool) {
	if ctx == nil {
		return BindScope{}, false
	}
	scope, ok := ctx.Value(bindScopeKey{}).(BindScope)
	if !ok || (scope.ID == "" && scope.Consumer == "" && scope.ProjectRef == "") {
		return BindScope{}, false
	}
	return scope, true
}

// BindDiscoverSpace is one default-discovery row (no credentials).
type BindDiscoverSpace struct {
	Space       string `json:"space"`
	Kind        string `json:"kind"`
	Tier        string `json:"tier,omitempty"`
	BindDefault bool   `json:"bind_default"`
	Adopted     bool   `json:"adopted"`
	Location    string `json:"location,omitempty"`
}

// NormalizeBindPrefix cleans a workspace-relative bind prefix. Empty, "." and
// "/" mean the whole space.
func NormalizeBindPrefix(prefix string) (string, error) {
	p := strings.TrimSpace(filepath.ToSlash(prefix))
	p = strings.Trim(p, "/")
	if p == "" || p == "." {
		return "", nil
	}
	cleaned := path.Clean(p)
	if cleaned == "." {
		return "", nil
	}
	if cleaned == ".." || strings.HasPrefix(cleaned, "../") || strings.Contains(cleaned, ":") {
		return "", errors.New("path_not_allowed")
	}
	if !filepath.IsLocal(filepath.FromSlash(cleaned)) {
		return "", errors.New("path_not_allowed")
	}
	return cleaned, nil
}

// BindPrefixContains reports whether rel is the prefix directory or a descendant.
func BindPrefixContains(prefix, rel string) bool {
	prefix, err := NormalizeBindPrefix(prefix)
	if err != nil {
		return false
	}
	rel = strings.Trim(filepath.ToSlash(strings.TrimSpace(rel)), "/")
	if rel == "" || rel == "." {
		rel = ""
	} else {
		rel = path.Clean(rel)
		if rel == "." {
			rel = ""
		}
		if rel == ".." || strings.HasPrefix(rel, "../") {
			return false
		}
	}
	if prefix == "" {
		return true
	}
	if rel == "" {
		return false
	}
	return rel == prefix || strings.HasPrefix(rel, prefix+"/")
}

func (s *Service) Bind(id string) (BindRecord, error) {
	var rec BindRecord
	if s == nil || s.DB == nil || id == "" || s.DB.First(&rec, "id = ?", id).Error != nil {
		return rec, errors.New("bind_not_found")
	}
	return rec, nil
}

// Binds returns every bind record, oldest first.
func (s *Service) Binds() []BindRecord {
	var out []BindRecord
	if s == nil || s.DB == nil {
		return out
	}
	s.DB.Order("created_at, id").Find(&out)
	return out
}

// AddBind registers a consumer/project binding onto a space prefix.
func (s *Service) AddBind(req BindRequest) (*BindRecord, bool, error) {
	if strings.TrimSpace(req.Consumer) == "" || strings.TrimSpace(req.ProjectRef) == "" || strings.TrimSpace(req.Space) == "" || strings.TrimSpace(req.Purpose) == "" {
		return nil, false, errors.New("bind_missing_flag")
	}
	if strings.ContainsAny(req.Consumer, ":/\\") || strings.ContainsAny(req.ProjectRef, ":/\\") || strings.ContainsAny(req.Space, ":/\\") {
		return nil, false, errors.New("invalid_space")
	}
	if !AllowedBindPurposes[req.Purpose] {
		return nil, false, errors.New("bind_purpose_unknown")
	}
	prefix, err := NormalizeBindPrefix(req.Prefix)
	if err != nil {
		return nil, false, err
	}
	if _, err = s.Space(req.Space); err != nil {
		return nil, false, err
	}
	if err = s.rejectEscapeBind(req.Space, req.AllowGeneric); err != nil {
		return nil, false, err
	}
	if req.ConnectionID != "" {
		if _, err = s.Connection(req.ConnectionID); err != nil {
			return nil, false, err
		}
	}
	if req.IdempotencyKey == "" {
		req.IdempotencyKey = "bind:" + req.Consumer + ":" + req.ProjectRef + ":" + req.Space + ":" + prefix + ":" + req.Purpose
	}

	var byKey BindRecord
	if s.DB.Where("idempotency_key = ?", req.IdempotencyKey).First(&byKey).Error == nil {
		if !bindFieldsMatch(byKey, req, prefix) {
			return nil, false, errors.New("bind_mismatch")
		}
		return &byKey, true, nil
	}

	rec := BindRecord{
		ID: newID(), Consumer: req.Consumer, ProjectRef: req.ProjectRef,
		SpaceID: req.Space, Prefix: prefix, Purpose: req.Purpose,
		ConnectionID: req.ConnectionID, IdempotencyKey: req.IdempotencyKey,
	}
	if err = s.DB.Create(&rec).Error; err != nil {
		return nil, false, errors.New("state_unavailable")
	}
	return &rec, false, nil
}

func bindFieldsMatch(rec BindRecord, req BindRequest, prefix string) bool {
	return rec.Consumer == req.Consumer && rec.ProjectRef == req.ProjectRef &&
		rec.SpaceID == req.Space && rec.Prefix == prefix && rec.Purpose == req.Purpose &&
		rec.ConnectionID == req.ConnectionID
}

func (s *Service) rejectEscapeBind(spaceID string, allowGeneric bool) error {
	if allowGeneric {
		return nil
	}
	p, err := s.Space(spaceID)
	if err != nil {
		return err
	}
	kind, adopted := s.spaceBindKind(p)
	if adopted || IsBindDefaultKind(kind) {
		return nil
	}
	return errors.New("bind_escape_denied")
}

// RevokeBind stops new scoped reads for this bind. It does not delete the
// space, adopt record, or owner files.
func (s *Service) RevokeBind(id string) (BindRecord, error) {
	rec, err := s.Bind(id)
	if err != nil {
		return rec, err
	}
	if rec.RevokedAt != nil {
		return rec, nil
	}
	now := time.Now().UTC()
	rec.RevokedAt = &now
	if err = s.DB.Save(&rec).Error; err != nil {
		return rec, errors.New("state_unavailable")
	}
	return rec, nil
}

// DiscoverBindSpaces lists spaces eligible for bind. Without allowGeneric,
// escape kinds are omitted unless the space has an adopt record.
func (s *Service) DiscoverBindSpaces(allowGeneric bool) []BindDiscoverSpace {
	catalog := map[string]KindDef{}
	for _, k := range StorageKinds() {
		catalog[k.Kind] = k
	}
	var out []BindDiscoverSpace
	for _, p := range s.Spaces() {
		kind, adopted := s.spaceBindKind(p)
		def := catalog[kind]
		if !allowGeneric && !adopted && !def.BindDefault {
			continue
		}
		row := BindDiscoverSpace{
			Space: p.ID, Kind: kind, Tier: def.Tier,
			BindDefault: def.BindDefault, Adopted: adopted,
		}
		if rec, ok := s.AdoptBySpace(p.ID); ok {
			row.Adopted = true
			row.Location = rec.Location
			row.Kind = rec.Kind
			if d, ok := catalog[rec.Kind]; ok {
				row.Tier = d.Tier
				row.BindDefault = d.BindDefault
			}
		}
		out = append(out, row)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Space < out[j].Space })
	return out
}

func (s *Service) spaceBindKind(p Space) (kind string, adopted bool) {
	if rec, ok := s.AdoptBySpace(p.ID); ok {
		return rec.Kind, true
	}
	if p.Kind() == "onedrive" {
		return "onedrive", false
	}
	return rcloneStorageKind(p.Remote), false
}

func rcloneStorageKind(remote string) string {
	switch typ := rconfig.GetValue(remote, "type"); typ {
	case "drive":
		return "gdrive"
	case "s3":
		return "s3"
	case "onedrive":
		return "onedrive"
	case "webdav":
		return "webdav"
	case "smb":
		return "smb"
	case "sftp":
		return "sftp"
	case "alias":
		if localAliasTarget(rconfig.GetValue(remote, "remote")) {
			return "local"
		}
		return "alias"
	case "local":
		return "local"
	case "":
		return "alias"
	default:
		return typ
	}
}

func localAliasTarget(target string) bool {
	target = strings.TrimSpace(target)
	if target == "" {
		return false
	}
	if filepath.IsAbs(target) {
		return true
	}
	// rclone remote form is name:path (colon after a remote name, not a drive letter).
	i := strings.IndexByte(target, ':')
	if i > 1 {
		return false
	}
	return false
}

// IsBindDefaultKind reports whether kind is in the default bind/adopt surface.
func IsBindDefaultKind(kind string) bool {
	for _, k := range BindDefaultKinds() {
		if k.Kind == kind {
			return true
		}
	}
	return false
}

func (s *Service) rejectBindFence(ctx context.Context, space string, rels ...string) error {
	scope, ok := bindScopeFrom(ctx)
	if !ok {
		return nil
	}
	recs, err := s.resolveBinds(scope, space)
	if err != nil {
		return err
	}
	for _, rel := range rels {
		if !anyBindPrefixContains(recs, rel) {
			return errors.New("bind_prefix_denied")
		}
	}
	return nil
}

func anyBindPrefixContains(recs []BindRecord, rel string) bool {
	for _, rec := range recs {
		if BindPrefixContains(rec.Prefix, rel) {
			return true
		}
	}
	return false
}

func (s *Service) resolveBinds(scope BindScope, space string) ([]BindRecord, error) {
	if scope.ID != "" {
		rec, err := s.Bind(scope.ID)
		if err != nil {
			return nil, err
		}
		if rec.RevokedAt != nil {
			return nil, errors.New("bind_revoked")
		}
		if space != "" && rec.SpaceID != space {
			return nil, errors.New("bind_prefix_denied")
		}
		if scope.Consumer != "" && rec.Consumer != scope.Consumer {
			return nil, errors.New("bind_prefix_denied")
		}
		if scope.ProjectRef != "" && rec.ProjectRef != scope.ProjectRef {
			return nil, errors.New("bind_prefix_denied")
		}
		return []BindRecord{rec}, nil
	}
	if scope.Consumer == "" || scope.ProjectRef == "" || space == "" {
		return nil, errors.New("bind_missing_flag")
	}
	var recs []BindRecord
	if err := s.DB.Where("consumer = ? AND project_ref = ? AND space_id = ?", scope.Consumer, scope.ProjectRef, space).Find(&recs).Error; err != nil {
		return nil, errors.New("state_unavailable")
	}
	var active []BindRecord
	for _, rec := range recs {
		if rec.RevokedAt == nil {
			active = append(active, rec)
		}
	}
	if len(active) == 0 {
		if len(recs) > 0 {
			return nil, errors.New("bind_revoked")
		}
		return nil, errors.New("bind_not_found")
	}
	return active, nil
}
