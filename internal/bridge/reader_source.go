package bridge

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"github.com/yeisme/drivebridge-server/pkg/managedstate"
	"gorm.io/gorm"
	"path/filepath"
	"time"
)

// RegisterReaderRoot records an explicitly selected Pinax owner root. This is
// local reader custody, not a cloud storage connection or a provider adapter.
// Ordinary mutations stay behind the existing adoption/managed-vault gates.
func (s *Service) RegisterReaderRoot(root string) error {
	canonical, err := canonicalMutationPath(root)
	if err != nil || canonical != filepath.Clean(root) {
		return errors.New("managed_scope_unverifiable")
	}
	if _, err = guardRootIdentity(canonical); err != nil {
		return err
	}
	if same, inside, ancestor := sameOrNested(canonical, s.state); same || inside || ancestor {
		return errors.New("managed_scope_unverifiable")
	}
	layout, err := managedstate.Inspect(s.state, "cli")
	if err != nil || layout.Layout != "fenced" {
		return errors.New("managed_scope_unverifiable")
	}
	s.guardMu.Lock()
	defer s.guardMu.Unlock()
	digest := sha256.Sum256([]byte(canonical))
	id := "reader-" + hex.EncodeToString(digest[:16])
	return s.DB.Transaction(func(tx *gorm.DB) error {
		var spaces []Space
		if err := tx.Find(&spaces).Error; err != nil {
			return errors.New("guard_unavailable")
		}
		for _, space := range spaces {
			candidate, err := canonicalMutationPath(space.LocalRoot)
			if err != nil {
				return errors.New("managed_scope_unverifiable")
			}
			same, inside, ancestor := sameOrNested(canonical, candidate)
			if same {
				var adopt AdoptRecord
				if err := tx.Where("space_id = ? AND kind = ? AND consumer = ?", space.ID, "local", "pinax").First(&adopt).Error; err != nil || adopt.Location != canonical {
					return errors.New("managed_scope_unverifiable")
				}
				return nil
			}
			if inside || ancestor {
				return errors.New("managed_scope_unverifiable")
			}
		}
		now := time.Now().UTC()
		space := Space{ID: id, Backend: "reader-local", LocalRoot: canonical}
		if err := tx.Create(&space).Error; err != nil {
			return errors.New("guard_unavailable")
		}
		adopt := AdoptRecord{ID: id, Consumer: "pinax", Kind: "local", SpaceID: id, Location: canonical, IdempotencyKey: "reader-register:" + id, CreatedAt: now, UpdatedAt: now}
		if err := tx.Create(&adopt).Error; err != nil {
			return errors.New("guard_unavailable")
		}
		return nil
	})
}
