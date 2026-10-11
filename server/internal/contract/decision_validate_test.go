package contract

import "testing"

func TestSuggestionDecisionValidation(t *testing.T) {
	for _, tc := range []struct {
		decision string
		labels   []string
		valid    bool
	}{
		{"accept", []string{"reference"}, true}, {"reject", nil, true},
		{"accept", nil, false}, {"reject", []string{"reference"}, false},
		{"accept", []string{"reference", "reference"}, false}, {"accept", []string{" "}, false}, {"unknown", nil, false},
	} {
		in := SuggestionDecisionRequest{BaseVersionID: "v1", Decision: tc.decision, Labels: tc.labels}
		if (in.Validate() == nil) != tc.valid {
			t.Fatalf("unexpected validation: %+v", tc)
		}
	}
}

func TestSubmissionDecisionRequiresVersionAndKnownDecision(t *testing.T) {
	if (SubmissionDecisionRequest{VersionID: "v1", Decision: "accept"}).Validate() != nil {
		t.Fatal("valid decision rejected")
	}
	if (SubmissionDecisionRequest{Decision: "accept"}).Validate() == nil {
		t.Fatal("version required")
	}
	if (SubmissionDecisionRequest{VersionID: "v1", Decision: "archive"}).Validate() == nil {
		t.Fatal("archive is not acceptance")
	}
}
