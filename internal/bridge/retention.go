package bridge

import (
	"context"
	"errors"
	"time"
)

// SpaceRetention carries the per-space retention policy: TTLDays is 0 for a
// permanent space (the default) and positive for scratch spaces whose
// unreferenced files a manual gc may collect.
type SpaceRetention struct {
	SpaceID string `json:"space_id" gorm:"primaryKey"`
	TTLDays int    `json:"ttl_days"`
}

// FileRetention tracks per-reference retention state. Pinned references are
// never collected; LastRef records the most recent explicit reference
// activity (link, reuse, pin) on top of the event-derived baseline.
type FileRetention struct {
	Ref     string    `json:"ref" gorm:"primaryKey"`
	Pinned  bool      `json:"pinned"`
	LastRef time.Time `json:"last_ref"`
}

// ensureRetention lazily creates the retention tables next to the core state.
// It runs once per database and is safe to call from every retention entry
// point.
func (s *Service) ensureRetention() error {
	if s.DB.Migrator().HasTable(&FileRetention{}) && s.DB.Migrator().HasTable(&SpaceRetention{}) {
		return nil
	}
	if err := s.DB.AutoMigrate(&SpaceRetention{}, &FileRetention{}); err != nil {
		return errors.New("state_unavailable")
	}
	return nil
}

// touchRetention records fresh reference activity without changing the pin.
func (s *Service) touchRetention(space, ref string) {
	if err := s.ensureRetention(); err != nil {
		return
	}
	var row FileRetention
	if s.DB.First(&row, "ref = ?", ref).Error == nil {
		if row.Pinned {
			s.DB.Model(&FileRetention{}).Where("ref = ?", ref).Update("last_ref", time.Now().UTC())
			return
		}
		row.LastRef = time.Now().UTC()
		_ = s.DB.Save(&row).Error
		return
	}
	_ = s.DB.Create(&FileRetention{Ref: ref, LastRef: time.Now().UTC()}).Error
}

// SetSpaceTTL configures the retention policy for a space. Zero restores the
// permanent default; negative values are rejected.
func (s *Service) SetSpaceTTL(space string, days int) error {
	if days < 0 {
		return errors.New("invalid_space")
	}
	if _, err := s.Space(space); err != nil {
		return err
	}
	if err := s.ensureRetention(); err != nil {
		return err
	}
	if err := s.DB.Save(&SpaceRetention{SpaceID: space, TTLDays: days}).Error; err != nil {
		return errors.New("state_unavailable")
	}
	return nil
}

// TTL reports the configured retention window for a space (0 = permanent).
func (s *Service) TTL(space string) (int, error) {
	if _, err := s.Space(space); err != nil {
		return 0, err
	}
	var row SpaceRetention
	if s.DB.First(&row, "space_id = ?", space).Error != nil {
		return 0, nil
	}
	return row.TTLDays, nil
}

// Pin raises the retention marker on a reference: a pinned reference is never
// collected by gc regardless of age.
func (s *Service) Pin(ref string) (File, error) {
	if !s.Enabled {
		return File{}, errors.New("disabled")
	}
	var f File
	if s.DB.First(&f, "ref = ?", ref).Error != nil {
		return File{}, errors.New("reference_not_found")
	}
	if f.Trashed != nil {
		return File{}, errors.New("file_trashed")
	}
	if err := s.ensureRetention(); err != nil {
		return File{}, err
	}
	if err := s.DB.Save(&FileRetention{Ref: ref, Pinned: true, LastRef: time.Now().UTC()}).Error; err != nil {
		return File{}, errors.New("state_unavailable")
	}
	return f, nil
}

// GCReport is the manual collection report. Collected references were moved
// to the space trash (restorable); nothing is permanently deleted here.
type GCReport struct {
	Space       string   `json:"space"`
	TTLDays     int      `json:"ttl_days"`
	DryRun      bool     `json:"dry_run"`
	Scanned     int      `json:"scanned"`
	Pinned      int      `json:"pinned"`
	Candidates  []string `json:"candidates"`
	Collected   []string `json:"collected"`
	Failed      int      `json:"failed"`
	CollectedAt string   `json:"collected_at,omitempty"`
}

// GC manually collects retention-expired references in a space. A space with
// no TTL configured (the default) reports zero candidates: gc never guesses an
// expiry for a permanent space. Files whose last activity is unknown are kept.
func (s *Service) GC(ctx context.Context, space string, dryRun bool) (GCReport, error) {
	if !s.Enabled {
		return GCReport{}, errors.New("disabled")
	}
	p, err := s.Space(space)
	if err != nil {
		return GCReport{}, err
	}
	if err = s.ensureRetention(); err != nil {
		return GCReport{}, err
	}
	report := GCReport{Space: space, DryRun: dryRun, Candidates: []string{}, Collected: []string{}}
	var policy SpaceRetention
	if s.DB.First(&policy, "space_id = ?", space).Error != nil {
		policy.TTLDays = 0
	}
	report.TTLDays = policy.TTLDays
	if policy.TTLDays <= 0 {
		return report, nil
	}
	cutoff := time.Now().UTC().Add(-time.Duration(policy.TTLDays) * 24 * time.Hour)
	var files []File
	if e := s.DB.Where("space_id = ? AND trashed_at IS NULL", space).Find(&files).Error; e != nil {
		return GCReport{}, errors.New("state_unavailable")
	}
	report.Scanned = len(files)
	for _, f := range files {
		var row FileRetention
		pinned := false
		if s.DB.First(&row, "ref = ?", f.Ref).Error == nil {
			pinned = row.Pinned
		}
		if pinned {
			report.Pinned++
			continue
		}
		last := row.LastRef
		if last.IsZero() {
			var ev Event
			if s.DB.Where("space_id = ? AND ref = ?", p.ID, f.Ref).Order("id DESC").First(&ev).Error == nil {
				last = ev.At
			}
		}
		// Unknown age is never a reason to collect.
		if last.IsZero() || !last.Before(cutoff) {
			continue
		}
		report.Candidates = append(report.Candidates, f.Ref)
	}
	if dryRun || len(report.Candidates) == 0 {
		return report, nil
	}
	report.CollectedAt = time.Now().UTC().Format(time.RFC3339)
	for _, ref := range report.Candidates {
		if _, e := s.Trash(ctx, ref); e != nil {
			report.Failed++
			continue
		}
		report.Collected = append(report.Collected, ref)
	}
	return report, nil
}
