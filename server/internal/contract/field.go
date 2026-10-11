package contract

import (
	"bytes"
	"encoding/json"
)

// Field preserves omitted, explicit null, and concrete PATCH values.
// Decode each request into a fresh DTO; an omitted key leaves Field untouched.
// Field is input-only. Public response DTOs use their concrete wire types.
type Field[T any] struct {
	Present bool
	Null    bool
	Value   T
}

func (f *Field[T]) UnmarshalJSON(data []byte) error {
	next := Field[T]{Present: true}
	if bytes.Equal(bytes.TrimSpace(data), []byte("null")) {
		next.Null = true
	} else if err := json.Unmarshal(data, &next.Value); err != nil {
		return err
	}
	*f = next
	return nil
}
