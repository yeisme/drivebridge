package insights

import (
	"crypto/sha256"
	"encoding/hex"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
	"strings"
	"time"
)

func key(parts ...string) string {
	h := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(h[:])
}
func Open(db *gorm.DB) (*Store, error) {
	s := &Store{DB: db, Now: time.Now}
	if e := db.AutoMigrate(&Fact{}, &Daily{}, &Capacity{}, &AuditEvent{}, &AuditDaily{}, &ScanPolicy{}, &ScanRun{}, &Collection{}); e != nil {
		return nil, ErrUnavailable
	}
	c := Collection{ID: 1, Started: s.Now().UTC(), Heartbeat: s.Now().UTC(), Clean: true}
	if e := db.Clauses(clause.OnConflict{DoNothing: true}).Create(&c).Error; e != nil {
		return nil, ErrUnavailable
	}
	if e := db.First(&c, 1).Error; e != nil {
		return nil, ErrUnavailable
	}
	gap := c.Gap || !c.Clean
	s.Degraded.Store(false)
	if e := db.Model(&Collection{}).Where("id = ?", 1).Updates(map[string]any{"clean": false, "gap": gap, "heartbeat": s.Now().UTC()}).Error; e != nil {
		return nil, ErrUnavailable
	}
	// A previous scanner cannot still hold the owner process lock.
	db.Model(&ScanRun{}).Where("status = ?", "running").Updates(map[string]any{"status": "interrupted", "error": "scan_interrupted"})
	return s, nil
}
func ReadOnly(db *gorm.DB) *Store { return &Store{DB: db, Now: time.Now} }
func (s *Store) Close() {
	s.scanMu.Lock()
	s.closing = true
	if s.scanCancel != nil {
		s.scanCancel()
	}
	s.scanMu.Unlock()
	s.scanWG.Wait()
	s.DB.Model(&Collection{}).Where("id = ?", 1).Updates(map[string]any{"clean": true, "heartbeat": s.Now().UTC()})
}
func (s *Store) Gap() {
	s.Degraded.Store(true)
	s.DB.Model(&Collection{}).Where("id = ?", 1).Update("gap", true)
}
func (s *Store) Record(f Fact) error {
	if f.ID == "" {
		f.ID = ID()
	}
	if f.At.IsZero() {
		f.At = s.Now().UTC()
	}
	for _, v := range []string{f.Owner, f.Space, f.Connection, f.Actor, f.ActorKind, f.Caller, f.Operation, f.Resource, f.Metric, f.Direction, f.Purpose} {
		if !boundedID(v) {
			return ErrUnavailable
		}
	}
	if f.Bytes < 0 {
		return ErrUnavailable
	}
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		r := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&f)
		if r.Error != nil {
			return r.Error
		}
		if r.RowsAffected == 0 {
			return nil
		}
		d := Daily{Dimensions: f.Dimensions, Day: Day(f.At), Metric: f.Metric, Direction: f.Direction, Purpose: f.Purpose, Bytes: f.Bytes, Count: 1}
		d.Key = key(d.Owner, d.Space, d.Connection, d.Actor, d.ActorKind, d.Caller, d.Operation, d.Resource, d.Day, d.Metric, d.Direction, d.Purpose)
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoUpdates: clause.Assignments(map[string]any{"bytes": gorm.Expr("bytes + ?", d.Bytes), "count": gorm.Expr("count + 1")})}).Create(&d).Error
	})
	if e != nil {
		s.Gap()
		return ErrUnavailable
	}
	s.Degraded.Store(false)
	return nil
}
func (s *Store) Capture(v Capacity) error {
	v.Day = Day(s.Now())
	v.At = s.Now().UTC()
	v.Key = key(v.Owner, v.Space, v.Day)
	if e := s.DB.Clauses(clause.OnConflict{UpdateAll: true}).Create(&v).Error; e != nil {
		s.Gap()
		return ErrUnavailable
	}
	return nil
}
func (s *Store) Log(e AuditEvent) error {
	for _, v := range []string{e.Owner, e.Space, e.Actor, e.ActorKind, e.Action, e.Outcome, e.Resource, e.Version, e.Operation, e.Correlation, e.Entry} {
		if !boundedID(v) {
			return ErrAudit
		}
	}
	if e.At.IsZero() {
		e.At = s.Now().UTC()
	}
	if e.Correlation == "" {
		e.Correlation = ID()
	}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(&e).Error; err != nil {
			return err
		}
		d := AuditDaily{Owner: e.Owner, Actor: e.Actor, Day: Day(e.At), Action: e.Action, Outcome: e.Outcome, Count: 1}
		d.Key = key(d.Owner, d.Actor, d.Day, d.Action, d.Outcome)
		return tx.Clauses(clause.OnConflict{Columns: []clause.Column{{Name: "key"}}, DoUpdates: clause.Assignments(map[string]any{"count": gorm.Expr("count + 1")})}).Create(&d).Error
	})
	if err != nil {
		s.AuditDegraded.Store(true)
		s.DB.Model(&Collection{}).Where("id = ?", 1).Update("audit_unavailable", true)
		s.Gap()
		return ErrAudit
	}
	if s.AuditDegraded.Swap(false) {
		s.DB.Model(&Collection{}).Where("id = ?", 1).Update("audit_unavailable", false)
		// We can prove a gap occurred, not reconstruct its missing events.
		gap := AuditEvent{Owner: e.Owner, Action: "audit.gap", Outcome: "history_incomplete", ActorKind: "system", Correlation: ID(), Entry: "recovery", At: s.Now().UTC()}
		if s.DB.Create(&gap).Error != nil {
			s.AuditDegraded.Store(true)
		}
	}
	return nil
}
func apply(q *gorm.DB, s Scope) *gorm.DB {
	q = q.Where("owner = ?", s.Owner)
	if s.Space != "" {
		q = q.Where("space = ?", s.Space)
	}
	if s.Connection != "" {
		q = q.Where("connection = ?", s.Connection)
	}
	return q
}
func dimensions(q *gorm.DB, s Scope, f Filter) *gorm.DB {
	q = apply(q, s).Where("day >= ? AND day <= ?", f.From, f.To)
	actor := f.Actor
	if s.Actor != "" {
		actor = s.Actor
	}
	if actor != "" {
		q = q.Where("actor = ?", actor)
	}
	if s.RestrictOperations {
		q = q.Where("(operation IN ? OR resource IN ?)", s.OperationIDs, s.ResourceIDs)
	}
	if f.Caller != "" {
		q = q.Where("caller = ?", f.Caller)
	}
	if f.Direction != "" {
		q = q.Where("direction = ?", f.Direction)
	}
	if f.Purpose != "" {
		q = q.Where("purpose = ?", f.Purpose)
	}
	return q
}
func (s *Store) Prune() error {
	now := s.Now().UTC()
	// Facts and aggregates are committed atomically. Never prune if that guarantee
	// is currently degraded; retained details permit operator investigation.
	if s.Degraded.Load() || s.AuditDegraded.Load() {
		return ErrUnavailable
	}
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		for _, model := range []any{&Fact{}, &AuditEvent{}} {
			if e := tx.Where("at < ?", now.AddDate(0, 0, -90)).Delete(model).Error; e != nil {
				return e
			}
		}
		for _, model := range []any{&Daily{}, &AuditDaily{}, &Capacity{}} {
			if e := tx.Where("day < ?", Day(now.AddDate(-1, 0, 0))).Delete(model).Error; e != nil {
				return e
			}
		}
		// Keep policy and latest complete scan for every scope even if it is stale.
		policies := []ScanPolicy{}
		if e := tx.Find(&policies).Error; e != nil {
			return e
		}
		for _, p := range policies {
			var last ScanRun
			tx.Where("owner = ? AND space = ? AND status = ?", p.Owner, p.Space, "complete").Order("started DESC").First(&last)
			if e := tx.Where("owner = ? AND space = ? AND started < ? AND id <> ?", p.Owner, p.Space, now.AddDate(0, 0, -90), last.ID).Delete(&ScanRun{}).Error; e != nil {
				return e
			}
		}
		return tx.Model(&Collection{}).Where("id = ?", 1).Updates(map[string]any{"last_pruned": now, "heartbeat": now}).Error
	})
	if e != nil {
		s.Gap()
		return ErrUnavailable
	}
	return nil
}
