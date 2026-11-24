package service

import (
	"encoding/json"
	"reflect"
)

// API visibility is not a backup policy. Preserve private persistence fields
// separately from the existing public row shape; never print these snapshots.
// Connection secrets are already sealed ciphertext, not provider credentials.
func metadataPrivateFields(value reflect.Value, visit func(string, reflect.Value) error) error {
	for i := 0; i < value.NumField(); i++ {
		field, item := value.Type().Field(i), value.Field(i)
		if field.PkgPath != "" || field.Tag.Get("gorm") == "-" {
			continue
		}
		if field.Anonymous && item.Kind() == reflect.Struct {
			if err := metadataPrivateFields(item, visit); err != nil {
				return err
			}
		} else if field.Tag.Get("json") == "-" {
			if err := visit(field.Name, item); err != nil {
				return err
			}
		}
	}
	return nil
}

func metadataWireRow(value reflect.Value) (map[string]any, error) {
	data, err := json.Marshal(value.Interface())
	if err != nil {
		return nil, err
	}
	row := map[string]any{}
	// Keep RawMessage values: decoding public integers through float64 would
	// change revisions above 2^53 in a metadata migration.
	var public map[string]json.RawMessage
	if err := json.Unmarshal(data, &public); err != nil {
		return nil, err
	}
	for key, value := range public {
		row[key] = value
	}
	private := map[string]any{}
	if err := metadataPrivateFields(value, func(name string, item reflect.Value) error {
		private[name] = item.Interface()
		return nil
	}); err != nil {
		return nil, err
	}
	row["_storage_version"], row["_storage"] = 1, private
	return row, nil
}

func metadataWireRows(rows any) ([]map[string]any, error) {
	value := reflect.ValueOf(rows)
	result := make([]map[string]any, 0, value.Len())
	for i := 0; i < value.Len(); i++ {
		row, err := metadataWireRow(value.Index(i))
		if err != nil {
			return nil, err
		}
		result = append(result, row)
	}
	return result, nil
}

func decodeMetadataRows(data json.RawMessage, destination any) error {
	if err := json.Unmarshal(data, destination); err != nil {
		return err
	}
	var rows []struct {
		Version int                        `json:"_storage_version"`
		Private map[string]json.RawMessage `json:"_storage"`
	}
	if err := json.Unmarshal(data, &rows); err != nil {
		return err
	}
	values := reflect.ValueOf(destination).Elem()
	for i, row := range rows {
		if row.Version != 0 && row.Version != 1 {
			return code("metadata_snapshot_incomplete")
		}
		seen := 0
		if err := metadataPrivateFields(values.Index(i), func(name string, item reflect.Value) error {
			value, ok := row.Private[name]
			// Old public-only exports cannot prove recovery identities. Refuse
			// incomplete rows instead of restoring zero owners or new keys.
			if row.Version != 1 || !ok {
				return code("metadata_snapshot_incomplete")
			}
			seen++
			return json.Unmarshal(value, item.Addr().Interface())
		}); err != nil {
			return err
		}
		if seen != len(row.Private) {
			return code("metadata_snapshot_incomplete")
		}
	}
	return nil
}
