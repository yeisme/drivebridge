package service

import (
	"context"
	"encoding/base64"
	"errors"
	"reflect"
	"strings"

	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

const itemKeyEncoding = "db-item-v1:"

// Only the storage representation changes. API identities and fingerprints
// retain the original tuple, including its separators; old SQLite rows decode
// unchanged. This encoding avoids PostgreSQL text's NUL prohibition.
type itemKeySerializer struct{}

func init() {
	schema.RegisterSerializer("itemkey", itemKeySerializer{})
	schema.RegisterSerializer("operationkey", operationKeySerializer{})
}

func storedItemKey(key string) string {
	if key == "" {
		return ""
	}
	return itemKeyEncoding + base64.RawURLEncoding.EncodeToString([]byte(key))
}

func (itemKeySerializer) Value(_ context.Context, _ *schema.Field, _ reflect.Value, value interface{}) (interface{}, error) {
	key, ok := value.(string)
	if !ok {
		return nil, errors.New("invalid_item_key")
	}
	return storedItemKey(key), nil
}

func (itemKeySerializer) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, value interface{}) error {
	var key string
	switch v := value.(type) {
	case nil:
	case string:
		key = v
	case []byte:
		key = string(v)
	default:
		return errors.New("invalid_item_key")
	}
	if strings.HasPrefix(key, itemKeyEncoding) && !strings.ContainsRune(key, 0) {
		decoded, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(key, itemKeyEncoding))
		if err != nil {
			return errors.New("invalid_item_key")
		}
		key = string(decoded)
	}
	return field.Set(ctx, dst, key)
}

func matchingItemKey(tx *gorm.DB, key string) *gorm.DB {
	encoded := storedItemKey(key)
	// PostgreSQL could never store a legacy NUL tuple, and rejects even a
	// comparison parameter containing NUL. SQLite retains legacy lookup.
	if tx.Dialector.Name() == "postgres" && strings.ContainsRune(key, 0) {
		return tx.Where("item_key = ?", encoded)
	}
	return tx.Where("item_key IN ?", []string{key, encoded})
}

// Operation keys have an explicit encoding version because unlike item tuples,
// caller-provided keys may legitimately start with the storage marker. Legacy
// values are never interpreted by their spelling alone.
type operationKeySerializer struct{}

func operationKeyVersion(ctx context.Context, field *schema.Field, dst reflect.Value) int {
	versionField := field.Schema.LookUpField("KeyEncodingVersion")
	if versionField == nil {
		return -1
	}
	value, _ := versionField.ValueOf(ctx, dst)
	version, ok := value.(int)
	if !ok {
		return -1
	}
	return version
}
func (operationKeySerializer) Value(ctx context.Context, field *schema.Field, dst reflect.Value, value interface{}) (interface{}, error) {
	key, ok := value.(string)
	if !ok {
		return nil, errors.New("invalid_operation_key")
	}
	switch operationKeyVersion(ctx, field, dst) {
	case 0:
		return key, nil
	case 1:
		return storedItemKey(key), nil
	default:
		return nil, errors.New("invalid_operation_key")
	}
}
func (operationKeySerializer) Scan(ctx context.Context, field *schema.Field, dst reflect.Value, value interface{}) error {
	var key string
	switch v := value.(type) {
	case nil:
	case string:
		key = v
	case []byte:
		key = string(v)
	default:
		return errors.New("invalid_operation_key")
	}
	switch operationKeyVersion(ctx, field, dst) {
	case 0:
	case 1:
		if !strings.HasPrefix(key, itemKeyEncoding) {
			return errors.New("invalid_operation_key")
		}
		raw, err := base64.RawURLEncoding.DecodeString(strings.TrimPrefix(key, itemKeyEncoding))
		if err != nil {
			return errors.New("invalid_operation_key")
		}
		key = string(raw)
	default:
		return errors.New("invalid_operation_key")
	}
	return field.Set(ctx, dst, key)
}
func matchingOperationKey(tx *gorm.DB, owner, key string) *gorm.DB {
	query := tx.Where("owner = ?", owner)
	if tx.Dialector.Name() == "postgres" && strings.ContainsRune(key, 0) {
		return query.Where("key_encoding_version = ? AND key = ?", 1, storedItemKey(key))
	}
	return query.Where("(key_encoding_version = ? AND key = ?) OR (key_encoding_version = ? AND key = ?)", 0, key, 1, storedItemKey(key))
}

func operationKeyEncoding(key string) int {
	if strings.ContainsRune(key, 0) {
		return 1
	}
	return 0
}
