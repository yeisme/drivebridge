package desktop

import (
	"github.com/yeisme/drivebridge/internal/bridge"
	"os"
	"path/filepath"
	"testing"
)

func TestNativeSourceRegistrationPreservesOwnerAndRefusesOverlap(t *testing.T) {
	admission, existing := newAdmissionService(t)
	root := existingRoot(t)
	if err := admission.service.RegisterReaderRoot(root); err != nil {
		t.Fatal(err)
	}
	if err := admission.service.RegisterReaderRoot(root); err != nil {
		t.Fatal("registration replay failed")
	}
	child := filepath.Join(root, "nested")
	if err := os.Mkdir(child, 0700); err != nil {
		t.Fatal(err)
	}
	if err := admission.service.RegisterReaderRoot(child); err == nil {
		t.Fatal("overlapping source admitted")
	}
	var spaces []bridge.Space
	if err := admission.service.DB.Where("local_root = ?", root).Find(&spaces).Error; err != nil || len(spaces) != 1 || spaces[0].Kind() != "reader-local" || spaces[0].Remote != "" {
		t.Fatal("reader registration invented provider capability")
	}
	var adoptions []bridge.AdoptRecord
	if err := admission.service.DB.Where("space_id = ?", spaces[0].ID).Find(&adoptions).Error; err != nil || len(adoptions) != 1 || adoptions[0].Consumer != "pinax" {
		t.Fatal("reader root lost owner adoption")
	}
	if err := admission.service.RegisterReaderRoot(existing); err != nil {
		t.Fatal("existing exact owner adoption should be preserved")
	}
	p := NewPreflightWithAdmission(admission)
	defer p.Close()
	fixture := &admissionFixture{root: root, preflight: p}
	preview := fixture.preview(t)
	receipt, err := p.Confirm(confirmation(preview), "native-register-confirm")
	if err != nil || !receipt.Binding.ProtectionActive {
		t.Fatal("registered source not protected", err)
	}
	if _, err = admission.service.AcquireGuardLease(receipt.Binding.BindingID, "cli", receipt.Binding.SourceID, root, p.bindings[receipt.Binding.BindingID].Epoch, []string{"writer.editor"}); err == nil || err.Error() != "session_request_invalid" {
		t.Fatal("source registration granted writer authority")
	}
}
