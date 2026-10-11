package service

import (
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"time"
	"unicode/utf8"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

func (s *Service) receiverEnabled() error {
	if !s.Config.Enabled {
		return code("disabled")
	}
	if !s.Config.ShareReceivers {
		return code("unsupported_action")
	}
	return nil
}

func (s *Service) IssueShareChallenge(id string) (ShareChallenge, error) {
	if err := s.receiverEnabled(); err != nil {
		return ShareChallenge{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var v Share
	if s.DB.First(&v, "id = ?", id).Error != nil || !shareActive(v) {
		return ShareChallenge{}, code("share_unavailable")
	}
	now := time.Now().UTC()
	if s.shareChallenges == nil {
		s.shareChallenges = make(map[string]shareChallengeState)
	}
	pending := 0
	for key, challenge := range s.shareChallenges {
		if !now.Before(challenge.Expires) {
			delete(s.shareChallenges, key)
		} else if challenge.ShareID == id {
			pending++
		}
	}
	// Bound unauthenticated challenge memory without invalidating another tab's nonce.
	if pending >= 100 || len(s.shareChallenges) >= 10000 {
		return ShareChallenge{}, code("throttled")
	}
	out := ShareChallenge{Nonce: ID(), Expires: now.Add(5 * time.Minute)}
	s.shareChallenges[Hash(out.Nonce)] = shareChallengeState{ShareID: id, Expires: out.Expires}
	return out, nil
}

func (s *Service) consumeShareChallenge(id, nonce string) bool {
	key := Hash(nonce)
	v, ok := s.shareChallenges[key]
	if !ok || v.ShareID != id {
		return false
	}
	delete(s.shareChallenges, key)
	return time.Now().Before(v.Expires)
}

func (s *Service) OpenShareProof(p Principal, id string, q ShareProofRequest) (ShareOpenResult, error) {
	if err := s.receiverEnabled(); err != nil {
		return ShareOpenResult{}, err
	}
	return s.openShareReceiver(p, id, "proof", "", q, ShareDeviceOpenRequest{})
}

func (s *Service) OpenShareWithDevice(p Principal, id string, q ShareDeviceOpenRequest) (ShareOpenResult, error) {
	if err := s.receiverEnabled(); err != nil {
		return ShareOpenResult{}, err
	}
	if p.Agent != nil || p.ServiceIdentity != nil {
		return ShareOpenResult{}, code("forbidden")
	}
	return s.openShareReceiver(p, id, "device", "", ShareProofRequest{}, q)
}

func shareProofFailure(tx *gorm.DB, v *Share) error {
	v.ProofFailures++
	if v.ProofFailures >= 10 {
		v.ProofFailures = 0
		v.ProofLockedUntil = time.Now().Add(5 * time.Minute)
	}
	return tx.Model(v).Updates(map[string]any{"proof_failures": v.ProofFailures, "proof_locked_until": v.ProofLockedUntil}).Error
}

// Authentication failures commit their isolated throttle counters, never a session
// or quota debit. The row lock serializes issuances across database connections.
func (s *Service) openShareReceiver(p Principal, id, kind, grant string, proof ShareProofRequest, deviceRequest ShareDeviceOpenRequest) (ShareOpenResult, error) {
	if !s.Config.Enabled {
		return ShareOpenResult{}, code("disabled")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if kind != "grant" {
		nonce := proof.Nonce
		if kind == "device" {
			nonce = deviceRequest.Nonce
		}
		if !s.consumeShareChallenge(id, nonce) {
			return ShareOpenResult{}, code("share_unavailable")
		}
	}
	var out ShareOpenResult
	var denied error
	err := s.DB.Transaction(func(tx *gorm.DB) error {
		var v Share
		if err := tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&v, "id = ?", id).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				denied = code("share_unavailable")
				return nil
			}
			return err
		}
		if !shareActive(v) {
			denied = code("share_unavailable")
			return nil
		}
		if kind != "grant" && time.Now().Before(v.ProofLockedUntil) {
			denied = code("throttled")
			return nil
		}
		var device ShareDevice
		valid := false
		switch kind {
		case "grant":
			valid = subtle.ConstantTimeCompare([]byte(Hash(grant)), []byte(v.TokenHash)) == 1
		case "proof":
			var secret string
			if err := s.unseal(v.Owner, v.Secret, &secret); err != nil {
				return err
			}
			mac := hmac.New(sha256.New, []byte(secret))
			mac.Write([]byte(id + ":" + proof.Nonce))
			decoded, err := hex.DecodeString(proof.Proof)
			valid = err == nil && hmac.Equal(mac.Sum(nil), decoded)
		case "device":
			if v.Audience != "link" {
				denied = code("share_unavailable")
				return nil
			}
			err := tx.First(&device, "id = ? AND share_id = ?", deviceRequest.DeviceID, id).Error
			if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
				return err
			}
			signature, decodeErr := hex.DecodeString(deviceRequest.Signature)
			valid = err == nil && !device.Revoked && len(device.PublicKey) == ed25519.PublicKeySize && decodeErr == nil && ed25519.Verify(device.PublicKey, []byte(id+":"+deviceRequest.Nonce), signature)
		}
		if !valid {
			denied = code("share_unavailable")
			if kind != "grant" {
				if err := shareProofFailure(tx, &v); err != nil {
					return err
				}
				if time.Now().Before(v.ProofLockedUntil) {
					denied = code("throttled")
				}
			}
			return nil
		}
		if kind != "grant" {
			if err := tx.Model(&v).Update("proof_failures", 0).Error; err != nil {
				return err
			}
		}
		if kind != "device" {
			if time.Now().Before(v.LockedUntil) {
				denied = code("throttled")
				return nil
			}
			if len(v.Passcode) > 0 && bcrypt.CompareHashAndPassword(v.Passcode, []byte(proof.Passcode)) != nil {
				v.Failures++
				if v.Failures >= 10 {
					v.Failures = 0
					v.LockedUntil = time.Now().Add(5 * time.Minute)
				}
				denied = code("share_passcode_required")
				return tx.Model(&v).Updates(map[string]any{"failures": v.Failures, "locked_until": v.LockedUntil}).Error
			}
		}
		if v.Audience == "users" && (p.UserID == "" || p.Agent != nil || p.ServiceIdentity != nil || !containsRecipient(v, p.UserID)) {
			denied = code("forbidden")
			return nil
		}
		if v.MaxOpens != nil && v.OpensUsed >= *v.MaxOpens {
			denied = code("share_unavailable")
			return nil
		}
		now := time.Now().UTC()
		token := ID()
		session := ShareSession{ID: ID(), TokenHash: Hash(token), ShareID: id, Kind: kind, Created: now, Expires: now.Add(time.Hour)}
		if !v.Expires.IsZero() && v.Expires.Before(session.Expires) {
			session.Expires = v.Expires
		}
		receiver := "anonymous"
		if kind == "device" {
			session.DeviceFingerprint = device.Fingerprint
			receiver = "device"
			if err := tx.Model(&device).Updates(map[string]any{"opens": gorm.Expr("opens + 1"), "last_seen": now}).Error; err != nil {
				return err
			}
		} else if v.Audience == "users" {
			session.UserID = p.UserID
			receiver = "user"
		}
		if err := tx.Create(&session).Error; err != nil {
			return err
		}
		updates := map[string]any{"opens_used": gorm.Expr("opens_used + 1")}
		if kind != "device" {
			updates["failures"] = 0
		}
		if err := tx.Model(&v).Updates(updates).Error; err != nil {
			return err
		}
		out = ShareOpenResult{Opened: true, Token: token, Expires: session.Expires, SessionKind: kind, ReceiverKind: receiver}
		return nil
	})
	if err != nil {
		return ShareOpenResult{}, code("database_unavailable")
	}
	if denied != nil {
		return ShareOpenResult{}, denied
	}
	return out, nil
}

func (s *Service) shareReceiverSession(tx *gorm.DB, v Share, token string) (ShareSession, string, error) {
	var session ShareSession
	if !shareActive(v) || token == "" {
		return session, "", code("share_unavailable")
	}
	if !v.SessionRequired && v.MaxOpens == nil && v.MaxDownloads == nil && !v.SingleUse && subtle.ConstantTimeCompare([]byte(Hash(token)), []byte(v.TokenHash)) == 1 {
		// Only pre-session, unlimited shares grandfather old grant cookies.
		// New and quota-bearing shares must have completed a real opening.
		return ShareSession{ID: "legacy:" + v.TokenHash, ShareID: v.ID, Kind: "legacy"}, "anonymous", nil
	}
	err := tx.First(&session, "share_id = ? AND token_hash = ?", v.ID, Hash(token)).Error
	if err != nil || session.Revoked || !time.Now().Before(session.Expires) {
		return ShareSession{}, "", code("share_unavailable")
	}
	if session.Kind == "device" {
		var device ShareDevice
		if tx.First(&device, "share_id = ? AND fingerprint = ? AND revoked = ?", v.ID, session.DeviceFingerprint, false).Error != nil {
			return ShareSession{}, "", code("share_unavailable")
		}
		return session, "device", nil
	}
	if session.UserID != "" {
		return session, "user", nil
	}
	return session, "anonymous", nil
}

func (s *Service) RegisterShareReceiverDevice(p Principal, id, token string, q ShareDeviceRequest) (ShareDevice, error) {
	if err := s.receiverEnabled(); err != nil {
		return ShareDevice{}, err
	}
	if p.Agent != nil || p.ServiceIdentity != nil {
		return ShareDevice{}, code("forbidden")
	}
	public, err := hex.DecodeString(q.PublicKey)
	if err != nil || len(public) != ed25519.PublicKeySize || len(q.Label) > 120 || !utf8.ValidString(q.Label) {
		return ShareDevice{}, code("invalid_share")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var out ShareDevice
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		var v Share
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&v, "id = ?", id).Error != nil || !shareActive(v) {
			return code("share_unavailable")
		}
		if v.Audience != "link" {
			return code("invalid_share")
		}
		session, _, err := s.shareReceiverSession(tx, v, token)
		if err != nil {
			return err
		}
		if session.Kind != "grant" && session.Kind != "proof" {
			return code("forbidden")
		}
		fingerprint := Hash(string(public))
		err = tx.First(&out, "share_id = ? AND fingerprint = ?", id, fingerprint).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return code("database_unavailable")
		}
		var count int64
		if tx.Model(&ShareDevice{}).Where("share_id = ?", id).Count(&count).Error != nil {
			return code("database_unavailable")
		}
		if count >= 20 {
			return code("invalid_share")
		}
		out = ShareDevice{ID: ID(), ShareID: id, Fingerprint: fingerprint, PublicKey: public, Label: q.Label, Created: time.Now().UTC()}
		if tx.Create(&out).Error != nil {
			return code("database_unavailable")
		}
		return nil
	})
	return out, err
}

func (s *Service) receiverOwner(tx *gorm.DB, p Principal, id string) (Share, error) {
	var v Share
	if p.Agent != nil || p.ServiceIdentity != nil || p.Key != nil || p.SessionHash == "" || p.UserID == "" {
		return v, code("forbidden")
	}
	if err := tx.First(&v, "id = ? AND owner = ?", id, p.dataOwner()).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return v, code("not_found")
		}
		return v, code("database_unavailable")
	}
	if v.ActorID != "" && v.ActorID != p.UserID {
		return v, code("forbidden")
	}
	return v, nil
}

func (s *Service) ListShareReceivers(p Principal, id string) (ShareReceivers, error) {
	if err := s.receiverEnabled(); err != nil {
		return ShareReceivers{}, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	v, err := s.receiverOwner(s.DB, p, id)
	if err != nil {
		return ShareReceivers{}, err
	}
	out := ShareReceivers{Devices: []ShareDevice{}, OpensUsed: v.OpensUsed, OpensMax: v.MaxOpens, DownloadsUsed: v.DownloadsUsed, DownloadsMax: v.MaxDownloads}
	// There can be at most 20 devices, below the standard 50-item page size.
	if s.DB.Where("share_id = ?", id).Order("created, id").Find(&out.Devices).Error != nil {
		return ShareReceivers{}, code("database_unavailable")
	}
	return out, nil
}

func (s *Service) RevokeShareReceiver(p Principal, id, deviceID string) error {
	if err := s.receiverEnabled(); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.DB.Transaction(func(tx *gorm.DB) error {
		if _, err := s.receiverOwner(tx, p, id); err != nil {
			return err
		}
		var locked Share
		if tx.Clauses(clause.Locking{Strength: "UPDATE"}).First(&locked, "id = ?", id).Error != nil {
			return code("database_unavailable")
		}
		result := tx.Model(&ShareDevice{}).Where("share_id = ? AND id = ?", id, deviceID).Update("revoked", true)
		if result.Error != nil {
			return code("database_unavailable")
		}
		if result.RowsAffected == 0 {
			return code("not_found")
		}
		return nil
	})
}
