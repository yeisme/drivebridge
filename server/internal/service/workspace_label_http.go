package service

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/url"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

func (s *Service) labelRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/spaces/{id}/label-definitions", s.wrapObserved("labels.definitions_read", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		values, page, err := labelPageQuery(r.URL.RawQuery, true)
		if err != nil { return err }
		namespace := values.Get("namespace")
		if namespace == "" { namespace = "manual" }
		out, err := s.LabelDefinitions(r.Context(), p, r.PathValue("id"), namespace, page.Cursor, page.Limit)
		if err != nil { return err }
		respond(w, out)
		return nil
	}, false))
	mux.HandleFunc("PUT /api/v1/spaces/{id}/label-definitions", s.wrap("labels.definitions_update", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" { return code("invalid_request") }
		input, err := contract.DecodeRequest[struct {
			ExpectedRevision uint64 `json:"expected_revision"`
			Definitions []json.RawMessage `json:"definitions"`
		}](http.MaxBytesReader(w, r.Body, 256*1024))
		if err != nil || len(input.Definitions) > 200 { return code("invalid_request") }
		q := contract.LabelDefinitionUpdate{ExpectedRevision: input.ExpectedRevision, Definitions: []contract.LabelDefinitionInput{}}
		for _, raw := range input.Definitions {
			row, err := contract.DecodeRequest[contract.LabelDefinitionInput](bytes.NewReader(raw))
			if err != nil { return code("invalid_request") }
			q.Definitions = append(q.Definitions, row)
		}
		key, err := workspaceMetadataKey(r)
		if err != nil { return err }
		out, err := s.UpdateLabelDefinitions(r.Context(), p, r.PathValue("id"), q, key)
		if err != nil { return err }
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/spaces/{id}/label-definitions/lookup", s.wrapObserved("labels.definitions_read", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" { return code("invalid_request") }
		q, err := contract.DecodeRequest[contract.IdempotencyLookupRequest](http.MaxBytesReader(w, r.Body, 2048))
		if err != nil { return code("invalid_request") }
		out, err := s.LookupLabelDefinitionUpdate(r.Context(), p, r.PathValue("id"), q.IdempotencyKey)
		if err != nil { return err }
		respond(w, out)
		return nil
	}, false))
	mux.HandleFunc("POST /api/v1/spaces/{id}/label-merge-plans", s.wrap("labels.merge_preview", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" { return code("invalid_request") }
		q, err := contract.DecodeRequest[contract.LabelMergePreview](http.MaxBytesReader(w, r.Body, 128*1024))
		if err != nil { return code("invalid_request") }
		key, err := workspaceMetadataKey(r)
		if err != nil { return err }
		out, err := s.CreateLabelMergePlan(r.Context(), p, r.PathValue("id"), q, key)
		if err != nil { return err }
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/spaces/{id}/label-merge-plans/lookup", s.wrapObserved("labels.merge_read", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" { return code("invalid_request") }
		q, err := contract.DecodeRequest[contract.IdempotencyLookupRequest](http.MaxBytesReader(w, r.Body, 2048))
		if err != nil { return code("invalid_request") }
		out, err := s.LookupLabelMergePlan(r.Context(), p, r.PathValue("id"), q.IdempotencyKey)
		if err != nil { return err }
		respond(w, out)
		return nil
	}, false))
}

func labelPageQuery(raw string, namespace bool) (url.Values, contract.PageQuery, error) {
	values, err := url.ParseQuery(raw)
	if err != nil { return nil, contract.PageQuery{}, code("invalid_request") }
	for name, entries := range values {
		if len(entries) != 1 || (name != "cursor" && name != "limit" && !(namespace && name == "namespace")) { return nil, contract.PageQuery{}, code("invalid_request") }
	}
	if entries, ok := values["namespace"]; ok && entries[0] != "manual" { return nil, contract.PageQuery{}, code("invalid_request") }
	page, err := contract.ParsePageQuery(values)
	return values, page, err
}
