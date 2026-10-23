package bridge

import (
	"context"
	"errors"
	"gorm.io/gorm"
	"os"
	"sort"
)

// ResourceDirectory reads existing metadata through the same domain records.
// The caller supplies a read-only GORM connection; this function never migrates.
func ResourceDirectory(ctx context.Context, db *gorm.DB, kind string) ([]map[string]any, error) {
	rows := []map[string]any{}
	switch kind {
	case "connection":
		var values []Connection
		if err := db.WithContext(ctx).Find(&values).Error; err != nil {
			return nil, err
		}
		for _, v := range values {
			configured := true
			if v.CredentialSource == "env" {
				configured = os.Getenv("DRIVEBRIDGE_TOKEN") != ""
			}
			if v.CredentialSource == "token-file" {
				info, err := os.Stat(v.TokenFile)
				configured = err == nil && info.Mode().IsRegular() && info.Mode().Perm()&0077 == 0
			}
			rows = append(rows, map[string]any{"id": v.ID, "connection_id": v.ID, "label": v.Label, "base_url": v.BaseURL, "instance_id": v.InstanceID, "space_id": v.SpaceID, "project_ref": v.ProjectRef, "credential_source": v.CredentialSource, "credential_configured": configured, "synthetic": false})
		}
	case "bind":
		var values []BindRecord
		if err := db.WithContext(ctx).Find(&values).Error; err != nil {
			return nil, err
		}
		for _, v := range values {
			row := map[string]any{"id": v.ID, "bind_id": v.ID, "consumer": v.Consumer, "project_ref": v.ProjectRef, "space": v.SpaceID, "prefix": v.Prefix, "purpose": v.Purpose, "revoked": v.RevokedAt != nil}
			if v.IdempotencyKey != "" {
				row["idempotency_key"] = v.IdempotencyKey
			}
			if v.ConnectionID != "" {
				row["connection_id"] = v.ConnectionID
			}
			rows = append(rows, row)
		}
	default:
		return nil, errors.New("unknown_resource")
	}
	sort.Slice(rows, func(i, j int) bool { return rows[i]["id"].(string) < rows[j]["id"].(string) })
	return rows, nil
}
