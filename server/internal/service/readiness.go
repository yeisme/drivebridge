package service

import (
	"encoding/json"
	"time"
)

// ActionReadiness is a read-only projection of whether the current identity
// may run one action in one space right now. It is not a second authorization
// system and never caches a future execution permit: every layer reports its
// own fact, and only an observed dependency can yield "available".
type ActionReadiness struct {
	Action               string              `json:"action"`
	Implemented          bool                `json:"implemented"`
	Configured           bool                `json:"configured"`
	Authorized           bool                `json:"authorized"`
	DependencyStatus     string              `json:"dependency_status"`
	ConnectionObservedAt *time.Time          `json:"connection_observed_at"`
	Availability         string              `json:"availability"`
	BlockReason          string              `json:"block_reason,omitempty"`
	Recovery             *CapabilityRecovery `json:"recovery"`
	Limits               map[string]*int64   `json:"limits,omitempty"`
}

// CapabilityRecovery names the next safe step only: an action identifier, the
// acting party, where it runs, and at most a safe parameter name. It never
// carries tokens, owner-private paths, or temporary URLs.
type CapabilityRecovery struct {
	Action    string `json:"action"`
	Actor     string `json:"actor"`
	Location  string `json:"location"`
	Parameter string `json:"parameter,omitempty"`
}

// actionDefinition is one catalogued action. The catalog is the single source
// shared by HTTP, MCP and SDK; unknown actions are refused, never guessed.
type actionDefinition struct {
	scope           string // capability scope checked against the principal
	needsStorage    bool   // depends on a configured object-store connection
	ownerHost       bool   // executes on the owner host (mount adapter)
	agentAllowed    bool   // whether an agent principal may run it
	uploadsOverHTTP bool   // bytes flow over the authenticated HTTP part path
	recovery        CapabilityRecovery
}

var actionCatalog = map[string]actionDefinition{
	"upload_create":   {scope: "write", needsStorage: true, agentAllowed: true, uploadsOverHTTP: true, recovery: CapabilityRecovery{Action: "connections.check", Actor: "human", Location: "server", Parameter: "connection_id"}},
	"upload_complete": {scope: "write", needsStorage: true, agentAllowed: true, uploadsOverHTTP: true, recovery: CapabilityRecovery{Action: "operation_get", Actor: "agent", Location: "server", Parameter: "operation_id"}},
	"file_read":       {scope: "read", needsStorage: true, agentAllowed: true, recovery: CapabilityRecovery{Action: "connections.check", Actor: "human", Location: "server", Parameter: "connection_id"}},
	"file_list":       {scope: "read", agentAllowed: true, recovery: CapabilityRecovery{Action: "capability_check", Actor: "human", Location: "server"}},
	"file_search":     {scope: "read", agentAllowed: true, recovery: CapabilityRecovery{Action: "capability_check", Actor: "human", Location: "server"}},
	"file_archive":    {scope: "write", needsStorage: true, agentAllowed: true, recovery: CapabilityRecovery{Action: "connections.check", Actor: "human", Location: "server", Parameter: "connection_id"}},
	"file_trash":      {scope: "delete", agentAllowed: true, recovery: CapabilityRecovery{Action: "capability_check", Actor: "human", Location: "server"}},
	"file_restore":    {scope: "delete", agentAllowed: true, recovery: CapabilityRecovery{Action: "capability_check", Actor: "human", Location: "server"}},
	// Mounting runs on the owner host: a pure MCP client can discover the
	// action and its dependency, but only the owner host can satisfy it.
	"mount_directory": {scope: "read", ownerHost: true, agentAllowed: false, recovery: CapabilityRecovery{Action: "install_mount_adapter", Actor: "human", Location: "owner_host"}},
	"usage_scan":      {scope: "read", agentAllowed: true, recovery: CapabilityRecovery{Action: "capability_check", Actor: "human", Location: "server"}},
}

// ImplementedActions lists catalogued actions for capability discovery.
func ImplementedActions() []string {
	out := make([]string, 0, len(actionCatalog))
	for name := range actionCatalog {
		out = append(out, name)
	}
	sortStrings(out)
	return out
}

func sortStrings(v []string) {
	for i := 1; i < len(v); i++ {
		for j := i; j > 0 && v[j] < v[j-1]; j-- {
			v[j], v[j-1] = v[j-1], v[j]
		}
	}
}

// spaceConnection resolves the storage connection backing one space without
// probing it: configuration and observation stay separate facts. A nil result
// means "not configured" (no binding, missing row, or disabled).
func (s *Service) spaceConnection(owner, spaceID string) *Connection {
	var sp Space
	if s.DB.First(&sp, "id = ?", spaceID).Error != nil {
		return nil
	}
	if sp.ConnectionID == "" {
		return nil
	}
	var conn Connection
	if s.DB.First(&conn, "id = ? AND (owner = ? OR owner = ?)", sp.ConnectionID, owner, "").Error != nil {
		return nil
	}
	if conn.Disabled {
		return nil
	}
	return &conn
}

// CheckCapability is the application service behind HTTP, MCP and SDK. It is
// strictly read-only: no probe, no connection change, no permission change.
func (s *Service) CheckCapability(p Principal, action, spaceID, ref string) (ActionReadiness, error) {
	out := ActionReadiness{Action: action, Availability: "blocked", DependencyStatus: "none"}
	def, implemented := actionCatalog[action]
	if action == "" || spaceID == "" {
		return out, code("invalid_request")
	}
	if !implemented {
		// Unknown actions never fall back to a near guess; the client must
		// re-read the discovery contract.
		return out, code("unsupported_action")
	}
	out.Implemented = true
	sp, e := s.space(p, spaceID)
	if e != nil {
		return out, e
	}
	out.Limits = s.capabilityLimits(&sp)
	out.Recovery = &def.recovery
	// An optional ref narrows the check to one pinned file; a ref outside the
	// requested space is a client error, not a cross-space lookup.
	if ref != "" {
		fileID, _, e := s.resolveRef(ref)
		if e != nil {
			return out, e
		}
		f, e := s.file(p, fileID, def.scope)
		if e != nil {
			out.Authorized = false
			out.BlockReason = safeCode(e)
			// The failure body must not leak directory entries; only the
			// readiness verdict and the recovery pointer are returned.
			return out, nil
		}
		if f.SpaceID != spaceID {
			return out, code("invalid_request")
		}
	}
	// Authorization is evaluated with the existing permission stack; identity
	// comes from the principal, never from the request body. A prefix-scoped
	// agent is judged on its own prefix: a space-wide verdict would overstate
	// what the agent may touch.
	out.Authorized = true
	probePath := ""
	if p.Agent != nil {
		// Prefix agents are judged on their own prefix; with a ref the file
		// itself was already checked at file granularity above.
		probePath = p.Agent.Prefix
	}
	if e := s.allow(p, spaceID, probePath, def.scope); e != nil {
		out.Authorized = false
		out.BlockReason = safeCode(e)
	}
	if !def.agentAllowed && p.Agent != nil {
		out.Authorized = false
		out.BlockReason = "agent_not_permitted"
	}
	// Dependency facts: configured is a stored row; observed is a stored probe
	// result. Missing observation keeps availability "unknown".
	if def.needsStorage {
		conn := s.spaceConnection(p.dataOwner(), spaceID)
		out.Configured = conn != nil
		if conn == nil {
			out.DependencyStatus = "unavailable"
			out.BlockReason = "storage_not_configured"
			out.Recovery = &CapabilityRecovery{Action: "connections.s3", Actor: "human", Location: "server"}
			return out, nil
		}
		out.ConnectionObservedAt = conn.ObservedAt
		switch {
		case conn.ObservedAt == nil || conn.ObservedStatus == "":
			out.DependencyStatus = "not_probed"
			out.Availability = "unknown"
			return out, nil
		case conn.ObservedStatus != "verified":
			out.DependencyStatus = "unavailable"
			out.BlockReason = "dependency_unavailable"
			out.Availability = "blocked"
			return out, nil
		}
		out.DependencyStatus = "ok"
	} else if def.ownerHost {
		// The mount adapter lives on the owner host; the service can observe
		// neither its presence nor its health from here.
		out.DependencyStatus = "not_probed"
	}
	if !out.Authorized {
		out.Availability = "blocked"
		return out, nil
	}
	if def.ownerHost {
		// Owner-host actions stay blocked from this surface: only the owner
		// host can confirm and run them.
		out.BlockReason = "owner_host_dependency"
		out.Availability = "blocked"
		return out, nil
	}
	if out.DependencyStatus == "not_probed" {
		out.Availability = "unknown"
		return out, nil
	}
	out.Availability = "available"
	return out, nil
}

// capabilityLimits reports the effective limits as the intersection of the
// known instance and space limits. Unknown values stay null (JSON nil): null
// never means unlimited.
func (s *Service) capabilityLimits(sp *Space) map[string]*int64 {
	limits := map[string]*int64{}
	if sp != nil {
		instance := s.Config.MaxBytes
		effective := instance
		if sp.MaxBytes > 0 && sp.MaxBytes < effective {
			effective = sp.MaxBytes
		}
		limits["max_file_bytes"] = &effective
		if sp.MaxBytes > 0 {
			quota := sp.MaxBytes
			remaining := sp.MaxBytes - sp.Used - sp.Reserved
			if remaining < 0 {
				remaining = 0
			}
			limits["space_quota_bytes"] = &quota
			limits["space_remaining_bytes"] = &remaining
		}
	} else {
		instance := s.Config.MaxBytes
		limits["max_file_bytes"] = &instance
	}
	// Batch limits arrive with the upload-batch package; until then the value
	// is unknown, not zero and not unlimited.
	limits["max_batch_items"] = nil
	return limits
}

// CapabilityExtension is the additive block attached to the existing
// capabilities document: discovery of implemented actions plus effective
// limits. All pre-existing keys keep their types and semantics.
func (s *Service) CapabilityExtension(p Principal, spaceID string) (map[string]any, error) {
	var sp *Space
	if spaceID != "" {
		resolved, e := s.space(p, spaceID)
		if e != nil {
			return nil, e
		}
		sp = &resolved
	}
	ext := map[string]any{"spec_version": "0.1", "actions": ImplementedActions()}
	b, _ := json.Marshal(s.capabilityLimits(sp))
	limits := map[string]any{}
	json.Unmarshal(b, &limits)
	ext["limits"] = limits
	return ext, nil
}
