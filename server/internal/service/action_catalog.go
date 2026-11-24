package service

import (
	"sort"
	"time"
)

// ActionCatalog is the read-only interaction directory shared by HTTP, MCP,
// SDK and Web consumers. It describes how an action may be used; it is never
// an authorization cache or an execution permit.
type ActionCatalog struct {
	CatalogVersion string                `json:"catalog_version"`
	Actions        []ActionCatalogAction `json:"actions"`
	Limits         map[string]*int64     `json:"limits,omitempty"`
	Context        ActionCatalogContext  `json:"context"`
}

type ActionCatalogContext struct {
	InstanceID      string   `json:"instance_id"`
	LocalFileAccess bool     `json:"local_file_access"`
	PublicBaseURL   string   `json:"public_base_url,omitempty"`
	Auth            string   `json:"auth"`
	ContextHeaders  []string `json:"context_headers,omitempty"`
}

type ActionCatalogAction struct {
	ID                string     `json:"id"`
	Intent            string     `json:"intent"`
	Surface           []string   `json:"surface"`
	Tool              string     `json:"tool,omitempty"`
	Implemented       bool       `json:"implemented"`
	Configured        bool       `json:"configured"`
	Authorized        bool       `json:"authorized"`
	DependencyStatus  string     `json:"dependency_status"`
	Availability      string     `json:"availability"`
	BlockReason       string     `json:"block_reason,omitempty"`
	SideEffect        string     `json:"side_effect"`
	Actor             string     `json:"actor"`
	ExecutionLocation string     `json:"execution_location"`
	InputSchemaRef    string     `json:"input_schema_ref,omitempty"`
	ResultKind        string     `json:"result_kind,omitempty"`
	ByteTransport     string     `json:"byte_transport,omitempty"`
	Recovery          []string   `json:"recovery,omitempty"`
	ObservedAt        *time.Time `json:"observed_at,omitempty"`
}

type actionCatalogSpec struct {
	id                string
	intent            string
	legacy            string
	workspace         string
	tool              string
	surface           []string
	sideEffect        string
	actor             string
	executionLocation string
	inputSchemaRef    string
	resultKind        string
	byteTransport     string
	recovery          []string
}

var actionCatalogSpecs = []actionCatalogSpec{
	{
		id: "files.list", intent: "list_authorized_files", legacy: "file_list", tool: "files",
		surface: []string{"http", "mcp", "sdk", "web"}, sideEffect: "read", actor: "human_or_agent", executionLocation: "server",
		inputSchemaRef: "#/components/schemas/FileListRequest", resultKind: "file_page", recovery: []string{"capabilities.check"},
	},
	{
		id: "files.search", intent: "search_authorized_files", legacy: "file_search", tool: "search",
		surface: []string{"http", "mcp", "sdk", "web"}, sideEffect: "read", actor: "human_or_agent", executionLocation: "server",
		inputSchemaRef: "#/components/schemas/FileSearchRequest", resultKind: "file_page", recovery: []string{"capabilities.check"},
	},
	{
		id: "files.inspect", intent: "inspect_fixed_file", legacy: "file_read", tool: "stat",
		surface: []string{"http", "mcp", "sdk", "web"}, sideEffect: "read", actor: "human_or_agent", executionLocation: "server",
		inputSchemaRef: "#/components/schemas/FileInspectRequest", resultKind: "file_ref", recovery: []string{"capabilities.check"},
	},
	{
		id: "upload.create", intent: "create_file_upload", legacy: "upload_create", tool: "create_upload",
		surface: []string{"http", "mcp", "sdk", "web"}, sideEffect: "write", actor: "human_or_agent", executionLocation: "server",
		inputSchemaRef: "#/components/schemas/UploadRequest", resultKind: "operation", byteTransport: "authenticated_http_parts", recovery: []string{"operation.recover", "operation.lookup"},
	},
	{
		id: "upload.complete", intent: "complete_file_upload", legacy: "upload_complete", tool: "complete_upload",
		surface: []string{"http", "mcp", "sdk", "web"}, sideEffect: "write", actor: "human_or_agent", executionLocation: "server",
		inputSchemaRef: "#/components/schemas/CompleteUploadRequest", resultKind: "file_version", byteTransport: "authenticated_http_parts", recovery: []string{"operation.recover"},
	},
	{
		id: "operation.recover", intent: "recover_original_operation", tool: "operation_get",
		surface: []string{"http", "mcp", "sdk", "cli", "web"}, sideEffect: "read", actor: "human_or_agent", executionLocation: "server",
		inputSchemaRef: "#/components/schemas/OperationRecoveryRequest", resultKind: "operation", recovery: []string{"operation.recover"},
	},
	{
		id: "inbox.submit", intent: "submit_agent_artifact", workspace: "inbox.submit",
		surface: []string{"http", "sdk", "web"}, sideEffect: "write", actor: "agent", executionLocation: "server",
		inputSchemaRef: "#/components/schemas/CreateSubmissionRequest", resultKind: "submission", byteTransport: "authenticated_http_parts", recovery: []string{"inbox.submission.lookup", "operation.recover"},
	},
	{
		id: "labels.suggest", intent: "suggest_file_labels", workspace: "labels.suggest",
		surface: []string{"http", "sdk", "web"}, sideEffect: "write", actor: "agent", executionLocation: "server",
		inputSchemaRef: "#/components/schemas/LabelSuggestionRequest", resultKind: "label_suggestion", recovery: []string{"labels.suggest.lookup"},
	},
	{
		id: "inbox.inspect", intent: "inspect_agent_submission",
		surface: []string{"http", "sdk", "web"}, sideEffect: "read", actor: "human_or_agent", executionLocation: "server",
		inputSchemaRef: "#/components/schemas/SubmissionInspectRequest", resultKind: "submission", recovery: []string{"inbox.inspect"},
	},
	{
		id: "inbox.decide", intent: "decide_agent_submission",
		surface: []string{"http", "sdk", "web"}, sideEffect: "decision", actor: "human", executionLocation: "server",
		inputSchemaRef: "#/components/schemas/SubmissionDecisionRequest", resultKind: "submission", recovery: []string{"inbox.inspect"},
	},
	{
		id: "inbox.archive", intent: "archive_accepted_submission",
		surface: []string{"http", "sdk", "web"}, sideEffect: "write", actor: "human", executionLocation: "server",
		inputSchemaRef: "#/components/schemas/ArchiveSubmissionRequest", resultKind: "operation", recovery: []string{"inbox.inspect", "operation.recover"},
	},
}

func (s *Service) ActionCatalog(p Principal, spaceID string) (ActionCatalog, error) {
	catalog := ActionCatalog{
		CatalogVersion: "1.0",
		Actions:        make([]ActionCatalogAction, 0, len(actionCatalogSpecs)),
		Context: ActionCatalogContext{
			InstanceID:      s.Instance,
			LocalFileAccess: false,
			PublicBaseURL:   s.Config.Origin,
			Auth:            "scoped_session_or_agent_token",
			ContextHeaders:  []string{"X-DriveBridge-Tenant", "X-DriveBridge-Space"},
		},
	}
	if spaceID != "" {
		sp, err := s.space(p, spaceID)
		if err != nil {
			return catalog, err
		}
		catalog.Limits = s.capabilityLimits(&sp)
	} else {
		catalog.Limits = s.capabilityLimits(nil)
	}
	if p.Agent != nil && spaceID == "" {
		spaceID = p.Agent.SpaceID
	}

	for _, spec := range actionCatalogSpecs {
		entry := ActionCatalogAction{
			ID:                spec.id,
			Intent:            spec.intent,
			Surface:           append([]string(nil), spec.surface...),
			Tool:              spec.tool,
			Implemented:       true,
			Configured:        true,
			Authorized:        false,
			DependencyStatus:  "not_probed",
			Availability:      "unknown",
			SideEffect:        spec.sideEffect,
			Actor:             spec.actor,
			ExecutionLocation: spec.executionLocation,
			InputSchemaRef:    spec.inputSchemaRef,
			ResultKind:        spec.resultKind,
			ByteTransport:     spec.byteTransport,
			Recovery:          append([]string(nil), spec.recovery...),
		}

		if spec.workspace != "" {
			entry.Actor = "agent"
			if p.Agent == nil {
				entry.Authorized = false
				entry.DependencyStatus = "none"
				entry.Availability = "blocked"
				entry.BlockReason = "agent_only"
			} else if spaceID == "" {
				entry.Availability = "unknown"
				entry.BlockReason = "space_required"
			} else {
				grant, err := s.authorizeWorkspaceActionOn(s.DB, p, p.Agent.SpaceID, p.Agent.ConnectionID, p.Agent.Prefix, spec.workspace, time.Now().UTC())
				if err != nil {
					entry.Availability = "blocked"
					entry.BlockReason = safeCode(err)
				} else {
					entry.Authorized = true
					entry.Configured = grant.ID != ""
					entry.DependencyStatus = "none"
					entry.Availability = "available"
				}
			}
			catalog.Actions = append(catalog.Actions, entry)
			continue
		}

		if spec.legacy != "" {
			if spaceID == "" {
				entry.Availability = "unknown"
				entry.BlockReason = "space_required"
				catalog.Actions = append(catalog.Actions, entry)
				continue
			}
			readiness, err := s.CheckCapability(p, spec.legacy, spaceID, "")
			if err != nil {
				return catalog, err
			}
			entry.Configured = readiness.Configured
			entry.Authorized = readiness.Authorized
			entry.DependencyStatus = readiness.DependencyStatus
			entry.Availability = readiness.Availability
			entry.BlockReason = readiness.BlockReason
			entry.ObservedAt = readiness.ConnectionObservedAt
		}

		if spec.actor == "human" && p.Agent != nil {
			entry.Authorized = false
			entry.Availability = "blocked"
			entry.BlockReason = "human_only"
		}
		catalog.Actions = append(catalog.Actions, entry)
	}

	sort.Slice(catalog.Actions, func(i, j int) bool { return catalog.Actions[i].ID < catalog.Actions[j].ID })
	return catalog, nil
}

func (c ActionCatalog) Find(id string) (ActionCatalogAction, bool) {
	for _, action := range c.Actions {
		if action.ID == id {
			return action, true
		}
	}
	return ActionCatalogAction{}, false
}
