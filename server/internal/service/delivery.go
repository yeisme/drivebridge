package service

import (
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
)

// Fixed delivery: frozen manifests over share snapshots, graded receiver
// facts and retention-guarded publication.

// PrepareFileDeliveryRequest freezes one delivery from a snapshot.
type PrepareFileDeliveryRequest struct {
	SnapshotID            string `json:"snapshot_id"`
	ExpectedShareRevision uint64 `json:"expected_share_revision"`
}

// DeliveryManifestView is the manifest read model.
type DeliveryManifestView struct {
	ManifestID       string             `json:"manifest_id"`
	ShareID          string             `json:"share_id"`
	ManifestRevision uint64             `json:"manifest_revision"`
	Phase            string             `json:"phase"`
	RequiredItems    int64              `json:"required_items"`
	VerifiedItems    int64              `json:"verified_items"`
	Complete         bool               `json:"complete"`
	RetentionReady   bool               `json:"retention_ready"`
	Items            []DeliveryItemView `json:"items,omitempty"`
}

// DeliveryItemView is one frozen member's outcome.
type DeliveryItemView struct {
	FileID    string `json:"file_id"`
	VersionID string `json:"version_id"`
	SHA256    string `json:"sha256,omitempty"`
	Verified  bool   `json:"verified"`
	HoldRef   string `json:"hold_ref,omitempty"`
}

// shareForDelivery loads one share in the caller's management scope.
func (s *Service) shareForDelivery(p Principal, shareID string) (Share, error) {
	var share Share
	if s.DB.First(&share, "id = ?", shareID).Error != nil {
		return share, code("not_found")
	}
	if share.Owner != p.dataOwner() {
		return share, code("forbidden")
	}
	return share, nil
}

// PrepareFileDelivery freezes one delivery manifest from a collection
// snapshot under the caller's share management permission. Verification
// runs per item; missing items never get silently dropped.
func (s *Service) PrepareFileDelivery(p Principal, shareID string, q PrepareFileDeliveryRequest, key string) (DeliveryManifestView, error) {
	if p.Agent != nil {
		// Share management is a human permission; agents never prepare
		// deliveries over someone's share.
		return DeliveryManifestView{}, code("forbidden")
	}
	if key == "" || len(key) > 200 || q.SnapshotID == "" {
		return DeliveryManifestView{}, code("invalid_request")
	}
	share, e := s.shareForDelivery(p, shareID)
	if e != nil {
		return DeliveryManifestView{}, e
	}
	var snapshot CollectionSnapshot
	if s.DB.First(&snapshot, "id = ? AND owner = ?", q.SnapshotID, p.dataOwner()).Error != nil {
		return DeliveryManifestView{}, code("not_found")
	}
	var entries []CollectionSnapshotEntry
	s.DB.Where("snapshot_id = ?", snapshot.ID).Order("created_at").Find(&entries)
	if len(entries) == 0 {
		return DeliveryManifestView{}, code("invalid_request")
	}
	// The fingerprint normalization lives in one helper shared with
	// LookupOperation: recovery recomputes exactly what creation computed.
	// The share revision joins it: the same key replaying the same
	// expectation reuses the manifest, a different expectation is a
	// different business input.
	fingerprint := deliveryFingerprint(shareID, q.SnapshotID, q.ExpectedShareRevision)
	s.mu.Lock()
	defer s.mu.Unlock()
	var manifest DeliveryManifest
	e = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&manifest, "owner = ? AND key = ?", p.dataOwner(), key).Error; err == nil {
			// Same key, same fingerprint: the idempotent replay of a lost
			// creation response reuses the original manifest whatever the
			// share did afterwards.
			if manifest.Fingerprint != fingerprint {
				return code("idempotency_conflict")
			}
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		// New preparations are optimistic-concurrency guarded: the share's
		// monotonic revision must equal the caller's expectation. A stale
		// expectation is version_conflict — never silently recomputed.
		if share.Revision != q.ExpectedShareRevision {
			return code("version_conflict")
		}
		now := time.Now().UTC()
		// Manifest revisions advance per share: a replacement for a missing
		// member is a new revision needing fresh confirmation, never an
		// edit of the old manifest.
		var maxRev uint64
		tx.Model(&DeliveryManifest{}).Where("share_id = ?", share.ID).Select("COALESCE(MAX(revision),0)").Scan(&maxRev)
		manifest = DeliveryManifest{ID: ID(), Owner: p.dataOwner(), ShareID: share.ID, Revision: maxRev + 1, SnapshotID: q.SnapshotID, Phase: "verifying", RequiredItems: int64(len(entries)), Key: key, Fingerprint: fingerprint, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&manifest).Error; err != nil {
			return code("database_unavailable")
		}
		for _, entry := range entries {
			item := DeliveryManifestItem{ID: ID(), ManifestID: manifest.ID, FileID: entry.FileID, VersionID: entry.VersionID, SHA256: entry.SHA256, Verified: false, CreatedAt: now}
			if err := tx.Create(&item).Error; err != nil {
				return code("database_unavailable")
			}
		}
		return nil
	})
	if e != nil {
		return DeliveryManifestView{}, e
	}
	return s.verifyDeliveryManifest(p, manifest), nil
}

// deliveryFingerprint is the canonical normalization of one delivery
// preparation: the single definition shared by PrepareFileDelivery and
// LookupOperation. The expected share revision is part of the business
// input, so a different expectation under the same key is a conflict.
func deliveryFingerprint(shareID, snapshotID string, expectedShareRevision uint64) string {
	fingerprintSource := struct {
		Share    string `json:"share"`
		Snapshot string `json:"snapshot"`
		Revision uint64 `json:"expected_share_revision"`
	}{shareID, snapshotID, expectedShareRevision}
	body, _ := json.Marshal(fingerprintSource)
	return Hash(string(body))
}

// verifyDeliveryManifest re-checks every frozen member: readability, the
// pinned version and the digest; retention readiness follows the active
// hold coverage. Incomplete stays incomplete — no member is dropped.
func (s *Service) verifyDeliveryManifest(p Principal, manifest DeliveryManifest) DeliveryManifestView {
	var items []DeliveryManifestItem
	s.DB.Where("manifest_id = ?", manifest.ID).Order("created_at").Find(&items)
	verified := int64(0)
	now := time.Now().UTC()
	for i := range items {
		item := &items[i]
		ok := false
		var f File
		if s.DB.First(&f, "id = ?", item.FileID).Error == nil && !f.Trashed && f.VersionID == item.VersionID {
			if _, e := s.file(p, item.FileID, "read"); e == nil {
				var v Version
				if s.DB.First(&v, "id = ?", item.VersionID).Error == nil {
					if item.SHA256 == "" || v.SHA256 == item.SHA256 {
						ok = true
					}
				}
			}
		}
		// Retention coverage: an active hold on the pinned version counts.
		holdRef := ""
		var hold RetentionHold
		if s.DB.Where("file_id = ? AND (version_id = ? OR version_id = '') AND state = ? AND expires_at > ?", item.FileID, item.VersionID, "active", now).First(&hold).Error == nil {
			holdRef = hold.ID
		}
		if ok {
			verified++
		}
		s.DB.Model(&DeliveryManifestItem{}).Where("id = ?", item.ID).Updates(map[string]any{"verified": ok, "hold_ref": holdRef})
		item.Verified = ok
		item.HoldRef = holdRef
	}
	complete := verified == manifest.RequiredItems && verified > 0
	retentionReady := complete
	if retentionReady {
		for _, item := range items {
			if item.HoldRef == "" {
				retentionReady = false
			}
		}
	}
	phase := manifest.Phase
	if manifest.Phase != "published" {
		if complete {
			phase = "ready"
		} else {
			phase = "verifying"
		}
	}
	s.DB.Model(&DeliveryManifest{}).Where("id = ?", manifest.ID).Updates(map[string]any{"verified_items": verified, "complete": complete, "retention_ready": retentionReady, "phase": phase, "updated_at": now})
	view := DeliveryManifestView{ManifestID: manifest.ID, ShareID: manifest.ShareID, ManifestRevision: manifest.Revision, Phase: phase, RequiredItems: manifest.RequiredItems, VerifiedItems: verified, Complete: complete, RetentionReady: retentionReady, Items: []DeliveryItemView{}}
	for _, item := range items {
		view.Items = append(view.Items, DeliveryItemView{FileID: item.FileID, VersionID: item.VersionID, SHA256: item.SHA256, Verified: item.Verified, HoldRef: item.HoldRef})
	}
	return view
}

// GetFileDelivery reads one manifest revision with per-item facts.
func (s *Service) GetFileDelivery(p Principal, shareID string, revision uint64) (DeliveryManifestView, error) {
	if _, e := s.shareForDelivery(p, shareID); e != nil {
		return DeliveryManifestView{}, e
	}
	var manifest DeliveryManifest
	if s.DB.First(&manifest, "share_id = ? AND revision = ?", shareID, revision).Error != nil {
		return DeliveryManifestView{}, code("not_found")
	}
	return s.verifyDeliveryManifest(p, manifest), nil
}

// PublishFileDelivery is the human authorization: it only completes when
// every required member is verified and retention protection stands.
func (s *Service) PublishFileDelivery(p Principal, shareID string, revision uint64) (DeliveryManifestView, error) {
	if p.Agent != nil {
		return DeliveryManifestView{}, code("forbidden")
	}
	if _, e := s.shareForDelivery(p, shareID); e != nil {
		return DeliveryManifestView{}, e
	}
	var manifest DeliveryManifest
	if s.DB.First(&manifest, "share_id = ? AND revision = ?", shareID, revision).Error != nil {
		return DeliveryManifestView{}, code("not_found")
	}
	view := s.verifyDeliveryManifest(p, manifest)
	if !view.Complete {
		return DeliveryManifestView{}, code("delivery_incomplete")
	}
	if !view.RetentionReady {
		return DeliveryManifestView{}, code("retention_not_ready")
	}
	s.DB.Model(&DeliveryManifest{}).Where("id = ?", manifest.ID).Updates(map[string]any{"phase": "published", "updated_at": time.Now().UTC()})
	view.Phase = "published"
	return view, nil
}

// DeliveryReceiptRequest records one delivery fact.
type DeliveryReceiptRequest struct {
	ManifestRevision uint64 `json:"manifest_revision"`
	Kind             string `json:"kind"`
	Digest           string `json:"digest,omitempty"`
	Note             string `json:"note,omitempty"`
}

// RecordDeliveryReceipt records one delivery fact on a published manifest.
// Anonymous share access can carry server-observed facts only; verified
// confirmations need an authenticated receiver — a self-report never gets
// upgraded into someone else's confirmation.
func (s *Service) RecordDeliveryReceipt(p Principal, shareID string, q DeliveryReceiptRequest) (DeliveryReceipt, error) {
	switch q.Kind {
	case "accessible", "opened", "bytes_served", "declared":
	case "receiver_verified", "receiver_confirmed":
		// Confirmed receiver facts require a real authenticated receiver.
		if p.UserID == "" {
			return DeliveryReceipt{}, code("receiver_identity_required")
		}
	default:
		return DeliveryReceipt{}, code("invalid_request")
	}
	var share Share
	if s.DB.First(&share, "id = ?", shareID).Error != nil {
		return DeliveryReceipt{}, code("not_found")
	}
	if share.Revoked || !shareActive(share) {
		return DeliveryReceipt{}, code("share_unavailable")
	}
	var manifest DeliveryManifest
	if s.DB.First(&manifest, "share_id = ? AND revision = ?", shareID, q.ManifestRevision).Error != nil {
		return DeliveryReceipt{}, code("not_found")
	}
	if manifest.Phase != "published" {
		return DeliveryReceipt{}, code("invalid_request")
	}
	receiverKind := "anonymous"
	receiverRef := ""
	if p.UserID != "" {
		receiverKind = "authenticated"
		receiverRef = p.UserID
	}
	now := time.Now().UTC()
	receipt := DeliveryReceipt{ID: ID(), ManifestID: manifest.ID, ManifestRevision: q.ManifestRevision, Owner: manifest.Owner, ReceiverKind: receiverKind, ReceiverRef: receiverRef, Kind: q.Kind, Digest: q.Digest, Note: q.Note, ObservedAt: now, CreatedAt: now}
	if e := s.DB.Create(&receipt).Error; e != nil {
		return DeliveryReceipt{}, code("database_unavailable")
	}
	receipt.Owner = ""
	return receipt, nil
}
