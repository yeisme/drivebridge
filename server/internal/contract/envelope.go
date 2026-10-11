package contract

// DataEnvelope matches the existing HTTP success envelope, not MCP text output.
type DataEnvelope[T any] struct {
	Data T `json:"data"`
}

type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type ErrorEnvelope struct {
	Error ErrorDetail `json:"error"`
}

// Credential is never present in ordinary grant reads. Secret is delivered
// only by create/rotate and is not available in replay responses.
type IssuedCredential struct {
	Secret string `json:"secret"`
	State  string `json:"state" jsonschema:"enum=issued"`
}

type GrantIssuedView struct {
	Grant      GrantView        `json:"grant"`
	Credential IssuedCredential `json:"credential"`
}
