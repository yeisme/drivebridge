// Package insights defines versioned, provider-neutral usage and audit projections.
// Each owner stores these records in its own database. It never transfers them
// between instances and never accepts file contents or provider credentials.
package insights

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"gorm.io/gorm"
	"regexp"
	"sync"
	"sync/atomic"
	"time"
)

const MetricVersion = "1.0"

var ErrUnavailable = errors.New("insights_unavailable")
var ErrAudit = errors.New("audit_unavailable")
var ErrBudget = errors.New("scan_budget_exceeded")
var safeLabel = regexp.MustCompile(`^[a-zA-Z0-9._:-]{0,200}$`)

func ID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return hex.EncodeToString(b)
}
func safe(v string) bool     { return safeLabel.MatchString(v) }
func Day(t time.Time) string { return t.UTC().Format("2006-01-02") }

type Dimensions struct {
	Owner      string `json:"-" gorm:"index"`
	Space      string `json:"space_id,omitempty" gorm:"index"`
	Connection string `json:"connection_id,omitempty"`
	Actor      string `json:"actor_id,omitempty"`
	ActorKind  string `json:"actor_kind"`
	Caller     string `json:"caller,omitempty"`
	Resource   string `json:"resource_id,omitempty"`
	Operation  string `json:"operation_id,omitempty" gorm:"index"`
}
type Fact struct {
	Sequence uint64 `json:"sequence" gorm:"primaryKey"`
	ID       string `json:"id" gorm:"uniqueIndex"`
	Dimensions
	Metric    string    `json:"metric"`
	Direction string    `json:"direction,omitempty"`
	Purpose   string    `json:"purpose,omitempty"`
	Bytes     int64     `json:"bytes"`
	At        time.Time `json:"at" gorm:"index"`
}

func (Fact) TableName() string { return "insight_facts" }

type Daily struct {
	Key string `json:"-" gorm:"primaryKey"`
	Dimensions
	Day       string `json:"day" gorm:"index"`
	Metric    string `json:"metric"`
	Direction string `json:"direction,omitempty"`
	Purpose   string `json:"purpose,omitempty"`
	Bytes     int64  `json:"bytes"`
	Count     int64  `json:"count"`
}

func (Daily) TableName() string { return "insight_daily" }

type Collection struct {
	ID               uint      `gorm:"primaryKey" json:"-"`
	Started          time.Time `json:"collection_started_at"`
	Heartbeat        time.Time `json:"last_heartbeat"`
	Clean            bool      `json:"clean_shutdown"`
	AuditUnavailable bool      `json:"audit_unavailable"`
	Gap              bool      `json:"gap_detected"`
	LastPruned       time.Time `json:"last_pruned_at"`
}

func (Collection) TableName() string { return "insight_collection" }

type Capacity struct {
	Key        string    `json:"-" gorm:"primaryKey"`
	Owner      string    `json:"-" gorm:"index"`
	Space      string    `json:"space_id" gorm:"index"`
	Connection string    `json:"connection_id"`
	Day        string    `json:"day" gorm:"index"`
	Current    int64     `json:"current_bytes"`
	History    *int64    `json:"history_bytes"`
	Trash      int64     `json:"trash_bytes"`
	Reserved   int64     `json:"reserved_bytes"`
	Quota      *int64    `json:"quota_bytes"`
	Coverage   string    `json:"coverage"`
	At         time.Time `json:"observed_at"`
}

func (Capacity) TableName() string { return "insight_capacity" }

type AuditEvent struct {
	ID           uint64    `json:"id" gorm:"primaryKey"`
	Owner        string    `json:"-" gorm:"index"`
	Space        string    `json:"space_id,omitempty"`
	Connection   string    `json:"connection_id,omitempty"`
	Actor        string    `json:"actor_id,omitempty"`
	ActorKind    string    `json:"actor_kind"`
	Action       string    `json:"action"`
	Outcome      string    `json:"outcome"`
	ResourceKind string    `json:"resource_kind,omitempty"`
	Resource     string    `json:"resource_id,omitempty"`
	Version      string    `json:"version_id,omitempty"`
	Operation    string    `json:"operation_id,omitempty"`
	Correlation  string    `json:"correlation_id" gorm:"index"`
	Entry        string    `json:"entry"`
	At           time.Time `json:"at" gorm:"index"`
}

func (AuditEvent) TableName() string { return "insight_audit" }

type AuditDaily struct {
	Key     string `json:"-" gorm:"primaryKey"`
	Owner   string `json:"-" gorm:"index"`
	Actor   string `json:"actor_id,omitempty"`
	Day     string `json:"day" gorm:"index"`
	Action  string `json:"action"`
	Outcome string `json:"outcome"`
	Count   int64  `json:"count"`
}

func (AuditDaily) TableName() string { return "insight_audit_daily" }

type ScanPolicy struct {
	Key         string    `json:"-" gorm:"primaryKey"`
	Owner       string    `json:"-" gorm:"index"`
	Space       string    `json:"space_id"`
	Connection  string    `json:"connection_id"`
	Enabled     bool      `json:"enabled"`
	MaxEntries  int       `json:"max_entries"`
	MaxSeconds  int       `json:"max_seconds"`
	LastAttempt time.Time `json:"last_attempt_at"`
}

func (ScanPolicy) TableName() string { return "insight_scan_policy" }

type ScanRun struct {
	ID         string     `json:"id" gorm:"primaryKey"`
	Owner      string     `json:"-" gorm:"index"`
	Space      string     `json:"space_id" gorm:"index"`
	Connection string     `json:"connection_id"`
	Status     string     `json:"status"`
	Entries    int64      `json:"entries"`
	Bytes      int64      `json:"observed_bytes"`
	Started    time.Time  `json:"started_at"`
	Finished   *time.Time `json:"finished_at,omitempty"`
	Error      string     `json:"error,omitempty"`
	Coverage   string     `json:"coverage"`
	Resumable  bool       `json:"resumable"`
	Cursor     string     `json:"-"`
}

func (ScanRun) TableName() string { return "insight_scans" }

type Store struct {
	DB            *gorm.DB
	Now           func() time.Time
	Degraded      atomic.Bool
	AuditDegraded atomic.Bool
	scanning      atomic.Bool
	scanMu        sync.Mutex
	scanWG        sync.WaitGroup
	scanCancel    context.CancelFunc
	scanID        string
	closing       bool
}

// Scope must be constructed by the owning authorization layer, not the client.
type Scope struct {
	Owner              string
	Actor              string
	Space              string
	Connection         string
	OperationIDs       []string
	ResourceIDs        []string
	RestrictOperations bool
}
type Filter struct {
	From      string `json:"from"`
	To        string `json:"to"`
	Actor     string `json:"actor"`
	Caller    string `json:"caller"`
	Direction string `json:"direction"`
	Purpose   string `json:"purpose"`
}

func (f *Filter) Validate(now time.Time) error {
	if f.To == "" {
		f.To = Day(now)
	}
	if f.From == "" {
		f.From = Day(now.AddDate(0, 0, -29))
	}
	a, e := time.Parse("2006-01-02", f.From)
	if e != nil {
		return errors.New("invalid_time")
	}
	b, e := time.Parse("2006-01-02", f.To)
	if e != nil || a.After(b) || b.Sub(a) > 366*24*time.Hour {
		return errors.New("invalid_time")
	}
	for _, v := range []string{f.Actor, f.Caller, f.Direction, f.Purpose} {
		if !safe(v) {
			return errors.New("invalid_filter")
		}
	}
	return nil
}

func boundedID(v string) bool {
	if len(v) > 1024 {
		return false
	}
	for _, r := range v {
		if r < 32 || r == 127 {
			return false
		}
	}
	return true
}
