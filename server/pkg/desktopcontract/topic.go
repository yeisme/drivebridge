package desktopcontract

import (
	"errors"
	"time"
)

// TopicRef preserves the owner's stable reference; it never creates a tag or project.
type TopicRef struct {
	SourceID      string `json:"source_id" jsonschema:"minLength=1,maxLength=128"`
	Kind          string `json:"kind" jsonschema:"enum=project,enum=tag"`
	OwnerTargetID string `json:"owner_target_id" jsonschema:"minLength=1,maxLength=128"`
}
type UpdateTopicPin struct {
	TargetRef        TopicRef `json:"target_ref"`
	ExpectedRevision uint64   `json:"expected_revision" jsonschema:"maximum=9007199254740991"`
	Pinned           bool     `json:"pinned"`
}
type TopicPin struct {
	TargetRef TopicRef  `json:"target_ref"`
	Pinned    bool      `json:"pinned"`
	Revision  uint64    `json:"revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	UpdatedAt time.Time `json:"updated_at"`
}
type TopicPinPage struct {
	Items      []TopicPin `json:"items" jsonschema:"maxItems=200"`
	NextCursor string     `json:"next_cursor" jsonschema:"maxLength=4096"`
	ObservedAt time.Time  `json:"observed_at"`
}
type TopicPinLookup struct {
	IdempotencyKey string `json:"idempotency_key" jsonschema:"minLength=1,maxLength=128,pattern=^[A-Za-z0-9][A-Za-z0-9._:-]*$"`
}
type TopicPinLookupResult struct {
	Found bool      `json:"found"`
	Pin   *TopicPin `json:"pin" jsonschema:"nullable"`
}

func (r TopicRef) Validate() error {
	if !validID(r.SourceID) || !validID(r.OwnerTargetID) || (r.Kind != "project" && r.Kind != "tag") {
		return errors.New("invalid_topic_ref")
	}
	return nil
}
func (r UpdateTopicPin) Validate() error {
	if r.ExpectedRevision > maxSafeRevision {
		return errors.New("invalid_topic_revision")
	}
	return r.TargetRef.Validate()
}
