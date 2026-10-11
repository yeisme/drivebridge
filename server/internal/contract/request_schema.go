package contract

import (
	"reflect"

	"github.com/invopop/jsonschema"
	"github.com/wk8/go-ordered-map/v2"
)

// fieldSchema unwraps input presence wrappers. Null is forbidden unless a
// specific public field explicitly permits it (organization group removal).
func fieldSchema(t reflect.Type) *jsonschema.Schema {
	switch t {
	case reflect.TypeFor[Field[string]]():
		return &jsonschema.Schema{Type: "string"}
	case reflect.TypeFor[Field[int64]]():
		return &jsonschema.Schema{Type: "integer"}
	case reflect.TypeFor[Field[bool]]():
		return &jsonschema.Schema{Type: "boolean"}
	case reflect.TypeFor[Field[[]string]]():
		return &jsonschema.Schema{Type: "array", Items: &jsonschema.Schema{Type: "string"}}
	}
	return nil
}

func propertyCondition(name string, value any) *jsonschema.Schema {
	properties := orderedmap.New[string, *jsonschema.Schema]()
	if value == nil {
		properties.Set(name, &jsonschema.Schema{Type: "null"})
	} else {
		properties.Set(name, &jsonschema.Schema{Const: value})
	}
	return &jsonschema.Schema{Properties: properties, Required: []string{name}}
}

func expiryRules(s *jsonschema.Schema) {
	mode, _ := s.Properties.Get("expiry_mode")
	mode.Enum = []any{"never", "duration"}
	seconds, _ := s.Properties.Get("expires_in_seconds")
	seconds.Enum = []any{3600, 86400, 604800, 2592000, 7776000, 31536000}
	s.AllOf = append(s.AllOf, &jsonschema.Schema{
		If:   propertyCondition("expiry_mode", "duration"),
		Then: &jsonschema.Schema{Required: []string{"expires_in_seconds"}},
		Else: &jsonschema.Schema{Not: &jsonschema.Schema{Required: []string{"expires_in_seconds"}}},
	})
}

func actionRules(s *jsonschema.Schema) {
	actions, _ := s.Properties.Get("actions")
	one := uint64(1)
	actions.MinItems = &one
	actions.UniqueItems = true
	actions.Items = &jsonschema.Schema{Type: "string", Enum: []any{"files.read", "labels.suggest", "inbox.submit"}}
}

func RequestSchemas() map[string]*jsonschema.Schema {
	r := jsonschema.Reflector{Anonymous: true, DoNotReference: true, Mapper: fieldSchema}
	members := r.Reflect(CollectionMembersChangeRequest{})
	for _, field := range []string{"include", "exclude"} {
		ids, _ := members.Properties.Get(field)
		ids.UniqueItems = true
		ids.Items.Pattern = `^[a-f0-9]{48}$`
	}
	create := r.Reflect(CreateGrantRequest{})
	patch := r.Reflect(PatchGrantRequest{})
	for _, schema := range []*jsonschema.Schema{create, patch} {
		expiryRules(schema)
		actionRules(schema)
	}
	name, _ := patch.Properties.Get("name")
	min, max := uint64(1), uint64(120)
	name.MinLength, name.MaxLength = &min, &max
	name.Pattern = `\S`
	createName, _ := create.Properties.Get("name")
	createName.Pattern = `\S`
	organization := r.Reflect(PatchOrganizationRequest{})
	organization.Properties.Set("group_id", &jsonschema.Schema{OneOf: []*jsonschema.Schema{{Type: "string", MinLength: &min, Pattern: `\S`}, {Type: "null"}}})
	suggestion := r.Reflect(LabelSuggestionRequest{})
	labels, _ := suggestion.Properties.Get("labels")
	labelMin, labelMax := uint64(1), uint64(64)
	labels.Items.MinLength, labels.Items.MaxLength = &labelMin, &labelMax
	decision := r.Reflect(SuggestionDecisionRequest{})
	decisionLabels, _ := decision.Properties.Get("labels")
	decisionLabels.UniqueItems = true
	decisionLabels.Items.MinLength, decisionLabels.Items.MaxLength = &labelMin, &labelMax
	decisionLabels.Items.Pattern = `\S`
	positive := orderedmap.New[string, *jsonschema.Schema]()
	positive.Set("labels", &jsonschema.Schema{Type: "array", MinItems: &labelMin})
	zero := uint64(0)
	negative := orderedmap.New[string, *jsonschema.Schema]()
	negative.Set("labels", &jsonschema.Schema{Type: "array", MaxItems: &zero})
	decision.AllOf = []*jsonschema.Schema{{If: propertyCondition("decision", "accept"), Then: &jsonschema.Schema{Required: []string{"labels"}, Properties: positive}, Else: &jsonschema.Schema{Properties: negative}}}
	return map[string]*jsonschema.Schema{
		"LabelDefinitionUpdate":          r.Reflect(LabelDefinitionUpdate{}),
		"LabelMergePreview":              r.Reflect(LabelMergePreview{}),
		"LabelMergeConfirm":              r.Reflect(LabelMergeConfirm{}),
		"LabelMergeExecute":              r.Reflect(LabelMergeExecute{}),
		"PropertyMigrationPreview":       r.Reflect(PropertyMigrationPreview{}),
		"PropertyMigrationDecision":      r.Reflect(PropertyMigrationDecision{}),
		"PropertyMigrationLookup":        r.Reflect(PropertyMigrationLookup{}),
		"WorkspaceHoldCreate":            r.Reflect(WorkspaceHoldCreate{}),
		"WorkspaceHoldRelease":           r.Reflect(WorkspaceHoldRelease{}),
		"WorkspaceHoldLookup":            r.Reflect(WorkspaceHoldLookup{}),
		"FavoriteCreate":                 r.Reflect(FavoriteCreate{}),
		"FavoriteDelete":                 r.Reflect(FavoriteDelete{}),
		"FavoriteLookup":                 r.Reflect(FavoriteLookup{}),
		"FileLinkRebuild":                r.Reflect(FileLinkRebuild{}),
		"FileLinkLookup":                 r.Reflect(FileLinkLookup{}),
		"AutomationChangeRequest":        r.Reflect(AutomationChangeRequest{}),
		"AutomationExecuteRequest":       r.Reflect(AutomationExecuteRequest{}),
		"AutomationPreviewRequest":       r.Reflect(struct{}{}),
		"CreateFileCollectionRequest":    collectionCreateSchema(),
		"CollectionRule":                 collectionRuleSchema(),
		"CollectionRuleFilter":           collectionFilterSchema(),
		"CollectionRuleSort":             r.Reflect(CollectionRuleSort{}),
		"PropertySidecarPatch":           sidecarPatchSchema(r.Reflect(PropertySidecarPatch{})),
		"PropertySidecarLookup":          r.Reflect(PropertySidecarLookup{}),
		"PropertyDefinitionUpdate":       r.Reflect(PropertyDefinitionUpdate{}),
		"SnapshotCreationRequest":        r.Reflect(SnapshotCreationRequest{}),
		"CollectionMembersChangeRequest": members,
		"ResourceCheckRequest":           r.Reflect(ResourceCheckRequest{}),
		"RegisterResourceRequest":        resourceTargetSchema(r.Reflect(RegisterResourceRequest{})),
		"GrantChangeLookupRequest":       r.Reflect(GrantChangeLookupRequest{}),
		"SaveDocumentRequest":            r.Reflect(SaveDocumentRequest{}),
		"SuggestionDecisionRequest":      decision,
		"SubmissionDecisionRequest":      r.Reflect(SubmissionDecisionRequest{}),
		"ArchiveSubmissionRequest":       r.Reflect(ArchiveSubmissionRequest{}),
		"CreateGroupRequest":             r.Reflect(CreateGroupRequest{}),
		"RenameGroupRequest":             r.Reflect(RenameGroupRequest{}),
		"RevisionRequest":                r.Reflect(RevisionRequest{}),
		"CreateSubmissionRequest":        r.Reflect(CreateSubmissionRequest{}),
		"IdempotencyLookupRequest":       r.Reflect(IdempotencyLookupRequest{}),
		"LabelSuggestionRequest":         suggestion,
		"CreateGrantRequest":             create,
		"PatchGrantRequest":              patch,
		"PatchOrganizationRequest":       organization,
		"NotebookCreate":                 r.Reflect(NotebookCreate{}),
		"NotebookCreateLookup":           r.Reflect(NotebookCreateLookup{}),
		"NotebookPagePatch":              r.Reflect(NotebookPagePatch{}),
	}
}
