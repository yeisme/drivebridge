package service

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

// organizationRoutes registers the virtual organization surface: typed
// queries, label authority, collections, snapshots and saved queries.
// Every route reuses the same application service the SDK and MCP tools
// call; agents act only inside their scopes and the label namespace rule
// is enforced at the service layer.
func (s *Service) organizationRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/files/{id}/label-suggestions", s.wrap("org.labels_get", s.workspaceSuggestionList))
	mux.HandleFunc("POST /api/v1/files/{id}/label-suggestions", s.wrap("org.labels_set", s.workspaceSuggestionSubmit))
	mux.HandleFunc("POST /api/v1/label-suggestions/{id}/decision", s.wrap("org.labels_set", s.workspaceSuggestionDecision))
	mux.HandleFunc("POST /api/v1/file-queries", s.wrap("org.file_queries", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q FileQueryRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		if q.Mode == "duplicates" {
			view, e := s.QueryDuplicateCandidates(p, q.SpaceIDs)
			if e != nil {
				return e
			}
			respond(w, view)
			return nil
		}
		view, e := s.QueryFiles(p, q)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/files/{id}/labels", s.wrap("org.labels_get", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.LabelAssignmentsFor(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("PUT /api/v1/files/{id}/labels", s.wrap("org.labels_set", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q SetFileLabelsRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.SetFileLabels(p, r.PathValue("id"), q)
		if e != nil {
			return e
		}
		markAuditTarget(r, r.PathValue("id"))
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/file-collections", s.wrap("org.collections_list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		views, e := s.ListFileCollections(p, r.URL.Query().Get("space_id"))
		if e != nil {
			return e
		}
		respond(w, views)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/file-collections/lookup", s.wrapObserved("org.collections_read", s.collectionCreationLookup, false))
	mux.HandleFunc("POST /api/v1/file-collections", s.wrap("org.collections_create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if r.URL.RawQuery != "" {
			return code("invalid_request")
		}
		raw, e := io.ReadAll(http.MaxBytesReader(w, r.Body, 128*1024))
		if e != nil {
			return code("invalid_request")
		}
		q, e := contract.DecodeRequest[CreateFileCollectionRequest](bytes.NewReader(raw))
		if e != nil {
			return code("invalid_request")
		}
		// The legacy rule fingerprint includes RawMessage whitespace. Validate
		// the closed shape first, then retain the original nested rule bytes.
		if json.Unmarshal(raw, &q) != nil {
			return code("invalid_request")
		}
		keys := r.Header.Values("Idempotency-Key")
		if len(keys) != 1 {
			return code("invalid_request")
		}
		view, e := s.CreateFileCollection(p, q, keys[0])
		if e != nil {
			return e
		}
		markAuditTarget(r, view.CollectionID)
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/file-collections/{id}", s.wrap("org.collections_read", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.ReadFileCollection(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/file-collections/{id}/members/decisions", s.wrapObserved("org.collections_read", s.collectionMemberDecisionsHandler, false))
	mux.HandleFunc("POST /api/v1/file-collections/{id}/members/changes", s.wrap("org.members_set", s.collectionMembersChangeHandler(false)))
	mux.HandleFunc("POST /api/v1/file-collections/{id}/members/changes/lookup", s.wrapObserved("org.collections_read", s.collectionMembersChangeHandler(true), false))
	mux.HandleFunc("PUT /api/v1/file-collections/{id}/members", s.wrap("org.members_set", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q SetCollectionMembersRequest
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.SetCollectionMembers(p, r.PathValue("id"), q)
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/file-collections/{id}/snapshots", s.wrapObserved("org.snapshots_read", s.snapshotIndexHandler, false))
	mux.HandleFunc("POST /api/v1/file-collections/{id}/snapshots/creations", s.wrap("org.snapshots_create", s.snapshotCreationHandler(false)))
	mux.HandleFunc("POST /api/v1/file-collections/{id}/snapshots/creations/lookup", s.wrapObserved("org.snapshots_read", s.snapshotCreationHandler(true), false))
	mux.HandleFunc("POST /api/v1/file-collections/{id}/snapshots", s.wrap("org.snapshots_create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.CreateFileSnapshot(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		markAuditTarget(r, view.SnapshotID)
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/collection-snapshots/{id}", s.wrap("org.snapshots_read", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		view, e := s.ReadSnapshot(p, r.PathValue("id"))
		if e != nil {
			return e
		}
		respond(w, view)
		return nil
	}))
	mux.HandleFunc("GET /api/v1/saved-searches", s.wrap("org.saved_list", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		views, e := s.SavedFileQueriesList(p)
		if e != nil {
			return e
		}
		respond(w, views)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/saved-searches", s.wrap("org.saved_create", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q struct {
			Name  string           `json:"name"`
			Query FileQueryRequest `json:"query"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		view, e := s.SavedFileQueryCreate(p, q.Name, q.Query)
		if e != nil {
			return e
		}
		markAuditTarget(r, view.QueryID)
		respond(w, view)
		return nil
	}))
	_ = json.Marshal
}
