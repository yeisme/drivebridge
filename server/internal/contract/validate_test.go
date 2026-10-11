package contract

import (
	"strings"
	"testing"
)

func TestCreateGrantValidationRejectsInvalidActions(t *testing.T) {
	base := CreateGrantRequest{Name: "Research", Protocol: "mcp", SpaceID: "space", ConnectionID: "connection", Actions: []string{"files.read"}}
	if err := base.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, actions := range [][]string{nil, {}, {"write"}, {"files.read", "files.read"}, {"inbox.submit", "delete"}} {
		next := base
		next.Actions = actions
		if err := next.Validate(); err == nil {
			t.Fatalf("accepted %v", actions)
		}
	}
	for _, name := range []string{"", "  ", strings.Repeat("中", 121)} {
		next := base
		next.Name = name
		if err := next.Validate(); err == nil {
			t.Fatal("accepted invalid name")
		}
	}
}

func TestPatchValueValidation(t *testing.T) {
	for _, body := range []string{`{"expected_revision":0,"name":null}`, `{"expected_revision":0,"enabled":null}`, `{"expected_revision":0,"actions":null}`, `{"expected_revision":0,"actions":[]}`, `{"expected_revision":0,"prefix":null}`, `{"expected_revision":0,"expiry_mode":"duration"}`} {
		input, err := DecodeRequest[PatchGrantRequest](strings.NewReader(body))
		if err != nil {
			t.Fatal(err)
		}
		if input.Validate() == nil {
			t.Fatalf("accepted %s", body)
		}
	}
	for _, body := range []string{`{"expected_revision":0}`, `{"expected_revision":0,"enabled":false}`, `{"expected_revision":0,"prefix":""}`, `{"expected_revision":0,"expiry_mode":"never"}`} {
		input, err := DecodeRequest[PatchGrantRequest](strings.NewReader(body))
		if err != nil || input.Validate() != nil {
			t.Fatalf("rejected %s", body)
		}
	}
}
