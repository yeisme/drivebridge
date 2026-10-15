package service

import (
	"context"
	"gorm.io/gorm"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"net/http"
)

type temporaryAccess interface {
	Access(context.Context, string, time.Duration) (string, error)
}

func (st *s3Store) Access(ctx context.Context, key string, ttl time.Duration) (string, error) {
	out, e := s3.NewPresignClient(st.client).PresignGetObject(ctx, &s3.GetObjectInput{Bucket: aws.String(st.cfg.Bucket), Key: aws.String(st.key(key))}, func(o *s3.PresignOptions) { o.Expires = ttl })
	if e != nil {
		return "", code("access_unavailable")
	}
	return out.URL, nil
}

const accessTTL = 5 * time.Minute

func (s *Service) accessHandler(w http.ResponseWriter, r *http.Request, p Principal) error {
	var q struct {
		VersionID string `json:"version_id"`
	}
	if e := decode(r, &q); e != nil {
		return e
	}
	f, e := s.file(p, r.PathValue("id"), "read")
	if e != nil {
		return e
	}
	if f.Trashed {
		return code("file_trashed")
	}
	var v Version
	if s.DB.First(&v, "id = ? AND owner = ? AND file_id = ?", q.VersionID, p.dataOwner(), f.ID).Error != nil {
		return code("not_found")
	}
	sp, _ := s.spaceFor(p, f.SpaceID)
	// All authenticated downloads use a revocable server-side capability. A
	// storage-provider URL must never become an untracked bearer credential.
	token := ID()
	grant := AccessGrant{ActorID: p.UserID, TenantID: p.TenantID, Hash: Hash(token), Owner: p.dataOwner(), FileID: f.ID, VersionID: v.ID, Expires: time.Now().UTC().Add(accessTTL)}
	s.DB.Delete(&AccessGrant{}, "expires < ?", time.Now().UTC())
	if err := s.DB.Transaction(func(tx *gorm.DB) error {
		if sp.TeamID != "" {
			if e := teamWritable(tx, sp.ID); e != nil {
				return e
			}
		}
		return tx.Create(&grant).Error
	}); err != nil {
		if safeCode(err) == "team_archived" {
			return err
		}
		return code("database_unavailable")
	}
	respond(w, map[string]any{"url": s.Config.Origin + "/api/v1/access/" + token, "expires_at": grant.Expires, "version_id": v.ID, "transport": "server_stream"})
	return nil
}

// accessStreamHandler serves the short-lived decryption stream granted by
// POST /api/v1/files/{id}/access on encrypted spaces. The random token is the
// bearer capability; scope and expiry match the presigned link it replaces.
func (s *Service) accessStreamHandler(w http.ResponseWriter, r *http.Request) {
	if !s.Config.Enabled {
		writeError(w, code("disabled"))
		return
	}
	var grant AccessGrant
	if s.DB.First(&grant, "hash = ?", Hash(r.PathValue("token"))).Error != nil || !time.Now().Before(grant.Expires) {
		writeError(w, code("unauthorized"))
		return
	}
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("Referrer-Policy", "no-referrer")
	p := Principal{AccessGrantHash: grant.Hash, ExternalRead: true, UserID: defaultString(grant.ActorID, grant.Owner), DataOwner: grant.Owner, TenantID: grant.TenantID}
	f, v, body, e := s.Download(r.Context(), p, grant.FileID, grant.VersionID)
	if e != nil {
		writeError(w, e)
		return
	}
	defer body.Close()
	e = s.sendContent(w, p, f, v, body)
	outcome := "success"
	if e != nil {
		outcome = safeCode(e)
	}
	s.audit(p, "files.access_stream", f.ID, outcome)
}

func (s *Service) spaceFor(p Principal, id string) (Space, bool) {
	var sp Space
	if s.DB.First(&sp, "id = ? AND owner = ?", id, p.dataOwner()).Error != nil {
		return sp, false
	}
	return sp, true
}
