package service

import (
	"encoding/json"
	"errors"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// This file implements the version-discussion application service: file
// threads anchored to versions, plain-text messages with optimistic
// revision checks, private notes scoped to their author, and the
// preview-only note publication. Resolving a thread never touches domain
// review state.

// nextDiscussionSeq allocates the next stable sequence for one file inside
// the caller's transaction.
func nextDiscussionSeq(tx *gorm.DB, fileID string) (int64, error) {
	var row DiscussionRevision
	if e := tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&DiscussionRevision{FileID: fileID, Next: 1}).Error; e != nil {
		return 0, e
	}
	if e := tx.Where("file_id = ?", fileID).First(&row).Error; e != nil {
		return 0, e
	}
	if e := tx.Model(&DiscussionRevision{}).Where("file_id = ?", fileID).Update("next", row.Next+1).Error; e != nil {
		return 0, e
	}
	return row.Next, nil
}

// commentWriteAllowed separates reading from speaking: the caller needs
// ordinary write scope on the file's path for comment write.
func (s *Service) commentWriteAllowed(p Principal, f File) error {
	if p.Agent != nil {
		return s.allow(p, f.SpaceID, f.Path, "write")
	}
	return s.allow(p, f.SpaceID, f.Path, "write")
}

// anchorView reports the anchor state of one thread: a cleaned-up anchor
// keeps the discussion readable with an explicit unavailable marker.
func (s *Service) anchorView(t FileThread) map[string]any {
	anchor := map[string]any{"version_id": t.VersionID, "kind": "file"}
	if t.VersionID == "" {
		anchor["kind"] = "file"
		return anchor
	}
	var count int64
	s.DB.Model(&Version{}).Where("id = ? AND file_id = ?", t.VersionID, t.FileID).Count(&count)
	if count == 0 {
		anchor["version_unavailable"] = true
	}
	return anchor
}

func threadView(t FileThread) map[string]any {
	return map[string]any{"thread_id": t.ID, "file_id": t.FileID, "version_id": t.VersionID, "seq": t.Seq, "state": t.State, "revision": t.Revision, "author_id": t.AuthorID, "author_kind": t.AuthorKind, "created_at": t.CreatedAt.UTC().Format(time.RFC3339)}
}

func messageView(m ThreadMessage) map[string]any {
	out := map[string]any{"message_id": m.ID, "thread_id": m.ThreadID, "seq": m.Seq, "author_id": m.AuthorID, "author_kind": m.AuthorKind, "revision": m.Revision, "created_at": m.CreatedAt.UTC().Format(time.RFC3339)}
	if m.Deleted {
		out["deleted"] = true
		out["body"] = ""
	} else {
		out["body"] = m.Body
	}
	if m.EditedAt != nil {
		out["edited_at"] = m.EditedAt.UTC().Format(time.RFC3339)
	}
	return out
}

// CreateFileThread opens one thread on a file (default file-level anchor,
// or an explicit version anchor) under a durable idempotency key.
func (s *Service) CreateFileThread(p Principal, fileID, versionID, body, key string) (map[string]any, error) {
	if key == "" || len(key) > 200 || !threadBodyValid(body) {
		return nil, code("invalid_request")
	}
	f, e := s.file(p, fileID, "read")
	if e != nil {
		return nil, e
	}
	if e := s.commentWriteAllowed(p, f); e != nil {
		return nil, code("comment_write_forbidden")
	}
	if versionID != "" {
		var count int64
		s.DB.Model(&Version{}).Where("id = ? AND file_id = ?", versionID, f.ID).Count(&count)
		if count == 0 {
			return nil, code("anchor_version_unavailable")
		}
	}
	authorKind := "user"
	if p.Agent != nil {
		authorKind = "agent"
	}
	fpBody, _ := json.Marshal(map[string]string{"file_id": f.ID, "version_id": versionID, "body": body})
	fingerprint := Hash(string(fpBody))
	var out FileThread
	e = s.DB.Transaction(func(tx *gorm.DB) error {
		var existing Operation
		// Threads reuse the operation idempotency space: the same key with
		// the same fingerprint replays, a different fingerprint conflicts.
		if err := tx.First(&existing, "owner = ? AND key = ? AND kind = ?", p.dataOwner(), key, "file_thread").Error; err == nil {
			if existing.Fingerprint != fingerprint {
				return code("idempotency_conflict")
			}
			return tx.First(&out, "id = ?", existing.ID).Error
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		seq, err := nextDiscussionSeq(tx, f.ID)
		if err != nil {
			return code("database_unavailable")
		}
		now := time.Now().UTC()
		out = FileThread{ID: ID(), Owner: p.dataOwner(), TenantID: p.TenantID, FileID: f.ID, VersionID: versionID, Seq: seq, AuthorID: p.UserID, AuthorKind: authorKind, State: "open", Revision: 1, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&out).Error; err != nil {
			return code("database_unavailable")
		}
		anchor := Operation{ActorID: p.UserID, ID: out.ID, Owner: p.dataOwner(), Key: key, Fingerprint: fingerprint, Kind: "file_thread", SpaceID: f.SpaceID, Status: "completed", Updated: now}
		if err := tx.Create(&anchor).Error; err != nil {
			return code("database_unavailable")
		}
		msg := ThreadMessage{ID: ID(), ThreadID: out.ID, Seq: 1, AuthorID: p.UserID, AuthorKind: authorKind, Body: body, Revision: 1, CreatedAt: now, UpdatedAt: now}
		return tx.Create(&msg).Error
	})
	if e != nil {
		return nil, e
	}
	view := threadView(out)
	view["anchor"] = s.anchorView(out)
	return view, nil
}

// thread loads one thread within the caller's identity scope.
func (s *Service) thread(p Principal, id string) (FileThread, error) {
	var t FileThread
	if s.DB.First(&t, "id = ? AND owner = ?", id, p.dataOwner()).Error != nil {
		return t, code("not_found")
	}
	return t, nil
}

// ListFileThreads returns one keyset page of threads with their messages.
// Every page re-authorizes the file read; comments stay visible even when
// the anchored version is gone (marked, never re-anchored).
func (s *Service) ListFileThreads(p Principal, fileID, cursor string, limit int) (map[string]any, error) {
	f, e := s.file(p, fileID, "read")
	if e != nil {
		return nil, e
	}
	if limit <= 0 {
		limit = 50
	}
	if limit > 200 {
		limit = 200
	}
	query := s.DB.Where("file_id = ? AND owner = ?", f.ID, p.dataOwner())
	if cursor != "" {
		var after FileThread
		if s.DB.First(&after, "id = ? AND owner = ?", cursor, p.dataOwner()).Error != nil {
			return nil, code("invalid_request")
		}
		query = query.Where("seq > ?", after.Seq)
	}
	var threads []FileThread
	if e := query.Order("seq").Limit(limit + 1).Find(&threads).Error; e != nil {
		return nil, code("database_unavailable")
	}
	next := ""
	if len(threads) > limit {
		next = threads[limit-1].ID
		threads = threads[:limit]
	}
	out := []map[string]any{}
	for _, t := range threads {
		view := threadView(t)
		view["anchor"] = s.anchorView(t)
		var messages []ThreadMessage
		s.DB.Where("thread_id = ?", t.ID).Order("seq").Find(&messages)
		msgs := []map[string]any{}
		for _, m := range messages {
			msgs = append(msgs, messageView(m))
		}
		view["messages"] = msgs
		out = append(out, view)
	}
	return map[string]any{"file_id": f.ID, "threads": out, "next_cursor": next}, nil
}

// ReplyFileThread appends one message under a durable idempotency key.
func (s *Service) ReplyFileThread(p Principal, threadID, body, key string) (map[string]any, error) {
	if key == "" || len(key) > 200 || !threadBodyValid(body) {
		return nil, code("invalid_request")
	}
	t, e := s.thread(p, threadID)
	if e != nil {
		return nil, e
	}
	f, e := s.file(p, t.FileID, "read")
	if e != nil {
		return nil, e
	}
	if e := s.commentWriteAllowed(p, f); e != nil {
		return nil, code("comment_write_forbidden")
	}
	authorKind := "user"
	if p.Agent != nil {
		authorKind = "agent"
	}
	fpBody, _ := json.Marshal(map[string]string{"thread_id": t.ID, "body": body})
	fingerprint := Hash(string(fpBody))
	var out ThreadMessage
	e = s.DB.Transaction(func(tx *gorm.DB) error {
		var existing Operation
		if err := tx.First(&existing, "owner = ? AND key = ? AND kind = ?", p.dataOwner(), key, "thread_reply").Error; err == nil {
			if existing.Fingerprint != fingerprint {
				return code("idempotency_conflict")
			}
			return tx.First(&out, "id = ?", existing.ID).Error
		} else if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		seq, err := nextDiscussionSeq(tx, t.FileID)
		if err != nil {
			return code("database_unavailable")
		}
		now := time.Now().UTC()
		out = ThreadMessage{ID: ID(), ThreadID: t.ID, Seq: seq, AuthorID: p.UserID, AuthorKind: authorKind, Body: body, Revision: 1, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&out).Error; err != nil {
			return code("database_unavailable")
		}
		anchor := Operation{ActorID: p.UserID, ID: out.ID, Owner: p.dataOwner(), Key: key, Fingerprint: fingerprint, Kind: "thread_reply", SpaceID: f.SpaceID, Status: "completed", Updated: now}
		if err := tx.Create(&anchor).Error; err != nil {
			return code("database_unavailable")
		}
		return tx.Model(&FileThread{}).Where("id = ?", t.ID).Updates(map[string]any{"revision": gorm.Expr("revision + 1"), "updated_at": now}).Error
	})
	if e != nil {
		return nil, e
	}
	return messageView(out), nil
}

// message loads one message and its thread within the caller's scope.
func (s *Service) message(p Principal, id string) (ThreadMessage, FileThread, error) {
	var m ThreadMessage
	if s.DB.First(&m, "id = ?", id).Error != nil {
		return m, FileThread{}, code("not_found")
	}
	t, e := s.thread(p, m.ThreadID)
	if e != nil {
		return m, t, e
	}
	return m, t, nil
}

// EditThreadMessage replaces the caller's own message body under an
// optimistic revision check. A stale revision refuses with the server body
// untouched, so the client keeps its draft.
func (s *Service) EditThreadMessage(p Principal, messageID, body string, expectedRevision uint64) (map[string]any, error) {
	if !threadBodyValid(body) {
		return nil, code("invalid_request")
	}
	m, t, e := s.message(p, messageID)
	if e != nil {
		return nil, e
	}
	if m.Deleted {
		return nil, code("operation_closed")
	}
	// Only the author manages their own body.
	if m.AuthorID != p.UserID || (m.AuthorKind == "agent") != (p.Agent != nil) {
		return nil, code("comment_write_forbidden")
	}
	f, e := s.file(p, t.FileID, "read")
	if e != nil {
		return nil, e
	}
	if e := s.commentWriteAllowed(p, f); e != nil {
		return nil, code("comment_write_forbidden")
	}
	var out ThreadMessage
	e = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&out, "id = ?", m.ID).Error; err != nil {
			return code("not_found")
		}
		if out.Revision != expectedRevision {
			return code("revision_conflict")
		}
		now := time.Now().UTC()
		out.Body = body
		out.Revision++
		out.EditedAt = &now
		out.UpdatedAt = now
		if err := tx.Save(&out).Error; err != nil {
			return code("database_unavailable")
		}
		return tx.Model(&FileThread{}).Where("id = ?", t.ID).Updates(map[string]any{"revision": gorm.Expr("revision + 1"), "updated_at": now}).Error
	})
	if e != nil {
		return nil, e
	}
	return messageView(out), nil
}

// DeleteThreadMessage replaces the caller's own message with an audit
// placeholder under an optimistic revision check.
func (s *Service) DeleteThreadMessage(p Principal, messageID string, expectedRevision uint64) (map[string]any, error) {
	m, t, e := s.message(p, messageID)
	if e != nil {
		return nil, e
	}
	if m.Deleted {
		return messageView(m), nil
	}
	if m.AuthorID != p.UserID || (m.AuthorKind == "agent") != (p.Agent != nil) {
		return nil, code("comment_write_forbidden")
	}
	f, e := s.file(p, t.FileID, "read")
	if e != nil {
		return nil, e
	}
	if e := s.commentWriteAllowed(p, f); e != nil {
		return nil, code("comment_write_forbidden")
	}
	var out ThreadMessage
	e = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&out, "id = ?", m.ID).Error; err != nil {
			return code("not_found")
		}
		if out.Deleted {
			return nil
		}
		if out.Revision != expectedRevision {
			return code("revision_conflict")
		}
		now := time.Now().UTC()
		out.Deleted = true
		out.DeletedBy = p.UserID
		out.Revision++
		out.UpdatedAt = now
		if err := tx.Save(&out).Error; err != nil {
			return code("database_unavailable")
		}
		return tx.Model(&FileThread{}).Where("id = ?", t.ID).Updates(map[string]any{"revision": gorm.Expr("revision + 1"), "updated_at": now}).Error
	})
	if e != nil {
		return nil, e
	}
	return messageView(out), nil
}

// SetThreadResolution resolves or reopens a thread. This only changes the
// thread state: domain review facts (ReviewPending, governance) are a
// different fact and never move here.
func (s *Service) SetThreadResolution(p Principal, threadID string, resolve bool) (map[string]any, error) {
	t, e := s.thread(p, threadID)
	if e != nil {
		return nil, e
	}
	f, e := s.file(p, t.FileID, "read")
	if e != nil {
		return nil, e
	}
	if e := s.commentWriteAllowed(p, f); e != nil {
		return nil, code("comment_write_forbidden")
	}
	var out FileThread
	e = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.First(&out, "id = ?", t.ID).Error; err != nil {
			return code("not_found")
		}
		now := time.Now().UTC()
		if resolve {
			if out.State == "resolved" {
				return nil
			}
			out.State = "resolved"
			out.ResolvedBy = p.UserID
			out.ResolvedAt = &now
		} else {
			if out.State != "resolved" {
				return nil
			}
			out.State = "open"
			out.ResolvedBy = ""
			out.ResolvedAt = nil
		}
		out.Revision++
		out.UpdatedAt = now
		return tx.Save(&out).Error
	})
	if e != nil {
		return nil, e
	}
	view := threadView(out)
	view["anchor"] = s.anchorView(out)
	return view, nil
}

// PrivateFileNote reads or writes the caller's private note on one file.
// Only the author row is ever returned, and only while the author still
// holds read permission on the file.
func (s *Service) PrivateFileNote(p Principal, fileID, body string, expectedRevision uint64, set bool) (map[string]any, error) {
	f, e := s.file(p, fileID, "read")
	if e != nil {
		return nil, e
	}
	if set {
		if !threadBodyValid(body) {
			return nil, code("invalid_request")
		}
		if e := s.commentWriteAllowed(p, f); e != nil {
			return nil, code("comment_write_forbidden")
		}
		var out PrivateNote
		e = s.DB.Transaction(func(tx *gorm.DB) error {
			if err := tx.First(&out, "file_id = ? AND author_id = ?", f.ID, p.UserID).Error; err == nil {
				if out.Revision != expectedRevision {
					return code("revision_conflict")
				}
				out.Body = body
				out.Revision++
				out.UpdatedAt = time.Now().UTC()
				return tx.Save(&out).Error
			} else if !errors.Is(err, gorm.ErrRecordNotFound) {
				return code("database_unavailable")
			}
			if expectedRevision != 0 {
				return code("revision_conflict")
			}
			out = PrivateNote{FileID: f.ID, AuthorID: p.UserID, Body: body, Revision: 1, UpdatedAt: time.Now().UTC()}
			return tx.Create(&out).Error
		})
		if e != nil {
			return nil, e
		}
		return map[string]any{"file_id": out.FileID, "revision": out.Revision, "updated_at": out.UpdatedAt.UTC().Format(time.RFC3339)}, nil
	}
	var note PrivateNote
	if s.DB.First(&note, "file_id = ? AND author_id = ?", f.ID, p.UserID).Error != nil {
		return nil, code("not_found")
	}
	return map[string]any{"file_id": note.FileID, "body": note.Body, "revision": note.Revision, "updated_at": note.UpdatedAt.UTC().Format(time.RFC3339)}, nil
}

// PreviewNotePublication previews what publishing the private note would
// create. It writes nothing: the actual publication is an explicit
// file_thread_create with the same body.
func (s *Service) PreviewNotePublication(p Principal, fileID string) (map[string]any, error) {
	f, e := s.file(p, fileID, "read")
	if e != nil {
		return nil, e
	}
	if e := s.commentWriteAllowed(p, f); e != nil {
		return nil, code("comment_write_forbidden")
	}
	var note PrivateNote
	if s.DB.First(&note, "file_id = ? AND author_id = ?", f.ID, p.UserID).Error != nil {
		return nil, code("not_found")
	}
	return map[string]any{"file_id": f.ID, "target": "team_thread", "body_preview": note.Body, "note_revision": note.Revision, "note": "preview only; publish explicitly with file_thread_create; the private note stays private"}, nil
}
