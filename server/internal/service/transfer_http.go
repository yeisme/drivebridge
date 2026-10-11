package service

import "net/http"

// transferRoutes registers the cross-storage transfer surface. Source
// cleanup has no agent tool: it stays a human confirmation.
func (s *Service) transferRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/transfer-plans", s.wrap("transfer.plan_create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q CreateTransferPlanRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.CreateTransferPlan(p, q, r.Header.Get("Idempotency-Key"))
		if e != nil {
			return e
		}
		markAuditTarget(r, view.PlanID)
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/transfer-plans/{id}", s.wrap("transfer.plan_get", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		limit := 0
		if raw := r.URL.Query().Get("limit"); raw != "" {
			for _, c := range raw {
				if c < '0' || c > '9' {
					return code("invalid_request")
				}
			}
		}
		view, e := s.GetTransferPlan(p, r.PathValue("id"), r.URL.Query().Get("cursor"), limit)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/transfer-plans/{id}/execute", s.wrap("transfer.execute", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q ExecuteTransferPlanRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.ExecuteTransferPlan(r.Context(), p, r.PathValue("id"), q)
		if e != nil {
			return e
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/transfer-plans/{id}/cleanup-source", s.wrap("transfer.cleanup_source", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			ExpectedRevision uint64 `json:"expected_revision"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.CleanupTransferSource(p, r.PathValue("id"), q.ExpectedRevision)
		if e != nil {
			return e
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/transfer-plans/{id}/cancel", s.wrap("transfer.cancel", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.CancelTransferPlan(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	_ = http.StatusOK
}
