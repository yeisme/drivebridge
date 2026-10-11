package service

import "github.com/yeisme/drivebridge-server/pkg/personalstate"

// WorkspaceFavorite remains source-compatible and retains the same table,
// indexes and constraints. Shared ownership lives in personalstate.
type WorkspaceFavorite = personalstate.Favorite
