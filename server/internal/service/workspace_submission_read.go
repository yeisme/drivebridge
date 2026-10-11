package service

import (
	"errors"
	"net/http"
	"path"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func (s *Service) workspaceSubmissionGet(w http.ResponseWriter, r *http.Request, p Principal) error {
	if p.UserID == "" || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" {
		return code("forbidden")
	}
	var view contract.SubmissionView
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var err error
		view, err = s.workspaceSubmissionViewOn(tx, p, r.PathValue("id"))
		return err
	})
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	respond(w, view)
	return nil
}

func (s *Service) workspaceSubmissionViewOn(tx *gorm.DB, p Principal, id string) (contract.SubmissionView, error) {
	var view contract.SubmissionView
	err := func() error {
		var sub WorkspaceSubmission
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.UserID, p.TenantID).First(&sub).Error; err != nil {
			return code("not_found")
		}
		var original Operation
		if err := tx.Where("id = ? AND owner = ?", sub.UploadOperationID, sub.Owner).First(&original).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return code("not_found")
			}
			return code("database_unavailable")
		}
		if original.SpaceID != sub.SpaceID || original.WorkspaceGrantID != sub.GrantID || original.AgentID != sub.AgentID || original.Purpose != "inbox_staging" {
			return code("forbidden")
		}
		if p.Agent != nil {
			if sub.AgentID != p.Agent.ID {
				return code("not_found")
			}
			if _, _, err := s.workspaceOperationOn(tx, p, sub.UploadOperationID, time.Now().UTC()); err != nil {
				return err
			}
		} else {
			var owner User
			if err := tx.Where("id = ?", p.UserID).First(&owner).Error; err != nil || owner.Disabled || owner.Archived {
				return code("forbidden")
			}
			if (p.ScopeSpace != "" && p.ScopeSpace != sub.SpaceID) || (p.DataOwner != "" && p.DataOwner != sub.Owner) {
				return code("not_found")
			}
			var space Space
			if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", sub.SpaceID, p.UserID, p.TenantID).First(&space).Error; err != nil {
				return code("not_found")
			}
			if err := s.governanceAllow(tx, p, space.ID, original.Path, "read"); err != nil {
				return err
			}
		}
		view = contract.SubmissionView{SubmissionID: sub.ID, Revision: sub.Revision, SpaceID: sub.SpaceID, AgentID: sub.AgentID, GrantID: sub.GrantID, SourceKind: "agent", UploadOperationRef: sub.UploadOperationID, State: sub.State, CreatedAt: sub.CreatedAt, UpdatedAt: sub.UpdatedAt}
		view.FileName, view.MIMEType, view.SizeBytes, view.SourceClaim = path.Base(original.Path), original.MIME, &original.Size, original.Caller
		var author Agent
		if err := tx.Select("name").Where("id = ? AND owner = ? AND space_id = ?", sub.AgentID, sub.Owner, sub.SpaceID).First(&author).Error; err == nil {
			view.AgentName = author.Name
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		if sub.FileID != nil {
			view.FileRef = *sub.FileID
		}
		if sub.VersionID != nil {
			view.VersionID = *sub.VersionID
		}
		if sub.State == "accepted" || sub.State == "rejected" || sub.State == "archiving" || sub.State == "archived" {
			var decision WorkspaceSubmissionDecision
			if err := tx.Where("submission_id = ?", sub.ID).Order("revision DESC").First(&decision).Error; err != nil {
				return code("database_unavailable")
			}
			view.Decision = &contract.SubmissionDecisionView{VersionID: decision.VersionID, Decision: decision.Decision, Reason: decision.Reason, DecidedAt: decision.CreatedAt}
		}
		if sub.ArchiveOperationID != nil {
			view.ArchiveOperationRef = *sub.ArchiveOperationID
		}
		if sub.ArchivedFileRef != nil {
			view.ArchivedFileRef = *sub.ArchivedFileRef
		}
		return nil
	}()
	return view, err
}
