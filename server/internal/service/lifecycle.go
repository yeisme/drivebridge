package service

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// uploadSessionExpired reports whether the durable backend transfer window
// lapsed: S3 multipart sessions must complete within 7 days of initiation,
// so parts after that point land on a dead backend session.
func uploadSessionExpired(session UploadSession) bool {
	return session.ExpiresAt != nil && time.Now().UTC().After(*session.ExpiresAt)
}
func (s *Service) operationContext(ctx context.Context, id string) (context.Context, func()) {
	child, cancel := context.WithCancel(ctx)
	s.activeMu.Lock()
	s.active[id] = cancel
	s.activeMu.Unlock()
	return child, func() { cancel(); s.activeMu.Lock(); delete(s.active, id); s.activeMu.Unlock() }
}
func (s *Service) interruptOperation(id string) {
	s.activeMu.Lock()
	cancel := s.active[id]
	s.activeMu.Unlock()
	if cancel != nil {
		cancel()
	}
}
func (s *Service) failedOperation(id string, e error) {
	if e == nil {
		return
	}
	s.DB.Model(&Operation{}).Where("id = ? AND status NOT IN ?", id, []string{"completed", "cancelled"}).Updates(map[string]any{"status": "failed", "error": safeCode(e), "updated": time.Now().UTC()})
}

// Resume continues an existing operation from stored fields. It must not
// reconstruct a new fingerprint from a defaulted conflict mode or a renamed
// destination — that is how default-rename copy/directory_copy became
// unresumable (idempotency_conflict).
func (s *Service) Resume(ctx context.Context, p Principal, id string) (Operation, error) {
	op, e := s.operation(p, id, "transfer")
	if e != nil {
		return Operation{}, e
	}
	if op.Status == "completed" || op.Status == "cancelled" {
		return op, nil
	}
	switch op.Kind {
	case "import":
		return s.runImport(ctx, p, op)
	case "export":
		return s.runExport(ctx, p, op)
	case "archive", "copy":
		// All parts are already staged once verification begins. Re-sending
		// them is rejected by putPart and cannot recover a read-back failure.
		if op.Status == "verifying" {
			return s.Complete(ctx, p, op.ID)
		}
		return s.streamExistingVersion(ctx, p, op, op.ArchiveFileID, op.ArchiveVersionID)
	case "directory_copy":
		files, dirs, err := s.listPrefix(p, op.SourceSpaceID, op.SourcePath)
		if err != nil {
			return op, err
		}
		return s.runDirectoryCopy(ctx, p, op, files, dirs)
	case "directory_export":
		return s.ExportDirectory(ctx, p, DirectoryExportRequest{SpaceID: op.SpaceID, Path: op.SourcePath, ConnectionID: op.ExternalConnectionID}, op.Key)
	default:
		if op.BatchID != "" && op.ItemKey != "" {
			var session UploadSession
			if s.DB.First(&session, "operation_id = ?", op.ID).Error == nil && uploadSessionExpired(session) {
				// Explicit recovery by the original actor: rebuild the
				// backend transfer session under the same operation. The
				// generation records the rebuild; client part numbering and
				// the business identity never change.
				window := time.Now().UTC().Add(7 * 24 * time.Hour)
				s.DB.Model(&UploadSession{}).Where("id = ?", session.ID).Updates(map[string]any{"generation": gorm.Expr("generation + 1"), "expires_at": window, "updated_at": time.Now().UTC()})
			}
		}
		if op.Error == "original_source_required" {
			// The staged source bytes are gone (metadata-only restore):
			// resume must not pretend the missing parts can be re-sent
			// blind against nothing.
			return op, code("source_reselection_required")
		}
		if op.Transferred == op.Size {
			return s.Complete(ctx, p, op.ID)
		}
		s.DB.Model(&op).Updates(map[string]any{"status": "awaiting_upload", "error": "", "updated": time.Now().UTC()})
		op.Status, op.Error = "awaiting_upload", ""
		return op, nil
	}
}
