package contract

// RevisionRequest binds human lifecycle changes to an observed grant revision.
type RevisionRequest struct {
	ExpectedRevision uint64 `json:"expected_revision"`
}

// GrantReplayView never includes a secret, even after response loss.
type GrantReplayView struct {
	Grant      GrantView        `json:"grant"`
	Credential ReplayCredential `json:"credential"`
}

type ReplayCredential struct {
	State string `json:"state" jsonschema:"enum=not_replayable"`
}

type RotationReplayView struct {
	GrantID              string           `json:"grant_id"`
	CredentialGeneration uint64           `json:"credential_generation"`
	Credential           ReplayCredential `json:"credential"`
}
