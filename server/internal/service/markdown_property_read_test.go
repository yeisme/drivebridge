package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

func TestMarkdownPropertyTypedReadAndOpaqueUnknown(t *testing.T) {
	defs := []contract.PropertyDefinition{{Key: "text", Type: "text"}, {Key: "number", Type: "number"}, {Key: "date", Type: "date"}, {Key: "active", Type: "checkbox"}, {Key: "tags", Type: "list"}, {Key: "source", Type: "reference"}, {Key: "block", Type: "text"}, {Key: "wrong", Type: "text"}, {Key: "custom", Type: "text"}}
	ref := "drivebridge://instance/file/" + strings.Repeat("a", 48) + "/version/" + strings.Repeat("b", 48)
	source := "---\ntext: \"true\"\nnumber: 1.250\ndate: 2026-10-06\nactive: false\ntags: [one, one, \"\"]\nsource: " + ref + "\nblock: |\n  preserved body\nwrong: true\ncustom: !private secret\nunknown: {private: [do, not, expose]}\n---\n# Markdown\n"
	got, err := readMarkdownProperties(source, defs)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Values["number"]) != "1.250" || string(got.Values["date"]) != `"2026-10-06"` || string(got.Values["active"]) != "false" || string(got.Values["tags"]) != `["one","one",""]` {
		t.Fatal("coerced typed values")
	}
	if got.ReadOnly["block"] != "unsupported_representation" || got.ReadOnly["wrong"] != "type_mismatch" || got.ReadOnly["custom"] != "type_mismatch" {
		t.Fatal("readonly distinction lost")
	}
	if got.Values["wrong"] != nil || got.Values["custom"] != nil || got.Values["unknown"] != nil || len(got.UnknownKeys) != 1 || got.UnknownKeys[0] != "unknown" {
		t.Fatal("untyped values exposed")
	}
	var actual string
	if json.Unmarshal(got.Values["source"], &actual) != nil || actual != ref {
		t.Fatal("reference changed")
	}
	// Parsing a reference does not resolve or authorize it.
	if _, err = readMarkdownProperties(strings.Replace(source, "drivebridge://instance/", "drivebridge://foreign/", 1), defs); err != nil {
		t.Fatal("pure parse unexpectedly grants or resolves authority", err)
	}
}

func TestMarkdownPropertyNumberRoundTrip(t *testing.T) {
	defs := []contract.PropertyDefinition{{Key: "value", Type: "number"}}
	for _, literal := range []string{"0", "-0", "1.250", "1e3", "1e+3", "1E-3", "1e-308", "9007199254740991", "-9007199254740991"} {
		t.Run(literal, func(t *testing.T) {
			out, err := patchMarkdownProperties("---\nvalue: 42 # keep\n---\nbody", defs, map[string]json.RawMessage{"value": json.RawMessage(literal)}, []string{})
			if err != nil {
				t.Fatal(err)
			}
			read, err := readMarkdownProperties(out, defs)
			if err != nil || string(read.Values["value"]) != literal {
				t.Fatal("number token lost", err)
			}
		})
	}
	for _, literal := range []string{"0x10", "01", "1_000", ".nan", ".inf", "+1", "1:20", "9007199254740992"} {
		read, err := readMarkdownProperties("---\nvalue: "+literal+"\n---\nbody", defs)
		if err != nil || read.Values["value"] != nil || read.ReadOnly["value"] != "type_mismatch" {
			t.Fatal("YAML number silently coerced", literal, err)
		}
	}
}

func TestMarkdownPropertyAllTypesSemanticRoundTrip(t *testing.T) {
	for _, test := range []struct{ kind, raw string }{
		{"text", `"\u0085\u2028\u2029\n\t\"\\"`}, {"text", `""`}, {"text", `"yes: # quoted"`},
		{"list", `["one","one","", "true", "2026-01-01"]`}, {"list", `[]`}, {"checkbox", `true`}, {"checkbox", `false`}, {"date", `"2024-02-29"`},
		{"reference", `"drivebridge://instance/file/aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa/version/bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"`},
	} {
		defs := []contract.PropertyDefinition{{Key: "value", Type: test.kind}}
		out, err := patchMarkdownProperties("body", defs, map[string]json.RawMessage{"value": json.RawMessage(test.raw)}, []string{})
		if err != nil {
			t.Fatal(test.kind, err)
		}
		got, err := readMarkdownProperties(out, defs)
		if err != nil || !sameMarkdownJSON(got.Values["value"], json.RawMessage(test.raw)) {
			t.Fatal("semantic roundtrip failed", test.kind, err)
		}
	}
	if verifyMarkdownPatch("---\nvalue: 12\n---\nbody", []contract.PropertyDefinition{{Key: "value", Type: "number"}}, map[string]json.RawMessage{"value": json.RawMessage(`13`)}, []string{}) == nil {
		t.Fatal("wrong result accepted")
	}
	if verifyMarkdownPatch("---\nvalue: text\n---\nbody", []contract.PropertyDefinition{{Key: "value", Type: "text"}}, map[string]json.RawMessage{}, []string{"value"}) == nil {
		t.Fatal("failed deletion accepted")
	}
}

func TestMarkdownPropertyReadUnsupportedAndDefinitions(t *testing.T) {
	defs := []contract.PropertyDefinition{{Key: "value", Type: "list"}}
	for _, source := range []string{"---\nvalue:\n  - one\n  - two\n---\nbody", "---\n'value': [one,two]\n---\nbody"} {
		got, err := readMarkdownProperties(source, defs)
		if err != nil || string(got.Values["value"]) != `["one","two"]` || got.ReadOnly["value"] != "unsupported_representation" {
			t.Fatal("complex typed read failed", err)
		}
		if _, err = patchMarkdownProperties(source, defs, map[string]json.RawMessage{"value": json.RawMessage(`[]`)}, []string{}); err == nil {
			t.Fatal("complex node mutation allowed")
		}
	}
	for _, bad := range [][]contract.PropertyDefinition{{{Key: "bad-key", Type: "text"}}, {{Key: "value", Type: "script"}}, {{Key: "value", Type: "text"}, {Key: "value", Type: "text"}}, make([]contract.PropertyDefinition, 201)} {
		if _, err := readMarkdownProperties("body", bad); err == nil {
			t.Fatal("invalid dictionary accepted")
		}
	}
	got, err := readMarkdownProperties("body", defs)
	if err != nil || len(got.Values) != 0 || got.UnknownKeys == nil || got.ReadOnly == nil {
		t.Fatal("absent frontmatter not explicit empty", err)
	}
}
