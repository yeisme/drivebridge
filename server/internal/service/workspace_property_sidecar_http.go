package service

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"unicode/utf8"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

func (s *Service) propertySidecarRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/files/{id}/properties", s.wrapObserved("properties.values_read", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		q, err := url.ParseQuery(r.URL.RawQuery)
		if err != nil || len(q) != 1 || len(q["version_id"]) != 1 || !propertyHexID(q.Get("version_id")) {
			return code("invalid_request")
		}
		out, err := s.FilePropertySidecar(r.Context(), p, r.PathValue("id"), q.Get("version_id"))
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}, false))
	mux.HandleFunc("PATCH /api/v1/files/{id}/properties", s.wrap("properties.values_update", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, sidecarBudget))
		if err != nil {
			return code("invalid_request")
		}
		// Validate the original bytes before flat DTO decoding can re-encode keys
		// or replace malformed UTF-16 escapes. Persist values from the original map.
		if !utf8.Valid(raw) || !propertyJSONSurrogates(raw) {
			return code("invalid_request")
		}
		if _, err = contract.DecodeRequest[contract.PropertySidecarPatch](bytes.NewReader(raw)); err != nil {
			return code("invalid_request")
		}
		var q contract.PropertySidecarPatch
		if json.Unmarshal(raw, &q) != nil {
			return code("invalid_request")
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil {
			return code("invalid_request")
		}
		values, err := decodeSidecarValues(string(fields["updates"]))
		if err != nil {
			return code("invalid_request")
		}
		q.Updates = values
		keys := r.Header.Values("Idempotency-Key")
		if len(keys) != 1 {
			return code("invalid_request")
		}
		out, err := s.PatchFilePropertySidecar(r.Context(), p, r.PathValue("id"), q, keys[0])
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/files/{id}/properties/lookup", s.wrapObserved("properties.values_read", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		q, err := contract.DecodeRequest[contract.PropertySidecarLookup](http.MaxBytesReader(w, r.Body, 2048))
		if err != nil {
			return code("invalid_request")
		}
		out, err := s.LookupFilePropertySidecar(r.Context(), p, r.PathValue("id"), q.VersionID, q.IdempotencyKey)
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}, false))
}
