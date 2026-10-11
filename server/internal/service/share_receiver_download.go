package service

import (
	"errors"
	"net/http"
	"time"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Service) admitShareDownload(p Principal, id, token string, file File) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var v Share
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&v, "id = ?", id).Error != nil || !shareActive(v) {
			return code("share_unavailable")
		}
		session, _, err := s.shareReceiverSession(tx, v, token)
		if err != nil {
			return err
		}
		if session.Kind == "device" && (p.Agent != nil || p.ServiceIdentity != nil) {
			return code("forbidden")
		}
		if v.Audience == "users" && (p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || !containsRecipient(v, p.UserID) || session.UserID != "" && session.UserID != p.UserID) {
			return code("forbidden")
		}
		var previous ShareDownload
		err = tx.First(&previous, "share_id = ? AND session_id = ? AND file_id = ? AND version_id = ?", id, session.ID, file.ID, file.VersionID).Error
		// An admitted stream can resume after exhausting the quota; new tuples
		// cannot. Checking the durable tuple before the limit preserves restarts.
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		if v.MaxDownloads != nil && v.DownloadsUsed >= *v.MaxDownloads {
			return code("share_unavailable")
		}
		row := ShareDownload{ShareID: id, SessionID: session.ID, FileID: file.ID, VersionID: file.VersionID, Created: time.Now().UTC()}
		if tx.Create(&row).Error != nil {
			return code("database_unavailable")
		}
		if tx.Model(&v).Update("downloads_used", gorm.Expr("downloads_used + 1")).Error != nil {
			return code("database_unavailable")
		}
		return nil
	})
}

// Intercept only the existing media writer's successful stream establishment.
// Storage failures, rejected ranges and HEAD requests never consume quota;
// all range parsing, encryption and transport remain in mediaContent.
type shareDownloadWriter struct {
	http.ResponseWriter
	admit   func() error
	err     error
	started bool
	bytes   int64
	status  int
}

func (w *shareDownloadWriter) WriteHeader(status int) {
	if w.started || w.err != nil || w.status != 0 {
		return
	}
	w.status = status
	if status == http.StatusOK || status == http.StatusPartialContent {
		// A successful header does not prove storage produced any bytes.
		// Hold it until Write, or explicit completion of a verified empty file.
		return
	}
	w.started = true
	w.ResponseWriter.WriteHeader(status)
}

func (w *shareDownloadWriter) establish() {
	if w.started || w.err != nil {
		return
	}
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.err = w.admit(); w.err != nil {
		return
	}
	w.started = true
	w.ResponseWriter.WriteHeader(w.status)
}

func (w *shareDownloadWriter) Write(body []byte) (int, error) {
	if len(body) == 0 {
		return 0, w.err
	}
	w.establish()
	if w.err != nil {
		return 0, w.err
	}
	n, err := w.ResponseWriter.Write(body)
	w.bytes += int64(n)
	return n, err
}

func (s *Service) serveShareReceiverContent(w http.ResponseWriter, r *http.Request, v Share, file File, attachment bool) (err error) {
	if r.Method == "HEAD" {
		return s.mediaContent(w, r, s.sharePrincipal(v), file.ID, file.VersionID, attachment)
	}
	cookie, err := r.Cookie(shareCookie(v.ID))
	if err != nil {
		return code("unauthorized")
	}
	p, err := s.optionalSharePrincipal(r)
	if err != nil {
		return err
	}
	writer := &shareDownloadWriter{ResponseWriter: w, admit: func() error { return s.admitShareDownload(p, v.ID, cookie.Value, file) }}
	defer func() {
		if recovered := recover(); recovered != nil {
			// Only an admission failure before headers can replace the media
			// stream's abort with the normal API error envelope.
			if recovered != http.ErrAbortHandler || writer.err == nil {
				panic(recovered)
			}
		}
		if writer.err != nil {
			for _, key := range []string{"Content-Type", "Content-Length", "Content-Disposition", "Content-Range", "Accept-Ranges", "Content-Security-Policy"} {
				w.Header().Del(key)
			}
			err = writer.err
		}
	}()
	err = s.mediaContent(writer, r, s.sharePrincipal(v), file.ID, file.VersionID, attachment)
	if err == nil && file.Size == 0 && !writer.started && writer.err == nil {
		// mediaContent has already probed the zero-length object for EOF.
		writer.establish()
	}
	if writer.err != nil {
		return writer.err
	}
	if err == nil && writer.bytes > 0 {
		// Bytes already sent cannot be replaced with a JSON failure. Persist
		// only truthful observations and retain the existing abort behavior.
		if observationErr := s.observeShareReceiverDownload(p, v, cookie.Value, file); observationErr != nil {
			panic(http.ErrAbortHandler)
		}
	}
	return err
}

// Published fixed-delivery is the only receipt destination. A missing or
// unpublished manifest is not a delivery, and never gets a fabricated receipt.
func (s *Service) observeShareReceiverDownload(p Principal, v Share, token string, file File) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var manifest DeliveryManifest
		err := tx.Where("share_id = ? AND phase = ?", v.ID, "published").Order("revision DESC").First(&manifest).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return code("database_unavailable")
		}
		var item DeliveryManifestItem
		err = tx.First(&item, "manifest_id = ? AND file_id = ? AND version_id = ?", manifest.ID, file.ID, file.VersionID).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil
		}
		if err != nil {
			return code("database_unavailable")
		}
		session, receiver, err := s.shareReceiverSession(tx, v, token)
		if err != nil {
			return err
		}
		ref, kind := "", "bytes_served"
		if receiver == "device" {
			ref, kind = session.DeviceFingerprint, "receiver_verified"
		}
		if v.Audience == "users" {
			receiver, ref, kind = "user", p.UserID, "receiver_verified"
		}
		// A resumed range reports the same observed fact, not a new confirmation.
		var count int64
		if tx.Model(&DeliveryReceipt{}).Where("manifest_id = ? AND receiver_kind = ? AND receiver_ref = ? AND kind = ?", manifest.ID, receiver, ref, kind).Count(&count).Error != nil {
			return code("database_unavailable")
		}
		if count != 0 {
			return nil
		}
		now := time.Now().UTC()
		receipt := DeliveryReceipt{ID: ID(), ManifestID: manifest.ID, ManifestRevision: manifest.Revision, Owner: manifest.Owner, ReceiverKind: receiver, ReceiverRef: ref, Kind: kind, ObservedAt: now, CreatedAt: now}
		if tx.Create(&receipt).Error != nil {
			return code("database_unavailable")
		}
		return nil
	})
}
