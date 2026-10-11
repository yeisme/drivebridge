package service

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// Agent organization: frozen plans over exact snapshots, human-confirmed
// or policy-preauthorized execution, and compensation as a separate
// reverse plan. Every write re-checks permission, source version and
// policy; drift makes the item stale, never silently recalculated.

var organizationActions = map[string]bool{"add_label": true, "move": true, "trash": true, "remove_label": true, "restore": true}

// OrganizationPolicyRequest is the human policy upsert.
type OrganizationPolicyRequest struct {
	Actions          []string `json:"actions"`
	Prefixes         []string `json:"prefixes,omitempty"`
	MaxItems         int64    `json:"max_items,omitempty"`
	MaxBytes         int64    `json:"max_bytes,omitempty"`
	Enabled          bool     `json:"enabled"`
	ExpiresAt        string   `json:"expires_at,omitempty"`
	ExpectedRevision uint64   `json:"expected_revision,omitempty"`
}

// OrganizationPolicyView is the policy read model.
type OrganizationPolicyView struct {
	PolicyID  string     `json:"policy_id"`
	SpaceID   string     `json:"space_id"`
	Actions   []string   `json:"actions"`
	Prefixes  []string   `json:"prefixes,omitempty"`
	MaxItems  int64      `json:"max_items,omitempty"`
	MaxBytes  int64      `json:"max_bytes,omitempty"`
	Revision  uint64     `json:"revision"`
	Enabled   bool       `json:"enabled"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// SetOrganizationPolicy is the human-only pre-authorization write.
func (s *Service) SetOrganizationPolicy(p Principal, spaceID string, q OrganizationPolicyRequest) (OrganizationPolicyView, error) {
	if p.Agent != nil {
		return OrganizationPolicyView{}, code("forbidden")
	}
	if _, e := s.space(p, spaceID); e != nil {
		return OrganizationPolicyView{}, e
	}
	if len(q.Actions) == 0 || len(q.Actions) > 8 || len(q.Prefixes) > 32 {
		return OrganizationPolicyView{}, code("invalid_request")
	}
	for _, a := range q.Actions {
		if !organizationActions[a] {
			return OrganizationPolicyView{}, code("invalid_request")
		}
	}
	var expires *time.Time
	if q.ExpiresAt != "" {
		t, e := time.Parse(time.RFC3339, q.ExpiresAt)
		if e != nil {
			return OrganizationPolicyView{}, code("invalid_request")
		}
		expires = &t
	}
	var policy OrganizationPolicy
	s.DB.First(&policy, "owner = ? AND space_id = ?", p.dataOwner(), spaceID)
	if policy.ID != "" && q.ExpectedRevision != policy.Revision {
		return OrganizationPolicyView{}, code("version_conflict")
	}
	if policy.ID == "" && q.ExpectedRevision != 0 {
		return OrganizationPolicyView{}, code("version_conflict")
	}
	now := time.Now().UTC()
	if policy.ID == "" {
		policy = OrganizationPolicy{ID: ID(), Owner: p.dataOwner(), SpaceID: spaceID, Revision: 1, Actions: strings.Join(q.Actions, ","), Prefixes: strings.Join(q.Prefixes, ","), MaxItems: q.MaxItems, MaxBytes: q.MaxBytes, Enabled: q.Enabled, ExpiresAt: expires, CreatedAt: now, UpdatedAt: now}
		if e := s.DB.Create(&policy).Error; e != nil {
			return OrganizationPolicyView{}, code("database_unavailable")
		}
	} else {
		if e := s.DB.Model(&OrganizationPolicy{}).Where("id = ?", policy.ID).Updates(map[string]any{"actions": strings.Join(q.Actions, ","), "prefixes": strings.Join(q.Prefixes, ","), "max_items": q.MaxItems, "max_bytes": q.MaxBytes, "enabled": q.Enabled, "expires_at": expires, "revision": policy.Revision + 1, "updated_at": now}).Error; e != nil {
			return OrganizationPolicyView{}, code("database_unavailable")
		}
		s.DB.First(&policy, "id = ?", policy.ID)
	}
	return organizationPolicyView(policy), nil
}

func organizationPolicyView(policy OrganizationPolicy) OrganizationPolicyView {
	split := func(v string) []string {
		if v == "" {
			return nil
		}
		return strings.Split(v, ",")
	}
	return OrganizationPolicyView{PolicyID: policy.ID, SpaceID: policy.SpaceID, Actions: split(policy.Actions), Prefixes: split(policy.Prefixes), MaxItems: policy.MaxItems, MaxBytes: policy.MaxBytes, Revision: policy.Revision, Enabled: policy.Enabled, ExpiresAt: policy.ExpiresAt, UpdatedAt: policy.UpdatedAt}
}

// GetOrganizationPolicy reads one space's policy.
func (s *Service) GetOrganizationPolicy(p Principal, spaceID string) (OrganizationPolicyView, error) {
	if _, e := s.space(p, spaceID); e != nil {
		return OrganizationPolicyView{}, e
	}
	var policy OrganizationPolicy
	if s.DB.First(&policy, "owner = ? AND space_id = ?", p.dataOwner(), spaceID).Error != nil {
		return OrganizationPolicyView{}, code("not_found")
	}
	return organizationPolicyView(policy), nil
}

// OrganizationAction is one typed action in a plan. The label payload rides
// in value and the move destination prefix in target_prefix, matching the
// design contract's {kind, namespace, value} shape.
type OrganizationAction struct {
	Kind         string `json:"kind"`
	Namespace    string `json:"namespace,omitempty"`
	Value        string `json:"value,omitempty"`
	TargetPrefix string `json:"target_prefix,omitempty"` // move destination prefix
}

// CreateOrganizationPlanRequest freezes one organization intent.
type CreateOrganizationPlanRequest struct {
	SourceSnapshotID string               `json:"source_snapshot_id"`
	Refs             []string             `json:"refs,omitempty"`
	Actions          []OrganizationAction `json:"actions"`
	PolicyID         string               `json:"policy_id,omitempty"`
}

// OrganizationPlanView is the plan read model.
type OrganizationPlanView = contract.LabelMergePlanView

// OrganizationPlanItemView is one item's frozen intent and outcome.
type OrganizationPlanItemView = contract.LabelMergePlanItemView

// organizationPolicyCovers reports whether the policy currently
// pre-authorizes the action set over the given paths within its limits.
// policy_scoped is a routing fact, never an authentication bypass: every
// execution still checks scope, expiry and quota.
func organizationPolicyCovers(policy OrganizationPolicy, actions []OrganizationAction, paths []string, totalBytes int64) bool {
	if !policy.Enabled || policy.ExpiresAt != nil && time.Now().UTC().After(*policy.ExpiresAt) {
		return false
	}
	allowed := map[string]bool{}
	for _, a := range strings.Split(policy.Actions, ",") {
		allowed[a] = true
	}
	for _, action := range actions {
		if !allowed[action.Kind] {
			return false
		}
		// Agents may only pre-authorize the rule namespace; manual stays
		// with human confirmation.
		if action.Kind == "add_label" && action.Namespace == "manual" {
			return false
		}
	}
	if policy.MaxItems > 0 && int64(len(paths)) > policy.MaxItems {
		return false
	}
	if policy.MaxBytes > 0 && totalBytes > policy.MaxBytes {
		return false
	}
	if policy.Prefixes != "" {
		for _, path := range paths {
			ok := false
			for _, prefix := range strings.Split(policy.Prefixes, ",") {
				if contains(prefix, path) {
					ok = true
				}
			}
			if !ok {
				return false
			}
		}
	}
	return true
}

// CreateOrganizationPlan freezes the intent under an idempotency key. The
// source is an exact fixed collection: a snapshot or explicit refs;
// semantic Top-K is never executable as a whole.
func (s *Service) CreateOrganizationPlan(p Principal, q CreateOrganizationPlanRequest, key string) (OrganizationPlanView, error) {
	if key == "" || len(key) > 200 || len(q.Actions) == 0 || len(q.Actions) > 16 {
		return OrganizationPlanView{}, code("invalid_request")
	}
	for _, a := range q.Actions {
		if !organizationActions[a.Kind] {
			return OrganizationPlanView{}, code("invalid_request")
		}
		if a.Kind == "add_label" && (a.Namespace == "" || a.Value == "") {
			return OrganizationPlanView{}, code("invalid_request")
		}
		if a.Kind == "move" && a.TargetPrefix == "" {
			return OrganizationPlanView{}, code("invalid_request")
		}
	}
	type member struct {
		fileID, versionID, path string
		size                    int64
		spaceID                 string
	}
	var members []member
	if q.SourceSnapshotID != "" {
		var snapshot CollectionSnapshot
		if s.DB.First(&snapshot, "id = ? AND owner = ?", q.SourceSnapshotID, p.dataOwner()).Error != nil {
			return OrganizationPlanView{}, code("not_found")
		}
		var entries []CollectionSnapshotEntry
		s.DB.Where("snapshot_id = ?", snapshot.ID).Order("created_at").Find(&entries)
		for _, entry := range entries {
			var f File
			if s.DB.First(&f, "id = ?", entry.FileID).Error != nil {
				continue
			}
			members = append(members, member{fileID: f.ID, versionID: entry.VersionID, path: f.Path, size: f.Size, spaceID: f.SpaceID})
		}
	} else {
		for _, ref := range q.Refs {
			fileID, versionID, e := s.resolveRef(ref)
			if e != nil {
				return OrganizationPlanView{}, e
			}
			f, e := s.file(p, fileID, "read")
			if e != nil {
				return OrganizationPlanView{}, e
			}
			members = append(members, member{fileID: f.ID, versionID: versionID, path: f.Path, size: f.Size, spaceID: f.SpaceID})
		}
	}
	if len(members) == 0 || len(members) > 10000 {
		return OrganizationPlanView{}, code("invalid_request")
	}
	spaceID := members[0].spaceID
	totalBytes := int64(0)
	paths := make([]string, 0, len(members))
	for _, m := range members {
		totalBytes += m.size
		paths = append(paths, m.path)
	}
	// Policy routing: matched pre-authorization marks policy_scoped, else
	// the plan waits for explicit human confirmation.
	risk := "approval_required"
	policyID, policyRevision := "", uint64(0)
	if q.PolicyID != "" {
		var policy OrganizationPolicy
		if s.DB.First(&policy, "id = ? AND owner = ?", q.PolicyID, p.dataOwner()).Error == nil && policy.SpaceID == spaceID {
			if organizationPolicyCovers(policy, q.Actions, paths, totalBytes) {
				risk = "policy_scoped"
				policyID = policy.ID
				policyRevision = policy.Revision
			}
		}
	}
	impact, _ := json.Marshal(struct {
		Actions []OrganizationAction `json:"actions"`
		Files   []string             `json:"files"`
	}{q.Actions, paths})
	// plan.Actions keeps the plain actions array: it is the payload
	// decodePlanActions reads back at execution time.
	actionsBody, _ := json.Marshal(q.Actions)
	// The fingerprint normalization lives in one helper shared with
	// LookupOperation: recovery recomputes exactly what creation computed.
	fingerprint := organizationPlanFingerprint(q)
	s.mu.Lock()
	defer s.mu.Unlock()
	var plan OrganizationPlan
	e := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&plan, "owner = ? AND key = ?", p.dataOwner(), key).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		now := time.Now().UTC()
		plan = OrganizationPlan{ID: ID(), Owner: p.dataOwner(), TenantID: p.TenantID, SpaceID: spaceID, SourceSnapshotID: q.SourceSnapshotID, Actions: string(actionsBody), PolicyID: policyID, PolicyRevision: policyRevision, Revision: 1, Phase: "previewed", Risk: risk, ImpactDigest: Hash(string(impact)), ItemsTotal: int64(len(members)), Key: key, Fingerprint: fingerprint, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&plan).Error; err != nil {
			return code("database_unavailable")
		}
		for _, m := range members {
			for _, action := range q.Actions {
				detail := ""
				before, after := "", ""
				switch action.Kind {
				case "add_label":
					detail = action.Namespace + ":" + action.Value
				case "move":
					base := m.path
					if idx := strings.LastIndex(base, "/"); idx >= 0 {
						base = base[idx+1:]
					}
					detail = action.TargetPrefix
					before = m.path
					after = action.TargetPrefix + "/" + base
				case "trash":
					before = m.path
				}
				item := OrganizationPlanItem{ID: ID(), PlanID: plan.ID, ItemKey: m.fileID + "\x00" + action.Kind + "\x00" + detail, Owner: p.dataOwner(), FileID: m.fileID, VersionID: m.versionID, Path: m.path, ActionKind: action.Kind, ActionDetail: detail, BeforeValue: before, AfterValue: after, Status: "pending", CreatedAt: now, UpdatedAt: now}
				if err := tx.Create(&item).Error; err != nil {
					return code("database_unavailable")
				}
			}
		}
		return nil
	})
	if e != nil {
		return OrganizationPlanView{}, e
	}
	if plan.Fingerprint != fingerprint {
		return OrganizationPlanView{}, code("idempotency_conflict")
	}
	return s.OrganizationPlanViewFor(p, plan, "", 0)
}

// organizationPlanFingerprint is the canonical normalization of one plan
// creation: it is the single definition shared by CreateOrganizationPlan
// and LookupOperation so a lost creation response replays onto the same
// identity.
func organizationPlanFingerprint(q CreateOrganizationPlanRequest) string {
	fingerprintSource := struct {
		Snapshot string               `json:"snapshot"`
		Refs     []string             `json:"refs"`
		Actions  []OrganizationAction `json:"actions"`
		Policy   string               `json:"policy"`
	}{q.SourceSnapshotID, q.Refs, q.Actions, q.PolicyID}
	body, _ := json.Marshal(fingerprintSource)
	return Hash(string(body))
}

// OrganizationPlanViewFor builds the paged read model with outcome facts.
func (s *Service) OrganizationPlanViewFor(p Principal, plan OrganizationPlan, cursor string, limit int) (OrganizationPlanView, error) {
	if IsPropertyMigrationPlan(plan) { return OrganizationPlanView{}, code("unsupported_action") }
	if isLabelMergePlan(plan) {
		if err := s.DB.Transaction(func(tx *gorm.DB) error { return s.labelMergePlanAuthorizeOn(tx, p, plan, false) }); err != nil { return OrganizationPlanView{}, err }
	}
	limit = orgPageLimit(limit)
	var items []OrganizationPlanItem
	tx := s.DB.Where("plan_id = ?", plan.ID).Order("id")
	if cursor != "" {
		if isLabelMergePlan(plan) {
			page, err := s.openOrgCursor(cursor, p.UserID, "label-plan:"+p.TenantID+":"+plan.ID, revisionString(plan.Revision))
			if err != nil { return OrganizationPlanView{}, err }
			cursor = page.LastKey
		}
		tx = tx.Where("id > ?", cursor)
	}
	if limit > 0 {
		tx = tx.Limit(limit + 1)
	}
	if e := tx.Find(&items).Error; e != nil {
		return OrganizationPlanView{}, code("database_unavailable")
	}
	view := OrganizationPlanView{PlanID: plan.ID, SpaceID: plan.SpaceID, Revision: plan.Revision, Phase: plan.Phase, Risk: plan.Risk, ApprovalRequired: plan.Risk == "approval_required", PolicyID: plan.PolicyID, PolicyRevision: plan.PolicyRevision, ImpactDigest: plan.ImpactDigest, ItemsTotal: plan.ItemsTotal, CompensationOf: plan.CompensationOf, Items: []OrganizationPlanItemView{}}
	if err := s.DB.Model(&OrganizationPlanItem{}).Where("plan_id = ? AND status = ?", plan.ID, "done").Count(&view.ItemsDone).Error; err != nil { return OrganizationPlanView{}, code("database_unavailable") }
	if err := s.DB.Model(&OrganizationPlanItem{}).Where("plan_id = ? AND status = ?", plan.ID, "stale").Count(&view.ItemsStale).Error; err != nil { return OrganizationPlanView{}, code("database_unavailable") }
	if err := s.DB.Model(&OrganizationPlanItem{}).Where("plan_id = ? AND status = ?", plan.ID, "failed").Count(&view.ItemsFailed).Error; err != nil { return OrganizationPlanView{}, code("database_unavailable") }
	more := limit > 0 && len(items) > limit
	if more {
		items = items[:limit]
	}
	for _, item := range items {
		view.Items = append(view.Items, OrganizationPlanItemView{ItemKey: item.ItemKey, FileID: item.FileID, VersionID: item.VersionID, Path: item.Path, Action: item.ActionKind, ActionDetail: item.ActionDetail, Before: item.BeforeValue, After: item.AfterValue, Status: item.Status, Error: item.Error, ResultVersion: item.ResultVersion})
	}
	if more && len(items) > 0 {
		view.NextCursor = items[len(items)-1].ID
		if isLabelMergePlan(plan) {
			var err error
			view.NextCursor, err = s.sealOrgCursor(orgCursorPayload{Owner: p.UserID, Query: "label-plan:"+p.TenantID+":"+plan.ID, CatalogueRevision: revisionString(plan.Revision), LastKey: view.NextCursor, Kind: "label-plan"})
			if err != nil { return OrganizationPlanView{}, err }
		}
	}
	return view, nil
}

// GetOrganizationPlan reads one plan with paged item impacts. Agents see
// only the items inside their own prefix scope.
func (s *Service) GetOrganizationPlan(p Principal, planID, cursor string, limit int) (OrganizationPlanView, error) {
	var plan OrganizationPlan
	if err := s.DB.First(&plan, "id = ? AND owner = ?", planID, p.dataOwner()).Error; err != nil {
		return OrganizationPlanView{}, resourceReadError(err)
	}
	view, e := s.OrganizationPlanViewFor(p, plan, cursor, limit)
	if e != nil {
		return OrganizationPlanView{}, e
	}
	if p.Agent != nil {
		filtered := make([]OrganizationPlanItemView, 0, len(view.Items))
		for _, item := range view.Items {
			if contains(p.Agent.Prefix, item.Path) {
				filtered = append(filtered, item)
			}
		}
		view.Items = filtered
	}
	return view, nil
}

// ConfirmOrganizationPlanRequest carries the human confirmation.
type ConfirmOrganizationPlanRequest = contract.LabelMergeConfirm

// ConfirmOrganizationPlan is the human confirmation. It binds the exact
// plan revision and impact digest; confirmation never expands any file
// permission.
func (s *Service) ConfirmOrganizationPlan(p Principal, planID string, q ConfirmOrganizationPlanRequest) (OrganizationPlanView, error) {
	if p.Agent != nil {
		return OrganizationPlanView{}, code("forbidden")
	}
	var plan OrganizationPlan
	if err := s.DB.First(&plan, "id = ? AND owner = ?", planID, p.dataOwner()).Error; err != nil {
		return OrganizationPlanView{}, resourceReadError(err)
	}
	if IsPropertyMigrationPlan(plan) { return OrganizationPlanView{}, code("unsupported_action") }
	if isLabelMergePlan(plan) { return s.confirmLabelMergePlan(p, plan, q) }
	if q.ExpectedRevision != plan.Revision {
		return OrganizationPlanView{}, code("plan_stale")
	}
	if q.ImpactDigest != plan.ImpactDigest {
		return OrganizationPlanView{}, code("plan_stale")
	}
	if plan.Phase != "previewed" {
		return OrganizationPlanView{}, code("plan_stale")
	}
	s.DB.Model(&OrganizationPlan{}).Where("id = ?", plan.ID).Updates(map[string]any{"phase": "confirmed", "confirmed_by": p.UserID, "revision": gorm.Expr("revision + 1"), "updated_at": time.Now().UTC()})
	s.DB.First(&plan, "id = ?", plan.ID)
	return s.OrganizationPlanViewFor(p, plan, "", 0)
}

// ExecuteOrganizationPlanRequest carries the optimistic revision.
type ExecuteOrganizationPlanRequest = contract.LabelMergeExecute

// ExecuteOrganizationPlan runs the plan: only a human confirmation or a
// currently-valid pre-authorization opens execution; every item write
// re-checks permission, source version and policy scope; drift marks the
// single item stale without touching approved results elsewhere.
func (s *Service) ExecuteOrganizationPlan(p Principal, planID string, q ExecuteOrganizationPlanRequest) (OrganizationPlanView, error) {
	var plan OrganizationPlan
	if err := s.DB.First(&plan, "id = ? AND owner = ?", planID, p.dataOwner()).Error; err != nil {
		return OrganizationPlanView{}, resourceReadError(err)
	}
	if IsPropertyMigrationPlan(plan) { return OrganizationPlanView{}, code("unsupported_action") }
	labelMerge := isLabelMergePlan(plan)
	if labelMerge {
		if err := s.DB.Transaction(func(tx *gorm.DB) error { return s.labelMergePlanAuthorizeOn(tx, p, plan, true) }); err != nil { return OrganizationPlanView{}, err }
	}
	if q.ExpectedRevision != plan.Revision {
		return OrganizationPlanView{}, code("plan_stale")
	}
	if plan.Phase != "confirmed" && plan.Phase != "partial" && !(labelMerge && plan.Phase == "executing") && !(plan.Phase == "previewed" && plan.Risk == "policy_scoped") {
		if plan.Risk == "approval_required" {
			return OrganizationPlanView{}, code("approval_required")
		}
		return OrganizationPlanView{}, code("plan_stale")
	}
	// Re-check the pre-authorization at execution time: a revoked or
	// narrowed policy stops the not-yet-executed items.
	if plan.PolicyID != "" {
		var policy OrganizationPolicy
		if s.DB.First(&policy, "id = ?", plan.PolicyID).Error != nil || !organizationPolicyCovers(policy, decodePlanActions(plan.Actions), planItemPaths(s, plan.ID), 0) {
			s.DB.Model(&OrganizationPlanItem{}).Where("plan_id = ? AND status = ?", plan.ID, "pending").Updates(map[string]any{"status": "stale", "error": "policy_revoked", "updated_at": time.Now().UTC()})
			s.DB.Model(&OrganizationPlan{}).Where("id = ?", plan.ID).Updates(map[string]any{"phase": "partial", "updated_at": time.Now().UTC()})
			return OrganizationPlanView{}, code("policy_revoked")
		}
	}
	claim := s.DB.Model(&OrganizationPlan{}).Where("id = ? AND revision = ? AND phase = ?", plan.ID, plan.Revision, plan.Phase).Updates(map[string]any{"phase": "executing", "updated_at": time.Now().UTC()})
	if claim.Error != nil { return OrganizationPlanView{}, code("database_unavailable") }
	if claim.RowsAffected != 1 { return OrganizationPlanView{}, code("plan_stale") }
	var items []OrganizationPlanItem
	itemQuery := s.DB.Where("plan_id = ? AND status = ?", plan.ID, "pending").Order("created_at, id")
	if labelMerge { itemQuery = itemQuery.Limit(201) }
	if err := itemQuery.Find(&items).Error; err != nil { return OrganizationPlanView{}, code("database_unavailable") }
	if labelMerge && len(items) > 200 { return OrganizationPlanView{}, code("query_budget_exceeded") }
	now := time.Now().UTC()
	for _, item := range items {
		if labelMerge {
			if err := s.executeLabelMergeItem(p, plan, item); err != nil { return OrganizationPlanView{}, err }
			continue
		}
		var f File
		if s.DB.First(&f, "id = ?", item.FileID).Error != nil || f.VersionID != item.VersionID || f.Trashed && item.ActionKind != "restore" {
			s.DB.Model(&OrganizationPlanItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "stale", "error": "source_changed", "updated_at": now})
			continue
		}
		if e := s.allow(p, f.SpaceID, f.Path, "write"); e != nil {
			s.DB.Model(&OrganizationPlanItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "failed", "error": safeCode(e), "updated_at": now})
			continue
		}
		switch item.ActionKind {
		case "add_label":
			namespace := "rule"
			label := item.ActionDetail
			if idx := strings.Index(label, ":"); idx >= 0 {
				namespace, label = label[:idx], label[idx+1:]
			}
			if p.Agent != nil && namespace != "rule" && namespace != "suggested" {
				s.DB.Model(&OrganizationPlanItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "failed", "error": "label_namespace_forbidden", "updated_at": now})
				continue
			}
			if e := s.organizationLabelMutation(p, item, namespace, label, false, now); e != nil {
				s.DB.Model(&OrganizationPlanItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "failed", "error": safeCode(e), "updated_at": now})
				continue
			}
		case "remove_label":
			namespace, label := "rule", item.ActionDetail
			if idx := strings.Index(label, ":"); idx >= 0 {
				namespace, label = label[:idx], label[idx+1:]
			}
			if e := s.organizationLabelMutation(p, item, namespace, label, true, now); e != nil {
				s.DB.Model(&OrganizationPlanItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "failed", "error": safeCode(e), "updated_at": now})
				continue
			}
		case "move":
			target := item.AfterValue
			// The destination is a fresh write scope: clean it and authorize
			// it explicitly so a pre-authorized prefix cannot move files
			// into paths the caller may not write.
			if _, e := cleanPath(target); e != nil {
				s.DB.Model(&OrganizationPlanItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "failed", "error": safeCode(e), "updated_at": now})
				continue
			}
			if e := s.allow(p, f.SpaceID, target, "write"); e != nil {
				s.DB.Model(&OrganizationPlanItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "failed", "error": safeCode(e), "updated_at": now})
				continue
			}
			if plan.PolicyID != "" {
				var movePolicy OrganizationPolicy
				if s.DB.First(&movePolicy, "id = ?", plan.PolicyID).Error == nil {
					if !organizationPolicyCovers(movePolicy, []OrganizationAction{{Kind: "move", TargetPrefix: target}}, []string{f.Path, target}, f.Size) {
						s.DB.Model(&OrganizationPlanItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "failed", "error": "policy_revoked", "updated_at": now})
						continue
					}
				}
			}
			var occupied int64
			s.DB.Model(&File{}).Where("space_id = ? AND path = ?", f.SpaceID, target).Count(&occupied)
			if occupied > 0 {
				s.DB.Model(&OrganizationPlanItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "failed", "error": "name_conflict", "updated_at": now})
				continue
			}
			if e := s.DB.Model(&File{}).Where("id = ?", f.ID).Updates(map[string]any{"path": target, "updated": now}).Error; e != nil {
				s.DB.Model(&OrganizationPlanItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "failed", "error": "database_unavailable", "updated_at": now})
				continue
			}
		case "trash":
			// Soft deletes go through the canonical transactional path so
			// the catalogue event joins the same commit — projections and
			// subscriptions observe organized files exactly like manual
			// ones.
			if e := s.DB.Transaction(func(tx *gorm.DB) error {
				rev := fileRevision(now)
				if err := tx.Model(&File{}).Where("id = ?", f.ID).Updates(map[string]any{"trashed": true, "trashed_at": now, "updated": now}).Error; err != nil {
					return err
				}
				return s.recordFileEvent(tx, p.dataOwner(), f.ID, rev, true, map[string]any{"path": f.Path, "trashed": true})
			}); e != nil {
				s.DB.Model(&OrganizationPlanItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "failed", "error": "database_unavailable", "updated_at": now})
				continue
			}
		case "restore":
			if e := s.DB.Transaction(func(tx *gorm.DB) error {
				rev := fileRevision(now)
				if err := tx.Model(&File{}).Where("id = ?", f.ID).Updates(map[string]any{"trashed": false, "trashed_at": time.Time{}, "updated": now}).Error; err != nil {
					return err
				}
				return s.recordFileEvent(tx, p.dataOwner(), f.ID, rev, false, map[string]any{"path": f.Path, "restored": true})
			}); e != nil {
				s.DB.Model(&OrganizationPlanItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "failed", "error": "database_unavailable", "updated_at": now})
				continue
			}
		}
		s.DB.Model(&OrganizationPlanItem{}).Where("id = ?", item.ID).Updates(map[string]any{"status": "done", "result_version": f.VersionID, "updated_at": now})
	}
	var pending int64
	if err := s.DB.Model(&OrganizationPlanItem{}).Where("plan_id = ? AND status IN ?", plan.ID, []string{"pending", "stale", "failed"}).Count(&pending).Error; err != nil { return OrganizationPlanView{}, code("database_unavailable") }
	phase := "completed"
	if pending > 0 {
		phase = "partial"
	}
	finished := s.DB.Model(&OrganizationPlan{}).Where("id = ? AND revision = ? AND phase = ?", plan.ID, plan.Revision, "executing").Updates(map[string]any{"phase": phase, "revision": gorm.Expr("revision + 1"), "updated_at": now})
	if finished.Error != nil { return OrganizationPlanView{}, code("database_unavailable") }
	if finished.RowsAffected != 1 { return OrganizationPlanView{}, code("plan_stale") }
	var fresh OrganizationPlan
	if err := s.DB.First(&fresh, "id = ?", plan.ID).Error; err != nil { return OrganizationPlanView{}, resourceReadError(err) }
	return s.OrganizationPlanViewFor(p, fresh, "", 0)
}

func decodePlanActions(raw string) []OrganizationAction {
	var actions []OrganizationAction
	json.Unmarshal([]byte(raw), &actions)
	return actions
}

func planItemPaths(s *Service, planID string) []string {
	var items []OrganizationPlanItem
	s.DB.Where("plan_id = ?", planID).Find(&items)
	paths := make([]string, 0, len(items))
	seen := map[string]bool{}
	for _, item := range items {
		if !seen[item.FileID] {
			seen[item.FileID] = true
			paths = append(paths, item.Path)
		}
	}
	return paths
}

// CreateCompensationPlan derives the reverse intent from a completed
// plan's recorded results. It never executes immediately and only covers
// metadata that still matches this run's results; later modifications
// surface as compensation_conflict at execution time.
func (s *Service) CreateCompensationPlan(p Principal, planID string, key string) (OrganizationPlanView, error) {
	if key == "" || len(key) > 200 {
		return OrganizationPlanView{}, code("invalid_request")
	}
	var plan OrganizationPlan
	if err := s.DB.First(&plan, "id = ? AND owner = ?", planID, p.dataOwner()).Error; err != nil {
		return OrganizationPlanView{}, resourceReadError(err)
	}
	if IsPropertyMigrationPlan(plan) || isLabelMergePlan(plan) { return OrganizationPlanView{}, code("unsupported_action") }
	// Compensation only covers results that still match this run: a source
	// whose current version differs from the recorded result version is a
	// conflict for manual resolution, never an overwritten later edit.
	var done []OrganizationPlanItem
	s.DB.Where("plan_id = ? AND status = ?", plan.ID, "done").Find(&done)
	for _, item := range done {
		var f File
		if s.DB.First(&f, "id = ?", item.FileID).Error != nil || f.VersionID != item.VersionID {
			// The source moved past the version this run organized: the
			// later edit is a conflict for manual resolution.
			return OrganizationPlanView{}, code("compensation_conflict")
		}
	}
	if len(done) == 0 {
		return OrganizationPlanView{}, code("invalid_request")
	}
	reverse := map[string]OrganizationAction{}
	for _, item := range done {
		switch item.ActionKind {
		case "add_label":
			reverse["remove:"+item.ActionDetail] = OrganizationAction{Kind: "remove_label", Namespace: strings.SplitN(item.ActionDetail, ":", 2)[0], Value: strings.SplitN(item.ActionDetail, ":", 2)[1]}
		case "trash":
			reverse["restore:"+item.FileID] = OrganizationAction{Kind: "restore"}
		case "move":
			// Reverse moves stay manual: path reversals depend on current
			// tree state and are not derivable as a blanket rule.
		}
	}
	actions := make([]OrganizationAction, 0, len(reverse))
	for _, a := range reverse {
		actions = append(actions, a)
	}
	if len(actions) == 0 {
		return OrganizationPlanView{}, code("invalid_request")
	}
	refs := make([]string, 0, len(done))
	seen := map[string]bool{}
	for _, item := range done {
		if !seen[item.FileID] {
			seen[item.FileID] = true
			refs = append(refs, "drivebridge://"+s.Instance+"/file/"+item.FileID+"/version/"+item.ResultVersion)
		}
	}
	view, e := s.CreateOrganizationPlan(p, CreateOrganizationPlanRequest{Refs: refs, Actions: actions}, key)
	if e != nil {
		return OrganizationPlanView{}, e
	}
	s.DB.Model(&OrganizationPlan{}).Where("id = ?", view.PlanID).Update("compensation_of", plan.ID)
	view.CompensationOf = plan.ID
	return view, nil
}
