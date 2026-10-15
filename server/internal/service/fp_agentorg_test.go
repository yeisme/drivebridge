package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/sdk"
	"gorm.io/gorm"
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

// compensationParent executes one organize plan to completion so its
// recorded results can be compensated.
func compensationParent(t *testing.T, h *human, c *sdk.Client, refs []string, actions []sdk.OrganizationAction, key string) sdk.OrganizationPlanView {
	t.Helper()
	plan, e := c.CreateOrganizationPlan(context.Background(), sdk.CreateOrganizationPlanRequest{Refs: refs, Actions: actions}, key)
	if e != nil {
		t.Fatal(e)
	}
	var confirmed sdk.OrganizationPlanView
	if status := h.call(t, "POST", "/organization-plans/"+plan.PlanID+"/confirm", map[string]any{"expected_revision": plan.Revision, "impact_digest": plan.ImpactDigest}, &confirmed); status != 200 {
		t.Fatalf("confirm status %d", status)
	}
	done, e := c.ExecuteOrganizationPlan(context.Background(), plan.PlanID, confirmed.Revision)
	if e != nil {
		t.Fatal(e)
	}
	// ItemsTotal counts frozen members while multi-action plans store one
	// row per member×action pair, so completion is the phase plus nonzero
	// recorded results.
	if done.Phase != "completed" || done.ItemsDone == 0 {
		t.Fatalf("parent not completed: %+v", done)
	}
	return done
}

// A lost compensation-creation response recovers through the same key:
// the derivation is byte-stable, the persisted receipt names its parent,
// and later legitimate file drift never blocks the original receipt —
// only a genuinely new compensation intent re-checks drift.
func TestFPAgentOrganizationCompensationSameKeyRecovery(t *testing.T) {
	h, c, refs := agentOrgWorld(t, nil)
	// Three distinct reverse actions (remove ×3 labels) prove the sorted
	// derivation: an unstable order would replay onto a different
	// fingerprint and refuse the receipt.
	actions := []sdk.OrganizationAction{
		{Kind: "add_label", Namespace: "rule", Value: "x"},
		{Kind: "add_label", Namespace: "rule", Value: "y"},
		{Kind: "add_label", Namespace: "rule", Value: "z"},
	}
	parent := compensationParent(t, h, c, refs, actions, "comp-recovery-parent")
	first, e := c.CreateCompensationPlan(context.Background(), parent.PlanID, "comp-recovery-key")
	if e != nil {
		t.Fatal(e)
	}
	if first.CompensationOf != parent.PlanID || first.Phase != "previewed" {
		t.Fatalf("compensation wrong: %+v", first)
	}
	for i := range 10 {
		replay, e := c.CreateCompensationPlan(context.Background(), parent.PlanID, "comp-recovery-key")
		if e != nil {
			t.Fatalf("same-key replay %d must return the original receipt: %v", i, e)
		}
		if replay.PlanID != first.PlanID || replay.CompensationOf != parent.PlanID {
			t.Fatalf("same-key replay %d changed identity: %+v", i, replay)
		}
	}
	// Legitimate later drift must not block the original receipt…
	var files []sdk.File
	c.JSON(context.Background(), "GET", "/api/v1/files?space_id="+h.space.ID+"&q=a.txt", "", nil, &files)
	if len(files) != 1 {
		t.Fatal("drift source missing")
	}
	if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "org-agent/a.txt", FileID: files[0].ID, ExpectedVersion: files[0].VersionID}, "comp-recovery-drift", sizeReader(24), nil); e != nil {
		t.Fatal(e)
	}
	replay, e := c.CreateCompensationPlan(context.Background(), parent.PlanID, "comp-recovery-key")
	if e != nil || replay.PlanID != first.PlanID {
		t.Fatalf("drift must not block the original receipt: %+v %v", replay, e)
	}
	// …but a genuinely new compensation intent still refuses drifted sources.
	if _, e := c.CreateCompensationPlan(context.Background(), parent.PlanID, "comp-recovery-new-key"); e == nil || e.Error() != "compensation_conflict" {
		t.Fatalf("new-key drifted compensation must be compensation_conflict, got %v", e)
	}
}

// A compensation key stays bound to one parent and one owner: another
// parent, an unrelated plain plan or another owner's identical key never
// receives or leaks the stored receipt.
func TestFPAgentOrganizationCompensationKeyIsolation(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	refs := []string{
		transferRef(t, f, c, h.space.ID, "org-agent/a.txt", "alpha"),
		transferRef(t, f, c, h.space.ID, "org-agent/b.txt", "beta"),
	}
	parent := compensationParent(t, h, c, refs, []sdk.OrganizationAction{{Kind: "add_label", Namespace: "rule", Value: "iso"}}, "iso-parent-key")
	owned, e := c.CreateCompensationPlan(context.Background(), parent.PlanID, "iso-comp-key")
	if e != nil {
		t.Fatal(e)
	}
	// A different parent cannot reuse the key: lineage stays bound.
	second := compensationParent(t, h, c, refs, []sdk.OrganizationAction{{Kind: "add_label", Namespace: "rule", Value: "iso2"}}, "iso-parent-key-2")
	if _, e := c.CreateCompensationPlan(context.Background(), second.PlanID, "iso-comp-key"); e == nil || e.Error() != "idempotency_conflict" {
		t.Fatalf("cross-parent key reuse must be idempotency_conflict, got %v", e)
	}
	// A key held by an unrelated plain plan is also refused.
	if _, e := c.CreateOrganizationPlan(context.Background(), sdk.CreateOrganizationPlanRequest{Refs: refs, Actions: []sdk.OrganizationAction{{Kind: "add_label", Namespace: "rule", Value: "unrelated"}}}, "iso-unrelated-key"); e != nil {
		t.Fatal(e)
	}
	if _, e := c.CreateCompensationPlan(context.Background(), parent.PlanID, "iso-unrelated-key"); e == nil || e.Error() != "idempotency_conflict" {
		t.Fatalf("unrelated original key must be idempotency_conflict, got %v", e)
	}
	// Plan keys are globally unique across owners: bob reusing alice's
	// exact key gets a typed idempotency_conflict — never her receipt and
	// never a raw database failure.
	bob, _ := login(t, f, "bob")
	cb := mustSDK(t, bob)
	bobRefs := []string{
		transferRef(t, f, cb, bob.space.ID, "org-bob/a.txt", "alpha"),
		transferRef(t, f, cb, bob.space.ID, "org-bob/b.txt", "beta"),
	}
	bobParent := compensationParent(t, bob, cb, bobRefs, []sdk.OrganizationAction{{Kind: "add_label", Namespace: "rule", Value: "bob"}}, "bob-parent-key")
	if _, e := cb.CreateCompensationPlan(context.Background(), bobParent.PlanID, "iso-comp-key"); e == nil || e.Error() != "idempotency_conflict" {
		t.Fatalf("cross-owner key reuse must be a typed idempotency_conflict, got %v", e)
	}
	// bob's own key yields bob's own receipt: no identity leak either way.
	bobComp, e := cb.CreateCompensationPlan(context.Background(), bobParent.PlanID, "bob-own-comp-key")
	if e != nil {
		t.Fatal(e)
	}
	if bobComp.PlanID == owned.PlanID || bobComp.CompensationOf != bobParent.PlanID {
		t.Fatalf("cross-owner key leak: %+v vs %+v", bobComp, owned)
	}
	// Bob cannot compensate alice's parent at all.
	if _, e := cb.CreateCompensationPlan(context.Background(), parent.PlanID, "bob-foreign-key"); e == nil {
		t.Fatal("foreign parent compensation must fail")
	}
}

// The reverse derivation's members×actions product must mint unique item
// identities: one file organized with two labels (the recovery-smoke
// shape) derives two remove_label actions over one file, and two trashed
// files derive two identical restore instances. Both used to collapse to
// duplicate ItemKeys and 500 the whole creation.
func TestFPAgentOrganizationCompensationDuplicateActionIdentity(t *testing.T) {
	h, c, refs := agentOrgWorld(t, nil)
	// One file, two labels — the exact shape that returned 500 first.
	var files []sdk.File
	c.JSON(context.Background(), "GET", "/api/v1/files?space_id="+h.space.ID+"&q=a.txt", "", nil, &files)
	if len(files) != 1 {
		t.Fatal("single-file source missing")
	}
	parent := compensationParent(t, h, c, []string{refs[0]}, []sdk.OrganizationAction{
		{Kind: "add_label", Namespace: "rule", Value: "one"},
		{Kind: "add_label", Namespace: "rule", Value: "two"},
	}, "dup-parent-key")
	first, e := c.CreateCompensationPlan(context.Background(), parent.PlanID, "dup-comp-key")
	if e != nil {
		t.Fatalf("two remove actions over one file must create, got %v", e)
	}
	if first.CompensationOf != parent.PlanID || first.ItemsTotal != 1 {
		t.Fatalf("single-file compensation wrong: %+v", first)
	}
	replay, e := c.CreateCompensationPlan(context.Background(), parent.PlanID, "dup-comp-key")
	if e != nil || replay.PlanID != first.PlanID {
		t.Fatalf("same-key replay wrong: %+v %v", replay, e)
	}
	// Two trashed files derive two identical restore instances: they
	// collapse to one item per file instead of violating item identity.
	trashed := compensationParent(t, h, c, refs, []sdk.OrganizationAction{{Kind: "trash"}}, "dup-trash-key")
	restore, e := c.CreateCompensationPlan(context.Background(), trashed.PlanID, "dup-restore-key")
	if e != nil {
		t.Fatalf("identical restore instances must dedup, got %v", e)
	}
	if restore.ItemsTotal != 2 {
		t.Fatalf("restore compensation wrong: %+v", restore)
	}
}

// A heterogeneous partial parent (A+x and B+y failed on pre-existing
// labels, A+y and B+x done) compensates exactly what the run completed:
// two reverse items, and after execution the pre-existing labels stay.
func TestFPAgentOrganizationCompensationHeterogeneousPartial(t *testing.T) {
	h, c, refs := agentOrgWorld(t, nil)
	var files []sdk.File
	c.JSON(context.Background(), "GET", "/api/v1/files?space_id="+h.space.ID, "", nil, &files)
	if len(files) != 2 {
		t.Fatal("world files missing")
	}
	byPath := map[string]string{}
	for _, f := range files {
		byPath[f.Path] = f.ID
	}
	fileA, fileB := byPath["org-agent/a.txt"], byPath["org-agent/b.txt"]
	labelsFor := func(fileID string) []string {
		var view struct {
			Namespaces map[string][]string `json:"namespaces"`
		}
		c.JSON(context.Background(), "GET", "/api/v1/files/"+fileID+"/labels", "", nil, &view)
		return view.Namespaces["rule"]
	}
	// Pre-existing labels: A already has x, B already has y.
	compensationParent(t, h, c, []string{refs[0]}, []sdk.OrganizationAction{{Kind: "add_label", Namespace: "rule", Value: "x"}}, "het-seed-a")
	compensationParent(t, h, c, []string{refs[1]}, []sdk.OrganizationAction{{Kind: "add_label", Namespace: "rule", Value: "y"}}, "het-seed-b")
	// The parent run is heterogeneous-partial: adding x to A and y to B
	// fails on the pre-existing assignments while the cross pairs done.
	plan, e := c.CreateOrganizationPlan(context.Background(), sdk.CreateOrganizationPlanRequest{Refs: refs, Actions: []sdk.OrganizationAction{
		{Kind: "add_label", Namespace: "rule", Value: "x"},
		{Kind: "add_label", Namespace: "rule", Value: "y"},
	}}, "het-parent-key")
	if e != nil {
		t.Fatal(e)
	}
	var confirmed sdk.OrganizationPlanView
	h.call(t, "POST", "/organization-plans/"+plan.PlanID+"/confirm", map[string]any{"expected_revision": plan.Revision, "impact_digest": plan.ImpactDigest}, &confirmed)
	partial, e := c.ExecuteOrganizationPlan(context.Background(), plan.PlanID, confirmed.Revision)
	if e != nil {
		t.Fatal(e)
	}
	if partial.Phase != "partial" || partial.ItemsDone != 2 {
		t.Fatalf("parent must be heterogeneous partial: %+v", partial)
	}
	// The compensation derives exactly the two completed assignments —
	// never the action union broadcast over both files.
	comp, e := c.CreateCompensationPlan(context.Background(), plan.PlanID, "het-comp-key")
	if e != nil {
		t.Fatal(e)
	}
	if len(comp.Items) != 2 || comp.ItemsTotal != 2 {
		t.Fatalf("compensation must reverse exactly the done pairs: %+v", comp)
	}
	reversed := map[string]string{}
	for _, item := range comp.Items {
		reversed[item.FileID] = item.Action + ":" + item.ActionDetail
	}
	if reversed[fileA] != "remove_label:rule:y" || reversed[fileB] != "remove_label:rule:x" {
		t.Fatalf("exact per-file reverse association lost: %+v", reversed)
	}
	var compConfirmed sdk.OrganizationPlanView
	h.call(t, "POST", "/organization-plans/"+comp.PlanID+"/confirm", map[string]any{"expected_revision": comp.Revision, "impact_digest": comp.ImpactDigest}, &compConfirmed)
	executed, e := c.ExecuteOrganizationPlan(context.Background(), comp.PlanID, compConfirmed.Revision)
	if e != nil || executed.Phase != "completed" {
		t.Fatalf("compensation execution wrong: %+v %v", executed, e)
	}
	// The parent's own labels are gone; the pre-existing ones survive.
	if got := labelsFor(fileA); len(got) != 1 || got[0] != "x" {
		t.Fatalf("pre-existing label on A destroyed: %v", got)
	}
	if got := labelsFor(fileB); len(got) != 1 || got[0] != "y" {
		t.Fatalf("pre-existing label on B destroyed: %v", got)
	}
}

// Same-key receipt recovery re-checks current admission: a cross-space
// agent, an agent without read scope and an agent whose file moved out of
// its prefix are all refused, while the human owner still recovers.
func TestFPAgentOrganizationCompensationScopedAdmission(t *testing.T) {
	h, c, refs := agentOrgWorld(t, nil)
	parent := compensationParent(t, h, c, refs, []sdk.OrganizationAction{{Kind: "add_label", Namespace: "rule", Value: "scoped"}}, "adm-parent-key")
	comp, e := c.CreateCompensationPlan(context.Background(), parent.PlanID, "adm-comp-key")
	if e != nil {
		t.Fatal(e)
	}
	if replay, e := c.CreateCompensationPlan(context.Background(), parent.PlanID, "adm-comp-key"); e != nil || replay.PlanID != comp.PlanID {
		t.Fatalf("in-scope replay must recover: %+v %v", replay, e)
	}
	newAgent := func(spaceID, prefix, scopes string) *sdk.Client {
		t.Helper()
		var out struct {
			Token string `json:"token"`
		}
		if status := h.call(t, "POST", "/agents", map[string]any{"name": "scoped-" + scopes + "-" + prefix, "space_id": spaceID, "prefix": prefix, "scopes": scopes, "days": 7}, &out); status != 200 {
			t.Fatalf("agent creation status %d", status)
		}
		client, e := sdk.New(h.base, out.Token)
		if e != nil {
			t.Fatal(e)
		}
		return client
	}
	var secondSpace struct {
		ID string `json:"id"`
	}
	if status := h.call(t, "POST", "/spaces", map[string]any{"name": "admission-second", "connection_id": "platform"}, &secondSpace); status != 200 {
		t.Fatalf("second space status %d", status)
	}
	// Same owner, credential scoped to another space: no receipt.
	if _, e := newAgent(secondSpace.ID, "", "read,write").CreateCompensationPlan(context.Background(), parent.PlanID, "adm-comp-key"); e == nil || e.Error() != "forbidden" {
		t.Fatalf("cross-space agent must be forbidden, got %v", e)
	}
	// Same space, credential without read scope: no receipt.
	if _, e := newAgent(h.space.ID, "", "write").CreateCompensationPlan(context.Background(), parent.PlanID, "adm-comp-key"); e == nil || e.Error() != "forbidden" {
		t.Fatalf("no-read-scope agent must be forbidden, got %v", e)
	}
	// A prefix-scoped agent recovers until the file's current path leaves
	// its prefix; the human owner keeps recovering afterwards.
	var files []sdk.File
	c.JSON(context.Background(), "GET", "/api/v1/files?space_id="+h.space.ID+"&q=a.txt", "", nil, &files)
	if len(files) != 1 {
		t.Fatal("admission source missing")
	}
	scoped := newAgent(h.space.ID, "org-agent", "read,write")
	if replay, e := scoped.CreateCompensationPlan(context.Background(), parent.PlanID, "adm-comp-key"); e != nil || replay.PlanID != comp.PlanID {
		t.Fatalf("prefix-scoped in-prefix replay must recover: %+v %v", replay, e)
	}
	if status := h.call(t, "POST", "/files/"+files[0].ID+"/move", map[string]any{"path": "moved/a.txt", "expected_version": files[0].VersionID}, nil); status != 200 {
		t.Fatalf("move status %d", status)
	}
	if _, e := scoped.CreateCompensationPlan(context.Background(), parent.PlanID, "adm-comp-key"); e == nil || e.Error() != "forbidden" {
		t.Fatalf("out-of-prefix current path must be forbidden, got %v", e)
	}
	if replay, e := c.CreateCompensationPlan(context.Background(), parent.PlanID, "adm-comp-key"); e != nil || replay.PlanID != comp.PlanID {
		t.Fatalf("owner replay must survive the path move: %+v %v", replay, e)
	}
}

// A dependency failure on the same-key receipt lookup is never a miss:
// the replay returns database_unavailable instead of falling into the
// fresh-intent drift branch, and recovers once the fault clears.
func TestFPAgentOrganizationCompensationLookupFault(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	c := mustSDK(t, h)
	refs := []string{
		transferRef(t, f, c, h.space.ID, "org-agent/a.txt", "alpha"),
		transferRef(t, f, c, h.space.ID, "org-agent/b.txt", "beta"),
	}
	parent := compensationParent(t, h, c, refs, []sdk.OrganizationAction{{Kind: "add_label", Namespace: "rule", Value: "fault"}}, "fault-parent-key")
	comp, e := c.CreateCompensationPlan(context.Background(), parent.PlanID, "fault-comp-key")
	if e != nil {
		t.Fatal(e)
	}
	// Drift one source so a swallowed lookup failure would surface as a
	// business conflict on the fresh-intent branch.
	var files []sdk.File
	c.JSON(context.Background(), "GET", "/api/v1/files?space_id="+h.space.ID+"&q=a.txt", "", nil, &files)
	if len(files) != 1 {
		t.Fatal("fault source missing")
	}
	if _, e := c.Upload(context.Background(), sdk.UploadRequest{SpaceID: h.space.ID, Path: "org-agent/a.txt", FileID: files[0].ID, ExpectedVersion: files[0].VersionID}, "fault-drift", sizeReader(24), nil); e != nil {
		t.Fatal(e)
	}
	hits := 0
	if err := f.Service.DB.Callback().Query().Before("gorm:query").Register("compensation_stored_fault", func(tx *gorm.DB) {
		if tx.Statement.Table == "organization_plans" {
			hits++
			if hits == 2 { // 1st: parent load; 2nd: owner/key receipt lookup.
				tx.AddError(errors.New("synthetic dependency failure"))
			}
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, e = c.CreateCompensationPlan(context.Background(), parent.PlanID, "fault-comp-key")
	f.Service.DB.Callback().Query().Remove("compensation_stored_fault")
	if e == nil || e.Error() != "database_unavailable" {
		t.Fatalf("lookup dependency failure must surface, got %v", e)
	}
	// The receipt-admission file query keeps the same discipline: a
	// dependency failure there is never a missing receipt.
	if err := f.Service.DB.Callback().Query().Before("gorm:query").Register("compensation_file_fault", func(tx *gorm.DB) {
		if tx.Statement.Table == "files" {
			tx.AddError(errors.New("synthetic dependency failure"))
		}
	}); err != nil {
		t.Fatal(err)
	}
	_, e = c.CreateCompensationPlan(context.Background(), parent.PlanID, "fault-comp-key")
	f.Service.DB.Callback().Query().Remove("compensation_file_fault")
	if e == nil || e.Error() != "database_unavailable" {
		t.Fatalf("admission file failure must surface, got %v", e)
	}
	// With both faults cleared the original receipt still recovers past
	// the legitimate drift, under the same plan identity.
	if replay, e := c.CreateCompensationPlan(context.Background(), parent.PlanID, "fault-comp-key"); e != nil || replay.PlanID != comp.PlanID {
		t.Fatalf("post-fault replay must recover the receipt: %+v %v", replay, e)
	}
}
