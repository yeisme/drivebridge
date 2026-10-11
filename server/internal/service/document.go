package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"path"
	"strings"
	"unicode/utf8"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func ordinaryDocument(file File) bool {
	ext := strings.ToLower(path.Ext(file.Path))
	return ext == ".md" || ext == ".markdown" || ext == ".txt"
}

func (s *Service) DocumentCapability(p Principal, id string) (contract.DocumentCapability, error) {
	view := contract.DocumentCapability{MaxBytes: contract.DocumentMaxBytes}
	if err := workspaceGroupHumanOn(s.DB, p); err != nil {
		return view, err
	}
	file, err := s.file(p, id, "read")
	if err != nil {
		return view, err
	}
	view.FileID, view.VersionID = file.ID, file.VersionID
	view.Reason = "unsupported"
	if !s.Config.DocumentEditing || !ordinaryDocument(file) || file.Size > contract.DocumentMaxBytes || file.Trashed {
		return view, nil
	}
	if err = s.allow(p, file.SpaceID, file.Path, "write"); err != nil {
		if !workspaceVisibilityDenied(err) {
			return view, err
		}
		view.Reason = "forbidden"
		return view, nil
	}
	if s.governanceActive() {
		if err = s.allow(p, file.SpaceID, file.Path, "overwrite"); err != nil {
			if !workspaceVisibilityDenied(err) {
				return view, err
			}
			view.Reason = "forbidden"
			return view, nil
		}
	}
	view.Editable, view.Reason = true, "ready"
	return view, nil
}

// The operation key is scoped to the authenticated actor, tenant and target.
// Each replay also verifies actor, target, base revision and body digest.
func documentKey(p Principal, id, key string) (string, error) {
	if !provenanceID.MatchString(key) {
		return "", code("invalid_request")
	}
	b, _ := json.Marshal([]string{p.UserID, p.TenantID, id, key})
	return "document:" + Hash(string(b)), nil
}

func (s *Service) documentResult(op Operation) contract.DocumentSaveView {
	view := contract.DocumentSaveView{FileID: op.FileID, OperationID: op.ID, Status: op.Status}
	if op.Status == "completed" {
		view.VersionID = op.VersionID
		view.FileRef = s.FileRef(File{ID: op.FileID, VersionID: op.VersionID})
	}
	return view
}

func (s *Service) LookupDocumentSave(p Principal, id, key string) (contract.DocumentLookupView, error) {
	var view contract.DocumentLookupView
	if err := workspaceGroupHumanOn(s.DB, p); err != nil {
		return view, err
	}
	if _, err := s.file(p, id, "read"); err != nil {
		return view, err
	}
	key, err := documentKey(p, id, key)
	if err != nil {
		return view, err
	}
	var op Operation
	err = s.DB.Where("owner = ? AND key = ? AND file_id = ? AND actor_id = ?", p.dataOwner(), key, id, p.UserID).First(&op).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return view, nil
	}
	if err != nil {
		return view, code("database_unavailable")
	}
	result := s.documentResult(op)
	return contract.DocumentLookupView{Found: true, Save: &result}, nil
}

// SaveDocument reuses durable upload parts, quota, verified publication and CAS.
// No new body store or parallel version writer is introduced.
func (s *Service) SaveDocument(ctx context.Context, p Principal, id string, q contract.SaveDocumentRequest, key string) (contract.DocumentSaveView, error) {
	var empty contract.DocumentSaveView
	if err := workspaceGroupHumanOn(s.DB, p); err != nil {
		return empty, err
	}
	if !s.Config.DocumentEditing {
		return empty, code("unsupported_action")
	}
	if q.BaseVersionID == "" || len(q.Text) > contract.DocumentMaxBytes || !utf8.ValidString(q.Text) || strings.ContainsRune(q.Text, 0) {
		return empty, code("invalid_request")
	}
	key, err := documentKey(p, id, key)
	if err != nil {
		return empty, err
	}
	file, err := s.file(p, id, "write")
	if err != nil {
		return empty, err
	}
	if file.Trashed || !ordinaryDocument(file) {
		return empty, code("unsupported_action")
	}
	request := UploadRequest{SpaceID: file.SpaceID, Path: file.Path, Size: int64(len(q.Text)), SHA256: Hash(q.Text), FileID: id, ExpectedVersion: q.BaseVersionID}
	// A path change after publication must not break readback of a lost response.
	var prior Operation
	lookupErr := s.DB.Where("owner = ? AND key = ?", p.dataOwner(), key).First(&prior).Error
	if lookupErr == nil {
		if prior.ActorID != p.UserID || prior.FileID != id || prior.ExpectedVersion != q.BaseVersionID || prior.SHA256 != request.SHA256 || prior.Size != request.Size {
			return empty, code("idempotency_conflict")
		}
		if prior.Status == "completed" {
			return s.documentResult(prior), nil
		}
		request.Path = prior.Path
	} else if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
		return empty, code("database_unavailable")
	}
	op, err := s.CreateUpload(p, request, key)
	if err != nil {
		return empty, err
	}
	if op.Status == "completed" {
		return s.documentResult(op), nil
	}
	if request.Size > 0 {
		if _, err = s.PutPart(p, op.ID, 0, strings.NewReader(q.Text)); err != nil {
			// Another identical request may have completed while this one waited.
			if current, readErr := s.operation(p, op.ID, "read"); readErr == nil && current.Status == "completed" {
				return s.documentResult(current), nil
			}
			return empty, err
		}
	}
	op, err = s.Complete(ctx, p, op.ID)
	if err != nil {
		// A deterministic CAS rejection never published this operation. Release
		// its reservation through the existing cancellation path, not raw updates.
		if safeCode(err) == "version_conflict" {
			if _, cancelErr := s.Cancel(p, op.ID); cancelErr != nil {
				return empty, cancelErr
			}
		}
		return empty, err
	}
	return s.documentResult(op), nil
}

func (s *Service) documentRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/files/{id}/document-capability", s.wrap("files.document_capability", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		result, err := s.DocumentCapability(p, r.PathValue("id"))
		if err != nil {
			return err
		}
		w.Header().Set("Cache-Control", "private, no-store")
		respond(w, result)
		return nil
	}))
	mux.HandleFunc("PUT /api/v1/files/{id}/document", s.wrap("files.document_save", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		// JSON escaping can expand valid UTF-8 text; the service enforces byte budget.
		r.Body = http.MaxBytesReader(w, r.Body, 13*1024*1024)
		q, err := contract.DecodeRequest[contract.SaveDocumentRequest](r.Body)
		if err != nil {
			return code("invalid_request")
		}
		result, err := s.SaveDocument(r.Context(), p, r.PathValue("id"), q, r.Header.Get("Idempotency-Key"))
		if err != nil {
			return err
		}
		w.Header().Set("Cache-Control", "private, no-store")
		respond(w, result)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/files/{id}/document/lookup", s.wrap("files.document_lookup", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		q, err := contract.DecodeRequest[contract.IdempotencyLookupRequest](r.Body)
		if err != nil {
			return code("invalid_request")
		}
		result, err := s.LookupDocumentSave(p, r.PathValue("id"), q.IdempotencyKey)
		if err != nil {
			return err
		}
		w.Header().Set("Cache-Control", "private, no-store")
		respond(w, result)
		return nil
	}))
}
