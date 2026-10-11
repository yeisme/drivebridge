// Package desktopcontract defines the planned local desktop wire contract.
// It contains no vault access, credentials, persistence, or runtime capability claims.
package desktopcontract

import (
	"errors"
	"strings"
	"time"

	"github.com/invopop/jsonschema"
)

const Version = "drivebridge.desktop.v1alpha1"

// FileRef and PinaxRef deliberately do not share a path-based identity.
type FileRef struct {
	Kind       string `json:"kind" jsonschema:"enum=file"`
	InstanceID string `json:"instance_id" jsonschema:"minLength=1,maxLength=128"`
	SpaceID    string `json:"space_id" jsonschema:"minLength=1,maxLength=128"`
	FileID     string `json:"file_id" jsonschema:"minLength=1,maxLength=128"`
	VersionID  string `json:"version_id" jsonschema:"minLength=1,maxLength=128"`
}

type PinaxRef struct {
	Kind       string `json:"kind" jsonschema:"enum=pinax"`
	InstanceID string `json:"instance_id" jsonschema:"minLength=1,maxLength=128"`
	SourceID   string `json:"source_id" jsonschema:"minLength=1,maxLength=128"`
	ObjectID   string `json:"object_id" jsonschema:"minLength=1,maxLength=128"`
	Revision   string `json:"revision" jsonschema:"pattern=^sha256:[a-f0-9]{64}$"`
	URI        string `json:"uri" jsonschema:"minLength=1,maxLength=1024,pattern=^pinax://note/"`
}

// ReadingRef is a flattened discriminated union on the wire. Its schema uses
// the concrete DTOs so mixed identities and missing fields fail validation.
type ReadingRef struct {
	Kind       string `json:"kind"`
	InstanceID string `json:"instance_id"`
	SpaceID    string `json:"space_id,omitempty"`
	FileID     string `json:"file_id,omitempty"`
	VersionID  string `json:"version_id,omitempty"`
	SourceID   string `json:"source_id,omitempty"`
	ObjectID   string `json:"object_id,omitempty"`
	Revision   string `json:"revision,omitempty"`
	URI        string `json:"uri,omitempty"`
}

func (ReadingRef) JSONSchema() *jsonschema.Schema {
	r := jsonschema.Reflector{Anonymous: true, DoNotReference: true}
	file, pinax := r.Reflect(FileRef{}), r.Reflect(PinaxRef{})
	file.Version, pinax.Version = "", ""
	return &jsonschema.Schema{OneOf: []*jsonschema.Schema{file, pinax}}
}

type KnowledgeSource struct {
	SourceID    string `json:"source_id" jsonschema:"minLength=1,maxLength=128"`
	Kind        string `json:"kind" jsonschema:"enum=file,enum=pinax"`
	InstanceID  string `json:"instance_id" jsonschema:"minLength=1,maxLength=128"`
	DisplayName string `json:"display_name" jsonschema:"minLength=1,maxLength=256"`
	State       string `json:"state" jsonschema:"enum=ready,enum=hidden,enum=blocked,enum=unavailable"`
	// Granted/implemented intersection from the eventual application service.
	Capabilities    []string `json:"capabilities" jsonschema:"maxItems=32,uniqueItems=true"`
	BindingRevision uint64   `json:"binding_revision" jsonschema:"minimum=1"`
}

type Position struct {
	Revision string `json:"revision" jsonschema:"minLength=1,maxLength=128"`
	Anchor   string `json:"anchor" jsonschema:"minLength=1,maxLength=512"`
	Offset   uint32 `json:"offset" jsonschema:"maximum=8388608"`
}

type ReadingState struct {
	TargetRef ReadingRef `json:"target_ref"`
	Favorite  bool       `json:"favorite"`
	ReadLater bool       `json:"read_later"`
	Position  *Position  `json:"position" jsonschema:"nullable"`
	Revision  uint64     `json:"revision" jsonschema:"minimum=1"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Selected fields are optional, but null is not a request to clear them.
// ClearPosition is explicit so omitted position cannot delete stored progress.
type UpdateReadingState struct {
	TargetRef        ReadingRef `json:"target_ref"`
	ExpectedRevision uint64     `json:"expected_revision"`
	Favorite         *bool      `json:"favorite,omitempty"`
	ReadLater        *bool      `json:"read_later,omitempty"`
	Position         *Position  `json:"position,omitempty"`
	ClearPosition    *bool      `json:"clear_position,omitempty"`
}

type LookupReadingMutation struct {
	IdempotencyKey string `json:"idempotency_key" jsonschema:"minLength=1,maxLength=128,pattern=^[A-Za-z0-9][A-Za-z0-9._:-]*$"`
}

type PageQuery struct {
	Limit  *uint16 `json:"limit,omitempty" jsonschema:"minimum=1,maximum=200"`
	Cursor string  `json:"cursor,omitempty" jsonschema:"maxLength=4096"`
}

type SourcePage struct {
	Items      []KnowledgeSource `json:"items" jsonschema:"maxItems=200"`
	NextCursor string            `json:"next_cursor" jsonschema:"maxLength=4096"`
	ObservedAt time.Time         `json:"observed_at"`
}

type ReadingStatePage struct {
	Items      []ReadingState `json:"items" jsonschema:"maxItems=200"`
	NextCursor string         `json:"next_cursor" jsonschema:"maxLength=4096"`
	ObservedAt time.Time      `json:"observed_at"`
}

func validID(value string) bool {
	return len(value) > 0 && len(value) <= 128 && strings.TrimSpace(value) == value && !strings.ContainsAny(value, "/\\\x00\r\n")
}

func (ref ReadingRef) Validate() error {
	invalid := errors.New("invalid_reading_ref")
	if !validID(ref.InstanceID) {
		return invalid
	}
	switch ref.Kind {
	case "file":
		if !validID(ref.SpaceID) || !validID(ref.FileID) || !validID(ref.VersionID) || ref.SourceID != "" || ref.ObjectID != "" || ref.Revision != "" || ref.URI != "" {
			return invalid
		}
	case "pinax":
		if !validID(ref.SourceID) || !validID(ref.ObjectID) || ref.SpaceID != "" || ref.FileID != "" || ref.VersionID != "" || len(ref.Revision) != 71 || !strings.HasPrefix(ref.Revision, "sha256:") {
			return invalid
		}
		for _, char := range ref.Revision[7:] {
			if !(char >= '0' && char <= '9' || char >= 'a' && char <= 'f') {
				return invalid
			}
		}
		// URI is descriptive, not identity or authorization. Match the fixed ref.
		if ref.URI != "pinax://note/"+ref.ObjectID+"?v="+ref.Revision {
			return invalid
		}
	default:
		return invalid
	}
	return nil
}

func (request UpdateReadingState) Validate() error {
	if err := request.TargetRef.Validate(); err != nil {
		return err
	}
	if request.ClearPosition != nil && !*request.ClearPosition {
		return errors.New("invalid_clear_position")
	}
	if request.Favorite == nil && request.ReadLater == nil && request.Position == nil && (request.ClearPosition == nil || !*request.ClearPosition) {
		return errors.New("empty_reading_update")
	}
	if request.Position != nil {
		if request.ClearPosition != nil && *request.ClearPosition {
			return errors.New("conflicting_reading_position")
		}
		revision := request.TargetRef.VersionID
		if request.TargetRef.Kind == "pinax" {
			revision = request.TargetRef.Revision
		}
		if request.Position.Revision != revision || len(request.Position.Anchor) == 0 || len(request.Position.Anchor) > 512 || request.Position.Offset > 8388608 {
			return errors.New("invalid_reading_position")
		}
	}
	return nil
}
