package service

import (
	"bytes"
	"encoding/json"
	"math"
	"math/big"
	"net/url"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

const propertyValueBudget = 16 * 1024

// validatePropertyValue checks shape only. References must additionally pass
// resolveRef and current fixed-version authorization in the owning transaction.
// Original JSON bytes are retained by callers; this function never coerces them.
func validatePropertyValue(kind string, raw json.RawMessage) error {
	if len(raw) == 0 || len(raw) > propertyValueBudget || !utf8.Valid(raw) || !json.Valid(raw) || !propertyJSONSurrogates(raw) {
		return code("invalid_request")
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	var value any
	if err := decoder.Decode(&value); err != nil {
		return code("invalid_request")
	}
	valid := false
	switch kind {
	case "text":
		text, ok := value.(string)
		valid = ok && len(text) <= 8192 && !strings.ContainsRune(text, 0)
	case "checkbox":
		_, valid = value.(bool)
	case "date":
		text, ok := value.(string)
		if ok && len(text) == 10 {
			parsed, err := time.Parse("2006-01-02", text)
			valid = err == nil && parsed.Year() >= 1 && parsed.Format("2006-01-02") == text
		}
	case "number":
		number, ok := value.(json.Number)
		if ok && len(number) <= 128 {
			text := string(number)
			exponent := 0
			if i := strings.IndexAny(text, "eE"); i >= 0 {
				var err error
				exponent, err = strconv.Atoi(text[i+1:])
				if err != nil || exponent > 308 || exponent < -308 {
					return code("invalid_request")
				}
			}
			f, err := number.Float64()
			exact, parsed := new(big.Rat).SetString(text)
			if err == nil && !math.IsInf(f, 0) && !math.IsNaN(f) && parsed {
				magnitude := new(big.Rat).Abs(exact)
				valid = magnitude.Cmp(new(big.Rat).SetInt64(9007199254740991)) <= 0 && (f != 0 || exact.Sign() == 0)
			}
		}
	case "list":
		items, ok := value.([]any)
		valid = ok && len(items) <= 100
		if valid {
			for _, item := range items {
				text, isText := item.(string)
				if !isText || len(text) > 1024 || strings.ContainsRune(text, 0) {
					valid = false
					break
				}
			}
		}
	case "reference":
		text, ok := value.(string)
		if ok && len(text) <= 2048 {
			ref, err := url.Parse(text)
			if err == nil && ref.Scheme == "drivebridge" && ref.Host != "" && ref.Opaque == "" && ref.User == nil && ref.RawQuery == "" && !ref.ForceQuery && ref.Fragment == "" && !strings.ContainsAny(text, "?#%") {
				parts := strings.Split(ref.Path, "/")
				if len(parts) == 5 && parts[0] == "" && parts[1] == "file" && parts[3] == "version" {
					valid = propertyHexID(parts[2]) && propertyHexID(parts[4])
				}
			}
		}
	}
	if !valid {
		return code("invalid_request")
	}
	return nil
}

func propertyHexID(id string) bool {
	if len(id) != 48 {
		return false
	}
	for _, ch := range id {
		if !(ch >= '0' && ch <= '9' || ch >= 'a' && ch <= 'f') {
			return false
		}
	}
	return true
}

// encoding/json replaces isolated surrogate escapes with U+FFFD. Reject them
// instead of silently changing user metadata; valid pairs remain accepted.
func propertyJSONSurrogates(raw []byte) bool {
	for i := 0; i < len(raw); i++ {
		if raw[i] != '\\' {
			continue
		}
		i++
		if i >= len(raw) {
			return false
		}
		if raw[i] != 'u' {
			continue
		}
		if i+4 >= len(raw) {
			return false
		}
		unit, err := strconv.ParseUint(string(raw[i+1:i+5]), 16, 16)
		if err != nil {
			return false
		}
		i += 4
		if unit >= 0xdc00 && unit <= 0xdfff {
			return false
		}
		if unit >= 0xd800 && unit <= 0xdbff {
			if i+6 >= len(raw) || raw[i+1] != '\\' || raw[i+2] != 'u' {
				return false
			}
			low, err := strconv.ParseUint(string(raw[i+3:i+7]), 16, 16)
			if err != nil || low < 0xdc00 || low > 0xdfff {
				return false
			}
			i += 6
		}
	}
	return true
}
