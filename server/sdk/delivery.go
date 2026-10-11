// Fixed delivery SDK surface. Field names match the design contract.
package sdk

import (
	"context"
	"net/url"
	"time"
)

type PrepareFileDeliveryRequest struct {
	SnapshotID            string `json:"snapshot_id"`
	ExpectedShareRevision uint64 `json:"expected_share_revision"`
}

type DeliveryItemView struct {
	FileID    string `json:"file_id"`
	VersionID string `json:"version_id"`
	SHA256    string `json:"sha256,omitempty"`
	Verified  bool   `json:"verified"`
	HoldRef   string `json:"hold_ref,omitempty"`
}

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

type DeliveryReceiptRequest struct {
	ManifestRevision uint64 `json:"manifest_revision"`
	Kind             string `json:"kind"`
	Digest           string `json:"digest,omitempty"`
	Note             string `json:"note,omitempty"`
}

type DeliveryReceipt struct {
	ReceiptID        string    `json:"receipt_id"`
	ManifestID       string    `json:"manifest_id"`
	ManifestRevision uint64    `json:"manifest_revision"`
	ReceiverKind     string    `json:"receiver_kind"`
	ReceiverRef      string    `json:"receiver_ref,omitempty"`
	Kind             string    `json:"kind"`
	Digest           string    `json:"digest,omitempty"`
	Note             string    `json:"note,omitempty"`
	ObservedAt       time.Time `json:"observed_at"`
}

func (c *Client) PrepareFileDelivery(ctx context.Context, shareID string, q PrepareFileDeliveryRequest, key string) (DeliveryManifestView, error) {
	var out DeliveryManifestView
	e := c.JSON(ctx, "POST", "/api/v1/shares/"+url.PathEscape(shareID)+"/delivery-manifests", key, q, &out)
	return out, e
}

func (c *Client) GetFileDelivery(ctx context.Context, shareID string, revision uint64) (DeliveryManifestView, error) {
	var out DeliveryManifestView
	e := c.JSON(ctx, "GET", "/api/v1/shares/"+url.PathEscape(shareID)+"/delivery-manifests/"+url.PathEscape(u64toa(revision)), "", nil, &out)
	return out, e
}

func (c *Client) PublishFileDelivery(ctx context.Context, shareID string, revision uint64) (DeliveryManifestView, error) {
	var out DeliveryManifestView
	e := c.JSON(ctx, "POST", "/api/v1/shares/"+url.PathEscape(shareID)+"/delivery-manifests/"+url.PathEscape(u64toa(revision))+"/publish", "", nil, &out)
	return out, e
}

func (c *Client) RecordDeliveryReceipt(ctx context.Context, shareID string, q DeliveryReceiptRequest) (DeliveryReceipt, error) {
	var out DeliveryReceipt
	e := c.JSON(ctx, "POST", "/api/v1/delivery-receipts?share_id="+url.QueryEscape(shareID), "", q, &out)
	return out, e
}

func u64toa(v uint64) string {
	if v == 0 {
		return "0"
	}
	digits := ""
	for v > 0 {
		digits = string(rune('0'+v%10)) + digits
		v /= 10
	}
	return digits
}
