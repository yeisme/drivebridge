package service

import (
	"errors"
	"math"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// patchWorkspaceOrganizationOn changes personal organization only. Callers
// own the transaction, receipt and audit; authority remains in Agent/grant.
func (s *Service) patchWorkspaceOrganizationOn(tx *gorm.DB, p Principal, agentID string, input contract.PatchOrganizationRequest, now time.Time) (WorkspaceAgentOrganization, error) {
	var org WorkspaceAgentOrganization
	if err := workspaceGroupHumanOn(tx, p); err != nil {
		return org, err
	}
	if err := input.Validate(); err != nil {
		return org, err
	}
	if (!input.GroupID.Present && !input.Pinned.Present) || input.ExpectedRevision == math.MaxUint64 {
		return org, code("invalid_request")
	}
	if err := s.authorizeWorkspaceOrganizationOn(tx, p, agentID); err != nil {
		return org, err
	}
	if input.GroupID.Present && !input.GroupID.Null {
		var group WorkspaceAgentGroup
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner = ? AND tenant_id = ?", input.GroupID.Value, p.UserID, p.TenantID).First(&group).Error; err != nil {
			if !errors.Is(err, gorm.ErrRecordNotFound) {
				return org, code("database_unavailable")
			}
			// A scoped durable delete receipt proves a previously owned group;
			// unknown or foreign IDs must not disclose that distinction.
			var deleted int64
			if err := tx.Model(&WorkspaceRequest{}).Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND resource_id = ?", p.UserID, p.TenantID, p.UserID, "group.delete", input.GroupID.Value).Count(&deleted).Error; err != nil {
				return org, code("database_unavailable")
			}
			if deleted > 0 {
				return org, code("revision_conflict")
			}
			return org, code("not_found")
		}
	}
	err := tx.Where("owner = ? AND tenant_id = ? AND agent_id = ?", p.UserID, p.TenantID, agentID).First(&org).Error
	fresh := errors.Is(err, gorm.ErrRecordNotFound)
	if err != nil && !fresh {
		return org, code("database_unavailable")
	}
	if fresh {
		if input.ExpectedRevision != 0 {
			return org, code("revision_conflict")
		}
		org = WorkspaceAgentOrganization{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, AgentID: agentID, CreatedAt: now}
	} else if org.Revision != input.ExpectedRevision {
		return org, code("revision_conflict")
	}
	if input.GroupID.Present {
		org.GroupID = nil
		if !input.GroupID.Null {
			id := input.GroupID.Value
			org.GroupID = &id
		}
	}
	if input.Pinned.Present {
		org.Pinned = input.Pinned.Value
	}
	org.Revision++
	org.UpdatedAt = now
	if fresh {
		if err := tx.Create(&org).Error; err != nil {
			return org, code("database_unavailable")
		}
	} else {
		result := tx.Model(&WorkspaceAgentOrganization{}).Where("id = ? AND revision = ?", org.ID, input.ExpectedRevision).Updates(map[string]any{"group_id": org.GroupID, "pinned": org.Pinned, "revision": org.Revision, "updated_at": now})
		if result.Error != nil {
			return org, code("database_unavailable")
		}
		if result.RowsAffected != 1 {
			return org, code("revision_conflict")
		}
	}
	return org, nil
}
