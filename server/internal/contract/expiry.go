package contract

import "errors"

var ErrInvalidExpiry = errors.New("invalid_expiry")

// ExpiryInput is shared by create and PATCH validation. Missing PATCH fields
// preserve the existing expiry; explicit null never means unlimited access.
type ExpiryInput struct {
	ExpiryMode       Field[string] `json:"expiry_mode"`
	ExpiresInSeconds Field[int64]  `json:"expires_in_seconds"`
}

// Resolve returns an empty mode only for an unchanged PATCH expiry.
// Time is deliberately excluded: the application fixes its transaction time.
func (in ExpiryInput) Resolve(patch bool) (string, int64, error) {
	mode := in.ExpiryMode
	seconds := in.ExpiresInSeconds
	if !mode.Present {
		if seconds.Present {
			return "", 0, ErrInvalidExpiry
		}
		if patch {
			return "", 0, nil
		}
		return "never", 0, nil
	}
	if mode.Null {
		return "", 0, ErrInvalidExpiry
	}
	switch mode.Value {
	case "never":
		if !seconds.Present {
			return "never", 0, nil
		}
	case "duration":
		if seconds.Present && !seconds.Null {
			switch seconds.Value {
			case 3600, 86400, 604800, 2592000, 7776000, 31536000:
				return "duration", seconds.Value, nil
			}
		}
	}
	return "", 0, ErrInvalidExpiry
}
