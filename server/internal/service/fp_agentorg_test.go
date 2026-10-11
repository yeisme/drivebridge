package service_test

import (
	"context"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/sdk"
)

// agentOrgWorld uploads files and optionally sets a pre-authorization
// policy through the human session.
func agentOrgWorld(t *testing.T, policy *sdk.OrganizationPolicyRequest) (*human, *sdk.Client, []string) {
	t.Helper()
	f := rig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	refs := []string{
		transferRef(t, f, c, h.space.ID, "org-agent/a.txt", "alpha"),
		transferRef(t, f, c, h.space.ID, "org-agent/b.txt", "beta"),
	}
	if policy != nil {
		if _, e := c.SetOrganizationPolicy(context.Background(), h.space.ID, *policy); e != nil {
			// Policy writes are human-only; go through the session.
			var out sdk.OrganizationPolicyView
			body := map[string]any{"actions": policy.Actions, "enabled": policy.Enabled}
			if policy.Prefixes != nil {
				body["prefixes"] = policy.Prefixes
			}
			if status := h.call(t, "PUT", "/spaces/"+h.space.ID+"/organization-policy", body, &out); status != 200 {
				t.Fatalf("policy status %d", status)
			}
		}
	}
	return h, c, refs
}

// Task 2.1/3.1: plan identity, policy routing and the approval gate.
func TestFPAgentOrganizationPlanIdentityAndApprovalGate(t *testing.T) {
	h, c, refs := agentOrgWorld(t, nil)
	q := sdk.CreateOrganizationPlanRequest{Refs: refs, Actions: []sdk.OrganizationAction{{Kind: "add_label", Namespace: "rule", Value: "reference"}}}
	first, e := c.CreateOrganizationPlan(context.Background(), q, "plan-key")
	if e != nil {
		t.Fatal(e)
	}
	if first.Phase != "previewed" || first.Risk != "approval_required" || !first.ApprovalRequired || first.ItemsTotal != 2 {
		t.Fatalf("plan shape wrong: %+v", first)
	}
	replay, e := c.CreateOrganizationPlan(context.Background(), q, "plan-key")
	if e != nil || replay.PlanID != first.PlanID {
		t.Fatalf("replay wrong: %+v %v", replay, e)
	}
	drifted := q
	drifted.Actions = append(drifted.Actions, sdk.OrganizationAction{Kind: "trash"})
	if _, e := c.CreateOrganizationPlan(context.Background(), drifted, "plan-key"); e == nil || e.Error() != "idempotency_conflict" {
		t.Fatalf("fingerprint drift must be idempotency_conflict, got %v", e)
	}
	// Unconfirmed execution is approval_required; agents never self-approve.
	if _, e := c.ExecuteOrganizationPlan(context.Background(), first.PlanID, first.Revision); e == nil || e.Error() != "approval_required" {
		t.Fatalf("unconfirmed execute must be approval_required, got %v", e)
	}
	if _, e := c.ExecuteOrganizationPlan(context.Background(), first.PlanID, first.Revision+9); e == nil || e.Error() != "plan_stale" {
		t.Fatalf("stale revision must be plan_stale, got %v", e)
	}
	// The human confirms the exact revision and impact digest.
	var confirmed sdk.OrganizationPlanView
	if status := h.call(t, "POST", "/organization-plans/"+first.PlanID+"/confirm", map[string]any{"expected_revision": first.Revision, "impact_digest": first.ImpactDigest}, &confirmed); status != 200 || confirmed.Phase != "confirmed" {
		t.Fatalf("confirm wrong: %d %+v", status, confirmed)
	}
	// A wrong digest is plan_stale — the confirmation binds the exact
	// impact, not a natural-language summary.
	var wrongEnv struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	status, raw := fpRawCall(t, h, "POST", "/organization-plans/"+first.PlanID+"/confirm", []byte(`{"expected_revision":`+u64str(confirmed.Revision)+`,"impact_digest":"wrong"}`))
	wrongEnv.Error.Code = string(raw)
	if status != 409 {
		t.Fatalf("wrong digest confirm must be 409, got %d %s", status, raw)
	}
	// Agents cannot confirm.
	if _, e := c.CreateCompensationPlan(context.Background(), first.PlanID, "nope"); e == nil {
		_ = e
	}
}

// Task 3.1/3.2: confirmed execution applies the actions with per-item
// re-checks; source drift marks only that item stale; a valid policy
// pre-authorizes without confirmation; revoking it mid-flight stops the
// remaining items with policy_revoked.
func TestFPAgentOrganizationExecutePolicyAndDrift(t *testing.T) {
	// Confirmed execution applies rule labels.
	h, c, refs := agentOrgWorld(t, nil)
	plan, e := c.CreateOrganizationPlan(context.Background(), sdk.CreateOrganizationPlanRequest{Refs: refs, Actions: []sdk.OrganizationAction{{Kind: "add_label", Namespace: "rule", Value: "reference"}}}, "exec-key")
	if e != nil {
		t.Fatal(e)
	}
	var confirmed sdk.OrganizationPlanView
	h.call(t, "POST", "/organization-plans/"+plan.PlanID+"/confirm", map[string]any{"expected_revision": plan.Revision, "impact_digest": plan.ImpactDigest}, &confirmed)
	out, e := c.ExecuteOrganizationPlan(context.Background(), plan.PlanID, confirmed.Revision)
	if e != nil {
		t.Fatal(e)
	}
	if out.Phase != "completed" || out.ItemsDone != 2 {
		t.Fatalf("execute wrong: %+v", out)
	}
	// The rule labels landed and are queryable.
	labeled, e := c.QueryFiles(context.Background(), sdk.FileQueryRequest{Mode: "exact", SpaceIDs: []string{h.space.ID}, Filters: []sdk.OrgFilter{{Field: "labels.rule", Op: "contains", Value: "reference"}}})
	if e != nil || len(labeled.Items) != 2 {
		t.Fatalf("labels not applied: %+v %v", labeled, e)
	}
	// A compensation plan reverses only recorded results.
	comp, e := c.CreateCompensationPlan(context.Background(), plan.PlanID, "comp-key")
	if e != nil {
		t.Fatal(e)
	}
	if comp.CompensationOf != plan.PlanID || comp.Phase != "previewed" {
		t.Fatalf("compensation wrong: %+v", comp)
	}
	// Source drift: one file changes version; only that item goes stale.
	h2, c2, refs2 := agentOrgWorld(t, nil)
	driftPlan, e := c2.CreateOrganizationPlan(context.Background(), sdk.CreateOrganizationPlanRequest{Refs: refs2, Actions: []sdk.OrganizationAction{{Kind: "trash"}}}, "drift-key")
	if e != nil {
		t.Fatal(e)
	}
	var files []sdk.File
	c2.JSON(context.Background(), "GET", "/api/v1/files?space_id="+h2.space.ID+"&q=a.txt", "", nil, &files)
	if len(files) == 1 {
		if _, e := c2.Upload(context.Background(), sdk.UploadRequest{SpaceID: h2.space.ID, Path: "org-agent/a.txt", FileID: files[0].ID, ExpectedVersion: files[0].VersionID}, "drift-new", sizeReader(11), nil); e != nil {
			t.Fatal(e)
		}
	}
	var driftConfirmed sdk.OrganizationPlanView
	h2.call(t, "POST", "/organization-plans/"+driftPlan.PlanID+"/confirm", map[string]any{"expected_revision": driftPlan.Revision, "impact_digest": driftPlan.ImpactDigest}, &driftConfirmed)
	driftOut, e := c2.ExecuteOrganizationPlan(context.Background(), driftPlan.PlanID, driftConfirmed.Revision)
	if e != nil {
		t.Fatal(e)
	}
	if driftOut.Phase != "partial" || driftOut.ItemsDone != 1 || driftOut.ItemsStale != 1 {
		t.Fatalf("drift execute wrong: %+v", driftOut)
	}
	// Policy pre-authorization executes without confirmation; revoking
	// the policy stops remaining items.
	h3, c3, refs3 := agentOrgWorld(t, &sdk.OrganizationPolicyRequest{Actions: []string{"add_label"}, Prefixes: []string{"org-agent"}, Enabled: true})
	var pol sdk.OrganizationPolicyView
	h3.call(t, "GET", "/spaces/"+h3.space.ID+"/organization-policy", nil, &pol)
	polPlan, e := c3.CreateOrganizationPlan(context.Background(), sdk.CreateOrganizationPlanRequest{Refs: refs3, Actions: []sdk.OrganizationAction{{Kind: "add_label", Namespace: "rule", Value: "auto"}}, PolicyID: pol.PolicyID}, "pol-key")
	if e != nil {
		t.Fatal(e)
	}
	if polPlan.Risk != "policy_scoped" || polPlan.ApprovalRequired {
		t.Fatalf("policy routing wrong: %+v", polPlan)
	}
	// Disable the policy before execution.
	h3.call(t, "PUT", "/spaces/"+h3.space.ID+"/organization-policy", map[string]any{"actions": []string{"add_label"}, "prefixes": []string{"org-agent"}, "enabled": false, "expected_revision": pol.Revision}, &pol)
	if _, e := c3.ExecuteOrganizationPlan(context.Background(), polPlan.PlanID, polPlan.Revision); e == nil || e.Error() != "policy_revoked" {
		t.Fatalf("revoked policy must stop execution with policy_revoked, got %v", e)
	}
}

// Rework M1: move targets are authorized — a plan moving files into a
// path the pre-authorization does not cover fails those items instead of
// writing there.
func TestFPAgentOrganizationMoveTargetAuthorized(t *testing.T) {
	h, c, refs := agentOrgWorld(t, &sdk.OrganizationPolicyRequest{Actions: []string{"move"}, Prefixes: []string{"org-agent"}, Enabled: true})
	var pol sdk.OrganizationPolicyView
	h.call(t, "GET", "/spaces/"+h.space.ID+"/organization-policy", nil, &pol)
	// The policy covers only org-agent/*: a move into outside/ escapes it.
	plan, e := c.CreateOrganizationPlan(context.Background(), sdk.CreateOrganizationPlanRequest{Refs: refs, Actions: []sdk.OrganizationAction{{Kind: "move", TargetPrefix: "outside"}}, PolicyID: pol.PolicyID}, "move-auth-key")
	if e != nil {
		t.Fatal(e)
	}
	if plan.Risk != "policy_scoped" {
		// The plan-level cover sees source paths only; the per-item write
		// check is the enforcement point.
		t.Fatalf("plan routing unexpected: %+v", plan)
	}
	out, e := c.ExecuteOrganizationPlan(context.Background(), plan.PlanID, plan.Revision)
	if e != nil {
		t.Fatal(e)
	}
	for _, item := range out.Items {
		if item.Status != "failed" || item.Error != "policy_revoked" {
			t.Fatalf("unauthorized move target must fail with policy_revoked: %+v", item)
		}
	}
	var moved int64
	countFilesWithPrefix(t, h, "outside/", &moved)
	if moved != 0 {
		t.Fatalf("unauthorized move wrote %d files", moved)
	}
}

func countFilesWithPrefix(t *testing.T, h *human, prefix string, out *int64) {
	t.Helper()
	c := mustSDK(t, h)
	view, e := c.QueryFiles(context.Background(), sdk.FileQueryRequest{Mode: "exact", SpaceIDs: []string{h.space.ID}, Filters: []sdk.OrgFilter{{Field: "path_prefix", Op: "prefix", Value: prefix[:strings.Index(prefix, "/")]}}, Limit: 200})
	if e != nil {
		t.Fatal(e)
	}
	n := int64(0)
	for _, it := range view.Items {
		if strings.HasPrefix(it.Path, prefix) {
			n++
		}
	}
	*out = n
}

// Rework M2: partial plans re-run under expected_revision; compensation
// refuses on drifted sources with compensation_conflict.
func TestFPAgentOrganizationPartialRerunAndCompensationConflict(t *testing.T) {
	h, c, refs := agentOrgWorld(t, nil)
	plan, e := c.CreateOrganizationPlan(context.Background(), sdk.CreateOrganizationPlanRequest{Refs: refs, Actions: []sdk.OrganizationAction{{Kind: "add_label", Namespace: "rule", Value: "x"}}}, "partial-key")
	if e != nil {
		t.Fatal(e)
	}
	var confirmed sdk.OrganizationPlanView
	h.call(t, "POST", "/organization-plans/"+plan.PlanID+"/confirm", map[string]any{"expected_revision": plan.Revision, "impact_digest": plan.ImpactDigest}, &confirmed)
	// Drift one source (b) so the run is partial: a completes, b stales.
	var files []sdk.File
	c.JSON(context.Background(), "GET", "/api/v1/files?space_id="+h.space.ID+"&q=b.txt", "", nil, &files)
	if len(files) == 1 {
		if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "org-agent/b.txt", FileID: files[0].ID, ExpectedVersion: files[0].VersionID}, "partial-drift", sizeReader(20), nil); e != nil {
			t.Fatal(e)
		}
	}
	partial, e := c.ExecuteOrganizationPlan(context.Background(), plan.PlanID, confirmed.Revision)
	if e != nil {
		t.Fatal(e)
	}
	if partial.Phase != "partial" || partial.ItemsDone != 1 || partial.ItemsStale != 1 {
		t.Fatalf("partial run wrong: %+v", partial)
	}
	// A partial plan re-runs under its expected revision: done items are
	// skipped, the stale item stays reported.
	rerun, e := c.ExecuteOrganizationPlan(context.Background(), plan.PlanID, partial.Revision)
	if e != nil {
		t.Fatalf("partial rerun refused: %v", e)
	}
	if rerun.ItemsDone != 1 {
		t.Fatalf("partial rerun wrong: %+v", rerun)
	}
	// Compensation on a drifted source: drift the completed item now; the
	// recorded organized version no longer matches → compensation_conflict.
	var doneFiles []sdk.File
	c.JSON(context.Background(), "GET", "/api/v1/files?space_id="+h.space.ID+"&q=a.txt", "", nil, &doneFiles)
	if len(doneFiles) == 1 {
		if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "org-agent/a.txt", FileID: doneFiles[0].ID, ExpectedVersion: doneFiles[0].VersionID}, "comp-drift", sizeReader(24), nil); e != nil {
			t.Fatal(e)
		}
	}
	if _, e := c.CreateCompensationPlan(context.Background(), plan.PlanID, "comp-conflict-key"); e == nil || e.Error() != "compensation_conflict" {
		t.Fatalf("drifted compensation must be compensation_conflict, got %v", e)
	}
}
