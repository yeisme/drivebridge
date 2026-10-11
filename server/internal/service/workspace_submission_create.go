package service

import (
	"crypto/rand"
	"encoding/json"
	"errors"
	"path"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// createWorkspaceSubmissionOn allocates the original upload operation and its
// grant-scoped receipt atomically. The caller supplies the transaction and audit.
// No bytes are published and no instant-blob reuse is permitted here.
func (s *Service) createWorkspaceSubmissionOn(tx *gorm.DB, p Principal, key string, input contract.CreateSubmissionRequest, now time.Time) (WorkspaceSubmission, Operation, bool, error) {
	fail := func(err error) (WorkspaceSubmission, Operation, bool, error) {
		return WorkspaceSubmission{}, Operation{}, false, err
	}
	if err := input.Validate(); err != nil {
		return fail(err)
	}
	if key == "" || len(key) > 200 || input.SizeBytes > s.Config.MaxBytes || p.Agent == nil {
		return fail(code("invalid_request"))
	}
	grant, err := s.authorizeWorkspaceActionOn(tx, p, p.Agent.SpaceID, p.Agent.ConnectionID, p.Agent.Prefix, "inbox.submit", now)
	if err != nil {
		return fail(err)
	}
	payload, err := json.Marshal(input)
	if err != nil {
		return fail(code("invalid_request"))
	}
	digest := Hash(string(payload))
	var sub WorkspaceSubmission
	err = tx.Where("owner = ? AND tenant_id = ? AND grant_id = ? AND creation_key = ?", p.UserID, p.TenantID, grant.ID, key).First(&sub).Error
	if err == nil {
		if sub.RequestDigest != digest {
			return fail(code("idempotency_conflict"))
		}
		op, bound, err := s.workspaceOperationOn(tx, p, sub.UploadOperationID, now)
		if err != nil {
			return fail(err)
		}
		return bound, op, false, nil
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fail(code("database_unavailable"))
	}
	if err := teamWritable(tx, grant.SpaceID); err != nil {
		return fail(err)
	}
	var sp Space
	if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner = ?", grant.SpaceID, p.UserID).First(&sp).Error; err != nil {
		return fail(code("not_found"))
	}
	var conn Connection
	if err := tx.Where("id = ? AND disabled = ?", grant.ConnectionID, false).First(&conn).Error; err != nil {
		return fail(code("storage_disconnected"))
	}
	if sp.Used < 0 || sp.Reserved < 0 || sp.Used > sp.MaxBytes || sp.Reserved > sp.MaxBytes-sp.Used || input.SizeBytes > sp.MaxBytes-sp.Used-sp.Reserved {
		return fail(code("quota_exceeded"))
	}
	id := ID()
	staging := path.Join(grant.Prefix, ".inbox-staging", grant.ID, id, input.Name)
	if _, err := s.authorizeWorkspaceActionOn(tx, p, grant.SpaceID, grant.ConnectionID, staging, "inbox.submit", now); err != nil {
		return fail(err)
	}
	var nonce []byte
	if sp.Encrypted {
		nonce = make([]byte, 12)
		if _, err := rand.Read(nonce); err != nil {
			return fail(code("encryption_unavailable"))
		}
	}
	op := Operation{ID: ID(), Owner: p.UserID, ActorID: p.UserID, AgentID: p.Agent.ID, Key: "workspace-inbox:" + id, Fingerprint: digest, Kind: "upload", Purpose: "inbox_staging", WorkspaceGrantID: grant.ID, SpaceID: grant.SpaceID, ConnectionID: grant.ConnectionID, Path: staging, VersionID: ID(), Size: input.SizeBytes, SHA256: input.SHA256, MIME: input.MIMEType, Caller: input.Caller, TaskRef: input.TaskRef, Status: "awaiting_upload", ReviewPending: true, Nonce: nonce, Updated: now}
	sub = WorkspaceSubmission{ID: id, Owner: p.UserID, TenantID: p.TenantID, GrantID: grant.ID, AgentID: p.Agent.ID, SpaceID: grant.SpaceID, UploadOperationID: op.ID, CreationKey: key, RequestDigest: digest, State: "uploading", Revision: 1, CreatedAt: now, UpdatedAt: now}
	if err := tx.Create(&op).Error; err != nil {
		return fail(code("database_unavailable"))
	}
	if err := tx.Create(&sub).Error; err != nil {
		return fail(code("database_unavailable"))
	}
	if err := tx.Model(&sp).Update("reserved", sp.Reserved+input.SizeBytes).Error; err != nil {
		return fail(code("database_unavailable"))
	}
	return sub, op, true, nil
}
