package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	sdk "github.com/yeisme/judgment-sdk"
	"github.com/yeisme/judgment-sdk/providers"
	"gorm.io/gorm"
)

// Managed judgment producer (drivebridge-judgment-sdk-managed-execution-v1).
// The section is dormant by default: no activation row, disabled profile or
// mode=off means zero assembly, zero evaluation, zero credential reads, and
// the workspace suggestion flow keeps its original behavior. Model results
// only ever become pending proposals; human decisions stay human-only.

const (
	WorkspaceJudgmentModeOff    = "off"
	WorkspaceJudgmentModeShadow = "shadow"
	WorkspaceJudgmentModeAssist = "assist"

	// judgmentQuestionSetID pins the frozen producer contract. The question
	// set asks one non-required binary suitability question per candidate
	// label over file metadata; the digest is the SHA-256 of that contract
	// text so providers may abstain without blocking the batch.
	judgmentQuestionSetID      = "workspace-label-suitability"
	judgmentQuestionSetVersion = "1"
	judgmentQuestionSetDigest  = "sha256:ce3ba2e5c768713aafa0492aee8da9801afd5f7c409638587563a44f1d515e1c"

	// judgmentPolicyID pins the conservative advisory policy; the digest is
	// the SHA-256 of the policy text: answers never merge labels, abstention
	// drops a candidate, and only a human decision writes FileLabelAssignment.
	judgmentPolicyID      = "workspace-label-advisory"
	judgmentPolicyVersion = "1"
	judgmentPolicyDigest  = "sha256:3c9e54c8fd6eb8940fa01efc3fc8542b6b1545f74e97f593297147befdee9e07"

	// judgmentAttemptFirst is the only attempt identity this producer issues
	// per request. Unknown outcomes keep it; nothing here re-dispatches.
	judgmentAttemptFirst  = "1"
	judgmentMaxCandidates = 64
)

// judgmentEnvLookup is the host environment lookup injected into the SDK
// resolver. It is only reachable after assembly, which only happens for an
// enabled profile activated in shadow or assist mode.
var judgmentEnvLookup = os.LookupEnv

// judgmentSharedResolver stays unset until an explicit shared-credential
// integration is wired. A shared binding without a resolver stays
// unavailable; the host never falls back to another source.
var judgmentSharedResolver func(context.Context, string) (string, error)

// WorkspaceJudgmentOutcome reports what the producer did. It is internal
// evidence for callers and tests, not a provider verdict and not adoption.
type WorkspaceJudgmentOutcome struct {
	Configured bool
	Mode       string
	ProfileID  string
	// State mirrors the durable receipt: completed | unknown | failed.
	State     string
	Receipt   judgmentReceiptJSON
	Labels    []string
	Abstained []string
	// SuggestionID is set only in assist mode when a pending proposal was
	// recorded. Manual label assignment still requires a human decision.
	SuggestionID string
	Created      bool
}

// judgmentRuntime bundles the assembled SDK pieces: the durable transport
// proves the configured SDK assembles in a normal build, and the runtime is
// the receipt-visible evaluation core the producer calls.
type judgmentRuntime struct {
	transport *providers.DurableTransport
	runtime   providers.Runtime
}

// Service.judgmentRuntime assembles the configured SDK or returns nil for
// every dormant state (no activation, disabled profile, mode=off).
func (s *Service) judgmentRuntime(ctx context.Context, owner string) (*judgmentRuntime, error) {
	profile, activation, ok, err := activeWorkspaceJudgmentProfile(ctx, s.DB, owner)
	if err != nil || !ok {
		return nil, err
	}
	if activation.Mode != WorkspaceJudgmentModeShadow && activation.Mode != WorkspaceJudgmentModeAssist {
		return nil, nil
	}
	credentials := providers.Resolver{Environment: judgmentEnvLookup}
	if judgmentSharedResolver != nil {
		credentials.Shared = judgmentSharedResolver
	}
	options := providers.Options{
		Profile:     profile,
		Credentials: credentials,
		Authorize: func(ctx context.Context, req *sdk.Request) error {
			return workspaceJudgmentAuthorize(owner, req)
		},
	}
	if s.judgmentRoundTripper != nil {
		options.RoundTripper = s.judgmentRoundTripper
	}
	store := newJudgmentStore(s.DB, owner)
	transport, err := providers.NewDurableTransport(options, store)
	if err != nil {
		return nil, errors.New("judgment_configuration_invalid")
	}
	return &judgmentRuntime{transport: transport, runtime: providers.Runtime{Options: options, Store: store}}, nil
}

// ProduceWorkspaceLabelSuggestion is the explicit application-service entry
// for the managed suggestion producer. A human principal requests label
// candidates for a fixed file/version; evaluation happens at most once per
// request identity and its result is a proposal, never a decision.
func (s *Service) ProduceWorkspaceLabelSuggestion(ctx context.Context, p Principal, fileID, versionID, key string, candidates []string, now time.Time) (WorkspaceJudgmentOutcome, error) {
	outcome := WorkspaceJudgmentOutcome{}
	if p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || p.ExternalRead || p.ShareID != "" || p.AccessGrantHash != "" || p.ExecutionID != "" {
		return outcome, code("forbidden")
	}
	labels, err := judgmentCandidateLabels(candidates)
	if err != nil {
		return outcome, err
	}
	if fileID == "" || key == "" || len(key) > 200 || versionID == "" {
		return outcome, code("invalid_request")
	}
	var file File
	if err := s.DB.WithContext(ctx).Where("id = ? AND owner = ?", fileID, p.UserID).First(&file).Error; err != nil || file.Trashed || file.PurgeStartedAt != nil {
		return outcome, code("not_found")
	}
	var version Version
	if err := s.DB.WithContext(ctx).Where("id = ? AND file_id = ? AND owner = ?", versionID, file.ID, p.UserID).First(&version).Error; err != nil {
		return outcome, code("not_found")
	}
	var space Space
	if err := s.DB.WithContext(ctx).Where("id = ? AND owner = ? AND tenant_id = ?", file.SpaceID, p.UserID, p.TenantID).First(&space).Error; err != nil {
		return outcome, code("not_found")
	}
	if (p.ScopeSpace != "" && p.ScopeSpace != space.ID) || (p.DataOwner != "" && p.DataOwner != p.UserID) {
		return outcome, code("forbidden")
	}
	if err := s.governanceAllow(s.DB, p, space.ID, file.Path, "read"); err != nil {
		return outcome, err
	}
	_, activation, configured, err := activeWorkspaceJudgmentProfile(ctx, s.DB, p.UserID)
	if err != nil {
		return outcome, err
	}
	outcome.Configured = configured
	outcome.Mode = activation.Mode
	outcome.ProfileID = activation.ProfileID
	if !configured {
		return outcome, code("judgment_not_configured")
	}
	runtime, err := s.judgmentRuntime(ctx, p.UserID)
	if err != nil {
		return outcome, err
	}
	if runtime == nil {
		return outcome, code("judgment_not_configured")
	}
	request, ordered, err := s.judgmentLabelRequest(p.UserID, file, version, key, labels)
	if err != nil {
		return outcome, err
	}
	result, receiptPtr, evalErr := runtime.runtime.Evaluate(ctx, request)
	if receiptPtr != nil {
		outcome.Receipt = judgmentReceiptOfReceipt(*receiptPtr)
		outcome.State = receiptPtr.State
	}
	if receiptPtr != nil && receiptPtr.State == "completed" && result != nil {
		answered, abstained, invalid := judgmentLabelAnswer(request, ordered, result)
		outcome.Labels = answered
		outcome.Abstained = abstained
		if invalid {
			outcome.State = "failed"
		}
	}
	if evalErr != nil {
		var classified *sdk.Error
		if errors.As(evalErr, &classified) {
			outcome.Receipt.ErrorCode = classified.Code
			outcome.Receipt.SubmissionState = classified.SubmissionState
		}
		return outcome, evalErr
	}
	if activation.Mode != WorkspaceJudgmentModeAssist || len(outcome.Labels) == 0 {
		return outcome, nil
	}
	suggestion, created, err := s.recordWorkspaceJudgmentProposal(ctx, p, file, version, key, activation, outcome.Labels, now)
	if err != nil {
		return outcome, err
	}
	outcome.SuggestionID = suggestion.ID
	outcome.Created = created
	return outcome, nil
}

// judgmentCandidateLabels normalizes and validates candidate labels in
// request order. Duplicates and empties are rejected: the input digest binds
// exactly this ordered list.
func judgmentCandidateLabels(candidates []string) ([]string, error) {
	if len(candidates) == 0 || len(candidates) > judgmentMaxCandidates {
		return nil, code("invalid_request")
	}
	seen := make(map[string]bool, len(candidates))
	out := make([]string, 0, len(candidates))
	for _, label := range candidates {
		label = strings.TrimSpace(label)
		if label == "" || len(label) > 200 || seen[label] {
			return nil, code("invalid_request")
		}
		seen[label] = true
		out = append(out, label)
	}
	return out, nil
}

// judgmentLabelRequest builds the frozen contract request. Sources carry file
// metadata only — path, media type, size and version timestamp — never file
// content bytes. The input digest covers candidates and metadata, so the
// same key with changed input is an attempt conflict, not a silent re-run.
func (s *Service) judgmentLabelRequest(owner string, file File, version Version, key string, labels []string) (*sdk.Request, []string, error) {
	metadata := fmt.Sprintf("file_path=%s\nmedia_type=%s\nsize_bytes=%d\nversion_created_at=%s", file.Path, file.MIME, file.Size, version.Created.UTC().Format(time.RFC3339))
	sourceDigest := "sha256:" + Hash(metadata)
	sources := []map[string]any{{"source_id": "file-metadata", "revision": version.ID, "digest": sourceDigest, "inline_text": metadata, "language": nil, "local_ref": nil}}
	candidateRows := make([]map[string]any, 0, len(labels))
	questionRows := make([]map[string]any, 0, len(labels))
	for i, label := range labels {
		id := fmt.Sprintf("c-%d", i)
		candidateRows = append(candidateRows, map[string]any{"candidate_id": id, "source_bindings": []map[string]any{{"source_id": "file-metadata"}}})
		questionRows = append(questionRows, map[string]any{
			"question_id":   fmt.Sprintf("q-label-%d", i),
			"primitive":     "binary",
			"prompt":        "Is the label \"" + label + "\" appropriate for this file's metadata?",
			"candidate_ids": []string{id},
			"required":      false,
			"answer_domain": nil,
		})
	}
	raw := map[string]any{
		"schema_version": "1.0",
		"request_id":     judgmentRequestID(owner, file.ID, version.ID, key),
		"attempt_id":     judgmentAttemptFirst,
		"scope":          map[string]any{"owner_id": owner, "project_id": judgmentProjectScope, "principal_id": owner, "subject": nil},
		"model":          map[string]any{"transport_provider": "openai", "model_provider": "openai", "requested_model": "gpt-6-luna", "response_model": nil, "underlying_revision": nil, "pin_level": "provider_exact_model", "underlying_revision_verified": false},
		"question_set":   map[string]any{"id": judgmentQuestionSetID, "version": judgmentQuestionSetVersion, "digest": judgmentQuestionSetDigest},
		"policy_ref":     map[string]any{"id": judgmentPolicyID, "version": judgmentPolicyVersion, "digest": judgmentPolicyDigest},
		"sources":        sources,
		"candidates":     candidateRows,
		"questions":      questionRows,
		"limits":         map[string]any{"deadline_ms": 120000, "max_candidates": len(labels), "max_questions": len(questionRows)},
		"extensions":     []map[string]any{},
	}
	data, err := json.Marshal(raw)
	if err != nil {
		return nil, nil, code("invalid_request")
	}
	parsed, validation := sdk.ParseRequest(data, sdk.RequestOptions{})
	if validation != nil {
		return nil, nil, code("invalid_request")
	}
	return parsed, labels, nil
}

// judgmentLabelAnswer maps a completed neutral result back to candidate
// labels. Abstained answers drop their candidate; a missing pair, an error
// answer or a malformed value marks the batch invalid so no partial proposal
// is written.
func judgmentLabelAnswer(request *sdk.Request, labels []string, result *sdk.Result) (answered []string, abstained []string, invalid bool) {
	for i, question := range request.Questions {
		if i >= len(labels) || len(question.CandidateIDs) != 1 {
			return nil, nil, true
		}
		item := result.ItemByPair(question.CandidateIDs[0], question.QuestionID)
		if item == nil {
			return nil, nil, true
		}
		switch item.AnswerStatus {
		case sdk.AnswerAnswered:
			if item.Value == nil || item.Value.Binary == nil {
				return nil, nil, true
			}
			if *item.Value.Binary {
				answered = append(answered, labels[i])
			}
		case sdk.AnswerAbstained:
			abstained = append(abstained, labels[i])
		default:
			return nil, nil, true
		}
	}
	return answered, abstained, false
}

// recordWorkspaceJudgmentProposal writes a pending proposal attributed to
// the judgment producer identity. It reuses the unchanged human decision
// flow: nothing merges into FileLabelAssignment without a human decision on
// this pending suggestion.
func (s *Service) recordWorkspaceJudgmentProposal(ctx context.Context, p Principal, file File, version Version, key string, activation WorkspaceJudgmentActivation, labels []string, now time.Time) (WorkspaceLabelSuggestion, bool, error) {
	var suggestion WorkspaceLabelSuggestion
	agentID := "judgment:" + activation.ProfileID
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Where("owner = ? AND tenant_id = ? AND agent_id = ? AND file_id = ? AND version_id = ? AND creation_key = ?", p.UserID, p.TenantID, agentID, file.ID, version.ID, key).First(&suggestion).Error; err == nil {
			return nil
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		suggestion = WorkspaceLabelSuggestion{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, AgentID: agentID, GrantID: "workspace-judgment@" + activation.Digest, FileID: file.ID, VersionID: version.ID, CreationKey: key, RequestDigest: judgmentRequestID(p.UserID, file.ID, version.ID, key), Labels: append([]string(nil), labels...), State: "pending", Revision: 1, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&suggestion).Error; err != nil {
			return code("database_unavailable")
		}
		audit := Audit{Owner: p.UserID, ActorID: p.UserID, AgentID: agentID, TenantID: p.TenantID, Action: "workspace.judgment.propose", Resource: suggestion.ID, Outcome: "success", At: now}
		if err := tx.Create(&audit).Error; err != nil {
			return code("audit_unavailable")
		}
		return nil
	})
	if err != nil {
		return WorkspaceLabelSuggestion{}, false, err
	}
	return suggestion, true, nil
}

func judgmentReceiptOfReceipt(receipt providers.Receipt) judgmentReceiptJSON {
	if receipt.RequestID == "" {
		return judgmentReceiptJSON{}
	}
	return judgmentReceiptJSON{SchemaVersion: receipt.SchemaVersion, Owner: receipt.Owner, Project: receipt.Project, RequestID: receipt.RequestID, AttemptID: receipt.AttemptID, InputDigest: receipt.InputDigest, ProfileDigest: receipt.ProfileDigest, State: receipt.State, ErrorCode: receipt.ErrorCode, SubmissionState: receipt.SubmissionState, Result: receipt.Result, CreatedAt: receipt.CreatedAt, UpdatedAt: receipt.UpdatedAt}
}
