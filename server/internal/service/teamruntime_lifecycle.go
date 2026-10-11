package service

import (
	"context"
	"time"

	"gorm.io/gorm"
)

// migrateTeamRuntime adds the team-runtime tables additively: old columns and
// IDs never change, and rollback only drops these new tables.
func (s *Service) migrateTeamRuntime() error {
	return s.DB.AutoMigrate(&RuntimeJob{}, &OutboxEvent{}, &WorkerLease{}, &ProjectionCheckpoint{}, &ProjectionGeneration{}, &ProjectionRow{}, &RuntimeSequence{}, &GenerationScopeProgress{})
}

// reconcileTeamRuntime recovers state left by a stopped process. Claims whose
// leases expired return to pending; rebuild operations stay resumable through
// their original operation id. Reconciliation never fabricates completion.
func (s *Service) reconcileTeamRuntime() error {
	now := time.Now().UTC()
	if e := s.DB.Transaction(func(tx *gorm.DB) error {
		// Release only expired claims: a live lease still owns its job.
		var stale []WorkerLease
		if e := tx.Where("expires_at <= ?", now).Find(&stale).Error; e != nil {
			return e
		}
		for _, lease := range stale {
			// Expired leases do not prove the old worker stopped; the job
			// becomes claimable again and the next claim's fencing token
			// invalidates any late write from the previous owner.
			if e := tx.Model(&RuntimeJob{}).Where("id = ? AND status = ?", lease.ID, "claimed").Updates(map[string]any{"status": "pending", "updated_at": now}).Error; e != nil {
				return e
			}
			if e := tx.Delete(&WorkerLease{}, "id = ?", lease.ID).Error; e != nil {
				return e
			}
		}
		// projection_apply jobs are per-event work accounting, not history:
		// done rows are merged away once they are old enough to be useless
		// for diagnostics, so the queue table stays bounded under sustained
		// writes.
		cutoff := now.Add(-time.Hour)
		if e := tx.Where("kind = ? AND status = ? AND updated_at < ?", "projection_apply", "done", cutoff).Delete(&RuntimeJob{}).Error; e != nil {
			return e
		}
		return nil
	}); e != nil {
		return code("database_unavailable")
	}
	return nil
}

// startRuntimeWorker runs the in-process projection worker. It claims jobs,
// applies the outbox for the claimed owner scope, and completes the job with
// the lease's fencing token. A failed apply leaves the job claimable again —
// the business data itself never depends on the projection succeeding.
func (s *Service) startRuntimeWorker() {
	ctx, cancel := context.WithCancel(context.Background())
	s.runtimeWG.Add(1)
	s.runtimeCancel = cancel
	go func() {
		defer s.runtimeWG.Done()
		ticker := time.NewTicker(700 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				for {
					job, token, e := s.ClaimRuntimeJob("in-process", time.Minute)
					if e != nil || job == nil {
						break
					}
					_, _, applyErr := s.ApplyOutboxEvents(fileCatalogueProjection, job.OwnerScope, 500)
					if applyErr != nil {
						// Leave the outcome as claimed: the lease expires and
						// reconciliation re-queues it; events stay recoverable.
						continue
					}
					if e := s.CompleteRuntimeJob("in-process", token, job.ID, "done"); e != nil {
						continue
					}
				}
				// Self-driven catch-up: apply any owner scope with unpublished
				// events even when no job is queued (batch publishers, replayed
				// fixtures). The outbox stays the truth; jobs only add work
				// accounting.
				var scopes []string
				if e := s.DB.Model(&OutboxEvent{}).Where("published = ? AND kind = ?", false, "file_catalogue.upsert").Distinct().Pluck("owner_scope", &scopes).Error; e != nil {
					continue
				}
				for _, scope := range scopes {
					for {
						applied, _, e := s.ApplyOutboxEvents(fileCatalogueProjection, scope, 500)
						if e != nil || applied == 0 {
							break
						}
					}
				}
			}
		}
	}()
}
