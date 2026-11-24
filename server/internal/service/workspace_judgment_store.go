package service

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	sdk "github.com/yeisme/judgment-sdk"
	"github.com/yeisme/judgment-sdk/providers"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// judgmentStore implements the public ReceiptStore contract on this owner's
// GORM state. Claim is an atomic insert-if-absent keyed by owner/project/
// request/attempt and compares both digests on conflict; Finish preserves the
// original binding. A nil store is never assembled: evaluation must not run
// without durable claims.
type judgmentStore struct {
	db      *gorm.DB
	owner   string
	project string
}

func newJudgmentStore(db *gorm.DB, owner string) *judgmentStore {
	return &judgmentStore{db: db, owner: owner, project: judgmentProjectScope}
}

func judgmentAttemptKey(owner, project, request, attempt string) string {
	return Hash(owner + "\x00" + project + "\x00" + request + "\x00" + attempt)
}

func (s *judgmentStore) Claim(ctx context.Context, r providers.Receipt) (providers.Receipt, bool, error) {
	if s == nil || s.db == nil || r.Owner != s.owner || r.Project != s.project || r.RequestID == "" || r.AttemptID == "" || r.State != "dispatching" {
		return providers.Receipt{}, false, errors.New("judgment_scope_invalid")
	}
	key := judgmentAttemptKey(r.Owner, r.Project, r.RequestID, r.AttemptID)
	row := WorkspaceJudgmentAttempt{Key: key, Owner: r.Owner, Project: r.Project, RequestID: r.RequestID, AttemptID: r.AttemptID, InputDigest: r.InputDigest, ProfileDigest: r.ProfileDigest, State: r.State, CreatedAt: r.CreatedAt, UpdatedAt: r.UpdatedAt}
	result := s.db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
	if result.Error != nil {
		return providers.Receipt{}, false, code("database_unavailable")
	}
	if result.RowsAffected == 1 {
		return r, true, nil
	}
	var old WorkspaceJudgmentAttempt
	if err := s.db.WithContext(ctx).Where("key = ? AND owner = ? AND project = ?", key, s.owner, s.project).First(&old).Error; err != nil {
		return providers.Receipt{}, false, code("database_unavailable")
	}
	if old.InputDigest != r.InputDigest || old.ProfileDigest != r.ProfileDigest {
		return providers.Receipt{}, false, errors.New("judgment_attempt_conflict")
	}
	return judgmentReceiptOf(old), false, nil
}

func (s *judgmentStore) Finish(ctx context.Context, r providers.Receipt) error {
	if s == nil || s.db == nil || r.Owner != s.owner || r.Project != s.project || (r.State != "completed" && r.State != "failed" && r.State != "unknown") {
		return errors.New("judgment_scope_invalid")
	}
	key := judgmentAttemptKey(r.Owner, r.Project, r.RequestID, r.AttemptID)
	result := s.db.WithContext(ctx).Model(&WorkspaceJudgmentAttempt{}).Where("key = ? AND owner = ? AND project = ? AND input_digest = ? AND profile_digest = ? AND state = ?", key, s.owner, s.project, r.InputDigest, r.ProfileDigest, "dispatching").Updates(map[string]any{"state": r.State, "error_code": r.ErrorCode, "submission_state": r.SubmissionState, "result": r.Result, "updated_at": r.UpdatedAt})
	if result.Error != nil {
		return code("database_unavailable")
	}
	if result.RowsAffected != 1 {
		return errors.New("judgment_attempt_state_conflict")
	}
	return nil
}

// judgmentAttemptReceipt returns the stored receipt for original-operation
// readback. Unknown attempts keep their identity; callers never re-dispatch.
func judgmentAttemptReceipt(ctx context.Context, db *gorm.DB, owner, requestID, attemptID string) (providers.Receipt, bool, error) {
	key := judgmentAttemptKey(owner, judgmentProjectScope, requestID, attemptID)
	var row WorkspaceJudgmentAttempt
	if err := db.WithContext(ctx).Where("key = ? AND owner = ? AND project = ?", key, owner, judgmentProjectScope).First(&row).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return providers.Receipt{}, false, nil
		}
		return providers.Receipt{}, false, code("database_unavailable")
	}
	return judgmentReceiptOf(row), true, nil
}

func judgmentReceiptOf(row WorkspaceJudgmentAttempt) providers.Receipt {
	return providers.Receipt{SchemaVersion: "1.0", Owner: row.Owner, Project: row.Project, RequestID: row.RequestID, AttemptID: row.AttemptID, InputDigest: row.InputDigest, ProfileDigest: row.ProfileDigest, State: row.State, ErrorCode: row.ErrorCode, SubmissionState: row.SubmissionState, Result: row.Result, CreatedAt: row.CreatedAt, UpdatedAt: row.UpdatedAt}
}

// saveWorkspaceJudgmentProfileOn records an immutable snapshot. The digest is
// recomputed from the validated profile; storing different content under the
// same revision is rejected. Snapshots hold references only, never secrets.
func saveWorkspaceJudgmentProfileOn(ctx context.Context, db *gorm.DB, owner string, profile providers.ExecutionProfile) (string, error) {
	if db == nil || owner == "" {
		return "", code("invalid_request")
	}
	if err := profile.Validate(); err != nil {
		return "", code("invalid_request")
	}
	digest, err := profile.Digest()
	if err != nil {
		return "", code("invalid_request")
	}
	raw, err := jsonMarshalProfile(profile)
	if err != nil {
		return "", code("invalid_request")
	}
	row := WorkspaceJudgmentProfileSnapshot{ID: ID(), Owner: owner, ProfileID: profile.ID, Revision: profile.Revision, Provider: profile.Provider, Model: profile.Model, CredentialSource: profile.Credential.Source, CredentialRef: profile.Credential.Ref, CredentialEnv: profile.Credential.Env, Enabled: profile.Enabled, Digest: digest, Profile: raw, CreatedAt: time.Now().UTC()}
	if err = db.WithContext(ctx).Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error; err != nil {
		return "", code("database_unavailable")
	}
	var stored WorkspaceJudgmentProfileSnapshot
	if err = db.WithContext(ctx).Where("owner = ? AND profile_id = ? AND revision = ?", owner, profile.ID, profile.Revision).First(&stored).Error; err != nil {
		return "", code("database_unavailable")
	}
	if stored.Digest != digest {
		return "", errors.New("judgment_revision_conflict")
	}
	return digest, nil
}

// activateWorkspaceJudgmentOn is the explicit application-service activation.
// It compares the previously applied digest (CAS): the caller must present the
// digest it believes is active, or empty for a first activation. Execution
// never calls this path; there is no implicit central sync.
func activateWorkspaceJudgmentOn(ctx context.Context, db *gorm.DB, owner, expectedDigest, mode, actor string, profile providers.ExecutionProfile) (string, uint64, error) {
	if mode != WorkspaceJudgmentModeOff && mode != WorkspaceJudgmentModeShadow && mode != WorkspaceJudgmentModeAssist {
		return "", 0, code("invalid_request")
	}
	digest, err := saveWorkspaceJudgmentProfileOn(ctx, db, owner, profile)
	if err != nil {
		return "", 0, err
	}
	if !profile.Enabled && mode != WorkspaceJudgmentModeOff {
		return "", 0, code("invalid_request")
	}
	var generation uint64
	err = db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var old WorkspaceJudgmentActivation
		lookup := tx.Where("owner = ?", owner).First(&old).Error
		if errors.Is(lookup, gorm.ErrRecordNotFound) {
			if expectedDigest != "" {
				return errors.New("judgment_binding_conflict")
			}
			row := WorkspaceJudgmentActivation{Owner: owner, ProfileID: profile.ID, Revision: profile.Revision, Digest: digest, Mode: mode, Generation: 1, ActivatedBy: actor, UpdatedAt: time.Now().UTC()}
			if err := tx.Create(&row).Error; err != nil {
				return code("database_unavailable")
			}
			generation = 1
			return nil
		}
		if lookup != nil {
			return code("database_unavailable")
		}
		if old.Digest != expectedDigest {
			return errors.New("judgment_binding_conflict")
		}
		result := tx.Model(&WorkspaceJudgmentActivation{}).Where("owner = ? AND digest = ? AND generation = ?", owner, expectedDigest, old.Generation).Updates(map[string]any{"profile_id": profile.ID, "revision": profile.Revision, "digest": digest, "mode": mode, "generation": old.Generation + 1, "activated_by": actor, "updated_at": time.Now().UTC()})
		if result.Error != nil {
			return code("database_unavailable")
		}
		if result.RowsAffected != 1 {
			return errors.New("judgment_binding_conflict")
		}
		generation = old.Generation + 1
		return nil
	})
	if err != nil {
		return "", 0, err
	}
	return digest, generation, nil
}

// activeWorkspaceJudgmentProfile is the dormant-by-default readback. Missing
// activation, disabled profile or mode=off all mean: no judgment, no
// credential reads, original business flow unchanged.
func activeWorkspaceJudgmentProfile(ctx context.Context, db *gorm.DB, owner string) (providers.ExecutionProfile, WorkspaceJudgmentActivation, bool, error) {
	var activation WorkspaceJudgmentActivation
	if err := db.WithContext(ctx).Where("owner = ?", owner).First(&activation).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return providers.ExecutionProfile{}, activation, false, nil
		}
		return providers.ExecutionProfile{}, activation, false, code("database_unavailable")
	}
	if activation.Digest == "" || activation.Mode == WorkspaceJudgmentModeOff {
		return providers.ExecutionProfile{}, activation, false, nil
	}
	var snapshot WorkspaceJudgmentProfileSnapshot
	if err := db.WithContext(ctx).Where("owner = ? AND profile_id = ? AND revision = ? AND digest = ?", owner, activation.ProfileID, activation.Revision, activation.Digest).First(&snapshot).Error; err != nil {
		return providers.ExecutionProfile{}, activation, false, code("judgment_snapshot_corrupt")
	}
	var profile providers.ExecutionProfile
	if err := jsonUnmarshalProfile(snapshot.Profile, &profile); err != nil {
		return providers.ExecutionProfile{}, activation, false, code("judgment_snapshot_corrupt")
	}
	digest, err := profile.Digest()
	if err != nil || digest != activation.Digest || !profile.Enabled {
		return providers.ExecutionProfile{}, activation, false, code("judgment_snapshot_corrupt")
	}
	return profile, activation, true, nil
}

// workspaceJudgmentAuthorize is the host authorization check. It runs before
// any credential resolution inside the SDK runtime: the request scope must
// bind to this workspace owner and the pinned project, and the question set
// and policy reference must match the frozen producer contract.
func workspaceJudgmentAuthorize(owner string, req *sdk.Request) error {
	if req == nil || req.Scope.OwnerID == nil || *req.Scope.OwnerID != owner || req.Scope.ProjectID == nil || *req.Scope.ProjectID != judgmentProjectScope {
		return errors.New("judgment_scope_denied")
	}
	if req.QuestionSet.ID != judgmentQuestionSetID || req.QuestionSet.Version != judgmentQuestionSetVersion || req.QuestionSet.Digest != judgmentQuestionSetDigest {
		return errors.New("judgment_question_set_denied")
	}
	if req.PolicyRef.ID != judgmentPolicyID || req.PolicyRef.Version != judgmentPolicyVersion || req.PolicyRef.Digest != judgmentPolicyDigest {
		return errors.New("judgment_policy_denied")
	}
	return nil
}

func jsonMarshalProfile(profile providers.ExecutionProfile) ([]byte, error) {
	return json.Marshal(profile)
}

func jsonUnmarshalProfile(raw []byte, out *providers.ExecutionProfile) error {
	return json.Unmarshal(raw, out)
}
