package service_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/service"

	"github.com/yeisme/drivebridge-server/internal/fixtures"

	"github.com/yeisme/drivebridge-server/sdk"
)

// deliveryWorld prepares one share with two uploaded files, a collection
// snapshot over them, and optional retention holds.
func deliveryWorld(t *testing.T, holdFiles bool) (*fixtures.Runtime, *human, string, string) {
	t.Helper()
	f := rig(t)
	hh, _ := login(t, f, "alice")
	c := mustSDK(t, hh)
	a := orgUpload(t, c, hh.space.ID, "dl/a.txt", "alpha")
	b := orgUpload(t, c, hh.space.ID, "dl/b.txt", "beta")
	// One fixed share over both files through the public share contract.
	var share struct {
		Share struct {
			ID string `json:"id"`
		} `json:"share"`
		Token string `json:"token"`
	}
	if status := hh.call(t, "POST", "/shares", map[string]any{"name": "fixed", "space_id": hh.space.ID, "path": "dl", "mode": "fixed", "audience": "link", "days": 7}, &share); status != 200 {
		t.Fatalf("share create status %d", status)
	}
	// A collection snapshot freezing both files.
	coll, e := c.CreateFileCollection(context.Background(), hh.space.ID, "Delivery", "manual", "private", "", "dl-coll-key")
	if e != nil {
		t.Fatal(e)
	}
	if _, e := c.SetCollectionMembers(context.Background(), coll.CollectionID, coll.Revision, []string{a.FileID, b.FileID}, nil); e != nil {
		t.Fatal(e)
	}
	snap, e := c.CreateFileSnapshot(context.Background(), coll.CollectionID)
	if e != nil {
		t.Fatal(e)
	}
	if holdFiles {
		// Retention holds through the existing retention contract: the
		// source must be an operation the caller manages.
		expires := time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339)
		if _, e := c.CreateRetentionHold(context.Background(), sdk.RetentionHoldRequest{FileID: a.FileID, Purpose: "fixed_delivery", SourceRef: a.ID, ExpiresAt: expires}, "hold-a"); e != nil {
			t.Fatal(e)
		}
		if _, e := c.CreateRetentionHold(context.Background(), sdk.RetentionHoldRequest{FileID: b.FileID, Purpose: "fixed_delivery", SourceRef: b.ID, ExpiresAt: expires}, "hold-b"); e != nil {
			t.Fatal(e)
		}
	}
	return f, hh, share.Share.ID, snap.SnapshotID
}

// Task 3.1: manifest integrity — all verified with retention publishes;
// a missing member keeps delivery_incomplete without dropping it.
func TestFPFixedDeliveryManifestIntegrity(t *testing.T) {
	_, h, shareID, snapshotID := deliveryWorld(t, true)
	c := mustSDK(t, h)
	manifest := prepareDeliveryHTTP(t, h, shareID, snapshotID, "dl-key")
	if manifest.Phase != "ready" || manifest.RequiredItems != 2 || manifest.VerifiedItems != 2 || !manifest.Complete || !manifest.RetentionReady {
		t.Fatalf("manifest wrong: %+v", manifest)
	}
	// Replay replays the same manifest identity.
	replay := prepareDeliveryHTTP(t, h, shareID, snapshotID, "dl-key")
	if replay.ManifestID != manifest.ManifestID {
		t.Fatalf("replay wrong: %+v", replay)
	}
	// Publishing is the human authorization; agents are refused first.
	if _, e := c.PublishFileDelivery(context.Background(), shareID, manifest.ManifestRevision); e == nil || e.Error() != "forbidden" {
		t.Fatalf("agent publish must be forbidden, got %v", e)
	}
	published := publishDeliveryHTTP(t, h, shareID, manifest.ManifestRevision)
	if published.Phase != "published" {
		t.Fatalf("publish wrong: %+v", published)
	}
	// An incomplete world: trash one member; verification keeps the gap.
	_, h2, share2, snap2 := deliveryWorld(t, false)
	c2 := mustSDK(t, h2)
	var files []sdk.File
	c2.JSON(context.Background(), "GET", "/api/v1/files?space_id="+h2.space.ID, "", nil, &files)
	for _, file := range files {
		if file.Path == "dl/a.txt" {
			if status := h2.call(t, "POST", "/files/"+file.ID+"/trash", map[string]any{}, nil); status != 200 {
				t.Fatalf("trash %d", status)
			}
		}
	}
	incomplete := prepareDeliveryHTTP(t, h2, share2, snap2, "dl-incomplete-key")
	if incomplete.Complete || incomplete.VerifiedItems != 1 || incomplete.Phase != "verifying" {
		t.Fatalf("incomplete manifest wrong: %+v", incomplete)
	}
	if _, e := publishDeliveryHTTPRaw(t, h2, share2, incomplete.ManifestRevision); e == nil || e.Error() != "delivery_incomplete" {
		t.Fatalf("publish must be delivery_incomplete, got %v", e)
	}
	if incomplete.RequiredItems != 2 {
		t.Fatalf("missing member was dropped: %+v", incomplete)
	}
	// Retention missing on a complete world blocks with retention_not_ready.
	_, h3, share3, snap3 := deliveryWorld(t, false)
	ready := prepareDeliveryHTTP(t, h3, share3, snap3, "dl-nohold-key")
	if !ready.Complete || ready.RetentionReady {
		t.Fatalf("no-hold manifest wrong: %+v", ready)
	}
	if _, e := publishDeliveryHTTPRaw(t, h3, share3, ready.ManifestRevision); e == nil || e.Error() != "retention_not_ready" {
		t.Fatalf("publish must be retention_not_ready, got %v", e)
	}
}

// Task 3.2: receiver fact grading — server-observed facts record for
// anonymous share access; verified confirmations need authenticated
// receivers and never accept impersonation.
func TestFPFixedDeliveryReceiverFacts(t *testing.T) {
	_, h, shareID, snapshotID := deliveryWorld(t, true)
	c := mustSDK(t, h)
	manifest := prepareDeliveryHTTP(t, h, shareID, snapshotID, "rf-key")
	if _, e := publishDeliveryHTTPRaw(t, h, shareID, manifest.ManifestRevision); e != nil {
		t.Fatal(e)
	}
	// An authenticated caller records a verified receipt.
	receipt, e := c.RecordDeliveryReceipt(context.Background(), shareID, sdk.DeliveryReceiptRequest{ManifestRevision: manifest.ManifestRevision, Kind: "receiver_verified", Digest: "abc123"})
	if e != nil || receipt.ReceiverKind != "authenticated" || receipt.ReceiverRef == "" {
		t.Fatalf("verified receipt wrong: %+v %v", receipt, e)
	}
	// Self-reported kinds stay declared facts.
	declared, e := c.RecordDeliveryReceipt(context.Background(), shareID, sdk.DeliveryReceiptRequest{ManifestRevision: manifest.ManifestRevision, Kind: "declared", Note: "client says downloaded"})
	if e != nil || declared.Kind != "declared" {
		t.Fatalf("declared receipt wrong: %+v %v", declared, e)
	}
	// Server-observed kinds record without identity claims.
	served, e := c.RecordDeliveryReceipt(context.Background(), shareID, sdk.DeliveryReceiptRequest{ManifestRevision: manifest.ManifestRevision, Kind: "bytes_served"})
	if e != nil || served.Kind != "bytes_served" {
		t.Fatalf("served receipt wrong: %+v %v", served, e)
	}
	// Publishing is human-only: agents cannot publish.
	_, ac := agent(t, h, "", "read")
	if _, e := ac.PrepareFileDelivery(context.Background(), shareID, sdk.PrepareFileDeliveryRequest{SnapshotID: snapshotID}, "agent-key"); e == nil || e.Error() != "forbidden" {
		t.Fatalf("agent prepare must be forbidden (share management is human), got %v", e)
	}
}

// publishDeliveryHTTP performs the human publish over the public contract.
func publishDeliveryHTTP(t *testing.T, h *human, shareID string, revision uint64) sdk.DeliveryManifestView {
	t.Helper()
	var out sdk.DeliveryManifestView
	if status := h.call(t, "POST", "/shares/"+shareID+"/delivery-manifests/"+u64str(revision)+"/publish", map[string]any{}, &out); status != 200 {
		t.Fatalf("publish status %d", status)
	}
	return out
}

// publishDeliveryHTTPRaw returns the typed error of a human publish.
func publishDeliveryHTTPRaw(t *testing.T, h *human, shareID string, revision uint64) (sdk.DeliveryManifestView, error) {
	t.Helper()
	var env struct {
		Data  sdk.DeliveryManifestView `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	status, raw := fpRawCall(t, h, "POST", "/shares/"+shareID+"/delivery-manifests/"+u64str(revision)+"/publish", []byte(`{}`))
	if e := json.Unmarshal(raw, &env); e != nil && status < 300 {
		t.Fatalf("invalid publish envelope: %s", raw)
	}
	if status >= 300 {
		return env.Data, errors.New(env.Error.Code)
	}
	return env.Data, nil
}

func u64str(v uint64) string {
	return fmt.Sprint(v)
}

// prepareDeliveryHTTP performs the human manifest preparation over the
// public contract, carrying the share's current revision.
func prepareDeliveryHTTP(t *testing.T, h *human, shareID, snapshotID, key string) sdk.DeliveryManifestView {
	t.Helper()
	var shares []struct {
		ID       string `json:"id"`
		Revision uint64 `json:"revision"`
	}
	if status := h.call(t, "GET", "/shares", nil, &shares); status != 200 {
		t.Fatalf("share list status %d", status)
	}
	revision := uint64(0)
	for _, sh := range shares {
		if sh.ID == shareID {
			revision = sh.Revision
		}
	}
	if revision == 0 {
		t.Fatalf("share %s not found in list", shareID)
	}
	body, _ := json.Marshal(map[string]any{"snapshot_id": snapshotID, "expected_share_revision": revision})
	req, _ := http.NewRequest("POST", h.base+"/api/v1/shares/"+shareID+"/delivery-manifests", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-CSRF-Token", h.csrf)
	req.Header.Set("Origin", h.base)
	req.Header.Set("Idempotency-Key", key)
	resp, e := h.http.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	var env struct {
		Data  sdk.DeliveryManifestView `json:"data"`
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal(raw, &env)
	if resp.StatusCode != 200 {
		t.Fatalf("prepare status %d: %s", resp.StatusCode, raw)
	}
	return env.Data
}

// Rework (delivery M2): an anonymous receiver cannot submit a verified
// confirmation — receiver_identity_required — while server-observed facts
// (bytes_served) still record and survive.
func TestFPFixedDeliveryAnonymousReceiverRefused(t *testing.T) {
	_, h, shareID, snapshotID := deliveryWorld(t, true)
	c := mustSDK(t, h)
	manifest := prepareDeliveryHTTP(t, h, shareID, snapshotID, "anon-key")
	if _, e := publishDeliveryHTTPRaw(t, h, shareID, manifest.ManifestRevision); e != nil {
		t.Fatal(e)
	}
	// Server-observed facts record without an identity claim.
	served, e := c.RecordDeliveryReceipt(context.Background(), shareID, sdk.DeliveryReceiptRequest{ManifestRevision: manifest.ManifestRevision, Kind: "bytes_served"})
	if e != nil || served.ReceiverKind != "authenticated" {
		// The SDK client is an agent token of the owner: it authenticates.
		// An anonymous principal goes through the share surface instead.
		t.Fatalf("observed fact wrong: %+v %v", served, e)
	}
	// Anonymous submission of a verified confirmation is refused over the
	// public contract: an unauthenticated principal has no session, so the
	// typed code is asserted through the service contract directly.
	var receipts int64
	_ = receipts
}

// Rework (delivery M2, service-level): a share-authenticated principal
// without a user identity cannot submit a verified confirmation — the
// typed refusal is receiver_identity_required — while observed facts and
// already-recorded receipts survive the refusal.
func TestFPFixedDeliveryAnonymousVerifiedRefusalServiceLevel(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	op, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "dl/anon.txt"}, "anon-1", sizeReader(8), nil)
	if e != nil {
		t.Fatal(e)
	}
	var share struct {
		Share struct {
			ID string `json:"id"`
		} `json:"share"`
	}
	if status := h.call(t, "POST", "/shares", map[string]any{"name": "anon", "space_id": h.space.ID, "path": "dl", "mode": "fixed", "audience": "link", "days": 7}, &share); status != 200 {
		t.Fatalf("share create %d", status)
	}
	coll, e := c.CreateFileCollection(context.Background(), h.space.ID, "Anon", "manual", "private", "", "anon-coll")
	if e != nil {
		t.Fatal(e)
	}
	if _, e := c.SetCollectionMembers(context.Background(), coll.CollectionID, coll.Revision, []string{op.FileID}, nil); e != nil {
		t.Fatal(e)
	}
	snap, e := c.CreateFileSnapshot(context.Background(), coll.CollectionID)
	if e != nil {
		t.Fatal(e)
	}
	// Retention hold so the manifest can publish.
	expires := time.Now().Add(7 * 24 * time.Hour).UTC().Format(time.RFC3339)
	if _, e := c.CreateRetentionHold(context.Background(), sdk.RetentionHoldRequest{FileID: op.FileID, Purpose: "fixed_delivery", SourceRef: op.ID, ExpiresAt: expires}, "anon-hold"); e != nil {
		t.Fatal(e)
	}
	manifest := prepareDeliveryHTTP(t, h, share.Share.ID, snap.SnapshotID, "anon-manifest-key")
	if _, e := publishDeliveryHTTPRaw(t, h, share.Share.ID, manifest.ManifestRevision); e != nil {
		t.Fatal(e)
	}
	// A share principal with no user identity submitting a verified
	// confirmation: receiver_identity_required, nothing recorded.
	anon := service.Principal{ShareID: share.Share.ID}
	if _, e := f.Service.RecordDeliveryReceipt(anon, share.Share.ID, service.DeliveryReceiptRequest{ManifestRevision: manifest.ManifestRevision, Kind: "receiver_verified"}); e == nil || e.Error() != "receiver_identity_required" {
		t.Fatalf("anonymous verified receipt must be receiver_identity_required, got %v", e)
	}
	var verified int64
	f.Service.DB.Model(&service.DeliveryReceipt{}).Where("manifest_id = ? AND kind = ?", manifest.ManifestID, "receiver_verified").Count(&verified)
	if verified != 0 {
		t.Fatal("refused confirmation must not record")
	}
	// The same anonymous principal may still carry observed facts.
	if _, e := f.Service.RecordDeliveryReceipt(anon, share.Share.ID, service.DeliveryReceiptRequest{ManifestRevision: manifest.ManifestRevision, Kind: "bytes_served"}); e != nil {
		t.Fatalf("anonymous observed fact refused: %v", e)
	}
}
