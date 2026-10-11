package service

import (
	"net/http"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

func (s *Service) linkRoutes(mux *http.ServeMux) {
	for _, route := range []struct {
		path      string
		backlinks bool
	}{{"links", false}, {"backlinks", true}} {
		mux.HandleFunc("GET /api/v1/files/{id}/"+route.path, s.wrapObserved("files."+route.path, func(w http.ResponseWriter, r *http.Request, p Principal) error {
			query, page, err := workspaceMetadataQuery(r.URL.RawQuery, "version_id")
			if err != nil || !propertyHexID(query.Get("version_id")) { return code("invalid_request") }
			out, err := s.WorkspaceLinks(r.Context(), p, r.PathValue("id"), query.Get("version_id"), route.backlinks, page)
			if err != nil {
				return err
			}
			respond(w, out)
			return nil
		}, false))
	}
	mux.HandleFunc("POST /api/v1/files/{id}/links/rebuild", s.wrap("files.links_rebuild", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		q, err := contract.DecodeRequest[contract.FileLinkRebuild](http.MaxBytesReader(w, r.Body, 2048))
		if err != nil {
			return code("invalid_request")
		}
		key, err := workspaceMetadataKey(r)
		if err != nil {
			return err
		}
		out, err := s.RebuildWorkspaceLinks(r.Context(), p, r.PathValue("id"), q, key)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/files/{id}/links/lookup", s.wrapObserved("files.links_lookup", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		q, err := contract.DecodeRequest[contract.FileLinkLookup](http.MaxBytesReader(w, r.Body, 2048))
		if err != nil {
			return code("invalid_request")
		}
		out, err := s.LookupWorkspaceLinks(r.Context(), p, r.PathValue("id"), q.VersionID, q.IdempotencyKey)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}, false))
}
