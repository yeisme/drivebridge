package service

import (
	"net/http"
	"net/url"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

func (s *Service) workspaceHoldRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/files/{id}/retention-holds", s.wrapObserved("workspace.holds.list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		values, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil { return code("invalid_request") }
		for name, entries := range values {
			if len(entries) != 1 || (name != "version_id" && name != "cursor" && name != "limit") { return code("invalid_request") }
		}
		if values.Has("version_id") && !propertyHexID(values.Get("version_id")) { return code("invalid_request") }
		page, err := contract.ParsePageQuery(values)
		if err != nil { return err }
		out, err := s.WorkspaceHolds(r.Context(), p, r.PathValue("id"), values.Get("version_id"), page)
		if err != nil { return err }
		respond(w, out)
		return nil
	}, false))
	mux.HandleFunc("POST /api/v1/files/{id}/retention-holds", s.wrap("workspace.holds.create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" { return code("invalid_request") }
		q, err := contract.DecodeRequest[contract.WorkspaceHoldCreate](http.MaxBytesReader(w, r.Body, 4096))
		if err != nil { return code("invalid_request") }
		key, err := workspaceMetadataKey(r)
		if err != nil { return err }
		out, err := s.CreateWorkspaceHold(r.Context(), p, r.PathValue("id"), q, key)
		if err != nil { return err }
		markAuditTarget(r, out.HoldID)
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/retention/holds/{id}/release-cas", s.wrap("workspace.holds.release", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" { return code("invalid_request") }
		q, err := contract.DecodeRequest[contract.WorkspaceHoldRelease](http.MaxBytesReader(w, r.Body, 2048))
		if err != nil { return code("invalid_request") }
		key, err := workspaceMetadataKey(r)
		if err != nil { return err }
		out, err := s.ReleaseWorkspaceHold(r.Context(), p, r.PathValue("id"), q, key)
		if err != nil { return err }
		markAuditTarget(r, out.HoldID)
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/files/{id}/retention-holds/lookup", s.wrapObserved("workspace.holds.lookup", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" { return code("invalid_request") }
		q, err := contract.DecodeRequest[contract.WorkspaceHoldLookup](http.MaxBytesReader(w, r.Body, 2048))
		if err != nil { return code("invalid_request") }
		out, err := s.LookupWorkspaceHold(r.Context(), p, r.PathValue("id"), q)
		if err != nil { return err }
		respond(w, out)
		return nil
	}, false))
}
