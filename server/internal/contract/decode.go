package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
)

var ErrInvalidRequest = errors.New("invalid_request")

// DecodeRequest accepts a single flat request DTO. Required JSON fields must
// be present and non-null, even when zero is a valid value. Optional Field
// values retain explicit null for the application validator to decide.
// The HTTP boundary must limit body size before calling this function.
func DecodeRequest[T any](reader io.Reader) (T, error) {
	var zero T
	typ := reflect.TypeFor[T]()
	if typ.Kind() != reflect.Struct {
		return zero, ErrInvalidRequest
	}
	decoder := json.NewDecoder(reader)
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return zero, ErrInvalidRequest
	}
	fields := map[string]json.RawMessage{}
	for decoder.More() {
		token, err = decoder.Token()
		if err != nil {
			return zero, ErrInvalidRequest
		}
		name, ok := token.(string)
		if !ok {
			return zero, ErrInvalidRequest
		}
		if _, exists := fields[name]; exists {
			return zero, ErrInvalidRequest
		}
		var raw json.RawMessage
		if err = decoder.Decode(&raw); err != nil {
			return zero, ErrInvalidRequest
		}
		fields[name] = raw
	}
	if token, err = decoder.Token(); err != nil || token != json.Delim('}') {
		return zero, ErrInvalidRequest
	}
	var trailing any
	if err = decoder.Decode(&trailing); err != io.EOF {
		return zero, ErrInvalidRequest
	}
	allowed := map[string]bool{}
	for i := 0; i < typ.NumField(); i++ {
		field := typ.Field(i)
		tag := strings.Split(field.Tag.Get("json"), ",")
		name := tag[0]
		if name == "-" || field.PkgPath != "" {
			continue
		}
		if name == "" {
			name = field.Name
		}
		allowed[name] = true
		optional := false
		for _, option := range tag[1:] {
			if option == "omitempty" {
				optional = true
			}
		}
		raw, present := fields[name]
		if !optional && !present {
			return zero, ErrInvalidRequest
		}
		if present && bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
			// Only presence-aware fields may preserve null for later validation.
			_, custom := reflect.New(field.Type).Interface().(json.Unmarshaler)
			if !optional || !custom {
				return zero, ErrInvalidRequest
			}
		}
	}
	for name := range fields {
		if !allowed[name] {
			return zero, ErrInvalidRequest
		}
	}
	data, err := json.Marshal(fields)
	if err != nil {
		return zero, ErrInvalidRequest
	}
	var value T
	if err = json.Unmarshal(data, &value); err != nil {
		return zero, ErrInvalidRequest
	}
	return value, nil
}
