package desktopcontract

import (
	"errors"
	"fmt"
	"time"
)

const SourceRangeMaxBytes uint64 = 1 << 20
const AssetRangeMaxBytes uint64 = 8 << 20

// ExportRequest is main-to-facade only. Renderer exportSource accepts a fixed
// ref; main obtains the native selection before constructing this request.
// Neither a destination path nor arbitrary bytes are part of this contract.
type ExportRequest struct {
	TargetRef                   ReadingRef `json:"target_ref"`
	NativeSelectionID           string     `json:"native_selection_id" jsonschema:"minLength=1,maxLength=128"`
	ExpectedDestinationRevision string     `json:"expected_destination_revision" jsonschema:"minLength=1,maxLength=128"`
}

type ExportLookup struct {
	IdempotencyKey string `json:"idempotency_key" jsonschema:"minLength=1,maxLength=128,pattern=^[A-Za-z0-9][A-Za-z0-9._:-]*$"`
}

// ExportOperation is a redacted projection of an existing file operation,
// never a second executable job or permission to resume with another key.
type ExportOperation struct {
	OperationID      string     `json:"operation_id" jsonschema:"minLength=1,maxLength=128"`
	TargetRef        ReadingRef `json:"target_ref"`
	OwnerStatus      string     `json:"owner_status" jsonschema:"minLength=1,maxLength=64"`
	UIState          string     `json:"ui_state" jsonschema:"enum=pending,enum=running,enum=completed,enum=failed,enum=cancelled,enum=unknown"`
	BytesTransferred uint64     `json:"bytes_transferred" jsonschema:"minimum=0,maximum=9007199254740991"`
	TotalBytes       *uint64    `json:"total_bytes" jsonschema:"nullable,minimum=0,maximum=9007199254740991"`
	ObservedAt       time.Time  `json:"observed_at"`
}

type ExportLookupResult struct {
	Found     bool             `json:"found"`
	Operation *ExportOperation `json:"operation" jsonschema:"nullable"`
}

// ByteRange is an inclusive, single closed HTTP range. No suffix, open-ended
// or multipart reads are admitted on this IPC value surface.
type ByteRange struct {
	Start uint64 `json:"start" jsonschema:"minimum=0,maximum=9007199254740991"`
	End   uint64 `json:"end" jsonschema:"minimum=0,maximum=9007199254740991"`
}

type SourceRangeRequest struct {
	TargetRef PinaxRef  `json:"target_ref"`
	Range     ByteRange `json:"range"`
}

type AssetRangeRequest struct {
	Handle string    `json:"handle" jsonschema:"minLength=1,maxLength=128,pattern=^[A-Za-z0-9][A-Za-z0-9._:-]*$"`
	Range  ByteRange `json:"range"`
}

func (r ExportRequest) Validate() error {
	if err := r.TargetRef.Validate(); err != nil {
		return err
	}
	if !validID(r.NativeSelectionID) || !validID(r.ExpectedDestinationRevision) {
		return errors.New("invalid_export_selection")
	}
	return nil
}
func (r ExportLookup) Validate() error {
	if !mutationKey.MatchString(r.IdempotencyKey) {
		return errors.New("invalid_export_lookup")
	}
	return nil
}
func (r ByteRange) Validate(maxBytes uint64) error {
	if maxBytes == 0 || r.Start > maxSafeRevision || r.End > maxSafeRevision || r.End < r.Start || r.End-r.Start >= maxBytes {
		return errors.New("invalid_byte_range")
	}
	return nil
}
func (r ByteRange) Header(maxBytes uint64) (string, error) {
	if err := r.Validate(maxBytes); err != nil {
		return "", err
	}
	return fmt.Sprintf("bytes=%d-%d", r.Start, r.End), nil
}
func (r SourceRangeRequest) Validate() error {
	p := r.TargetRef
	ref := ReadingRef{Kind: p.Kind, InstanceID: p.InstanceID, SourceID: p.SourceID, ObjectID: p.ObjectID, Revision: p.Revision, URI: p.URI}
	if err := ref.Validate(); err != nil {
		return err
	}
	return r.Range.Validate(SourceRangeMaxBytes)
}
func (r AssetRangeRequest) Validate() error {
	if !mutationKey.MatchString(r.Handle) {
		return errors.New("invalid_asset_handle")
	}
	return r.Range.Validate(AssetRangeMaxBytes)
}
func (o ExportOperation) Validate() error {
	if err := o.TargetRef.Validate(); err != nil {
		return err
	}
	if !validID(o.OperationID) || len(o.OwnerStatus) == 0 || len(o.OwnerStatus) > 64 || o.ObservedAt.IsZero() || o.BytesTransferred > maxSafeRevision {
		return errors.New("invalid_export_operation")
	}
	switch o.UIState {
	case "pending", "running", "completed", "failed", "cancelled", "unknown":
	default:
		return errors.New("invalid_export_state")
	}
	if o.TotalBytes != nil && (*o.TotalBytes > maxSafeRevision || o.BytesTransferred > *o.TotalBytes) {
		return errors.New("invalid_export_progress")
	}
	if o.UIState == "completed" && (o.TotalBytes == nil || o.BytesTransferred != *o.TotalBytes) {
		return errors.New("incomplete_export_receipt")
	}
	return nil
}
func (r ExportLookupResult) Validate() error {
	if r.Found != (r.Operation != nil) {
		return errors.New("invalid_export_lookup_result")
	}
	if r.Operation != nil {
		return r.Operation.Validate()
	}
	return nil
}
