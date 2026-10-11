package contract

import (
	"github.com/invopop/jsonschema"
	"regexp"
)

// Route describes a planned public contract, not an installed HTTP handler.
// Runtime discovery must not advertise this registry as implemented capability.
type Route struct {
	Method, Path, OperationID, RequestSchema, ResponseSchema string
	HumanOnly, Idempotent                                    bool
	Action, SuccessStatus                                    string
	QueryFields                                              []string
	ReadPolicy                                               string
}

func Routes() []Route {
	return []Route{
		{Method: "get", Path: "/api/v1/spaces/{id}/label-definitions", OperationID: "getLabelDefinitions", ResponseSchema: "LabelDefinitionPageEnvelope", HumanOnly: true, QueryFields: []string{"namespace", "cursor", "limit"}},
		{Method: "put", Path: "/api/v1/spaces/{id}/label-definitions", OperationID: "updateLabelDefinitions", RequestSchema: "LabelDefinitionUpdate", ResponseSchema: "LabelDefinitionReceiptEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "post", Path: "/api/v1/spaces/{id}/label-definitions/lookup", OperationID: "lookupLabelDefinitions", RequestSchema: "IdempotencyLookupRequest", ResponseSchema: "LabelDefinitionReceiptEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/spaces/{id}/label-merge-plans", OperationID: "createLabelMergePlan", RequestSchema: "LabelMergePreview", ResponseSchema: "LabelMergePlanEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "post", Path: "/api/v1/spaces/{id}/label-merge-plans/lookup", OperationID: "lookupLabelMergePlan", RequestSchema: "IdempotencyLookupRequest", ResponseSchema: "LabelMergePlanEnvelope", HumanOnly: true},
		{Method: "get", Path: "/api/v1/organization-plans/{id}", OperationID: "getOrganizationPlan", ResponseSchema: "LabelMergePlanEnvelope", QueryFields: []string{"cursor", "limit"}, ReadPolicy: "Current plan owner and space authorization; agent prefix-scoped items; label merge plans human-only"},
		{Method: "post", Path: "/api/v1/organization-plans/{id}/confirm", OperationID: "confirmOrganizationPlan", RequestSchema: "LabelMergeConfirm", ResponseSchema: "LabelMergePlanEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/organization-plans/{id}/execute", OperationID: "executeOrganizationPlan", RequestSchema: "LabelMergeExecute", ResponseSchema: "LabelMergePlanEnvelope", Action: "write"},
		{Method: "get", Path: "/api/v1/spaces/{id}/property-migrations/capability", OperationID: "getPropertyMigrationCapability", ResponseSchema: "PropertyMigrationCapabilityEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/spaces/{id}/property-migrations", OperationID: "previewPropertyMigration", RequestSchema: "PropertyMigrationPreview", ResponseSchema: "PropertyMigrationReceiptEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "post", Path: "/api/v1/spaces/{id}/property-migrations/lookup", OperationID: "lookupPropertyMigration", RequestSchema: "PropertyMigrationLookup", ResponseSchema: "PropertyMigrationReceiptEnvelope", HumanOnly: true},
		{Method: "get", Path: "/api/v1/property-migrations/{id}", OperationID: "getPropertyMigration", ResponseSchema: "PropertyMigrationViewEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/property-migrations/{id}/confirm", OperationID: "confirmPropertyMigration", RequestSchema: "PropertyMigrationDecision", ResponseSchema: "PropertyMigrationReceiptEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "post", Path: "/api/v1/property-migrations/{id}/execute", OperationID: "executePropertyMigration", RequestSchema: "PropertyMigrationDecision", ResponseSchema: "PropertyMigrationReceiptEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "get", Path: "/api/v1/files/{id}/retention-holds", OperationID: "listWorkspaceHolds", ResponseSchema: "WorkspaceHoldListEnvelope", HumanOnly: true, QueryFields: []string{"version_id", "cursor", "limit"}},
		{Method: "post", Path: "/api/v1/files/{id}/retention-holds", OperationID: "createWorkspaceHold", RequestSchema: "WorkspaceHoldCreate", ResponseSchema: "WorkspaceHoldReceiptEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "post", Path: "/api/v1/files/{id}/retention-holds/lookup", OperationID: "lookupWorkspaceHold", RequestSchema: "WorkspaceHoldLookup", ResponseSchema: "WorkspaceHoldReceiptEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/retention/holds/{id}/release-cas", OperationID: "releaseWorkspaceHold", RequestSchema: "WorkspaceHoldRelease", ResponseSchema: "WorkspaceHoldReceiptEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "get", Path: "/api/v1/favorites", OperationID: "listWorkspaceFavorites", ResponseSchema: "FavoriteListEnvelope", HumanOnly: true, QueryFields: []string{"space_id", "cursor", "limit"}},
		{Method: "post", Path: "/api/v1/favorites", OperationID: "createWorkspaceFavorite", RequestSchema: "FavoriteCreate", ResponseSchema: "FavoriteReceiptEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "delete", Path: "/api/v1/favorites/{id}", OperationID: "deleteWorkspaceFavorite", RequestSchema: "FavoriteDelete", ResponseSchema: "FavoriteReceiptEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "post", Path: "/api/v1/favorites/lookup", OperationID: "lookupWorkspaceFavorite", RequestSchema: "FavoriteLookup", ResponseSchema: "FavoriteReceiptEnvelope", HumanOnly: true},
		{Method: "get", Path: "/api/v1/files/{id}/links", OperationID: "getFileLinks", ResponseSchema: "FileLinkListEnvelope", HumanOnly: true, QueryFields: []string{"version_id", "cursor", "limit"}},
		{Method: "get", Path: "/api/v1/files/{id}/backlinks", OperationID: "getFileBacklinks", ResponseSchema: "FileLinkListEnvelope", HumanOnly: true, QueryFields: []string{"version_id", "cursor", "limit"}},
		{Method: "post", Path: "/api/v1/files/{id}/links/rebuild", OperationID: "rebuildFileLinks", RequestSchema: "FileLinkRebuild", ResponseSchema: "FileLinkReceiptEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "post", Path: "/api/v1/files/{id}/links/lookup", OperationID: "lookupFileLinkRebuild", RequestSchema: "FileLinkLookup", ResponseSchema: "FileLinkReceiptEnvelope", HumanOnly: true},
		{Method: "get", Path: "/api/v1/spaces/{id}/automation-management", OperationID: "getAutomationManagement", ResponseSchema: "AutomationManagementEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/spaces/{id}/automation-rules/changes", OperationID: "changeAutomationRule", RequestSchema: "AutomationChangeRequest", ResponseSchema: "AutomationReceiptEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "post", Path: "/api/v1/spaces/{id}/automation-rules/changes/lookup", OperationID: "lookupAutomationChange", RequestSchema: "IdempotencyLookupRequest", ResponseSchema: "AutomationReceiptEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/automation-rules/{id}/preview", OperationID: "previewAutomationRule", RequestSchema: "AutomationPreviewRequest", ResponseSchema: "AutomationPreviewEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/automation-rules/{id}/execute", OperationID: "executeAutomationPreview", RequestSchema: "AutomationExecuteRequest", ResponseSchema: "AutomationReceiptEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "post", Path: "/api/v1/automation-rules/{id}/execute/lookup", OperationID: "lookupAutomationExecution", RequestSchema: "IdempotencyLookupRequest", ResponseSchema: "AutomationReceiptEnvelope", HumanOnly: true},
		{Method: "get", Path: "/api/v1/file-collections/{id}/snapshots", OperationID: "listCollectionSnapshots", ResponseSchema: "SnapshotIndexEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/file-collections", OperationID: "createFileCollection", RequestSchema: "CreateFileCollectionRequest", ResponseSchema: "FileCollectionViewEnvelope", ReadPolicy: "human_or_legacy_agent_private_collection", Idempotent: true},
		{Method: "get", Path: "/api/v1/files/{id}/properties", OperationID: "getFilePropertySidecar", ResponseSchema: "PropertySidecarViewEnvelope", HumanOnly: true, QueryFields: []string{"version_id"}},
		{Method: "patch", Path: "/api/v1/files/{id}/properties", OperationID: "patchFilePropertySidecar", RequestSchema: "PropertySidecarPatch", ResponseSchema: "PropertySidecarReceiptEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "post", Path: "/api/v1/files/{id}/properties/lookup", OperationID: "lookupFilePropertySidecar", RequestSchema: "PropertySidecarLookup", ResponseSchema: "PropertySidecarReceiptEnvelope", HumanOnly: true},
		{Method: "get", Path: "/api/v1/spaces/{id}/property-definitions", OperationID: "getPropertyDefinitions", ResponseSchema: "PropertyDefinitionViewEnvelope", HumanOnly: true},
		{Method: "put", Path: "/api/v1/spaces/{id}/property-definitions", OperationID: "updatePropertyDefinitions", RequestSchema: "PropertyDefinitionUpdate", ResponseSchema: "PropertyDefinitionReceiptEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "post", Path: "/api/v1/spaces/{id}/property-definitions/lookup", OperationID: "lookupPropertyDefinitionUpdate", RequestSchema: "IdempotencyLookupRequest", ResponseSchema: "PropertyDefinitionReceiptEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/file-collections/{id}/snapshots/creations", OperationID: "createCollectionSnapshot", RequestSchema: "SnapshotCreationRequest", ResponseSchema: "SnapshotCreationReceiptEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "post", Path: "/api/v1/file-collections/{id}/snapshots/creations/lookup", OperationID: "lookupSnapshotCreation", RequestSchema: "IdempotencyLookupRequest", ResponseSchema: "SnapshotCreationReceiptEnvelope", HumanOnly: true},
		{Method: "get", Path: "/api/v1/file-collections/{id}/members/decisions", OperationID: "getCollectionMemberDecisions", ResponseSchema: "CollectionMemberDecisionsEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/file-collections/{id}/members/changes", OperationID: "changeCollectionMembers", RequestSchema: "CollectionMembersChangeRequest", ResponseSchema: "CollectionMembersReceiptEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "post", Path: "/api/v1/file-collections/{id}/members/changes/lookup", OperationID: "lookupCollectionMembersChange", RequestSchema: "IdempotencyLookupRequest", ResponseSchema: "CollectionMembersReceiptEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/file-collections/lookup", OperationID: "lookupCollectionCreation", RequestSchema: "IdempotencyLookupRequest", ResponseSchema: "CollectionCreationReceiptEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/resource-descriptors/{id}/checks", OperationID: "checkResourceIntegrity", RequestSchema: "ResourceCheckRequest", ResponseSchema: "ResourceIntegrityReceiptEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/resource-descriptors/{id}/checks/lookup", OperationID: "lookupResourceIntegrity", RequestSchema: "ResourceCheckRequest", ResponseSchema: "ResourceIntegrityReceiptEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/resource-descriptors/lookup", OperationID: "lookupResourceRegistration", RequestSchema: "IdempotencyLookupRequest", ResponseSchema: "ResourceRegistrationReceiptEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/resource-descriptors", OperationID: "registerResource", RequestSchema: "RegisterResourceRequest", ResponseSchema: "ResourceEnvelope", HumanOnly: true, Idempotent: true, SuccessStatus: "201"},
		{Method: "get", Path: "/api/v1/resource-descriptors", OperationID: "listResources", ResponseSchema: "ResourcePageEnvelope", HumanOnly: true, QueryFields: []string{"space_id", "kind", "cursor", "limit"}},
		{Method: "get", Path: "/api/v1/resource-descriptors/{id}", OperationID: "getResource", ResponseSchema: "ResourceEnvelope", HumanOnly: true},
		{Method: "get", Path: "/api/v1/workspace/home", OperationID: "getWorkspaceHome", ResponseSchema: "WorkspaceHomeEnvelope", HumanOnly: true, QueryFields: []string{"space_id", "recent_limit"}},
		{Method: "get", Path: "/api/v1/workspace/tasks/upload_batch/{id}/items", OperationID: "listWorkspaceBatchTaskItems", ResponseSchema: "BatchTaskItemsEnvelope", HumanOnly: true, QueryFields: []string{"cursor", "limit"}},
		{Method: "get", Path: "/api/v1/workspace/tasks/{kind}/{id}", OperationID: "getWorkspaceOperationTask", ResponseSchema: "OperationTaskDetailEnvelope", HumanOnly: true},
		{Method: "get", Path: "/api/v1/workspace/tasks", OperationID: "listWorkspaceOperationTasks", ResponseSchema: "TaskPageEnvelope", HumanOnly: true, QueryFields: []string{"kind", "space_id", "agent_id", "state", "cursor", "limit"}},
		{Method: "get", Path: "/api/v1/files/{id}/document-capability", OperationID: "getDocumentCapability", ResponseSchema: "DocumentCapabilityEnvelope", HumanOnly: true},
		{Method: "put", Path: "/api/v1/files/{id}/document", OperationID: "saveDocument", RequestSchema: "SaveDocumentRequest", ResponseSchema: "DocumentSaveEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "post", Path: "/api/v1/files/{id}/document/lookup", OperationID: "lookupDocumentSave", RequestSchema: "IdempotencyLookupRequest", ResponseSchema: "DocumentLookupEnvelope", HumanOnly: true},
		{Method: "get", Path: "/api/v1/notebook/pages", OperationID: "listNotebookPages", ResponseSchema: "NotebookPageListEnvelope", HumanOnly: true, QueryFields: []string{"space_id", "cursor", "limit"}},
		{Method: "get", Path: "/api/v1/notebook/pages/{id}", OperationID: "getNotebookPage", ResponseSchema: "NotebookPageEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/notebook/pages", OperationID: "createNotebookPage", RequestSchema: "NotebookCreate", ResponseSchema: "NotebookCreationEnvelope", HumanOnly: true, Idempotent: true, SuccessStatus: "201"},
		{Method: "post", Path: "/api/v1/notebook/pages/lookup", OperationID: "lookupNotebookCreation", RequestSchema: "NotebookCreateLookup", ResponseSchema: "NotebookCreationEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/notebook/pages/resume", OperationID: "resumeNotebookCreation", RequestSchema: "NotebookCreateLookup", ResponseSchema: "NotebookCreationEnvelope", HumanOnly: true},
		{Method: "patch", Path: "/api/v1/notebook/pages/{id}", OperationID: "patchNotebookPage", RequestSchema: "NotebookPagePatch", ResponseSchema: "NotebookPageReceiptEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "post", Path: "/api/v1/notebook/pages/{id}/lookup", OperationID: "lookupNotebookPagePatch", RequestSchema: "IdempotencyLookupRequest", ResponseSchema: "NotebookPageReceiptEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/inbox-submissions/{id}/archive", OperationID: "archiveInboxSubmission", RequestSchema: "ArchiveSubmissionRequest", ResponseSchema: "ArchiveSubmissionEnvelope", HumanOnly: true, Idempotent: true, SuccessStatus: "202"},
		{Method: "post", Path: "/api/v1/label-suggestions/{id}/decision", OperationID: "decideLabelSuggestion", RequestSchema: "SuggestionDecisionRequest", ResponseSchema: "LabelSuggestionEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "get", Path: "/api/v1/inbox-submissions", OperationID: "listInboxSubmissions", ResponseSchema: "SubmissionPageEnvelope", ReadPolicy: "human_visible_or_agent_own_submission", QueryFields: []string{"space_id", "state", "agent_id", "cursor", "limit"}},
		{Method: "get", Path: "/api/v1/inbox-submissions/{id}", OperationID: "getInboxSubmission", ResponseSchema: "SubmissionEnvelope", ReadPolicy: "human_visible_or_agent_own_submission"},
		{Method: "get", Path: "/api/v1/files/{id}/label-suggestions", OperationID: "listLabelSuggestions", ResponseSchema: "SuggestionPageEnvelope", ReadPolicy: "human_visible_or_agent_own_suggestion", QueryFields: []string{"version_id", "cursor", "limit"}},
		{Method: "get", Path: "/api/v1/agent-invocations", OperationID: "listAgentInvocations", ResponseSchema: "InvocationPageEnvelope", HumanOnly: true, QueryFields: []string{"space_id", "agent_id", "result", "started_after", "started_at_or_after", "started_before", "order", "cursor", "limit"}},
		{Method: "post", Path: "/api/v1/agent-grants/{id}/changes/lookup", OperationID: "lookupAgentGrantChange", RequestSchema: "GrantChangeLookupRequest", ResponseSchema: "GrantChangeReceiptEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/agent-grants/{id}/checks", OperationID: "checkAgentGrant", RequestSchema: "RevisionRequest", ResponseSchema: "GrantChecksEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/inbox-submissions/{id}/decision", OperationID: "decideInboxSubmission", RequestSchema: "SubmissionDecisionRequest", ResponseSchema: "SubmissionEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "get", Path: "/api/v1/agent-grants", OperationID: "listAgentGrants", ResponseSchema: "GrantPageEnvelope", HumanOnly: true, QueryFields: []string{"space_id", "state", "q", "group_id", "ungrouped", "pinned", "order", "cursor", "limit"}},
		{Method: "get", Path: "/api/v1/agent-groups", OperationID: "listAgentGroups", ResponseSchema: "GroupPageEnvelope", HumanOnly: true, QueryFields: []string{"cursor", "limit"}},
		{Method: "post", Path: "/api/v1/agent-groups", OperationID: "createAgentGroup", RequestSchema: "CreateGroupRequest", ResponseSchema: "GroupEnvelope", HumanOnly: true, Idempotent: true, SuccessStatus: "201"},
		{Method: "get", Path: "/api/v1/agent-groups/{id}", OperationID: "getAgentGroup", ResponseSchema: "GroupEnvelope", HumanOnly: true},
		{Method: "patch", Path: "/api/v1/agent-groups/{id}", OperationID: "renameAgentGroup", RequestSchema: "RenameGroupRequest", ResponseSchema: "GroupEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "delete", Path: "/api/v1/agent-groups/{id}", OperationID: "deleteAgentGroup", RequestSchema: "RevisionRequest", ResponseSchema: "DeletedGroupEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "get", Path: "/api/v1/agent-organizations", OperationID: "listAgentOrganizations", ResponseSchema: "OrganizationPageEnvelope", HumanOnly: true, QueryFields: []string{"space_id", "group_id", "ungrouped", "pinned", "cursor", "limit"}},
		{Method: "get", Path: "/api/v1/agent-organizations/{agent_id}", OperationID: "getAgentOrganization", ResponseSchema: "OrganizationEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/agent-grants", OperationID: "createAgentGrant", RequestSchema: "CreateGrantRequest", ResponseSchema: "GrantWriteEnvelope", HumanOnly: true, Idempotent: true, SuccessStatus: "201"},
		{Method: "post", Path: "/api/v1/agent-grants/lookup", OperationID: "lookupAgentGrant", RequestSchema: "IdempotencyLookupRequest", ResponseSchema: "GrantReplayEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/agent-grants/{id}/rotate", OperationID: "rotateAgentGrant", RequestSchema: "RevisionRequest", ResponseSchema: "GrantWriteEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "post", Path: "/api/v1/agent-grants/{id}/rotate/lookup", OperationID: "lookupAgentGrantRotation", RequestSchema: "IdempotencyLookupRequest", ResponseSchema: "RotationReplayEnvelope", HumanOnly: true},
		{Method: "post", Path: "/api/v1/agent-grants/{id}/revoke", OperationID: "revokeAgentGrant", RequestSchema: "RevisionRequest", ResponseSchema: "GrantEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "post", Path: "/api/v1/files/{id}/label-suggestions", OperationID: "createLabelSuggestion", RequestSchema: "LabelSuggestionRequest", ResponseSchema: "LabelSuggestionEnvelope", Action: "labels.suggest", Idempotent: true},
		{Method: "post", Path: "/api/v1/inbox-submissions", OperationID: "createInboxSubmission", RequestSchema: "CreateSubmissionRequest", ResponseSchema: "SubmissionUploadEnvelope", Action: "inbox.submit", Idempotent: true, SuccessStatus: "201"},
		{Method: "post", Path: "/api/v1/inbox-submissions/lookup", OperationID: "lookupInboxSubmission", RequestSchema: "IdempotencyLookupRequest", ResponseSchema: "SubmissionUploadEnvelope", Action: "inbox.submit"},
		{Method: "get", Path: "/api/v1/agent-grants/{id}", OperationID: "getAgentGrant", ResponseSchema: "GrantEnvelope", HumanOnly: true},
		{Method: "patch", Path: "/api/v1/agent-grants/{id}", OperationID: "patchAgentGrant", RequestSchema: "PatchGrantRequest", ResponseSchema: "GrantEnvelope", HumanOnly: true, Idempotent: true},
		{Method: "patch", Path: "/api/v1/agent-organizations/{agent_id}", OperationID: "patchAgentOrganization", RequestSchema: "PatchOrganizationRequest", ResponseSchema: "OrganizationEnvelope", HumanOnly: true, Idempotent: true},
	}
}

func EnvelopeSchemas() map[string]*jsonschema.Schema {
	r := jsonschema.Reflector{Anonymous: true, DoNotReference: true}
	grant := r.Reflect(DataEnvelope[GrantView]{})
	// Use the exact response schema including cross-field expiry constraints.
	grant.Properties.Set("data", ResponseSchemas()["GrantView"])
	issued := r.Reflect(DataEnvelope[GrantIssuedView]{})
	replay := r.Reflect(DataEnvelope[GrantReplayView]{})
	for _, envelope := range []*jsonschema.Schema{issued, replay} {
		data, _ := envelope.Properties.Get("data")
		data.Properties.Set("grant", ResponseSchemas()["GrantView"])
	}
	resource := r.Reflect(DataEnvelope[ResourceView]{})
	resource.Properties.Set("data", resourceTargetSchema(r.Reflect(ResourceView{})))
	receipt := r.Reflect(DataEnvelope[ResourceRegistrationReceipt]{})
	receiptData, _ := receipt.Properties.Get("data")
	receiptData.Properties.Set("resource", resourceTargetSchema(r.Reflect(ResourceView{})))
	return map[string]*jsonschema.Schema{
		"LabelDefinitionPageEnvelope":         r.Reflect(DataEnvelope[LabelDefinitionPage]{}),
		"LabelDefinitionReceiptEnvelope":      r.Reflect(DataEnvelope[LabelDefinitionReceipt]{}),
		"LabelMergePlanEnvelope":              r.Reflect(DataEnvelope[LabelMergePlanView]{}),
		"PropertyMigrationCapabilityEnvelope": r.Reflect(DataEnvelope[PropertyMigrationCapability]{}),
		"PropertyMigrationViewEnvelope":       r.Reflect(DataEnvelope[PropertyMigrationView]{}),
		"PropertyMigrationReceiptEnvelope":    r.Reflect(DataEnvelope[PropertyMigrationReceipt]{}),
		"WorkspaceHoldListEnvelope":           r.Reflect(DataEnvelope[WorkspaceHoldList]{}),
		"WorkspaceHoldReceiptEnvelope":        r.Reflect(DataEnvelope[WorkspaceHoldReceipt]{}),
		"FavoriteListEnvelope":                r.Reflect(DataEnvelope[FavoriteList]{}),
		"FavoriteReceiptEnvelope":             r.Reflect(DataEnvelope[FavoriteReceipt]{}),
		"FileLinkListEnvelope":                r.Reflect(DataEnvelope[FileLinkList]{}),
		"FileLinkReceiptEnvelope":             r.Reflect(DataEnvelope[FileLinkReceipt]{}),
		"AutomationManagementEnvelope":        r.Reflect(DataEnvelope[AutomationManagement]{}),
		"AutomationPreviewEnvelope":           r.Reflect(DataEnvelope[AutomationPreview]{}),
		"AutomationReceiptEnvelope":           r.Reflect(DataEnvelope[AutomationReceipt]{}),
		"SnapshotIndexEnvelope":               r.Reflect(DataEnvelope[SnapshotIndex]{}),
		"FileCollectionViewEnvelope":          r.Reflect(DataEnvelope[FileCollectionView]{}),
		"PropertySidecarViewEnvelope":         sidecarViewEnvelopeSchema(r.Reflect(DataEnvelope[PropertySidecarView]{})),
		"PropertySidecarReceiptEnvelope":      r.Reflect(DataEnvelope[PropertySidecarReceipt]{}),
		"PropertyDefinitionViewEnvelope":      r.Reflect(DataEnvelope[PropertyDefinitionView]{}),
		"PropertyDefinitionReceiptEnvelope":   r.Reflect(DataEnvelope[PropertyDefinitionReceipt]{}),
		"SnapshotCreationReceiptEnvelope":     r.Reflect(DataEnvelope[SnapshotCreationReceipt]{}),
		"CollectionMemberDecisionsEnvelope":   r.Reflect(DataEnvelope[CollectionMemberDecisions]{}),
		"CollectionMembersReceiptEnvelope":    r.Reflect(DataEnvelope[CollectionMembersReceipt]{}),
		"CollectionCreationReceiptEnvelope":   r.Reflect(DataEnvelope[CollectionCreationReceipt]{}),
		"ResourceIntegrityReceiptEnvelope":    r.Reflect(DataEnvelope[ResourceIntegrityReceipt]{}),
		"ResourceRegistrationReceiptEnvelope": receipt,
		"ResourceEnvelope":                    resource,
		"WorkspaceHomeEnvelope":               r.Reflect(DataEnvelope[WorkspaceHome]{}),
		"BatchTaskItemsEnvelope":              r.Reflect(DataEnvelope[BatchTaskItems]{}),
		"OperationTaskDetailEnvelope":         r.Reflect(DataEnvelope[OperationTaskDetail]{}),
		"DocumentCapabilityEnvelope":          r.Reflect(DataEnvelope[DocumentCapability]{}),
		"DocumentSaveEnvelope":                r.Reflect(DataEnvelope[DocumentSaveView]{}),
		"DocumentLookupEnvelope":              r.Reflect(DataEnvelope[DocumentLookupView]{}),
		"NotebookPageEnvelope":                r.Reflect(DataEnvelope[NotebookPage]{}),
		"NotebookPageListEnvelope":            r.Reflect(DataEnvelope[Page[NotebookPage]]{}),
		"NotebookCreationEnvelope":            r.Reflect(DataEnvelope[NotebookCreation]{}),
		"NotebookPageReceiptEnvelope":         r.Reflect(DataEnvelope[NotebookPageReceipt]{}),
		"ArchiveSubmissionEnvelope":           r.Reflect(DataEnvelope[ArchiveSubmissionView]{}),
		"GrantChangeReceiptEnvelope":          r.Reflect(DataEnvelope[GrantChangeReceiptView]{}),
		"GrantChecksEnvelope":                 r.Reflect(DataEnvelope[GrantChecksView]{}),
		"GrantWriteEnvelope":                  {OneOf: []*jsonschema.Schema{issued, replay}},
		"GrantIssuedEnvelope":                 issued,
		"GrantReplayEnvelope":                 replay,
		"RotationReplayEnvelope":              r.Reflect(DataEnvelope[RotationReplayView]{}),
		"GrantEnvelope":                       grant,
		"GroupEnvelope":                       r.Reflect(DataEnvelope[AgentGroupView]{}),
		"DeletedGroupEnvelope":                r.Reflect(DataEnvelope[DeletedGroupView]{}),
		"LabelSuggestionEnvelope":             r.Reflect(DataEnvelope[LabelSuggestionView]{}),
		"OrganizationEnvelope":                r.Reflect(DataEnvelope[AgentOrganizationView]{}),
		"ErrorEnvelope":                       r.Reflect(ErrorEnvelope{}),
		"SubmissionUploadEnvelope":            r.Reflect(DataEnvelope[SubmissionUploadView]{}),
		"SubmissionEnvelope":                  r.Reflect(DataEnvelope[SubmissionView]{}),
	}
}

func schemaRef(name string) map[string]string {
	return map[string]string{"$ref": "#/components/schemas/" + name}
}

var pathParameter = regexp.MustCompile(`\{([^{}]+)\}`)

func RegisteredPaths() map[string]any {
	paths := map[string]any{}
	for _, route := range Routes() {
		parameters := []any{}
		for _, match := range pathParameter.FindAllStringSubmatch(route.Path, -1) {
			schema := map[string]any{"type": "string"}
			if route.OperationID == "getWorkspaceOperationTask" && match[1] == "kind" {
				schema["enum"] = []string{"operation"}
			}
			parameters = append(parameters, map[string]any{"name": match[1], "in": "path", "required": true, "schema": schema})
		}
		for _, name := range route.QueryFields {
			schema := map[string]any{"type": "string"}
			switch name {
			case "limit":
				schema = map[string]any{"type": "integer", "minimum": 1, "maximum": 200, "default": 50}
			case "order":
				schema = map[string]any{"type": "string", "enum": []string{"pinned"}}
			case "pinned", "ungrouped":
				schema = map[string]any{"type": "boolean"}
			}
			if route.OperationID == "listAgentInvocations" {
				switch name {
				case "order":
					schema = map[string]any{"type": "string", "enum": []string{"started_desc"}, "description": "Sort by started_at instant descending, then invocation_id ascending. Omit for legacy ID ascending order. Changing order invalidates cursors."}
				case "started_after":
					schema["format"] = "date-time"
					schema["description"] = "Exclusive lower bound (RFC3339Nano instant). Mutually exclusive with started_at_or_after. Must precede started_before when both are supplied."
				case "started_at_or_after":
					schema["format"] = "date-time"
					schema["description"] = "Inclusive lower bound (RFC3339Nano instant). Mutually exclusive with started_after. With started_before selects [start, end); start must precede end."
				case "started_before":
					schema["format"] = "date-time"
					schema["description"] = "Exclusive upper bound (RFC3339Nano instant). Must follow the supplied lower bound."
				}
			}
			required := route.OperationID == "getWorkspaceHome" && name == "space_id"
			if route.OperationID == "getFilePropertySidecar" && name == "version_id" {
				required = true
				schema["pattern"] = `^[a-f0-9]{48}$`
			}
			if route.OperationID == "getWorkspaceHome" && name == "recent_limit" {
				schema = map[string]any{"type": "integer", "minimum": 1, "maximum": 20, "default": 6}
			}
			if route.OperationID == "listResources" && name == "kind" {
				schema = map[string]any{"type": "string", "enum": []string{"skill", "template", "reference_pack"}}
			}
			if route.OperationID == "listWorkspaceOperationTasks" && name == "kind" {
				schema = map[string]any{"type": "string", "enum": []string{"operation", "upload_batch", "organization_plan", "transfer_plan", "automation_intent"}}
				required = true
			}
			parameters = append(parameters, map[string]any{"name": name, "in": "query", "required": required, "schema": schema})
		}
		if route.Idempotent {
			parameters = append(parameters, map[string]any{"name": "Idempotency-Key", "in": "header", "required": true, "schema": map[string]string{"type": "string"}})
		}
		status := route.SuccessStatus
		if status == "" {
			status = "200"
		}
		responses := map[string]any{status: map[string]any{"description": "Successful result", "content": map[string]any{"application/json": map[string]any{"schema": schemaRef(route.ResponseSchema)}}}}
		for _, status := range []string{"400", "401", "403", "404", "409", "410", "422", "500", "503"} {
			responses[status] = map[string]any{"description": "Safe error; status semantics follow the application contract", "content": map[string]any{"application/json": map[string]any{"schema": schemaRef("ErrorEnvelope")}}}
		}
		if route.OperationID == "listCollectionSnapshots" || route.OperationID == "getPropertyDefinitions" || route.OperationID == "createCollectionSnapshot" || route.OperationID == "getCollectionMemberDecisions" || route.OperationID == "listResources" || route.OperationID == "registerResource" || route.OperationID == "getResource" || route.OperationID == "checkResourceIntegrity" || route.OperationID == "lookupResourceIntegrity" {
			responses["429"] = map[string]any{"description": "Resource scan budget exceeded", "content": map[string]any{"application/json": map[string]any{"schema": schemaRef("ErrorEnvelope")}}}
		}
		op := map[string]any{"operationId": route.OperationID, "parameters": parameters, "responses": responses, "x-implementation-status": "planned", "x-human-only": route.HumanOnly, "security": []any{map[string]any{"bearerAuth": []string{}}, map[string]any{"sessionAuth": []string{}}}}
		if route.ReadPolicy != "" {
			op["x-read-policy"] = route.ReadPolicy
		}
		if route.Action != "" {
			op["x-required-action"] = route.Action
			op["security"] = []any{map[string]any{"bearerAuth": []string{}}}
		}
		if route.RequestSchema != "" {
			op["requestBody"] = map[string]any{"required": true, "content": map[string]any{"application/json": map[string]any{"schema": schemaRef(route.RequestSchema)}}}
			op["x-csrf-required-for-session"] = true
		}
		entry, ok := paths[route.Path].(map[string]any)
		if !ok {
			entry = map[string]any{}
			paths[route.Path] = entry
		}
		entry[route.Method] = op
	}
	return paths
}
