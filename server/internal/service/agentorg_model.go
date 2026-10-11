package service

import "time"

// OrganizationPolicy is the human-managed pre-authorization for agent
// organization actions: which actions, in which prefixes, under which
// batch and byte limits, valid until when. Agents never write policies.
type OrganizationPolicy struct {
	ID        string     `json:"policy_id" gorm:"primaryKey"`
	Owner     string     `json:"-" gorm:"uniqueIndex:organization_policy_space"`
	SpaceID   string     `json:"space_id" gorm:"uniqueIndex:organization_policy_space"`
	Actions   string     `json:"actions"` // comma list: add_label,move,trash
	Prefixes  string     `json:"prefixes,omitempty"`
	MaxItems  int64      `json:"max_items,omitempty"`
	MaxBytes  int64      `json:"max_bytes,omitempty"`
	Revision  uint64     `json:"revision"`
	Enabled   bool       `json:"enabled"`
	ExpiresAt *time.Time `json:"expires_at,omitempty"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
}

func (OrganizationPolicy) TableName() string { return "organization_policies" }

// OrganizationPlan freezes one organization intent: an exact source
// snapshot, a typed action list, the policy it claims, its impact digest
// and the approval state. Execution re-checks everything per item.
type OrganizationPlan struct {
	ID               string    `json:"plan_id" gorm:"primaryKey"`
	Owner            string    `json:"-" gorm:"index"`
	TenantID         string    `json:"tenant_id,omitempty"`
	SpaceID          string    `json:"space_id"`
	SourceSnapshotID string    `json:"source_snapshot_id,omitempty"`
	Actions          string    `json:"actions"`
	PolicyID         string    `json:"policy_id,omitempty"`
	PolicyRevision   uint64    `json:"policy_revision,omitempty"`
	Revision         uint64    `json:"revision"`
	Phase            string    `json:"phase"` // previewed|confirmed|executing|completed|partial|cancelled
	Risk             string    `json:"risk"`  // policy_scoped|approval_required
	ImpactDigest     string    `json:"impact_digest"`
	ItemsTotal       int64     `json:"items_total"`
	CompensationOf   string    `json:"compensation_of,omitempty"`
	ConfirmedBy      string    `json:"-"`
	Key              string    `json:"-" gorm:"uniqueIndex:organization_plan_key"`
	Fingerprint      string    `json:"-"`
	CreatedAt        time.Time `json:"created_at"`
	UpdatedAt        time.Time `json:"updated_at"`
}

func (OrganizationPlan) TableName() string { return "organization_plans" }

// OrganizationPlanItem is one frozen plan member with its outcome.
type OrganizationPlanItem struct {
	ID            string    `gorm:"primaryKey"`
	PlanID        string    `json:"plan_id" gorm:"index;uniqueIndex:organization_item_identity"`
	ItemKey       string    `json:"item_key" gorm:"uniqueIndex:organization_item_identity"`
	Owner         string    `json:"-"`
	FileID        string    `json:"file_id"`
	VersionID     string    `json:"version_id"`
	Path          string    `json:"path"`
	ActionKind    string    `json:"action"`
	ActionDetail  string    `json:"action_detail,omitempty"`
	BeforeValue   string    `json:"before,omitempty"`
	AfterValue    string    `json:"after,omitempty"`
	Status        string    `json:"status"` // pending|executing|done|stale|failed|cancelled
	Error         string    `json:"error,omitempty"`
	ResultVersion string    `json:"result_version,omitempty"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func (OrganizationPlanItem) TableName() string { return "organization_plan_items" }
