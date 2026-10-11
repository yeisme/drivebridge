package desktopcontract

import (
	"strings"
	"time"

	"github.com/invopop/jsonschema"
	"github.com/wk8/go-ordered-map/v2"
)

func condition(field string, values ...any) *jsonschema.Schema {
	props := orderedmap.New[string, *jsonschema.Schema]()
	props.Set(field, &jsonschema.Schema{Enum: values})
	return &jsonschema.Schema{Properties: props, Required: []string{field}}
}
func lookupSchema(schema *jsonschema.Schema, field string) *jsonschema.Schema {
	schema.If = condition("found", false)
	schema.Then = condition(field, nil)
	schema.Else = &jsonschema.Schema{Not: condition(field, nil)}
	return schema
}
func bindingSchemas() map[string]*jsonschema.Schema {
	r := jsonschema.Reflector{Anonymous: true, DoNotReference: true}
	preview := r.Reflect(BindingPreview{})
	scopes, _ := preview.Properties.Get("scopes")
	scopes.Items.Enum = []any{string(ScopeMetadata), string(ScopeBody), string(ScopeRelations), string(ScopeAssets)}
	scopes.Contains = &jsonschema.Schema{Enum: []any{string(ScopeMetadata)}}
	preview.If = condition("decision", "ready")
	preview.Then = &jsonschema.Schema{AllOf: []*jsonschema.Schema{condition("reason", "none"), condition("pending_mutations", 0)}}
	preview.Else = &jsonschema.Schema{Not: condition("reason", "none")}
	confirm := r.Reflect(ConfirmBinding{})
	confirm.AllOf = []*jsonschema.Schema{condition("confirmed", true)}
	release := r.Reflect(ReleaseProtection{})
	release.AllOf = []*jsonschema.Schema{condition("confirmed", true)}
	binding := r.Reflect(BindingView{})
	binding.If = condition("state", "released")
	binding.Then = condition("protection_active", false)
	binding.Else = condition("protection_active", true)
	receipt := r.Reflect(BindingMutationReceipt{})
	receipt.Properties.Set("binding", binding)
	confirmBinding := r.Reflect(BindingView{})
	confirmBinding.AllOf = []*jsonschema.Schema{condition("state", "active"), condition("protection_active", true)}
	releaseBinding := r.Reflect(BindingView{})
	releaseBinding.AllOf = []*jsonschema.Schema{condition("state", "released"), condition("protection_active", false)}
	thenProps := orderedmap.New[string, *jsonschema.Schema]()
	thenProps.Set("binding", confirmBinding)
	elseProps := orderedmap.New[string, *jsonschema.Schema]()
	elseProps.Set("binding", releaseBinding)
	receipt.If = condition("action", "confirm")
	receipt.Then = &jsonschema.Schema{Properties: thenProps}
	receipt.Else = &jsonschema.Schema{Properties: elseProps}
	lookup := lookupSchema(r.Reflect(BindingLookupResult{}), "receipt")
	lookup.Properties.Set("receipt", &jsonschema.Schema{OneOf: []*jsonschema.Schema{receipt, {Type: "null"}}})
	result := map[string]*jsonschema.Schema{
		"NativeSelection": r.Reflect(NativeSelection{}), "BindingPreview": preview, "ConfirmBinding": confirm,
		"SetSourceVisibility": r.Reflect(SetSourceVisibility{}), "ReleaseProtection": release, "BindingView": binding,
		"BindingMutationLookup": r.Reflect(BindingMutationLookup{}), "BindingMutationReceipt": receipt, "BindingLookupResult": lookup,
		"TopicRef": r.Reflect(TopicRef{}), "UpdateTopicPin": r.Reflect(UpdateTopicPin{}), "TopicPin": r.Reflect(TopicPin{}),
		"TopicPinPage": r.Reflect(TopicPinPage{}), "TopicPinLookup": r.Reflect(TopicPinLookup{}), "TopicPinLookupResult": lookupSchema(r.Reflect(TopicPinLookupResult{}), "pin"),
	}
	for _, schema := range result {
		schema.Version = ""
	}
	return result
}
func bindingFixtures() map[string]any {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	digest := "sha256:" + strings.Repeat("b", 64)
	binding := BindingView{BindingID: "binding-fixture", SourceID: "source-fixture", InstanceID: "instance-fixture", State: "active", Revision: 1, GuardRevision: 1, ProtectionActive: true, UpdatedAt: now}
	topic := TopicRef{SourceID: "source-fixture", Kind: "tag", OwnerTargetID: "tag-v1:" + strings.Repeat("a", 64)}
	pin := TopicPin{TargetRef: topic, Pinned: true, Revision: 1, UpdatedAt: now}
	return map[string]any{
		"NativeSelection":     NativeSelection{NativeSelectionID: "native-fixture"},
		"BindingPreview":      BindingPreview{PreviewID: "preview-fixture", PreviewDigest: digest, DisplayName: "Fixture notes", Scopes: []ReaderScope{ScopeMetadata, ScopeBody}, Decision: "ready", Reason: "none", ObservedAt: now, ExpiresAt: now.Add(5 * time.Minute)},
		"ConfirmBinding":      ConfirmBinding{PreviewID: "preview-fixture", PreviewDigest: digest, Confirmed: true},
		"SetSourceVisibility": SetSourceVisibility{ExpectedRevision: 1, Hidden: true}, "ReleaseProtection": ReleaseProtection{ExpectedRevision: 1, Confirmed: true},
		"BindingView": binding, "BindingMutationLookup": BindingMutationLookup{Action: "confirm", IdempotencyKey: "original-fixture"},
		"BindingMutationReceipt": BindingMutationReceipt{OperationID: "operation-fixture", Action: "confirm", Binding: binding}, "BindingLookupResult": BindingLookupResult{Found: false},
		"TopicRef": topic, "UpdateTopicPin": UpdateTopicPin{TargetRef: topic, Pinned: true}, "TopicPin": pin, "TopicPinPage": TopicPinPage{Items: []TopicPin{pin}, ObservedAt: now},
		"TopicPinLookup": TopicPinLookup{IdempotencyKey: "original-fixture"}, "TopicPinLookupResult": TopicPinLookupResult{Found: true, Pin: &pin},
	}
}
