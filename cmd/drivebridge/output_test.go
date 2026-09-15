package main

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge/internal/bridge"
)

// capture redirects stdout and stderr around fn and returns both buffers.
func capture(t *testing.T, fn func()) (string, string) {
	t.Helper()
	outR, outW, _ := os.Pipe()
	errR, errW, _ := os.Pipe()
	oldOut, oldErr := os.Stdout, os.Stderr
	os.Stdout, os.Stderr = outW, errW
	outCh, errCh := make(chan string, 1), make(chan string, 1)
	go func() { outCh <- slurp(outR) }()
	go func() { errCh <- slurp(errR) }()
	fn()
	outW.Close()
	errW.Close()
	os.Stdout, os.Stderr = oldOut, oldErr
	return <-outCh, <-errCh
}

func slurp(r io.Reader) string {
	b, _ := io.ReadAll(r)
	return string(b)
}

func TestFailEnvelopesPerMode(t *testing.T) {
	data := map[string]any{"operation": map[string]any{"id": "op-1", "status": "interrupted"}}

	t.Run("json", func(t *testing.T) {
		out, _ := capture(t, func() {
			_ = outputMode{JSON: true}.fail("upload", "resume_required", "Operation op-1 stopped.", data)
		})
		var envelope map[string]any
		if err := json.Unmarshal([]byte(out), &envelope); err != nil {
			t.Fatalf("stdout is not one JSON object: %v (%q)", err, out)
		}
		if envelope["status"] != "failed" || envelope["mode"] != "json" || envelope["command"] != "drivebridge.upload" {
			t.Fatalf("envelope headers: %+v", envelope)
		}
		e, _ := envelope["error"].(map[string]any)
		if e["code"] != "resume_required" || e["message"] == "" {
			t.Fatalf("error object must carry the typed code: %+v", e)
		}
	})

	t.Run("agent", func(t *testing.T) {
		out, _ := capture(t, func() {
			_ = outputMode{Agent: true}.fail("upload", "resume_required", "Operation op-1 stopped.", data)
		})
		for _, want := range []string{"spec_version=1.0", "mode=agent", "command=drivebridge.upload", "status=failed", "error.code=resume_required"} {
			if !strings.Contains(out, want+"\n") {
				t.Fatalf("agent output missing %q: %q", want, out)
			}
		}
	})

	t.Run("events", func(t *testing.T) {
		out, _ := capture(t, func() {
			_ = outputMode{Events: true}.fail("upload", "resume_required", "Operation op-1 stopped.", data)
		})
		lines := strings.Split(strings.TrimSpace(out), "\n")
		if len(lines) != 2 {
			t.Fatalf("expected start+error events, got %d lines: %q", len(lines), out)
		}
		var start, end map[string]any
		json.Unmarshal([]byte(lines[0]), &start)
		json.Unmarshal([]byte(lines[1]), &end)
		if start["type"] != "start" || start["spec_version"] != "1.0" || start["command"] != "drivebridge.upload" {
			t.Fatalf("start event: %+v", start)
		}
		if end["type"] != "error" || end["seq"].(float64) <= start["seq"].(float64) {
			t.Fatalf("error event: %+v", end)
		}
	})

	t.Run("summary", func(t *testing.T) {
		_, errs := capture(t, func() {
			_ = outputMode{}.fail("upload", "resume_required", "Operation op-1 stopped.", data)
		})
		if !strings.Contains(errs, "resume_required") || !strings.Contains(errs, "Recommended next step:") {
			t.Fatalf("human failure needs code + one next step on stderr: %q", errs)
		}
	})

	t.Run("explain", func(t *testing.T) {
		out, _ := capture(t, func() {
			_ = outputMode{Explain: true}.fail("upload", "resume_required", "Operation op-1 stopped.", data)
		})
		for _, want := range []string{"Conclusion:", "Evidence:", "Confidence:"} {
			if !strings.Contains(out, want) {
				t.Fatalf("explain missing %q: %q", want, out)
			}
		}
	})
}

func TestWriteEventsStartCarriesSpecVersion(t *testing.T) {
	out, _ := capture(t, func() {
		_ = outputMode{Events: true}.write("recent", "success", "ok", map[string]any{"count": 1})
	})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 2 {
		t.Fatalf("expected start+end, got %q", out)
	}
	var start map[string]any
	if json.Unmarshal([]byte(lines[0]), &start) != nil || start["spec_version"] != "1.0" {
		t.Fatalf("start event must carry spec_version: %q", lines[0])
	}
	var end map[string]any
	json.Unmarshal([]byte(lines[1]), &end)
	if end["type"] != "end" {
		t.Fatalf("end event: %q", lines[1])
	}
}

func TestFlattenFactsOneLevel(t *testing.T) {
	data := map[string]any{
		"count":     3,
		"name":      "op-1",
		"multiline": "a\nb",
		"files":     []any{1, 2, 3},
		"operation": map[string]any{"id": "x", "status": "done", "nested": map[string]any{"deep": "dropped"}},
	}
	rows := flattenFacts(data, "fact.", 0)
	joined := strings.Join(rows, "\n")
	for _, want := range []string{"fact.count=3", "fact.name=op-1", "fact.files_count=3", "fact.operation.id=x", "fact.operation.status=done"} {
		if !strings.Contains(joined, want) {
			t.Fatalf("missing %q in %q", want, joined)
		}
	}
	if strings.Contains(joined, "multiline") || strings.Contains(joined, "deep") {
		t.Fatalf("multi-line values and depth>1 must stay in --json: %q", joined)
	}
}

func TestTerminalCode(t *testing.T) {
	cases := []struct {
		op   bridge.Operation
		want string
	}{
		{bridge.Operation{Status: "failed", Error: "source_changed"}, "source_changed"},
		{bridge.Operation{Status: "interrupted"}, "resume_required"},
		{bridge.Operation{Status: "cancelled"}, "operation_cancelled"},
		{bridge.Operation{Status: "failed", Error: "provider said: boom"}, "transfer_failed"},
	}
	for _, c := range cases {
		if got := terminalCode(c.op); got != c.want {
			t.Fatalf("terminalCode(%+v) = %q, want %q", c.op, got, c.want)
		}
	}
}

func TestCodifyBoundaries(t *testing.T) {
	code, _ := codify(fail("state_locked", "held"))
	if code != "state_locked" {
		t.Fatalf("cliFailure code: %q", code)
	}
	code, _ = codify(io.ErrUnexpectedEOF)
	if code != "command_failed" {
		t.Fatalf("third-party error must collapse: %q", code)
	}
	code, _ = codify(io.ErrClosedPipe)
	if code != "command_failed" {
		t.Fatalf("sentence error must collapse: %q", code)
	}
	code, _ = codify(&noCodeError{})
	if code != "no_code_error" {
		t.Fatalf("typed snake_case error must pass: %q", code)
	}
}

type noCodeError struct{}

func (*noCodeError) Error() string { return "no_code_error" }

func TestEventStreamStartProgressEndOrdering(t *testing.T) {
	c := &cli{mode: outputMode{Events: true}}
	out, _ := capture(t, func() {
		stream := c.stream("upload")
		stream.progress(bridge.Operation{ID: "op-1", Transferred: 5, Bytes: 10})
		stream.progress(bridge.Operation{ID: "op-1", Transferred: 10, Bytes: 10})
		_ = stream.end("success", "Uploaded op-1.", nil)
	})
	lines := strings.Split(strings.TrimSpace(out), "\n")
	if len(lines) != 4 {
		t.Fatalf("expected start+2 progress+end, got %q", out)
	}
	last := 0
	for i, line := range lines {
		var event map[string]any
		if json.Unmarshal([]byte(line), &event) != nil {
			t.Fatalf("line %d is not JSON: %q", i, line)
		}
		seq := int(event["seq"].(float64))
		if seq <= last {
			t.Fatalf("seq must be monotonic: %q", out)
		}
		last = seq
	}
	if !strings.Contains(lines[0], `"type":"start"`) || !strings.Contains(lines[0], `"spec_version":"1.0"`) {
		t.Fatalf("first event must be start with spec_version: %q", lines[0])
	}
	if !strings.Contains(lines[1], `"type":"progress"`) || !strings.Contains(lines[1], `"bytes_transferred":5`) {
		t.Fatalf("progress event: %q", lines[1])
	}
	if !strings.Contains(lines[3], `"type":"end"`) {
		t.Fatalf("last event must be end: %q", lines[3])
	}
}

func TestEventStreamSummaryProgressOnStderr(t *testing.T) {
	c := &cli{mode: outputMode{}}
	_, errs := capture(t, func() {
		stream := c.stream("download")
		stream.progress(bridge.Operation{ID: "op-1", Transferred: 4, Bytes: 9})
		_ = stream.end("success", "Downloaded via op-1.", nil)
	})
	if !strings.Contains(errs, "4/9 bytes transferred") {
		t.Fatalf("human progress must refresh on stderr: %q", errs)
	}
}
