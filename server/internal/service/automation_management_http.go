package service

import (
	"net/http"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

func (s *Service) automationManagementRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/spaces/{id}/automation-management", s.wrapObserved("automation.rules_list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		out, err := s.AutomationManagementAvailable(p, r.PathValue("id"))
		if err != nil {
			return err
		}
		w.Header().Set("Cache-Control", "no-store")
		respond(w, out)
		return nil
	}, false))
	mux.HandleFunc("POST /api/v1/spaces/{id}/automation-rules/changes", s.wrap("automation.rules_set", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" || len(r.Header.Values("Idempotency-Key")) != 1 {
			return code("invalid_request")
		}
		q, err := contract.DecodeRequest[contract.AutomationChangeRequest](http.MaxBytesReader(w, r.Body, 8192))
		if err != nil {
			return code("invalid_request")
		}
		out, err := s.ChangeAutomationRule(p, r.PathValue("id"), q, r.Header.Get("Idempotency-Key"))
		if err != nil {
			return err
		}
		w.Header().Set("Cache-Control", "no-store")
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/automation-rules/{id}/preview", s.wrapObserved("automation.rules_list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		if _, err := contract.DecodeRequest[struct{}](http.MaxBytesReader(w, r.Body, 1024)); err != nil {
			return code("invalid_request")
		}
		out, err := s.PreviewAutomationRule(p, r.PathValue("id"))
		if err != nil {
			return err
		}
		w.Header().Set("Cache-Control", "no-store")
		respond(w, out)
		return nil
	}, false))
	mux.HandleFunc("POST /api/v1/automation-rules/{id}/execute", s.wrap("automation.rules_run", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" || len(r.Header.Values("Idempotency-Key")) != 1 {
			return code("invalid_request")
		}
		q, err := contract.DecodeRequest[contract.AutomationExecuteRequest](http.MaxBytesReader(w, r.Body, 2048))
		if err != nil {
			return code("invalid_request")
		}
		out, err := s.ExecuteAutomationPreview(p, r.PathValue("id"), q, r.Header.Get("Idempotency-Key"))
		if err != nil {
			return err
		}
		w.Header().Set("Cache-Control", "no-store")
		respond(w, out)
		return nil
	}))
	for _, entry := range []struct{ path, action string }{{"/api/v1/spaces/{id}/automation-rules/changes/lookup", "automation.change"}, {"/api/v1/automation-rules/{id}/execute/lookup", "automation.execute"}} {
		mux.HandleFunc("POST "+entry.path, s.wrapObserved("automation.rules_list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
			if r.URL.RawQuery != "" {
				return code("invalid_request")
			}
			q, err := contract.DecodeRequest[contract.IdempotencyLookupRequest](http.MaxBytesReader(w, r.Body, 2048))
			if err != nil {
				return code("invalid_request")
			}
			out, err := s.LookupAutomationRequest(p, r.PathValue("id"), entry.action, q.IdempotencyKey)
			if err != nil {
				return err
			}
			w.Header().Set("Cache-Control", "no-store")
			respond(w, out)
			return nil
		}, false))
	}
}
