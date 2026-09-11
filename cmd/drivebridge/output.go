package main

import (
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
)

type outputMode struct{ JSON, Agent, Events, Explain bool }

func (m outputMode) write(command, status, summary string, data any) error {
	mode := "summary"
	if m.JSON {
		mode = "json"
	} else if m.Agent {
		mode = "agent"
	} else if m.Events {
		mode = "events"
	} else if m.Explain {
		mode = "explain"
	}
	envelope := map[string]any{"spec_version": "1.0", "mode": mode, "command": "drivebridge." + command, "status": status, "summary": summary}
	if data != nil {
		envelope["data"] = data
	}
	if status == "failed" {
		envelope["error"] = map[string]string{"code": "command_failed", "message": summary}
	}
	switch mode {
	case "json":
		return json.NewEncoder(os.Stdout).Encode(envelope)
	case "events":
		enc := json.NewEncoder(os.Stdout)
		if err := enc.Encode(map[string]any{"type": "start", "seq": 1, "command": envelope["command"]}); err != nil {
			return err
		}
		envelope["seq"] = 2
		envelope["type"] = "end"
		if status == "failed" {
			envelope["type"] = "error"
		}
		return enc.Encode(envelope)
	case "agent":
		fmt.Printf("spec_version=1.0\nmode=agent\ncommand=drivebridge.%s\nstatus=%s\n", command, status)
		// Scalar facts only; nested payloads stay in --json.
		if data, ok := envelope["data"].(map[string]any); ok {
			keys := make([]string, 0, len(data))
			for k := range data {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				switch v := data[k].(type) {
				case string, bool, int, int64, float64:
					if s, isStr := v.(string); isStr && strings.ContainsAny(s, "\n") {
						continue
					}
					fmt.Printf("fact.%s=%v\n", k, v)
				}
			}
		}
	case "explain":
		fmt.Printf("Conclusion: %s\nEvidence: Local application projection; no live provider readiness is inferred.\n", summary)
	default:
		fmt.Println(summary)
	}
	return nil
}
