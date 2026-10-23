package desktopcontract

// IPCMethod is a build-time signature and routing declaration. It is not an
// installed handler, a runtime capability, or permission to invoke a method.
// Schema names beginning pinax. always resolve from the owner export.
type IPCMethod struct {
	Name            string `json:"name"`
	HTTPMethod      string `json:"http_method"`
	Path            string `json:"path"`
	RequestSchema   string `json:"request_schema"`
	ResponseSchema  string `json:"response_schema"`
	TargetField     string `json:"target_field,omitempty"`
	Idempotent      bool   `json:"idempotent"`
	NativeSelection bool   `json:"native_selection"`
	Status          string `json:"status"`
}

// IPCMethods enumerates individual preload methods, never generic fetch/fs.
// TargetField is an opaque route parameter. It is not a URL or local path.
func IPCMethods() []IPCMethod {
	methods := []IPCMethod{
		{Name: "capabilities", HTTPMethod: "GET", Path: "/v1/desktop/capabilities", RequestSchema: "EmptyRequest", ResponseSchema: "DesktopCapabilities"},
		{Name: "listSources", HTTPMethod: "GET", Path: "/v1/desktop/sources", RequestSchema: "PageQuery", ResponseSchema: "SourcePage"},
		{Name: "chooseVault", HTTPMethod: "POST", Path: "/v1/desktop/bindings/preview", RequestSchema: "EmptyRequest", ResponseSchema: "BindingPreview", NativeSelection: true},
		{Name: "confirmBinding", HTTPMethod: "POST", Path: "/v1/desktop/bindings", RequestSchema: "ConfirmBinding", ResponseSchema: "BindingMutationReceipt", Idempotent: true},
		{Name: "lookupBindingMutation", HTTPMethod: "POST", Path: "/v1/desktop/bindings/lookup", RequestSchema: "BindingMutationLookup", ResponseSchema: "BindingLookupResult"},
		{Name: "setSourceVisibility", HTTPMethod: "PATCH", Path: "/v1/desktop/bindings/{binding_id}", TargetField: "binding_id", RequestSchema: "SetSourceVisibility", ResponseSchema: "BindingView", Idempotent: true},
		{Name: "releaseProtection", HTTPMethod: "POST", Path: "/v1/desktop/bindings/{binding_id}/release", TargetField: "binding_id", RequestSchema: "ReleaseProtection", ResponseSchema: "BindingMutationReceipt", Idempotent: true},
		{Name: "listNotes", HTTPMethod: "GET", Path: "/v1/desktop/sources/{source_id}/notes", TargetField: "source_id", RequestSchema: "pinax.reader.notes.request.v1alpha1", ResponseSchema: "pinax.reader.note_list.v1alpha1"},
		{Name: "searchNotes", HTTPMethod: "GET", Path: "/v1/desktop/sources/{source_id}/search", TargetField: "source_id", RequestSchema: "pinax.reader.search.request.v1alpha1", ResponseSchema: "pinax.reader.search_results.v1alpha1"},
		{Name: "readDocument", HTTPMethod: "GET", Path: "/v1/desktop/sources/{source_id}/notes/{object_id}/document", TargetField: "source_id", RequestSchema: "pinax.reader.document.request.v1alpha1", ResponseSchema: "pinax.reader.document.v1alpha1"},
		{Name: "readLinks", HTTPMethod: "GET", Path: "/v1/desktop/sources/{source_id}/notes/{object_id}/links", TargetField: "source_id", RequestSchema: "pinax.reader.links.request.v1alpha1", ResponseSchema: "pinax.reader.link_page.v1alpha1"},
		{Name: "listTags", HTTPMethod: "GET", Path: "/v1/desktop/sources/{source_id}/tags", TargetField: "source_id", RequestSchema: "pinax.reader.tags.request.v1alpha1", ResponseSchema: "pinax.reader.tags.v1alpha1"},
		{Name: "readGraph", HTTPMethod: "GET", Path: "/v1/desktop/sources/{source_id}/graph", TargetField: "source_id", RequestSchema: "pinax.reader.graph.request.v1alpha1", ResponseSchema: "pinax.reader.graph.v1alpha1"},
		{Name: "findDaily", HTTPMethod: "GET", Path: "/v1/desktop/sources/{source_id}/journals/daily", TargetField: "source_id", RequestSchema: "pinax.reader.daily.request.v1alpha1", ResponseSchema: "pinax.reader.daily.v1alpha1"},
		{Name: "readEditorSource", HTTPMethod: "GET", Path: "/v1/desktop/sources/{source_id}/writer/editor-source", TargetField: "source_id", RequestSchema: "pinax.writer.editor_source.request.v1alpha1", ResponseSchema: "pinax.writer.editor_source.v1alpha1"},
		{Name: "proposeEditorDraft", HTTPMethod: "POST", Path: "/v1/desktop/sources/{source_id}/writer/editor:propose", TargetField: "source_id", RequestSchema: "DesktopEditorDraftInput", ResponseSchema: "pinax.projection.v1"},
		{Name: "syncStatus", HTTPMethod: "GET", Path: "/v1/desktop/sources/{source_id}/sync/status", TargetField: "source_id", RequestSchema: "pinax.sync.status.request.v1", ResponseSchema: "pinax.projection.v1"},
		{Name: "syncDiff", HTTPMethod: "POST", Path: "/v1/desktop/sources/{source_id}/sync:diff", TargetField: "source_id", RequestSchema: "pinax.sync.diff.request.v1", ResponseSchema: "pinax.projection.v1"},
		{Name: "syncPush", HTTPMethod: "POST", Path: "/v1/desktop/sources/{source_id}/sync:push", TargetField: "source_id", RequestSchema: "pinax.sync.push.request.v1", ResponseSchema: "pinax.projection.v1", Idempotent: true},
		{Name: "syncPull", HTTPMethod: "POST", Path: "/v1/desktop/sources/{source_id}/sync:pull", TargetField: "source_id", RequestSchema: "pinax.sync.pull.request.v1", ResponseSchema: "pinax.projection.v1", Idempotent: true},
		{Name: "listSyncConflicts", HTTPMethod: "GET", Path: "/v1/desktop/sources/{source_id}/sync/conflicts", TargetField: "source_id", RequestSchema: "pinax.sync.conflicts.list.request.v1", ResponseSchema: "pinax.projection.v1"},
		{Name: "diffSyncConflict", HTTPMethod: "GET", Path: "/v1/desktop/sources/{source_id}/sync/conflicts:diff", TargetField: "source_id", RequestSchema: "pinax.sync.conflicts.diff.request.v1", ResponseSchema: "pinax.projection.v1"},
		{Name: "resolveSyncConflict", HTTPMethod: "POST", Path: "/v1/desktop/sources/{source_id}/sync/conflicts:resolve", TargetField: "source_id", RequestSchema: "DesktopSyncConflictResolveInput", ResponseSchema: "pinax.projection.v1", Idempotent: true},
		{Name: "listReadingState", HTTPMethod: "GET", Path: "/v1/desktop/reading-state", RequestSchema: "PageQuery", ResponseSchema: "ReadingStatePage"},
		{Name: "updateReadingState", HTTPMethod: "PUT", Path: "/v1/desktop/reading-state", RequestSchema: "UpdateReadingState", ResponseSchema: "ReadingState", Idempotent: true},
		{Name: "lookupReadingMutation", HTTPMethod: "POST", Path: "/v1/desktop/reading-state/lookup", RequestSchema: "LookupReadingMutation", ResponseSchema: "ReadingMutationResult"},
		{Name: "listTopicPins", HTTPMethod: "GET", Path: "/v1/desktop/topic-pins", RequestSchema: "PageQuery", ResponseSchema: "TopicPinPage"},
		{Name: "updateTopicPin", HTTPMethod: "PUT", Path: "/v1/desktop/topic-pins", RequestSchema: "UpdateTopicPin", ResponseSchema: "TopicPin", Idempotent: true},
		{Name: "lookupTopicPinMutation", HTTPMethod: "POST", Path: "/v1/desktop/topic-pins/lookup", RequestSchema: "TopicPinLookup", ResponseSchema: "TopicPinLookupResult"},
		{Name: "exportSource", HTTPMethod: "POST", Path: "/v1/desktop/exports", RequestSchema: "ExportSourceInput", ResponseSchema: "ExportOperation", Idempotent: true, NativeSelection: true},
		{Name: "getExport", HTTPMethod: "GET", Path: "/v1/desktop/exports/{operation_id}", TargetField: "operation_id", RequestSchema: "EmptyRequest", ResponseSchema: "ExportOperation"},
		{Name: "lookupExport", HTTPMethod: "POST", Path: "/v1/desktop/exports/lookup", RequestSchema: "ExportLookup", ResponseSchema: "ExportLookupResult"},
	}
	for i := range methods {
		methods[i].Status = "planned"
	}
	return methods
}

type EmptyRequest struct{}

// DesktopSyncConflictResolveInput deliberately excludes Pinax's merged_path.
// A renderer may choose only owner-defined conflict outcomes; a host-side
// bounded input contract can be added later when merge content has its own
// proposal and ticket semantics.
type DesktopSyncConflictResolveInput struct {
	File       string `json:"file" jsonschema:"minLength=1,maxLength=1024"`
	KeepLocal  bool   `json:"keep_local"`
	KeepRemote bool   `json:"keep_remote"`
	Yes        bool   `json:"yes"`
}

// DesktopEditorDraftInput is the renderer-safe subset of a writer proposal.
// The host derives the owner change, fixed note reference and approval context;
// renderer code never supplies a vault path, bearer or operation identity.
type DesktopEditorDraftInput struct {
	ObjectID string `json:"object_id" jsonschema:"minLength=1,maxLength=256"`
	Revision string `json:"revision" jsonschema:"pattern=^sha256:[a-f0-9]{64}$"`
	Body     string `json:"body" jsonschema:"maxLength=65536"`
}

// ExportSourceInput is renderer-safe; main alone supplies native selection and
// destination revision after the user completes the OS dialog.
type ExportSourceInput struct {
	TargetRef ReadingRef `json:"target_ref"`
}

type ReadingMutationResult struct {
	Found bool          `json:"found"`
	State *ReadingState `json:"state" jsonschema:"nullable"`
}

type DesktopCapabilities struct {
	SchemaVersion string `json:"schema_version" jsonschema:"enum=drivebridge.desktop.v1alpha1"`
	// Available is a runtime intersection, never copied from IPCMethods.
	Available []string `json:"available" jsonschema:"maxItems=64,uniqueItems=true"`
}

// DesktopError never contains upstream provider error text or private paths.
// Origin/code retain the bounded owner identifier, not an owner payload.
type DesktopError struct {
	Origin      string  `json:"origin" jsonschema:"enum=drivebridge,enum=pinax"`
	Code        string  `json:"code" jsonschema:"minLength=1,maxLength=128,pattern=^[a-z][a-z0-9_]*$"`
	Message     string  `json:"message" jsonschema:"minLength=1,maxLength=512"`
	Recovery    string  `json:"recovery" jsonschema:"enum=none,enum=reauthorize,enum=refresh,enum=lookup_original,enum=owner_required"`
	OperationID *string `json:"operation_id" jsonschema:"nullable,minLength=1,maxLength=128"`
}
