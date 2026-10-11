package service

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// userKeyPrefix keeps operator-issued user identities on a distinct bearer
// namespace from agent tokens so authenticate never guesses tables.
const userKeyPrefix = "dbu_"

// validImportedKey accepts externally pre-generated keys of any prefix:
// 32-128 printable ASCII characters after trimming. The length floor is the
// brute-force entropy guard; generated keys stay dbu_+96hex by contrast.
func validImportedKey(k string) bool {
	k = strings.TrimSpace(k)
	if len(k) < 32 || len(k) > 128 {
		return false
	}
	for _, c := range k {
		if c < '!' || c > '~' {
			return false
		}
	}
	return true
}

// UserImport is one batch entry: an operator-supplied key (imported as-is) or
// an empty key (generated). Days is optional lifetime; zero means no expiry.
type UserImport struct {
	Name  string `json:"name"`
	Key   string `json:"key,omitempty"`
	Days  int    `json:"days,omitempty"`
	Admin bool   `json:"admin,omitempty"`
}

// UserKeyIssued returns issuance material; only its encrypted form persists.
type UserKeyIssued struct {
	UserID    string    `json:"user_id"`
	Name      string    `json:"name"`
	Key       string    `json:"key"`
	Admin     bool      `json:"admin"`
	ExpiresAt time.Time `json:"expires_at"`
}

// validateImports applies the whole-batch gate before creating anything so a
// bad entry cannot leave partial users behind.
func validateImports(entries []UserImport) error {
	if len(entries) == 0 || len(entries) > 1000 {
		return code("invalid_user_batch")
	}
	for _, v := range entries {
		if v.Name == "" || len(v.Name) > 120 {
			return code("invalid_user_batch")
		}
		if v.Key != "" && !validImportedKey(v.Key) {
			return code("invalid_user_key")
		}
		if v.Days < 0 {
			return code("invalid_user_batch")
		}
	}
	return nil
}

// ImportUsers batch-creates key users. It runs with operator authority (local
// CLI on the state directory); the HTTP surface wraps it with an admin-key
// check in CreateUserUsers.
func (s *Service) ImportUsers(entries []UserImport) ([]UserKeyIssued, error) {
	if e := validateImports(entries); e != nil {
		return nil, e
	}
	// Duplicate hashes are rejected before any row lands: the whole batch
	// either imports or leaves the database untouched. Duplicates inside one
	// batch and collisions with stored keys or agent tokens all reject.
	var n int64
	seen := map[string]bool{}
	for _, v := range entries {
		if v.Key == "" {
			continue
		}
		h := Hash(v.Key)
		if seen[h] {
			return nil, code("user_key_duplicate")
		}
		seen[h] = true
		if s.DB.Model(&UserKey{}).Where("token_hash = ?", h).Count(&n).Error != nil {
			return nil, code("database_unavailable")
		}
		if n > 0 {
			return nil, code("user_key_duplicate")
		}
		// Importing an agent token as a user key would make hash-first
		// bearer routing ambiguous; reject the whole batch.
		if s.DB.Model(&Agent{}).Where("token_hash = ?", h).Count(&n).Error != nil || n > 0 {
			return nil, code("user_key_duplicate")
		}
	}
	out := make([]UserKeyIssued, 0, len(entries))
	for _, v := range entries {
		key := v.Key
		if key == "" {
			key = userKeyPrefix + ID() + ID()
		}
		user := User{ID: ID(), Issuer: "key", Subject: "", Name: v.Name}
		user.Subject = user.ID
		k := UserKey{ID: ID(), Owner: user.ID, Name: v.Name, TokenHash: Hash(key), Admin: v.Admin, Created: time.Now().UTC()}
		if err := s.sealUserKey(&k, key); err != nil {
			return nil, err
		}
		if v.Days > 0 {
			k.Expires = k.Created.AddDate(0, 0, v.Days)
		}
		if e := s.DB.Create(&user).Error; e != nil {
			return nil, code("database_unavailable")
		}
		if e := s.DB.Create(&k).Error; e != nil {
			return nil, code("database_unavailable")
		}
		out = append(out, UserKeyIssued{UserID: user.ID, Name: v.Name, Key: key, Admin: v.Admin, ExpiresAt: k.Expires})
	}
	return out, nil
}

// CreateUserUsers is the HTTP path: admin arrives either as a user key
// bearer or as a session exchanged from an admin key.
func (s *Service) CreateUserUsers(p Principal, entries []UserImport) ([]UserKeyIssued, error) {
	if s.governanceActive() {
		return nil, code("account_recovery_required")
	}
	if !p.Admin {
		return nil, code("forbidden")
	}
	return s.ImportUsers(entries)
}

// ListUserKeys reports lifecycle metadata only; hashes never leave the store.
func (s *Service) ListUserKeys() []UserKey {
	var keys []UserKey
	s.DB.Order("created_at").Find(&keys)
	return keys
}

// RevokeUserKey flips the revocation switch; the next request is unauthorized.
func (s *Service) RevokeUserKey(id string) error {
	if id == "" {
		return code("invalid_user_key")
	}
	return s.DB.Transaction(func(tx *gorm.DB) error {
		res := tx.Model(&UserKey{}).Where("id = ?", id).Updates(map[string]any{"revoked": true, "sealed_key": nil})
		if res.Error != nil || res.RowsAffected == 0 {
			return code("not_found")
		}
		return tx.Where("origin_key = ?", id).Delete(&Session{}).Error
	})
}

// RotateUserKey retains the operator contract; browser self-service uses the
// same transaction with an exact owner and current-session constraint.
func (s *Service) RotateUserKey(id string, days int) (UserKeyIssued, error) {
	issued, _, err := s.rotateUserKey(id, days, "", "")
	return issued, err
}

// VerifyUserKey reports one key's lifecycle by its plaintext value: a local
// hash lookup with no network traffic and no authentication side effects.
// It exists so an operator can confirm a distributed key before handing it
// to a user or revoking it.
// SetUserKeyDisabled is the operator CLI path: disable keeps ciphertext,
// enable restores login. It still refuses to disable the last live key.
func (s *Service) SetUserKeyDisabled(id string, disabled bool) error {
	if id == "" {
		return code("invalid_user_key")
	}
	return s.setKeyDisabled(Principal{Admin: true, UserID: ""}, id, disabled)
}

func (s *Service) VerifyUserKey(key string) (UserKey, error) {
	if key == "" || len(key) > 128 {
		return UserKey{}, code("invalid_user_key")
	}
	var k UserKey
	if s.DB.First(&k, "token_hash = ?", Hash(key)).Error != nil {
		return UserKey{}, code("not_found")
	}
	return k, nil
}
