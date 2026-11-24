package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	sdk "github.com/yeisme/judgment-sdk"
	"github.com/yeisme/judgment-sdk/providers"
	"gorm.io/gorm"
)

// Managed judgment wiring tests (drivebridge-judgment-sdk-managed-execution-v1).
// All provider traffic is a local fake transport; no real provider, no real
// credentials, no live evidence.

func judgmentEnabledProfile() providers.ExecutionProfile {
	return providers.ExecutionProfile{ID: "workspace-labels", Revision: 1, Provider: "openai_decisions", Model: "gpt-6-luna", Enabled: true, Credential: providers.CredentialBinding{Source: "env", Env: "DRIVEBRIDGE_JUDGMENT_KEY"}}
}

type judgmentFakeTransport struct {
	calls     int
	responses []func() ([]byte, error)
}

func (f *judgmentFakeTransport) RoundTrip(*http.Request) (*http.Response, error) {
	index := f.calls
	f.calls++
	if index >= len(f.responses) {
		return nil, errors.New("unexpected provider dispatch")
	}
	body, err := f.responses[index]()
	if err != nil {
		return nil, err
	}
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(body))}, nil
}

func judgmentProviderBody(answers []map[string]any) []byte {
	doc := map[string]any{"model": "gpt-6-luna", "answers": answers, "usage": map[string]any{"input_tokens": 10, "output_tokens": 5}}
	data, err := json.Marshal(doc)
	if err != nil {
		panic(err)
	}
	return data
}

type judgmentEnvCounter struct{ reads int }

func (c *judgmentEnvCounter) lookup(name string) (string, bool) {
	c.reads++
	return "synthetic-secret", true
}

// judgmentRig opens a disposable workspace database with the judgment models
// and one human-owned file/version.
func judgmentRig(t *testing.T) (*Service, Principal, File, Version, func()) {
	t.Helper()
	db := workspaceTestDB(t)
	if err := db.AutoMigrate(&User{}, &Space{}, &File{}, &Version{}, &Audit{}); err != nil {
		t.Fatal(err)
	}
	if err := migrateWorkspaceModels(db); err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	file := File{ID: "file", Owner: "human", SpaceID: "space", Path: "refs/file.txt", VersionID: "version", Size: 42, MIME: "text/plain"}
	version := Version{ID: "version", Owner: "human", FileID: "file", ConnectionID: "connection", Size: 42, Created: now}
	rows := []any{
		&User{ID: "human", Issuer: "fixture", Subject: "human"},
		&Space{ID: "space", Owner: "human", TenantID: "tenant", ConnectionID: "connection"},
		&file,
		&version,
	}
	for _, row := range rows {
		if err := db.Create(row).Error; err != nil {
			t.Fatal(err)
		}
	}
	s := &Service{DB: db}
	restore := judgmentSwapEnv(&judgmentEnvCounter{})
	return s, Principal{UserID: "human", TenantID: "tenant"}, file, version, restore
}

// judgmentSwapEnv replaces the host environment lookup with a counter so
// tests assert credential reads stay at zero in dormant states.
func judgmentSwapEnv(counter *judgmentEnvCounter) func() {
	original := judgmentEnvLookup
	judgmentEnvLookup = counter.lookup
	return func() { judgmentEnvLookup = original }
}

func judgmentActivate(t *testing.T, s *Service, profile providers.ExecutionProfile, expectedDigest, mode string) (string, uint64) {
	t.Helper()
	digest, generation, err := activateWorkspaceJudgmentOn(context.Background(), s.DB, "human", expectedDigest, mode, "human", profile)
	if err != nil {
		t.Fatalf("activation failed: %v", err)
	}
	return digest, generation
}

func TestWorkspaceJudgmentDefaultOffZeroSideEffects(t *testing.T) {
	s, p, file, version, restore := judgmentRig(t)
	defer restore()
	calls := &judgmentFakeTransport{}
	s.judgmentRoundTripper = calls
	outcome, err := s.ProduceWorkspaceLabelSuggestion(context.Background(), p, file.ID, version.ID, "key", []string{"reference"}, time.Now().UTC())
	if err == nil || err.Error() != "judgment_not_configured" {
		t.Fatalf("expected judgment_not_configured, got %v", err)
	}
	if outcome.Configured || outcome.Mode != "" || outcome.SuggestionID != "" {
		t.Fatalf("dormant outcome leaked state: %+v", outcome)
	}
	var attempts int64
	s.DB.Model(&WorkspaceJudgmentAttempt{}).Count(&attempts)
	if attempts != 0 {
		t.Fatalf("dormant producer wrote %d attempts", attempts)
	}
	var suggestions int64
	s.DB.Model(&WorkspaceLabelSuggestion{}).Count(&suggestions)
	if suggestions != 0 || calls.calls != 0 {
		t.Fatalf("dormant producer side effects: suggestions=%d calls=%d", suggestions, calls.calls)
	}
	counter := &judgmentEnvCounter{}
	judgmentEnvLookup = counter.lookup
	defer restore()
	runtime, err := s.judgmentRuntime(context.Background(), "human")
	if err != nil || runtime != nil {
		t.Fatalf("dormant runtime assembled: %v %v", runtime, err)
	}
	if counter.reads != 0 {
		t.Fatalf("dormant assembly read credentials %d times", counter.reads)
	}
}

func TestWorkspaceJudgmentActivationCASAndSnapshots(t *testing.T) {
	s, _, _, _, _ := judgmentRig(t)
	ctx := context.Background()
	disabled := judgmentEnabledProfile()
	disabled.Enabled = false
	firstDigest, generation := judgmentActivate(t, s, disabled, "", WorkspaceJudgmentModeOff)
	if generation != 1 {
		t.Fatalf("first generation %d", generation)
	}
	if _, _, err := activateWorkspaceJudgmentOn(ctx, s.DB, "human", "sha256:"+strings.Repeat("0", 64), WorkspaceJudgmentModeOff, "human", disabled); err == nil || err.Error() != "judgment_binding_conflict" {
		t.Fatalf("stale expected digest accepted: %v", err)
	}
	if _, _, err := activateWorkspaceJudgmentOn(ctx, s.DB, "human", firstDigest, WorkspaceJudgmentModeShadow, "human", disabled); err == nil || err.Error() != "invalid_request" {
		t.Fatalf("shadow activation of disabled profile accepted: %v", err)
	}
	enabled := judgmentEnabledProfile()
	enabled.Revision = 2
	secondDigest, generation := judgmentActivate(t, s, enabled, firstDigest, WorkspaceJudgmentModeShadow)
	if generation != 2 || secondDigest == firstDigest {
		t.Fatalf("activation did not advance: generation=%d digest_changed=%v", generation, secondDigest == firstDigest)
	}
	profile, activation, ok, err := activeWorkspaceJudgmentProfile(ctx, s.DB, "human")
	if err != nil || !ok || !profile.Enabled || activation.Mode != WorkspaceJudgmentModeShadow || activation.Digest != secondDigest {
		t.Fatalf("active readback mismatch: ok=%v profile=%+v activation=%+v err=%v", ok, profile, activation, err)
	}
	drifted := judgmentEnabledProfile()
	drifted.Credential.Env = "OTHER_KEY"
	if _, err := saveWorkspaceJudgmentProfileOn(ctx, s.DB, "human", drifted); err == nil || err.Error() != "judgment_revision_conflict" {
		t.Fatalf("same revision different content accepted: %v", err)
	}
	next := judgmentEnabledProfile()
	next.Revision = 3
	thirdDigest, generation := judgmentActivate(t, s, next, secondDigest, WorkspaceJudgmentModeAssist)
	if generation != 3 || thirdDigest == secondDigest {
		t.Fatalf("assist activation did not advance: generation=%d", generation)
	}
	_, activation, ok, err = activeWorkspaceJudgmentProfile(ctx, s.DB, "human")
	if err != nil || !ok || activation.Mode != WorkspaceJudgmentModeAssist {
		t.Fatalf("assist readback mismatch: %v %v", activation, err)
	}
}

func TestWorkspaceJudgmentAuthorizePinsScopeAndContract(t *testing.T) {
	s, p, file, version, _ := judgmentRig(t)
	request, _, err := s.judgmentLabelRequest(p.UserID, file, version, "key", []string{"reference"})
	if err != nil {
		t.Fatal(err)
	}
	if err := workspaceJudgmentAuthorize(p.UserID, request); err != nil {
		t.Fatalf("pinned request denied: %v", err)
	}
	tampered := *request
	other := "other-owner"
	tampered.Scope.OwnerID = &other
	if err := workspaceJudgmentAuthorize(p.UserID, &tampered); err == nil {
		t.Fatal("scope owner drift accepted")
	}
	wrongProject := *request
	project := "other-project"
	wrongProject.Scope.ProjectID = &project
	if err := workspaceJudgmentAuthorize(p.UserID, &wrongProject); err == nil {
		t.Fatal("scope project drift accepted")
	}
	wrongQuestionSet := *request
	wrongQuestionSet.QuestionSet.Digest = "sha256:" + strings.Repeat("f", 64)
	if err := workspaceJudgmentAuthorize(p.UserID, &wrongQuestionSet); err == nil {
		t.Fatal("question set drift accepted")
	}
	wrongPolicy := *request
	wrongPolicy.PolicyRef.Digest = "sha256:" + strings.Repeat("e", 64)
	if err := workspaceJudgmentAuthorize(p.UserID, &wrongPolicy); err == nil {
		t.Fatal("policy drift accepted")
	}
}

func TestWorkspaceJudgmentStoreClaimDigestBinding(t *testing.T) {
	s, _, _, _, _ := judgmentRig(t)
	ctx := context.Background()
	store := newJudgmentStore(s.DB, "human")
	now := time.Now().UTC()
	receipt := providers.Receipt{SchemaVersion: "1.0", Owner: "human", Project: judgmentProjectScope, RequestID: "req", AttemptID: judgmentAttemptFirst, InputDigest: "input-1", ProfileDigest: "profile-1", State: "dispatching", CreatedAt: now, UpdatedAt: now}
	first, claimed, err := store.Claim(ctx, receipt)
	if err != nil || !claimed {
		t.Fatalf("first claim failed: %v", err)
	}
	replay, claimed, err := store.Claim(ctx, receipt)
	if err != nil || claimed || replay.RequestID != first.RequestID {
		t.Fatalf("identical replay was not idempotent: %v", err)
	}
	driftedInput := receipt
	driftedInput.InputDigest = "input-2"
	if _, _, err := store.Claim(ctx, driftedInput); err == nil || err.Error() != "judgment_attempt_conflict" {
		t.Fatalf("input digest drift accepted: %v", err)
	}
	driftedProfile := receipt
	driftedProfile.ProfileDigest = "profile-2"
	if _, _, err := store.Claim(ctx, driftedProfile); err == nil || err.Error() != "judgment_attempt_conflict" {
		t.Fatalf("profile digest drift accepted: %v", err)
	}
	receipt.State = "completed"
	receipt.Result = judgmentProviderBody(nil)
	if err := store.Finish(ctx, receipt); err != nil {
		t.Fatalf("finish failed: %v", err)
	}
	stored, found, err := judgmentAttemptReceipt(ctx, s.DB, "human", receipt.RequestID, receipt.AttemptID)
	if err != nil || !found || stored.State != "completed" || stored.InputDigest != "input-1" {
		t.Fatalf("readback mismatch: found=%v state=%s err=%v", found, stored.State, err)
	}
	replay.State = "completed"
	if err := store.Finish(ctx, replay); err == nil {
		t.Fatal("second finish accepted")
	}
}

func TestWorkspaceJudgmentUnknownIsStickyAcrossRestart(t *testing.T) {
	s, p, file, version, restore := judgmentRig(t)
	defer restore()
	calls := &judgmentFakeTransport{responses: []func() ([]byte, error){
		func() ([]byte, error) { return nil, errors.New("synthetic interruption") },
		func() ([]byte, error) { return nil, errors.New("must not re-dispatch") },
	}}
	s.judgmentRoundTripper = calls
	judgmentActivate(t, s, judgmentEnabledProfile(), "", WorkspaceJudgmentModeShadow)
	now := time.Now().UTC()
	outcome, err := s.ProduceWorkspaceLabelSuggestion(context.Background(), p, file.ID, version.ID, "key", []string{"reference"}, now)
	if err == nil {
		t.Fatal("unknown classified as success")
	}
	var sdkErr *sdk.Error
	if !errors.As(err, &sdkErr) || sdkErr.Code != sdk.CodeOutcomeUnknown {
		t.Fatalf("expected outcome_unknown, got %v", err)
	}
	if outcome.State != "unknown" || outcome.Receipt.RequestID == "" {
		t.Fatalf("outcome lost receipt: %+v", outcome)
	}
	// Restart: a fresh store over the same durable state must read the
	// original attempt back without dispatching again.
	restarted := &Service{DB: s.DB, judgmentRoundTripper: calls}
	if _, err := restarted.ProduceWorkspaceLabelSuggestion(context.Background(), p, file.ID, version.ID, "key", []string{"reference"}, now); err == nil {
		t.Fatal("unknown replay classified as success")
	}
	if calls.calls != 1 {
		t.Fatalf("unknown attempt re-dispatched: calls=%d", calls.calls)
	}
	var attempts []WorkspaceJudgmentAttempt
	if s.DB.Find(&attempts).Error != nil || len(attempts) != 1 || attempts[0].State != "unknown" {
		t.Fatalf("attempt rows wrong: %+v", attempts)
	}
}

func TestWorkspaceJudgmentCompletedReplayZeroCallsAndMapping(t *testing.T) {
	s, p, file, version, restore := judgmentRig(t)
	defer restore()
	calls := &judgmentFakeTransport{responses: []func() ([]byte, error){
		func() ([]byte, error) {
			return judgmentProviderBody([]map[string]any{
				{"name": "q0", "type": "predicate", "probability": 0.92},
				{"name": "q1", "type": "refusal"},
				{"name": "q2", "type": "predicate", "probability": 0.11},
			}), nil
		},
		func() ([]byte, error) { return nil, errors.New("completed must not re-dispatch") },
	}}
	s.judgmentRoundTripper = calls
	judgmentActivate(t, s, judgmentEnabledProfile(), "", WorkspaceJudgmentModeShadow)
	now := time.Now().UTC()
	outcome, err := s.ProduceWorkspaceLabelSuggestion(context.Background(), p, file.ID, version.ID, "key", []string{"reference", "archive", "draft"}, now)
	if err != nil {
		t.Fatalf("completed evaluation failed: %v", err)
	}
	if outcome.State != "completed" || len(outcome.Labels) != 1 || outcome.Labels[0] != "reference" {
		t.Fatalf("mapping wrong: %+v", outcome)
	}
	if len(outcome.Abstained) != 1 || outcome.Abstained[0] != "archive" {
		t.Fatalf("abstention mapping wrong: %+v", outcome)
	}
	replay, err := s.ProduceWorkspaceLabelSuggestion(context.Background(), p, file.ID, version.ID, "key", []string{"reference", "archive", "draft"}, now)
	if err != nil {
		t.Fatalf("completed replay failed: %v", err)
	}
	if calls.calls != 1 {
		t.Fatalf("completed replay re-dispatched: calls=%d", calls.calls)
	}
	if len(replay.Labels) != 1 || replay.Labels[0] != "reference" {
		t.Fatalf("replay mapping changed: %+v", replay)
	}
	var suggestions int64
	s.DB.Model(&WorkspaceLabelSuggestion{}).Count(&suggestions)
	if suggestions != 0 {
		t.Fatalf("shadow mode wrote %d suggestions", suggestions)
	}
}

func TestWorkspaceJudgmentAssistCreatesPendingProposalAndHumanDecision(t *testing.T) {
	s, p, file, version, restore := judgmentRig(t)
	defer restore()
	calls := &judgmentFakeTransport{responses: []func() ([]byte, error){
		func() ([]byte, error) {
			return judgmentProviderBody([]map[string]any{
				{"name": "q0", "type": "predicate", "probability": 0.95},
				{"name": "q1", "type": "predicate", "probability": 0.9},
			}), nil
		},
		func() ([]byte, error) { return nil, errors.New("proposal replay must not re-dispatch") },
	}}
	s.judgmentRoundTripper = calls
	judgmentActivate(t, s, judgmentEnabledProfile(), "", WorkspaceJudgmentModeAssist)
	now := time.Now().UTC()
	outcome, err := s.ProduceWorkspaceLabelSuggestion(context.Background(), p, file.ID, version.ID, "key", []string{"reference", "invoice"}, now)
	if err != nil || outcome.SuggestionID == "" || !outcome.Created {
		t.Fatalf("assist produce failed: %v %+v", err, outcome)
	}
	if len(outcome.Labels) != 2 {
		t.Fatalf("expected both labels, got %+v", outcome.Labels)
	}
	replay, err := s.ProduceWorkspaceLabelSuggestion(context.Background(), p, file.ID, version.ID, "key", []string{"reference", "invoice"}, now)
	if err != nil || calls.calls != 1 || replay.SuggestionID != outcome.SuggestionID {
		t.Fatalf("assist replay re-dispatched or changed identity: err=%v calls=%d", err, calls.calls)
	}
	var suggestion WorkspaceLabelSuggestion
	if err := s.DB.Where("id = ?", outcome.SuggestionID).First(&suggestion).Error; err != nil {
		t.Fatal(err)
	}
	if suggestion.State != "pending" || suggestion.AgentID != "judgment:workspace-labels" {
		t.Fatalf("proposal wrong: %+v", suggestion)
	}
	if suggestion.Labels[0] != "reference" || suggestion.Labels[1] != "invoice" {
		t.Fatalf("proposal labels wrong: %+v", suggestion.Labels)
	}
	var assignments int64
	s.DB.Model(&FileLabelAssignment{}).Count(&assignments)
	if assignments != 0 {
		t.Fatalf("producer merged labels without a human decision: %d", assignments)
	}
	var audits int64
	s.DB.Model(&Audit{}).Where("action = ?", "workspace.judgment.propose").Count(&audits)
	if audits != 1 {
		t.Fatalf("proposal audit rows %d", audits)
	}
	// Human-only decision: the unchanged acceptance path merges labels.
	if err := s.DB.AutoMigrate(&FileLabelAssignment{}); err != nil {
		t.Fatal(err)
	}
	decision := contract.SuggestionDecisionRequest{ExpectedRevision: 1, BaseVersionID: version.ID, Decision: "accept", Labels: []string{"reference"}}
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		_, err := s.decideWorkspaceSuggestionOn(tx, p, suggestion.ID, decision, now)
		return err
	})
	if err != nil {
		t.Fatalf("human decision failed: %v", err)
	}
	var merged []FileLabelAssignment
	if err := s.DB.Where("file_id = ?", file.ID).Find(&merged).Error; err != nil || len(merged) != 1 {
		t.Fatalf("human decision did not merge exactly the accepted label: %+v", merged)
	}
	if merged[0].Label != "reference" || merged[0].AuthorKind != "human" || merged[0].AcceptedFrom != suggestion.ID {
		t.Fatalf("merged assignment wrong: %+v", merged[0])
	}
}

func TestWorkspaceJudgmentAuthorizationRunsBeforeCredentials(t *testing.T) {
	s, p, file, version, restore := judgmentRig(t)
	defer restore()
	counter := &judgmentEnvCounter{}
	judgmentEnvLookup = counter.lookup
	judgmentActivate(t, s, judgmentEnabledProfile(), "", WorkspaceJudgmentModeShadow)
	runtime, err := s.judgmentRuntime(context.Background(), p.UserID)
	if err != nil || runtime == nil {
		t.Fatalf("runtime assembly failed: %v", err)
	}
	request, _, err := s.judgmentLabelRequest(p.UserID, file, version, "key", []string{"reference"})
	if err != nil {
		t.Fatal(err)
	}
	tampered := *request
	other := "attacker"
	tampered.Scope.OwnerID = &other
	_, _, evalErr := runtime.runtime.Evaluate(context.Background(), &tampered)
	if evalErr == nil {
		t.Fatal("scope drift evaluated")
	}
	var sdkErr *sdk.Error
	if !errors.As(evalErr, &sdkErr) || sdkErr.Code != sdk.CodeUnauthorized {
		t.Fatalf("expected unauthorized, got %v", evalErr)
	}
	if counter.reads != 0 {
		t.Fatalf("credential read before authorization: %d", counter.reads)
	}
	var attempts int64
	s.DB.Model(&WorkspaceJudgmentAttempt{}).Count(&attempts)
	if attempts != 0 {
		t.Fatalf("denied authorization consumed %d claims", attempts)
	}
}
