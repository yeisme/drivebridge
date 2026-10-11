package service

import (
	"bytes"
	"encoding/json"
	"net/http"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

func (s *Service) propertyDefinitionRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/spaces/{id}/property-definitions", s.wrapObserved("properties.definitions_read", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		out, err := s.PropertyDefinitions(r.Context(), p, r.PathValue("id"))
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}, false))
	mux.HandleFunc("PUT /api/v1/spaces/{id}/property-definitions", s.wrap("properties.definitions_update", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		// Decode nested rows independently: the flat decoder deliberately does not
		// enforce nested required fields or reject duplicate object keys for callers.
		input, err := contract.DecodeRequest[struct {
			ExpectedRevision uint64            `json:"expected_revision"`
			Definitions      []json.RawMessage `json:"definitions"`
		}](http.MaxBytesReader(w, r.Body, 256*1024))
		if err != nil || len(input.Definitions) > 200 {
			return code("invalid_request")
		}
		q := contract.PropertyDefinitionUpdate{ExpectedRevision: input.ExpectedRevision, Definitions: []contract.PropertyDefinition{}}
		for _, raw := range input.Definitions {
			row, e := contract.DecodeRequest[contract.PropertyDefinition](bytes.NewReader(raw))
			if e != nil {
				return code("invalid_request")
			}
			q.Definitions = append(q.Definitions, row)
		}
		keys := r.Header.Values("Idempotency-Key")
		if len(keys) != 1 {
			return code("invalid_request")
		}
		out, err := s.UpdatePropertyDefinitions(r.Context(), p, r.PathValue("id"), q, keys[0])
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/spaces/{id}/property-definitions/lookup", s.wrapObserved("properties.definitions_read", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		q, err := contract.DecodeRequest[contract.IdempotencyLookupRequest](http.MaxBytesReader(w, r.Body, 2048))
		if err != nil {
			return code("invalid_request")
		}
		out, err := s.LookupPropertyDefinitionUpdate(r.Context(), p, r.PathValue("id"), q.IdempotencyKey)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}, false))
}
