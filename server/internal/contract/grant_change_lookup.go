package contract

import "time"

// GrantChangeLookupRequest identifies a committed lifecycle request, not a
// desired current state. Creation and rotation have separate lookup contracts.
type GrantChangeLookupRequest struct {
	Action         string `json:"action" jsonschema:"enum=disable,enum=enable,enum=revoke,enum=expiry"`
	IdempotencyKey string `json:"idempotency_key" jsonschema:"minLength=1,maxLength=200"`
}

func (r GrantChangeLookupRequest) Validate() error {
	switch r.Action {
	case "disable", "enable", "revoke", "expiry":
	default:
		return ErrInvalidRequest
	}
	if len(r.IdempotencyKey) == 0 || len(r.IdempotencyKey) > 200 {
		return ErrInvalidRequest
	}
	for _, ch := range r.IdempotencyKey {
		if ch < 33 || ch > 126 {
			return ErrInvalidRequest
		}
	}
	return nil
}

// GrantChangeReceiptView proves only that the original operation committed at
// result_revision. Later mutations may have superseded its resulting state.
type GrantChangeReceiptView struct {
	GrantID        string    `json:"grant_id"`
	Action         string    `json:"action" jsonschema:"enum=disable,enum=enable,enum=revoke,enum=expiry"`
	ResultRevision uint64    `json:"result_revision"`
	CommittedAt    time.Time `json:"committed_at"`
}
