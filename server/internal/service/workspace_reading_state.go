package service

import (
	"context"
	"errors"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"github.com/yeisme/drivebridge-server/pkg/personalstate"
	"gorm.io/gorm"
)

// WorkspaceReadingStates opens the shared reading-state store on the workspace
// database for one human principal. The store is the same application the
// desktop facade runs locally; states stay per deployment and desktop states
// are never uploaded across devices. Pinax references are machine-local, so
// the server refuses them instead of approximating their identity.
func (s *Service) WorkspaceReadingStates(p Principal) (*personalstate.ReadingStore, error) {
	if p.UserID == "" {
		return nil, code("unauthorized")
	}
	if err := personalstate.MigrateReading(s.DB); err != nil {
		return nil, code("database_unavailable")
	}
	return personalstate.NewReadingStore(s.DB, s.workspaceReadingAuthorize(p))
}

// workspaceReadingAuthorize resolves a fixed file reference through the
// workspace's own read admission on every call, including replays. Missing,
// trashed, purged and staged targets are hidden rather than destroyed: stored
// reading state survives target deletion and becomes readable again if the
// target returns. Scope subject is bound to the authenticated user so a
// session can never read another subject's states through this adapter.
func (s *Service) workspaceReadingAuthorize(p Principal) personalstate.Authorize {
	return func(ctx context.Context, scope personalstate.Scope, ref desktopcontract.ReadingRef) error {
		if scope.Subject != p.UserID || ref.Kind != "file" {
			return errors.New("reading_target_unavailable")
		}
		var row File
		if err := s.DB.WithContext(ctx).Where("id = ? AND owner = ? AND space_id = ?", ref.FileID, p.UserID, ref.SpaceID).First(&row).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return personalstate.ErrTargetHidden
			}
			return err
		}
		if row.Trashed || row.PurgeStartedAt != nil || workspaceStagingPath(row.Path) {
			return personalstate.ErrTargetHidden
		}
		return s.allowOn(s.DB, p, ref.SpaceID, row.Path, "read")
	}
}
