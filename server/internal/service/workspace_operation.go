package service

import (
	"time"

	"gorm.io/gorm"
)

// workspaceOperationOn authorizes recovery and transfer of an existing inbox
// operation. It does not confer file-read authority or admit legacy uploads.
func (s *Service) workspaceOperationOn(tx *gorm.DB, p Principal, id string, now time.Time) (Operation, WorkspaceSubmission, error) {
	var op Operation
	var submission WorkspaceSubmission
	denied := func() (Operation, WorkspaceSubmission, error) {
		return Operation{}, WorkspaceSubmission{}, code("forbidden")
	}
	if p.Agent == nil || id == "" {
		return denied()
	}
	if err := tx.Where("id = ? AND owner = ?", id, p.UserID).First(&op).Error; err != nil {
		return denied()
	}
	if op.Purpose != "inbox_staging" || op.WorkspaceGrantID == "" || op.AgentID != p.Agent.ID || op.ActorID != p.UserID || op.Kind != "upload" || op.ExpectedVersion != "" || op.BatchID != "" || op.ItemKey != "" || op.ArchiveFileID != "" || op.RenameOnConflict {
		return denied()
	}
	if err := tx.Where("upload_operation_id = ? AND owner = ? AND tenant_id = ?", op.ID, p.UserID, p.TenantID).First(&submission).Error; err != nil {
		return denied()
	}
	if submission.GrantID != op.WorkspaceGrantID || submission.AgentID != op.AgentID || submission.SpaceID != op.SpaceID {
		return denied()
	}
	grant, err := s.authorizeWorkspaceActionOn(tx, p, op.SpaceID, op.ConnectionID, op.Path, "inbox.submit", now)
	if err != nil {
		return Operation{}, WorkspaceSubmission{}, err
	}
	if grant.ID != op.WorkspaceGrantID {
		return denied()
	}
	return op, submission, nil
}
