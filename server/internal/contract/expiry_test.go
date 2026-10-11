package contract

import (
	"encoding/json"
	"testing"
)

func TestExpiryCreateAndPatch(t *testing.T) {
	for _, tc := range []struct {
		body         string
		patch, valid bool
		mode         string
	}{
		{`{}`, false, true, "never"}, {`{}`, true, true, ""},
		{`{"expiry_mode":"never"}`, false, true, "never"},
		{`{"expiry_mode":null}`, false, false, ""},
		{`{"expiry_mode":"never","expires_in_seconds":null}`, false, false, ""},
		{`{"expiry_mode":"never","expires_in_seconds":0}`, false, false, ""},
		{`{"expiry_mode":"duration"}`, false, false, ""},
		{`{"expiry_mode":"duration","expires_in_seconds":3600}`, true, true, "duration"},
		{`{"expires_in_seconds":3600}`, true, false, ""},
		{`{"expiry_mode":"duration","expires_in_seconds":42}`, false, false, ""},
		{`{"expiry_mode":"unknown"}`, false, false, ""},
	} {
		t.Run(tc.body, func(t *testing.T) {
			var input ExpiryInput
			if err := json.Unmarshal([]byte(tc.body), &input); err != nil {
				t.Fatal(err)
			}
			mode, _, err := input.Resolve(tc.patch)
			if (err == nil) != tc.valid {
				t.Fatalf("valid=%v err=%v", tc.valid, err)
			}
			if tc.valid && mode != tc.mode {
				t.Fatalf("mode=%q", mode)
			}
		})
	}
}

func TestExpiryAllDurations(t *testing.T) {
	for _, seconds := range []int64{3600, 86400, 604800, 2592000, 7776000, 31536000} {
		in := ExpiryInput{ExpiryMode: Field[string]{Present: true, Value: "duration"}, ExpiresInSeconds: Field[int64]{Present: true, Value: seconds}}
		mode, got, err := in.Resolve(false)
		if err != nil || mode != "duration" || got != seconds {
			t.Fatalf("%d: %q %d %v", seconds, mode, got, err)
		}
	}
}
