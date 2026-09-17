package bridge

import (
	"context"
	"errors"
	"io"
	"strings"
)

// Node is the stable VFS node shape shared by every mount adapter; display
// names may change while open/save stay pinned to file_id/version_id.
type Node struct {
	SpaceID   string `json:"space_id"`
	RelPath   string `json:"rel_path"`
	Name      string `json:"name"`
	Kind      string `json:"kind"`
	FileID    string `json:"file_id,omitempty"`
	VersionID string `json:"version_id,omitempty"`
	Ref       string `json:"ref,omitempty"`
	SHA256    string `json:"sha256,omitempty"`
	SizeBytes int64  `json:"size_bytes"`
	Readable  bool   `json:"readable"`
	Writable  bool   `json:"writable"`
}

// VFS is the only semantic layer for mounts; WebDAV/FUSE translate it and
// never invent their own overwrite, quota or identity rules.
type VFS struct {
	s        *Service
	spaces   []string
	writable bool
}

func NewVFS(s *Service, spaceIDs []string, writable bool) *VFS {
	return &VFS{s: s, spaces: spaceIDs, writable: writable}
}

func hiddenEntry(name string) bool {
	return strings.HasPrefix(name, ".drivebridge-")
}

// splitRef resolves "<space>/<rel>" against the mount's explicit space list;
// unlisted spaces are indistinguishable from nonexistent ones.
func (v *VFS) splitRef(path string) (Space, string, error) {
	path = strings.Trim(path, "/")
	space, rel, _ := strings.Cut(path, "/")
	if space == "" {
		return Space{}, "", errors.New("invalid_path")
	}
	for _, id := range v.spaces {
		if id == space {
			p, err := v.s.Space(id)
			if err != nil {
				return Space{}, "", errors.New("space_not_allowed")
			}
			if rel != "" {
				if rel, err = remoteDir(rel); err != nil {
					return Space{}, "", errors.New("path_not_allowed")
				}
			}
			return p, rel, nil
		}
	}
	return Space{}, "", errors.New("not_found")
}

func (v *VFS) Readdir(ctx context.Context, path string) ([]Node, error) {
	path = strings.Trim(path, "/")
	if path == "" {
		nodes := []Node{}
		for _, id := range v.spaces {
			if _, err := v.s.Space(id); err != nil {
				continue
			}
			nodes = append(nodes, Node{SpaceID: id, Name: id, Kind: "dir", Readable: true, Writable: v.writable})
		}
		return nodes, nil
	}
	p, rel, err := v.splitRef(path)
	if err != nil {
		return nil, err
	}
	entries, err := v.s.listEntries(ctx, p.ID, rel, true)
	if err != nil {
		return nil, err
	}
	nodes := []Node{}
	for _, f := range entries {
		if hiddenEntry(f.Name) {
			continue
		}
		n := Node{SpaceID: p.ID, RelPath: strings.Trim(strings.TrimPrefix(rel+"/"+f.Name, "/"), "/"), Name: f.Name, Readable: true, Writable: v.writable}
		if f.Directory {
			n.Kind = "dir"
		} else {
			n.Kind, n.FileID, n.VersionID, n.Ref, n.SHA256, n.SizeBytes = "file", f.ItemID, f.Version, f.Ref, f.SHA256, f.Size
		}
		nodes = append(nodes, n)
	}
	return nodes, nil
}

func (v *VFS) Lookup(ctx context.Context, path string) (Node, error) {
	path = strings.Trim(path, "/")
	if path == "" {
		return Node{Kind: "dir", Readable: true, Writable: v.writable}, nil
	}
	p, rel, err := v.splitRef(path)
	if err != nil {
		return Node{}, err
	}
	if rel == "" {
		return Node{SpaceID: p.ID, Name: p.ID, Kind: "dir", Readable: true, Writable: v.writable}, nil
	}
	if hiddenEntry(filepath0(rel)) {
		return Node{}, errors.New("not_found")
	}
	dir, name := splitDir(rel)
	entries, err := v.s.listEntries(ctx, p.ID, dir, true)
	if err != nil {
		return Node{}, err
	}
	for _, f := range entries {
		if f.Name != name || hiddenEntry(f.Name) {
			continue
		}
		n := Node{SpaceID: p.ID, RelPath: rel, Name: f.Name, Readable: true, Writable: v.writable}
		if f.Directory {
			n.Kind = "dir"
		} else {
			n.Kind, n.FileID, n.VersionID, n.Ref, n.SHA256, n.SizeBytes = "file", f.ItemID, f.Version, f.Ref, f.SHA256, f.Size
		}
		return n, nil
	}
	// Existence is proven only by the parent listing; a mount never creates
	// remote directories from a read path.
	return Node{}, errors.New("not_found")
}

func splitDir(rel string) (string, string) {
	i := strings.LastIndex(rel, "/")
	if i < 0 {
		return "", rel
	}
	return rel[:i], rel[i+1:]
}

func filepath0(rel string) string {
	_, name := splitDir(rel)
	return name
}

// Open freezes the node's pinned version through Stat before streaming; the
// read never follows a remote replacement mid-open.
func (v *VFS) Open(ctx context.Context, path, versionID string) (io.ReadCloser, Node, error) {
	n, err := v.Lookup(ctx, path)
	if err != nil {
		return nil, n, err
	}
	if n.Kind != "file" {
		return nil, n, errors.New("invalid_path")
	}
	if versionID != "" && versionID != n.VersionID {
		return nil, n, errors.New("file_version_changed")
	}
	f, err := v.s.Stat(ctx, n.Ref)
	if err != nil {
		return nil, n, err
	}
	p, err := v.s.Space(n.SpaceID)
	if err != nil {
		return nil, n, err
	}
	body, err := v.s.providerFor(p).Open(ctx, p, f)
	if err != nil {
		return nil, n, err
	}
	n.SizeBytes = f.Size
	return body, n, nil
}

func (v *VFS) Mkdir(ctx context.Context, path string) error {
	if !v.writable {
		return errors.New("read_only")
	}
	p, rel, err := v.splitRef(path)
	if err != nil {
		return err
	}
	if rel == "" || hiddenEntry(filepath0(rel)) {
		return errors.New("path_not_allowed")
	}
	return v.s.Mkdir(ctx, p.ID, rel)
}

// Unlink maps to trash; mounts can never purge.
func (v *VFS) Unlink(ctx context.Context, path string) error {
	if !v.writable {
		return errors.New("read_only")
	}
	n, err := v.Lookup(ctx, path)
	if err != nil {
		return err
	}
	if n.Kind != "file" {
		return errors.New("invalid_path")
	}
	_, err = v.s.Trash(ctx, n.Ref)
	return err
}

// Rename maps to the existing move contract; cross-space moves are refused.
func (v *VFS) Rename(ctx context.Context, from, to string) error {
	if !v.writable {
		return errors.New("read_only")
	}
	n, err := v.Lookup(ctx, from)
	if err != nil {
		return err
	}
	tp, rel, err := v.splitRef(to)
	if err != nil {
		return err
	}
	if tp.ID != n.SpaceID {
		return errors.New("path_not_allowed")
	}
	dir, name := splitDir(rel)
	if !safeName(name) || hiddenEntry(name) {
		return errors.New("invalid_name")
	}
	_, err = v.s.RunLocalFileAction(ctx, LocalFileAction{Action: "move", Ref: n.Ref, Path: dir, Name: name})
	return err
}

// Create publishes whole objects only: the caller stages complete bytes and
// the upload contract keeps its no-overwrite, resumable semantics.
func (v *VFS) Create(ctx context.Context, path, stagedRel, idempotencyKey string) (Operation, error) {
	if !v.writable {
		return Operation{}, errors.New("read_only")
	}
	p, rel, err := v.splitRef(path)
	if err != nil {
		return Operation{}, err
	}
	dir, name := splitDir(rel)
	if !safeName(name) || hiddenEntry(name) {
		return Operation{}, errors.New("invalid_name")
	}
	return v.s.Upload(ctx, p.ID, dir, stagedRel, name, idempotencyKey)
}

func (v *VFS) Statfs(ctx context.Context, spaceID string) (maxBytes, usedBytes int64, err error) {
	p, _, err := v.splitRef(spaceID)
	if err != nil {
		return 0, 0, err
	}
	var used struct{ Total int64 }
	sizes := []int64{}
	if err = v.s.DB.Model(&File{}).Where("space_id = ? AND trashed_at IS NULL", p.ID).Pluck("size", &sizes).Error; err != nil {
		return 0, 0, errors.New("state_unavailable")
	}
	for _, size := range sizes {
		used.Total += size
	}
	return p.MaxBytes, used.Total, nil
}
