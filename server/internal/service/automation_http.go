package service

import "net/http"

// automationRoutes registers the controlled automation surface. Rule
// writes are human-only; subscriptions and reads are scoped.
func (s *Service) automationRoutes(mux *http.ServeMux) {
	s.automationManagementRoutes(mux)
	mux.HandleFunc("GET /api/v1/changes", s.wrap("automation.changes", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		limit := 0
		if raw := r.URL.Query().Get("limit"); raw != "" {
			for _, c := range raw {
				if c < '0' || c > '9' {
					return code("invalid_request")
				}
				limit = limit*10 + int(c-'0')
			}
		}
		view, e := s.ReadChanges(p, r.URL.Query().Get("space_id"), r.URL.Query().Get("cursor"), limit)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/change-subscriptions", s.wrap("automation.subscriptions_create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q CreateChangeSubscriptionRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		sub, e := s.CreateChangeSubscription(p, q)
		if e != nil {
			return e
		}
		markAuditTarget(r, sub.ID)
		respond(w, sub)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/change-subscriptions/{id}/ack", s.wrap("automation.subscriptions_ack", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q AckChangesRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		sub, e := s.AckChanges(p, r.PathValue("id"), q)
		if e != nil {
			return e
		}
		respond(w, sub)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/spaces/{id}/automation-rules", s.wrap("automation.rules_list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		rules, e := s.AutomationRules(p, r.PathValue("id"), nil)
		if e != nil {
			return e
		}
		respond(w, rules)
		return nil
	}))
	mux.HandleFunc("PUT /api/v1/spaces/{id}/automation-rules", s.wrap("automation.rules_set", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q AutomationRuleRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		rules, e := s.AutomationRules(p, r.PathValue("id"), &q)
		if e != nil {
			return e
		}
		respond(w, rules)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/automation-rules/{id}/run", s.wrap("automation.rules_run", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		intents, e := s.RunAutomationRule(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		respond(w, intents)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/automation-intents/{id}", s.wrap("automation.intents_get", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		intent, e := s.GetAutomationIntent(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		respond(w, intent)
		return nil
	}))
	_ = http.StatusOK
}
