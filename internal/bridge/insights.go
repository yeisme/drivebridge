package bridge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"strings"
	"time"

	"github.com/rclone/rclone/fs"
	"github.com/yeisme/drivebridge-server/sdk/insights"
)

const localOwner = "local"

func refID(ref string) string { return strings.TrimPrefix(ref, "drivebridge://file/") }
func (s *Service) usageDims(op Operation) insights.Dimensions {
	connection := ""
	if p, e := s.Space(op.SpaceID); e == nil {
		connection = p.Remote
	}
	return insights.Dimensions{Owner: localOwner, Space: op.SpaceID, Connection: connection, Actor: "local-user", ActorKind: "local_user", Caller: op.Caller, Operation: op.ID, Resource: refID(op.FileRef)}
}
func (s *Service) observe(op Operation, direction, purpose string, n int64) {
	if s.Insights != nil {
		_ = s.Insights.Record(insights.Fact{Dimensions: s.usageDims(op), Metric: "observed_bytes", Direction: direction, Purpose: purpose, Bytes: n})
	}
}
func (s *Service) auditLocal(space, action, resource, outcome string) error {
	if s.Insights == nil {
		return nil
	}
	return s.Insights.Log(insights.AuditEvent{Owner: localOwner, Space: space, Actor: "local-user", ActorKind: "local_user", Action: action, Resource: refID(resource), Outcome: outcome, Entry: "local", Correlation: insights.ID()})
}
func (s *Service) CaptureUsage() {
	if s.Insights == nil {
		return
	}
	for _, p := range s.Spaces() {
		files := []File{}
		if s.DB.Where("space_id = ?", p.ID).Order("observed_at DESC, ref DESC").Find(&files).Error != nil {
			s.Insights.Gap()
			continue
		}
		cap := insights.Capacity{Owner: localOwner, Space: p.ID, Connection: p.Remote, Coverage: "observed_index_no_historical_snapshot"}
		seen := map[string]bool{}
		for _, f := range files {
			key := f.DriveID + "\x00" + f.ItemID
			if seen[key] {
				continue
			}
			seen[key] = true
			if f.Trashed != nil {
				cap.Trash += f.Size
			} else {
				cap.Current += f.Size
			}
		}
		_ = s.Insights.Capture(cap)
	}
}
func (s *Service) Usage(kind string, q url.Values) (any, error) {
	space := q.Get("space_id")
	if space != "" {
		if _, e := s.Space(space); e != nil {
			return nil, e
		}
	}
	return QueryInsights(s.Insights, kind, q)
}
func (s *Service) ConfigureScan(space string, enabled bool, maxEntries, maxSeconds int) (insights.ScanPolicy, error) {
	p, e := s.Space(space)
	if e != nil {
		return insights.ScanPolicy{}, e
	}
	if e = s.auditLocal(space, "scan.configure", "", "intent"); e != nil {
		return insights.ScanPolicy{}, e
	}
	return s.Insights.Configure(localOwner, p.ID, p.Remote, enabled, maxEntries, maxSeconds)
}

type inventoryProvider interface {
	Inventory(context.Context, Space, string) (insights.InventoryPage, error)
}

func (s *Service) Scan(ctx context.Context, space, resume string) (insights.ScanRun, error) {
	p, e := s.Space(space)
	if e != nil {
		return insights.ScanRun{}, e
	}
	provider, ok := s.providerFor(p).(inventoryProvider)
	if !ok {
		return insights.ScanRun{}, errors.New("inventory_unsupported")
	}
	run, e := s.Insights.Scan(ctx, localOwner, space, func(ctx context.Context, cursor string) (insights.InventoryPage, error) {
		return provider.Inventory(ctx, p, cursor)
	}, resume)
	if e == nil {
		_ = s.auditLocal(space, "scan.complete", run.ID, run.Status)
	}
	return run, e
}
func (s *Service) StartInsights() {
	if s.Insights == nil {
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
					s.CaptureUsage()
					lastCapture = time.Now()
				}
				policies, e := s.Insights.Policies()
				if e == nil {
					for _, p := range policies {
						if ctx.Err() != nil {
							return
						}
						if time.Since(p.LastAttempt) >= 24*time.Hour {
							_, _ = s.Scan(ctx, p.Space, "")
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

// Inventory uses the rclone remote's selected root, not the transfer listing
// (which intentionally hides oversized files). Directory checkpoints are not
// replayed across runs because rclone does not promise stable pagination.
func inventoryPage(ctx context.Context, f fs.Fs, cursor string) (insights.InventoryPage, error) {
	queue := []string{""}
	if cursor != "" {
		if len(cursor) > 1<<20 || json.Unmarshal([]byte(cursor), &queue) != nil || len(queue) == 0 {
			return insights.InventoryPage{}, errors.New("invalid_inventory")
		}
	}
	dir := queue[0]
	queue = queue[1:]
	if _, e := remoteDir(dir); e != nil {
		return insights.InventoryPage{}, e
	}
	entries, e := f.List(ctx, dir)
	if e != nil {
		return insights.InventoryPage{}, errors.New("inventory_unavailable")
	}
	page := insights.InventoryPage{Entries: int64(len(entries)), Resumable: false}
	for _, entry := range entries {
		if e = ctx.Err(); e != nil {
			return page, e
		}
		switch v := entry.(type) {
		case fs.Directory:
			sub := v.Remote()
			if _, e = remoteDir(sub); e != nil {
				return page, e
			}
			if sub == dir || dir != "" && !strings.HasPrefix(sub, dir+"/") {
				return page, errors.New("invalid_inventory")
			}
			queue = append(queue, sub)
		case fs.Object:
			if v.Size() < 0 {
				return page, errors.New("inventory_unavailable")
			}
			page.Bytes += v.Size()
		}
	}
	if len(queue) > 0 {
		b, e := json.Marshal(queue)
		if e != nil || len(b) > 1<<20 {
			return page, insights.ErrBudget
		}
		page.Next = string(b)
	}
	return page, nil
}
func (g *Generic) Inventory(ctx context.Context, p Space, cursor string) (insights.InventoryPage, error) {
	f, e := g.remote(p, ctx)
	if e != nil {
		return insights.InventoryPage{}, e
	}
	return inventoryPage(ctx, f, cursor)
}
func (r *OneDrive) Inventory(ctx context.Context, p Space, cursor string) (insights.InventoryPage, error) {
	r.mu.Lock()
	f, e := r.client(ctx, p)
	r.mu.Unlock()
	if e != nil {
		return insights.InventoryPage{}, e
	}
	return inventoryPage(ctx, f, cursor)
}
func (s *Service) observedBody(op Operation, body io.ReadCloser, purpose string) io.ReadCloser {
	return s.Insights.Observe(body, insights.Fact{Dimensions: s.usageDims(op), Metric: "observed_bytes", Direction: "storage_to_local", Purpose: purpose})
}
func privateIdentifier(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func QueryInsights(store *insights.Store, kind string, q url.Values) (any, error) {
	if kind != "files" {
		return store.Query(insights.Scope{Owner: localOwner, Space: q.Get("space_id"), Connection: q.Get("connection_id")}, kind, q)
	}
	category := q.Get("category")
	if category == "" {
		category = "current"
	}
	items := []map[string]any{}
	if category == "history" {
		return map[string]any{"items": items, "partial": false, "coverage": "local_history_not_materialized"}, nil
	}
	if category != "current" && category != "trash" {
		return nil, errors.New("invalid_category")
	}
	files := []File{}
	query := store.DB
	if space := q.Get("space_id"); space != "" {
		query = query.Where("space_id = ?", space)
	}
	if query.Order("observed_at DESC, ref DESC").Find(&files).Error != nil {
		return nil, insights.ErrUnavailable
	}
	seen := map[string]bool{}
	partial := false
	for _, f := range files {
		identity := f.SpaceID + "\x00" + f.ItemID
		if seen[identity] {
			continue
		}
		seen[identity] = true
		if category == "trash" && f.Trashed == nil || category == "current" && f.Trashed != nil {
			continue
		}
		if connection := q.Get("connection_id"); connection != "" {
			var p Space
			if store.DB.First(&p, "id = ?", f.SpaceID).Error != nil || p.Remote != connection {
				continue
			}
		}
		if name := q.Get("q"); name != "" && !strings.Contains(strings.ToLower(f.Name), strings.ToLower(name)) {
			continue
		}
		items = append(items, map[string]any{"resource_id": refID(f.Ref), "version_id": f.Version, "ref": f.Ref, "name": f.Name, "size_bytes": f.Size, "space_id": f.SpaceID, "category": category, "caller": f.Caller})
		if len(items) >= 200 {
			partial = true
			break
		}
	}
	return map[string]any{"items": items, "partial": partial, "coverage": "observed_index_verify_with_stat", "category": category}, nil
}

func (s *Service) beginFileAudit(space, action, resource string) (func(error), error) {
	if s.Insights == nil {
		return func(error) {}, nil
	}
	event := insights.AuditEvent{Owner: localOwner, Space: space, Actor: "local-user", ActorKind: "local_user", Action: action, Resource: refID(resource), ResourceKind: "file", Outcome: "intent", Entry: "local", Correlation: insights.ID()}
	if e := s.Insights.Log(event); e != nil {
		return nil, e
	}
	return func(e error) {
		event.Outcome = "success"
		if e != nil {
			event.Outcome = safeError(e)
		}
		_ = s.Insights.Log(event)
	}, nil
}
