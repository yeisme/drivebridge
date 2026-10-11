package insights

import (
	"context"
	"errors"
	"gorm.io/gorm/clause"
	"time"
)

type InventoryPage struct {
	Entries   int64
	Bytes     int64
	Next      string
	Resumable bool
}
type Inventory func(context.Context, string) (InventoryPage, error)

func (s *Store) Configure(owner, space, connection string, enabled bool, maxEntries, maxSeconds int) (ScanPolicy, error) {
	p := ScanPolicy{Key: key(owner, space), Owner: owner, Space: space, Connection: connection, Enabled: enabled, MaxEntries: maxEntries, MaxSeconds: maxSeconds}
	if maxEntries == 0 {
		p.MaxEntries = 100000
	}
	if maxSeconds == 0 {
		p.MaxSeconds = 600
	}
	if p.MaxEntries < 1 || p.MaxEntries > 10000000 || p.MaxSeconds < 1 || p.MaxSeconds > 3600 {
		return p, errors.New("invalid_scan_budget")
	}
	if e := s.DB.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoUpdates: clause.AssignmentColumns([]string{"connection", "enabled", "max_entries", "max_seconds"})}).Create(&p).Error; e != nil {
		return p, ErrUnavailable
	}
	s.DB.First(&p, "key = ?", p.Key)
	return p, nil
}
func (s *Store) Policies() ([]ScanPolicy, error) {
	p := []ScanPolicy{}
	e := s.DB.Where("enabled = ?", true).Find(&p).Error
	return p, e
}
func (s *Store) Scan(ctx context.Context, owner, space string, fetch Inventory, resume string) (ScanRun, error) {
	var run ScanRun
	s.scanMu.Lock()
	if s.closing || !s.scanning.CompareAndSwap(false, true) {
		s.scanMu.Unlock()
		return run, errors.New("scan_busy")
	}
	s.scanWG.Add(1)
	s.scanMu.Unlock()
	defer func() {
		s.scanMu.Lock()
		s.scanCancel = nil
		s.scanID = ""
		s.scanning.Store(false)
		s.scanMu.Unlock()
		s.scanWG.Done()
	}()
	var p ScanPolicy
	if s.DB.First(&p, "owner = ? AND space = ?", owner, space).Error != nil || !p.Enabled {
		return run, errors.New("scan_not_enabled")
	}
	now := s.Now().UTC()
	if resume != "" {
		if s.DB.First(&run, "id = ? AND owner = ? AND space = ?", resume, owner, space).Error != nil {
			return run, errors.New("scan_not_found")
		}
		if !run.Resumable || run.Cursor == "" || run.Status == "complete" {
			return run, errors.New("scan_not_resumable")
		}
	} else {
		run = ScanRun{ID: ID(), Owner: owner, Space: space, Connection: p.Connection, Started: now, Coverage: "visible_objects_non_atomic_no_hidden_versions"}
	}
	run.Status = "running"
	run.Error = ""
	run.Finished = nil
	if s.DB.Save(&run).Error != nil {
		return run, ErrUnavailable
	}
	if s.DB.Model(&p).Update("last_attempt", now).Error != nil {
		return run, ErrUnavailable
	}
	child, cancel := context.WithTimeout(ctx, time.Duration(p.MaxSeconds)*time.Second)
	defer cancel()
	s.scanMu.Lock()
	s.scanCancel = cancel
	s.scanID = run.ID
	if s.closing {
		cancel()
	}
	s.scanMu.Unlock()
	startEntries := run.Entries
	var failure error
	for {
		if failure = child.Err(); failure != nil {
			break
		}
		var page InventoryPage
		page, failure = fetch(child, run.Cursor)
		if failure != nil {
			break
		}
		if page.Bytes < 0 || page.Entries < 0 {
			failure = errors.New("invalid_inventory")
			break
		}
		if run.Entries-startEntries+page.Entries > int64(p.MaxEntries) {
			failure = ErrBudget
			break
		}
		if child.Err() != nil {
			failure = child.Err()
			break
		}
		if page.Next != "" && page.Next == run.Cursor {
			failure = errors.New("scan_cursor_stalled")
			break
		}
		run.Entries += page.Entries
		run.Bytes += page.Bytes
		run.Cursor = page.Next
		run.Resumable = page.Resumable
		if s.DB.Save(&run).Error != nil {
			failure = ErrUnavailable
			break
		}
		if run.Cursor == "" {
			break
		}
	}
	done := s.Now().UTC()
	run.Finished = &done
	if failure == nil {
		run.Status = "complete"
	} else {
		run.Status = "partial"
		switch {
		case errors.Is(failure, context.Canceled):
			run.Status = "cancelled"
			run.Error = "scan_cancelled"
		case errors.Is(failure, context.DeadlineExceeded), errors.Is(failure, ErrBudget):
			run.Error = "scan_budget_exceeded"
		case errors.Is(failure, ErrUnavailable):
			run.Error = "insights_unavailable"
		default:
			run.Status = "failed"
			run.Error = "inventory_unavailable"
		}
	}
	if s.DB.Save(&run).Error != nil {
		s.Gap()
		return run, ErrUnavailable
	}
	return run, nil
}
func (s *Store) Scans(scope Scope) ([]ScanRun, error) {
	out := []ScanRun{}
	if scope.RestrictOperations || scope.Actor != "" {
		return out, nil
	}
	if apply(s.DB, scope).Order("started DESC").Limit(100).Find(&out).Error != nil {
		return nil, ErrUnavailable
	}
	return out, nil
}

func (s *Store) CancelScan(owner, space, id string) error {
	var run ScanRun
	if s.DB.First(&run, "id = ? AND owner = ? AND space = ?", id, owner, space).Error != nil {
		return errors.New("scan_not_found")
	}
	s.scanMu.Lock()
	defer s.scanMu.Unlock()
	if s.scanID != id || s.scanCancel == nil {
		return errors.New("scan_not_active")
	}
	s.scanCancel()
	return nil
}
