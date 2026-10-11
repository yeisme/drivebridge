package service

import (
	"encoding/json"
	"net/http"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func (s *Service) workspaceSubmissionArchive(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return code("forbidden")
	}
	input, err := contract.DecodeRequest[contract.ArchiveSubmissionRequest](r.Body)
	if err != nil {
		return err
	}
	keys := r.Header.Values("Idempotency-Key")
	if len(keys) != 1 || keys[0] == "" || len(keys[0]) > 200 {
		return code("invalid_request")
	}
	for _, ch := range keys[0] {
		if ch < 33 || ch > 126 {
			return code("invalid_request")
		}
	}
	id := r.PathValue("id")
	payload, err := json.Marshal(struct {
		ID    string
		Input contract.ArchiveSubmissionRequest
	}{id, input})
	if err != nil {
		return code("invalid_request")
	}
	digest := Hash(string(payload))
	var op Operation
	var q ArchiveRequest
	var found bool
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		op, found, err = s.workspaceArchiveReplayOn(tx, p, id, keys[0], digest)
		if err != nil || found {
			return err
		}
		q, err = s.prepareWorkspaceArchiveOn(tx, p, id, input)
		return err
	})
	if err != nil {
		return err
	}
	if !found {
		if err := s.requireManagedTarget(p, q.TargetSpaceID); err != nil {
			return err
		}
		var source File
		if err := s.DB.Where("id = ? AND owner = ?", q.SourceFileID, p.UserID).First(&source).Error; err != nil {
			return code("not_found")
		}
		var version Version
		if err := s.DB.Where("id = ? AND file_id = ? AND owner = ?", q.SourceVersionID, source.ID, p.UserID).First(&version).Error; err != nil {
			return code("not_found")
		}
		origin := &ingestOrigin{Kind: "archive", Source: q, Workspace: &workspaceArchiveIntent{SubmissionID: id, RequestKey: keys[0], RequestDigest: digest, Request: input}}
		identity, _ := json.Marshal([]string{p.TenantID, p.UserID, keys[0]})
		op, err = s.createUpload(p, UploadRequest{Caller: source.Caller, TaskRef: source.TaskRef, SpaceID: q.TargetSpaceID, Path: q.TargetPath, Size: version.Size, SHA256: version.SHA256}, "workspace-archive:"+Hash(string(identity)), origin)
		if err != nil {
			return err
		}
	}
	// Replay never schedules another transfer. Recovery uses the original
	// operation; only a newly allocated operation starts streaming here.
	if !found && op.Status != "completed" {
		op, err = s.streamExistingVersion(r.Context(), p, op, op.ArchiveFileID, op.ArchiveVersionID)
		if err != nil {
			return err
		}
	}
	view := contract.ArchiveSubmissionView{OperationRef: op.ID}
	if op.Status == "completed" && op.FileID != "" {
		view.FileRef = &op.FileID
	}
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(http.StatusAccepted)
	respond(w, view)
	return nil
}
