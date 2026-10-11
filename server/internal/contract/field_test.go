package contract

import (
	"encoding/json"
	"testing"
)

func TestFieldPreservesPatchPresence(t *testing.T) {
	for _, tc := range []struct {
		input                string
		present, null, value bool
	}{
		{`{}`, false, false, false},
		{`{"pinned":null}`, true, true, false},
		{`{"pinned":false}`, true, false, false},
		{`{"pinned":true}`, true, false, true},
	} {
		t.Run(tc.input, func(t *testing.T) {
			var got struct {
				Pinned Field[bool] `json:"pinned"`
			}
			if err := json.Unmarshal([]byte(tc.input), &got); err != nil {
				t.Fatal(err)
			}
			if got.Pinned.Present != tc.present || got.Pinned.Null != tc.null || got.Pinned.Value != tc.value {
				t.Fatalf("unexpected field: %+v", got.Pinned)
			}
		})
	}
}

func TestFieldRejectsInvalidValueWithoutMutation(t *testing.T) {
	field := Field[bool]{Present: true, Value: true}
	if err := json.Unmarshal([]byte(`"false"`), &field); err == nil {
		t.Fatal("expected type error")
	}
	if !field.Present || !field.Value || field.Null {
		t.Fatalf("mutated on error: %+v", field)
	}
}

func TestFieldDecodeReplacesNullAndValue(t *testing.T) {
	var field Field[bool]
	for _, input := range []string{`true`, `null`, `false`} {
		if err := json.Unmarshal([]byte(input), &field); err != nil {
			t.Fatal(err)
		}
		if !field.Present || field.Null != (input == "null") || field.Value != (input == "true") {
			t.Fatalf("%s: %+v", input, field)
		}
	}
}
