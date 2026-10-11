package service

import (
	"context"
	"strings"
	"time"

	"gorm.io/gorm"
)

const (
	defaultTrashRetentionDays = 30
	maxTrashBatch             = 100
	trashSweepInterval        = time.Hour
)

func (sp *Space) BeforeCreate(tx *gorm.DB) error {
	if sp.TrashRetentionDays < 1 {
		sp.TrashRetentionDays = defaultTrashRetentionDays
	}
	return nil
}

func clampTrashRetention(days int) int {
	if days < 1 {
		return defaultTrashRetentionDays
	}
	if days > 365 {
		return 365
	}
	return days
}

func (s *Service) backfillTrashRetention() error {
	if e := s.DB.Model(&Space{}).Where("trash_retention_days < ?", 1).Update("trash_retention_days", defaultTrashRetentionDays).Error; e != nil {
		return e
	}
	now := time.Now().UTC()
	return s.DB.Model(&File{}).Where("trashed = ? AND (trashed_at IS NULL OR trashed_at < ?)", true, time.Date(2, 1, 1, 0, 0, 0, 0, time.UTC)).Update("trashed_at", now).Error
}

func (s *Service) startTrashSweep() {
	ctx, cancel := context.WithCancel(context.Background())
	s.sweepCancel = cancel
	s.sweepWG.Add(1)
	go func() {
		defer s.sweepWG.Done()
		_ = s.SweepExpiredTrash(ctx)
		tick := time.NewTicker(trashSweepInterval)
		defer tick.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				_ = s.SweepExpiredTrash(ctx)
			}
		}
	}()
}

func (s *Service) spaceRetention(spaceID string) int {
	var sp Space
	if s.DB.First(&sp, "id = ?", spaceID).Error != nil {
		return defaultTrashRetentionDays
	}
	return clampTrashRetention(sp.TrashRetentionDays)
}

func (s *Service) attachPurgeAfter(files []File) {
	cache := map[string]int{}
	for i := range files {
		if !files[i].Trashed {
			continue
		}
		days, ok := cache[files[i].SpaceID]
		if !ok {
			days = s.spaceRetention(files[i].SpaceID)
			cache[files[i].SpaceID] = days
		}
		start := files[i].TrashedAt
		if start.IsZero() || start.Year() < 2 {
			start = files[i].Updated
		}
		at := start.UTC().AddDate(0, 0, days)
		files[i].PurgeAfter = &at
	}
}

func (s *Service) SetTrashPolicy(p Principal, spaceID string, days int) (Space, error) {
	if p.Agent != nil {
		return Space{}, code("forbidden")
	}
	if days < 1 || days > 365 {
		return Space{}, code("invalid_trash_policy")
	}
	sp, err := s.space(p, spaceID)
	if err != nil {
		return Space{}, err
	}
	if e := s.allow(p, spaceID, "", "delete"); e != nil {
		return Space{}, e
	}
	if e := s.DB.Model(&sp).Update("trash_retention_days", days).Error; e != nil {
		return Space{}, code("database_unavailable")
	}
	sp.TrashRetentionDays = days
	return sp, nil
}

type BatchItemError struct {
	ID   string `json:"id"`
	Code string `json:"code"`
}

func (s *Service) BatchRestore(p Principal, ids []string) (ok []string, failed []BatchItemError, err error) {
	if p.Agent != nil && !strings.Contains(","+p.Agent.Scopes+",", ",delete,") {
		return nil, nil, code("forbidden")
	}
	if len(ids) == 0 || len(ids) > maxTrashBatch {
		return nil, nil, code("invalid_batch")
	}
	for _, id := range ids {
		if e := s.Trash(p, id, true); e != nil {
			failed = append(failed, BatchItemError{ID: id, Code: e.Error()})
			continue
		}
		ok = append(ok, id)
	}
	return ok, failed, nil
}

func (s *Service) BatchPurge(ctx context.Context, p Principal, ids []string) (ok []string, failed []BatchItemError, err error) {
	if p.Agent != nil {
		return nil, nil, code("forbidden")
	}
	if len(ids) == 0 || len(ids) > maxTrashBatch {
		return nil, nil, code("invalid_batch")
	}
	for _, id := range ids {
		if e := s.Purge(ctx, p, id); e != nil {
			failed = append(failed, BatchItemError{ID: id, Code: e.Error()})
			continue
		}
		ok = append(ok, id)
	}
	return ok, failed, nil
}

func (s *Service) SweepExpiredTrash(ctx context.Context) error {
	var spaces []Space
	if e := s.DB.Find(&spaces).Error; e != nil {
		return e
	}
	now := time.Now().UTC()
	for _, sp := range spaces {
		days := clampTrashRetention(sp.TrashRetentionDays)
		cutoff := now.AddDate(0, 0, -days)
		var files []File
		if e := s.DB.Where("space_id = ? AND trashed = ? AND trashed_at > ? AND trashed_at <= ?", sp.ID, true, time.Date(2, 1, 1, 0, 0, 0, 0, time.UTC), cutoff).Find(&files).Error; e != nil {
			return e
		}
		for _, f := range files {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			_ = s.Purge(ctx, Principal{UserID: f.Owner}, f.ID)
		}
	}
	return nil
}
