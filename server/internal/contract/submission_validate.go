package contract

import (
	"encoding/hex"
	"mime"
	"strings"
	"unicode/utf8"
)

// Validate checks metadata only; quotas, staging identity, authorization and
// content digest verification are enforced by the application and transfer.
func (in CreateSubmissionRequest) Validate() error {
	if !validName(in.Name, 255) || strings.ContainsAny(in.Name, "/\\\x00\r\n") || in.Name == "." || in.Name == ".." || in.SizeBytes < 0 {
		return ErrInvalidRequest
	}
	if _, _, err := mime.ParseMediaType(in.MIMEType); err != nil {
		return ErrInvalidRequest
	}
	if len(in.SHA256) != 64 || in.SHA256 != strings.ToLower(in.SHA256) {
		return ErrInvalidRequest
	}
	if _, err := hex.DecodeString(in.SHA256); err != nil {
		return ErrInvalidRequest
	}
	for _, value := range []string{in.Caller, in.TaskRef} {
		if !utf8.ValidString(value) || utf8.RuneCountInString(value) > 120 || strings.ContainsAny(value, "\x00\r\n") {
			return ErrInvalidRequest
		}
	}
	return nil
}

func (in LabelSuggestionRequest) Validate() error {
	if strings.TrimSpace(in.VersionID) == "" || len(in.Labels) < 1 || len(in.Labels) > 100 {
		return ErrInvalidRequest
	}
	for _, label := range in.Labels {
		if !validName(label, 64) {
			return ErrInvalidRequest
		}
	}
	return nil
}
