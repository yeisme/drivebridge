package service

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

// Existing HTTP envelopes must stay consumable by the new public DTOs.
// This test does not install any of the planned workspace endpoints.
func TestWorkspaceContractExistingErrorEnvelope(t *testing.T) {
	for _, c := range []string{"unauthorized", "forbidden", "not_found", "revision_conflict", "cursor_expired", "database_unavailable"} {
		recorder := httptest.NewRecorder()
		writeError(recorder, code(c))
		got, err := contract.DecodeRequest[contract.ErrorEnvelope](recorder.Body)
		if err != nil || got.Error.Code != c || got.Error.Message == "" {
			t.Fatalf("%s: %+v %v", c, got, err)
		}
	}
}

func TestWorkspaceContractExistingSuccessEnvelope(t *testing.T) {
	recorder := httptest.NewRecorder()
	respond(recorder, []string{"existing"})
	var got contract.DataEnvelope[[]string]
	if err := json.Unmarshal(recorder.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if len(got.Data) != 1 || got.Data[0] != "existing" {
		t.Fatalf("old array response changed: %+v", got)
	}
}
