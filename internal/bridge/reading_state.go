package bridge

import (
	"context"
	"errors"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"github.com/yeisme/drivebridge-server/pkg/personalstate"
)

// ReadingStates opens the shared personal reading-state store on this local
// service state. Scope subject and instance come from the trusted desktop
// host identity, never from renderer input. States stay on this machine: the
// store never uploads to the server and never writes target files.
func (s *Service) ReadingStates() (*personalstate.ReadingStore, error) {
	if err := personalstate.MigrateReading(s.DB); err != nil {
		return nil, errors.New("state_unavailable")
	}
	return personalstate.NewReadingStore(s.DB, s.authorizeReading)
}

// authorizeReading resolves a fixed reference against local state only. File
// references must resolve to a known file in their space; a trashed target is
// hidden, not removed, so stored reading state survives trash and restore.
// Pinax notes have no ready binding at this stage and are refused instead of
// guessed; refusing never deletes stored progress.
func (s *Service) authorizeReading(ctx context.Context, scope personalstate.Scope, ref desktopcontract.ReadingRef) error {
	if ref.Kind != "file" {
		return errors.New("reading_target_unavailable")
	}
	var space Space
	if s.DB.WithContext(ctx).First(&space, "id = ?", ref.SpaceID).Error != nil {
		return personalstate.ErrTargetHidden
	}
	var file File
	if s.DB.WithContext(ctx).Where("ref = ? AND space_id = ?", ref.FileID, ref.SpaceID).First(&file).Error != nil {
		return personalstate.ErrTargetHidden
	}
	if file.Trashed != nil {
		return personalstate.ErrTargetHidden
	}
	return nil
}
