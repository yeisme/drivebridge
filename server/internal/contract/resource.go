package contract

import "time"

// RegisterResourceRequest binds a description, not permission to execute.
type RegisterResourceRequest struct {
	SpaceID     string `json:"space_id" jsonschema:"minLength=1,maxLength=512"`
	FileRef     string `json:"file_ref,omitempty" jsonschema:"minLength=1,maxLength=2048"`
	SnapshotID  string `json:"collection_snapshot_id,omitempty" jsonschema:"minLength=1,maxLength=512"`
	Kind        string `json:"kind" jsonschema:"enum=skill,enum=template,enum=reference_pack"`
	DisplayName string `json:"display_name" jsonschema:"minLength=1,maxLength=120"`
}

// ResourceRegistrationReceipt is an original commit fact, re-authorized at read.
type ResourceRegistrationReceipt struct {
	Resource       ResourceView `json:"resource"`
	ResultRevision uint64       `json:"result_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	CommittedAt    time.Time    `json:"committed_at"`
}

type ResourceView struct {
	DescriptorID string    `json:"descriptor_id"`
	SpaceID      string    `json:"space_id"`
	FileRef      string    `json:"file_ref,omitempty"`
	SnapshotID   string    `json:"collection_snapshot_id,omitempty"`
	Kind         string    `json:"kind" jsonschema:"enum=skill,enum=template,enum=reference_pack"`
	DisplayName  string    `json:"display_name"`
	Revision     uint64    `json:"descriptor_revision" jsonschema:"minimum=1,maximum=9007199254740991"`
	CheckState   string    `json:"check_state" jsonschema:"enum=unknown"`
	CreatedAt    time.Time `json:"created_at"`
}
