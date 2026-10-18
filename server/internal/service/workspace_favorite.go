package service

import (
	"context"
	"encoding/json"
	"errors"
	"net/url"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

const workspaceMetadataBudget = 1000

// Metadata bindings never inherit Agent write scope or grant target access.
func (s *Service) favoriteScopeOn(tx *gorm.DB, p Principal, space string) error {
	return s.propertySpaceOn(tx, p, space, false)
}

func (s *Service) favoriteTargetOn(tx *gorm.DB, p Principal, space, kind, ref string) error {
	if len(ref) > 4096 || !utf8.ValidString(ref) {
		return code("invalid_request")
	}
	u, err := url.Parse(ref)
	if err != nil || u.Scheme != "drivebridge" || u.Host != s.Instance || u.User != nil || u.Opaque != "" || u.RawQuery != "" || u.Fragment != "" || u.RawPath != "" || ref != u.String() {
		return code("invalid_request")
	}
	parts := strings.Split(strings.TrimPrefix(u.Path, "/"), "/")
	switch kind {
	case "file":
		if len(parts) != 2 || parts[0] != "file" || !propertyHexID(parts[1]) {
			return code("invalid_request")
		}
		var row File
		if err = tx.Where("id = ? AND owner = ? AND space_id = ?", parts[1], p.UserID, space).First(&row).Error; err != nil {
			return resourceReadError(err)
		}
		if row.Trashed || row.PurgeStartedAt != nil || workspaceStagingPath(row.Path) {
			return code("not_found")
		}
		return s.allowOn(tx, p, space, row.Path, "read")
	case "collection":
		if len(parts) != 2 || parts[0] != "collection" || !propertyHexID(parts[1]) {
			return code("invalid_request")
		}
		row, err := s.collectionReadAdmissionOn(tx, p, parts[1])
		if err != nil {
			return err
		}
		if row.SpaceID != space {
			return code("not_found")
		}
		return nil
	case "directory":
		// Managed roots are the space ID; external roots retain their distinct
		// connection/root/provider item identity, never a bare display path.
		if len(parts) < 6 || parts[0] != "connection" || parts[2] != "root" || parts[4] != "directory" {
			return code("invalid_request")
		}
		var sp Space
		if err = tx.Where("id = ? AND owner = ? AND tenant_id = ?", space, p.UserID, p.TenantID).First(&sp).Error; err != nil {
			return resourceReadError(err)
		}
		if parts[1] != sp.ConnectionID {
			return code("not_found")
		}
		var connection Connection
		if err = tx.Where("id = ?", sp.ConnectionID).First(&connection).Error; err != nil {
			return resourceReadError(err)
		}
		if connection.ID != "platform" && connection.Owner != p.UserID {
			return code("not_found")
		}
		if connection.Disabled {
			return code("not_found")
		}
		directory := strings.Join(parts[5:], "/")
		if parts[3] == space {
			if directory == "" {
				return s.allowOn(tx, p, space, "", "read")
			}
			clean, e := cleanPath(directory)
			if e != nil || clean != directory || workspaceStagingPath(directory) {
				return code("invalid_request")
			}
			var row Directory
			if err = tx.Where("owner = ? AND space_id = ? AND path = ?", p.UserID, space, directory).First(&row).Error; err != nil {
				return resourceReadError(err)
			}
		} else {
			var root CloudRoot
			if err = tx.Where("id = ? AND owner = ? AND tenant_id = ? AND connection_id = ?", parts[3], p.UserID, p.TenantID, parts[1]).First(&root).Error; err != nil {
				return resourceReadError(err)
			}
			if root.Status != "bound" && root.Status != "observed" {
				return code("not_found")
			}
			var entry ExternalEntry
			if err = tx.Where("root_id = ? AND owner = ? AND connection_id = ? AND provider_item_id = ? AND kind = ? AND tombstoned_at IS NULL", root.ID, p.UserID, parts[1], directory, "directory").First(&entry).Error; err != nil {
				return resourceReadError(err)
			}
			directory = entry.Path
		}
		return s.allowOn(tx, p, space, directory, "read")
	case "tag":
		if len(parts) < 3 || parts[0] != "tag" || (parts[1] != "manual" && parts[1] != "rule" && parts[1] != "suggested") {
			return code("invalid_request")
		}
		label := strings.Join(parts[2:], "/")
		if strings.TrimSpace(label) != label || label == "" || len(label) > 200 {
			return code("invalid_request")
		}
		for _, r := range label {
			if unicode.IsControl(r) {
				return code("invalid_request")
			}
		}
		var labels []FileLabelAssignment
		if err = tx.Where("owner = ? AND namespace = ? AND label = ?", p.UserID, parts[1], label).Order("id").Limit(workspaceMetadataBudget + 1).Find(&labels).Error; err != nil {
			return code("database_unavailable")
		}
		if len(labels) > workspaceMetadataBudget {
			return code("query_budget_exceeded")
		}
		for _, label := range labels {
			var file File
			err = tx.Where("id = ? AND owner = ? AND space_id = ?", label.FileID, p.UserID, space).First(&file).Error
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			if err != nil {
				return code("database_unavailable")
			}
			if file.Trashed || file.PurgeStartedAt != nil || workspaceStagingPath(file.Path) {
				continue
			}
			if err = s.allowOn(tx, p, space, file.Path, "read"); err == nil {
				return nil
			}
			if !workspaceVisibilityDenied(err) {
				return err
			}
		}
		return code("not_found")
	default:
		return code("invalid_request")
	}
}

func (s *Service) WorkspaceFavorites(ctx context.Context, p Principal, space string, page contract.PageQuery) (contract.FavoriteList, error) {
	out := contract.FavoriteList{Items: []contract.FavoriteView{}, ObservedAt: time.Now().UTC(), TotalKind: "unknown"}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.favoriteScopeOn(tx, p, space); err != nil {
			return err
		}
		var rows []WorkspaceFavorite
		if err := tx.Where("owner = ? AND tenant_id = ? AND space_id = ?", p.UserID, p.TenantID, space).Order("id").Limit(workspaceMetadataBudget + 1).Find(&rows).Error; err != nil {
			return code("database_unavailable")
		}
		if len(rows) > workspaceMetadataBudget {
			return code("query_budget_exceeded")
		}
		for _, row := range rows {
			if err := s.favoriteTargetOn(tx, p, space, row.TargetKind, row.TargetRef); err != nil {
				if workspaceVisibilityDenied(err) {
					continue
				}
				return err
			}
			out.Items = append(out.Items, contract.FavoriteView{FavoriteID: row.ID, SpaceID: space, TargetKind: row.TargetKind, TargetRef: row.TargetRef, Revision: row.Revision, Deleted: row.Deleted})
		}
		keys := make([]string, len(out.Items))
		for i, view := range out.Items {
			keys[i] = view.FavoriteID
		}
		start, end, next, err := s.workspaceMetadataPage(p, "workspace.favorites", []string{space}, page, out.Items, keys)
		if err != nil {
			return err
		}
		out.Items, out.NextCursor = out.Items[start:end], next
		return nil
	})
	if err != nil {
		return contract.FavoriteList{}, err
	}
	return out, nil
}

func favoriteReceipt(row WorkspaceRequest, space string) contract.FavoriteReceipt {
	return contract.FavoriteReceipt{FavoriteID: row.ResourceID, SpaceID: space, ResultRevision: row.ResultRevision, Deleted: row.Action == "favorite.delete", RequestDigest: row.RequestDigest, CommittedAt: row.CreatedAt}
}

func (s *Service) CreateWorkspaceFavorite(ctx context.Context, p Principal, space string, q contract.FavoriteCreate, key string) (contract.FavoriteReceipt, error) {
	return s.changeWorkspaceFavorite(ctx, p, space, "", q, false, key)
}
func (s *Service) DeleteWorkspaceFavorite(ctx context.Context, p Principal, id string, q contract.FavoriteDelete, key string) (contract.FavoriteReceipt, error) {
	if !collectionHumanPrincipal(p) {
		return contract.FavoriteReceipt{}, code("forbidden")
	}
	var row WorkspaceFavorite
	if err := s.DB.WithContext(ctx).Where("id = ? AND owner = ? AND tenant_id = ?", id, p.UserID, p.TenantID).First(&row).Error; err != nil {
		return contract.FavoriteReceipt{}, resourceReadError(err)
	}
	return s.changeWorkspaceFavorite(ctx, p, row.SpaceID, id, contract.FavoriteCreate{ExpectedRevision: q.ExpectedRevision}, true, key)
}

func (s *Service) changeWorkspaceFavorite(ctx context.Context, p Principal, space, id string, q contract.FavoriteCreate, remove bool, key string) (contract.FavoriteReceipt, error) {
	var out contract.FavoriteReceipt
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	if q.ExpectedRevision >= 9007199254740991 || (remove && !propertyHexID(id)) {
		return out, code("invalid_request")
	}
	action := "favorite.create"
	if remove {
		action = "favorite.delete"
	}
	raw, _ := json.Marshal(struct {
		SpaceID string
		ID      string
		Request contract.FavoriteCreate
	}{space, id, q})
	digest := Hash(string(raw))
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.favoriteScopeOn(tx, p, space); err != nil {
			return err
		}
		if !remove {
			if err := s.favoriteTargetOn(tx, p, space, q.TargetKind, q.TargetRef); err != nil {
				return err
			}
		}
		var receipt WorkspaceRequest
		err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, action, key).First(&receipt).Error
		if err == nil {
			if receipt.RequestDigest != digest {
				return code("idempotency_conflict")
			}
			out = favoriteReceipt(receipt, space)
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		var row WorkspaceFavorite
		query := tx.Where("owner = ? AND tenant_id = ? AND space_id = ?", p.UserID, p.TenantID, space)
		if remove {
			query = query.Where("id = ?", id)
		} else {
			query = query.Where("target_kind = ? AND target_ref = ?", q.TargetKind, q.TargetRef)
		}
		err = query.First(&row).Error
		missing := errors.Is(err, gorm.ErrRecordNotFound)
		if err != nil && !missing {
			return code("database_unavailable")
		}
		if remove && missing {
			return code("not_found")
		}
		if row.Revision != q.ExpectedRevision || (!missing && row.Deleted == remove) {
			return code("revision_conflict")
		}
		now := time.Now().UTC()
		if missing {
			row = WorkspaceFavorite{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, SpaceID: space, TargetKind: q.TargetKind, TargetRef: q.TargetRef, Revision: 1, CreatedAt: now, UpdatedAt: now}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
			if result.Error != nil {
				return code("database_unavailable")
			}
			if result.RowsAffected != 1 {
				return code("revision_conflict")
			}
		} else {
			result := tx.Model(&WorkspaceFavorite{}).Where("id = ? AND owner = ? AND tenant_id = ? AND revision = ? AND deleted = ?", row.ID, p.UserID, p.TenantID, row.Revision, row.Deleted).Updates(map[string]any{"revision": row.Revision + 1, "deleted": remove, "updated_at": now})
			if result.Error != nil {
				return code("database_unavailable")
			}
			if result.RowsAffected != 1 {
				return code("revision_conflict")
			}
			row.Revision++
		}
		receipt = WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: action, RequestKey: key, RequestDigest: digest, ResourceID: row.ID, ResultRevision: row.Revision, CreatedAt: now}
		if err = tx.Create(&receipt).Error; err != nil {
			return code("database_unavailable")
		}
		out = favoriteReceipt(receipt, space)
		return nil
	})
	if err != nil {
		return contract.FavoriteReceipt{}, err
	}
	return out, nil
}

func (s *Service) LookupWorkspaceFavorite(ctx context.Context, p Principal, space string, q contract.FavoriteLookup) (contract.FavoriteReceipt, error) {
	var out contract.FavoriteReceipt
	if err := collectionChangeKey(q.IdempotencyKey); err != nil {
		return out, err
	}
	if q.Action != "create" && q.Action != "delete" {
		return out, code("invalid_request")
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := s.favoriteScopeOn(tx, p, space); err != nil {
			return err
		}
		var receipt WorkspaceRequest
		if err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "favorite."+q.Action, q.IdempotencyKey).First(&receipt).Error; err != nil {
			return resourceReadError(err)
		}
		var row WorkspaceFavorite
		if err := tx.Where("id = ? AND owner = ? AND tenant_id = ? AND space_id = ?", receipt.ResourceID, p.UserID, p.TenantID, space).First(&row).Error; err != nil {
			return resourceReadError(err)
		}
		if q.Action == "create" {
			if err := s.favoriteTargetOn(tx, p, space, row.TargetKind, row.TargetRef); err != nil {
				return err
			}
		}
		out = favoriteReceipt(receipt, space)
		return nil
	})
	if err != nil {
		return contract.FavoriteReceipt{}, err
	}
	return out, nil
}
