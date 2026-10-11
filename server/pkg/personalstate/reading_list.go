package personalstate

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// ErrTargetHidden is the explicit authorization outcome for an invisible or
// revoked target. Other authorization errors are failures, never empty results.
var ErrTargetHidden = errors.New("reading_target_hidden")

const readingScanBudget = 1000
const readingCursorLifetime = 5 * time.Minute

// One row per local subject/instance. Updated in the same transaction as
// state+receipt, so every committed state change invalidates older cursors.
type readingGeneration struct {
	ID       string `gorm:"primaryKey"`
	Revision uint64 `gorm:"not null"`
}

func (readingGeneration) TableName() string { return "local_reading_generations" }
func generationID(scope Scope) string       { return hashParts(scope.Subject, scope.Instance) }
func bumpGeneration(tx *gorm.DB, scope Scope) error {
	row := readingGeneration{ID: generationID(scope)}
	if tx.Clauses(clause.OnConflict{DoNothing: true}).Create(&row).Error != nil {
		return errors.New("reading_state_unavailable")
	}
	if tx.First(&row, "id = ?", row.ID).Error != nil || row.Revision >= maxRevision {
		return errors.New("reading_state_unavailable")
	}
	changed := tx.Model(&readingGeneration{}).Where("id = ? AND revision = ?", row.ID, row.Revision).Update("revision", row.Revision+1)
	if changed.Error != nil || changed.RowsAffected != 1 {
		return errors.New("reading_state_unavailable")
	}
	return nil
}
func (s *ReadingStore) generation(ctx context.Context, scope Scope) (uint64, error) {
	var row readingGeneration
	err := s.db.WithContext(ctx).First(&row, "id = ?", generationID(scope)).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return 0, nil
	}
	if err != nil || row.Revision > maxRevision {
		return 0, errors.New("reading_state_unavailable")
	}
	return row.Revision, nil
}

type readingCursor struct {
	Version     uint8  `json:"v"`
	Subject     string `json:"subject"`
	Instance    string `json:"instance"`
	AccessEpoch string `json:"access_epoch"`
	Generation  uint64 `json:"generation"`
	Limit       int    `json:"limit"`
	After       string `json:"after"`
	Expires     int64  `json:"expires"`
}

func newCursorCipher() (cipher.AEAD, error) {
	var key [32]byte
	if _, err := rand.Read(key[:]); err != nil {
		return nil, errors.New("reading_state_unavailable")
	}
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, errors.New("reading_state_unavailable")
	}
	return cipher.NewGCM(block)
}
func (s *ReadingStore) encodeCursor(value readingCursor) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", errors.New("reading_state_unavailable")
	}
	nonce := make([]byte, s.cursorCipher.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", errors.New("reading_state_unavailable")
	}
	ciphertext := s.cursorCipher.Seal(nonce, nonce, data, []byte("drivebridge.reading.cursor.v1"))
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}
func (s *ReadingStore) decodeCursor(encoded string) (readingCursor, error) {
	var value readingCursor
	invalid := errors.New("cursor_expired")
	if len(encoded) > 4096 {
		return value, invalid
	}
	data, err := base64.RawURLEncoding.DecodeString(encoded)
	if err != nil || len(data) < s.cursorCipher.NonceSize()+s.cursorCipher.Overhead() {
		return value, invalid
	}
	nonce := data[:s.cursorCipher.NonceSize()]
	plain, err := s.cursorCipher.Open(nil, nonce, data[s.cursorCipher.NonceSize():], []byte("drivebridge.reading.cursor.v1"))
	if err != nil || json.Unmarshal(plain, &value) != nil {
		return value, invalid
	}
	return value, nil
}

// List returns only references currently admitted by the owner, with no total
// or hidden-item count. accessEpoch is a trusted permission/binding generation
// from the application, not a request parameter. Recreate the store on session
// change; its cursor key is memory-only. The caller holds its authorization
// lease for the complete request, as required by Authorize.
//
// At most 1000 rows are examined. If a next authorized item cannot be established
// within that budget, fail explicitly, rather than returning an empty page that
// would falsely claim the end of the list or expose hidden-only continuations.
func (s *ReadingStore) List(ctx context.Context, scope Scope, accessEpoch string, query desktopcontract.PageQuery) (desktopcontract.ReadingStatePage, error) {
	var empty desktopcontract.ReadingStatePage
	if !validScope(scope) || !keyPattern.MatchString(accessEpoch) || len(query.Cursor) > 4096 {
		return empty, errors.New("invalid_reading_query")
	}
	limit := 50
	if query.Limit != nil {
		limit = int(*query.Limit)
	}
	if limit < 1 || limit > 200 {
		return empty, errors.New("invalid_reading_query")
	}
	generation, err := s.generation(ctx, scope)
	if err != nil {
		return empty, err
	}
	after := ""
	expires := s.now().Add(readingCursorLifetime).Unix()
	if query.Cursor != "" {
		cursor, err := s.decodeCursor(query.Cursor)
		if err != nil || cursor.Version != 1 || cursor.Subject != scope.Subject || cursor.Instance != scope.Instance || cursor.AccessEpoch != accessEpoch || cursor.Limit != limit || cursor.Generation != generation || cursor.Expires <= s.now().Unix() {
			return empty, errors.New("cursor_expired")
		}
		after = cursor.After
		expires = cursor.Expires
	}
	var rows []readingRow
	tx := s.db.WithContext(ctx).Where("subject = ? AND instance = ? AND id > ?", scope.Subject, scope.Instance, after).Order("id").Limit(readingScanBudget + 1)
	if tx.Find(&rows).Error != nil {
		return empty, errors.New("reading_state_unavailable")
	}
	result := desktopcontract.ReadingStatePage{Items: []desktopcontract.ReadingState{}, ObservedAt: s.now().UTC()}
	last := ""
	hasMore := false
	for index, row := range rows {
		if err := ctx.Err(); err != nil {
			return empty, err
		}
		if index == readingScanBudget {
			return empty, errors.New("reading_scan_budget_exceeded")
		}
		state, err := decodeState(row.State)
		if err != nil || state.Revision != row.Revision || state.TargetRef.InstanceID != scope.Instance || targetKey(state.TargetRef) != row.Target || row.ID != hashParts(scope.Subject, scope.Instance, row.Target) {
			return empty, errors.New("reading_state_unavailable")
		}
		if err := s.authorize(ctx, scope, state.TargetRef); err != nil {
			if errors.Is(err, ErrTargetHidden) {
				continue
			}
			return empty, errors.New("reading_target_unavailable")
		}
		if len(result.Items) == limit {
			hasMore = true
			break
		}
		result.Items = append(result.Items, state)
		last = row.ID
	}
	current, err := s.generation(ctx, scope)
	if err != nil {
		return empty, err
	}
	if current != generation {
		return empty, errors.New("cursor_expired")
	}
	if hasMore {
		result.NextCursor, err = s.encodeCursor(readingCursor{Version: 1, Subject: scope.Subject, Instance: scope.Instance, AccessEpoch: accessEpoch, Generation: generation, Limit: limit, After: last, Expires: expires})
		if err != nil {
			return empty, err
		}
	}
	return result, nil
}
