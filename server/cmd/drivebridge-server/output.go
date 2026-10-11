package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type outputMode struct{ json, agent, events, explain bool }

func (m outputMode) emit(command, status, message string) error {
	return m.emitData(command, status, message, nil)
}

// emitData adds a data payload (JSON envelope / agent facts) without changing
// the byte output of emit for commands that pass nil.
func (m outputMode) emitData(command, status, message string, data map[string]any) error {
	mode := "summary"
	if m.json {
		mode = "json"
	} else if m.agent {
		mode = "agent"
	} else if m.events {
		mode = "events"
	} else if m.explain {
		mode = "explain"
	}
	out := map[string]any{"spec_version": "1.0", "mode": mode, "command": "drivebridge-server." + command, "status": status, "summary": message}
	if data != nil {
		out["data"] = data
	}
	switch mode {
	case "json":
		return json.NewEncoder(os.Stdout).Encode(out)
	case "events":
		enc := json.NewEncoder(os.Stdout)
		if e := enc.Encode(map[string]any{"type": "start", "seq": 1, "spec_version": "1.0", "command": out["command"]}); e != nil {
			return e
		}
		out["type"] = "end"
		out["seq"] = 2
		return enc.Encode(out)
	case "agent":
		fmt.Printf("spec_version=1.0\nmode=agent\ncommand=drivebridge-server.%s\nstatus=%s\n", command, status)
		for _, row := range flattenFacts(data, "fact.", 0) {
			fmt.Println(row)
		}
	case "explain":
		fmt.Printf("Conclusion: %s\nEvidence: Local command result; live account readiness is not inferred.\n", message)
	default:
		fmt.Println(message)
	}
	return nil
}

// emitFailure renders a failed projection with the typed error code: machine
// modes keep a valid envelope on stdout; the human summary goes to stderr.
// emitData never carries a failed status.
func (m outputMode) emitFailure(command, code, message string, data map[string]any) error {
	mode := "summary"
	if m.json {
		mode = "json"
	} else if m.agent {
		mode = "agent"
	} else if m.events {
		mode = "events"
	} else if m.explain {
		mode = "explain"
	}
	switch mode {
	case "json":
		out := map[string]any{"spec_version": "1.0", "mode": "json", "command": "drivebridge-server." + command, "status": "failed", "summary": message, "error": map[string]string{"code": code, "message": message}}
		if data != nil {
			out["data"] = data
		}
		return json.NewEncoder(os.Stdout).Encode(out)
	case "events":
		enc := json.NewEncoder(os.Stdout)
		if e := enc.Encode(map[string]any{"type": "start", "seq": 1, "spec_version": "1.0", "command": "drivebridge-server." + command}); e != nil {
			return e
		}
		end := map[string]any{"type": "error", "seq": 2, "command": "drivebridge-server." + command, "error": map[string]string{"code": code, "message": message}}
		if data != nil {
			end["data"] = data
		}
		return enc.Encode(end)
	case "agent":
		fmt.Printf("spec_version=1.0\nmode=agent\ncommand=drivebridge-server.%s\nstatus=failed\nerror.code=%s\n", command, code)
		return nil
	case "explain":
		fmt.Printf("Conclusion: %s failed before completing; error code %s.\nEvidence: %s\nConfidence: high\n", command, code, message)
		return nil
	default:
		fmt.Fprintf(os.Stderr, "DriveBridge server failed (%s): %s\n", code, message)
		return nil
	}
}

// flattenFacts renders scalar leaves as dotted key=value lines, flattening
// nested maps one level; slices reduce to counts. Deeper structure and
// multi-line values stay in --json.
func flattenFacts(data map[string]any, prefix string, depth int) []string {
	keys := make([]string, 0, len(data))
	for k := range data {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		v := data[k]
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
