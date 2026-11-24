package desktop

import (
	"context"
	"errors"
	"github.com/yeisme/pinax/pkg/pinaxclient"
	"mime"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type assetHandle struct {
	source, object, revision, ref string
	expires                       time.Time
}

var closedAssetRange = regexp.MustCompile(`^bytes=([0-9]+)-([0-9]+)$`)

const assetWindow = 8 << 20

func (f *facade) documentAssets(source string, document *pinaxclient.ReaderDocumentPage) error {
	f.assetMu.Lock()
	defer f.assetMu.Unlock()
	if f.assets == nil {
		f.assets = map[string]assetHandle{}
	}
	now := time.Now()
	for id, asset := range f.assets {
		if !now.Before(asset.expires) {
			delete(f.assets, id)
		}
	}
	var visit func([]pinaxclient.ReaderDocumentNode) error
	visit = func(nodes []pinaxclient.ReaderDocumentNode) error {
		for i := range nodes {
			if nodes[i].Type == "image" && strings.HasPrefix(nodes[i].Ref, "pinax-asset://") {
				assetRef := strings.TrimPrefix(nodes[i].Ref, "pinax-asset://")
				if !facadeKey.MatchString(assetRef) {
					return errors.New("request_invalid")
				}
				if len(f.assets) >= 256 {
					return errors.New("managed_scope_busy")
				}
				id, err := opaqueID()
				if err != nil {
					return err
				}
				f.assets[id] = assetHandle{source: source, object: document.ObjectID, revision: document.Revision, ref: assetRef, expires: now.Add(5 * time.Minute)}
				nodes[i].Ref = "drivebridge-asset://asset/" + id
			}
			if err := visit(nodes[i].Children); err != nil {
				return err
			}
		}
		return nil
	}
	return visit(document.Nodes)
}
func (f *facade) readAsset(w http.ResponseWriter, r *http.Request, id string) {
	if !facadeKey.MatchString(id) || r.ContentLength != 0 || len(r.Header.Values("Range")) > 1 {
		facadeError(w, "request_invalid")
		return
	}
	f.assetMu.Lock()
	asset, ok := f.assets[id]
	f.assetMu.Unlock()
	if !ok || !time.Now().Before(asset.expires) {
		facadeError(w, "source_unknown")
		return
	}
	var requested pinaxclient.ReaderByteRange
	if raw := r.Header.Get("Range"); raw != "" {
		match := closedAssetRange.FindStringSubmatch(raw)
		if len(match) != 3 {
			facadeError(w, "range_invalid")
			return
		}
		start, e1 := strconv.ParseInt(match[1], 10, 64)
		end, e2 := strconv.ParseInt(match[2], 10, 64)
		if e1 != nil || e2 != nil || end < start || end-start >= assetWindow {
			facadeError(w, "range_invalid")
			return
		}
		requested = pinaxclient.ReaderByteRange{Set: true, Start: start, End: end}
	}
	var content pinaxclient.ReaderAssetContent
	err := f.withOwner(r.Context(), asset.source, []string{"reader.metadata", "reader.body", "reader.assets"}, true, func(ctx context.Context, client *pinaxclient.Client) error {
		var err error
		content, err = client.ReaderAssetRange(ctx, asset.ref, asset.object, asset.revision, requested)
		return err
	})
	if err != nil {
		facadeCallError(w, err)
		return
	}
	media, _, err := mime.ParseMediaType(content.Descriptor.Mime)
	if err != nil || len(content.Body) > assetWindow || !time.Now().Before(asset.expires) {
		facadeError(w, "source_unknown")
		return
	}
	switch media {
	case "image/png", "image/jpeg", "image/gif", "image/webp", "image/avif":
	default:
		facadeError(w, "desktop_contract_unsupported")
		return
	}
	if !requested.Set && int64(len(content.Body)) != content.Descriptor.SizeBytes {
		facadeError(w, "owner_unavailable")
		return
	}
	w.Header().Set("Content-Type", media)
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "default-src 'none'")
	if content.ContentRange != "" {
		w.Header().Set("Content-Range", content.ContentRange)
		w.WriteHeader(http.StatusPartialContent)
	}
	_, _ = w.Write(content.Body)
}
