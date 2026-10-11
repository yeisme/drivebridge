package service

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"

	"gorm.io/gorm/clause"
)

// HandoffBinding is an explicit, immutable receiver authorization. Disabling
// it blocks subsequent reads without deleting delivered copies or receipts.
type HandoffBinding struct {
	ID         string `json:"id" gorm:"primaryKey"`
	Owner      string `json:"-" gorm:"index"`
	AgentID    string `json:"agent_id"`
	Consumer   string `json:"consumer"`
	ProjectRef string `json:"project_ref"`
	Purpose    string `json:"purpose"`
	SpaceID    string `json:"space_id"`
	Prefix     string `json:"prefix"`
	Disabled   bool   `json:"disabled"`
}

type BindReferenceRequest struct {
	BindingID          string     `json:"binding_id,omitempty"`
	Ref                string     `json:"ref"`
	Consumer           string     `json:"consumer"`
	ProjectRef         string     `json:"project_ref"`
	Purpose            string     `json:"purpose"`
	ExpectedSHA256     string     `json:"expected_sha256"`
	ConsumerRequestRef string     `json:"consumer_request_ref,omitempty"`
	RequestExpiresAt   *time.Time `json:"request_expires_at,omitempty"`
}

func handoffActor(p Principal) string {
	if p.Agent != nil {
		return "agent:" + p.Agent.ID
	}
	return "user:" + p.UserID
}

func validHandoffPurpose(purpose string) bool {
	switch purpose {
	case "reference", "vault", "notes-attachments", "materials", "saveback", "owner-store":
		return true
	default:
		return false
	}
}

func bindingCoversFile(b HandoffBinding, f File) error {
	if b.Disabled || f.Trashed || b.SpaceID != f.SpaceID || !contains(b.Prefix, f.Path) {
		return code("forbidden")
	}
	return nil
}

func (s *Service) enforceBindingPrefix(p Principal, b HandoffBinding, fileID string) error {
	f, err := s.file(p, fileID, "read")
	if err != nil {
		return err
	}
	return bindingCoversFile(b, f)
}

func (s *Service) CreateHandoffBinding(p Principal, q HandoffBinding) (HandoffBinding, error) {
	if p.Agent != nil || p.UserID == "" || p.dataOwner() != p.UserID {
		return HandoffBinding{}, code("forbidden")
	}
	if q.AgentID == "" || q.SpaceID == "" || q.Consumer == "" || q.ProjectRef == "" || q.Purpose == "" || !validProvenance(q.Consumer, "") || !validProvenance(q.ProjectRef, "") || !validProvenance(q.Purpose, "") || !validHandoffPurpose(q.Purpose) {
		return HandoffBinding{}, code("invalid_binding")
	}
	if q.Prefix != "" {
		if _, err := cleanPath(q.Prefix); err != nil {
			return HandoffBinding{}, err
		}
	}
	var agent Agent
	if err := s.DB.Where(&Agent{ID: q.AgentID, Owner: p.dataOwner()}).First(&agent).Error; err != nil || agent.Revoked || agent.Archived || !time.Now().Before(agent.Expires) || agent.SpaceID != q.SpaceID || !contains(agent.Prefix, q.Prefix) {
		return HandoffBinding{}, code("forbidden")
	}
	if err := s.allow(Principal{UserID: p.UserID, Agent: &agent, TenantID: p.TenantID, DataOwner: p.DataOwner}, q.SpaceID, q.Prefix, "read"); err != nil {
		return HandoffBinding{}, err
	}
	var existing HandoffBinding
	if s.DB.Where("owner = ? AND agent_id = ? AND consumer = ? AND project_ref = ? AND purpose = ? AND space_id = ? AND prefix = ? AND disabled = ?", p.dataOwner(), q.AgentID, q.Consumer, q.ProjectRef, q.Purpose, q.SpaceID, q.Prefix, false).First(&existing).Error == nil {
		return existing, nil
	}
	q.ID, q.Owner, q.Disabled = ID(), p.dataOwner(), false
	if err := s.DB.Create(&q).Error; err != nil {
		return HandoffBinding{}, code("database_unavailable")
	}
	return q, nil
}

func (s *Service) BindReference(p Principal, q BindReferenceRequest, key string) (HandoffGrant, error) {
	var zero HandoffGrant
	if key == "" || len(key) > 256 || q.Consumer == "" || q.ProjectRef == "" || q.Purpose == "" || !validProvenance(q.Consumer, "") || !validProvenance(q.ProjectRef, "") || !validProvenance(q.Purpose, "") {
		return zero, code("invalid_handoff")
	}
	fileID, versionID, err := s.resolveRef(q.Ref)
	if err != nil {
		return zero, err
	}
	f, err := s.file(p, fileID, "read")
	if err != nil {
		return zero, err
	}
	if f.Trashed {
		return zero, code("file_trashed")
	}
	if s.governanceActive() {
		if err := s.allow(p, f.SpaceID, f.Path, "share"); err != nil {
			return zero, err
		}
	}
	var v Version
	if s.DB.Where(&Version{ID: versionID, FileID: fileID, Owner: p.dataOwner()}).First(&v).Error != nil {
		return zero, code("not_found")
	}
	if q.ExpectedSHA256 != "" && q.ExpectedSHA256 != v.SHA256 {
		return zero, code("content_mismatch")
	}
	var binding HandoffBinding
	if q.BindingID != "" {
		if s.DB.Where(&HandoffBinding{ID: q.BindingID, Owner: p.dataOwner()}).First(&binding).Error != nil || binding.Consumer != q.Consumer || binding.ProjectRef != q.ProjectRef || binding.Purpose != q.Purpose || (p.Agent != nil && p.Agent.ID != binding.AgentID) {
			return zero, code("forbidden")
		}
		if err := bindingCoversFile(binding, f); err != nil {
			return zero, err
		}
	}
	now := time.Now().UTC()
	if q.ConsumerRequestRef != "" && (!validProvenance(q.ConsumerRequestRef, "") || q.RequestExpiresAt == nil) {
		return zero, code("invalid_handoff")
	}
	if q.RequestExpiresAt != nil && (q.ConsumerRequestRef == "" || !now.Before(*q.RequestExpiresAt) || q.RequestExpiresAt.After(now.Add(7*24*time.Hour))) {
		return zero, code("invalid_handoff")
	}
	fingerprint, _ := json.Marshal(q)
	unique := Hash(p.dataOwner() + "\x00" + handoffActor(p) + "\x00" + key)
	grant := HandoffGrant{ID: "handoff:" + ID(), Owner: p.dataOwner(), FileID: fileID, VersionID: versionID, Consumer: q.Consumer, ProjectRef: q.ProjectRef, Purpose: q.Purpose, Digest: v.SHA256, Expires: now.Add(15 * time.Minute), BindingID: q.BindingID, Initiator: handoffActor(p), IdempotencyKey: &unique, Fingerprint: Hash(string(fingerprint)), RequestRef: q.ConsumerRequestRef, RequestExpires: q.RequestExpiresAt}
	if q.RequestExpiresAt != nil && grant.Expires.After(*q.RequestExpiresAt) {
		grant.Expires = *q.RequestExpiresAt
	}
	if err := s.DB.Clauses(clause.OnConflict{DoNothing: true}).Create(&grant).Error; err != nil {
		return zero, code("database_unavailable")
	}
	var stored HandoffGrant
	if s.DB.Where(&HandoffGrant{IdempotencyKey: &unique}).First(&stored).Error != nil {
		return zero, code("database_unavailable")
	}
	if stored.Fingerprint != grant.Fingerprint {
		return zero, code("idempotency_conflict")
	}
	return stored, nil
}

func (s *Service) GetHandoff(p Principal, id string) (HandoffGrant, error) {
	if id == "" || p.UserID == "" {
		return HandoffGrant{}, code("not_found")
	}
	var g HandoffGrant
	if s.DB.Where(&HandoffGrant{ID: id, Owner: p.dataOwner()}).First(&g).Error != nil {
		return g, code("not_found")
	}
	if g.BindingID == "" {
		if g.Initiator != handoffActor(p) {
			return HandoffGrant{}, code("forbidden")
		}
		return g, nil
	}
	var b HandoffBinding
	if s.DB.Where(&HandoffBinding{ID: g.BindingID, Owner: g.Owner}).First(&b).Error != nil || b.Disabled {
		return HandoffGrant{}, code("forbidden")
	}
	if g.Initiator != handoffActor(p) && (p.Agent == nil || p.Agent.ID != b.AgentID) {
		return HandoffGrant{}, code("forbidden")
	}
	if err := s.enforceBindingPrefix(p, b, g.FileID); err != nil {
		return HandoffGrant{}, err
	}
	return g, nil
}

func (s *Service) ChangeHandoff(p Principal, id, action string) (HandoffGrant, error) {
	g, err := s.GetHandoff(p, id)
	if err != nil {
		return g, err
	}
	if action == "cancel" {
		err = s.DB.Model(&HandoffGrant{}).Where(&HandoffGrant{ID: g.ID}).Update("cancelled", true).Error
	} else if action == "renew" {
		if g.BindingID == "" || p.Agent == nil || g.Cancelled || g.RequestExpires == nil || !time.Now().Before(*g.RequestExpires) {
			return g, code("operation_closed")
		}
		var b HandoffBinding
		if s.DB.Where(&HandoffBinding{ID: g.BindingID}).First(&b).Error != nil || b.AgentID != p.Agent.ID {
			return g, code("forbidden")
		}
		expiry := time.Now().UTC().Add(15 * time.Minute)
		if expiry.After(*g.RequestExpires) {
			expiry = *g.RequestExpires
		}
		result := s.DB.Model(&HandoffGrant{}).Where(&HandoffGrant{ID: g.ID}).Where(map[string]any{"cancelled": false}).Update("expires", expiry)
		err = result.Error
		if err == nil && result.RowsAffected != 1 {
			return g, code("operation_closed")
		}
	} else {
		return g, code("invalid_action")
	}
	if err != nil {
		return g, code("database_unavailable")
	}
	return s.GetHandoff(p, id)
}

func (s *Service) readableHandoff(p Principal, id string) (HandoffGrant, error) {
	g, err := s.GetHandoff(p, id)
	if err != nil {
		return g, err
	}
	if g.BindingID == "" || p.Agent == nil {
		return g, code("forbidden")
	}
	var b HandoffBinding
	if s.DB.Where(&HandoffBinding{ID: g.BindingID, Owner: g.Owner}).First(&b).Error != nil || b.Disabled || b.AgentID != p.Agent.ID {
		return g, code("forbidden")
	}
	if err := s.enforceBindingPrefix(p, b, g.FileID); err != nil {
		return g, err
	}
	if g.Cancelled || !time.Now().Before(g.Expires) {
		return g, code("operation_closed")
	}
	return g, nil
}

type handoffReadCloser struct {
	io.ReadCloser
	check func() error
}

func (r handoffReadCloser) Read(b []byte) (int, error) {
	if err := r.check(); err != nil {
		return 0, err
	}
	return r.ReadCloser.Read(b)
}

func (s *Service) OpenHandoff(ctx context.Context, p Principal, id string) (Version, io.ReadCloser, error) {
	g, err := s.readableHandoff(p, id)
	if err != nil {
		return Version{}, nil, err
	}
	_, v, body, err := s.Download(ctx, p, g.FileID, g.VersionID)
	if err != nil {
		return v, nil, err
	}
	if _, err := s.readableHandoff(p, id); err != nil {
		body.Close()
		return Version{}, nil, err
	}
	return v, handoffReadCloser{body, func() error { _, err := s.readableHandoff(p, id); return err }}, nil
}

func (s *Service) handoffRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/handoff-bindings", s.wrap("handoff.configure", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q HandoffBinding
		if err := decode(r, &q); err != nil {
			return err
		}
		v, err := s.CreateHandoffBinding(p, q)
		if err == nil {
			respond(w, v)
		}
		return err
	}))
	mux.HandleFunc("GET /api/v1/handoff-bindings", s.wrap("handoff.configure", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil || p.dataOwner() != p.UserID {
			return code("forbidden")
		}
		var rows []HandoffBinding
		if err := s.DB.Where(&HandoffBinding{Owner: p.dataOwner()}).Find(&rows).Error; err != nil {
			return code("database_unavailable")
		}
		respond(w, rows)
		return nil
	}))
	mux.HandleFunc("POST /api/v1/handoff-bindings/{id}/disable", s.wrap("handoff.configure", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if p.Agent != nil || p.dataOwner() != p.UserID {
			return code("forbidden")
		}
		result := s.DB.Model(&HandoffBinding{}).Where(&HandoffBinding{ID: r.PathValue("id"), Owner: p.dataOwner()}).Update("disabled", true)
		if result.Error != nil {
			return code("database_unavailable")
		}
		if result.RowsAffected != 1 {
			return code("not_found")
		}
		respond(w, map[string]bool{"disabled": true})
		return nil
	}))
	mux.HandleFunc("POST /api/v1/handoffs", s.wrap("handoff.bind", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		var q BindReferenceRequest
		if err := decode(r, &q); err != nil {
			return err
		}
		if q.BindingID == "" {
			return code("disabled")
		}
		v, err := s.BindReference(p, q, r.Header.Get("Idempotency-Key"))
		if err == nil {
			respond(w, v)
		}
		return err
	}))
	mux.HandleFunc("GET /api/v1/handoffs/{id}", s.wrap("handoff.read", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		v, err := s.GetHandoff(p, r.PathValue("id"))
		if err == nil {
			respond(w, v)
		}
		return err
	}))
	for _, action := range []string{"renew", "cancel"} {
		mux.HandleFunc("POST /api/v1/handoffs/{id}/"+action, s.wrap("handoff."+action, func(w http.ResponseWriter, r *http.Request, p Principal) error {
			v, err := s.ChangeHandoff(p, r.PathValue("id"), action)
			if err == nil {
				respond(w, v)
			}
			return err
		}))
	}
	mux.HandleFunc("GET /api/v1/handoffs/{id}/content", s.wrap("handoff.read", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		v, body, err := s.OpenHandoff(r.Context(), p, r.PathValue("id"))
		if err != nil {
			return err
		}
		defer body.Close()
		w.Header().Set("Content-Type", v.MIME)
		w.Header().Set("Content-Length", strconv.FormatInt(v.Size, 10))
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("X-Content-Type-Options", "nosniff")
		if _, err := io.Copy(w, body); err != nil {
			panic(http.ErrAbortHandler)
		}
		return nil
	}))
}
