package contract

import (
	"strings"
	"testing"
)

func TestSubmissionMetadataValidation(t *testing.T) {
	base := CreateSubmissionRequest{Name: "reference.txt", MIMEType: "text/plain", SHA256: strings.Repeat("a", 64)}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"..", "a/b", "a\\b", "bad\x00name", " ", "bad\nname"} {
		in := base
		in.Name = name
		if in.Validate() == nil {
			t.Fatalf("accepted filename %q", name)
		}
	}
	for _, digest := range []string{"", strings.Repeat("g", 64), strings.Repeat("A", 64)} {
		in := base
		in.SHA256 = digest
		if in.Validate() == nil {
			t.Fatal("accepted invalid digest")
		}
	}
	in := base
	in.SizeBytes = -1
	if in.Validate() == nil {
		t.Fatal("accepted negative size")
	}
	in = base
	in.MIMEType = "not a media type"
	if in.Validate() == nil {
		t.Fatal("accepted invalid MIME")
	}
}

func TestSuggestionValues(t *testing.T) {
	if (LabelSuggestionRequest{VersionID: "v1", Labels: []string{"参考"}}).Validate() != nil {
		t.Fatal("valid suggestion rejected")
	}
	for _, labels := range [][]string{nil, {" "}, {strings.Repeat("中", 65)}} {
		if (LabelSuggestionRequest{VersionID: "v1", Labels: labels}).Validate() == nil {
			t.Fatal("invalid labels accepted")
		}
	}
}
