package service

import "net/http"

// contentRoutes registers the content retrieval surface. Policy widening
// stays human-only; agents consume bindings and queries inside scopes.
func (s *Service) contentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/content-bindings", s.wrap("content.bindings_list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		views, e := s.ListContentBindings(p, r.URL.Query().Get("space_id"))
		if e != nil {
			return e
		}
		respond(w, views)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/content-bindings", s.wrap("content.bindings_create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q CreateContentBindingRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.CreateContentBinding(p, q, r.Header.Get("Idempotency-Key"))
		if e != nil {
			return e
		}
		markAuditTarget(r, view.BindingID)
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/content-bindings/{id}", s.wrap("content.bindings_get", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.GetContentBinding(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/content-bindings/{id}/process", s.wrap("content.process", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q ProcessContentRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		q.BindingID = r.PathValue("id")
		view, e := s.ProcessContent(r.Context(), p, q, r.Header.Get("Idempotency-Key"))
		if e != nil {
			return e
		}
		markAuditTarget(r, view.BindingID)
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("PUT /api/v1/spaces/{id}/content-policy", s.wrap("content.policy_set", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q ContentPolicyRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.SetContentPolicy(p, r.PathValue("id"), q)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/spaces/{id}/content-policy", s.wrap("content.policy_get", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.GetContentPolicy(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/content-queries", s.wrap("content.queries", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q ContentQueryRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.QueryContent(r.Context(), p, q)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	_ = http.StatusOK
}
