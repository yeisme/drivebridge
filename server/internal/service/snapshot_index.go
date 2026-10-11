package service

import (
	"context"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
)

// ListCollectionSnapshots observes at most 1000 historical headers. No entry
// counts, paths, file references, content or request receipts are inspected.
// Larger histories fail explicitly rather than returning a truncated success.
func (s *Service) ListCollectionSnapshots(ctx context.Context, p Principal, id string) (contract.SnapshotIndex, error) {
	var out contract.SnapshotIndex
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		collection, err := s.snapshotCreationAdmissionOn(tx, p, id, false)
		if err != nil {
			return err
		}
		var rows []CollectionSnapshot
		if err = tx.Select("id", "collection_id", "owner", "revision", "created_at").Where("collection_id = ? AND owner = ?", id, p.UserID).Order("created_at DESC, id DESC").Limit(1001).Find(&rows).Error; err != nil {
			return code("database_unavailable")
		}
		if len(rows) > 1000 {
			return code("query_budget_exceeded")
		}
		out = contract.SnapshotIndex{CollectionID: id, Items: []contract.SnapshotIndexItem{}, ObservedAt: time.Now().UTC()}
		for _, row := range rows {
			if !propertyHexID(row.ID) || row.CollectionID != id || row.Owner != p.UserID || row.Revision == 0 || row.Revision > 9007199254740991 || row.CreatedAt.IsZero() {
				return code("dependency_unavailable")
			}
			out.Items = append(out.Items, contract.SnapshotIndexItem{SnapshotID: row.ID, CollectionRevision: row.Revision, CreatedAt: row.CreatedAt})
		}
		current, err := s.snapshotCreationAdmissionOn(tx, p, id, false)
		if err != nil {
			return err
		}
		if current.Revision != collection.Revision || current.SpaceID != collection.SpaceID || current.Kind != collection.Kind || current.Rule != collection.Rule || current.Scope != collection.Scope {
			return code("query_changed")
		}
		return nil
	})
	if err != nil {
		return contract.SnapshotIndex{}, err
	}
	return out, nil
}
