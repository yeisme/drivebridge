package service

import "time"

// Session credentials are returned once; only their hashes are persisted.
type ShareSession struct {
	ID                string    `json:"id" gorm:"primaryKey"`
	TokenHash         string    `json:"-" gorm:"uniqueIndex"`
	ShareID           string    `json:"-" gorm:"index"`
	Kind              string    `json:"session_kind"`
	DeviceFingerprint string    `json:"-" gorm:"index"`
	UserID            string    `json:"-"`
	Created           time.Time `json:"created_at"`
	Expires           time.Time `json:"expires_at"`
	Revoked           bool      `json:"revoked"`
}

type ShareDevice struct {
	ID          string    `json:"id" gorm:"primaryKey"`
	ShareID     string    `json:"-" gorm:"uniqueIndex:share_device_fingerprint"`
	Fingerprint string    `json:"fingerprint" gorm:"uniqueIndex:share_device_fingerprint"`
	PublicKey   []byte    `json:"-"`
	Label       string    `json:"label"`
	Created     time.Time `json:"created_at"`
	LastSeen    time.Time `json:"last_seen"`
	Opens       int64     `json:"opens"`
	Revoked     bool      `json:"revoked"`
}

// The composite primary key makes range resumption durable across restarts.
type ShareDownload struct {
	ShareID   string `gorm:"primaryKey"`
	SessionID string `gorm:"primaryKey"`
	FileID    string `gorm:"primaryKey"`
	VersionID string `gorm:"primaryKey"`
	Created   time.Time
}

type shareChallengeState struct {
	ShareID string
	Expires time.Time
}

type ShareChallenge struct {
	Nonce   string    `json:"nonce"`
	Expires time.Time `json:"expires_at"`
}

type ShareProofRequest struct {
	Nonce        string `json:"nonce"`
	Proof        string `json:"proof"`
	Passcode     string `json:"passcode,omitempty"`
	ReceiverKind string `json:"receiver_kind,omitempty"` // Ignored: the server derives this.
}

type ShareDeviceRequest struct {
	PublicKey    string `json:"public_key"` // Hex-encoded Ed25519 public key.
	Label        string `json:"label,omitempty"`
	ReceiverKind string `json:"receiver_kind,omitempty"`
}

type ShareDeviceOpenRequest struct {
	DeviceID     string `json:"device_id"`
	Nonce        string `json:"nonce"`
	Signature    string `json:"signature"` // Hex-encoded signature of share ID + ':' + nonce.
	ReceiverKind string `json:"receiver_kind,omitempty"`
}

type ShareOpenResult struct {
	Opened       bool      `json:"opened"`
	SessionKind  string    `json:"session_kind"`
	ReceiverKind string    `json:"receiver_kind"`
	Token        string    `json:"-"`
	Expires      time.Time `json:"-"`
}

type ShareReceivers struct {
	Devices       []ShareDevice `json:"devices"`
	OpensUsed     int64         `json:"opens_used"`
	OpensMax      *int64        `json:"opens_max"`
	DownloadsUsed int64         `json:"downloads_used"`
	DownloadsMax  *int64        `json:"downloads_max"`
}
