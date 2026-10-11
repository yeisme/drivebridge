package service

import (
	"gorm.io/gorm"
	"net/http"
	"time"
)

// Enrollment authorizes only an empty account's initial credential, never recovery.
type Enrollment struct {
	ID        string `gorm:"primaryKey"`
	UserID    string
	TokenHash string `gorm:"uniqueIndex"`
	Expires   time.Time
	Used      bool
}

func (s *Service) provisionAccount(p Principal, name string) (map[string]string, error) {
	if !s.platformAdmin(p.UserID) || !validName(name) {
		return nil, code("forbidden")
	}
	user := User{ID: ID(), Issuer: "key", Name: name}
	user.Subject = user.ID
	secret := ID() + ID()
	enrollment := Enrollment{ID: ID(), UserID: user.ID, TokenHash: Hash(secret), Expires: time.Now().UTC().Add(24 * time.Hour)}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if e := tx.Create(&user).Error; e != nil {
			return e
		}
		if e := tx.Create(&enrollment).Error; e != nil {
			return e
		}
		return s.event(tx, p, Space{TenantID: p.TenantID}, "account.provisioned", user.ID, "", "awaiting_activation", user.ID)
	})
	if err != nil {
		return nil, err
	}
	return map[string]string{"user_id": user.ID, "activation_code": secret}, nil
}
func (s *Service) enroll(w http.ResponseWriter, r *http.Request) {
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
		Code   string `json:"activation_code"`
		NewKey string `json:"new_key"`
	}
	if decode(r, &q) != nil || len(q.NewKey) < 64 || len(q.NewKey) > 200 {
		writeError(w, code("invalid_recovery"))
		return
	}
	s.governanceMu.Lock()
	defer s.governanceMu.Unlock()
	var enrollment Enrollment
	if s.DB.First(&enrollment, "token_hash = ? AND used = ? AND expires > ?", Hash(q.Code), false, time.Now()).Error != nil {
		writeError(w, code("invalid_recovery"))
		return
	}
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var u User
		if tx.First(&u, "id = ? AND disabled = ?", enrollment.UserID, false).Error != nil {
			return code("forbidden")
		}
		var keys int64
		if tx.Model(&UserKey{}).Where("owner = ?", u.ID).Count(&keys).Error != nil || keys != 0 {
			return code("invalid_recovery")
		}
		var agents int64
		if tx.Model(&Agent{}).Where("token_hash = ?", Hash(q.NewKey)).Count(&agents).Error != nil || agents != 0 {
			return code("invalid_recovery")
		}
		k := UserKey{ID: ID(), Owner: u.ID, Name: "Personal login", TokenHash: Hash(q.NewKey), Created: time.Now().UTC()}
		if e := s.sealUserKey(&k, q.NewKey); e != nil {
			return e
		}
		if e := tx.Create(&k).Error; e != nil {
			return code("invalid_recovery")
		}
		if e := tx.Model(&enrollment).Update("used", true).Error; e != nil {
			return e
		}
		return s.event(tx, Principal{UserID: u.ID, TenantID: s.Instance}, Space{TenantID: s.Instance}, "account.activated", u.ID, "", "success", u.ID)
	})
	if err != nil {
		writeError(w, err)
		return
	}
	respond(w, map[string]string{"user_id": enrollment.UserID, "status": "activated"})
}
