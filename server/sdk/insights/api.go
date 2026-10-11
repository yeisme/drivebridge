package insights

import (
	"errors"
	"net/url"
	"strconv"
	"time"
)

// Query is shared by CLI, local loopback view, and authenticated HTTP adapters.
// Scope is supplied by the host authorization layer, never parsed here.
func (s *Store) Query(scope Scope, kind string, q url.Values) (any, error) {
	f := Filter{From: q.Get("from"), To: q.Get("to"), Actor: q.Get("actor"), Caller: q.Get("caller"), Direction: q.Get("direction"), Purpose: q.Get("purpose")}
	before := uint64(0)
	if raw := q.Get("before"); raw != "" {
		v, e := strconv.ParseUint(raw, 10, 64)
		if e != nil {
			return nil, errors.New("invalid_cursor")
		}
		before = v
	}
	switch kind {
	case "summary":
		return s.Summary(scope, f)
	case "series":
		return s.Series(scope, f)
	case "breakdown":
		group := q.Get("group")
		if group == "" {
			group = "space"
		}
		return s.Breakdown(scope, f, group)
	case "events":
		return s.Facts(scope, f, before)
	case "scans":
		return s.Scans(scope)
	case "export":
		summary, e := s.Summary(scope, f)
		if e != nil {
			return nil, e
		}
		series, e := s.Series(scope, f)
		if e != nil {
			return nil, e
		}
		return map[string]any{"metric_version": MetricVersion, "summary": summary, "series": series, "exported_at": time.Now().UTC()}, nil
	case "audit_series":
		if scope.Actor == "" {
			scope.Actor = f.Actor
		}
		return s.AuditSeries(scope, f, AuditFilter{Action: q.Get("action"), Outcome: q.Get("outcome")})
	case "audit_events", "audit_export":
		if scope.Actor == "" {
			scope.Actor = f.Actor
		}
		limit := 100
		if raw := q.Get("limit"); raw != "" {
			v, e := strconv.Atoi(raw)
			if e != nil {
				return nil, errors.New("invalid_limit")
			}
			limit = v
		}
		return s.Audit(scope, AuditFilter{Before: before, Limit: limit, Action: q.Get("action"), Outcome: q.Get("outcome"), Resource: q.Get("resource"), Correlation: q.Get("correlation"), From: f.From, To: f.To})
	}
	return nil, errors.New("unknown_insights_query")
}
