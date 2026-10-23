package desktopcontract

import (
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/invopop/jsonschema"
)

// Schemas describes DTOs only. No endpoint or IPC is made available by export.
func Schemas() map[string]*jsonschema.Schema {
	r := jsonschema.Reflector{Anonymous: true, DoNotReference: true}
	update := r.Reflect(UpdateReadingState{})
	update.AnyOf = []*jsonschema.Schema{{Required: []string{"favorite"}}, {Required: []string{"read_later"}}, {Required: []string{"position"}}, {Required: []string{"clear_position"}}}
	clear, _ := update.Properties.Get("clear_position")
	clear.Enum = []any{true}
	update.Not = &jsonschema.Schema{Required: []string{"position", "clear_position"}}
	schemas := map[string]*jsonschema.Schema{
		"KnowledgeSource": r.Reflect(KnowledgeSource{}), "ReadingRef": r.Reflect(ReadingRef{}),
		"FileRef": r.Reflect(FileRef{}), "PinaxRef": r.Reflect(PinaxRef{}),
		"ReadingState": r.Reflect(ReadingState{}), "UpdateReadingState": update,
		"LookupReadingMutation": r.Reflect(LookupReadingMutation{}), "PageQuery": r.Reflect(PageQuery{}),
		"SourcePage": r.Reflect(SourcePage{}), "ReadingStatePage": r.Reflect(ReadingStatePage{}),
	}
	schemas["EmptyRequest"] = r.Reflect(EmptyRequest{})
	schemas["DesktopSyncConflictResolveInput"] = r.Reflect(DesktopSyncConflictResolveInput{})
	schemas["DesktopEditorDraftInput"] = r.Reflect(DesktopEditorDraftInput{})
	schemas["ExportSourceInput"] = r.Reflect(ExportSourceInput{})
	schemas["ReadingMutationResult"] = lookupSchema(r.Reflect(ReadingMutationResult{}), "state")
	schemas["DesktopCapabilities"] = r.Reflect(DesktopCapabilities{})
	schemas["DesktopError"] = r.Reflect(DesktopError{})
	for name, schema := range bindingSchemas() {
		schemas[name] = schema
	}
	for name, schema := range exportSchemas() {
		schemas[name] = schema
	}
	for _, schema := range schemas {
		schema.Version = ""
	}
	return schemas
}

func OpenAPI() map[string]any {
	return map[string]any{
		"openapi": "3.1.0", "info": map[string]string{"title": "DriveBridge desktop contracts", "version": Version},
		"jsonSchemaDialect":       "https://json-schema.org/draft/2020-12/schema",
		"x-implementation-status": "planned", "paths": map[string]any{},
		"components": map[string]any{"schemas": Schemas()},
	}
}

func Fixtures() map[string]any {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	file := ReadingRef{Kind: "file", InstanceID: "instance-fixture", SpaceID: "space-fixture", FileID: "file-fixture", VersionID: "version-fixture"}
	revision := "sha256:" + strings.Repeat("a", 64)
	pinax := ReadingRef{Kind: "pinax", InstanceID: "instance-fixture", SourceID: "source-fixture", ObjectID: "note_fixture", Revision: revision, URI: "pinax://note/note_fixture?v=" + revision}
	favorite := true
	source := KnowledgeSource{SourceID: "source-fixture", Kind: "pinax", InstanceID: "instance-fixture", DisplayName: "Fixture notes", State: "blocked", Capabilities: []string{}, BindingRevision: 1}
	state := ReadingState{TargetRef: file, Favorite: true, ReadLater: false, Revision: 1, UpdatedAt: now}
	fixtures := map[string]any{
		"KnowledgeSource": source, "ReadingRef": pinax,
		"FileRef": file, "PinaxRef": pinax, "ReadingState": state,
		"UpdateReadingState":    UpdateReadingState{TargetRef: file, ExpectedRevision: 0, Favorite: &favorite},
		"LookupReadingMutation": LookupReadingMutation{IdempotencyKey: "fixture-original-key"},
		"PageQuery":             PageQuery{}, "SourcePage": SourcePage{Items: []KnowledgeSource{source}, ObservedAt: now},
		"ReadingStatePage": ReadingStatePage{Items: []ReadingState{state}, ObservedAt: now},
	}
	fixtures["EmptyRequest"] = EmptyRequest{}
	fixtures["DesktopSyncConflictResolveInput"] = DesktopSyncConflictResolveInput{File: "notes/example.md", KeepLocal: true, Yes: true}
	fixtures["DesktopEditorDraftInput"] = DesktopEditorDraftInput{ObjectID: "note-fixture", Revision: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa", Body: "draft"}
	fixtures["ExportSourceInput"] = ExportSourceInput{TargetRef: pinax}
	fixtures["ReadingMutationResult"] = ReadingMutationResult{}
	fixtures["DesktopCapabilities"] = DesktopCapabilities{SchemaVersion: Version, Available: []string{}}
	fixtures["DesktopError"] = DesktopError{Origin: "drivebridge", Code: "owner_unavailable", Message: "The owner is unavailable.", Recovery: "owner_required"}
	for name, fixture := range bindingFixtures() {
		fixtures[name] = fixture
	}
	for name, fixture := range exportFixtures() {
		fixtures[name] = fixture
	}
	return fixtures
}

func Encode(kind string) ([]byte, error) {
	var value any
	switch kind {
	case "openapi":
		value = OpenAPI()
	case "response-schemas":
		value = Schemas()
	case "fixtures":
		value = Fixtures()
	case "ipc":
		value = IPCMethods()
	default:
		return nil, errors.New("unknown desktop contract output")
	}
	return json.MarshalIndent(value, "", "  ")
}
