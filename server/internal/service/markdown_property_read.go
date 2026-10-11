package service

import (
	"bytes"
	"encoding/json"
	"reflect"
	"sort"
	"strings"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gopkg.in/yaml.v3"
)

// Internal parsing evidence only. Patchable is a byte-edit property, never
// file authorization. Reference values require owner resolution before delivery.
type markdownPropertyRead struct {
	Values      map[string]json.RawMessage
	ReadOnly    map[string]string
	UnknownKeys []string
}

func markdownDefinitionTypes(definitions []contract.PropertyDefinition) (map[string]string, error) {
	if len(definitions) > 200 {
		return nil, code("query_budget_exceeded")
	}
	types := map[string]string{}
	for _, d := range definitions {
		if !propertyKey.MatchString(d.Key) || types[d.Key] != "" {
			return nil, code("invalid_request")
		}
		switch d.Type {
		case "text", "number", "date", "checkbox", "list", "reference":
		default:
			return nil, code("invalid_request")
		}
		types[d.Key] = d.Type
	}
	return types, nil
}

func markdownNodeValue(node *yaml.Node, kind string) (json.RawMessage, error) {
	var raw []byte
	switch kind {
	case "text", "reference":
		if node.Kind != yaml.ScalarNode || node.Tag != "!!str" {
			return nil, code("invalid_request")
		}
		raw, _ = json.Marshal(node.Value)
	case "date":
		if node.Kind != yaml.ScalarNode || (node.Tag != "!!str" && node.Tag != "!!timestamp") {
			return nil, code("invalid_request")
		}
		raw, _ = json.Marshal(node.Value)
	case "number":
		if node.Kind != yaml.ScalarNode || (node.Tag != "!!int" && node.Tag != "!!float") {
			return nil, code("invalid_request")
		}
		// No Decode(float64): retain decimal precision and signed zero. YAML-only
		// octal/hex/underscores/NaN remain unsupported, never silently coerced.
		raw = []byte(node.Value)
	case "checkbox":
		if node.Kind != yaml.ScalarNode || node.Tag != "!!bool" {
			return nil, code("invalid_request")
		}
		raw = []byte(node.Value)
	case "list":
		if node.Kind != yaml.SequenceNode || node.Tag != "!!seq" || len(node.Content) > 100 {
			return nil, code("invalid_request")
		}
		items := make([]string, 0, len(node.Content))
		for _, item := range node.Content {
			if item.Kind != yaml.ScalarNode || item.Tag != "!!str" {
				return nil, code("invalid_request")
			}
			items = append(items, item.Value)
		}
		raw, _ = json.Marshal(items)
	default:
		return nil, code("invalid_request")
	}
	if err := validatePropertyValue(kind, raw); err != nil {
		return nil, err
	}
	return raw, nil
}

func markdownPatchable(parsed markdownProperties, key, value *yaml.Node) bool {
	if key.Column != 1 || key.Style != 0 || key.Line != value.Line || value.Style&(yaml.LiteralStyle|yaml.FoldedStyle|yaml.TaggedStyle) != 0 || key.Line < 1 || key.Line > len(parsed.lines) {
		return false
	}
	line := strings.TrimSuffix(strings.TrimSuffix(parsed.lines[key.Line-1], "\n"), "\r")
	if !strings.HasPrefix(line, key.Value+":") {
		return false
	}
	var single yaml.Node
	return yaml.Unmarshal([]byte(line), &single) == nil && len(single.Content) == 1 && len(single.Content[0].Content) == 2 && samePropertyNode(value, single.Content[0].Content[1])
}

func readMarkdownProperties(source string, definitions []contract.PropertyDefinition) (markdownPropertyRead, error) {
	out := markdownPropertyRead{Values: map[string]json.RawMessage{}, ReadOnly: map[string]string{}, UnknownKeys: []string{}}
	types, err := markdownDefinitionTypes(definitions)
	if err != nil {
		return markdownPropertyRead{}, err
	}
	parsed, err := parseMarkdownProperties(source)
	if err != nil {
		return markdownPropertyRead{}, err
	}
	if parsed.root == nil {
		return out, nil
	}
	for i := 0; i < len(parsed.root.Content); i += 2 {
		key, node := parsed.root.Content[i], parsed.root.Content[i+1]
		kind, known := types[key.Value]
		if !known {
			out.UnknownKeys = append(out.UnknownKeys, key.Value)
			continue
		}
		raw, err := markdownNodeValue(node, kind)
		if err != nil {
			out.ReadOnly[key.Value] = "type_mismatch"
			continue
		}
		out.Values[key.Value] = raw
		if !markdownPatchable(parsed, key, node) {
			out.ReadOnly[key.Value] = "unsupported_representation"
		}
	}
	sort.Strings(out.UnknownKeys)
	return out, nil
}

func sameMarkdownJSON(a, b json.RawMessage) bool {
	decode := func(raw json.RawMessage) (any, error) {
		d := json.NewDecoder(bytes.NewReader(raw))
		d.UseNumber()
		var v any
		err := d.Decode(&v)
		return v, err
	}
	av, ae := decode(a)
	bv, be := decode(b)
	return ae == nil && be == nil && reflect.DeepEqual(av, bv)
}

func verifyMarkdownPatch(source string, definitions []contract.PropertyDefinition, updates map[string]json.RawMessage, remove []string) error {
	observed, err := readMarkdownProperties(source, definitions)
	if err != nil {
		return err
	}
	for key, want := range updates {
		got, ok := observed.Values[key]
		if !ok || !sameMarkdownJSON(want, got) {
			return code("unsupported_action")
		}
	}
	parsed, err := parseMarkdownProperties(source)
	if err != nil {
		return err
	}
	if parsed.root != nil {
		for i := 0; i < len(parsed.root.Content); i += 2 {
			for _, key := range remove {
				if parsed.root.Content[i].Value == key {
					return code("unsupported_action")
				}
			}
		}
	}
	return nil
}
