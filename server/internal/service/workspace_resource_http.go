package service

import (
	"encoding/json"
	"net/http"
	"net/url"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func (s *Service) resourceRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/resource-descriptors/{id}/checks", s.wrap("org.collections_read", s.resourceCheckHandler(false)))
	mux.HandleFunc("POST /api/v1/resource-descriptors/{id}/checks/lookup", s.wrapObserved("org.collections_read", s.resourceCheckHandler(true), false))
	mux.HandleFunc("POST /api/v1/resource-descriptors/lookup", s.wrapObserved("org.collections_read", s.workspaceResourceLookup, false))
	mux.HandleFunc("POST /api/v1/resource-descriptors", s.wrap("org.collections_create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		input, err := contract.DecodeRequest[contract.RegisterResourceRequest](http.MaxBytesReader(w, r.Body, 8192))
		if err != nil {
			return code("invalid_request")
		}
		keys := r.Header.Values("Idempotency-Key")
		if len(keys) != 1 {
			return code("invalid_request")
		}
		out, err := s.RegisterWorkspaceResource(p, input, keys[0])
		if err != nil {
			return err
		}
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusCreated)
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/resource-descriptors/{id}", s.wrap("org.collections_read", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		out, err := s.WorkspaceResource(p, r.PathValue("id"))
		if err != nil {
			return err
		}
		respond(w, out)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/resource-descriptors", s.wrap("org.collections_list", s.workspaceResourceList))
}

func (s *Service) workspaceResourceList(w http.ResponseWriter, r *http.Request, p Principal) error {
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil {
		return code("invalid_request")
	}
	for key, values := range q {
		if len(values) != 1 {
			return code("invalid_request")
		}
		switch key {
		case "space_id", "kind", "cursor", "limit":
		default:
			return code("invalid_request")
		}
	}
	if q.Has("space_id") && (q.Get("space_id") == "" || len(q.Get("space_id")) > 512) {
		return code("invalid_request")
	}
	switch q.Get("kind") {
	case "", "skill", "template", "reference_pack":
	default:
		return code("invalid_request")
	}
	page, err := contract.ParsePageQuery(q)
	if err != nil {
		return err
	}
	out := contract.Page[contract.ResourceView]{Items: []contract.ResourceView{}, TotalKind: "unknown", ObservedAt: time.Now().UTC()}
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := workspaceGroupHumanOn(tx, p); err != nil {
			return err
		}
		query := tx.Where("owner = ? AND tenant_id = ?", p.UserID, p.TenantID)
		if p.ScopeSpace != "" {
			query = query.Where("space_id = ?", p.ScopeSpace)
		}
		if q.Get("space_id") != "" {
			query = query.Where("space_id = ?", q.Get("space_id"))
		}
		if q.Get("kind") != "" {
			query = query.Where("kind = ?", q.Get("kind"))
		}
		var rows []WorkspaceResourceDescriptor
		if err := query.Order("id ASC").Limit(1001).Find(&rows).Error; err != nil {
			return code("database_unavailable")
		}
		if len(rows) > 1000 {
			return code("query_budget_exceeded")
		}
		views := []contract.ResourceView{}
		budget := 1000
		for _, row := range rows {
			view, err := s.resourceViewBudgetOn(tx, p, row, &budget)
			if err != nil {
				if workspaceVisibilityDenied(err) {
					continue
				}
				return err
			}
			views = append(views, view)
		}
		raw, err := json.Marshal(views)
		if err != nil {
			return code("database_unavailable")
		}
		generation := Hash(string(raw))
		bound, _ := json.Marshal([]any{"resources.v1", p.TenantID, p.ScopeSpace, p.DataOwner, q.Get("space_id"), q.Get("kind"), page.Limit})
		start := 0
		if page.Cursor != "" {
			c, err := s.openOrgCursor(page.Cursor, p.UserID, string(bound), generation)
			if err != nil || c.Kind != "resources" {
				return code("cursor_expired")
			}
			found := false
			for i, view := range views {
				if view.DescriptorID == c.LastKey {
					start = i + 1
					found = true
					break
				}
			}
			if !found {
				return code("cursor_expired")
			}
		}
		end := start + page.Limit
		if end > len(views) {
			end = len(views)
		}
		out.Items = append(out.Items, views[start:end]...)
		if end < len(views) {
			out.NextCursor, err = s.sealOrgCursor(orgCursorPayload{Owner: p.UserID, Query: string(bound), CatalogueRevision: generation, Kind: "resources", LastKey: views[end-1].DescriptorID})
			if err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return err
	}
	respond(w, out)
	return nil
}
