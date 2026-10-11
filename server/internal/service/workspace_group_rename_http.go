package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func (s *Service) workspaceGroupRename(w http.ResponseWriter, r *http.Request, p Principal) error {
	input, err := contract.DecodeRequest[contract.RenameGroupRequest](r.Body)
	if err != nil {
		return err
	}
	if _, _, err := workspaceGroupName(input.Name); err != nil {
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
		Input contract.RenameGroupRequest
	}{id, input})
	if err != nil {
		return code("invalid_request")
	}
	digest := Hash(string(payload))
	var group WorkspaceAgentGroup
	s.mu.Lock()
	defer s.mu.Unlock()
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := workspaceGroupHumanOn(tx, p); err != nil {
			return err
		}
		var receipt WorkspaceRequest
		err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "group.rename", keys[0]).First(&receipt).Error
		if err == nil {
			if receipt.RequestDigest != digest {
				return code("idempotency_conflict")
			}
			if err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", id, p.UserID, p.TenantID).First(&group).Error; err != nil {
				return code("not_found")
			}
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		now := time.Now().UTC()
		group, err = s.renameWorkspaceGroupOn(tx, p, id, input.Name, input.ExpectedRevision, now)
		if err != nil {
			return err
		}
		receipt = WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: "group.rename", RequestKey: keys[0], RequestDigest: digest, ResourceID: id, ResultRevision: group.Revision, CreatedAt: now}
		if err := tx.Create(&receipt).Error; err != nil {
			return code("database_unavailable")
		}
		audit := Audit{Owner: p.UserID, ActorID: p.UserID, TenantID: p.TenantID, Action: "workspace.group.rename", Resource: id, Outcome: "success", At: now}
		if err := tx.Create(&audit).Error; err != nil {
			return code("audit_unavailable")
		}
		return nil
	})
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	respond(w, workspaceGroupView(group))
	return nil
}
