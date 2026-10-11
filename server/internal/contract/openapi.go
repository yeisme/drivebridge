package contract

import "encoding/json"

// OpenAPIDocument publishes design contracts with explicit planned status.
// This build artifact is not runtime capability discovery.
func OpenAPIDocument() map[string]any {
	schemas := ResponseSchemas()
	for name, schema := range RequestSchemas() {
		schemas[name] = schema
	}
	for name, schema := range EnvelopeSchemas() {
		schemas[name] = schema
	}
	for name, schema := range PageSchemas() {
		schemas[name] = schema
	}
	for _, schema := range schemas {
		schema.Version = ""
	}
	return map[string]any{
		"openapi":           "3.1.0",
		"info":              map[string]string{"title": "DriveBridge personal workspace contracts", "version": "0.1.0"},
		"jsonSchemaDialect": "https://json-schema.org/draft/2020-12/schema",
		"paths":             RegisteredPaths(),
		"components": map[string]any{"schemas": schemas, "securitySchemes": map[string]any{
			"bearerAuth":  map[string]string{"type": "http", "scheme": "bearer", "description": "Scoped Bearer identity. Human-only routes reject Agent bearer; action routes enforce their registered grant action."},
			"sessionAuth": map[string]string{"type": "apiKey", "in": "cookie", "name": "__Host-db_session", "description": "Production session cookie; development uses db_session. Mutations require CSRF validation."},
		}},
	}
}

func EncodeOpenAPI() ([]byte, error) {
	return json.MarshalIndent(OpenAPIDocument(), "", "  ")
}
