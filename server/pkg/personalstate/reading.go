package personalstate

import (
	"context"
	"crypto/cipher"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"time"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"gorm.io/gorm"
)

const maxRevision uint64 = 9007199254740991

var keyPattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

// Scope comes from the authenticated application, never a renderer body.
// Instance prevents a reference from being attached to a different runtime.
type Scope struct{ Subject, Instance string }

// Authorize must resolve the fixed target through its owner on EVERY call,
// including receipt replay and lookup. It must refuse hidden/revoked sources.
// Called before the local transaction so a single-connection owner repository
// does not deadlock. The caller holds any source authorization/guard lease
// through the operation; this store cannot grant or freeze owner permissions.
type Authorize func(context.Context, Scope, desktopcontract.ReadingRef) error

type ReadingStore struct {
	db           *gorm.DB
	authorize    Authorize
	cursorCipher cipher.AEAD
	now          func() time.Time
}

type readingRow struct {
	ID       string `gorm:"primaryKey"`
	Subject  string `gorm:"not null;uniqueIndex:local_reading_identity"`
	Instance string `gorm:"not null;uniqueIndex:local_reading_identity"`
	Target   string `gorm:"not null;uniqueIndex:local_reading_identity"`
	Revision uint64 `gorm:"not null"`
	State    string `gorm:"not null"`
}

func (readingRow) TableName() string { return "local_reading_states" }

type readingReceipt struct {
	ID       string `gorm:"primaryKey"`
	Subject  string `gorm:"not null;uniqueIndex:local_reading_receipt_identity"`
	Instance string `gorm:"not null;uniqueIndex:local_reading_receipt_identity"`
	Key      string `gorm:"not null;uniqueIndex:local_reading_receipt_identity"`
	Digest   string `gorm:"not null"`
	State    string `gorm:"not null"`
}

func (readingReceipt) TableName() string { return "local_reading_receipts" }

// MigrateReading is explicit: constructing a store never touches a database.
// Only apply to the caller-owned LOCAL state DB, not the server favorites DB.
func MigrateReading(db *gorm.DB) error {
	if db == nil {
		return errors.New("reading_state_unavailable")
	}
	return db.Transaction(func(tx *gorm.DB) error { return tx.AutoMigrate(&readingRow{}, &readingReceipt{}, &readingGeneration{}) })
}
func NewReadingStore(db *gorm.DB, authorize Authorize) (*ReadingStore, error) {
	if db == nil || authorize == nil {
		return nil, errors.New("reading_state_unavailable")
	}
	cipher, err := newCursorCipher()
	if err != nil {
		return nil, err
	}
	return &ReadingStore{db: db, authorize: authorize, cursorCipher: cipher, now: time.Now}, nil
}
func validScope(scope Scope) bool {
	return keyPattern.MatchString(scope.Subject) && keyPattern.MatchString(scope.Instance)
}
func targetKey(ref desktopcontract.ReadingRef) string {
	// Revisions deliberately do not participate in logical favorite identity.
	if ref.Kind == "file" {
		return hashParts(ref.Kind, ref.InstanceID, ref.SpaceID, ref.FileID)
	}
	return hashParts(ref.Kind, ref.InstanceID, ref.SourceID, ref.ObjectID)
}
func hashParts(values ...string) string {
	encoded, _ := json.Marshal(values)
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:])
}
func refVersion(ref desktopcontract.ReadingRef) string {
	if ref.Kind == "file" {
		return ref.VersionID
	}
	return ref.Revision
}
func (s *ReadingStore) admit(ctx context.Context, scope Scope, ref desktopcontract.ReadingRef) error {
	if !validScope(scope) || ref.Validate() != nil || ref.InstanceID != scope.Instance {
		return errors.New("invalid_reading_ref")
	}
	if err := s.authorize(ctx, scope, ref); err != nil {
		return errors.New("reading_target_unavailable")
	}
	return nil
}
func decodeState(encoded string) (desktopcontract.ReadingState, error) {
	var state desktopcontract.ReadingState
	if len(encoded) > 8192 || json.Unmarshal([]byte(encoded), &state) != nil || state.TargetRef.Validate() != nil || state.Revision == 0 || state.Revision > maxRevision || state.UpdatedAt.IsZero() {
		return state, errors.New("reading_state_unavailable")
	}
	if state.Position != nil {
		update := desktopcontract.UpdateReadingState{TargetRef: state.TargetRef, Position: state.Position}
		if update.Validate() != nil {
			return state, errors.New("reading_state_unavailable")
		}
	}
	return state, nil
}

// Update atomically stores the CAS result and its original receipt. Replaying
// an old successful key returns its original result, not the latest row. A
// conflicting CAS records no receipt and never writes target bytes.
func (s *ReadingStore) Update(ctx context.Context, scope Scope, key string, request desktopcontract.UpdateReadingState) (desktopcontract.ReadingState, error) {
	var output desktopcontract.ReadingState
	if !keyPattern.MatchString(key) || !validScope(scope) || request.Validate() != nil || request.ExpectedRevision >= maxRevision {
		return output, errors.New("invalid_reading_update")
	}
	if request.Position != nil && strings.ContainsAny(request.Position.Anchor, "\x00\r\n") {
		return output, errors.New("invalid_reading_position")
	}
	encoded, err := json.Marshal(request)
	if err != nil || len(encoded) > 8192 {
		return output, errors.New("invalid_reading_update")
	}
	digest := hashParts(string(encoded))
	// Own request pointer fields for the complete transaction.
	if json.Unmarshal(encoded, &request) != nil {
		return output, errors.New("invalid_reading_update")
	}
	if err := s.admit(ctx, scope, request.TargetRef); err != nil {
		return output, err
	}
	err = s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var receipt readingReceipt
		receiptQuery := tx.Where("subject = ? AND instance = ? AND key = ?", scope.Subject, scope.Instance, key)
		result := receiptQuery.First(&receipt).Error
		if result == nil {
			if receipt.Digest != digest {
				return errors.New("idempotency_conflict")
			}
			stored, err := decodeState(receipt.State)
			if err != nil || stored.TargetRef != request.TargetRef {
				return errors.New("reading_state_unavailable")
			}
			output = stored
			return nil
		}
		if !errors.Is(result, gorm.ErrRecordNotFound) {
			return errors.New("reading_state_unavailable")
		}
		target := targetKey(request.TargetRef)
		var row readingRow
		result = tx.Where("subject = ? AND instance = ? AND target = ?", scope.Subject, scope.Instance, target).First(&row).Error
		current := desktopcontract.ReadingState{TargetRef: request.TargetRef}
		exists := result == nil
		if exists {
			var err error
			current, err = decodeState(row.State)
			if err != nil || current.Revision != row.Revision || targetKey(current.TargetRef) != target {
				return errors.New("reading_state_unavailable")
			}
		} else if !errors.Is(result, gorm.ErrRecordNotFound) {
			return errors.New("reading_state_unavailable")
		}
		if current.Revision != request.ExpectedRevision {
			return errors.New("revision_conflict")
		}
		if refVersion(current.TargetRef) != refVersion(request.TargetRef) {
			current.Position = nil
		}
		current.TargetRef = request.TargetRef
		if request.Favorite != nil {
			current.Favorite = *request.Favorite
		}
		if request.ReadLater != nil {
			current.ReadLater = *request.ReadLater
		}
		if request.Position != nil {
			position := *request.Position
			current.Position = &position
		}
		if request.ClearPosition != nil && *request.ClearPosition {
			current.Position = nil
		}
		current.Revision++
		current.UpdatedAt = time.Now().UTC()
		stored, err := json.Marshal(current)
		if err != nil {
			return errors.New("reading_state_unavailable")
		}
		if exists {
			change := tx.Model(&readingRow{}).Where("id = ? AND revision = ?", row.ID, request.ExpectedRevision).Updates(map[string]any{"revision": current.Revision, "state": string(stored)})
			if change.Error != nil {
				return errors.New("reading_state_unavailable")
			}
			if change.RowsAffected != 1 {
				return errors.New("revision_conflict")
			}
		} else {
			row = readingRow{ID: hashParts(scope.Subject, scope.Instance, target), Subject: scope.Subject, Instance: scope.Instance, Target: target, Revision: current.Revision, State: string(stored)}
			if tx.Create(&row).Error != nil {
				return errors.New("reading_state_unavailable")
			}
		}
		receipt = readingReceipt{ID: hashParts(scope.Subject, scope.Instance, key), Subject: scope.Subject, Instance: scope.Instance, Key: key, Digest: digest, State: string(stored)}
		if tx.Create(&receipt).Error != nil {
			return errors.New("reading_state_unavailable")
		}
		if err := bumpGeneration(tx, scope); err != nil {
			return err
		}
		output = current
		return nil
	})
	if err != nil {
		return desktopcontract.ReadingState{}, err
	}
	return output, nil
}

// Get resolves one admitted logical target. Stored position/ref stays pinned
// to its saved version; this method never pretends it belongs to the query's
// newer revision. The owner must also authorize that saved fixed reference.
func (s *ReadingStore) Get(ctx context.Context, scope Scope, ref desktopcontract.ReadingRef) (desktopcontract.ReadingState, bool, error) {
	var empty desktopcontract.ReadingState
	if err := s.admit(ctx, scope, ref); err != nil {
		return empty, false, err
	}
	var row readingRow
	err := s.db.WithContext(ctx).Where("subject = ? AND instance = ? AND target = ?", scope.Subject, scope.Instance, targetKey(ref)).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return empty, false, nil
	}
	if err != nil {
		return empty, false, errors.New("reading_state_unavailable")
	}
	state, err := decodeState(row.State)
	if err != nil {
		return empty, false, err
	}
	if state.Revision != row.Revision || targetKey(state.TargetRef) != targetKey(ref) {
		return empty, false, errors.New("reading_state_unavailable")
	}
	if err := s.admit(ctx, scope, state.TargetRef); err != nil {
		return empty, false, err
	}
	return state, true, nil
}

func (s *ReadingStore) Lookup(ctx context.Context, scope Scope, key string) (desktopcontract.ReadingMutationResult, error) {
	var empty desktopcontract.ReadingMutationResult
	if !validScope(scope) || !keyPattern.MatchString(key) {
		return empty, errors.New("invalid_reading_lookup")
	}
	var receipt readingReceipt
	err := s.db.WithContext(ctx).Where("subject = ? AND instance = ? AND key = ?", scope.Subject, scope.Instance, key).First(&receipt).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return empty, nil
	}
	if err != nil {
		return empty, errors.New("reading_state_unavailable")
	}
	state, err := decodeState(receipt.State)
	if err != nil {
		return empty, err
	}
	if err := s.admit(ctx, scope, state.TargetRef); err != nil {
		return empty, err
	}
	return desktopcontract.ReadingMutationResult{Found: true, State: &state}, nil
}
