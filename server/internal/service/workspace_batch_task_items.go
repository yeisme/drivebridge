package service

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

func (s *Service) workspaceBatchTaskItems(w http.ResponseWriter, r *http.Request, p Principal) error {
	q := r.URL.Query()
	for key, values := range q {
		if (key != "cursor" && key != "limit") || len(values) != 1 {
			return code("invalid_request")
		}
	}
	if r.PathValue("id") == "" || len(r.PathValue("id")) > 512 {
		return code("invalid_request")
	}
	page, err := contract.ParsePageQuery(q)
	if err != nil {
		return err
	}
	result := contract.BatchTaskItems{Items: []contract.BatchTaskItem{}, ObservedAt: time.Now().UTC()}
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := workspaceGroupHumanOn(tx, p); err != nil {
			return err
		}
		var batch UploadBatch
		err := tx.Where("id = ? AND owner = ? AND tenant_id = ?", r.PathValue("id"), p.UserID, p.TenantID).First(&batch).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return code("not_found")
		}
		if err != nil {
			return code("database_unavailable")
		}
		if p.ScopeSpace != "" && p.ScopeSpace != batch.SpaceID {
			return code("not_found")
		}
		var space Space
		err = tx.Where("id = ? AND owner = ? AND tenant_id = ?", batch.SpaceID, p.UserID, p.TenantID).First(&space).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return code("not_found")
		}
		if err != nil {
			return code("database_unavailable")
		}
		if err := s.allowOn(tx, p, batch.SpaceID, "", "read"); err != nil {
			return err
		}
		var anchor Operation
		if err := tx.Where("id = ? AND owner = ?", batch.OperationID, p.UserID).First(&anchor).Error; err != nil {
			return code("task_source_unavailable")
		}
		if anchor.Kind != "upload_batch" || anchor.SpaceID != batch.SpaceID || anchor.Purpose != "" || anchor.WorkspaceGrantID != "" {
			return code("task_source_unavailable")
		}
		var rows []UploadBatchItem
		if err := tx.Where("batch_id = ? AND owner = ?", batch.ID, p.UserID).Order("id ASC").Limit(1001).Find(&rows).Error; err != nil {
			return code("database_unavailable")
		}
		if len(rows) > 1000 {
			return code("task_scan_budget_exceeded")
		}
		if batch.ItemsTotal != nil && *batch.ItemsTotal != int64(len(rows)) {
			return code("task_source_unavailable")
		}
		// Authorize every row before exposing any row or cursor: no hidden counts.
		for _, row := range rows {
			if err := s.allowOn(tx, p, batch.SpaceID, row.RelativePath, "read"); err != nil {
				return err
			}
		}
		items := make([]contract.BatchTaskItem, 0, len(rows))
		for _, row := range rows {
			item := contract.BatchTaskItem{ItemID: row.ID, RelativePath: row.RelativePath, IsDir: row.IsDir, OwnerStatus: row.Status, ResultCertainty: "unknown"}
			if row.OperationID != "" {
				var op Operation
				err := tx.Where("id = ? AND owner = ?", row.OperationID, p.UserID).First(&op).Error
				if err != nil {
					return code("task_source_unavailable")
				}
				if row.IsDir || op.Kind != "upload" || op.BatchID != batch.ID || op.ItemKey != row.ItemKey || op.SpaceID != batch.SpaceID || op.Size != row.Size || op.SHA256 != row.SHA256 || op.Purpose != "" || op.WorkspaceGrantID != "" {
					return code("task_source_unavailable")
				}
				if err := s.allowOn(tx, p, op.SpaceID, op.Path, "read"); err != nil {
					return err
				}
				id := op.ID
				item.OperationID = &id
				if row.Status == "completed" && row.Certainty == "confirmed" && op.Status == "completed" {
					_, _, found, err := operationUploadResultOn(tx, op)
					if err != nil {
						return err
					}
					if found {
						item.ResultCertainty = "confirmed"
					}
				}
			}
			items = append(items, item)
		}
		raw, err := json.Marshal(struct {
			Items    []contract.BatchTaskItem
			Revision uint64
			Phase    string
		}{items, batch.Revision, batch.Phase})
		if err != nil {
			return code("database_unavailable")
		}
		generation := Hash(string(raw))
		result.BatchID = batch.ID
		result.Revision = "sha256:" + generation
		binding, _ := json.Marshal([]any{"workspace.batch.items.v1", p.TenantID, p.ScopeSpace, p.DataOwner, batch.ID, page.Limit})
		start := 0
		if page.Cursor != "" {
			cursor, err := s.openOrgCursor(page.Cursor, p.UserID, string(binding), generation)
			if err != nil || cursor.Kind != "workspace.batch.items" {
				return code("cursor_expired")
			}
			found := false
			for i, item := range items {
				if item.ItemID == cursor.LastKey {
					start = i + 1
					found = true
					break
				}
			}
			if !found {
				return code("cursor_expired")
			}
		}
		end := start + page.Limit
		if end > len(items) {
			end = len(items)
		}
		result.Items = append(result.Items, items[start:end]...)
		if end < len(items) {
			cursor, err := s.sealOrgCursor(orgCursorPayload{Owner: p.UserID, Query: string(binding), CatalogueRevision: generation, Kind: "workspace.batch.items", LastKey: items[end-1].ItemID})
			if err != nil {
				return err
			}
			result.NextCursor = cursor
		}
		return nil
	})
	if err != nil {
		return err
	}
	respond(w, result)
	return nil
}
