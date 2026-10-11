package contract

import "time"

// InvocationView reports observed calls; it is not an executable task record.
type InvocationView struct {
	InvocationID string     `json:"invocation_id"`
	AgentID      string     `json:"agent_id"`
	GrantID      string     `json:"grant_id,omitempty"`
	Protocol     string     `json:"protocol" jsonschema:"enum=mcp,enum=http"`
	Action       string     `json:"action"`
	ResourceRefs []string   `json:"resource_refs"`
	OperationRef string     `json:"operation_ref,omitempty"`
	Result       string     `json:"result" jsonschema:"enum=succeeded,enum=partial,enum=failed,enum=denied,enum=unknown"`
	ErrorCode    string     `json:"error_code,omitempty"`
	StartedAt    time.Time  `json:"started_at"`
	FinishedAt   *time.Time `json:"finished_at" jsonschema:"nullable"`
	RequestID    string     `json:"request_id"`
}

// Observation coverage is not an audit completeness guarantee.
type InvocationPage struct {
	Page[InvocationView]
	Coverage string `json:"coverage" jsonschema:"enum=best_effort"`
}

type GrantCheck struct {
	Name   string `json:"name"`
	Status string `json:"status" jsonschema:"enum=unknown,enum=observed_ok,enum=observed_error"`
}

type GrantChecksView struct {
	CheckedAt       time.Time    `json:"checked_at"`
	Checks          []GrantCheck `json:"checks"`
	ConnectionState string       `json:"connection_state" jsonschema:"enum=unknown,enum=observed_ok,enum=observed_error"`
}
