package service

import (
	"crypto/subtle"
	"gorm.io/gorm"
	"net/http"
	"time"
)

type RecoveryClaim struct {
	ID         string    `json:"id" gorm:"primaryKey"`
	UserID     string    `json:"user_id"`
	ProofHash  string    `json:"-"`
	NewKeyHash string    `json:"-"`
	CodeHash   string    `json:"-"`
	Kind       string    `json:"kind"`
	Status     string    `json:"status"`
	Reviewer   string    `json:"reviewer"`
	Created    time.Time `json:"created_at"`
	Expires    time.Time `json:"expires_at"`
}
type RecoveryCode struct {
	Hash   string `json:"-" gorm:"primaryKey"`
	UserID string `json:"user_id" gorm:"index"`
	Used   bool   `json:"used"`
}

func (s *Service) verifyProof(p Principal, proof string) error {
	if p.Agent != nil || p.SessionHash == "" {
		return code("reauthentication_required")
	}
	if proof != "" {
		var k UserKey
		if s.DB.First(&k, "token_hash = ? AND owner = ? AND revoked = ?", Hash(proof), p.UserID, false).Error == nil && (k.Expires.IsZero() || time.Now().Before(k.Expires)) {
			return nil
		}
	}
	var session Session
	if s.DB.First(&session, "hash = ?", p.SessionHash).Error == nil && session.OriginKey == "" && !session.AuthenticatedAt.IsZero() && time.Since(session.AuthenticatedAt) < 5*time.Minute {
		return nil
	}
	return code("reauthentication_required")
}
func (s *Service) recoveryRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/v1/governance/recovery-code", s.wrap("governance.recovery-code", func(w http.ResponseWriter, r *http.Request, p Principal) error {
		if !s.governanceActive() {
			return code("forbidden")
		}
		var q governanceInput
		if e := decode(r, &q); e != nil {
			return e
		}
		if e := s.verifyProof(p, q.Proof); e != nil {
			return e
		}
		secret := ID() + ID()
		if e := s.DB.Transaction(func(tx *gorm.DB) error {
			if e := tx.Where("user_id = ?", p.UserID).Delete(&RecoveryCode{}).Error; e != nil {
				return e
			}
			if e := tx.Create(&RecoveryCode{Hash: Hash(secret), UserID: p.UserID}).Error; e != nil {
				return e
			}
			return s.event(tx, p, Space{TenantID: p.TenantID}, "account.recovery_code", p.UserID, "", "success", p.UserID)
		}); e != nil {
			return e
		}
		respond(w, map[string]string{"recovery_code": secret})
		return nil
	}))
	mux.HandleFunc("POST /auth/recovery/request", func(w http.ResponseWriter, r *http.Request) {
		if !s.Config.Enabled || !s.governanceActive() {
			writeError(w, code("disabled"))
			return
		}
		if r.Header.Get("Origin") != s.Config.Origin {
			writeError(w, code("csrf_rejected"))
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var q struct {
			UserID       string `json:"user_id"`
			NewKey       string `json:"new_key"`
			RecoveryCode string `json:"recovery_code"`
		}
		if decode(r, &q) != nil || len(q.NewKey) < 64 || len(q.NewKey) > 200 {
			writeError(w, code("invalid_recovery"))
			return
		}
		secret := ID() + ID()
		claim := RecoveryClaim{ID: ID(), UserID: q.UserID, ProofHash: Hash(secret), NewKeyHash: Hash(q.NewKey), Kind: "manual", Status: "pending", Created: time.Now().UTC(), Expires: time.Now().UTC().Add(24 * time.Hour)}
		s.governanceMu.Lock()
		defer s.governanceMu.Unlock()
		var u User
		if s.DB.First(&u, "id = ? AND disabled = ?", q.UserID, false).Error == nil {
			var n int64
			s.DB.Model(&RecoveryClaim{}).Where("user_id = ? AND created > ?", q.UserID, time.Now().Add(-time.Hour)).Count(&n)
			if n >= 5 {
				writeError(w, code("throttled"))
				return
			}
			var duplicate int64
			s.DB.Model(&UserKey{}).Where("token_hash = ?", claim.NewKeyHash).Count(&duplicate)
			var agent int64
			s.DB.Model(&Agent{}).Where("token_hash = ?", claim.NewKeyHash).Count(&agent)
			if duplicate+agent > 0 {
				writeError(w, code("invalid_recovery"))
				return
			}
			if q.RecoveryCode != "" {
				var c RecoveryCode
				if s.DB.First(&c, "hash = ? AND user_id = ? AND used = ?", Hash(q.RecoveryCode), u.ID, false).Error == nil {
					claim.Kind, claim.Status, claim.CodeHash = "self", "approved", c.Hash
				}
			}
			// A fresh OIDC login is another proof of the same canonical identity.
			if p, e := s.authenticate(r); e == nil && p.UserID == u.ID && s.verifyProof(p, "") == nil {
				claim.Kind, claim.Status = "self", "approved"
			}
			if e := s.DB.Transaction(func(tx *gorm.DB) error {
				if e := tx.Create(&claim).Error; e != nil {
					return e
				}
				var roles []PlatformRole
				tx.Where("suspended = ?", false).Find(&roles)
				recipients := []string{u.ID}
				for _, role := range roles {
					recipients = append(recipients, role.UserID)
				}
				return s.event(tx, Principal{UserID: u.ID, TenantID: s.Instance}, Space{TenantID: s.Instance}, "account.recovery_requested", claim.ID, "", "pending", recipients...)
			}); e != nil {
				writeError(w, e)
				return
			}
		}
		// Same response for unknown identities. Only the applicant receives this proof.
		respond(w, map[string]string{"id": claim.ID, "claim_proof": secret, "status": "submitted"})
	})
	mux.HandleFunc("POST /auth/recovery/complete", func(w http.ResponseWriter, r *http.Request) {
		if !s.Config.Enabled || !s.governanceActive() {
			writeError(w, code("disabled"))
			return
		}
		if r.Header.Get("Origin") != s.Config.Origin {
			writeError(w, code("csrf_rejected"))
			return
		}
		r.Body = http.MaxBytesReader(w, r.Body, 4096)
		var q struct {
			ID    string `json:"id"`
			Proof string `json:"claim_proof"`
		}
		if decode(r, &q) != nil {
			writeError(w, code("invalid_recovery"))
			return
		}
		s.governanceMu.Lock()
		defer s.governanceMu.Unlock()
		var c RecoveryClaim
		if s.DB.First(&c, "id = ? AND status = ?", q.ID, "approved").Error != nil || !time.Now().Before(c.Expires) || subtle.ConstantTimeCompare([]byte(c.ProofHash), []byte(Hash(q.Proof))) != 1 {
			writeError(w, code("recovery_not_ready"))
			return
		}
		if c.Kind == "manual" && !s.platformAdmin(c.Reviewer) {
			writeError(w, code("forbidden"))
			return
		}
		err := s.DB.Transaction(func(tx *gorm.DB) error {
			var u User
			if tx.First(&u, "id = ? AND disabled = ?", c.UserID, false).Error != nil {
				return code("forbidden")
			}
			if c.CodeHash != "" {
				res := tx.Model(&RecoveryCode{}).Where("hash = ? AND used = ?", c.CodeHash, false).Update("used", true)
				if res.Error != nil || res.RowsAffected != 1 {
					return code("invalid_recovery")
				}
			}
			if e := tx.Model(&UserKey{}).Where("owner = ?", c.UserID).Update("revoked", true).Error; e != nil {
				return e
			}
			if e := tx.Where("owner = ?", c.UserID).Delete(&Session{}).Error; e != nil {
				return e
			}
			if e := tx.Model(&Agent{}).Where("owner = ?", c.UserID).Update("revoked", true).Error; e != nil {
				return e
			}
			if e := s.invalidateRequests(tx, Principal{UserID: c.UserID, TenantID: s.Instance}, tx.Model(&AccessRequest{}).Where("(requester = ? OR approver = ?) AND status IN ?", c.UserID, c.UserID, []string{"pending", "approved", "executing"})); e != nil {
				return e
			}
			if c.Kind == "manual" {
				if e := tx.Model(&PlatformRole{}).Where("user_id = ?", c.UserID).Update("suspended", true).Error; e != nil {
					return e
				}
				var ms []Membership
				tx.Where("user_id = ? AND role = ?", c.UserID, "admin").Find(&ms)
				for _, m := range ms {
					m.Role, m.SuspendedRole = "member", "admin"
					if e := tx.Save(&m).Error; e != nil {
						return e
					}
				}
				var ts []TeamMember
				tx.Where("user_id = ? AND role = ?", c.UserID, "admin").Find(&ts)
				for _, m := range ts {
					m.Role, m.SuspendedRole = "member", "admin"
					if e := tx.Save(&m).Error; e != nil {
						return e
					}
				}
				var ps []SpacePermission
				tx.Where("user_id = ? AND role = ?", c.UserID, "custodian").Find(&ps)
				for _, m := range ps {
					m.Role, m.SuspendedRole = "viewer", "custodian"
					if e := tx.Save(&m).Error; e != nil {
						return e
					}
				}
			}
			k := UserKey{ID: ID(), Owner: c.UserID, Name: "Recovered login", TokenHash: c.NewKeyHash, Created: time.Now().UTC()}
			if e := tx.Create(&k).Error; e != nil {
				return code("invalid_recovery")
			}
			c.Status = "completed"
			if e := tx.Save(&c).Error; e != nil {
				return e
			}
			return s.event(tx, Principal{UserID: c.UserID, TenantID: s.Instance}, Space{TenantID: s.Instance}, "account.recovered", c.ID, "", "success", c.UserID, c.Reviewer)
		})
		if err != nil {
			writeError(w, err)
			return
		}
		respond(w, map[string]bool{"completed": true, "login_required": true})
	})
}
func (s *Service) recoveryAdmin(w http.ResponseWriter, r *http.Request, p Principal, action string, q governanceInput) error {
	if !s.platformAdmin(p.UserID) || s.verifyProof(p, q.Proof) != nil {
		return code("reauthentication_required")
	}
	switch action {
	case "recovery-approve":
		var claim RecoveryClaim
		if s.DB.First(&claim, "id = ? AND status = ?", q.ClaimID, "pending").Error != nil || !time.Now().Before(claim.Expires) || claim.UserID == p.UserID || len(q.Reason) < 3 || len(q.Reason) > 1000 {
			return code("forbidden")
		}
		claim.Status, claim.Reviewer, claim.Expires = "approved", p.UserID, time.Now().UTC().Add(time.Hour)
		if e := s.DB.Transaction(func(tx *gorm.DB) error {
			if e := tx.Save(&claim).Error; e != nil {
				return e
			}
			return s.event(tx, p, Space{TenantID: s.Instance}, "account.recovery_verified", claim.ID, "", "approved", claim.UserID)
		}); e != nil {
			return e
		}
	case "account-status":
		action := "unfreeze"
		if q.Disabled {
			action = "freeze"
		}
		if err := s.changeAccount(p, q.UserID, action, nil); err != nil {
			return err
		}
	case "restore-privileges":
		if q.UserID == p.UserID {
			return code("forbidden")
		}
		if e := s.DB.Transaction(func(tx *gorm.DB) error {
			if e := tx.Model(&PlatformRole{}).Where("user_id = ?", q.UserID).Update("suspended", false).Error; e != nil {
				return e
			}
			var ms []Membership
			tx.Where("user_id = ? AND suspended_role <> ?", q.UserID, "").Find(&ms)
			for _, m := range ms {
				m.Role, m.SuspendedRole = m.SuspendedRole, ""
				if e := tx.Save(&m).Error; e != nil {
					return e
				}
			}
			var ts []TeamMember
			tx.Where("user_id = ? AND suspended_role <> ?", q.UserID, "").Find(&ts)
			for _, m := range ts {
				m.Role, m.SuspendedRole = m.SuspendedRole, ""
				if e := tx.Save(&m).Error; e != nil {
					return e
				}
			}
			var ps []SpacePermission
			tx.Where("user_id = ? AND suspended_role <> ?", q.UserID, "").Find(&ps)
			for _, m := range ps {
				m.Role, m.SuspendedRole = m.SuspendedRole, ""
				if e := tx.Save(&m).Error; e != nil {
					return e
				}
			}
			return s.event(tx, p, Space{TenantID: s.Instance}, "account.privileges_restored", q.UserID, "", "success", q.UserID)
		}); e != nil {
			return e
		}
	}
	respond(w, map[string]bool{"ok": true})
	return nil
}
