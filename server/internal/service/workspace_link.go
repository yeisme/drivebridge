package service

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Service) linkFileOn(tx *gorm.DB, p Principal, id, version string) (File, error) {
	if !collectionHumanPrincipal(p) {
		return File{}, code("forbidden")
	}
	if !propertyHexID(id) || !propertyHexID(version) {
		return File{}, code("invalid_request")
	}
	if err := savedQueryActorOn(tx, p); err != nil {
		return File{}, err
	}
	fixed, err := s.resourceFileOn(tx, p, id, version)
	if err != nil {
		return File{}, err
	}
	// resourceFileOn pins its result; retain the actual head for drift detection.
	var head File
	if err = tx.Where("id = ? AND owner = ?", fixed.ID, p.UserID).First(&head).Error; err != nil {
		return File{}, resourceReadError(err)
	}
	return head, nil
}

func ordinaryLinkDocument(file File) bool {
	ext := strings.ToLower(path.Ext(file.Path))
	if ext != ".md" && ext != ".markdown" {
		return false
	}
	// Pinax canonical/control records are never interpreted as ordinary links.
	for _, part := range strings.Split(strings.ToLower(file.Path), "/") {
		if part == ".pinax" {
			return false
		}
	}
	return true
}

func (s *Service) linkTargetOn(tx *gorm.DB, p Principal, source File, destination string) (File, string, error) {
	u, err := url.Parse(destination)
	if err != nil || u.User != nil || u.Opaque != "" || u.RawQuery != "" || len(u.Fragment) > 1024 {
		return File{}, "", code("not_found")
	}
	anchor := u.Fragment
	u.Fragment, u.RawFragment = "", ""
	if u.Scheme != "" {
		if u.Scheme != "drivebridge" || u.Host != s.Instance {
			return File{}, "", code("not_found")
		}
		id, version, err := s.resolveRef(u.String())
		if err != nil {
			return File{}, "", code("not_found")
		}
		file, err := s.resourceFileOn(tx, p, id, version)
		return file, anchor, err
	}
	if u.Host != "" || strings.HasPrefix(u.Path, "/") || strings.Contains(u.Path, "\\") {
		return File{}, "", code("not_found")
	}
	targetPath := source.Path
	if u.Path != "" {
		targetPath = path.Join(path.Dir(source.Path), u.Path)
	}
	clean, err := cleanPath(targetPath)
	if err != nil || clean != targetPath {
		return File{}, "", code("not_found")
	}
	var target File
	if err = tx.Where("owner = ? AND space_id = ? AND path = ?", p.UserID, source.SpaceID, targetPath).First(&target).Error; err != nil {
		return File{}, "", resourceReadError(err)
	}
	if target.ID == source.ID {
		target.VersionID = source.VersionID
	}
	file, err := s.resourceFileOn(tx, p, target.ID, target.VersionID)
	return file, anchor, err
}

func linkReceipt(row WorkspaceRequest, index WorkspaceFileLinkIndex) contract.FileLinkReceipt {
	return contract.FileLinkReceipt{FileID: index.FileID, VersionID: index.VersionID, RequestDigest: row.RequestDigest, CommittedAt: row.CreatedAt}
}

func (s *Service) linkReceiptOn(tx *gorm.DB, p Principal, id, version, key string) (contract.FileLinkReceipt, bool, error) {
	var receipt WorkspaceRequest
	err := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "links.rebuild", key).First(&receipt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return contract.FileLinkReceipt{}, false, nil
	}
	if err != nil {
		return contract.FileLinkReceipt{}, false, code("database_unavailable")
	}
	var index WorkspaceFileLinkIndex
	if err = tx.Where("id = ? AND owner = ? AND tenant_id = ?", receipt.ResourceID, p.UserID, p.TenantID).First(&index).Error; err != nil {
		return contract.FileLinkReceipt{}, false, code("dependency_unavailable")
	}
	if index.FileID != id || index.VersionID != version {
		return contract.FileLinkReceipt{}, false, code("idempotency_conflict")
	}
	return linkReceipt(receipt, index), true, nil
}

func (s *Service) LookupWorkspaceLinks(ctx context.Context, p Principal, id, version, key string) (contract.FileLinkReceipt, error) {
	var out contract.FileLinkReceipt
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, err := s.linkFileOn(tx, p, id, version); err != nil {
			return err
		}
		var found bool
		var err error
		out, found, err = s.linkReceiptOn(tx, p, id, version, key)
		if err != nil {
			return err
		}
		if !found {
			return code("not_found")
		}
		return nil
	})
	if err != nil {
		return contract.FileLinkReceipt{}, err
	}
	return out, nil
}

func (s *Service) RebuildWorkspaceLinks(ctx context.Context, p Principal, id string, q contract.FileLinkRebuild, key string) (contract.FileLinkReceipt, error) {
	var out contract.FileLinkReceipt
	if err := collectionChangeKey(key); err != nil {
		return out, err
	}
	file, err := s.linkFileOn(s.DB.WithContext(ctx), p, id, q.VersionID)
	if err != nil {
		return out, err
	}
	if !ordinaryLinkDocument(file) {
		return out, code("unsupported_action")
	}
	raw, _ := json.Marshal([]string{id, q.VersionID})
	digest := Hash(string(raw))
	// A replay never downloads bytes or rebuilds under the old request key.
	if prior, found, e := s.linkReceiptOn(s.DB.WithContext(ctx), p, id, q.VersionID, key); e != nil {
		return out, e
	} else if found {
		return prior, nil
	}
	_, version, body, err := s.Download(ctx, p, id, q.VersionID)
	if err != nil {
		return out, err
	}
	if version.Size < 0 || version.Size > workspaceLinkBytes {
		body.Close()
		return out, code("query_budget_exceeded")
	}
	bytes, readErr := io.ReadAll(io.LimitReader(body, workspaceLinkBytes+1))
	closeErr := body.Close()
	if readErr != nil || closeErr != nil {
		return out, code("storage_read_failed")
	}
	if int64(len(bytes)) != version.Size || len(bytes) > workspaceLinkBytes || Hash(string(bytes)) != version.SHA256 {
		return out, code("source_changed")
	}
	destinations, err := markdownLinkDestinations(ctx, bytes)
	if err != nil {
		return out, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	err = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		current, err := s.linkFileOn(tx, p, id, q.VersionID)
		if err != nil {
			return err
		}
		if current.Path != file.Path || current.SpaceID != file.SpaceID {
			return code("source_changed")
		}
		if !ordinaryLinkDocument(current) {
			return code("unsupported_action")
		}
		if prior, found, err := s.linkReceiptOn(tx, p, id, q.VersionID, key); err != nil {
			return err
		} else if found {
			out = prior
			return nil
		}
		current.VersionID = q.VersionID
		seen := map[string]bool{}
		rows := make([]WorkspaceFileLinkProjection, 0, len(destinations))
		now := time.Now().UTC()
		for _, destination := range destinations {
			target, anchor, err := s.linkTargetOn(tx, p, current, destination)
			if err != nil {
				if workspaceVisibilityDenied(err) {
					continue
				}
				return err
			}
			ref := s.FileRef(target)
			identity := ref + "\x00" + anchor
			if seen[identity] {
				continue
			}
			seen[identity] = true
			rows = append(rows, WorkspaceFileLinkProjection{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, SpaceID: current.SpaceID, FileID: id, VersionID: q.VersionID, TargetRef: ref, Anchor: anchor, CreatedAt: now})
		}
		var index WorkspaceFileLinkIndex
		err = tx.Where("file_id = ? AND version_id = ? AND owner = ? AND tenant_id = ?", id, q.VersionID, p.UserID, p.TenantID).First(&index).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			index = WorkspaceFileLinkIndex{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, SpaceID: current.SpaceID, FileID: id, VersionID: q.VersionID, CreatedAt: now, UpdatedAt: now}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&index)
			if result.Error != nil {
				return code("database_unavailable")
			}
			if result.RowsAffected != 1 {
				return code("revision_conflict")
			}
		} else if err != nil {
			return code("database_unavailable")
		} else {
			if err = tx.Model(&index).Update("updated_at", now).Error; err != nil {
				return code("database_unavailable")
			}
		}
		if err = tx.Where("file_id = ? AND version_id = ? AND owner = ? AND tenant_id = ?", id, q.VersionID, p.UserID, p.TenantID).Delete(&WorkspaceFileLinkProjection{}).Error; err != nil {
			return code("database_unavailable")
		}
		if len(rows) > 0 {
			if err = tx.CreateInBatches(rows, 100).Error; err != nil {
				return code("database_unavailable")
			}
		}
		receipt := WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: "links.rebuild", RequestKey: key, RequestDigest: digest, ResourceID: index.ID, ResultRevision: 1, CreatedAt: now}
		if err = tx.Create(&receipt).Error; err != nil {
			return code("database_unavailable")
		}
		out = linkReceipt(receipt, index)
		return nil
	})
	if err != nil {
		return contract.FileLinkReceipt{}, err
	}
	return out, nil
}

func (s *Service) WorkspaceLinks(ctx context.Context, p Principal, id, version string, backlinks bool, page contract.PageQuery) (contract.FileLinkList, error) {
	out := contract.FileLinkList{FileID: id, VersionID: version, Items: []contract.FileLinkView{}, ObservedAt: time.Now().UTC(), TotalKind: "unknown"}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		file, err := s.linkFileOn(tx, p, id, version)
		if err != nil {
			return err
		}
		out.Stale = file.VersionID != version
		if !backlinks && !ordinaryLinkDocument(file) {
			return code("unsupported_action")
		}
		var index WorkspaceFileLinkIndex
		err = tx.Where("file_id = ? AND version_id = ? AND owner = ? AND tenant_id = ?", id, version, p.UserID, p.TenantID).First(&index).Error
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		out.Indexed = err == nil
		query := tx.Where("owner = ? AND tenant_id = ?", p.UserID, p.TenantID)
		if backlinks {
			query = query.Where("target_ref = ?", s.FileRef(File{ID: id, VersionID: version}))
		} else {
			query = query.Where("file_id = ? AND version_id = ?", id, version)
		}
		var rows []WorkspaceFileLinkProjection
		if err = query.Order("file_id, version_id, target_ref, anchor").Limit(workspaceMetadataBudget + 1).Find(&rows).Error; err != nil {
			return code("database_unavailable")
		}
		if len(rows) > workspaceMetadataBudget {
			return code("query_budget_exceeded")
		}
		for _, row := range rows {
			source, err := s.linkFileOn(tx, p, row.FileID, row.VersionID)
			if err != nil {
				if workspaceVisibilityDenied(err) {
					continue
				}
				return err
			}
			if !ordinaryLinkDocument(source) || row.SpaceID != source.SpaceID {
				continue
			}
			targetID, targetVersion, err := s.resolveRef(row.TargetRef)
			if err != nil {
				return code("dependency_unavailable")
			}
			target, err := s.linkFileOn(tx, p, targetID, targetVersion)
			if err != nil {
				if workspaceVisibilityDenied(err) {
					continue
				}
				return err
			}
			out.Items = append(out.Items, contract.FileLinkView{FileRef: s.FileRef(File{ID: row.FileID, VersionID: row.VersionID}), TargetRef: row.TargetRef, Anchor: row.Anchor, SourcePath: source.Path, TargetPath: target.Path, Stale: source.VersionID != row.VersionID || target.VersionID != targetVersion})
		}
		keys := make([]string, len(out.Items))
		for i, view := range out.Items {
			encoded, _ := json.Marshal([]string{view.FileRef, view.TargetRef, view.Anchor})
			keys[i] = string(encoded)
		}
		kind := "workspace.links"
		if backlinks { kind = "workspace.backlinks" }
		generation := struct { Indexed bool; Stale bool; Items []contract.FileLinkView }{out.Indexed, out.Stale, out.Items}
		start, end, next, err := s.workspaceMetadataPage(p, kind, []string{id, version}, page, generation, keys)
		if err != nil { return err }
		out.Items, out.NextCursor = out.Items[start:end], next
		return nil
	})
	if err != nil {
		return contract.FileLinkList{}, err
	}
	return out, nil
}
