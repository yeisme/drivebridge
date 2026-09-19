package bridge

import (
	"context"
	"errors"
	"io"
	"path"
	"strconv"
	"strings"

	"github.com/rclone/rclone/fs"
)

const localDirectoryLimit = 1000

type dirRemover interface {
	Rmdir(ctx context.Context, p Space, dir string) error
}

func (s *Service) uniqueName(ctx context.Context, space, dir, name string) (string, error) {
	if !safeName(name) {
		return "", errors.New("invalid_name")
	}
	items, err := s.listEntries(ctx, space, dir, true)
	if err != nil {
		if err.Error() == "remote_not_found" {
			return name, nil
		}
		return "", err
	}
	taken := map[string]bool{}
	for _, item := range items {
		taken[item.Name] = true
	}
	if !taken[name] {
		return name, nil
	}
	ext := path.Ext(name)
	base := strings.TrimSuffix(name, ext)
	for n := 2; n < 1000; n++ {
		candidate := base + " (" + strconv.Itoa(n) + ")" + ext
		if !taken[candidate] {
			return candidate, nil
		}
	}
	return "", errors.New("target_conflict")
}

func (s *Service) copyByUpload(ctx context.Context, p Space, src File, target string) (File, error) {
	body, err := s.providerFor(p).Open(ctx, p, src)
	if err != nil {
		return File{}, err
	}
	defer body.Close()
	return s.providerFor(p).Upload(ctx, p, target, io.LimitReader(body, src.Size), src.Size)
}

func (s *Service) DuplicateFile(ctx context.Context, ref, destDir, destName string) (File, error) {
	if !s.Enabled {
		return File{}, errors.New("disabled")
	}
	src, err := s.Stat(ctx, ref)
	if err != nil {
		return File{}, err
	}
	if src.Trashed != nil {
		return File{}, errors.New("file_trashed")
	}
	if destDir, err = remoteDir(destDir); err != nil {
		return File{}, err
	}
	if destName == "" {
		destName = src.Name
	}
	if err = s.rejectBindFence(ctx, src.SpaceID, destDir, joinRel(destDir, destName)); err != nil {
		return File{}, err
	}
	destName, err = s.uniqueName(ctx, src.SpaceID, destDir, destName)
	if err != nil {
		return File{}, err
	}
	p, err := s.Space(src.SpaceID)
	if err != nil {
		return File{}, err
	}
	if err = s.rejectOwnerProtected(src.SpaceID, destDir, joinRel(destDir, destName)); err != nil {
		return File{}, err
	}
	target := destName
	if destDir != "" {
		target = destDir + "/" + destName
	}
	var remote File
	if copier, ok := s.providerFor(p).(remoteCopier); ok {
		remote, err = copier.Copy(ctx, p, src, target)
		if err != nil && (errors.Is(err, fs.ErrorCantCopy) || err.Error() == "remote_not_found" || err.Error() == "backend_unavailable") {
			remote, err = s.copyByUpload(ctx, p, src, target)
		}
	} else {
		remote, err = s.copyByUpload(ctx, p, src, target)
	}
	if err != nil {
		return File{}, err
	}
	stored, err := s.remember(p, remote)
	if err != nil {
		return File{}, err
	}
	s.touchRetention(p.ID, stored.Ref)
	s.emit(p.ID, "file_copied", stored.Ref, stored.Name)
	return stored, nil
}

func (s *Service) walkTree(ctx context.Context, space, dir string) ([]File, []string, error) {
	if dir == "" {
		return nil, nil, errors.New("invalid_directory")
	}
	files := []File{}
	dirs := []string{dir}
	queue := []string{dir}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		items, err := s.listEntries(ctx, space, cur, true)
		if err != nil {
			return nil, nil, err
		}
		for _, item := range items {
			if item.Directory {
				next := item.Name
				if cur != "" {
					next = cur + "/" + item.Name
				}
				dirs = append(dirs, next)
				queue = append(queue, next)
				continue
			}
			files = append(files, item)
			if len(files) > localDirectoryLimit {
				return nil, nil, errors.New("directory_too_large")
			}
		}
	}
	return files, dirs, nil
}

// unsupportedTreeFiles reports whether the tree holds files outside the
// space's transfer size window (empty or above MaxBytes). Those files have no
// stable reference and cannot be trashed or moved, so directory jobs refuse
// up front instead of silently stranding them while reporting success.
func (s *Service) unsupportedTreeFiles(ctx context.Context, space, dir string) error {
	if dir == "" {
		return errors.New("invalid_directory")
	}
	p, err := s.Space(space)
	if err != nil {
		return err
	}
	if dir, err = remoteDir(dir); err != nil {
		return err
	}
	count := 0
	queue := []string{dir}
	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		if err = s.rejectBindFence(ctx, space, cur); err != nil {
			return err
		}
		items, err := s.providerFor(p).List(ctx, p, cur)
		if err != nil {
			return err
		}
		for _, item := range items {
			if item.Directory {
				next := item.Name
				if cur != "" {
					next = cur + "/" + item.Name
				}
				queue = append(queue, next)
				continue
			}
			count++
			if count > localDirectoryLimit {
				return errors.New("directory_too_large")
			}
			if item.Size <= 0 || item.Size > p.MaxBytes {
				return errors.New("directory_contains_unsupported_files")
			}
		}
	}
	return nil
}

func remapLocal(path, src, dest string) string {
	if path == src {
		return dest
	}
	return dest + strings.TrimPrefix(path, src)
}

func (s *Service) DuplicateDirectory(ctx context.Context, space, src, dest string) (map[string]any, error) {
	src, err := remoteDir(src)
	if err != nil || src == "" {
		return nil, errors.New("invalid_directory")
	}
	dest, err = remoteDir(dest)
	if err != nil || dest == "" {
		return nil, errors.New("invalid_directory")
	}
	if err = s.rejectOwnerProtected(space, src, dest); err != nil {
		return nil, err
	}
	if err = s.unsupportedTreeFiles(ctx, space, src); err != nil {
		return nil, err
	}
	parent, name := path.Split(dest)
	parent = strings.TrimSuffix(parent, "/")
	name, err = s.uniqueName(ctx, space, parent, name)
	if err != nil {
		return nil, err
	}
	dest = name
	if parent != "" {
		dest = parent + "/" + name
	}
	files, dirs, err := s.walkTree(ctx, space, src)
	if err != nil {
		return nil, err
	}
	for _, d := range dirs {
		if err = s.Mkdir(ctx, space, remapLocal(d, src, dest)); err != nil {
			return nil, err
		}
	}
	copied := 0
	for _, f := range files {
		if _, cerr := s.DuplicateFile(ctx, f.Ref, remapLocal(f.Dir, src, dest), f.Name); cerr != nil {
			// Remove what this run created so a retry starts from the
			// requested destination name instead of forking "name (2)".
			s.compensateDirectoryCopy(ctx, space, dest)
			return nil, cerr
		}
		copied++
	}
	return map[string]any{"path": dest, "copied": copied}, nil
}

// compensateDirectoryCopy trashes the files and empty directories a failed
// duplicate run just created. Best effort: failures must not mask the
// original error, and trashed files stay individually restorable.
func (s *Service) compensateDirectoryCopy(ctx context.Context, space, dest string) {
	p, err := s.Space(space)
	if err != nil {
		return
	}
	files, dirs, err := s.walkTree(ctx, space, dest)
	if err != nil {
		return
	}
	for _, f := range files {
		if f.Trashed == nil {
			_, _ = s.Trash(ctx, f.Ref)
		}
	}
	_ = s.removeEmptyDirs(ctx, p, dirs)
}

func (s *Service) removeEmptyDirs(ctx context.Context, p Space, dirs []string) error {
	remover, ok := s.providerFor(p).(dirRemover)
	if !ok {
		return errors.New("unsupported")
	}
	var first error
	for i := len(dirs) - 1; i >= 0; i-- {
		if err := remover.Rmdir(ctx, p, dirs[i]); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func (s *Service) RelocateDirectory(ctx context.Context, space, src, dest string) (map[string]any, error) {
	src, err := remoteDir(src)
	if err != nil || src == "" {
		return nil, errors.New("invalid_directory")
	}
	dest, err = remoteDir(dest)
	if err != nil || dest == "" {
		return nil, errors.New("invalid_directory")
	}
	if dest == src || strings.HasPrefix(dest, src+"/") {
		return nil, errors.New("invalid_directory")
	}
	if err = s.rejectOwnerProtected(space, src, dest); err != nil {
		return nil, err
	}
	if err = s.unsupportedTreeFiles(ctx, space, src); err != nil {
		return nil, err
	}
	parent, name := path.Split(dest)
	parent = strings.TrimSuffix(parent, "/")
	name, err = s.uniqueName(ctx, space, parent, name)
	if err != nil {
		return nil, err
	}
	dest = name
	if parent != "" {
		dest = parent + "/" + name
	}
	files, dirs, err := s.walkTree(ctx, space, src)
	if err != nil {
		return nil, err
	}
	for _, d := range dirs {
		if err = s.Mkdir(ctx, space, remapLocal(d, src, dest)); err != nil {
			return nil, err
		}
	}
	moved := 0
	movedFiles := []File{}
	for _, f := range files {
		relDir := remapLocal(f.Dir, src, dest)
		if _, err = s.RunLocalFileAction(ctx, LocalFileAction{Action: "move", Ref: f.Ref, Path: relDir, Name: f.Name}); err != nil {
			// Move the already-relocated files back so a retry lands on the
			// same destination name instead of forking "name (2)" and
			// splitting the directory across two destinations.
			for i := moved - 1; i >= 0; i-- {
				g := movedFiles[i]
				_, _ = s.RunLocalFileAction(ctx, LocalFileAction{Action: "move", Ref: g.Ref, Path: g.Dir, Name: g.Name})
			}
			s.compensateDirectoryCopy(ctx, space, dest)
			return nil, err
		}
		movedFiles = append(movedFiles, f)
		moved++
	}
	p, err := s.Space(space)
	if err != nil {
		return nil, err
	}
	if err = s.removeEmptyDirs(ctx, p, dirs); err != nil {
		return nil, err
	}
	return map[string]any{"path": dest, "moved": moved}, nil
}

func (s *Service) TrashDirectory(ctx context.Context, space, src string) (map[string]any, error) {
	src, err := remoteDir(src)
	if err != nil || src == "" {
		return nil, errors.New("invalid_directory")
	}
	if err = s.rejectBindFence(ctx, space, src); err != nil {
		return nil, err
	}
	if err = s.rejectOwnerProtected(space, src); err != nil {
		return nil, err
	}
	if err = s.unsupportedTreeFiles(ctx, space, src); err != nil {
		return nil, err
	}
	files, dirs, err := s.walkTree(ctx, space, src)
	if err != nil {
		return nil, err
	}
	trashed := 0
	for _, f := range files {
		if _, err = s.Trash(ctx, f.Ref); err != nil {
			return nil, err
		}
		trashed++
	}
	p, err := s.Space(space)
	if err != nil {
		return nil, err
	}
	if err = s.removeEmptyDirs(ctx, p, dirs); err != nil {
		return nil, err
	}
	return map[string]any{"path": src, "trashed": trashed}, nil
}

func (g *Generic) Rmdir(ctx context.Context, p Space, dir string) error {
	f, err := g.remote(p)
	if err != nil {
		return err
	}
	if err = f.Rmdir(ctx, dir); err != nil {
		return errors.New("backend_unavailable")
	}
	return nil
}
