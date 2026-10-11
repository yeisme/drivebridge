package service

import (
	"bytes"
	"gorm.io/gorm"
	"io"
	"net/http"
	"time"
)

func (s *Service) expireRequests() error {
	var expired []AccessRequest
	now := time.Now().UTC()
	if s.DB.Where("(status = ? AND expires <= ?) OR (status = ? AND grant_until <= ?)", "pending", now, "approved", now).Find(&expired).Error != nil {
		return code("database_unavailable")
	}
	for _, a := range expired {
		var sp Space
		s.DB.First(&sp, "id = ?", a.SpaceID)
		if err := s.DB.Transaction(func(tx *gorm.DB) error {
			r := tx.Model(&AccessRequest{}).Where("id = ? AND status = ?", a.ID, a.Status).Update("status", "expired")
			if r.Error != nil {
				return r.Error
			}
			if r.RowsAffected == 0 {
				return nil
			}
			return s.event(tx, Principal{UserID: a.Requester, TenantID: a.TenantID}, sp, "request.expired", a.ID, a.ID, "expired", a.Requester, a.Approver)
		}); err != nil {
			return err
		}
	}
	return nil
}
func (s *Service) requestChange(w http.ResponseWriter, r *http.Request, p Principal, action string, q governanceInput) error {
	if err := s.expireRequests(); err != nil {
		return err
	}
	var a AccessRequest
	var sp Space
	if action == "requests" || action == "emergency" {
		if e := teamWritable(s.DB, q.SpaceID); e != nil {
			return e
		}
		if s.DB.First(&sp, "id = ? AND tenant_id = ?", q.SpaceID, p.TenantID).Error != nil || s.scopeRole(p, sp) == "" {
			return code("forbidden")
		}
		if len(q.Reason) < 3 || len(q.Reason) > 1000 {
			return code("invalid_reason")
		}
		if q.Prefix != "" {
			if _, e := cleanPath(q.Prefix); e != nil {
				return e
			}
		}
		a = AccessRequest{ID: ID(), TenantID: p.TenantID, SpaceID: sp.ID, Requester: p.UserID, Reason: q.Reason, Kind: q.Kind, Action: q.Action, Prefix: q.Prefix, Status: "pending", Hours: q.Hours, Created: time.Now().UTC(), Expires: time.Now().UTC().Add(24 * time.Hour), Items: q.Items}
		if action == "emergency" {
			if !s.platformAdmin(p.UserID) || s.verifyProof(p, q.Proof) != nil {
				return code("reauthentication_required")
			}
			if q.Action != "write" && q.Action != "delete" && q.Action != "share" && q.Action != "all" {
				return code("invalid_action")
			}
			a.Kind, a.Status = "emergency", "approved"
			a.GrantUntil = time.Now().UTC().Add(15 * time.Minute)
		} else if a.Kind == "range" {
			if a.Action != "write" {
				return code("invalid_action")
			}
			if a.Hours == 0 {
				a.Hours = 1
			}
			if a.Hours < 1 || a.Hours > 8 {
				return code("invalid_duration")
			}
			a.Items = nil
		} else if a.Kind == "exact" {
			if len(a.Items) == 0 || len(a.Items) > 100 {
				return code("invalid_selection")
			}
			seen := map[string]bool{}
			for i := range a.Items {
				it := &a.Items[i]
				var f File
				if s.DB.First(&f, "id = ? AND space_id = ? AND version_id = ?", it.FileID, sp.ID, it.VersionID).Error != nil || seen[it.FileID] {
					return code("version_conflict")
				}
				seen[it.FileID] = true
				if it.Action != "trash" && it.Action != "restore" && it.Action != "purge" && it.Action != "move" && it.Action != "overwrite" && it.Action != "share" {
					return code("invalid_action")
				}
				if it.Action == "move" {
					if _, e := cleanPath(it.Path); e != nil {
						return e
					}
				}
				if it.Action == "overwrite" {
					var source Version
					if s.DB.First(&source, "id = ? AND file_id = ? AND owner = ?", it.SourceVersionID, it.SourceFileID, sp.Owner).Error != nil {
						return code("not_found")
					}
				}
				it.Status, it.Error, it.ResultID = "pending", "", ""
				if it.Action != "overwrite" {
					it.SourceFileID, it.SourceVersionID = "", ""
				}
				if it.Action != "move" {
					it.Path = ""
				}
			}
		} else {
			return code("invalid_request_kind")
		}
		event := "request.created"
		if action == "emergency" {
			event = "emergency.started"
		}
		reviewers := []string{p.UserID}
		if sp.TeamID != "" {
			var ms []Membership
			s.DB.Where("tenant_id = ? AND role = ? AND status = ?", p.TenantID, "admin", "active").Find(&ms)
			for _, m := range ms {
				reviewers = append(reviewers, m.UserID)
			}
			var ps []PlatformRole
			s.DB.Where("suspended = ?", false).Find(&ps)
			for _, v := range ps {
				reviewers = append(reviewers, v.UserID)
			}
		}
		if err := s.DB.Transaction(func(tx *gorm.DB) error {
			if e := tx.Create(&a).Error; e != nil {
				return e
			}
			return s.event(tx, p, sp, event, a.ID, a.ID, a.Status, reviewers...)
		}); err != nil {
			return err
		}
		respond(w, a)
		return nil
	}
	if s.DB.First(&a, "id = ? AND tenant_id = ?", q.ID, p.TenantID).Error != nil || s.DB.First(&sp, "id = ?", a.SpaceID).Error != nil {
		return code("not_found")
	}
	can := s.canApprove(p, sp, a.Requester)
	switch action {
	case "approve", "reject":
		if !can || a.Status != "pending" {
			return code("forbidden")
		}
		a.Approver = p.UserID
		if action == "approve" {
			a.Status = "approved"
			hours := 1
			if a.Kind == "range" {
				hours = a.Hours
			}
			a.GrantUntil = time.Now().UTC().Add(time.Duration(hours) * time.Hour)
		} else {
			a.Status = "rejected"
		}
	case "revoke":
		if (p.UserID != a.Requester && !can && !s.platformAdmin(p.UserID)) || (a.Status != "pending" && a.Status != "approved" && a.Status != "executing") {
			return code("forbidden")
		}
		a.Status = "revoked"
	case "execute":
		if a.Requester != p.UserID || a.Kind != "exact" || a.Status != "approved" || !time.Now().Before(a.GrantUntil) || !s.canApprove(Principal{UserID: a.Approver, TenantID: a.TenantID}, sp, a.Requester) || s.scopeRole(p, sp) == "" {
			return code("forbidden")
		}
		return s.executeRequest(w, r, p, sp, a)
	default:
		return code("invalid_action")
	}
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Save(&a).Error; e != nil {
			return e
		}
		return s.event(tx, p, sp, "request."+a.Status, a.ID, a.ID, a.Status, a.Requester, a.Approver)
	}); err != nil {
		return err
	}
	respond(w, a)
	return nil
}
func actionPermission(action string) string {
	switch action {
	case "trash", "restore", "purge":
		return "delete"
	case "share":
		return "share"
	default:
		return "write"
	}
}
func (s *Service) executeRequest(w http.ResponseWriter, r *http.Request, p Principal, sp Space, a AccessRequest) error {
	// Claim once, before any effect. A crash leaves an explicit interrupted receipt.
	a.Status = "executing"
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&AccessRequest{}).Where("id = ? AND status = ?", a.ID, "approved").Update("status", "executing")
		if res.Error != nil || res.RowsAffected != 1 {
			return code("operation_closed")
		}
		return s.event(tx, p, sp, "request.executing", a.ID, a.ID, "intent", a.Requester, a.Approver)
	}); err != nil {
		return err
	}
	// Release decision serialization during I/O so revocation can take effect.
	s.governanceMu.Unlock()
	defer s.governanceMu.Lock()
	p.DataOwner, p.ScopeSpace, p.ExecutionID = sp.Owner, sp.ID, a.ID
	failed := false
	for i := range a.Items {
		it := &a.Items[i]
		var f File
		err := error(nil)
		var current AccessRequest
		if s.DB.First(&current, "id = ?", a.ID).Error != nil || current.Status != "executing" {
			err = code("operation_closed")
		}
		if err != nil {
		} else if !time.Now().Before(a.GrantUntil) || s.scopeRole(p, sp) == "" {
			err = code("forbidden")
		} else if s.DB.First(&f, "id = ? AND space_id = ? AND version_id = ?", it.FileID, sp.ID, it.VersionID).Error != nil {
			err = code("version_conflict")
		}
		if err == nil {
			it.Status = "running"
			res := s.DB.Model(&AccessRequest{}).Where("id = ? AND status = ?", a.ID, "executing").Select("Items").Updates(&a)
			if res.Error != nil {
				return code("audit_unavailable")
			}
			if res.RowsAffected != 1 {
				return code("operation_closed")
			}
			switch it.Action {
			case "trash":
				err = s.Trash(p, f.ID, false)
			case "restore":
				err = s.Trash(p, f.ID, true)
			case "purge":
				err = s.Purge(r.Context(), p, f.ID)
			case "move":
				s.mu.Lock()
				err = pathAvailable(s.DB, sp.Owner, sp.ID, it.Path, f.ID)
				if err == nil {
					res := s.DB.Model(&File{}).Where("id = ? AND version_id = ? AND path = ?", f.ID, it.VersionID, f.Path).Updates(map[string]any{"path": it.Path, "updated": time.Now().UTC()})
					err = res.Error
					if res.RowsAffected != 1 {
						err = code("version_conflict")
					}
				}
				s.mu.Unlock()
			case "share":
				var share Share
				share, _, err = s.createShare(p, ShareRequest{Name: "Approved share", SpaceID: sp.ID, Path: f.Path, Mode: "fixed", Audience: "link", Download: true, Days: 1, Items: []ShareItem{{FileID: f.ID, VersionID: it.VersionID}}})
				it.ResultID = share.ID
			case "overwrite":
				var source Version
				var body io.ReadCloser
				_, source, body, err = s.Download(r.Context(), p, it.SourceFileID, it.SourceVersionID)
				if err == nil {
					var op Operation
					op, err = s.CreateUpload(p, UploadRequest{SpaceID: sp.ID, Path: f.Path, FileID: f.ID, ExpectedVersion: it.VersionID, Size: source.Size, SHA256: source.SHA256}, "approval:"+a.ID+":"+f.ID)
					if err == nil {
						for index := 0; int64(index)*PartSize < source.Size; index++ {
							part, e := io.ReadAll(io.LimitReader(body, PartSize))
							if e != nil {
								err = e
								break
							}
							if _, err = s.PutPart(p, op.ID, index, bytes.NewReader(part)); err != nil {
								break
							}
						}
						if err == nil {
							_, err = s.Complete(r.Context(), p, op.ID)
						}
					}
					body.Close()
				}
			}
		}
		if err != nil {
			it.Status, it.Error = "failed", safeCode(err)
			failed = true
		} else {
			it.Status = "succeeded"
		}
		if e := s.DB.Transaction(func(tx *gorm.DB) error {
			if e := tx.Model(&AccessRequest{}).Where("id = ?", a.ID).Select("Items").Updates(&a).Error; e != nil {
				return e
			}
			return s.event(tx, p, sp, "request.item", it.FileID, a.ID, it.Status, a.Requester, a.Approver)
		}); e != nil {
			return e
		}
	}
	a.Status = "completed"
	if failed {
		a.Status = "partial"
	}
	if e := s.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&AccessRequest{}).Where("id = ? AND status = ?", a.ID, "executing").Update("status", a.Status)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			var current AccessRequest
			if e := tx.First(&current, "id = ?", a.ID).Error; e != nil {
				return e
			}
			a.Status = current.Status
		}
		return s.event(tx, p, sp, "request.result", a.ID, a.ID, a.Status, a.Requester, a.Approver)
	}); e != nil {
		return e
	}
	respond(w, a)
	return nil
}
