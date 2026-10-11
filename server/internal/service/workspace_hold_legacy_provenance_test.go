package service_test

import (
	"errors"
	"testing"

	"github.com/yeisme/drivebridge-server/internal/service"
	"gorm.io/gorm"
)

func TestWorkspaceHoldLegacyProvenanceFailureDeniesRelease(t *testing.T) {
	f := rig(t)
	h, _ := login(t, f, "alice")
	p := service.Principal{UserID: h.user.ID}
	op := fpUploadBytes(t, mustSDK(t, h), h.space.ID, "held.txt", "upload", []byte("held"))
	hold, err := f.Service.CreateRetentionHold(p, service.RetentionHoldRequest{FileID: op.FileID, Purpose: "legal_hold", SourceRef: op.ID}, "legacy")
	if err != nil { t.Fatal(err) }
	hits := 0
	if err = f.Service.DB.Callback().Query().Before("gorm:query").Register("hold_provenance_failure", func(tx *gorm.DB) {
		if tx.Statement.Table == "workspace_requests" { hits++; tx.AddError(errors.New("synthetic provenance failure")) }
	}); err != nil { t.Fatal(err) }
	_, releaseErr := f.Service.ReleaseRetentionHold(p, hold.HoldID)
	if err = f.Service.DB.Callback().Query().Remove("hold_provenance_failure"); err != nil { t.Fatal(err) }
	if hits == 0 || releaseErr == nil || releaseErr.Error() != "database_unavailable" { t.Fatal(hits, releaseErr) }
	var row service.RetentionHold
	if err = f.Service.DB.First(&row, "id = ?", hold.HoldID).Error; err != nil || row.State != "active" || row.Revision != 1 { t.Fatal("release without proven legacy origin", row, err) }
}
