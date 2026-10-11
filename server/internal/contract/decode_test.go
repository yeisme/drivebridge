package contract

import (
	"strings"
	"testing"
)

func TestDecodeRequestRejectsUnsafeShapes(t *testing.T) {
	for _, body := range []string{`null`, `[]`, `{}`, `{"name":"file","path":"formal"}`, `{"name":"file"} {}`, `{"name":"file"} false`, `{"name":true}`, `{"name":"file"`} {
		got, err := DecodeRequest[CreateSubmissionRequest](strings.NewReader(body))
		if err != ErrInvalidRequest || got.Name != "" {
			t.Fatalf("body=%s got=%+v err=%v", body, got, err)
		}
	}
}

func TestDecodeRequestRequiresRevisionWithoutRejectingZero(t *testing.T) {
	for _, body := range []string{`{}`, `{"expected_revision":null}`, `{"expected_revision":0,"expected_revision":1}`} {
		if _, err := DecodeRequest[PatchOrganizationRequest](strings.NewReader(body)); err != ErrInvalidRequest {
			t.Fatalf("accepted %s", body)
		}
	}
}

func TestDecodeRequestRejectsNullOptionalScalar(t *testing.T) {
	type input struct {
		Name string `json:"name,omitempty"`
	}
	if _, err := DecodeRequest[input](strings.NewReader(`{"name":null}`)); err != ErrInvalidRequest {
		t.Fatal("null collapsed to empty string")
	}
	if _, err := DecodeRequest[input](strings.NewReader(`{"Name":"alias"}`)); err != ErrInvalidRequest {
		t.Fatal("case-insensitive alias accepted")
	}
	if _, err := DecodeRequest[input](strings.NewReader(`{}`)); err != nil {
		t.Fatal(err)
	}
}

func TestDecodeRequestPreservesFalseAndNull(t *testing.T) {
	got, err := DecodeRequest[PatchOrganizationRequest](strings.NewReader(`{"expected_revision":0,"pinned":false,"group_id":null}`))
	if err != nil {
		t.Fatal(err)
	}
	if !got.Pinned.Present || got.Pinned.Value || got.Pinned.Null || !got.GroupID.Null {
		t.Fatalf("presence lost: %+v", got)
	}
}
