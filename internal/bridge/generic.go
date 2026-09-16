package bridge

import (
	"context"
	"errors"
	"io"
	"path"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/object"
	"github.com/rclone/rclone/fs/operations"
)

// Generic is the transfer-only provider for any configured rclone remote
// (S3, WebDAV, SMB, Google Drive, ...). Stable references and byte transfers
// work; direct URLs and the OneDrive reference handoff never do.
type Generic struct {
	mu      sync.Mutex
	clients map[string]fs.Fs
}

func NewGeneric() *Generic { return &Generic{clients: map[string]fs.Fs{}} }

func (g *Generic) remote(p Space, contexts ...context.Context) (fs.Fs, error) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if f, ok := g.clients[p.ID]; ok {
		return f, nil
	}
	kind := config.GetValue(p.Remote, "type")
	if kind == "" {
		return nil, errors.New("backend_unavailable")
	}
	ctx := context.Background()
	if len(contexts) > 0 {
		ctx = contexts[0]
	}
	f, err := fs.NewFs(ctx, p.Remote+":"+p.RootID)
	if err != nil {
		return nil, errors.New("backend_unavailable")
	}
	g.clients[p.ID] = f
	return f, nil
}

// objectVersion pins a version from the mutable metadata rclone backends
// expose; content changes surface as version drift, never silent reuse.
func objectVersion(size int64, modTime time.Time) string {
	return "m" + strconv.FormatInt(modTime.UnixNano(), 10) + "-s" + strconv.FormatInt(size, 10)
}

func fileFromObject(ctx context.Context, p Space, o fs.Object) File {
	return File{DriveID: p.DriveID, ItemID: o.Remote(), Name: path.Base(o.Remote()), Size: o.Size(), Version: objectVersion(o.Size(), o.ModTime(ctx))}
}

func (g *Generic) List(ctx context.Context, p Space, dir string) ([]File, error) {
	f, err := g.remote(p)
	if err != nil {
		return nil, err
	}
	entries, err := f.List(ctx, dir)
	if err != nil {
		return nil, errors.New("remote_not_found")
	}
	out := []File{}
	for _, entry := range entries {
		if _, ok := entry.(fs.Directory); ok {
			out = append(out, File{Directory: true, Name: path.Base(entry.Remote()), Dir: dir})
			continue
		}
		o, ok := entry.(fs.Object)
		if !ok || o.Size() <= 0 || o.Size() > p.MaxBytes {
			continue
		}
		out = append(out, fileFromObject(ctx, p, o))
	}
	return out, nil
}

func (g *Generic) Stat(ctx context.Context, p Space, id string) (File, error) {
	f, err := g.remote(p)
	if err != nil {
		return File{}, err
	}
	o, err := f.NewObject(ctx, id)
	if err != nil {
		return File{}, errors.New("remote_not_found")
	}
	return fileFromObject(ctx, p, o), nil
}

func (g *Generic) Upload(ctx context.Context, p Space, name string, in io.Reader, n int64) (File, error) {
	f, err := g.remote(p)
	if err != nil {
		return File{}, err
	}
	info := object.NewStaticObjectInfo(name, time.Now().UTC(), n, true, nil, nil)
	o, err := f.Put(ctx, in, info)
	if err != nil {
		return File{}, errors.New("backend_unavailable")
	}
	return fileFromObject(ctx, p, o), nil
}

func (g *Generic) Open(ctx context.Context, p Space, f File) (io.ReadCloser, error) {
	remote, err := g.remote(p)
	if err != nil {
		return nil, err
	}
	o, err := remote.NewObject(ctx, f.ItemID)
	if err != nil {
		return nil, errors.New("remote_not_found")
	}
	handle, err := o.Open(ctx)
	if err != nil {
		return nil, errors.New("download_unavailable")
	}
	return handle, nil
}

// Mkdir creates a workspace directory; the rclone contract makes an existing
// directory a success, so agents can organize idempotently.
func (g *Generic) Mkdir(ctx context.Context, p Space, dir string) error {
	f, err := g.remote(p)
	if err != nil {
		return err
	}
	if err := f.Mkdir(ctx, dir); err != nil {
		return errors.New("backend_unavailable")
	}
	return nil
}

// Move renames within the same remote (server-side where the backend
// supports it); the destination directory must already exist.
func (g *Generic) Move(ctx context.Context, p Space, f File, destDir, destName string) error {
	remote, err := g.remote(p)
	if err != nil {
		return err
	}
	if destDir, err = remoteDir(destDir); err != nil {
		return err
	}
	if !safeName(path.Base(destName)) {
		return errors.New("path_not_allowed")
	}
	src, err := remote.NewObject(ctx, g.sourcePath(f))
	if err != nil {
		return errors.New("remote_not_found")
	}
	dest := destName
	if destDir != "" {
		dest = destDir + "/" + destName
	}
	if _, err = operations.Move(ctx, remote, nil, dest, src); err != nil {
		if strings.Contains(err.Error(), "exists") {
			return errors.New("target_conflict")
		}
		return errors.New("backend_unavailable")
	}
	return nil
}

// sourcePath resolves where the object currently lives: the original path
// until trashed, the trash path afterwards.
func (g *Generic) sourcePath(f File) string {
	if f.Trashed != nil && f.TrashPath != "" {
		return f.TrashPath
	}
	if f.Dir != "" {
		return f.Dir + "/" + f.Name
	}
	return f.Name
}

// Delete permanently removes the object at its current location.
func (g *Generic) Delete(ctx context.Context, p Space, f File) error {
	remote, err := g.remote(p)
	if err != nil {
		return err
	}
	obj, err := remote.NewObject(ctx, g.sourcePath(f))
	if err != nil {
		return errors.New("remote_not_found")
	}
	if err = obj.Remove(ctx); err != nil {
		return errors.New("backend_unavailable")
	}
	return nil
}

// Access is deliberately unsupported: temporary direct URLs stay an
// OneDrive-only capability.
func (g *Generic) Access(context.Context, Space, File) (string, error) {
	return "", errors.New("access_unsupported")
}
