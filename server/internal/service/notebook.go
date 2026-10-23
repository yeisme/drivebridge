package service

import (
	"context"
	"encoding/json"
	"errors"
	"path"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/yeisme/drivebridge-server/internal/contract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func notebookIcon(value string) bool {
	switch value {
	case "file", "book", "calendar", "idea", "project":
		return true
	}
	return false
}

func (s *Service) notebookFileOn(tx *gorm.DB, p Principal, id string, write bool) (File, error) {
	var file File
	if err := workspaceGroupHumanOn(tx, p); err != nil {
		return file, err
	}
	if err := tx.Where("id = ? AND owner = ?", id, p.UserID).First(&file).Error; err != nil {
		return file, resourceReadError(err)
	}
	if err := s.propertySpaceOn(tx, p, file.SpaceID, false); err != nil {
		return File{}, err
	}
	if file.Trashed || file.PurgeStartedAt != nil || workspaceStagingPath(file.Path) || (strings.ToLower(path.Ext(file.Path)) != ".md" && strings.ToLower(path.Ext(file.Path)) != ".markdown") {
		return File{}, code("not_found")
	}
	action := "read"
	if write {
		action = "write"
		if !s.Config.DocumentEditing {
			return File{}, code("unsupported_action")
		}
	}
	if err := s.allowOn(tx, p, file.SpaceID, file.Path, action); err != nil {
		return File{}, err
	}
	return file, nil
}

func (s *Service) notebookPageOn(tx *gorm.DB, p Principal, file File) (contract.NotebookPage, error) {
	out := contract.NotebookPage{FileID: file.ID, SpaceID: file.SpaceID, VersionID: file.VersionID, Path: file.Path, Icon: "file"}
	var row NotebookPageMetadata
	err := tx.Where("file_id = ? AND owner = ? AND tenant_id = ? AND space_id = ?", file.ID, p.UserID, p.TenantID, file.SpaceID).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return out, nil
	}
	if err != nil {
		return out, code("database_unavailable")
	}
	out.Icon, out.Revision = row.Icon, row.Revision
	if row.ParentFileID != "" {
		parent, e := s.notebookFileOn(tx, p, row.ParentFileID, false)
		if e == nil && parent.SpaceID == file.SpaceID {
			out.ParentFileID = parent.ID
		} else if e != nil && !workspaceVisibilityDenied(e) {
			return out, e
		}
	}
	return out, nil
}

func (s *Service) NotebookPage(ctx context.Context, p Principal, id string) (contract.NotebookPage, error) {
	var out contract.NotebookPage
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		file, e := s.notebookFileOn(tx, p, id, false)
		if e != nil {
			return e
		}
		out, e = s.notebookPageOn(tx, p, file)
		return e
	})
	return out, err
}

func (s *Service) NotebookPages(ctx context.Context, p Principal, space string, q contract.PageQuery) (contract.Page[contract.NotebookPage], error) {
	out := contract.Page[contract.NotebookPage]{Items: []contract.NotebookPage{}, ObservedAt: time.Now().UTC(), TotalKind: "unknown"}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := s.propertySpaceOn(tx, p, space, false); e != nil {
			return e
		}
		var rows []File
		if e := tx.Where("owner = ? AND space_id = ? AND trashed = ?", p.UserID, space, false).Where("LOWER(path) LIKE ? OR LOWER(path) LIKE ?", "%.md", "%.markdown").Order("id ASC").Limit(workspaceMetadataBudget + 1).Find(&rows).Error; e != nil {
			return code("database_unavailable")
		}
		if len(rows) > workspaceMetadataBudget {
			return code("query_budget_exceeded")
		}
		keys := []string{}
		for _, row := range rows {
			file, e := s.notebookFileOn(tx, p, row.ID, false)
			if e != nil {
				if workspaceVisibilityDenied(e) {
					continue
				}
				return e
			}
			view, e := s.notebookPageOn(tx, p, file)
			if e != nil {
				return e
			}
			out.Items = append(out.Items, view)
			keys = append(keys, row.ID)
		}
		start, end, next, e := s.workspaceMetadataPage(p, "notebook.pages", []string{space}, q, out.Items, keys)
		if e != nil {
			return e
		}
		out.Items, out.NextCursor = out.Items[start:end], next
		return nil
	})
	return out, err
}

func (s *Service) notebookParentOn(tx *gorm.DB, p Principal, space, id, parent string) error {
	seen := map[string]bool{id: true}
	for depth := 0; parent != ""; depth++ {
		if seen[parent] || depth >= 100 {
			return code("invalid_request")
		}
		seen[parent] = true
		file, e := s.notebookFileOn(tx, p, parent, false)
		if e != nil {
			return e
		}
		if file.SpaceID != space {
			return code("not_found")
		}
		var row NotebookPageMetadata
		e = tx.Where("file_id = ? AND owner = ? AND tenant_id = ? AND space_id = ?", parent, p.UserID, p.TenantID, space).First(&row).Error
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return nil
		}
		if e != nil {
			return code("database_unavailable")
		}
		parent = row.ParentFileID
	}
	return nil
}

func (s *Service) PatchNotebookPage(ctx context.Context, p Principal, id string, q contract.NotebookPagePatch, key string) (contract.NotebookPageReceipt, error) {
	var out contract.NotebookPageReceipt
	if !notebookIcon(q.Icon) || q.ExpectedRevision >= 9007199254740991 {
		return out, code("invalid_request")
	}
	if e := collectionChangeKey(key); e != nil {
		return out, e
	}
	raw, _ := json.Marshal(struct {
		ID      string
		Request contract.NotebookPagePatch
	}{id, q})
	digest := Hash(string(raw))
	s.mu.Lock()
	defer s.mu.Unlock()
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		file, e := s.notebookFileOn(tx, p, id, true)
		if e != nil {
			return e
		}
		// Serialize relation edits across server processes on the existing space row.
		var sp Space
		if e = tx.Clauses(clause.Locking{Strength: "UPDATE"}).Where("id = ? AND owner = ?", file.SpaceID, p.UserID).First(&sp).Error; e != nil {
			return resourceReadError(e)
		}
		var receipt WorkspaceRequest
		e = tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ?", p.UserID, p.TenantID, p.UserID, "notebook.page", key).First(&receipt).Error
		if e == nil {
			if receipt.RequestDigest != digest {
				return code("idempotency_conflict")
			}
			out = contract.NotebookPageReceipt{FileID: id, ResultRevision: receipt.ResultRevision}
			return nil
		}
		if !errors.Is(e, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		if e = s.notebookParentOn(tx, p, file.SpaceID, id, q.ParentFileID); e != nil {
			return e
		}
		var row NotebookPageMetadata
		e = tx.Where("file_id = ? AND owner = ? AND tenant_id = ? AND space_id = ?", id, p.UserID, p.TenantID, file.SpaceID).First(&row).Error
		missing := errors.Is(e, gorm.ErrRecordNotFound)
		if e != nil && !missing {
			return code("database_unavailable")
		}
		if row.Revision != q.ExpectedRevision {
			return code("revision_conflict")
		}
		now := time.Now().UTC()
		if missing {
			row = NotebookPageMetadata{FileID: id, Owner: p.UserID, TenantID: p.TenantID, SpaceID: file.SpaceID, ParentFileID: q.ParentFileID, Icon: q.Icon, Revision: 1, CreatedAt: now, UpdatedAt: now}
			result := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row)
			if result.Error != nil {
				return code("database_unavailable")
			}
			if result.RowsAffected != 1 {
				return code("revision_conflict")
			}
		} else {
			result := tx.Model(&NotebookPageMetadata{}).Where("file_id = ? AND revision = ?", id, row.Revision).Updates(map[string]any{"parent_file_id": q.ParentFileID, "icon": q.Icon, "revision": row.Revision + 1, "updated_at": now})
			if result.Error != nil {
				return code("database_unavailable")
			}
			if result.RowsAffected != 1 {
				return code("revision_conflict")
			}
			row.Revision++
		}
		receipt = WorkspaceRequest{ID: ID(), Owner: p.UserID, TenantID: p.TenantID, ActorID: p.UserID, Action: "notebook.page", RequestKey: key, RequestDigest: digest, ResourceID: id, ResultRevision: row.Revision, CreatedAt: now}
		if e = tx.Create(&receipt).Error; e != nil {
			return code("database_unavailable")
		}
		if e = tx.Create(&Audit{Owner: p.UserID, ActorID: p.UserID, TenantID: p.TenantID, Action: "notebook.page.update", Resource: id, Outcome: "success", At: now}).Error; e != nil {
			return code("database_unavailable")
		}
		out = contract.NotebookPageReceipt{FileID: id, ResultRevision: row.Revision}
		return nil
	})
	return out, err
}

func (s *Service) LookupNotebookPage(ctx context.Context, p Principal, id, key string) (contract.NotebookPageReceipt, error) {
	var out contract.NotebookPageReceipt
	if e := collectionChangeKey(key); e != nil {
		return out, e
	}
	err := s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if _, e := s.notebookFileOn(tx, p, id, false); e != nil {
			return e
		}
		var row WorkspaceRequest
		if e := tx.Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ? AND resource_id = ?", p.UserID, p.TenantID, p.UserID, "notebook.page", key, id).First(&row).Error; e != nil {
			return resourceReadError(e)
		}
		out = contract.NotebookPageReceipt{FileID: id, ResultRevision: row.ResultRevision}
		return nil
	})
	return out, err
}

func notebookIdentity(p Principal, space, key, date, directory string) (string, error) {
	if e := collectionChangeKey(key); e != nil {
		return "", e
	}
	if date != "" {
		d, e := time.Parse("2006-01-02", date)
		if e != nil || d.Format("2006-01-02") != date {
			return "", code("invalid_request")
		}
		key = "daily:" + date
		if directory != "" {
			key += ":" + directory
		}
	} else {
		key = "request:" + key
	}
	raw, _ := json.Marshal([]string{p.UserID, p.TenantID, space, key})
	return Hash(string(raw)), nil
}

func (s *Service) notebookCreationView(ctx context.Context, p Principal, row NotebookCreationIntent) (contract.NotebookCreation, error) {
	out := contract.NotebookCreation{Found: true, Status: "pending"}
	var op Operation
	e := s.DB.WithContext(ctx).Where("owner = ? AND actor_id = ? AND key = ?", p.UserID, p.UserID, "notebook:"+row.ID).First(&op).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return out, nil
	}
	if e != nil {
		return out, code("database_unavailable")
	}
	if _, e = s.operation(p, op.ID, "read"); e != nil {
		return contract.NotebookCreation{}, e
	}
	out.OperationID = op.ID
	if op.Status != "completed" {
		out.Status = op.Status
		return out, nil
	}
	page, e := s.NotebookPage(ctx, p, op.FileID)
	if e != nil {
		return out, e
	}
	var receipt WorkspaceRequest
	if e = s.DB.WithContext(ctx).Where("owner = ? AND tenant_id = ? AND actor_id = ? AND action = ? AND request_key = ? AND resource_id = ?", p.UserID, p.TenantID, p.UserID, "notebook.page", "notebook-meta:"+row.ID, op.FileID).First(&receipt).Error; e != nil {
		if errors.Is(e, gorm.ErrRecordNotFound) {
			return out, nil
		}
		return out, code("database_unavailable")
	}
	out.Status = "completed"
	out.Page = &page
	return out, nil
}

func (s *Service) LookupNotebookCreation(ctx context.Context, p Principal, space string, q contract.NotebookCreateLookup) (contract.NotebookCreation, error) {
	directory, e := notebookDirectoryPath(q.DirectoryPath)
	if e != nil {
		return contract.NotebookCreation{}, e
	}
	identity, e := notebookIdentity(p, space, q.IdempotencyKey, q.DailyDate, directory)
	if e != nil {
		return contract.NotebookCreation{}, e
	}
	if e = s.propertySpaceOn(s.DB.WithContext(ctx), p, space, false); e != nil {
		return contract.NotebookCreation{}, e
	}
	var row NotebookCreationIntent
	e = s.DB.WithContext(ctx).Where("identity = ? AND owner = ? AND tenant_id = ? AND space_id = ?", identity, p.UserID, p.TenantID, space).First(&row).Error
	if errors.Is(e, gorm.ErrRecordNotFound) {
		return contract.NotebookCreation{Status: "not_found"}, nil
	}
	if e != nil {
		return contract.NotebookCreation{}, code("database_unavailable")
	}
	return s.notebookCreationView(ctx, p, row)
}

// Resume accepts only an existing identity. It never invents another target or
// accepts replacement content after an unknown publication result.
func (s *Service) ResumeNotebookCreation(ctx context.Context, p Principal, space string, q contract.NotebookCreateLookup) (contract.NotebookCreation, error) {
	directory, e := notebookDirectoryPath(q.DirectoryPath)
	if e != nil {
		return contract.NotebookCreation{}, e
	}
	identity, e := notebookIdentity(p, space, q.IdempotencyKey, q.DailyDate, directory)
	if e != nil {
		return contract.NotebookCreation{}, e
	}
	if e = s.propertySpaceOn(s.DB.WithContext(ctx), p, space, false); e != nil {
		return contract.NotebookCreation{}, e
	}
	var row NotebookCreationIntent
	if e = s.DB.WithContext(ctx).Where("identity = ? AND owner = ? AND tenant_id = ? AND space_id = ?", identity, p.UserID, p.TenantID, space).First(&row).Error; e != nil {
		return contract.NotebookCreation{}, resourceReadError(e)
	}
	return s.CreateNotebookPage(ctx, p, space, contract.NotebookCreate{SpaceID: space, Title: row.Title, DirectoryPath: row.DirectoryPath, ParentFileID: row.ParentFileID, Icon: row.Icon, DailyDate: q.DailyDate}, q.IdempotencyKey)
}

func notebookDirectoryPath(value string) (string, error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return "", nil
	}
	return cleanPath(value)
}

func notebookPhysicalTitle(value string) (string, error) {
	if strings.ContainsAny(value, "/\\") {
		return "", code("invalid_request")
	}
	for _, suffix := range []string{".markdown", ".md"} {
		if len(value) > len(suffix) && strings.EqualFold(value[len(value)-len(suffix):], suffix) {
			value = strings.TrimSpace(value[:len(value)-len(suffix)])
			break
		}
	}
	if value == "" {
		return "", code("invalid_request")
	}
	return value, nil
}

func (s *Service) CreateNotebookPage(ctx context.Context, p Principal, space string, q contract.NotebookCreate, key string) (contract.NotebookCreation, error) {
	var empty contract.NotebookCreation
	if q.SpaceID != "" && q.SpaceID != space {
		return empty, code("invalid_request")
	}
	q.SpaceID = space
	directory, e := notebookDirectoryPath(q.DirectoryPath)
	if e != nil {
		return empty, e
	}
	q.DirectoryPath = directory
	identity, e := notebookIdentity(p, space, key, q.DailyDate, directory)
	if e != nil {
		return empty, e
	}
	if !s.Config.DocumentEditing {
		return empty, code("unsupported_action")
	}
	if q.DailyDate != "" {
		if q.ParentFileID != "" {
			return empty, code("invalid_request")
		}
		q.Title = q.DailyDate
		q.Icon = "calendar"
	}
	q.Title = strings.TrimSpace(q.Title)
	if directory != "" {
		q.Title, e = notebookPhysicalTitle(q.Title)
		if e != nil {
			return empty, e
		}
	}
	if !notebookIcon(q.Icon) || !utf8.ValidString(q.Title) || utf8.RuneCountInString(q.Title) < 1 || utf8.RuneCountInString(q.Title) > 120 {
		return empty, code("invalid_request")
	}
	for _, r := range q.Title {
		if unicode.IsControl(r) {
			return empty, code("invalid_request")
		}
	}
	raw, _ := json.Marshal(q)
	digest := Hash(string(raw))
	var row NotebookCreationIntent
	e = s.DB.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if e := s.propertySpaceOn(tx, p, space, false); e != nil {
			return e
		}
		if e := s.notebookParentOn(tx, p, space, "", q.ParentFileID); e != nil {
			return e
		}
		var existing NotebookCreationIntent
		lookupErr := tx.Where("identity = ? AND owner = ? AND tenant_id = ? AND space_id = ?", identity, p.UserID, p.TenantID, space).First(&existing).Error
		if lookupErr == nil {
			if existing.Digest != digest {
				return code("idempotency_conflict")
			}
			row = existing
			return nil
		}
		if !errors.Is(lookupErr, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		id := ID()
		target := "笔记/" + id + ".md"
		if q.DailyDate != "" {
			target = "日记/" + q.DailyDate + ".md"
		}
		if directory != "" {
			target = path.Join(directory, q.Title+".md")
		}
		if e := s.allowOn(tx, p, space, target, "write"); e != nil {
			return e
		}
		if e := pathAvailable(tx, p.dataOwner(), space, target, ""); e != nil {
			return e
		}
		candidate := NotebookCreationIntent{ID: id, Identity: identity, Owner: p.UserID, TenantID: p.TenantID, SpaceID: space, Digest: digest, Title: q.Title, DirectoryPath: directory, Path: target, ParentFileID: q.ParentFileID, Icon: q.Icon, CreatedAt: time.Now().UTC()}
		if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&candidate).Error; e != nil {
			return code("database_unavailable")
		}
		if e := tx.Where("identity = ?", identity).First(&row).Error; e != nil {
			return code("database_unavailable")
		}
		if row.Digest != digest {
			return code("idempotency_conflict")
		}
		return nil
	})
	if e != nil {
		return empty, e
	}
	if existing, e := s.notebookCreationView(ctx, p, row); e != nil {
		return empty, e
	} else if existing.Status == "completed" {
		return existing, nil
	}
	// Escape Markdown punctuation; a title cannot introduce executable content.
	title := strings.NewReplacer("\\", "\\\\", "#", "\\#", "*", "\\*", "_", "\\_", "[", "\\[", "]", "\\]", "<", "&lt;", ">", "&gt;", "`", "\\`").Replace(row.Title)
	body := "# " + title + "\n\n"
	op, e := s.CreateUpload(p, UploadRequest{SpaceID: space, Path: row.Path, Size: int64(len(body)), SHA256: Hash(body)}, "notebook:"+row.ID)
	if e != nil {
		return empty, e
	}
	if op.Status != "completed" {
		if _, e = s.PutPart(p, op.ID, 0, strings.NewReader(body)); e != nil {
			if current, re := s.operation(p, op.ID, "read"); re != nil || current.Status != "completed" {
				return empty, e
			}
		}
		op, e = s.Complete(ctx, p, op.ID)
		if e != nil {
			return empty, e
		}
	}
	_, e = s.PatchNotebookPage(ctx, p, op.FileID, contract.NotebookPagePatch{ParentFileID: row.ParentFileID, Icon: row.Icon}, "notebook-meta:"+row.ID)
	if e != nil {
		return empty, e
	}
	return s.notebookCreationView(ctx, p, row)
}
