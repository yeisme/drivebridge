package service

import "time"

// DeliveryManifest freezes one fixed delivery: the member snapshot, its
// verification facts and the retention readiness. Publishing only completes
// when every required member is readable, verified and retention-protected.
type DeliveryManifest struct {
	ID             string    `json:"manifest_id" gorm:"primaryKey"`
	Owner          string    `json:"-" gorm:"index"`
	ShareID        string    `json:"share_id" gorm:"index;uniqueIndex:delivery_manifest_share_revision"`
	Revision       uint64    `json:"manifest_revision" gorm:"uniqueIndex:delivery_manifest_share_revision"`
	SnapshotID     string    `json:"snapshot_id"`
	Phase          string    `json:"phase"` // verifying|ready|published
	RequiredItems  int64     `json:"required_items"`
	VerifiedItems  int64     `json:"verified_items"`
	Complete       bool      `json:"complete"`
	RetentionReady bool      `json:"retention_ready"`
	Key            string    `json:"-" gorm:"uniqueIndex:delivery_manifest_key"`
	Fingerprint    string    `json:"-"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

func (DeliveryManifest) TableName() string { return "delivery_manifests" }

// DeliveryManifestItem is one frozen member with its verification and
// retention hold reference.
type DeliveryManifestItem struct {
	ID         string    `gorm:"primaryKey"`
	ManifestID string    `json:"manifest_id" gorm:"index;uniqueIndex:delivery_item_identity"`
	FileID     string    `json:"file_id" gorm:"uniqueIndex:delivery_item_identity"`
	VersionID  string    `json:"version_id"`
	SHA256     string    `json:"sha256,omitempty"`
	Size       int64     `json:"size_bytes,omitempty"`
	Verified   bool      `json:"verified"`
	HoldRef    string    `json:"hold_ref,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

func (DeliveryManifestItem) TableName() string { return "delivery_manifest_items" }

// DeliveryReceipt records one server-observed or receiver-authenticated
// delivery fact. Client self-reports stay declared facts; the server never
// upgrades them into verified confirmations.
type DeliveryReceipt struct {
	ID               string    `json:"receipt_id" gorm:"primaryKey"`
	ManifestID       string    `json:"manifest_id" gorm:"index"`
	ManifestRevision uint64    `json:"manifest_revision"`
	Owner            string    `json:"-"`
	ReceiverKind     string    `json:"receiver_kind"` // authenticated|anonymous
	ReceiverRef      string    `json:"receiver_ref,omitempty"`
	Kind             string    `json:"kind"` // accessible|opened|bytes_served|receiver_verified|receiver_confirmed|declared
	Digest           string    `json:"digest,omitempty"`
	Note             string    `json:"note,omitempty"`
	ObservedAt       time.Time `json:"observed_at"`
	CreatedAt        time.Time `json:"created_at"`
}

func (DeliveryReceipt) TableName() string { return "delivery_receipts" }
