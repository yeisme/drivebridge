package sdk_test

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/fixtures"
	"github.com/yeisme/drivebridge-server/internal/service"
	"github.com/yeisme/drivebridge-server/sdk"
)

func workspaceSDK(t *testing.T) (*fixtures.Runtime, *sdk.Client, service.Space) {
	t.Helper()
	f, err := fixtures.StartWithOptions(t.TempDir(), "", "", fixtures.Options{PropertyEditing: true})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(f.Close)
	users, err := f.Service.ImportUsers([]service.UserImport{{Name: "sdk-owner"}})
	if err != nil {
		t.Fatal(err)
	}
	c, err := sdk.New(f.App.URL, users[0].Key)
	if err != nil {
		t.Fatal(err)
	}
	var space service.Space
	if err := c.JSON(t.Context(), "POST", "/api/v1/spaces", "", map[string]string{"name": "SDK workspace", "connection_id": "platform"}, &space); err != nil {
		t.Fatal(err)
	}
	return f, c, space
}

func workspaceSDKError(t *testing.T, err error, want string) {
	t.Helper()
	var typed *sdk.Error
	if !errors.As(err, &typed) || typed.Code != want || err.Error() != want {
		t.Fatalf("expected service code %s, got %v", want, err)
	}
}

func TestWorkspaceSDKPresenceEncoding(t *testing.T) {
	cases := []struct {
		name  string
		input any
		want  string
	}{
		{"patch omitted", sdk.PatchGrantRequest{ExpectedRevision: 3}, `{"expected_revision":3}`},
		{"false and empty prefix", sdk.PatchGrantRequest{ExpectedRevision: 3, Enabled: sdk.Field[bool]{Present: true, Value: false}, Prefix: sdk.Field[string]{Present: true, Value: ""}}, `{"expected_revision":3,"enabled":false,"prefix":""}`},
		{"organization null", sdk.PatchOrganizationRequest{ExpectedRevision: 1, GroupID: sdk.Field[string]{Present: true, Null: true}, Pinned: sdk.Field[bool]{Present: true, Value: false}}, `{"expected_revision":1,"group_id":null,"pinned":false}`},
		{"expiry explicit null", sdk.PatchGrantRequest{ExpectedRevision: 1, ExpiryMode: sdk.Field[string]{Present: true, Null: true}}, `{"expected_revision":1,"expiry_mode":null}`},
		{"create default expiry", sdk.CreateGrantRequest{Name: "Reader", Protocol: "http", SpaceID: "space", ConnectionID: "platform", Actions: []string{"files.read"}}, `{"name":"Reader","protocol":"http","space_id":"space","connection_id":"platform","prefix":"","actions":["files.read"]}`},
		{"create duration", sdk.CreateGrantRequest{Name: "Reader", Protocol: "http", SpaceID: "space", ConnectionID: "platform", Actions: []string{"files.read"}, ExpiryMode: sdk.Field[string]{Present: true, Value: "duration"}, ExpiresInSeconds: sdk.Field[int64]{Present: true, Value: 3600}}, `{"name":"Reader","protocol":"http","space_id":"space","connection_id":"platform","prefix":"","actions":["files.read"],"expiry_mode":"duration","expires_in_seconds":3600}`},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			raw, err := json.Marshal(tt.input)
			if err != nil {
				t.Fatal(err)
			}
			var got, want map[string]json.RawMessage
			if json.Unmarshal(raw, &got) != nil || json.Unmarshal([]byte(tt.want), &want) != nil || len(got) != len(want) {
				t.Fatal("presence fields changed")
			}
			for k, v := range want {
				if !bytes.Equal(got[k], v) {
					t.Fatalf("field %s: got %s want %s", k, got[k], v)
				}
			}
		})
	}
}

func TestWorkspaceSDKGrantLifecycleAndOrganization(t *testing.T) {
	f, c, space := workspaceSDK(t)
	ctx := t.Context()
	input := sdk.CreateGrantRequest{Name: "Reader + 中文 &", Protocol: "http", SpaceID: space.ID, ConnectionID: space.ConnectionID, Actions: []string{"files.read", "inbox.submit", "labels.suggest"}}
	issued, err := c.CreateAgentGrant(ctx, input, "create-reader")
	if err != nil {
		t.Fatal(err)
	}
	if issued.Credential.State != "issued" || issued.Credential.Secret == "" || issued.Grant.ExpiresAt != nil || issued.Grant.ExpiryMode != "never" {
		t.Fatal("issuance lost credential state or nullable expiry")
	}
	agent, err := sdk.New(f.App.URL, issued.Credential.Secret)
	if err != nil {
		t.Fatal(err)
	}
	replay, err := c.CreateAgentGrant(ctx, input, "create-reader")
	if err != nil || replay.Grant.GrantID != issued.Grant.GrantID || replay.Credential.State != "not_replayable" || replay.Credential.Secret != "" {
		t.Fatal("creation replay changed identity or returned a secret")
	}
	lookup, err := c.LookupAgentGrant(ctx, "create-reader")
	if err != nil || lookup.Grant.GrantID != issued.Grant.GrantID || lookup.Credential.State != "not_replayable" {
		t.Fatal("creation lookup lost original grant")
	}
	users, err := f.Service.ImportUsers([]service.UserImport{{Name: "other-sdk-owner"}})
	if err != nil {
		t.Fatal(err)
	}
	other, err := sdk.New(f.App.URL, users[0].Key)
	if err != nil {
		t.Fatal(err)
	}
	_, err = other.GetAgentGrant(ctx, issued.Grant.GrantID)
	workspaceSDKError(t, err, "not_found")
	_, err = other.LookupAgentGrant(ctx, "create-reader")
	workspaceSDKError(t, err, "not_found")
	if _, err := agent.ListInboxSubmissions(ctx, sdk.SubmissionListOptions{}); err != nil {
		t.Fatalf("active Agent could not use its granted action: %v", err)
	}
	// Workspace credentials are rejected during route authentication, before
	// human management handlers run; the existing HTTP contract returns 401.
	_, err = agent.CreateAgentGroup(ctx, sdk.CreateGroupRequest{Name: "self-admin"}, "denied-group")
	workspaceSDKError(t, err, "unauthorized")
	_, err = agent.ListAgentGrants(ctx, sdk.AgentGrantListOptions{})
	workspaceSDKError(t, err, "unauthorized")
	groups, err := c.ListAgentGroups(ctx, sdk.PageOptions{})
	if err != nil || len(groups.Items) != 0 {
		t.Fatalf("denied Agent management created a group: %v", err)
	}
	group, err := c.CreateAgentGroup(ctx, sdk.CreateGroupRequest{Name: "Work"}, "group")
	if err != nil {
		t.Fatal(err)
	}
	org, err := c.PatchAgentOrganization(ctx, issued.Grant.AgentID, sdk.PatchOrganizationRequest{ExpectedRevision: 0, GroupID: sdk.Field[string]{Present: true, Value: group.GroupID}, Pinned: sdk.Field[bool]{Present: true, Value: true}}, "organize")
	if err != nil || org.GroupID == nil || !org.Pinned {
		t.Fatalf("organization patch failed: %v", err)
	}
	org, err = c.PatchAgentOrganization(ctx, issued.Grant.AgentID, sdk.PatchOrganizationRequest{ExpectedRevision: org.Revision, Pinned: sdk.Field[bool]{Present: true, Value: false}}, "unpin-grouped")
	if err != nil || org.GroupID == nil || *org.GroupID != group.GroupID || org.Pinned {
		t.Fatalf("omitted group changed during explicit unpin: %v", err)
	}
	org, err = c.PatchAgentOrganization(ctx, issued.Grant.AgentID, sdk.PatchOrganizationRequest{ExpectedRevision: org.Revision, Pinned: sdk.Field[bool]{Present: true, Value: true}}, "repin-grouped")
	if err != nil || org.GroupID == nil || *org.GroupID != group.GroupID || !org.Pinned {
		t.Fatalf("omitted group changed during explicit pin: %v", err)
	}
	org, err = c.PatchAgentOrganization(ctx, issued.Grant.AgentID, sdk.PatchOrganizationRequest{ExpectedRevision: org.Revision, GroupID: sdk.Field[string]{Present: true, Null: true}}, "ungroup")
	if err != nil || org.GroupID != nil || !org.Pinned {
		t.Fatalf("explicit null did not clear group or omitted pin changed: %v", err)
	}
	org, err = c.PatchAgentOrganization(ctx, issued.Grant.AgentID, sdk.PatchOrganizationRequest{ExpectedRevision: org.Revision, Pinned: sdk.Field[bool]{Present: true, Value: false}}, "unpin-ungrouped")
	if err != nil || org.GroupID != nil || org.Pinned {
		t.Fatalf("explicit false did not clear pin or omitted group changed: %v", err)
	}
	no := false
	page, err := c.ListAgentGrants(ctx, sdk.AgentGrantListOptions{Search: input.Name, Pinned: &no, Limit: 1})
	if err != nil || len(page.Items) != 1 || page.Items[0].GrantID != issued.Grant.GrantID || page.Items[0].Organization == nil {
		t.Fatalf("encoded search/explicit false lost grant: %v", err)
	}
	patch := sdk.PatchGrantRequest{ExpectedRevision: issued.Grant.Revision, Enabled: sdk.Field[bool]{Present: true, Value: false}}
	disabled, err := c.PatchAgentGrant(ctx, issued.Grant.GrantID, patch, "disable")
	if err != nil || disabled.State != "disabled" {
		t.Fatalf("disable: %v", err)
	}
	_, err = c.PatchAgentGrant(ctx, issued.Grant.GrantID, patch, "stale-disable")
	workspaceSDKError(t, err, "revision_conflict")
	receipt, err := c.LookupAgentGrantChange(ctx, issued.Grant.GrantID, sdk.GrantChangeLookupRequest{Action: "disable", IdempotencyKey: "disable"})
	if err != nil || receipt.ResultRevision != disabled.Revision {
		t.Fatalf("disable original receipt: %v", err)
	}
	enabled, err := c.PatchAgentGrant(ctx, disabled.GrantID, sdk.PatchGrantRequest{ExpectedRevision: disabled.Revision, Enabled: sdk.Field[bool]{Present: true, Value: true}}, "enable")
	if err != nil {
		t.Fatal(err)
	}
	rotated, err := c.RotateAgentGrant(ctx, enabled.GrantID, sdk.RevisionRequest{ExpectedRevision: enabled.Revision}, "rotate")
	if err != nil || rotated.Grant.AgentID != issued.Grant.AgentID || rotated.Credential.Secret == "" || rotated.Credential.Secret == issued.Credential.Secret {
		t.Fatal("rotation failed to retain identity and replace credential")
	}
	rotationReplay, err := c.RotateAgentGrant(ctx, enabled.GrantID, sdk.RevisionRequest{ExpectedRevision: enabled.Revision}, "rotate")
	if err != nil || rotationReplay.Grant.GrantID != rotated.Grant.GrantID || rotationReplay.Grant.AgentID != rotated.Grant.AgentID || rotationReplay.Grant.Revision != rotated.Grant.Revision || rotationReplay.Grant.CredentialGeneration != rotated.Grant.CredentialGeneration || rotationReplay.Credential.State != "not_replayable" || rotationReplay.Credential.Secret != "" {
		t.Fatal("same-key rotation replay changed identity/generation or returned a secret")
	}
	currentAgent, err := sdk.New(f.App.URL, rotated.Credential.Secret)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := currentAgent.ListInboxSubmissions(ctx, sdk.SubmissionListOptions{}); err != nil {
		t.Fatalf("rotation replay invalidated the newly issued credential: %v", err)
	}
	_, err = agent.ListInboxSubmissions(ctx, sdk.SubmissionListOptions{})
	workspaceSDKError(t, err, "unauthorized")
	rotation, err := c.LookupAgentGrantRotation(ctx, enabled.GrantID, "rotate")
	if err != nil || rotation.CredentialGeneration != rotated.Grant.CredentialGeneration || rotation.Credential.State != "not_replayable" {
		t.Fatal("rotation lookup changed original generation")
	}
	checks, err := c.CheckAgentGrant(ctx, rotated.Grant.GrantID, sdk.RevisionRequest{ExpectedRevision: rotated.Grant.Revision})
	if err != nil || checks.ConnectionState != "unknown" {
		t.Fatalf("grant checks implied remote health: %v", err)
	}
	revoked, err := c.RevokeAgentGrant(ctx, rotated.Grant.GrantID, sdk.RevisionRequest{ExpectedRevision: rotated.Grant.Revision}, "revoke")
	if err != nil || revoked.State != "revoked" {
		t.Fatalf("grant revoke: %v", err)
	}
}

func TestWorkspaceSDKDocumentPropertiesAndResourceReceipts(t *testing.T) {
	_, c, space := workspaceSDK(t)
	ctx := t.Context()
	op, err := c.Upload(ctx, sdk.UploadRequest{SpaceID: space.ID, Path: "note.md"}, "upload-note", bytes.NewReader([]byte("original")), nil)
	if err != nil {
		t.Fatal(err)
	}
	capability, err := c.GetDocumentCapability(ctx, op.FileID)
	if err != nil || !capability.Editable || capability.VersionID != op.VersionID {
		t.Fatalf("document capability: %v", err)
	}
	save, err := c.SaveDocument(ctx, op.FileID, sdk.SaveDocumentRequest{BaseVersionID: op.VersionID, Text: ""}, "save-empty")
	if err != nil || save.Status != "completed" || save.VersionID == op.VersionID {
		t.Fatalf("empty text save: %v", err)
	}
	lookup, err := c.LookupDocumentSave(ctx, op.FileID, "save-empty")
	if err != nil || !lookup.Found || lookup.Save == nil || lookup.Save.VersionID != save.VersionID {
		t.Fatal("document lookup lost committed version")
	}
	_, err = c.SaveDocument(ctx, op.FileID, sdk.SaveDocumentRequest{BaseVersionID: op.VersionID, Text: "stale"}, "stale-save")
	workspaceSDKError(t, err, "version_conflict")
	defs, err := c.UpdatePropertyDefinitions(ctx, space.ID, sdk.PropertyDefinitionUpdate{Definitions: []sdk.PropertyDefinition{{Key: "done", DisplayName: "Done", Type: "checkbox"}}}, "define")
	if err != nil {
		t.Fatal(err)
	}
	binary, err := c.Upload(ctx, sdk.UploadRequest{SpaceID: space.ID, Path: "attachment.pdf"}, "upload-pdf", bytes.NewReader([]byte("%PDF-1.4\n")), nil)
	if err != nil {
		t.Fatal(err)
	}
	property, err := c.PatchFilePropertySidecar(ctx, binary.FileID, sdk.PropertySidecarPatch{BaseVersionID: binary.VersionID, DefinitionRevision: defs.ResultRevision, Updates: map[string]json.RawMessage{"done": json.RawMessage("false")}, RemoveKeys: []string{}}, "set-false")
	if err != nil {
		t.Fatal(err)
	}
	values, err := c.GetFilePropertySidecar(ctx, binary.FileID, binary.VersionID)
	if err != nil || string(values.Values["done"]) != "false" || values.Revision != property.ResultRevision {
		t.Fatalf("typed property false lost: %v", err)
	}
	original, err := c.LookupFilePropertySidecar(ctx, binary.FileID, sdk.PropertySidecarLookup{VersionID: binary.VersionID, IdempotencyKey: "set-false"})
	if err != nil || original != property {
		t.Fatal("property lookup changed committed receipt")
	}
	resource, err := c.RegisterResource(ctx, sdk.RegisterResourceRequest{SpaceID: space.ID, FileRef: save.FileRef, Kind: "template", DisplayName: "Empty note"}, "register")
	if err != nil || resource.CheckState != "unknown" {
		t.Fatalf("resource registration: %v", err)
	}
	checkInput := sdk.ResourceCheckRequest{ExpectedRevision: resource.Revision, CheckerVersion: sdk.ResourceIntegrityChecker}
	check, err := c.CheckResourceIntegrity(ctx, resource.DescriptorID, checkInput)
	if err != nil || check.State != "passed" || check.CheckScope != sdk.ResourceIntegrityScope {
		t.Fatalf("resource check: %v", err)
	}
	checkLookup, err := c.LookupResourceIntegrity(ctx, resource.DescriptorID, checkInput)
	if err != nil || checkLookup.CheckID != check.CheckID {
		t.Fatal("resource lookup did not return original check")
	}
	home, err := c.GetWorkspaceHome(ctx, space.ID, 1)
	if err != nil || len(home.RecentFiles) != 1 || home.RecentFiles[0].VersionID != binary.VersionID {
		t.Fatalf("home fixed version: %v", err)
	}
	tasks, err := c.ListWorkspaceTasks(ctx, sdk.WorkspaceTaskListOptions{Kind: "operation", SpaceID: space.ID, Limit: 1})
	if err != nil || len(tasks.Items) != 1 || tasks.NextCursor == "" || tasks.Total != nil {
		t.Fatalf("task paging contract: %v", err)
	}
	next, err := c.ListWorkspaceTasks(ctx, sdk.WorkspaceTaskListOptions{Kind: "operation", SpaceID: space.ID, Limit: 1, Cursor: tasks.NextCursor})
	if err != nil || len(next.Items) != 1 || next.Items[0].OwnerID == tasks.Items[0].OwnerID {
		t.Fatalf("opaque task cursor did not advance: %v", err)
	}
	_, err = c.ListWorkspaceTasks(ctx, sdk.WorkspaceTaskListOptions{Kind: "operation", SpaceID: space.ID, State: "completed", Limit: 1, Cursor: tasks.NextCursor})
	workspaceSDKError(t, err, "cursor_expired")
}

func TestWorkspaceSDKSubmissionAndSuggestionIdentity(t *testing.T) {
	f, human, space := workspaceSDK(t)
	ctx := t.Context()
	issued, err := human.CreateAgentGrant(ctx, sdk.CreateGrantRequest{Name: "Contributor", Protocol: "http", SpaceID: space.ID, ConnectionID: space.ConnectionID, Actions: []string{"files.read", "inbox.submit", "labels.suggest"}}, "contributor")
	if err != nil {
		t.Fatal(err)
	}
	agent, err := sdk.New(f.App.URL, issued.Credential.Secret)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256(nil)
	input := sdk.CreateSubmissionRequest{Name: "empty.md", MIMEType: "text/markdown", SHA256: hex.EncodeToString(digest[:])}
	sub, err := agent.CreateInboxSubmission(ctx, input, "submission")
	if err != nil || sub.Purpose != "inbox_staging" {
		t.Fatalf("submission allocation: %v", err)
	}
	lookup, err := agent.LookupInboxSubmission(ctx, "submission")
	if err != nil || lookup.SubmissionID != sub.SubmissionID || lookup.OperationID != sub.OperationID {
		t.Fatal("submission recovery changed operation identity")
	}
	var completed sdk.Operation
	if err := agent.JSON(ctx, "POST", "/api/v1/operations/"+sub.OperationID+"/complete", "", nil, &completed); err != nil {
		t.Fatal(err)
	}
	view, err := human.GetInboxSubmission(ctx, sub.SubmissionID)
	if err != nil || view.State != "submitted" || view.AgentID != issued.Grant.AgentID || view.VersionID == "" || view.UploadOperationRef != completed.ID || completed.Status != "completed" {
		t.Fatalf("submission published identity: %v", err)
	}
	decision := sdk.SubmissionDecisionRequest{ExpectedRevision: view.Revision, VersionID: view.VersionID, Decision: "accept"}
	_, err = agent.DecideInboxSubmission(ctx, sub.SubmissionID, decision, "self-accept")
	workspaceSDKError(t, err, "unauthorized")
	unchanged, err := human.GetInboxSubmission(ctx, sub.SubmissionID)
	if err != nil || unchanged.State != "submitted" || unchanged.Revision != view.Revision || unchanged.Decision != nil {
		t.Fatalf("denied Agent self-approval mutated submission: %v", err)
	}
	accepted, err := human.DecideInboxSubmission(ctx, sub.SubmissionID, decision, "accept")
	if err != nil || accepted.State != "accepted" {
		t.Fatalf("human acceptance: %v", err)
	}
	archived, err := human.ArchiveInboxSubmission(ctx, sub.SubmissionID, sdk.ArchiveSubmissionRequest{ExpectedRevision: accepted.Revision, VersionID: accepted.VersionID, TargetSpaceID: space.ID, TargetPrefix: "reviewed"}, "archive")
	if err != nil || archived.FileRef == nil || archived.OperationRef == "" {
		t.Fatalf("archive result: %v", err)
	}
	file, err := human.Upload(ctx, sdk.UploadRequest{SpaceID: space.ID, Path: "suggest.md"}, "suggest-file", bytes.NewReader([]byte("labels")), nil)
	if err != nil {
		t.Fatal(err)
	}
	suggestion, err := agent.CreateLabelSuggestion(ctx, file.FileID, sdk.LabelSuggestionRequest{VersionID: file.VersionID, Labels: []string{"reviewed"}}, "suggest")
	if err != nil || suggestion.AgentID != issued.Grant.AgentID || suggestion.VersionID != file.VersionID {
		t.Fatalf("suggestion identity: %v", err)
	}
	decide := sdk.SuggestionDecisionRequest{ExpectedRevision: suggestion.Revision, BaseVersionID: file.VersionID, Decision: "accept", Labels: []string{"reviewed"}}
	_, err = agent.DecideLabelSuggestion(ctx, suggestion.SuggestionID, decide, "self-decide")
	workspaceSDKError(t, err, "unauthorized")
	pending, err := agent.ListLabelSuggestions(ctx, file.FileID, sdk.LabelSuggestionListOptions{VersionID: file.VersionID})
	if err != nil || len(pending.Items) != 1 || pending.Items[0].State != "pending" || pending.Items[0].Revision != suggestion.Revision {
		t.Fatalf("denied Agent self-approval mutated suggestion: %v", err)
	}
	chosen, err := human.DecideLabelSuggestion(ctx, suggestion.SuggestionID, decide, "decide")
	if err != nil || chosen.State != "accepted" {
		t.Fatalf("human suggestion decision: %v", err)
	}
	list, err := agent.ListLabelSuggestions(ctx, file.FileID, sdk.LabelSuggestionListOptions{VersionID: file.VersionID, Limit: 1})
	if err != nil || len(list.Items) != 1 || list.Items[0].SuggestionID != chosen.SuggestionID {
		t.Fatalf("version-filtered suggestion read: %v", err)
	}
	invocations, err := human.ListAgentInvocations(ctx, sdk.AgentInvocationListOptions{AgentID: issued.Grant.AgentID, Order: "started_desc", Limit: 1})
	if err != nil || invocations.Coverage != "best_effort" || len(invocations.Items) != 1 || invocations.Items[0].GrantID != issued.Grant.GrantID {
		t.Fatalf("invocation observation identity: %v", err)
	}
	start := invocations.Items[0].StartedAt.In(time.FixedZone("east", 8*3600))
	end := start.Add(time.Nanosecond)
	bounded, err := human.ListAgentInvocations(ctx, sdk.AgentInvocationListOptions{AgentID: issued.Grant.AgentID, StartedAtOrAfter: &start, StartedBefore: &end, Order: "started_desc", Limit: 1})
	if err != nil || len(bounded.Items) != 1 || bounded.Items[0].InvocationID != invocations.Items[0].InvocationID {
		t.Fatalf("offset/nanosecond inclusive boundary changed: %v", err)
	}
	exclusive, err := human.ListAgentInvocations(ctx, sdk.AgentInvocationListOptions{AgentID: issued.Grant.AgentID, StartedAfter: &start, StartedBefore: &end, Limit: 1})
	if err != nil || len(exclusive.Items) != 0 {
		t.Fatalf("exclusive boundary changed: %v", err)
	}
	_, err = human.ListAgentInvocations(ctx, sdk.AgentInvocationListOptions{StartedAfter: &start, StartedAtOrAfter: &start})
	workspaceSDKError(t, err, "invalid_request")
}
