package service

import (
	"bytes"
	"encoding/json"
	"io"
	"sort"
	"strings"
	"unicode/utf8"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gopkg.in/yaml.v3"
)

const frontmatterBudget = 64 * 1024

// These helpers operate on supplied bytes only. They do not authorize a file,
// expose an endpoint, resolve references or bypass the canonical document owner.
type markdownProperties struct {
	source     string
	start, end int
	newline    string
	lines      []string
	root       *yaml.Node
}

func parseMarkdownProperties(source string) (markdownProperties, error) {
	out := markdownProperties{source: source, newline: "\n"}
	if len(source) > contract.DocumentMaxBytes || !utf8.ValidString(source) || strings.ContainsRune(source, 0) {
		return out, code("invalid_request")
	}
	if strings.HasPrefix(source, "\ufeff") {
		return out, code("unsupported_action")
	}
	if strings.HasPrefix(source, "---\r\n") {
		out.start = 5
		out.newline = "\r\n"
	} else if strings.HasPrefix(source, "---\n") {
		out.start = 4
	} else {
		first, _, _ := strings.Cut(source, "\n")
		if strings.TrimSpace(first) == "---" || strings.HasPrefix(source, "---\r") {
			return out, code("unsupported_action")
		}
		if strings.Contains(source, "\r\n") {
			out.newline = "\r\n"
		}
		return out, nil
	}
	offset := out.start
	for offset <= len(source) {
		next := strings.IndexByte(source[offset:], '\n')
		end := len(source)
		if next >= 0 {
			end = offset + next + 1
		}
		line := source[offset:end]
		trimmed := strings.TrimSuffix(strings.TrimSuffix(line, "\n"), "\r")
		if trimmed == "---" {
			out.end = offset
			if out.end-out.start > frontmatterBudget {
				return out, code("query_budget_exceeded")
			}
			raw := source[out.start:out.end]
			// Node.Line must match our LF-indexed byte slices. YAML also treats
			// bare CR and Unicode separators as line breaks; do not guess offsets.
			if strings.ContainsAny(strings.ReplaceAll(raw, "\r\n", "\n"), "\r\u0085\u2028\u2029") {
				return out, code("unsupported_action")
			}
			decoder := yaml.NewDecoder(strings.NewReader(raw))
			var doc yaml.Node
			err := decoder.Decode(&doc)
			if err != nil && err != io.EOF {
				return out, code("invalid_request")
			}
			if err == nil {
				if len(doc.Content) != 1 || doc.Content[0].Kind != yaml.MappingNode || doc.Content[0].Style != 0 {
					return out, code("unsupported_action")
				}
				out.root = doc.Content[0]
				budget := 4096
				if err = checkPropertyYAML(out.root, 0, &budget); err != nil {
					return out, err
				}
				if decoder.Decode(new(yaml.Node)) != io.EOF {
					return out, code("invalid_request")
				}
			}
			out.lines = strings.SplitAfter(raw, "\n")
			return out, nil
		}
		if end-out.start > frontmatterBudget {
			return out, code("query_budget_exceeded")
		}
		if next < 0 {
			break
		}
		offset = end
	}
	return out, code("invalid_request")
}

func checkPropertyYAML(node *yaml.Node, depth int, budget *int) error {
	*budget--
	if depth > 32 || *budget < 0 {
		return code("query_budget_exceeded")
	}
	// No alias expansion or merge/anchor rewriting: changing one field must not
	// silently mutate another field's meaning. Such sources remain unsupported.
	if node.Kind == yaml.AliasNode || node.Anchor != "" || node.Tag == "!!merge" {
		return code("unsupported_action")
	}
	if node.Kind == yaml.MappingNode {
		seen := map[string]bool{}
		for i := 0; i < len(node.Content); i += 2 {
			key := node.Content[i]
			if key.Kind != yaml.ScalarNode || key.Tag != "!!str" || seen[key.Value] {
				return code("invalid_request")
			}
			seen[key.Value] = true
		}
	}
	for _, child := range node.Content {
		if err := checkPropertyYAML(child, depth+1, budget); err != nil {
			return err
		}
	}
	return nil
}

// patchMarkdownProperties changes only explicitly requested simple top-level
// fields. Unchanged YAML and the Markdown body are copied byte-for-byte. Complex
// target nodes remain read-only rather than being normalized by a YAML emitter.
func patchMarkdownProperties(source string, definitions []contract.PropertyDefinition, updates map[string]json.RawMessage, remove []string) (string, error) {
	if updates == nil || remove == nil || len(updates)+len(remove) == 0 || len(updates)+len(remove) > 200 {
		return "", code("invalid_request")
	}
	types, err := markdownDefinitionTypes(definitions)
	if err != nil {
		return "", err
	}
	changes := map[string]string{}
	deletes := map[string]bool{}
	for key, raw := range updates {
		if !propertyKey.MatchString(key) || validatePropertyValue(types[key], raw) != nil {
			return "", code("invalid_request")
		}
		// JSON is a YAML subset. Preserve numeric tokens, quoting and list order.
		var compact bytes.Buffer
		if json.Compact(&compact, raw) != nil {
			return "", code("invalid_request")
		}
		// YAML treats NEL/LS/PS as line breaks even inside quoted scalars.
		changes[key] = strings.NewReplacer("\u0085", `\u0085`, "\u2028", `\u2028`, "\u2029", `\u2029`).Replace(compact.String())
	}
	for _, key := range remove {
		if !propertyKey.MatchString(key) || types[key] == "" || deletes[key] {
			return "", code("invalid_request")
		}
		if _, exists := changes[key]; exists {
			return "", code("invalid_request")
		}
		deletes[key] = true
	}
	parsed, err := parseMarkdownProperties(source)
	if err != nil {
		return "", err
	}
	lines := append([]string(nil), parsed.lines...)
	found := map[string]bool{}
	if parsed.root != nil {
		for i := 0; i < len(parsed.root.Content); i += 2 {
			key, value := parsed.root.Content[i], parsed.root.Content[i+1]
			replacement, set := changes[key.Value]
			if !set && !deletes[key.Value] {
				continue
			}
			found[key.Value] = true
			// Only unquoted, untagged keys at column one and a one-line value have a
			// byte-local edit proof. Block/nested/multiline targets need a later editor.
			if !markdownPatchable(parsed, key, value) {
				return "", code("unsupported_action")
			}
			line := lines[key.Line-1]
			prefix := key.Value + ":"
			if !strings.HasPrefix(line, prefix) {
				return "", code("unsupported_action")
			}
			valueStart := len(prefix)
			for valueStart < len(line) && (line[valueStart] == ' ' || line[valueStart] == '\t') {
				valueStart++
			}
			ending := ""
			text := line
			if strings.HasSuffix(text, "\r\n") {
				ending = "\r\n"
				text = strings.TrimSuffix(text, ending)
			} else if strings.HasSuffix(text, "\n") {
				ending = "\n"
				text = strings.TrimSuffix(text, ending)
			}
			if valueStart > len(text) {
				valueStart = len(text)
			}
			// markdownPatchable proved this line has no value continuation.
			tail := ""
			comment := value.LineComment
			if comment == "" {
				comment = key.LineComment
			}
			if comment != "" {
				pos := strings.LastIndex(text, comment)
				if pos < valueStart {
					return "", code("unsupported_action")
				}
				start := pos
				for start > valueStart && (text[start-1] == ' ' || text[start-1] == '\t') {
					start--
				}
				tail = text[start:]
			} else {
				tail = text[len(strings.TrimRight(text, " \t")):]
			}
			if deletes[key.Value] {
				if comment != "" {
					lines[key.Line-1] = strings.TrimLeft(tail, " \t") + ending
				} else {
					lines[key.Line-1] = ""
				}
			} else {
				lead := text[:valueStart]
				if valueStart == len(prefix) {
					lead += " "
				}
				if strings.HasPrefix(tail, "#") {
					tail = " " + tail
				}
				lines[key.Line-1] = lead + replacement + tail + ending
			}
		}
	}
	added := []string{}
	for key := range changes {
		if !found[key] {
			added = append(added, key)
		}
	}
	sort.Strings(added)
	var extra strings.Builder
	for _, key := range added {
		extra.WriteString(key + ": " + changes[key] + parsed.newline)
	}
	var result string
	if parsed.start == 0 {
		if len(added) == 0 {
			return source, nil
		}
		result = "---" + parsed.newline + extra.String() + "---" + parsed.newline + source
	} else {
		result = source[:parsed.start] + strings.Join(lines, "") + extra.String() + source[parsed.end:]
	}
	if err = verifyMarkdownPatch(result, definitions, updates, remove); err != nil {
		return "", err
	}
	return result, nil
}

func samePropertyNode(a, b *yaml.Node) bool {
	if a.Kind != b.Kind || a.Tag != b.Tag || a.Value != b.Value || a.Style != b.Style || len(a.Content) != len(b.Content) {
		return false
	}
	for i := range a.Content {
		if !samePropertyNode(a.Content[i], b.Content[i]) {
			return false
		}
	}
	return true
}
