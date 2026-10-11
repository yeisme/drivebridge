package service

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestPropertyValueStrictTypes(t *testing.T) {
	ref := `"drivebridge://instance/file/` + strings.Repeat("a", 48) + `/version/` + strings.Repeat("b", 48) + `"`
	tests := []struct {
		kind, raw string
		valid     bool
	}{
		{"text", `""`, true}, {"text", `"正文\n第二行"`, true}, {"text", `"\ud83d\ude00"`, true}, {"text", `"\\ud800"`, true}, {"text", `"\ud800"`, false}, {"text", `"\udc00"`, false}, {"text", `"\ud800\u0041"`, false}, {"text", `"\u0000"`, false}, {"text", `null`, false}, {"text", `42`, false},
		{"checkbox", `true`, true}, {"checkbox", `false`, true}, {"checkbox", `"true"`, false}, {"checkbox", `1`, false},
		{"number", `0`, true}, {"number", `-0`, true}, {"number", `1.25`, true}, {"number", `1e3`, true}, {"number", `1e-308`, true}, {"number", `9007199254740991`, true}, {"number", `-9007199254740991`, true}, {"number", `9007199254740992`, false}, {"number", `9007199254740991.1`, false}, {"number", `-9007199254740991.1`, false}, {"number", `1e309`, false}, {"number", `1e-999999999`, false}, {"number", `1e-309`, false}, {"number", `NaN`, false}, {"number", `"1.25"`, false}, {"number", `01`, false}, {"number", `1 2`, false},
		{"date", `"2024-02-29"`, true}, {"date", `"2023-02-29"`, false}, {"date", `"0000-01-01"`, false}, {"date", `"2026-1-01"`, false}, {"date", `"2026-01-01T00:00:00Z"`, false}, {"date", `20260101`, false},
		{"list", `[]`, true}, {"list", `["", "one", "one"]`, true}, {"list", `[1]`, false}, {"list", `[true]`, false}, {"list", `[{}]`, false}, {"list", `[["nested"]]`, false}, {"list", `null`, false}, {"list", `["\ud800"]`, false},
		{"reference", ref, true}, {"reference", strings.Replace(ref, "instance", "foreign-instance", 1), true}, {"reference", strings.Replace(ref, "drivebridge:", "https:", 1), false}, {"reference", strings.Replace(ref, "instance", "user@instance", 1), false}, {"reference", strings.TrimSuffix(ref, `"`) + `?"`, false}, {"reference", strings.TrimSuffix(ref, `"`) + `#"`, false}, {"reference", strings.TrimSuffix(ref, `"`) + `/"`, false}, {"reference", strings.Replace(ref, strings.Repeat("a", 48), strings.Repeat("A", 48), 1), false},
		{"unknown", `"value"`, false}, {"text", `{"value":"nested"}`, false},
	}
	for _, tt := range tests {
		t.Run(tt.kind+"/"+tt.raw, func(t *testing.T) {
			raw := json.RawMessage(tt.raw)
			before := string(raw)
			err := validatePropertyValue(tt.kind, raw)
			if (err == nil) != tt.valid {
				t.Fatalf("valid=%v err=%v", tt.valid, err)
			}
			if string(raw) != before {
				t.Fatal("validation mutated original bytes")
			}
		})
	}
}

func TestPropertyValueBudgetsAndEncoding(t *testing.T) {
	quote := func(text string) json.RawMessage {
		raw, err := json.Marshal(text)
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	if err := validatePropertyValue("text", quote(strings.Repeat("x", 8192))); err != nil {
		t.Fatal("text boundary rejected", err)
	}
	if err := validatePropertyValue("text", quote(strings.Repeat("x", 8193))); err == nil {
		t.Fatal("text byte budget bypass")
	}
	if err := validatePropertyValue("text", quote(strings.Repeat("中", 2731))); err == nil {
		t.Fatal("UTF8 byte budget bypass")
	}
	if err := validatePropertyValue("text", []byte{'"', 0xff, '"'}); err == nil {
		t.Fatal("invalid UTF8 repaired")
	}
	if err := validatePropertyValue("text", append([]byte(strings.Repeat(" ", propertyValueBudget)), []byte(`"x"`)...)); err == nil {
		t.Fatal("wire budget ignored")
	}
	for _, count := range []int{100, 101} {
		items := make([]string, count)
		raw, _ := json.Marshal(items)
		if (validatePropertyValue("list", raw) == nil) != (count == 100) {
			t.Fatal("list count boundary", count)
		}
	}
	for _, size := range []int{1024, 1025} {
		raw, _ := json.Marshal([]string{strings.Repeat("x", size)})
		if (validatePropertyValue("list", raw) == nil) != (size == 1024) {
			t.Fatal("list element byte boundary", size)
		}
	}
	// Escaped control text may fit decoded budget but exceed the wire budget.
	if err := validatePropertyValue("text", quote(strings.Repeat("\x01", 3000))); err == nil {
		t.Fatal("escaped wire expansion not bounded")
	}
}
