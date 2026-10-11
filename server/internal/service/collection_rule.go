package service

import (
	"bytes"
	"encoding/json"
	"math"
	"math/big"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/yeisme/drivebridge-server/internal/contract"
)

// collectionRule validates the original bytes without changing the persisted
// rule or its released idempotency fingerprint. Only executable combinations
// are admitted; the legacy query engine must never silently drop a condition.
func collectionRule(raw []byte, space string) (FileQueryRequest, error) {
	var out FileQueryRequest
	invalid := func() (FileQueryRequest, error) { return FileQueryRequest{}, code("invalid_query") }
	if len(raw) == 0 || len(raw) > 64*1024 || !utf8.Valid(raw) || !propertyJSONSurrogates(raw) {
		return invalid()
	}
	input, err := contract.DecodeRequest[contract.CollectionRule](bytes.NewReader(raw))
	if err != nil || input.Mode != "exact" || input.Cursor != "" || input.Limit < 0 || input.Limit > orgPageMax || len(input.Filters) > 64 || len(input.Sort) > 1 {
		return invalid()
	}
	if input.SpaceIDs != nil && (len(input.SpaceIDs) != 1 || input.SpaceIDs[0] != space) {
		return invalid()
	}
	out = FileQueryRequest{Mode: "exact", SpaceIDs: []string{space}, Limit: orgPageMax}
	for _, item := range input.Filters {
		f, e := contract.DecodeRequest[contract.CollectionRuleFilter](bytes.NewReader(item))
		if e != nil {
			return invalid()
		}
		str, isString := f.Value.(string)
		if isString && (len(str) > 4096 || strings.ContainsRune(str, 0)) {
			return invalid()
		}
		comparison := f.Op == "gt" || f.Op == "gte" || f.Op == "lt" || f.Op == "lte"
		switch f.Field {
		case "path_prefix":
			if !isString || (f.Op != "prefix" && f.Op != "contains") || str == "" {
				return invalid()
			}
		case "mime_type", "caller", "task_ref":
			if !isString || f.Op != "eq" {
				return invalid()
			}
		case "labels.manual", "labels.rule", "labels.suggested":
			if !isString || str == "" || (f.Op != "eq" && f.Op != "ne" && f.Op != "contains") {
				return invalid()
			}
		case "updated_at":
			if !isString || !comparison {
				return invalid()
			}
			if _, e = time.Parse(time.RFC3339, str); e != nil {
				return invalid()
			}
		case "size_bytes":
			n, ok := f.Value.(float64)
			if !ok || !comparison || n < 0 || n > 9007199254740991 || math.Trunc(n) != n {
				return invalid()
			}
			// Validate the original numeric token too: float decoding may round a
			// fractional or out-of-range literal to an apparently valid integer.
			var fields map[string]json.RawMessage
			if json.Unmarshal(item, &fields) != nil || validatePropertyValue("number", fields["value"]) != nil {
				return invalid()
			}
			exact, ok := new(big.Rat).SetString(string(fields["value"]))
			if !ok || !exact.IsInt() {
				return invalid()
			}
		case "trashed":
			b, ok := f.Value.(bool)
			if !ok || b || f.Op != "eq" {
				return invalid()
			} // Collections expose live members only.
		default:
			return invalid()
		}
		out.Filters = append(out.Filters, FileQueryFilter{Field: f.Field, Op: f.Op, Value: f.Value})
	}
	for _, item := range input.Sort {
		term, e := contract.DecodeRequest[contract.CollectionRuleSort](bytes.NewReader(item))
		if e != nil || (term.Direction != "asc" && term.Direction != "desc") {
			return invalid()
		}
		if term.Field != "updated_at" && term.Field != "name" && term.Field != "path_prefix" && term.Field != "size_bytes" {
			return invalid()
		}
		out.Sort = append(out.Sort, FileQuerySort{Field: term.Field, Direction: term.Direction})
	}
	if validateFileQuery(out) != nil {
		return invalid()
	}
	return out, nil
}
