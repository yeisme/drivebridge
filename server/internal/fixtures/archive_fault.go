package fixtures

import (
	"errors"
	"sync/atomic"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

// installArchivePublishFailure is test-only: the first archive publication
// audit fails inside its real transaction. No HTTP arming endpoint is exposed.
// Keeping the callback installed avoids mutating GORM's callback registry while
// requests run. The atomic guard makes the failure one-shot across requests.
func installArchivePublishFailure(db *gorm.DB) error {
	var fired atomic.Bool
	return db.Callback().Create().Before("gorm:create").Register("fixture_archive_publish_once", func(tx *gorm.DB) {
		if audit, ok := tx.Statement.Dest.(*service.Audit); ok && audit.Action == "workspace.submission.archived" && fired.CompareAndSwap(false, true) {
			tx.AddError(errors.New("fixture archive publication failure"))
		}
	})
}
