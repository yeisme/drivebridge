package contract

import "time"

// TaskProjection is a read model of an existing owner, never an executable task.
// Revision is a content token, not a monotonic counter or a write precondition.
type TaskProjection struct {
	OwnerKind      string       `json:"owner_kind" jsonschema:"enum=operation,enum=upload_batch,enum=organization_plan,enum=transfer_plan,enum=automation_intent"`
	OwnerID        string       `json:"owner_id"`
	SpaceID        string       `json:"space_id"`
	Title          string       `json:"title"`
	ActorKind      string       `json:"actor_kind" jsonschema:"enum=human,enum=agent,enum=unknown"`
	AgentID        *string      `json:"agent_id" jsonschema:"nullable"`
	OwnerStatus    string       `json:"owner_status"`
	UIState        string       `json:"ui_state" jsonschema:"enum=waiting_confirmation,enum=running,enum=recoverable,enum=completed,enum=failed,enum=cancelled,enum=unknown"`
	Revision       string       `json:"revision"`
	Progress       TaskProgress `json:"progress"`
	AllowedActions []string     `json:"allowed_actions"`
	BlockedReason  string       `json:"blocked_reason"`
	ObservedAt     time.Time    `json:"observed_at"`
}

// OperationTaskDetail exposes only a fixed public result identity, never an
// object key, signed URL, source payload or an alternative execution path.
type OperationTaskDetail struct {
	Task        TaskProjection  `json:"task"`
	Outputs     []TaskOutputRef `json:"outputs"`
	OutputState string          `json:"output_state" jsonschema:"enum=available,enum=unavailable,enum=not_published,enum=not_projected"`
}
type TaskOutputRef struct {
	FileID    string `json:"file_id"`
	VersionID string `json:"version_id"`
	SpaceID   string `json:"space_id"`
}

// BatchTaskItem exposes an original item identity; it is not an executable ID.
type BatchTaskItem struct {
	ItemID          string  `json:"item_id"`
	RelativePath    string  `json:"relative_path"`
	IsDir           bool    `json:"is_dir"`
	OwnerStatus     string  `json:"owner_status"`
	ResultCertainty string  `json:"result_certainty" jsonschema:"enum=confirmed,enum=unknown"`
	OperationID     *string `json:"operation_id" jsonschema:"nullable"`
}
type BatchTaskItems struct {
	BatchID    string          `json:"batch_id"`
	Revision   string          `json:"revision"`
	Items      []BatchTaskItem `json:"items"`
	NextCursor string          `json:"next_cursor"`
	ObservedAt time.Time       `json:"observed_at"`
}

// Nil counts mean unproven, not zero. Byte counters have independent meanings.
type TaskProgress struct {
	Total         *int64 `json:"total" jsonschema:"nullable"`
	Succeeded     *int64 `json:"succeeded" jsonschema:"nullable"`
	Failed        *int64 `json:"failed" jsonschema:"nullable"`
	Pending       *int64 `json:"pending" jsonschema:"nullable"`
	BytesSent     *int64 `json:"bytes_sent" jsonschema:"nullable"`
	DurableBytes  *int64 `json:"durable_bytes" jsonschema:"nullable"`
	VerifiedBytes *int64 `json:"verified_bytes" jsonschema:"nullable"`
}
