package service

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
)

// TestWriteErrorStatusAndMessage pins the status mapping and the static
// message contract: every envelope carries error.code plus a non-empty
// message, and third-party text never leaks into either field.
func TestWriteErrorStatusAndMessage(t *testing.T) {
	cases := []struct {
		code   string
		status int
	}{
		{"unauthorized", 401},
		{"onedrive_authorization_required", 401},
		{"forbidden", 403},
		{"csrf_rejected", 403},
		{"not_found", 404},
		{"object_missing", 404},
		{"idempotency_conflict", 409},
		{"version_conflict", 409},
		{"operation_closed", 409},
		{"export_conflict", 409},
		{"user_key_duplicate", 409},
		{"quota_exceeded", 413},
		{"invalid_expiry", 422},
		{"throttled", 429},
		{"disabled", 503},
		{"storage_disconnected", 503},
		{"storage_unavailable", 503},
		{"storage_write_failed", 503},
		{"storage_read_failed", 503},
		{"storage_delete_failed", 503},
		{"transfer_failed", 503},
		{"delivery_unavailable", 503},
		{"audit_unavailable", 503},
		{"internal_error", 500},
		{"database_unavailable", 500},
		{"disk_write_failed", 500},
		{"encryption_unavailable", 500},
		// Unmapped request-level codes stay 400 with the generic message.
		{"invalid_upload", 400},
		{"unsupported", 400},
	}
	for _, c := range cases {
		rec := httptest.NewRecorder()
		writeError(rec, code(c.code))
		if rec.Code != c.status {
			t.Fatalf("%s: status %d, want %d", c.code, rec.Code, c.status)
		}
		var envelope struct {
			Error struct {
				Code    string `json:"code"`
				Message string `json:"message"`
			} `json:"error"`
		}
		if json.Unmarshal(rec.Body.Bytes(), &envelope) != nil {
			t.Fatalf("%s: body %s", c.code, rec.Body.String())
		}
		if envelope.Error.Code != c.code {
			t.Fatalf("%s: code round-trip got %q", c.code, envelope.Error.Code)
		}
		if envelope.Error.Message == "" {
			t.Fatalf("%s: message must never be empty", c.code)
		}
	}
}

// TestWriteErrorCollapsesUnsafeText keeps the redaction boundary: a long or
// punctuation-bearing error string collapses to internal_error.
func TestWriteErrorCollapsesUnsafeText(t *testing.T) {
	rec := httptest.NewRecorder()
	writeError(rec, errUnsafe("provider exploded: https://example.invalid/secret?token=x"))
	if rec.Code != 500 {
		t.Fatalf("unsafe text must map to 500, got %d", rec.Code)
	}
	var envelope struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	json.Unmarshal(rec.Body.Bytes(), &envelope)
	if envelope.Error.Code != "internal_error" {
		t.Fatalf("unsafe text collapsed to %q", envelope.Error.Code)
	}
}

type errUnsafe string

func (e errUnsafe) Error() string { return string(e) }
