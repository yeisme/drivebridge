package service

import (
	"net/http"
	"github.com/yeisme/drivebridge-server/internal/contract"
)

// agentorgRoutes registers the agent organization surface. Policy writes
// and confirmations are human-only; agents create and execute plans
// inside their scopes.
func (s *Service) agentorgRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/organization-plans", s.wrap("agentorg.plans_create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q CreateOrganizationPlanRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.CreateOrganizationPlan(p, q, r.Header.Get("Idempotency-Key"))
		if e != nil {
			return e
		}
		markAuditTarget(r, view.PlanID)
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/organization-plans/{id}", s.wrapObserved("agentorg.plans_get", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		_, page, err := labelPageQuery(r.URL.RawQuery, false)
		if err != nil { return err }
		view, e := s.GetOrganizationPlan(p, r.PathValue("id"), page.Cursor, page.Limit)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}, false))
	mux.HandleFunc("POST /api/v1/organization-plans/{id}/confirm", s.wrap("agentorg.plans_confirm", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" { return code("invalid_request") }
		q, err := contract.DecodeRequest[ConfirmOrganizationPlanRequest](http.MaxBytesReader(w, r.Body, 2048))
		if err != nil { return code("invalid_request") }
		view, e := s.ConfirmOrganizationPlan(p, r.PathValue("id"), q)
		if e != nil {
			return e
		}
		markAuditTarget(r, view.PlanID)
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/organization-plans/{id}/execute", s.wrap("agentorg.plans_execute", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" { return code("invalid_request") }
		q, err := contract.DecodeRequest[ExecuteOrganizationPlanRequest](http.MaxBytesReader(w, r.Body, 2048))
		if err != nil { return code("invalid_request") }
		view, e := s.ExecuteOrganizationPlan(p, r.PathValue("id"), q)
		if e != nil {
			return e
		}
		markAuditTarget(r, view.PlanID)
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/organization-plans/{id}/compensations", s.wrap("agentorg.plans_compensate", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.CreateCompensationPlan(p, r.PathValue("id"), r.Header.Get("Idempotency-Key"))
		if e != nil {
			return e
		}
		markAuditTarget(r, view.PlanID)
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("PUT /api/v1/spaces/{id}/organization-policy", s.wrap("agentorg.policy_set", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q OrganizationPolicyRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.SetOrganizationPolicy(p, r.PathValue("id"), q)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/spaces/{id}/organization-policy", s.wrap("agentorg.policy_get", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.GetOrganizationPolicy(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	_ = http.StatusOK
}
