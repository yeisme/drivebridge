package service

import (
	"net/http"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

func (s *Service) propertyMigrationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/spaces/{id}/property-migrations/capability", s.wrapObserved("properties.migration_read", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		out, err := s.PropertyMigrationCapability(r.Context(), p, r.PathValue("id"))
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}, false))
	mux.HandleFunc("GET /api/v1/property-migrations/{id}", s.wrapObserved("properties.migration_read", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		out, err := s.GetPropertyMigration(r.Context(), p, r.PathValue("id"))
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}, false))
	mux.HandleFunc("POST /api/v1/spaces/{id}/property-migrations/lookup", s.wrapObserved("properties.migration_read", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		q, err := contract.DecodeRequest[contract.PropertyMigrationLookup](http.MaxBytesReader(w, r.Body, 2048))
		if err != nil {
			return code("invalid_request")
		}
		out, err := s.LookupPropertyMigration(r.Context(), p, r.PathValue("id"), q)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}, false))
	mux.HandleFunc("POST /api/v1/spaces/{id}/property-migrations", s.wrap("properties.migration_preview", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" || len(r.Header.Values("Idempotency-Key")) != 1 {
			return code("invalid_request")
		}
		q, err := contract.DecodeRequest[contract.PropertyMigrationPreview](http.MaxBytesReader(w, r.Body, 4096))
		if err != nil {
			return code("invalid_request")
		}
		out, err := s.PreviewPropertyMigration(r.Context(), p, r.PathValue("id"), q, r.Header.Get("Idempotency-Key"))
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}))
	for _, stage := range []string{"confirm", "execute"} {
		mux.HandleFunc("POST /api/v1/property-migrations/{id}/"+stage, s.wrap("properties.migration_"+stage, func(w http.ResponseWriter, r *http.Request, p Principal) error {
			if r.URL.RawQuery != "" || len(r.Header.Values("Idempotency-Key")) != 1 {
				return code("invalid_request")
			}
			q, err := contract.DecodeRequest[contract.PropertyMigrationDecision](http.MaxBytesReader(w, r.Body, 2048))
			if err != nil {
				return code("invalid_request")
			}
			var out contract.PropertyMigrationReceipt
			if stage == "confirm" {
				out, err = s.ConfirmPropertyMigration(r.Context(), p, r.PathValue("id"), q, r.Header.Get("Idempotency-Key"))
			} else {
				out, err = s.ExecutePropertyMigration(r.Context(), p, r.PathValue("id"), q, r.Header.Get("Idempotency-Key"))
			}
			if err != nil {
				return err
			}
			respond(w, out)
			return nil
		}))
	}
}
