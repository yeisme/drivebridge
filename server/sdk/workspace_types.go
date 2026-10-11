package sdk

import (
	"encoding/json"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

// Workspace DTOs alias the canonical HTTP contract so consumers need not import
// an internal package. Authorization, validation and capability gates remain on
// the HTTP service; these types do not grant Agent management permissions.
type Field[T any] = contract.Field[T]
type Page[T any] = contract.Page[T]
type GrantView = contract.GrantView
type GrantReplayView = contract.GrantReplayView
type RotationReplayView = contract.RotationReplayView
type GrantChangeLookupRequest = contract.GrantChangeLookupRequest
type GrantChangeReceiptView = contract.GrantChangeReceiptView
type GrantChecksView = contract.GrantChecksView
type GrantCheck = contract.GrantCheck
type RevisionRequest = contract.RevisionRequest
type AgentGroupView = contract.AgentGroupView
type AgentOrganizationView = contract.AgentOrganizationView
type CreateGroupRequest = contract.CreateGroupRequest
type RenameGroupRequest = contract.RenameGroupRequest
type DeletedGroupView = contract.DeletedGroupView

// GrantWriteView represents both first issuance and idempotent replay. Read
// Credential.Secret only when State is "issued"; "not_replayable" has no secret.
// Neither this response nor its credential should be logged.
type GrantWriteView = contract.GrantIssuedView
type IssuedCredential = contract.IssuedCredential
type ReplayCredential = contract.ReplayCredential

type WorkspaceHome = contract.WorkspaceHome
type HomeSection = contract.HomeSection
type HomeSections = contract.HomeSections
type HomeFileRef = contract.HomeFileRef
type HomeCollection = contract.HomeCollection
type HomePending = contract.HomePending
type HomeUsage = contract.HomeUsage
type TaskProjection = contract.TaskProjection
type TaskProgress = contract.TaskProgress
type TaskOutputRef = contract.TaskOutputRef
type OperationTaskDetail = contract.OperationTaskDetail
type BatchTaskItem = contract.BatchTaskItem
type BatchTaskItems = contract.BatchTaskItems
type InvocationView = contract.InvocationView
type InvocationPage = contract.InvocationPage

type CreateSubmissionRequest = contract.CreateSubmissionRequest
type SubmissionUploadView = contract.SubmissionUploadView
type SubmissionView = contract.SubmissionView
type SubmissionDecisionView = contract.SubmissionDecisionView
type SubmissionDecisionRequest = contract.SubmissionDecisionRequest
type ArchiveSubmissionRequest = contract.ArchiveSubmissionRequest
type ArchiveSubmissionView = contract.ArchiveSubmissionView
type LabelSuggestionRequest = contract.LabelSuggestionRequest
type LabelSuggestionView = contract.LabelSuggestionView
type SuggestionDecisionRequest = contract.SuggestionDecisionRequest

type SaveDocumentRequest = contract.SaveDocumentRequest
type DocumentCapability = contract.DocumentCapability
type DocumentSaveView = contract.DocumentSaveView
type DocumentLookupView = contract.DocumentLookupView
type PropertyDefinition = contract.PropertyDefinition
type PropertyDefinitionUpdate = contract.PropertyDefinitionUpdate
type PropertyDefinitionView = contract.PropertyDefinitionView
type PropertyDefinitionReceipt = contract.PropertyDefinitionReceipt
type PropertySidecarPatch = contract.PropertySidecarPatch
type PropertySidecarLookup = contract.PropertySidecarLookup
type PropertySidecarView = contract.PropertySidecarView
type PropertySidecarReceipt = contract.PropertySidecarReceipt
type RegisterResourceRequest = contract.RegisterResourceRequest
type ResourceView = contract.ResourceView
type ResourceRegistrationReceipt = contract.ResourceRegistrationReceipt
type ResourceCheckRequest = contract.ResourceCheckRequest
type ResourceIntegrityReceipt = contract.ResourceIntegrityReceipt

const DocumentMaxBytes = contract.DocumentMaxBytes
const ResourceIntegrityChecker = contract.ResourceIntegrityChecker
const ResourceIntegrityScope = contract.ResourceIntegrityScope

// Canonical presence fields are decode-only. These three outgoing adapters
// preserve the DTO fields but marshal omission/null/value as HTTP JSON, never
// the internal {Present, Null, Value} representation used in server digests.
type CreateGrantRequest contract.CreateGrantRequest
type PatchGrantRequest contract.PatchGrantRequest
type PatchOrganizationRequest contract.PatchOrganizationRequest

func workspaceField[T any](body map[string]any, key string, field Field[T]) {
	if !field.Present {
		return
	}
	if field.Null {
		body[key] = nil
	} else {
		body[key] = field.Value
	}
}

func (q CreateGrantRequest) MarshalJSON() ([]byte, error) {
	body := map[string]any{"name": q.Name, "protocol": q.Protocol, "space_id": q.SpaceID, "connection_id": q.ConnectionID, "prefix": q.Prefix, "actions": q.Actions}
	workspaceField(body, "expiry_mode", q.ExpiryMode)
	workspaceField(body, "expires_in_seconds", q.ExpiresInSeconds)
	return json.Marshal(body)
}

func (q PatchGrantRequest) MarshalJSON() ([]byte, error) {
	body := map[string]any{"expected_revision": q.ExpectedRevision}
	workspaceField(body, "name", q.Name)
	workspaceField(body, "prefix", q.Prefix)
	workspaceField(body, "actions", q.Actions)
	workspaceField(body, "enabled", q.Enabled)
	workspaceField(body, "expiry_mode", q.ExpiryMode)
	workspaceField(body, "expires_in_seconds", q.ExpiresInSeconds)
	return json.Marshal(body)
}

func (q PatchOrganizationRequest) MarshalJSON() ([]byte, error) {
	body := map[string]any{"expected_revision": q.ExpectedRevision}
	workspaceField(body, "group_id", q.GroupID)
	workspaceField(body, "pinned", q.Pinned)
	return json.Marshal(body)
}
