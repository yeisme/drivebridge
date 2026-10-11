package desktopcontract

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestDesktopReadingRefIdentity(t *testing.T) {
	for _, name := range []string{"FileRef", "PinaxRef"} {
		raw, _ := json.Marshal(Fixtures()[name])
		var ref ReadingRef
		if err := json.Unmarshal(raw, &ref); err != nil {
			t.Fatal(err)
		}
		if err := ref.Validate(); err != nil {
			t.Fatal(err)
		}
		mixed := ref
		if ref.Kind == "file" {
			mixed.SourceID = "other"
		} else {
			mixed.FileID = "other"
		}
		if mixed.Validate() == nil {
			t.Fatal("mixed identity accepted")
		}
		ref.InstanceID = ""
		if ref.Validate() == nil {
			t.Fatal("missing instance accepted")
		}
	}
	raw, _ := json.Marshal(Fixtures()["PinaxRef"])
	var ref ReadingRef
	_ = json.Unmarshal(raw, &ref)
	ref.URI = "pinax://note/other?v=" + ref.Revision
	if ref.Validate() == nil {
		t.Fatal("URI identity drift accepted")
	}
}

func TestDesktopReadingStateMutation(t *testing.T) {
	raw, _ := json.Marshal(Fixtures()["UpdateReadingState"])
	var req UpdateReadingState
	_ = json.Unmarshal(raw, &req)
	if req.Validate() != nil {
		t.Fatal("fixture rejected")
	}
	req.Favorite = nil
	if req.Validate() == nil {
		t.Fatal("empty mutation")
	}
	req.Position = &Position{Revision: "wrong", Anchor: "heading"}
	if req.Validate() == nil {
		t.Fatal("wrong revision")
	}
	req.Position.Revision = req.TargetRef.VersionID
	if req.Validate() != nil {
		t.Fatal("valid position rejected")
	}
	clearFalse := false
	req.ClearPosition = &clearFalse
	if req.Validate() == nil {
		t.Fatal("false clear flag accepted")
	}
	clear := true
	req.ClearPosition = &clear
	if req.Validate() == nil {
		t.Fatal("conflicting position")
	}
}

func TestDesktopContractDeterministicAndNotAvailable(t *testing.T) {
	for _, kind := range []string{"openapi", "fixtures", "response-schemas"} {
		first, err := Encode(kind)
		if err != nil {
			t.Fatal(err)
		}
		second, err := Encode(kind)
		if err != nil || string(first) != string(second) {
			t.Fatal("unstable generation")
		}
		for _, forbidden := range []string{"vault_root", "bearer", "body_text", "canonical_root"} {
			if strings.Contains(string(first), forbidden) {
				t.Fatal("private field in contract")
			}
		}
	}
	if len(OpenAPI()["paths"].(map[string]any)) != 0 {
		t.Fatal("unimplemented endpoint advertised")
	}
	if _, err := Encode("bad"); err == nil {
		t.Fatal("unknown output accepted")
	}
}
