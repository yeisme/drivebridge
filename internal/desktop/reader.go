package desktop

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/yeisme/pinax/pkg/pinaxclient"
)

// Query decoding is closed and rejects duplicate fields before contacting the
// owner. Cursors and revisions are passed unchanged, never reconstructed.
func readerQuery(r *http.Request, allowed ...string) (url.Values, error) {
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || r.ContentLength != 0 || len(r.TransferEncoding) != 0 || len(r.URL.RawQuery) > 16384 {
		return nil, errors.New("request_invalid")
	}
	for key, values := range q {
		known := false
		for _, name := range allowed {
			if key == name {
				known = true
			}
		}
		if !known || len(values) != 1 || len(values[0]) > 4096 || strings.ContainsRune(values[0], 0) {
			return nil, errors.New("request_invalid")
		}
	}
	return q, nil
}

func readerPageQuery(r *http.Request, allowed ...string) (pinaxclient.ReaderPageQuery, error) {
	q, err := readerQuery(r, allowed...)
	if err != nil {
		return pinaxclient.ReaderPageQuery{}, err
	}
	page := pinaxclient.ReaderPageQuery{Cursor: q.Get("cursor"), Sort: q.Get("sort"), ProjectID: q.Get("project_id"), Status: q.Get("status")}
	if raw := q.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 200 {
			return page, errors.New("request_invalid")
		}
		page.Limit = limit
	}
	if raw := q.Get("tags"); raw != "" {
		page.Tags = strings.Split(raw, ",")
	}
	return page, nil
}

func readerDocumentRoute(path string) (source, object, action string, ok bool) {
	parts := strings.Split(strings.TrimPrefix(path, "/v1/desktop/sources/"), "/")
	if !strings.HasPrefix(path, "/v1/desktop/sources/") || len(parts) != 4 || parts[1] != "notes" || !facadeKey.MatchString(parts[0]) || !facadeKey.MatchString(parts[2]) || (parts[3] != "document" && parts[3] != "links") {
		return
	}
	return parts[0], parts[2], parts[3], true
}

func (f *facade) searchNotes(ctx context.Context, w http.ResponseWriter, r *http.Request, source string) {
	page, err := readerPageQuery(r, "q", "limit", "cursor", "sort", "project_id", "status", "tags")
	query := r.URL.Query().Get("q")
	if err != nil || strings.TrimSpace(query) == "" || len([]rune(query)) > 512 {
		facadeError(w, "request_invalid")
		return
	}
	var results pinaxclient.ReaderSearchResults
	err = f.withOwner(ctx, source, []string{"reader.metadata", "reader.body"}, true, func(ctx context.Context, client *pinaxclient.Client) error {
		var e error
		results, e = client.ReaderSearch(ctx, query, page)
		return e
	})
	if err != nil {
		facadeCallError(w, err)
		return
	}
	facadeData(w, results)
}

func (f *facade) readDocument(ctx context.Context, w http.ResponseWriter, r *http.Request, source, object string) {
	q, err := readerQuery(r, "revision", "cursor", "anchor", "outline_cursor")
	if err != nil || !fixedDigest(q.Get("revision")) || q.Get("cursor") != "" && q.Get("anchor") != "" {
		facadeError(w, "request_invalid")
		return
	}
	bound, _, err := f.preflight.sourceBinding(source)
	if err != nil {
		facadeCallError(w, err)
		return
	}
	scopes := []string{"reader.metadata", "reader.body"}
	assets := false
	for _, scope := range bound.Scopes {
		if scope == "reader.assets" {
			assets = true
			scopes = append(scopes, "reader.assets")
			break
		}
	}
	var document pinaxclient.ReaderDocumentPage
	err = f.withOwner(ctx, source, scopes, true, func(ctx context.Context, client *pinaxclient.Client) error {
		var e error
		document, e = client.ReaderDocument(ctx, pinaxclient.ReaderDocumentRequest{ObjectID: object, Revision: q.Get("revision"), Cursor: q.Get("cursor"), Anchor: q.Get("anchor"), OutlineCursor: q.Get("outline_cursor")})
		return e
	})
	if err != nil {
		facadeCallError(w, err)
		return
	}
	if document.ObjectID != object || document.Revision != q.Get("revision") {
		facadeError(w, "owner_unavailable")
		return
	}
	if assets {
		if err := f.documentAssets(source, &document); err != nil {
			facadeError(w, err.Error())
			return
		}
	}
	facadeData(w, document)
}

func (f *facade) readLinks(ctx context.Context, w http.ResponseWriter, r *http.Request, source, object string) {
	page, err := readerPageQuery(r, "revision", "direction", "limit", "cursor", "sort")
	q := r.URL.Query()
	if err != nil || !fixedDigest(q.Get("revision")) {
		facadeError(w, "request_invalid")
		return
	}
	var links pinaxclient.ReaderLinkPage
	err = f.withOwner(ctx, source, []string{"reader.metadata", "reader.relations"}, true, func(ctx context.Context, client *pinaxclient.Client) error {
		var e error
		links, e = client.ReaderLinks(ctx, object, q.Get("revision"), q.Get("direction"), page)
		return e
	})
	if err != nil {
		facadeCallError(w, err)
		return
	}
	facadeData(w, links)
}
