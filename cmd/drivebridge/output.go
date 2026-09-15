package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/yeisme/drivebridge/internal/bridge"
)

type outputMode struct{ JSON, Agent, Events, Explain bool }

func (m outputMode) name() string {
	switch {
	case m.JSON:
		return "json"
	case m.Agent:
		return "agent"
	case m.Events:
		return "events"
	case m.Explain:
		return "explain"
	}
	return "summary"
}

func driveCommand(command string) string { return "drivebridge." + command }

func (m outputMode) write(command, status, summary string, data any) error {
	envelope := map[string]any{"spec_version": "1.0", "mode": m.name(), "command": driveCommand(command), "status": status, "summary": summary}
	if data != nil {
		envelope["data"] = data
	}
	switch m.name() {
	case "json":
		return json.NewEncoder(os.Stdout).Encode(envelope)
	case "events":
		enc := json.NewEncoder(os.Stdout)
		if err := enc.Encode(map[string]any{"type": "start", "seq": 1, "spec_version": "1.0", "command": envelope["command"]}); err != nil {
			return err
		}
		envelope["seq"] = 2
		envelope["type"] = "end"
		return enc.Encode(envelope)
	case "agent":
		return writeAgent(envelope, data)
	case "explain":
		fmt.Printf("Conclusion: %s\nEvidence: Local application projection; no live provider readiness is inferred.\n", summary)
	default:
		fmt.Println(summary)
	}
	return nil
}

// fail renders a failed projection in the active mode. Machine modes get a
// valid envelope carrying the typed error code; the human summary goes to
// stderr with the registry hint as the single recommended next step. It is
// the only sanctioned way to surface command failure — write() never takes a
// failed status.
func (m outputMode) fail(command, code, message string, data any) error {
	switch m.name() {
	case "json":
		envelope := map[string]any{"spec_version": "1.0", "mode": "json", "command": driveCommand(command), "status": "failed", "summary": message, "error": map[string]string{"code": code, "message": message}}
		if data != nil {
			envelope["data"] = data
		}
		return json.NewEncoder(os.Stdout).Encode(envelope)
	case "events":
		enc := json.NewEncoder(os.Stdout)
		if err := enc.Encode(map[string]any{"type": "start", "seq": 1, "spec_version": "1.0", "command": driveCommand(command)}); err != nil {
			return err
		}
		end := map[string]any{"type": "error", "seq": 2, "command": driveCommand(command), "error": map[string]string{"code": code, "message": message}}
		if data != nil {
			end["data"] = data
		}
		return enc.Encode(end)
	case "agent":
		fmt.Printf("spec_version=1.0\nmode=agent\ncommand=%s\nstatus=failed\nerror.code=%s\n", driveCommand(command), code)
		return nil
	case "explain":
		fmt.Printf("Conclusion: %s failed before completing; error code %s.\nEvidence: %s\nConfidence: high\n", command, code, message)
		return nil
	default:
		fmt.Fprintf(os.Stderr, "DriveBridge failed (%s): %s\nRecommended next step: %s\n", code, message, hintEN(code))
		return nil
	}
}

// writeAgent prints the stable key=value projection: the four required lines
// plus one fact.<key> per scalar leaf, flattening nested maps one level.
func writeAgent(envelope map[string]any, data any) error {
	fmt.Printf("spec_version=%s\nmode=agent\ncommand=%s\nstatus=%s\n", envelope["spec_version"], envelope["command"], envelope["status"])
	for _, row := range flattenFacts(data, "fact.", 0) {
		fmt.Println(row)
	}
	return nil
}

// flattenFacts renders scalar leaves as dotted key=value lines. Nested maps
// flatten one level (fact.operation.status=...); slices reduce to a count
// (fact.files_count=3); deeper nesting and multi-line values stay in --json.
func flattenFacts(data any, prefix string, depth int) []string {
	m, ok := data.(map[string]any)
	if !ok {
		return nil
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		v := m[k]
		switch t := v.(type) {
		case string:
			if !strings.ContainsAny(t, "\n") {
				out = append(out, fmt.Sprintf("%s%s=%s", prefix, k, t))
			}
		case bool, int, int64, float64:
			out = append(out, fmt.Sprintf("%s%s=%v", prefix, k, t))
		case []any:
			out = append(out, fmt.Sprintf("%s%s_count=%d", prefix, k, len(t)))
		case map[string]any:
			if depth == 0 {
				out = append(out, flattenFacts(t, prefix+k+".", depth+1)...)
			}
		}
	}
	return out
}

// eventStream renders one NDJSON command stream: start, zero or more
// progress events, and a terminal end|error event with a monotonic seq.
// Non-events modes degrade to the ordinary write/fail renderers; the human
// summary gets a single refreshed progress line on stderr.
type eventStream struct {
	mode       outputMode
	command    string
	seq        int
	live       bool
	progressed bool
}

func (c *cli) stream(command string) *eventStream {
	e := &eventStream{mode: c.mode, command: command}
	if c.mode.Events {
		e.live = true
		e.emit("start", map[string]any{"spec_version": "1.0"})
	}
	return e
}

func (e *eventStream) emit(kind string, fields map[string]any) {
	e.seq++
	event := map[string]any{"type": kind, "seq": e.seq, "command": driveCommand(e.command)}
	for k, v := range fields {
		event[k] = v
	}
	_ = json.NewEncoder(os.Stdout).Encode(event)
}

func (e *eventStream) progress(op bridge.Operation) {
	if e.live {
		e.emit("progress", map[string]any{"operation_id": op.ID, "bytes_transferred": op.Transferred, "bytes_total": op.Bytes})
		return
	}
	if e.mode.name() == "summary" {
		e.progressed = true
		fmt.Fprintf(os.Stderr, "\r%s: %d/%d bytes transferred", e.command, op.Transferred, op.Bytes)
	}
}

func (e *eventStream) end(status, summary string, data any) error {
	if !e.live {
		return e.mode.write(e.command, status, summary, data)
	}
	fields := map[string]any{"status": status, "summary": summary}
	if data != nil {
		fields["data"] = data
	}
	e.emit("end", fields)
	return nil
}

func (e *eventStream) fail(code, message string, data any) error {
	if !e.live {
		return e.mode.fail(e.command, code, message, data)
	}
	fields := map[string]any{"error": map[string]string{"code": code, "message": message}}
	if data != nil {
		fields["data"] = data
	}
	e.emit("error", fields)
	return nil
}
