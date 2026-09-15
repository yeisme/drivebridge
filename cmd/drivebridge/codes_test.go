package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// renderErrorCodesDoc renders the Chinese reference table from the registry;
// codes_test.go compares it against docs/error-codes.md so the two can never
// drift apart. After editing codeRegistry, copy the test's failure output
// into the doc (the doc keeps a hand-written header).
func renderErrorCodesDoc() string {
	keys := make([]string, 0, len(codeRegistry))
	for k := range codeRegistry {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var b strings.Builder
	b.WriteString("| 错误码 | 触发与恢复 |\n| --- | --- |\n")
	for _, k := range keys {
		b.WriteString(fmt.Sprintf("| `%s` | %s |\n", k, codeRegistry[k].DocZH))
	}
	return b.String()
}

func TestErrorCodesDocInSync(t *testing.T) {
	body, err := os.ReadFile(filepath.Join("..", "..", "docs", "error-codes.md"))
	if err != nil {
		t.Fatalf("docs/error-codes.md missing: %v (paste renderErrorCodesDoc output)", err)
	}
	table := renderErrorCodesDoc()
	if !strings.Contains(string(body), table) {
		t.Fatalf("docs/error-codes.md table out of sync with codeRegistry; expected table:\n%s", table)
	}
}

func TestHintFallbackForUnknownCode(t *testing.T) {
	if h := hintEN("definitely_not_registered"); !strings.Contains(h, "Inspect the original operation") {
		t.Fatalf("fallback hint changed: %q", h)
	}
}
