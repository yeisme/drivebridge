package contract

import "unicode/utf8"

// Validate checks a human decision's wire values. The service must still
// authorize the human, compare revisions and verify the selected label subset.
func (in SuggestionDecisionRequest) Validate() error {
	if in.BaseVersionID == "" || (in.Decision != "accept" && in.Decision != "reject") || len(in.Labels) > 100 {
		return ErrInvalidRequest
	}
	if in.Decision == "accept" && len(in.Labels) == 0 {
		return ErrInvalidRequest
	}
	if in.Decision == "reject" && len(in.Labels) != 0 {
		return ErrInvalidRequest
	}
	seen := map[string]bool{}
	for _, label := range in.Labels {
		if !validName(label, 64) || seen[label] {
			return ErrInvalidRequest
		}
		seen[label] = true
	}
	return nil
}

func (in SubmissionDecisionRequest) Validate() error {
	if in.VersionID == "" || (in.Decision != "accept" && in.Decision != "reject") || !utf8.ValidString(in.Reason) || utf8.RuneCountInString(in.Reason) > 500 {
		return ErrInvalidRequest
	}
	return nil
}
