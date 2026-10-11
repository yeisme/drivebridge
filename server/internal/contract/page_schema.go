package contract

import "github.com/invopop/jsonschema"

func pageEnvelopeSchema[T any](item *jsonschema.Schema) *jsonschema.Schema {
	r := jsonschema.Reflector{Anonymous: true, DoNotReference: true}
	envelope := r.Reflect(DataEnvelope[Page[T]]{})
	page, _ := envelope.Properties.Get("data")
	items, _ := page.Properties.Get("items")
	items.Items = item
	page.AllOf = []*jsonschema.Schema{{
		If:   propertyCondition("total_kind", "unknown"),
		Then: propertyCondition("total", nil),
		Else: &jsonschema.Schema{Not: propertyCondition("total", nil)},
	}}
	return envelope
}

func PageSchemas() map[string]*jsonschema.Schema {
	responses := ResponseSchemas()
	r := jsonschema.Reflector{Anonymous: true, DoNotReference: true}
	invocations := r.Reflect(DataEnvelope[InvocationPage]{})
	invocationData, _ := invocations.Properties.Get("data")
	base, _ := pageEnvelopeSchema[InvocationView](r.Reflect(InvocationView{})).Properties.Get("data")
	invocationData.AllOf = base.AllOf
	return map[string]*jsonschema.Schema{
		"ResourcePageEnvelope":     pageEnvelopeSchema[ResourceView](resourceTargetSchema(r.Reflect(ResourceView{}))),
		"TaskPageEnvelope":         pageEnvelopeSchema[TaskProjection](r.Reflect(TaskProjection{})),
		"GrantPageEnvelope":        pageEnvelopeSchema[GrantView](responses["GrantView"]),
		"InvocationPageEnvelope":   invocations,
		"SubmissionPageEnvelope":   pageEnvelopeSchema[SubmissionView](r.Reflect(SubmissionView{})),
		"SuggestionPageEnvelope":   pageEnvelopeSchema[LabelSuggestionView](r.Reflect(LabelSuggestionView{})),
		"GroupPageEnvelope":        pageEnvelopeSchema[AgentGroupView](responses["AgentGroupView"]),
		"OrganizationPageEnvelope": pageEnvelopeSchema[AgentOrganizationView](responses["AgentOrganizationView"]),
	}
}
