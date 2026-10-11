package service

import (
	"net/http"
	"time"

	"gorm.io/gorm"
)

func workspaceOperationProgress(op Operation) map[string]any {
	return map[string]any{"id": op.ID, "kind": op.Kind, "purpose": op.Purpose, "status": op.Status, "size_bytes": op.Size, "bytes_transferred": op.Transferred, "updated_at": op.Updated}
}

// Recovery exposes transfer progress, not storage locations or file contents.
func (s *Service) workspaceOperationGet(w http.ResponseWriter, r *http.Request, p Principal) error {
	view, err := s.workspaceOperationRecovery(p, r.PathValue("id"))
	if err != nil {
		return err
	}
	w.Header().Set("Cache-Control", "no-store")
	respond(w, view)
	return nil
}

func (s *Service) workspaceOperationRecovery(p Principal, id string) (map[string]any, error) {
	var op Operation
	indexes := []int{}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		current, _, err := s.workspaceOperationOn(tx, p, id, time.Now().UTC())
		if err != nil {
			return err
		}
		op = current
		if err := tx.Model(&Part{}).Where("operation_id = ?", op.ID).Order("`index`").Pluck("index", &indexes).Error; err != nil {
			return code("database_unavailable")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return map[string]any{
		"operation":      workspaceOperationProgress(op),
		"received_parts": indexes,
	}, nil
}
