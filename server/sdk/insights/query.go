package insights

import (
	"errors"
	"sort"
	"time"
)

type Alert struct {
	Code     string `json:"code"`
	Severity string `json:"severity"`
	Space    string `json:"space_id,omitempty"`
}
type Totals struct {
	Effective  int64 `json:"effective_bytes"`
	Observed   int64 `json:"observed_bytes"`
	Operations int64 `json:"completed_operations"`
	Failures   int64 `json:"failed_attempts"`
}

func (t *Totals) add(r Daily) {
	switch r.Metric {
	case "effective_bytes":
		t.Effective += r.Bytes
		t.Operations += r.Count
	case "observed_bytes":
		t.Observed += r.Bytes
	case "failed_attempt":
		t.Failures += r.Count
	}
}

type Summary struct {
	MetricVersion       string       `json:"metric_version"`
	Timezone            string       `json:"timezone"`
	Range               Filter       `json:"range"`
	Collection          Collection   `json:"collection"`
	Totals              Totals       `json:"totals"`
	Capacities          []Capacity   `json:"capacities"`
	Scans               []ScanRun    `json:"scans"`
	Policies            []ScanPolicy `json:"scan_policies"`
	Alerts              []Alert      `json:"alerts"`
	ObservedTotal       *int64       `json:"observed_total_bytes"`
	ObservedTotalStatus string       `json:"observed_total_status"`
	Coverage            []string     `json:"coverage"`
}

func (s *Store) Daily(scope Scope, f Filter) ([]Daily, error) {
	if e := f.Validate(s.Now()); e != nil {
		return nil, e
	}
	rows := []Daily{}
	if e := dimensions(s.DB, scope, f).Order("day ASC").Find(&rows).Error; e != nil {
		return nil, ErrUnavailable
	}
	return rows, nil
}
func (s *Store) Summary(scope Scope, f Filter) (Summary, error) {
	out := Summary{MetricVersion: MetricVersion, Timezone: "UTC", Capacities: []Capacity{}, Scans: []ScanRun{}, Policies: []ScanPolicy{}, Alerts: []Alert{}, ObservedTotalStatus: "not_collected", Coverage: []string{"application_observed_bytes_not_network_billing", "provider_internal_retries_may_be_unobserved", "sdk_body_rereads_may_include_signing", "observed_scans_exclude_provider_hidden_versions"}}
	if e := f.Validate(s.Now()); e != nil {
		return out, e
	}
	out.Range = f
	if s.DB.First(&out.Collection, 1).Error != nil {
		return out, ErrUnavailable
	}
	rows, e := s.Daily(scope, f)
	if e != nil {
		return out, e
	}
	for _, r := range rows {
		out.Totals.add(r)
	}
	caps := []Capacity{}
	// Capacity/scan snapshots are whole-space projections; a partial Agent scope
	// must never infer another folder's occupancy from them.
	if !scope.RestrictOperations && scope.Actor == "" {
		if apply(s.DB, scope).Order("at DESC").Find(&caps).Error != nil {
			return out, ErrUnavailable
		}
		seen := map[string]bool{}
		for _, c := range caps {
			if !seen[c.Space] {
				out.Capacities = append(out.Capacities, c)
				seen[c.Space] = true
			}
		}
		if apply(s.DB, scope).Order("started DESC").Limit(100).Find(&out.Scans).Error != nil {
			return out, ErrUnavailable
		}
		if apply(s.DB, scope).Find(&out.Policies).Error != nil {
			return out, ErrUnavailable
		}
	} else {
		out.Coverage = append(out.Coverage, "capacity_unavailable_for_partial_agent_scope")
	}
	if out.Collection.Gap || s.Degraded.Load() {
		out.Alerts = append(out.Alerts, Alert{Code: "collection_gap", Severity: "warning"})
	}
	if s.AuditDegraded.Load() || out.Collection.AuditUnavailable {
		out.Alerts = append(out.Alerts, Alert{Code: "audit_unavailable", Severity: "critical"})
	}
	for _, c := range out.Capacities {
		used := c.Current + c.Trash
		if c.History != nil {
			used += *c.History
		}
		if c.Quota != nil && *c.Quota > 0 {
			if float64(used) >= float64(*c.Quota)*.95 {
				out.Alerts = append(out.Alerts, Alert{"quota_critical", "critical", c.Space})
			} else if float64(used) >= float64(*c.Quota)*.8 {
				out.Alerts = append(out.Alerts, Alert{"quota_warning", "warning", c.Space})
			}
		}
	}
	completeCount := 0
	var total int64
	for _, p := range out.Policies {
		var latest, full ScanRun
		q := s.DB.Where("owner = ? AND space = ?", p.Owner, p.Space)
		q.Order("started DESC").First(&latest)
		if s.DB.Where("owner = ? AND space = ? AND status = ?", p.Owner, p.Space, "complete").Order("started DESC").First(&full).Error == nil {
			completeCount++
			total += full.Bytes
		}
		if latest.Error != "" {
			out.Alerts = append(out.Alerts, Alert{"scan_incomplete", "warning", p.Space})
		}
		if p.Enabled && (full.Finished == nil || s.Now().Sub(*full.Finished) > 48*time.Hour) {
			out.Alerts = append(out.Alerts, Alert{"scan_stale", "warning", p.Space})
		}
		if full.Finished != nil {
			var previous ScanRun
			day, _ := time.Parse("2006-01-02", Day(full.Started))
			if s.DB.Where("owner = ? AND space = ? AND status = ? AND started < ?", p.Owner, p.Space, "complete", day).Order("started DESC").First(&previous).Error == nil && full.Bytes-previous.Bytes > 1<<30 && float64(full.Bytes) > float64(previous.Bytes)*1.2 {
				out.Alerts = append(out.Alerts, Alert{"rapid_growth", "warning", p.Space})
			}
		}
	}
	// Different connection names do not prove distinct accounts or roots.
	// Return individual snapshots, never add potentially overlapping roots.
	if completeCount == 1 && len(out.Policies) == 1 {
		out.ObservedTotal = &total
		out.ObservedTotalStatus = "single_scope_last_complete"
	} else if completeCount > 0 {
		out.ObservedTotalStatus = "not_summed_overlap_or_partial_coverage"
	}
	return out, nil
}

type SeriesPoint struct {
	Day string `json:"day"`
	Totals
	Capacities []Capacity `json:"capacities"`
}

func (s *Store) Series(scope Scope, f Filter) ([]SeriesPoint, error) {
	rows, e := s.Daily(scope, f)
	if e != nil {
		return nil, e
	}
	if e = f.Validate(s.Now()); e != nil {
		return nil, e
	}
	points := map[string]*SeriesPoint{}
	ensure := func(day string) *SeriesPoint {
		if points[day] == nil {
			points[day] = &SeriesPoint{Day: day, Capacities: []Capacity{}}
		}
		return points[day]
	}
	for _, r := range rows {
		ensure(r.Day).Totals.add(r)
	}
	if !scope.RestrictOperations && scope.Actor == "" {
		caps := []Capacity{}
		if apply(s.DB, scope).Where("day >= ? AND day <= ?", f.From, f.To).Find(&caps).Error != nil {
			return nil, ErrUnavailable
		}
		for _, c := range caps {
			p := ensure(c.Day)
			p.Capacities = append(p.Capacities, c)
		}
	}
	out := []SeriesPoint{}
	for _, p := range points {
		out = append(out, *p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Day < out[j].Day })
	return out, nil
}

type Breakdown struct {
	Key  string `json:"key"`
	Kind string `json:"kind"`
	Totals
}

func (s *Store) Breakdown(scope Scope, f Filter, group string) ([]Breakdown, error) {
	rows, e := s.Daily(scope, f)
	if e != nil {
		return nil, e
	}
	groups := map[string]*Breakdown{}
	for _, r := range rows {
		var k, kind string
		switch group {
		case "space":
			k = r.Space
		case "connection":
			k = r.Connection
		case "actor":
			k = r.Actor
			kind = r.ActorKind
		case "caller":
			k = r.Caller
			kind = "declared_label"
		case "direction":
			k = r.Direction
		case "purpose":
			k = r.Purpose
		default:
			return nil, errors.New("invalid_group")
		}
		if k == "" {
			k = "unknown"
		}
		key := kind + ":" + k
		if groups[key] == nil {
			groups[key] = &Breakdown{Key: k, Kind: kind}
		}
		groups[key].add(r)
	}
	out := []Breakdown{}
	for _, g := range groups {
		out = append(out, *g)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Observed == out[j].Observed {
			return out[i].Key < out[j].Key
		}
		return out[i].Observed > out[j].Observed
	})
	return out, nil
}

type AuditFilter struct {
	Before                                           uint64
	Limit                                            int
	Action, Outcome, Resource, Correlation, From, To string
}
type AuditPage struct {
	Events   []AuditEvent `json:"events"`
	Next     uint64       `json:"next_cursor"`
	Coverage string       `json:"coverage"`
}

func (s *Store) Audit(scope Scope, f AuditFilter) (AuditPage, error) {
	out := AuditPage{Events: []AuditEvent{}, Coverage: "recorded_events_not_tamper_proof"}
	if f.Limit == 0 {
		f.Limit = 100
	}
	if f.Limit < 1 || f.Limit > 200 {
		return out, errors.New("invalid_limit")
	}
	dates := Filter{From: f.From, To: f.To}
	if e := dates.Validate(s.Now()); e != nil {
		return out, e
	}
	end, _ := time.Parse("2006-01-02", dates.To)
	start, _ := time.Parse("2006-01-02", dates.From)
	q := apply(s.DB, scope).Where("at >= ? AND at < ?", start, end.AddDate(0, 0, 1))
	if scope.Actor != "" {
		q = q.Where("actor = ?", scope.Actor)
	}
	if scope.RestrictOperations {
		q = q.Where("(operation IN ? OR resource IN ?)", scope.OperationIDs, scope.ResourceIDs)
	}
	for _, v := range []string{f.Action, f.Outcome, f.Resource, f.Correlation} {
		if !safe(v) {
			return out, errors.New("invalid_filter")
		}
	}
	if f.Action != "" {
		q = q.Where("action = ?", f.Action)
	}
	if f.Outcome != "" {
		q = q.Where("outcome = ?", f.Outcome)
	}
	if f.Resource != "" {
		q = q.Where("resource = ?", f.Resource)
	}
	if f.Correlation != "" {
		q = q.Where("correlation = ?", f.Correlation)
	}
	if f.Before > 0 {
		q = q.Where("id < ?", f.Before)
	}
	if q.Order("id DESC").Limit(f.Limit+1).Find(&out.Events).Error != nil {
		return out, ErrUnavailable
	}
	if len(out.Events) > f.Limit {
		out.Events = out.Events[:f.Limit]
		out.Next = out.Events[len(out.Events)-1].ID
	}
	return out, nil
}
func (s *Store) AuditSeries(scope Scope, f Filter, filters ...AuditFilter) ([]AuditDaily, error) {
	if e := f.Validate(s.Now()); e != nil {
		return nil, e
	}
	rows := []AuditDaily{}
	q := s.DB.Where("owner = ? AND day >= ? AND day <= ?", scope.Owner, f.From, f.To)
	if len(filters) > 0 {
		if filters[0].Action != "" {
			q = q.Where("action = ?", filters[0].Action)
		}
		if filters[0].Outcome != "" {
			q = q.Where("outcome = ?", filters[0].Outcome)
		}
	}
	if scope.Actor != "" {
		q = q.Where("actor = ?", scope.Actor)
	}
	// Aggregated security events cannot be safely narrowed by historical paths.
	if scope.Space != "" || scope.RestrictOperations {
		return rows, nil
	}
	if q.Order("day ASC").Find(&rows).Error != nil {
		return nil, ErrUnavailable
	}
	return rows, nil
}

type FactPage struct {
	Events []Fact `json:"events"`
	Next   uint64 `json:"next_cursor"`
}

func (s *Store) Facts(scope Scope, f Filter, before uint64) (FactPage, error) {
	out := FactPage{Events: []Fact{}}
	if e := f.Validate(s.Now()); e != nil {
		return out, e
	}
	start, _ := time.Parse("2006-01-02", f.From)
	end, _ := time.Parse("2006-01-02", f.To)
	q := apply(s.DB, scope).Where("at >= ? AND at < ?", start, end.AddDate(0, 0, 1))
	actor := f.Actor
	if scope.Actor != "" {
		actor = scope.Actor
	}
	if actor != "" {
		q = q.Where("actor = ?", actor)
	}
	if scope.RestrictOperations {
		q = q.Where("(operation IN ? OR resource IN ?)", scope.OperationIDs, scope.ResourceIDs)
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
	if before > 0 {
		q = q.Where("sequence < ?", before)
	}
	if q.Order("sequence DESC").Limit(201).Find(&out.Events).Error != nil {
		return out, ErrUnavailable
	}
	if len(out.Events) > 200 {
		out.Events = out.Events[:200]
		out.Next = out.Events[199].Sequence
	}
	return out, nil
}
