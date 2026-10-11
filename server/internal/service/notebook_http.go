package service

import (
	"github.com/yeisme/drivebridge-server/internal/contract"
	"net/http"
)

func (s *Service) notebookRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/notebook/pages", s.wrapObserved("notebook.pages", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		values, page, err := workspaceMetadataQuery(r.URL.RawQuery, "space_id")
		if err != nil {
			return err
		}
		out, err := s.NotebookPages(r.Context(), p, values.Get("space_id"), page)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}, false))
	mux.HandleFunc("GET /api/v1/notebook/pages/{id}", s.wrapObserved("notebook.page", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		out, err := s.NotebookPage(r.Context(), p, r.PathValue("id"))
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}, false))
	mux.HandleFunc("POST /api/v1/notebook/pages", s.wrap("notebook.create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		q, err := contract.DecodeRequest[contract.NotebookCreate](http.MaxBytesReader(w, r.Body, 8192))
		if err != nil {
			return code("invalid_request")
		}
		key, err := workspaceMetadataKey(r)
		if err != nil {
			return err
		}
		out, err := s.CreateNotebookPage(r.Context(), p, q.SpaceID, q, key)
		if err != nil {
			return err
		}
		w.WriteHeader(http.StatusCreated)
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/notebook/pages/lookup", s.wrapObserved("notebook.create_lookup", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		q, err := contract.DecodeRequest[contract.NotebookCreateLookup](http.MaxBytesReader(w, r.Body, 2048))
		if err != nil {
			return code("invalid_request")
		}
		out, err := s.LookupNotebookCreation(r.Context(), p, r.URL.Query().Get("space_id"), q)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}, false))
	mux.HandleFunc("POST /api/v1/notebook/pages/resume", s.wrap("notebook.resume", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		q, err := contract.DecodeRequest[contract.NotebookCreateLookup](http.MaxBytesReader(w, r.Body, 2048))
		if err != nil {
			return code("invalid_request")
		}
		out, err := s.ResumeNotebookCreation(r.Context(), p, r.URL.Query().Get("space_id"), q)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("PATCH /api/v1/notebook/pages/{id}", s.wrap("notebook.page_patch", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		q, err := contract.DecodeRequest[contract.NotebookPagePatch](http.MaxBytesReader(w, r.Body, 4096))
		if err != nil {
			return code("invalid_request")
		}
		key, err := workspaceMetadataKey(r)
		if err != nil {
			return err
		}
		out, err := s.PatchNotebookPage(r.Context(), p, r.PathValue("id"), q, key)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/notebook/pages/{id}/lookup", s.wrapObserved("notebook.page_lookup", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		q, err := contract.DecodeRequest[contract.IdempotencyLookupRequest](http.MaxBytesReader(w, r.Body, 2048))
		if err != nil {
			return code("invalid_request")
		}
		out, err := s.LookupNotebookPage(r.Context(), p, r.PathValue("id"), q.IdempotencyKey)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}, false))
}
