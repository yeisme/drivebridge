package service

import (
	"context"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/yeisme/drivebridge-server/sdk/insights"
)

func (s *Service) insightScope(p Principal, space, connection string) (insights.Scope, error) {
	if p.ScopeSpace != "" {
		if space != "" && space != p.ScopeSpace {
			return insights.Scope{}, code("forbidden")
		}
		space = p.ScopeSpace
	}
	scope := insights.Scope{Owner: p.dataOwner(), Space: space, Connection: connection}
	if p.dataOwner() == "" {
		return scope, code("unauthorized")
	}
	if space != "" {
		if _, e := s.space(p, space); e != nil {
			return scope, e
		}
	}
	if p.Agent != nil {
		var current Agent
		if s.DB.First(&current, "id = ? AND owner = ?", p.Agent.ID, p.UserID).Error != nil || current.Revoked || !time.Now().Before(current.Expires) || !strings.Contains(","+current.Scopes+",", ",read,") {
			return scope, code("forbidden")
		}
		if space != "" && space != current.SpaceID {
			return scope, code("forbidden")
		}
		scope.Space, scope.Actor, scope.RestrictOperations = current.SpaceID, current.ID, true
		ops := []Operation{}
		if s.DB.Where("owner = ? AND space_id = ? AND agent_id = ?", p.dataOwner(), current.SpaceID, current.ID).Find(&ops).Error != nil {
			return scope, code("database_unavailable")
		}
		scope.OperationIDs = []string{}
		scope.ResourceIDs = []string{}
		files := []File{}
		s.DB.Where("owner = ? AND space_id = ?", p.dataOwner(), current.SpaceID).Find(&files)
		for _, file := range files {
			if contains(current.Prefix, file.Path) {
				scope.ResourceIDs = append(scope.ResourceIDs, file.ID)
			}
		}
		for _, op := range ops {
			if contains(current.Prefix, op.Path) {
				scope.OperationIDs = append(scope.OperationIDs, op.ID)
			}
		}
	}
	return scope, nil
}
func (s *Service) auditEvent(p Principal, action, resource, operation, outcome, entry, correlation string) insights.AuditEvent {
	actor, kind := p.UserID, "user"
	if p.Agent != nil {
		actor, kind = p.Agent.ID, "authenticated_agent"
	}
	if p.dataOwner() == "" {
		actor, kind = "", "anonymous"
	}
	// Resource IDs come only from validated owner records, never route strings.
	valid := func(v string) string {
		if len(v) != 48 {
			return ""
		}
		for _, r := range v {
			if !(r >= '0' && r <= '9') && !(r >= 'a' && r <= 'f') {
				return ""
			}
		}
		return v
	}
	event := insights.AuditEvent{Owner: p.dataOwner(), Actor: actor, ActorKind: kind, Action: action, Resource: valid(resource), Operation: valid(operation), Outcome: outcome, Entry: entry, Correlation: correlation}
	if event.Resource != "" && p.dataOwner() != "" {
		var f File
		var op Operation
		var agent Agent
		if s.DB.First(&f, "id = ? AND owner = ?", event.Resource, p.dataOwner()).Error == nil {
			event.Space = f.SpaceID
			event.Version = f.VersionID
			event.ResourceKind = "file"
		} else if s.DB.First(&op, "id = ? AND owner = ?", event.Resource, p.dataOwner()).Error == nil {
			event.Space = op.SpaceID
			event.Operation = op.ID
			event.Resource = op.FileID
			event.ResourceKind = "file"
		} else if s.DB.First(&agent, "id = ? AND owner = ?", event.Resource, p.dataOwner()).Error == nil {
			event.Space = agent.SpaceID
			event.ResourceKind = "agent"
		} else {
			event.ResourceKind = "unknown"
		}
	}
	return event
}
func (s *Service) requireAudit(p Principal, action, resource string) error {
	if s.Insights == nil {
		return nil
	}
	return s.Insights.Log(s.auditEvent(p, action, resource, "", "intent", "service", insights.ID()))
}
func auditCritical(action, method string) bool {
	if method == "GET" || method == "HEAD" {
		return false
	}
	switch action {
	case "agents.revoke", "connections.disconnect", "operations.cancel", "logout":
		return false
	}
	return strings.HasPrefix(action, "agents.") || strings.HasPrefix(action, "connections.") || strings.HasPrefix(action, "spaces.") || strings.HasPrefix(action, "files.") || action == "uploads.create"
}
func (s *Service) dimensions(p Principal, op Operation) insights.Dimensions {
	actor, kind := p.UserID, "user"
	if op.AgentID != "" {
		actor, kind = op.AgentID, "authenticated_agent"
	} else if p.Agent != nil {
		actor, kind = p.Agent.ID, "authenticated_agent"
	}
	return insights.Dimensions{Owner: p.dataOwner(), Space: op.SpaceID, Connection: op.ConnectionID, Actor: actor, ActorKind: kind, Caller: op.Caller, Operation: op.ID, Resource: op.FileID}
}
func (s *Service) observed(p Principal, op Operation, direction, purpose string, n int64) {
	if s.Insights != nil {
		_ = s.Insights.Record(insights.Fact{Dimensions: s.dimensions(p, op), Metric: "observed_bytes", Direction: direction, Purpose: purpose, Bytes: n})
	}
}
func (s *Service) CaptureUsage(owner string) {
	if s.Insights == nil {
		return
	}
	spaces := []Space{}
	q := s.DB
	if owner != "" {
		q = q.Where("owner = ?", owner)
	}
	if q.Find(&spaces).Error != nil {
		s.Insights.Gap()
		return
	}
	for _, sp := range spaces {
		var totals struct {
			Current int64
			History int64
			Trash int64
		}
		err := s.DB.Table("versions AS v").
			Select("COALESCE(SUM(CASE WHEN f.trashed = ? THEN v.size ELSE 0 END), 0) AS trash, COALESCE(SUM(CASE WHEN f.trashed = ? AND v.id = f.version_id THEN v.size ELSE 0 END), 0) AS current, COALESCE(SUM(CASE WHEN f.trashed = ? AND v.id <> f.version_id THEN v.size ELSE 0 END), 0) AS history", true, false, false).
			Joins("JOIN files AS f ON f.id = v.file_id AND f.owner = v.owner").
			Where("f.owner = ? AND f.space_id = ?", sp.Owner, sp.ID).
			Scan(&totals).Error
		if err != nil {
			s.Insights.Gap()
			continue
		}
		hist := totals.History
		quota := sp.MaxBytes
		cap := insights.Capacity{Owner: sp.Owner, Space: sp.ID, Connection: sp.ConnectionID, Current: totals.Current, Trash: totals.Trash, History: &hist, Reserved: sp.Reserved, Quota: &quota, Coverage: "managed_complete"}
		_ = s.Insights.Capture(cap)
	}
}
func (s *Service) startInsights() {
	if !s.Config.Enabled || s.Insights == nil {
		return
	}
	s.insightOnce.Do(func() {
		ctx, cancel := context.WithCancel(context.Background())
		s.insightCancel = cancel
		s.insightWG.Add(1)
		go func() {
			defer s.insightWG.Done()
			ticker := time.NewTicker(time.Minute)
			defer ticker.Stop()
			lastCapture := time.Time{}
			for {
				if lastCapture.IsZero() || time.Since(lastCapture) >= time.Hour {
					s.CaptureUsage("")
					lastCapture = time.Now()
				}
				if s.governanceActive() {
					s.governanceMu.Lock()
					_ = s.expireRequests()
					s.governanceMu.Unlock()
				}
				policies, e := s.Insights.Policies()
				if e == nil {
					for _, p := range policies {
						if ctx.Err() != nil {
							return
						}
						if time.Since(p.LastAttempt) >= 24*time.Hour {
							_, _ = s.scanSpace(ctx, s.personalOwnerPrincipal(p.Owner), p.Space, "")
						}
					}
				}
				var c insights.Collection
				if s.DB.First(&c, 1).Error == nil && time.Since(c.LastPruned) >= 24*time.Hour {
					_ = s.Insights.Prune()
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	})
}
func (s *Service) scanSpace(ctx context.Context, p Principal, space, resume string) (insights.ScanRun, error) {
	if p.Agent != nil {
		return insights.ScanRun{}, code("forbidden")
	}
	sp, e := s.space(p, space)
	if e != nil {
		return insights.ScanRun{}, e
	}
	store, e := s.storageForSpace(p.dataOwner(), sp.ID, sp.ConnectionID)
	if e != nil {
		return insights.ScanRun{}, e
	}
	scanner, ok := store.(interface {
		Inventory(context.Context, string, string) (insights.InventoryPage, error)
	})
	if !ok {
		return insights.ScanRun{}, code("inventory_unsupported")
	}
	// Scan every namespace that can hold this space's objects: the legacy
	// per-owner/space prefix plus the blob namespaces (plaintext blob keys are
	// instance-shared, so they attribute to the scanning owner; encrypted blobs
	// stay space-scoped). Namespaces chain through a synthetic cursor.
	prefixes := []string{s.Instance + "/" + p.dataOwner() + "/" + sp.ID + "/"}
	if sp.Encrypted {
		prefixes = append(prefixes, s.Instance+"/blobs/"+sp.ID+"/")
	} else {
		prefixes = append(prefixes, s.Instance+"/blobs/")
	}
	return s.Insights.Scan(ctx, p.dataOwner(), sp.ID, namespaceInventory(prefixes, scanner), resume)
}

// namespaceCursor joins a namespace index with a store continuation token.
// A boundary cursor ("ns:<i>") restarts namespace i from its first page; a
// chained cursor ("nsc:<i>:<token>") resumes namespace i mid-listing. Bare
// cursors belong to namespace 0.
const (
	nsBoundary  = "ns:"
	nsChained   = "nsc:"
	nsSeparator = ":"
)

func parseNamespaceCursor(cursor string) (int, string, error) {
	if rest, ok := strings.CutPrefix(cursor, nsBoundary); ok {
		idx, e := strconv.Atoi(rest)
		if e != nil || strings.Contains(rest, nsSeparator) {
			return 0, "", code("invalid_inventory")
		}
		return idx, "", nil
	}
	if rest, ok := strings.CutPrefix(cursor, nsChained); ok {
		i := strings.IndexByte(rest, ':')
		if i < 0 {
			return 0, "", code("invalid_inventory")
		}
		idx, e := strconv.Atoi(rest[:i])
		if e != nil {
			return 0, "", code("invalid_inventory")
		}
		return idx, rest[i+1:], nil
	}
	return 0, cursor, nil
}

type inventoryScanner interface {
	Inventory(context.Context, string, string) (insights.InventoryPage, error)
}

// namespaceInventory chains one Inventory callback across namespaces so a run
// resumes at the exact namespace and continuation token it stopped at.
func namespaceInventory(prefixes []string, scanner inventoryScanner) insights.Inventory {
	return func(ctx context.Context, cursor string) (insights.InventoryPage, error) {
		idx, storeCursor, e := parseNamespaceCursor(cursor)
		if e != nil {
			return insights.InventoryPage{}, e
		}
		if idx < 0 || idx >= len(prefixes) {
			return insights.InventoryPage{}, code("invalid_inventory")
		}
		page, e := scanner.Inventory(ctx, prefixes[idx], storeCursor)
		if e != nil {
			return page, e
		}
		if page.Next != "" {
			if idx > 0 {
				page.Next = nsChained + strconv.Itoa(idx) + nsSeparator + page.Next
			}
			return page, nil
		}
		if idx+1 < len(prefixes) {
			idx++
			page.Next = nsBoundary + strconv.Itoa(idx)
			page.Resumable = true
		}
		return page, nil
	}
}
func (s *Service) insightsRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/usage/files", s.wrap("usage.files", s.usageFiles))
	for _, kind := range []string{"summary", "series", "breakdown", "events", "scans", "export"} {
		kind := kind
		mux.HandleFunc("GET /api/v1/usage/"+kind, s.wrap("usage."+kind, func(w http.ResponseWriter, r *http.Request, p Principal) error {
			q := r.URL.Query()
			scope, e := s.insightScope(p, q.Get("space_id"), q.Get("connection_id"))
			if e != nil {
				return e
			}
			out, e := s.Insights.Query(scope, kind, q)
			if e == nil && kind == "export" {
				recorded := s.Insights.Log(s.auditEvent(p, "usage.export", "", "", "success", "http", insights.ID())) == nil
				out = map[string]any{"export": out, "audit_recorded": recorded}
			}
			if e != nil {
				return e
			}
			respond(w, out)
			return nil
		}))
	}
	mux.HandleFunc("POST /api/v1/usage/scans", s.wrap("usage.scan", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil {
			return code("forbidden")
		}
		var q struct {
			Space         string `json:"space_id"`
			Enabled       *bool  `json:"enabled"`
			MaxEntries    int    `json:"max_entries"`
			MaxSeconds    int    `json:"max_seconds"`
			Resume        string `json:"resume_id"`
			CancelID      string `json:"cancel_id"`
			ConfigureOnly bool   `json:"configure_only"`
		}
		if e := decode(r, &q); e != nil {
			return e
		}
		sp, e := s.space(p, q.Space)
		if e != nil {
			return e
		}
		if q.CancelID != "" {
			if e := s.Insights.CancelScan(p.dataOwner(), q.Space, q.CancelID); e != nil {
				return e
			}
			respond(w, map[string]bool{"cancel_requested": true})
			return nil
		}
		if q.Enabled != nil {
			policy, err := s.Insights.Configure(p.dataOwner(), sp.ID, sp.ConnectionID, *q.Enabled, q.MaxEntries, q.MaxSeconds)
			if err != nil {
				return err
			}
			if !*q.Enabled || q.ConfigureOnly {
				respond(w, policy)
				return nil
			}
		}
		run, e := s.scanSpace(r.Context(), p, q.Space, q.Resume)
		if e != nil {
			return e
		}
		respond(w, run)
		return nil
	}))
	for _, kind := range []string{"events", "export", "series"} {
		kind := kind
		mux.HandleFunc("GET /api/v1/audit/"+kind, s.wrap("audit."+kind, func(w http.ResponseWriter, r *http.Request, p Principal) error {
			q := r.URL.Query()
			scope, e := s.insightScope(p, q.Get("space_id"), "")
			if e != nil {
				return e
			}
			out, e := s.Insights.Query(scope, "audit_"+kind, q)
			if e == nil && kind == "export" {
				recorded := s.Insights.Log(s.auditEvent(p, "audit.export", "", "", "success", "http", insights.ID())) == nil
				out = map[string]any{"export": out, "audit_recorded": recorded}
			}
			if e != nil {
				return e
			}
			respond(w, out)
			return nil
		}))
	}
}
func defaultString(v, fallback string) string {
	if v == "" {
		return fallback
	}
	return v
}

type usageResponse struct {
	http.ResponseWriter
	Bytes int64
}

func (w *usageResponse) Write(p []byte) (int, error) {
	n, e := w.ResponseWriter.Write(p)
	w.Bytes += int64(n)
	return n, e
}
func (s *Service) sendContent(w http.ResponseWriter, p Principal, f File, v Version, body io.ReadCloser) error {
	meter := &usageResponse{ResponseWriter: w}
	err := serveContent(meter, f, v, body)
	op := Operation{SpaceID: f.SpaceID, ConnectionID: v.ConnectionID, FileID: f.ID}
	s.observed(p, op, "server_to_client", "download", meter.Bytes)
	if err == nil && s.Insights != nil {
		_ = s.Insights.Record(insights.Fact{Dimensions: s.dimensions(p, op), Metric: "effective_bytes", Direction: "egress", Purpose: "download", Bytes: v.Size})
	}
	return err
}

func (s *Service) personalOwnerPrincipal(owner string) Principal {
	p := Principal{UserID: owner}
	if s.governanceActive() {
		var m Membership
		if s.DB.First(&m, "data_owner = ? AND status = ?", owner, "active").Error == nil {
			p.UserID, p.DataOwner, p.TenantID = m.UserID, owner, m.TenantID
		}
	}
	return p
}
