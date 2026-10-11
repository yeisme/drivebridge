package service

import (
	"time"

	"gorm.io/gorm"
)

// Lock order remains per-operation lock -> service mutex. Never wait on a
// child's operation lock while holding s.mu: Complete/Cancel need s.mu too.
func (s *Service) cancelUploadBatch(p Principal, id string) (UploadBatchView, error) {
	var operations []string
	s.mu.Lock()
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var batch UploadBatch
		if err := tx.Where("id = ? AND owner = ?", id, p.dataOwner()).First(&batch).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				return code("not_found")
			}
			return code("database_unavailable")
		}
		if batch.TenantID != p.TenantID {
			return code("not_found")
		}
		var anchor Operation
		if err := tx.Where("id = ? AND owner = ?", batch.OperationID, p.dataOwner()).First(&anchor).Error; err != nil {
			return code("database_unavailable")
		}
		if anchor.Kind != "upload_batch" || anchor.SpaceID != batch.SpaceID || anchor.Purpose != "" || anchor.WorkspaceGrantID != "" {
			return code("invalid_operation")
		}
		if err := s.allowOn(tx, p, batch.SpaceID, "", "write"); err != nil {
			return err
		}
		if batch.Phase == "completed" {
			return code("operation_closed")
		}
		var items []UploadBatchItem
		if err := tx.Where("batch_id = ? AND owner = ? AND status IN ?", id, p.dataOwner(), []string{"transferring", "failed"}).Find(&items).Error; err != nil {
			return code("database_unavailable")
		}
		for _, item := range items {
			if item.OperationID == "" {
				if item.Status == "transferring" {
					return code("invalid_operation")
				}
				continue
			}
			var op Operation
			if err := tx.Where("id = ? AND owner = ?", item.OperationID, p.dataOwner()).First(&op).Error; err != nil {
				return code("database_unavailable")
			}
			if op.Kind != "upload" || op.BatchID != id || op.ItemKey != item.ItemKey || op.SpaceID != batch.SpaceID || op.Size != item.Size || op.SHA256 != item.SHA256 || op.Purpose != "" || op.WorkspaceGrantID != "" {
				return code("invalid_operation")
			}
			if err := s.allowOn(tx, p, op.SpaceID, op.Path, "write"); err != nil {
				return err
			}
			if s.governanceActive() && op.ActorID != "" && op.ActorID != p.UserID {
				return code("forbidden")
			}
			operations = append(operations, op.ID)
		}
		// Already cancelled is a resumable cleanup intent, not a new control write.
		if batch.Phase == "cancelled" {
			return nil
		}
		now := time.Now().UTC()
		if err := tx.Model(&UploadBatchItem{}).Where("batch_id = ? AND owner = ? AND status = ?", id, p.dataOwner(), "pending").Updates(map[string]any{"status": "cancelled", "updated_at": now}).Error; err != nil {
			return code("database_unavailable")
		}
		if err := tx.Model(&Operation{}).Where("id = ? AND owner = ?", batch.OperationID, p.dataOwner()).Updates(map[string]any{"status": "cancelled", "updated": now}).Error; err != nil {
			return code("database_unavailable")
		}
		return tx.Model(&UploadBatch{}).Where("id = ? AND owner = ?", id, p.dataOwner()).Updates(map[string]any{"phase": "cancelled", "revision": gorm.Expr("revision + 1"), "updated_at": now}).Error
	})
	s.mu.Unlock()
	if err != nil {
		return UploadBatchView{}, err
	}
	// The persisted terminal phase prevents new scheduling even if cleanup
	// fails or this process exits. Repeating cancel continues the original IDs.
	for _, operation := range operations {
		result, err := s.Cancel(p, operation)
		if err != nil {
			return UploadBatchView{}, err
		}
		// Cancel is idempotent at operation level. Reconcile a prior interrupted
		// aggregate update as well; completed files always remain completed.
		s.mu.Lock()
		if result.Status == "completed" {
			s.finishBatchItem(result, "completed", "")
		} else if result.Status == "cancelled" {
			s.finishBatchItem(result, "cancelled", "operation_cancelled")
		}
		s.mu.Unlock()
	}
	// Never report cleanup success while a linked child remains unsettled.
	var pending int64
	if err := s.DB.Model(&UploadBatchItem{}).Where("batch_id = ? AND owner = ? AND status IN ?", id, p.dataOwner(), []string{"transferring", "failed"}).Where("operation_id <> ?", "").Count(&pending).Error; err != nil {
		return UploadBatchView{}, code("database_unavailable")
	}
	if pending != 0 {
		return UploadBatchView{}, code("database_unavailable")
	}
	return s.GetUploadBatch(p, id, "", 0)
}
