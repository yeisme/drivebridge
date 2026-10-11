package service

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

func TestMarkdownPropertiesPreserveUntouchedBytes(t *testing.T) {
	defs := []contract.PropertyDefinition{{Key: "title", Type: "text"}, {Key: "rank", Type: "number"}, {Key: "tags", Type: "list"}, {Key: "active", Type: "checkbox"}, {Key: "date", Type: "date"}}
	source := "---\r\n# header comment\r\ntitle: 'Old'  # keep this\r\nunknown: !custom { x: [1, true] }\r\nnested:\r\n  content: |\r\n    keep exact spaces  \r\nrank: 1.250\r\ntags: [one, two] # tag note\r\n---\r\n# Body\r\n\r\n---\r\n正文  \r\n"
	got, err := patchMarkdownProperties(source, defs, map[string]json.RawMessage{"title": json.RawMessage(`"新标题"`), "active": json.RawMessage(`true`), "date": json.RawMessage(`"2026-10-06"`)}, []string{"tags"})
	want := strings.Replace(source, "title: 'Old'  # keep this", "title: \"新标题\"  # keep this", 1)
	want = strings.Replace(want, "tags: [one, two] # tag note\r\n", "# tag note\r\nactive: true\r\ndate: \"2026-10-06\"\r\n", 1)
	if err != nil || got != want {
		t.Fatalf("patch not byte preserving: %v\n%q", err, got)
	}
	if !strings.Contains(got, "rank: 1.250\r\n") {
		t.Fatal("unmodified numeric representation lost")
	}
	unchanged, err := patchMarkdownProperties(source, defs, map[string]json.RawMessage{}, []string{"active"})
	if err != nil || unchanged != source {
		t.Fatal("absent removal normalized document", err)
	}
}

func TestMarkdownPropertiesSimpleValuesAndDelimiters(t *testing.T) {
	defs := []contract.PropertyDefinition{{Key: "value", Type: "text"}, {Key: "tags", Type: "list"}}
	for _, source := range []string{"# Body\n---\nnot metadata\n", "---\n---\nbody", "---\n# comment\n---\nbody"} {
		out, err := patchMarkdownProperties(source, defs, map[string]json.RawMessage{"value": json.RawMessage(`"line one\nline two # quote: \""`)}, []string{})
		if err != nil || !strings.Contains(out, `value: "line one\nline two # quote: \""`) {
			t.Fatal("string escaping or empty metadata failed", err)
		}
	}
	for _, value := range []string{"'old'", "\"old # quoted\"", "plain text", "null", "[one, two]"} {
		source := "---\nvalue: " + value + "\nunknown: 01\n---\nbody"
		out, err := patchMarkdownProperties(source, defs, map[string]json.RawMessage{"value": json.RawMessage(`""`)}, []string{})
		if err != nil || out != "---\nvalue: \"\"\nunknown: 01\n---\nbody" {
			t.Fatal("simple replacement failed", value, err)
		}
	}
	out, err := patchMarkdownProperties("---\nvalue: old\n---", defs, map[string]json.RawMessage{"tags": json.RawMessage(`["a", "a", ""]`)}, []string{"value"})
	if err != nil || out != "---\ntags: [\"a\",\"a\",\"\"]\n---" {
		t.Fatal("list order or closing bytes changed", err)
	}
}

func TestMarkdownPropertiesBudgetsAndCRLFInsertion(t *testing.T) {
	defs := []contract.PropertyDefinition{{Key: "value", Type: "text"}}
	out, err := patchMarkdownProperties("# Body\r\nSecond\r\n", defs, map[string]json.RawMessage{"value": json.RawMessage(`"new"`)}, []string{})
	if err != nil || out != "---\r\nvalue: \"new\"\r\n---\r\n# Body\r\nSecond\r\n" {
		t.Fatal("new metadata changed newline convention", err)
	}
	for _, metadata := range []string{"deep: " + strings.Repeat("[", 34) + "x" + strings.Repeat("]", 34), "many: [" + strings.TrimSuffix(strings.Repeat("x,", 4200), ",") + "]"} {
		if _, err = patchMarkdownProperties("---\n"+metadata+"\n---\nbody", defs, map[string]json.RawMessage{"value": json.RawMessage(`"new"`)}, []string{}); err == nil || err.Error() != "query_budget_exceeded" {
			t.Fatal("node budget missing", err)
		}
	}
}

func TestMarkdownPropertiesCommentOnlyAndUnicodeSeparators(t *testing.T) {
	defs := []contract.PropertyDefinition{{Key: "value", Type: "text"}}
	raw, _ := json.Marshal("before\u2028middle\u2029after")
	// Use literal separators as a client may send, rather than Go's escaped form.
	raw = []byte(strings.ReplaceAll(strings.ReplaceAll(string(raw), `\u2028`, "\u2028"), `\u2029`, "\u2029"))
	out, err := patchMarkdownProperties("---\nvalue: # note\n---\nbody", defs, map[string]json.RawMessage{"value": raw}, []string{})
	if err != nil {
		t.Fatal(err)
	}
	parsed, err := parseMarkdownProperties(out)
	if err != nil || parsed.root.Content[1].Value != "before\u2028middle\u2029after" || parsed.root.Content[1].LineComment != "# note" {
		t.Fatal("replacement changed text or comment semantics", err)
	}
}

func TestMarkdownPropertiesRejectUnsafeTargetsAndInvalidSources(t *testing.T) {
	defs := []contract.PropertyDefinition{{Key: "value", Type: "text"}}
	for _, source := range []string{
		"---\nvalue: one\nvalue: two\n---\nbody", "---\nunknown: {x: 1, x: 2}\n---\nbody",
		"---\nunknown: &a one\nvalue: *a\n---\nbody", "---\nvalue: |\n  old\n---\nbody",
		"---\nvalue:\n  child: old\n---\nbody", "---\nvalue: \"multi\n line\"\n---\nbody",
		"---\n'value': old\n---\nbody", "--- \nvalue: x\n---\nbody", "---\nunknown: x\u2028value: old\n---\nbody", "---\nunknown: x\rvalue: old\n---\nbody", "---\nvalue: !custom old\n---\nbody",
		"---\n{value: old}\n---\nbody", "---\n- not mapping\n---\nbody", "---\nvalue: x", "\ufeff---\nvalue: x\n---\nbody",
		"---\nvalue: x\n...\n---\nbody\x00", "---\nvalue: \xff\n---\nbody",
	} {
		if out, err := patchMarkdownProperties(source, defs, map[string]json.RawMessage{"value": json.RawMessage(`"new"`)}, []string{}); err == nil || out != "" {
			t.Fatalf("unsafe source accepted: %q", source)
		}
	}
	for _, raw := range []string{`null`, `123`, `"\ud800"`, `"\u0000"`} {
		if _, err := patchMarkdownProperties("body", defs, map[string]json.RawMessage{"value": json.RawMessage(raw)}, []string{}); err == nil {
			t.Fatal("invalid typed update", raw)
		}
	}
	if _, err := patchMarkdownProperties("body", defs, map[string]json.RawMessage{"other": json.RawMessage(`"x"`)}, []string{}); err == nil {
		t.Fatal("unknown key accepted")
	}
	if _, err := patchMarkdownProperties("body", defs, map[string]json.RawMessage{"value": json.RawMessage(`"x"`)}, []string{"value"}); err == nil {
		t.Fatal("overlap accepted")
	}
	for _, source := range []string{"---\n#" + strings.Repeat("x", frontmatterBudget) + "\n---\nbody", strings.Repeat("x", contract.DocumentMaxBytes+1)} {
		if _, err := patchMarkdownProperties(source, defs, map[string]json.RawMessage{"value": json.RawMessage(`"x"`)}, []string{}); err == nil {
			t.Fatal("budget bypass")
		}
	}
}
