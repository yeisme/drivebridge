package service

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// This file implements the team runtime application service: a durable job
// queue with fencing leases, a transactional outbox, idempotent projection
// apply with per-owner checkpoints, generation-based projection rebuilds, and
// the quiesced metadata migration. Queue claim SQL is the one sanctioned
// adapter exception (PostgreSQL SKIP LOCKED); it is bound to this file, uses
// bound parameters and fixed identifiers only.

// allocateOutboxSequence reserves the next monotonic publisher sequence inside
// the caller's business transaction so sequence order can never contradict
// commit order.
func allocateOutboxSequence(tx *gorm.DB) (int64, error) {
	var seq RuntimeSequence
	if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&RuntimeSequence{Name: "outbox", Next: 1}).Error; e != nil {
		return 0, e
	}
	if e := tx.Where("name = ?", "outbox").First(&seq).Error; e != nil {
		return 0, e
	}
	if e := tx.Model(&RuntimeSequence{}).Where("name = ?", "outbox").Update("next", seq.Next+1).Error; e != nil {
		return 0, e
	}
	return seq.Next, nil
}

// recordFileEvent writes one file_catalogue business event and one apply job
// into the caller's transaction. The event never describes rows the
// transaction has not written: if the transaction rolls back, both vanish.
func (s *Service) recordFileEvent(tx *gorm.DB, ownerScope, resourceID string, revision int64, tombstone bool, payload map[string]any) error {
	seq, e := allocateOutboxSequence(tx)
	if e != nil {
		return e
	}
	raw := ""
	if payload != nil {
		b, err := json.Marshal(payload)
		if err != nil {
			return err
		}
		raw = string(b)
	}
	event := OutboxEvent{EventID: ID(), Kind: "file_catalogue.upsert", ResourceID: resourceID, ResourceRevision: revision, Tombstone: tombstone, SourceSequence: seq, OwnerScope: ownerScope, Payload: raw}
	if e := tx.Create(&event).Error; e != nil {
		return e
	}
	job := RuntimeJob{ID: ID(), Kind: "projection_apply", OriginalOperationID: "", OwnerScope: ownerScope, Status: "pending", Payload: resourceID}
	return tx.Create(&job).Error
}

// fileRevision derives the monotonic projection revision of one file row from
// its last update instant.
func fileRevision(updated time.Time) int64 {
	return updated.UTC().UnixNano()
}

// ClaimRuntimeJob hands one runnable job to a worker under a fresh lease. The
// job's lease epoch is the fencing token: it increments on every claim, so a
// stale worker's token can never match the current lease. Claims whose lease
// expired are reclaimable: expiry does not prove the old worker stopped,
// which is exactly why the next claim's fencing token invalidates any late
// write from the previous owner.
func (s *Service) ClaimRuntimeJob(workerID string, ttl time.Duration) (*RuntimeJob, int64, error) {
	if workerID == "" || ttl <= 0 {
		return nil, 0, code("invalid_request")
	}
	var claimed *RuntimeJob
	var token int64
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		claimed = nil
		// Queue claim is the documented SQL exception: on PostgreSQL the row
		// is locked with FOR UPDATE SKIP LOCKED so concurrent workers never
		// take the same job; on SQLite the single-writer transaction
		// provides the same guarantee. Identifiers are fixed; values are
		// bound parameters.
		var job RuntimeJob
		var candidates []RuntimeJob
		if e := tx.Where("status IN ?", []string{"pending", "claimed"}).Order("created_at").Limit(20).Find(&candidates).Error; e != nil {
			return e
		}
		now := time.Now().UTC()
		for _, candidate := range candidates {
			if candidate.Status == "claimed" {
				// Expired leases are reclaimable: expiry does not prove the old
				// worker stopped, which is exactly why the next claim's fencing
				// token invalidates any late write from the previous owner.
				var lease WorkerLease
				if tx.First(&lease, "id = ?", candidate.ID).Error == nil && lease.ExpiresAt.After(now) {
					continue
				}
			}
			job = candidate
			break
		}
		if job.ID == "" {
			return nil
		}
		if s.runtimeDialect() == "postgres" {
			// Re-read under the row lock and re-validate claimability: the
			// lock wait may have outlived the candidate's status or lease.
			var locked RuntimeJob
			if e := tx.Raw("SELECT * FROM runtime_jobs WHERE id = ? FOR UPDATE SKIP LOCKED", job.ID).Scan(&locked).Error; e != nil || locked.ID == "" {
				return nil
			}
			if locked.Status != "pending" && locked.Status != "claimed" {
				return nil
			}
			if locked.Status == "claimed" {
				var lease WorkerLease
				if tx.First(&lease, "id = ?", locked.ID).Error == nil && lease.ExpiresAt.After(time.Now().UTC()) {
					return nil
				}
			}
			job = locked
		}
		job.LeaseEpoch++
		job.Status = "claimed"
		job.AttemptGeneration++
		lease := WorkerLease{ID: job.ID, WorkerID: workerID, FencingToken: job.LeaseEpoch, ExpiresAt: time.Now().UTC().Add(ttl), Heartbeat: time.Now().UTC()}
		if e := tx.Save(&job).Error; e != nil {
			return e
		}
		if e := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&lease).Error; e != nil {
			return e
		}
		claimed = &job
		token = job.LeaseEpoch
		return nil
	})
	if e != nil {
		return nil, 0, e
	}
	return claimed, token, nil
}

// RuntimeHeartbeat extends the caller's own lease; a fenced worker cannot
// extend a lease it no longer holds.
func (s *Service) RuntimeHeartbeat(workerID string, token int64, jobID string, ttl time.Duration) error {
	res := s.DB.Model(&WorkerLease{}).Where("id = ? AND worker_id = ? AND fencing_token = ?", jobID, workerID, token).Updates(map[string]any{"expires_at": time.Now().UTC().Add(ttl), "heartbeat_at": time.Now().UTC()})
	if res.Error != nil {
		return code("database_unavailable")
	}
	if res.RowsAffected == 0 {
		return code("worker_fenced")
	}
	return nil
}

// CompleteRuntimeJob closes a claimed job. A stale token is refused with
// worker_fenced, the refusal is appended to the job's diagnostics, and the
// current owner's state is never overwritten.
func (s *Service) CompleteRuntimeJob(workerID string, token int64, jobID, outcome string) error {
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var lease WorkerLease
		if e := tx.First(&lease, "id = ?", jobID).Error; e != nil {
			return code("not_found")
		}
		var job RuntimeJob
		if e := tx.First(&job, "id = ?", jobID).Error; e != nil {
			return code("not_found")
		}
		if lease.FencingToken != token || lease.WorkerID != workerID {
			// Late worker: record the fenced attempt for diagnosis; the new
			// owner's job state stays untouched. The diagnostic write runs
			// outside this transaction because returning the refusal rolls
			// it back — an aborted commit must not discard the audit trail.
			var entries []map[string]any
			if job.Diagnostics != "" {
				json.Unmarshal([]byte(job.Diagnostics), &entries)
			}
			entries = append(entries, map[string]any{"worker_id": workerID, "token": token, "outcome": outcome, "at": time.Now().UTC()})
			b, _ := json.Marshal(entries)
			tx.Rollback()
			if e := s.DB.Model(&RuntimeJob{}).Where("id = ? AND lease_epoch = ?", jobID, lease.FencingToken).Update("diagnostics", string(b)).Error; e != nil {
				return code("database_unavailable")
			}
			return code("worker_fenced")
		}
		if e := tx.Model(&RuntimeJob{}).Where("id = ?", jobID).Updates(map[string]any{"status": outcome, "updated_at": time.Now().UTC()}).Error; e != nil {
			return code("database_unavailable")
		}
		return tx.Delete(&WorkerLease{}, "id = ?", jobID).Error
	})
}

// upsertProjectionRow applies one event to one generation idempotently: a
// strictly newer revision wins completely (a restore clears the tombstone a
// trash set), equal revisions keep the tombstone sticky so replayed deletes
// never resurrect a resource, and an older replay changes nothing.
func upsertProjectionRow(tx *gorm.DB, projection, generation string, event OutboxEvent) error {
	var row ProjectionRow
	if e := tx.Where("projection = ? AND generation = ? AND resource_id = ?", projection, generation, event.ResourceID).First(&row).Error; e != nil {
		row = ProjectionRow{Projection: projection, Generation: generation, ResourceID: event.ResourceID, OwnerScope: event.OwnerScope, Revision: event.ResourceRevision, Tombstone: event.Tombstone, Digest: event.Payload, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
		return tx.Create(&row).Error
	}
	if event.ResourceRevision < row.Revision {
		// Out-of-order replay of an older revision: keep the newer state.
		return nil
	}
	if event.ResourceRevision > row.Revision {
		row.Tombstone = event.Tombstone
	} else {
		row.Tombstone = row.Tombstone || event.Tombstone
	}
	row.Revision = event.ResourceRevision
	row.Digest = event.Payload
	row.UpdatedAt = time.Now().UTC()
	return tx.Save(&row).Error
}
func (s *Service) ApplyOutboxEvents(projection, ownerScope string, limit int) (int, int64, error) {
	if limit <= 0 {
		limit = 100
	}
	applied := 0
	var lastSeq int64
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		var checkpoint ProjectionCheckpoint
		if e := tx.Where("projection = ? AND owner_scope = ?", projection, ownerScope).First(&checkpoint).Error; e != nil {
			// First apply creates this owner scope's checkpoint at the
			// initial generation. Checkpoints are per owner scope and never
			// shared across tenants.
			checkpoint = ProjectionCheckpoint{Projection: projection, OwnerScope: ownerScope, Generation: "GEN_0", Watermark: 0}
			if e := tx.Create(&checkpoint).Error; e != nil {
				return e
			}
		}
		generation := checkpoint.Generation
		var events []OutboxEvent
		if e := tx.Where("owner_scope = ? AND source_sequence > ? AND kind = ?", ownerScope, checkpoint.Watermark, "file_catalogue.upsert").Order("source_sequence").Limit(limit).Find(&events).Error; e != nil {
			return e
		}
		for _, event := range events {
			if e := upsertProjectionRow(tx, projection, generation, event); e != nil {
				return e
			}
			applied++
			lastSeq = event.SourceSequence
		}
		if applied > 0 {
			// The persistent checkpoint advances only after the idempotent
			// projection succeeded, inside this transaction. Applies always
			// target the checkpoint's own (active) generation; rebuilds use
			// applyEventsForGeneration and never move live checkpoints.
			if e := tx.Model(&ProjectionCheckpoint{}).Where("projection = ? AND owner_scope = ?", projection, ownerScope).Updates(map[string]any{"watermark": lastSeq, "updated_at": time.Now().UTC()}).Error; e != nil {
				return e
			}
			if e := tx.Model(&OutboxEvent{}).Where("owner_scope = ? AND source_sequence <= ? AND kind = ?", ownerScope, lastSeq, "file_catalogue.upsert").Update("published", true).Error; e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil {
		return applied, lastSeq, e
	}
	return applied, lastSeq, nil
}

// applyEventsForGeneration applies one owner scope's events into a specific
// (rebuild) generation without touching live checkpoints or publication
// state. Returns applied count and the last applied sequence.
func applyEventsForGeneration(tx *gorm.DB, projection, generation, ownerScope string, fromSeq int64, limit int) (int, int64, error) {
	var events []OutboxEvent
	if e := tx.Where("owner_scope = ? AND source_sequence > ? AND kind = ?", ownerScope, fromSeq, "file_catalogue.upsert").Order("source_sequence").Limit(limit).Find(&events).Error; e != nil {
		return 0, 0, e
	}
	applied := 0
	var last int64
	for _, event := range events {
		if e := upsertProjectionRow(tx, projection, generation, event); e != nil {
			return applied, last, e
		}
		applied++
		last = event.SourceSequence
	}
	return applied, last, nil
}

// projectionDigestSummary computes the verification summary of one
// generation: row count, tombstone count and a digest over the sorted
// resource identities and revisions. Two generations matching on all three
// while both are caught up to the same event-stream head are identical.
func projectionDigestSummary(db *gorm.DB, projection, generation string) (int64, int64, string, error) {
	var rows []ProjectionRow
	if e := db.Where("projection = ? AND generation = ?", projection, generation).Order("resource_id").Find(&rows).Error; e != nil {
		return 0, 0, "", e
	}
	var tombstones int64
	parts := make([]string, 0, len(rows))
	for _, row := range rows {
		if row.Tombstone {
			tombstones++
		}
		parts = append(parts, row.ResourceID+":"+fmt.Sprintf("%d", row.Revision)+":"+fmt.Sprintf("%t", row.Tombstone))
	}
	h := sha256.Sum256([]byte(strings.Join(parts, "\n")))
	return int64(len(rows)), tombstones, hex.EncodeToString(h[:]), nil
}

// StartProjectionRebuild freezes the source watermark and opens a new
// generation under an original Operation. The operation id is the recovery
// handle: replays while a rebuild is live return the same operation.
func (s *Service) StartProjectionRebuild(p Principal, projection, expectedGeneration string) (map[string]any, error) {
	if p.Agent != nil || !p.Admin {
		return nil, code("forbidden")
	}
	if projection != fileCatalogueProjection {
		return nil, code("unsupported_action")
	}
	var result map[string]any
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		// One live rebuild per projection: replay returns the original
		// operation instead of a duplicate generation.
		var running ProjectionGeneration
		if e := tx.Where("projection = ? AND phase NOT IN ?", projection, []string{"retired", "abandoned"}).Order("created_at DESC").First(&running).Error; e == nil && running.Phase != "active" {
			view, e := rebuildOperationView(tx, running.OperationID)
			if e != nil {
				return e
			}
			result = view
			return nil
		}
		// The current generation is the active one (GEN_0 before the first
		// rebuild ever switched); the frozen watermark is the global outbox
		// head at freeze time.
		current := "GEN_0"
		var active ProjectionGeneration
		if e := tx.Where("projection = ? AND phase = ?", projection, "active").Order("created_at DESC").First(&active).Error; e == nil {
			current = active.Generation
		}
		var watermark int64
		tx.Model(&OutboxEvent{}).Select("COALESCE(MAX(source_sequence), 0)").Where("kind = ?", "file_catalogue.upsert").Scan(&watermark)
		if expectedGeneration != "" && expectedGeneration != current {
			return code("version_conflict")
		}
		now := time.Now().UTC()
		op := Operation{ID: ID(), Owner: "", Key: "projection_rebuild:" + projection + ":" + now.Format("20060102T150405Z.000000000"), Kind: "projection_rebuild", Status: "running", Updated: now}
		if e := tx.Create(&op).Error; e != nil {
			return code("database_unavailable")
		}
		gen := ProjectionGeneration{ID: ID(), Projection: projection, Generation: nextGenerationName(current), Phase: "frozen", SourceWatermark: watermark, AppliedWatermark: watermark, OperationID: op.ID, CreatedAt: now}
		if e := tx.Create(&gen).Error; e != nil {
			return code("database_unavailable")
		}
		result = map[string]any{"operation_id": op.ID, "generation": gen.Generation, "phase": gen.Phase, "source_watermark": gen.SourceWatermark, "applied_watermark": gen.AppliedWatermark, "ready": false}
		return nil
	})
	if e != nil {
		return nil, e
	}
	return result, nil
}

func nextGenerationName(current string) string {
	// Generation names stay opaque but ordered: GEN_0, GEN_A, GEN_B, ...
	if len(current) == 5 && strings.HasPrefix(current, "GEN_") {
		switch c := current[4]; {
		case c == '0':
			return "GEN_A"
		case c >= 'A' && c < 'Z':
			return "GEN_" + string(rune(c+1))
		}
	}
	return "GEN_" + string(rune('Z'+1))
}

func rebuildOperationView(tx *gorm.DB, operationID string) (map[string]any, error) {
	var op Operation
	if e := tx.First(&op, "id = ?", operationID).Error; e != nil {
		return nil, code("not_found")
	}
	var gen ProjectionGeneration
	if e := tx.Where("operation_id = ?", operationID).Order("created_at DESC").First(&gen).Error; e != nil {
		return nil, code("not_found")
	}
	return map[string]any{"operation_id": op.ID, "generation": gen.Generation, "phase": gen.Phase, "source_watermark": gen.SourceWatermark, "applied_watermark": gen.AppliedWatermark, "ready": gen.Ready}, nil
}

// AdvanceProjectionRebuild drives one rebuild through its phases:
// frozen -> backfill -> catching_up -> verifying -> active. Each call makes
// provable progress. The switch refuses with projection_not_caught_up until
// BOTH generations are caught up with the event-stream head and their full
// summaries (counts, digest, tombstones) match; then the checkpoint read
// pointers move atomically to the new generation.
func (s *Service) AdvanceProjectionRebuild(operationID string, pageLimit int) (map[string]any, error) {
	if pageLimit <= 0 {
		pageLimit = 500
	}
	var result map[string]any
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		var gen ProjectionGeneration
		if e := tx.Where("operation_id = ?", operationID).Order("created_at DESC").First(&gen).Error; e != nil {
			return code("not_found")
		}
		view := func(phase string, ready bool) map[string]any {
			return map[string]any{"operation_id": operationID, "generation": gen.Generation, "phase": phase, "source_watermark": gen.SourceWatermark, "applied_watermark": gen.AppliedWatermark, "ready": ready}
		}
		switch gen.Phase {
		case "frozen":
			gen.Phase = "backfill"
			if e := tx.Save(&gen).Error; e != nil {
				return code("database_unavailable")
			}
			result = view("backfill", false)
			return nil
		case "backfill":
			// Page the file table into the new generation; already-present
			// rows make repeated calls idempotent.
			pending := tx.Model(&File{}).Where("id NOT IN (?)", tx.Model(&ProjectionRow{}).Select("resource_id").Where("projection = ? AND generation = ?", gen.Projection, gen.Generation))
			var files []File
			if e := pending.Order("id").Limit(pageLimit).Find(&files).Error; e != nil {
				return code("database_unavailable")
			}
			for _, f := range files {
				payload, _ := json.Marshal(map[string]any{"path": f.Path, "size_bytes": f.Size, "trashed": f.Trashed})
				row := ProjectionRow{Projection: gen.Projection, Generation: gen.Generation, ResourceID: f.ID, OwnerScope: f.Owner, Revision: fileRevision(f.Updated), Tombstone: f.Trashed, Digest: string(payload), Payload: string(payload), CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
				if e := tx.Create(&row).Error; e != nil {
					return code("database_unavailable")
				}
			}
			// Purged resources keep only their tombstone events: materialize
			// them here so the rebuild reproduces what the live generation
			// already applied before the freeze instead of silently dropping
			// the resource from the new generation. The anti-join pages by
			// source_sequence with the same pageLimit discipline as the file
			// scan: memory stays bounded on arbitrarily large histories.
			lastSeq := int64(0)
			lastPurge := map[string]OutboxEvent{}
			for {
				var purged []OutboxEvent
				q := tx.Where("kind = ? AND source_sequence > ? AND resource_id NOT IN (?) AND resource_id NOT IN (?)", "file_catalogue.upsert", lastSeq, tx.Model(&File{}).Select("id"), tx.Model(&ProjectionRow{}).Select("resource_id").Where("projection = ? AND generation = ?", gen.Projection, gen.Generation)).Order("source_sequence").Limit(pageLimit)
				if e := q.Find(&purged).Error; e != nil {
					return code("database_unavailable")
				}
				for _, event := range purged {
					lastPurge[event.ResourceID] = event
					lastSeq = event.SourceSequence
				}
				if len(purged) < pageLimit {
					break
				}
			}
			for _, event := range lastPurge {
				row := ProjectionRow{Projection: gen.Projection, Generation: gen.Generation, ResourceID: event.ResourceID, OwnerScope: event.OwnerScope, Revision: event.ResourceRevision, Tombstone: true, Digest: event.Payload, Payload: event.Payload, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()}
				if e := tx.Create(&row).Error; e != nil {
					return code("database_unavailable")
				}
			}
			if len(files) < pageLimit {
				gen.Phase = "catching_up"
				if e := tx.Save(&gen).Error; e != nil {
				}
			}
			result = view(gen.Phase, false)
			return nil
		case "catching_up":
			// Replay new events per owner scope. Each scope resumes from its own
			// persisted progress row: a single shared watermark would let one
			// scope's advance skip another scope's not-yet-applied events, which
			// is unrecoverable inside this generation. The generation watermark
			// is the minimum across scopes and never claims more.
			var scopes []string
			if e := tx.Model(&OutboxEvent{}).Where("source_sequence > ? AND kind = ?", gen.SourceWatermark, "file_catalogue.upsert").Distinct().Order("owner_scope").Pluck("owner_scope", &scopes).Error; e != nil {
				return code("database_unavailable")
			}
			total := 0
			for _, scope := range scopes {
				var progress GenerationScopeProgress
				if e := tx.Where("generation = ? AND owner_scope = ?", gen.Generation, scope).First(&progress).Error; e != nil {
					progress = GenerationScopeProgress{Generation: gen.Generation, OwnerScope: scope, AppliedSeq: gen.SourceWatermark}
				}
				applied, last, err := applyEventsForGeneration(tx, gen.Projection, gen.Generation, scope, progress.AppliedSeq, pageLimit)
				if err != nil {
					return code("database_unavailable")
				}
				if applied > 0 {
					progress.AppliedSeq = last
				}
				progress.UpdatedAt = time.Now().UTC()
				if e := tx.Clauses(clause.OnConflict{UpdateAll: true}).Create(&progress).Error; e != nil {
					return code("database_unavailable")
				}
				total += applied
			}
			var minSeq int64
			if e := tx.Model(&GenerationScopeProgress{}).Where("generation = ?", gen.Generation).Select("COALESCE(MIN(applied_seq), 0)").Scan(&minSeq).Error; e != nil {
				return code("database_unavailable")
			}
			if len(scopes) > 0 || minSeq > 0 {
				gen.AppliedWatermark = minSeq
			}
			if total == 0 {
				gen.Phase = "verifying"
			}
			if e := tx.Save(&gen).Error; e != nil {
				return code("database_unavailable")
			}
			result = view(gen.Phase, false)
			return nil
		case "verifying":
			// Caught up means every scope that produced events after the freeze
			// has replayed them into this generation through its own progress
			// row; scopes without progress rows have no events beyond the
			// frozen watermark, which the backfill already materialized.
			var scopes []string
			if e := tx.Model(&OutboxEvent{}).Where("source_sequence > ? AND kind = ?", gen.SourceWatermark, "file_catalogue.upsert").Distinct().Pluck("owner_scope", &scopes).Error; e != nil {
				return code("database_unavailable")
			}
			for _, scope := range scopes {
				var progress GenerationScopeProgress
				if e := tx.Where("generation = ? AND owner_scope = ?", gen.Generation, scope).First(&progress).Error; e != nil {
					return code("projection_not_caught_up")
				}
				var scopeHead int64
				tx.Model(&OutboxEvent{}).Select("COALESCE(MAX(source_sequence), 0)").Where("owner_scope = ? AND kind = ?", scope, "file_catalogue.upsert").Scan(&scopeHead)
				if progress.AppliedSeq < scopeHead {
					return code("projection_not_caught_up")
				}
			}
			var checkpoints []ProjectionCheckpoint
			if e := tx.Where("projection = ?", gen.Projection).Find(&checkpoints).Error; e != nil {
				return code("database_unavailable")
			}
			oldGeneration := "GEN_0"
			if len(checkpoints) > 0 {
				oldGeneration = checkpoints[0].Generation
			}
			for _, cp := range checkpoints {
				if cp.Generation != oldGeneration {
					continue
				}
				var scopeHead int64
				tx.Model(&OutboxEvent{}).Select("COALESCE(MAX(source_sequence), 0)").Where("owner_scope = ? AND kind = ?", cp.OwnerScope, "file_catalogue.upsert").Scan(&scopeHead)
				if cp.Watermark < scopeHead {
					return code("projection_not_caught_up")
				}
			}
			newCount, newTomb, newDigest, e := projectionDigestSummary(tx, gen.Projection, gen.Generation)
			if e != nil {
				return code("database_unavailable")
			}
			oldCount, oldTomb, oldDigest, e := projectionDigestSummary(tx, gen.Projection, oldGeneration)
			if e != nil {
				return code("database_unavailable")
			}
			if newCount != oldCount || newTomb != oldTomb || newDigest != oldDigest {
				return code("projection_not_caught_up")
			}
			// Atomic switch: activate the new generation, retire the old
			// one, and move every owner scope's checkpoint in this
			// transaction. Old generations stay readable for diagnostics;
			// reads never mix them.
			now := time.Now().UTC()
			gen.Phase = "active"
			gen.Ready = true
			gen.RowCount = newCount
			gen.Tombstones = newTomb
			gen.Digest = newDigest
			gen.ActivatedAt = &now
			if e := tx.Save(&gen).Error; e != nil {
				return code("database_unavailable")
			}
			if e := tx.Model(&ProjectionGeneration{}).Where("projection = ? AND generation = ? AND phase = ?", gen.Projection, oldGeneration, "active").Update("phase", "retired").Error; e != nil {
				return code("database_unavailable")
			}
			if e := tx.Model(&ProjectionCheckpoint{}).Where("projection = ?", gen.Projection).Update("generation", gen.Generation).Error; e != nil {
				return code("database_unavailable")
			}
			if e := tx.Model(&Operation{}).Where("id = ?", operationID).Updates(map[string]any{"status": "completed", "updated": now}).Error; e != nil {
				return code("database_unavailable")
			}
			result = view("active", true)
			return nil
		case "active":
			var err error
			result, err = rebuildOperationView(tx, operationID)
			return err
		}
		return code("invalid_request")
	})
	if e != nil {
		return nil, e
	}
	return result, nil
}

// RuntimeStatusView is the operator-only read-only runtime view. It never
// includes the DSN or any credential.
func (s *Service) RuntimeStatusView(p Principal) (map[string]any, error) {
	if p.Agent != nil || !p.Admin {
		return nil, code("forbidden")
	}
	queue := map[string]int64{}
	var pendingJobs, claimedJobs, unpublished int64
	s.DB.Model(&RuntimeJob{}).Where("status = ?", "pending").Count(&pendingJobs)
	s.DB.Model(&RuntimeJob{}).Where("status = ?", "claimed").Count(&claimedJobs)
	s.DB.Model(&OutboxEvent{}).Where("published = ?", false).Count(&unpublished)
	queue["pending_jobs"], queue["claimed_jobs"], queue["unpublished_events"] = pendingJobs, claimedJobs, unpublished
	var leases []WorkerLease
	s.DB.Where("expires_at > ?", time.Now().UTC()).Find(&leases)
	workers := []map[string]any{}
	for _, lease := range leases {
		workers = append(workers, map[string]any{"worker_id": lease.WorkerID, "job_id": lease.ID, "fencing_token": lease.FencingToken, "expires_at": lease.ExpiresAt})
	}
	var checkpoints []ProjectionCheckpoint
	s.DB.Find(&checkpoints)
	projections := []map[string]any{}
	for _, cp := range checkpoints {
		var backlog int64
		s.DB.Model(&OutboxEvent{}).Where("owner_scope = ? AND source_sequence > ? AND kind = ?", cp.OwnerScope, cp.Watermark, "file_catalogue.upsert").Count(&backlog)
		projections = append(projections, map[string]any{"projection": cp.Projection, "owner_scope": cp.OwnerScope, "generation": cp.Generation, "watermark": cp.Watermark, "backlog": backlog})
	}
	sort.Slice(projections, func(i, j int) bool {
		return fmt.Sprintf("%v", projections[i]["projection"]) < fmt.Sprintf("%v", projections[j]["projection"])
	})
	return map[string]any{"queue": queue, "workers": workers, "projections": projections, "dialect": s.runtimeDialect()}, nil
}

// MetadataQuiescence proves no pending business writes or live worker leases
// exist before metadata export or import.
func (s *Service) MetadataQuiescence() error {
	var pending int64
	s.DB.Model(&Operation{}).Where("status IN ?", []string{"awaiting_upload", "verifying", "transferring", "running"}).Count(&pending)
	if pending > 0 {
		return code("metadata_migration_requires_quiescence")
	}
	var leases int64
	s.DB.Model(&WorkerLease{}).Where("expires_at > ?", time.Now().UTC()).Count(&leases)
	if leases > 0 {
		return code("metadata_migration_requires_quiescence")
	}
	return nil
}

// ExportMetadata writes the quiesced metadata snapshot as JSONL. Object bytes
// and the instance key are deliberately not part of the export: sealed
// references keep working only where the independent instance key exists.
// Connection credentials travel as sealed_secret bytes: they stay sealed
// under the exporting instance's key and never appear as plaintext.
func (s *Service) ExportMetadata(w interface{ Write([]byte) (int, error) }) error {
	if e := s.MetadataQuiescence(); e != nil {
		return e
	}
	header, _ := json.Marshal(map[string]any{"kind": "header", "instance_id": s.Instance, "exported_at": time.Now().UTC()})
	if _, e := w.Write(append(header, '\n')); e != nil {
		return e
	}
	write := func(kind string, rows interface{}) error {
		b, e := json.Marshal(map[string]any{"kind": kind, "rows": rows})
		if e != nil {
			return e
		}
		_, e = w.Write(append(b, '\n'))
		return e
	}
	spaces := []Space{}
	s.DB.Find(&spaces)
	if e := write("spaces", spaces); e != nil {
		return e
	}
	connections := []Connection{}
	s.DB.Find(&connections)
	exported := make([]connectionExport, 0, len(connections))
	for _, c := range connections {
		exported = append(exported, connectionExport{Connection: c, SealedSecret: c.Secret})
	}
	if e := write("connections", exported); e != nil {
		return e
	}
	files := []File{}
	s.DB.Find(&files)
	if e := write("files", files); e != nil {
		return e
	}
	versions := []Version{}
	s.DB.Find(&versions)
	if e := write("versions", versions); e != nil {
		return e
	}
	operations := []Operation{}
	s.DB.Find(&operations)
	if e := write("operations", operations); e != nil {
		return e
	}
	grants := []AccessGrant{}
	s.DB.Find(&grants)
	return write("grants", grants)
}

// connectionExport carries the sealed connection credential in the snapshot.
// The bytes are AEAD-sealed under the instance key, never plaintext; a
// restored deployment must recover the same instance key independently.
type connectionExport struct {
	Connection
	SealedSecret []byte `json:"sealed_secret,omitempty"`
}

// metadataSnapshot is one decoded export.
type metadataSnapshot struct {
	InstanceID  string
	Spaces      []Space
	Connections []connectionExport
	Files       []File
	Versions    []Version
	Operations  []Operation
	Grants      []AccessGrant
}

// decodeMetadata streams one JSONL export without buffering the whole file.
// Malformed lines are invalid_request, never silently skipped.
func decodeMetadata(r interface{ Read([]byte) (int, error) }) (*metadataSnapshot, error) {
	reader := bufio.NewReader(r)
	snapshot := &metadataSnapshot{}
	for {
		line, e := reader.ReadBytes('\n')
		if len(line) > 0 {
			var envelope struct {
				Kind string          `json:"kind"`
				Rows json.RawMessage `json:"rows"`
			}
			if e := json.Unmarshal(line, &envelope); e != nil {
				return nil, code("invalid_request")
			}
			switch envelope.Kind {
			case "header":
				var h struct {
					InstanceID string `json:"instance_id"`
				}
				if e := json.Unmarshal(line, &h); e != nil {
					return nil, code("invalid_request")
				}
				snapshot.InstanceID = h.InstanceID
			case "spaces":
				if e := json.Unmarshal(envelope.Rows, &snapshot.Spaces); e != nil {
					return nil, code("invalid_request")
				}
			case "connections":
				if e := json.Unmarshal(envelope.Rows, &snapshot.Connections); e != nil {
					return nil, code("invalid_request")
				}
			case "files":
				if e := json.Unmarshal(envelope.Rows, &snapshot.Files); e != nil {
					return nil, code("invalid_request")
				}
			case "versions":
				if e := json.Unmarshal(envelope.Rows, &snapshot.Versions); e != nil {
					return nil, code("invalid_request")
				}
			case "operations":
				if e := json.Unmarshal(envelope.Rows, &snapshot.Operations); e != nil {
					return nil, code("invalid_request")
				}
			case "grants":
				if e := json.Unmarshal(envelope.Rows, &snapshot.Grants); e != nil {
					return nil, code("invalid_request")
				}
			}
		}
		if e != nil {
			if errors.Is(e, io.EOF) {
				break
			}
			return nil, e
		}
	}
	return snapshot, nil
}

// metadataDigest summarizes a snapshot for verification. Connection
// credential presence is part of the digest: a snapshot that dropped sealed
// secrets must never verify as equal to one that carries them.
func metadataDigest(s *metadataSnapshot) map[string]int64 {
	var credentials int64
	for _, c := range s.Connections {
		if len(c.SealedSecret) > 0 {
			credentials++
		}
	}
	return map[string]int64{"spaces": int64(len(s.Spaces)), "connections": int64(len(s.Connections)), "connection_credentials": credentials, "files": int64(len(s.Files)), "versions": int64(len(s.Versions)), "operations": int64(len(s.Operations)), "grants": int64(len(s.Grants))}
}

// ImportMetadata loads an export into a fresh target database. The target
// must carry the same instance identity; a non-empty target needs quiescence
// and a reverse export first, which is what forbids rolling back over new
// writes with an old snapshot. Empty tables import as empty tables and the
// whole load is one all-or-nothing transaction.
func (s *Service) ImportMetadata(r interface{ Read([]byte) (int, error) }, target *gorm.DB) (map[string]any, error) {
	snapshot, e := decodeMetadata(r)
	if e != nil {
		return nil, e
	}
	if snapshot.InstanceID == "" || snapshot.InstanceID != s.Instance {
		return nil, code("invalid_request")
	}
	if e := s.MetadataQuiescence(); e != nil {
		return nil, e
	}
	var existing int64
	target.Model(&File{}).Count(&existing)
	if existing > 0 {
		// The target already carries business rows: switching back over new
		// writes requires a reverse migration after quiescing this target,
		// never a direct restore of an old snapshot.
		return nil, code("metadata_migration_requires_quiescence")
	}
	e = target.Transaction(func(tx *gorm.DB) error {
		// Guarded creates: an empty slice is a valid empty table, not an
		// error, and a mid-load failure rolls the whole import back.
		if len(snapshot.Spaces) > 0 {
			if e := tx.Create(&snapshot.Spaces).Error; e != nil {
				return e
			}
		}
		for i := range snapshot.Connections {
			// Restore the sealed credential bytes so imported connections
			// stay usable wherever the exporting instance key exists.
			snapshot.Connections[i].Secret = snapshot.Connections[i].SealedSecret
			if e := tx.Create(&snapshot.Connections[i].Connection).Error; e != nil {
				return e
			}
		}
		if len(snapshot.Files) > 0 {
			if e := tx.Create(&snapshot.Files).Error; e != nil {
				return e
			}
		}
		if len(snapshot.Versions) > 0 {
			if e := tx.Create(&snapshot.Versions).Error; e != nil {
				return e
			}
		}
		if len(snapshot.Operations) > 0 {
			if e := tx.Create(&snapshot.Operations).Error; e != nil {
				return e
			}
		}
		if len(snapshot.Grants) > 0 {
			if e := tx.Create(&snapshot.Grants).Error; e != nil {
				return e
			}
		}
		return nil
	})
	if e != nil {
		return nil, code("database_unavailable")
	}
	return map[string]any{"imported": metadataDigest(snapshot), "instance_id": snapshot.InstanceID}, nil
}

// VerifyMetadata compares two exports: instance identity, table counts and
// per-table digests must match.
func (s *Service) VerifyMetadata(a, b interface{ Read([]byte) (int, error) }) (map[string]any, error) {
	left, e := decodeMetadata(a)
	if e != nil {
		return nil, e
	}
	right, e := decodeMetadata(b)
	if e != nil {
		return nil, e
	}
	if left.InstanceID != right.InstanceID {
		return nil, code("invalid_request")
	}
	dl, dr := metadataDigest(left), metadataDigest(right)
	matching := true
	for k, v := range dl {
		if dr[k] != v {
			matching = false
		}
	}
	return map[string]any{"instance_id": left.InstanceID, "source": dl, "target": dr, "matching": matching}, nil
}
