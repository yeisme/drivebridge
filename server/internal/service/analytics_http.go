package service

import "net/http"

// analyticsRoutes registers the typed analytical query and snapshot
// surface. The legacy usage/audit tools keep their own contract.
func (s *Service) analyticsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/analytics/queries", s.wrap("analytics.queries", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q AnalyticsQueryRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.QueryAnalytics(p, q)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/analytics/queries/{id}", s.wrap("analytics.queries_get", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.GetAnalyticsQuery(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/analytics/snapshots", s.wrap("analytics.snapshots_create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q AnalyticsSnapshotRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.CreateAnalyticsSnapshot(r.Context(), p, q, r.Header.Get("Idempotency-Key"))
		if e != nil {
			return e
		}
		markAuditTarget(r, view.ID)
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/analytics/snapshots/{id}/queries", s.wrap("analytics.snapshots_query", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q AnalyticsQueryRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.QueryAnalyticsSnapshot(p, r.PathValue("id"), q)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	_ = http.StatusOK
}
