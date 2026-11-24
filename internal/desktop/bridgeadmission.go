package desktop

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"path/filepath"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"github.com/yeisme/drivebridge/internal/bridge"
)

// BridgeAdmission adapts the local CLI owner guard to the host admission seam.
// It resolves the selected directory to an owner space by physical identity, so
// a renderer path is never used as authorization and an unregistered directory
// simply cannot be bound.
type BridgeAdmission struct {
	service  *bridge.Service
	instance string
}

func NewBridgeAdmission(service *bridge.Service) (*BridgeAdmission, error) {
	if service == nil {
		return nil, errors.New("guard_unavailable")
	}
	return &BridgeAdmission{service: service, instance: "cli"}, nil
}

func (a *BridgeAdmission) resolve(root string) (spaceID, sourceID string, err error) {
	canonical, err := filepath.EvalSymlinks(root)
	if err != nil {
		return "", "", errors.New("managed_scope_unverifiable")
	}
	for _, space := range a.service.Spaces() {
		candidate, resolveErr := filepath.EvalSymlinks(space.LocalRoot)
		if resolveErr != nil || candidate != canonical {
			continue
		}
		spaceID = space.ID
		break
	}
	if spaceID == "" {
		// A directory that no owner space registered cannot join a protection
		// domain. Registering it is an owner action, not a host inference.
		return "", "", errors.New("managed_scope_unverifiable")
	}
	sum := sha256.Sum256([]byte("drivebridge-desktop-source:" + canonical))
	return spaceID, "src-" + hex.EncodeToString(sum[:16]), nil
}

func (a *BridgeAdmission) Preview(root string, scopes []desktopcontract.ReaderScope) (AdmissionPreview, error) {
	spaceID, sourceID, err := a.resolve(root)
	if err != nil {
		return AdmissionPreview{}, err
	}
	preview, err := a.service.PreviewGuard(bridge.GuardPreviewRequest{InstanceID: a.instance, SourceID: sourceID, SpaceID: spaceID, Scopes: scopes})
	if err != nil {
		return AdmissionPreview{}, err
	}
	return AdmissionPreview{BindingID: preview.BindingID, Digest: preview.Digest, Decision: preview.Decision, Reason: preview.Reason, ExpectedGuardRevision: preview.ExpectedGuardRevision, OverlappingRoots: preview.OverlappingRoots, PendingMutations: preview.PendingMutations}, nil
}

func (a *BridgeAdmission) Confirm(root, bindingID string, expected uint64, digest string, scopes []desktopcontract.ReaderScope, key string) (AdmissionReceipt, error) {
	spaceID, sourceID, err := a.resolve(root)
	if err != nil {
		return AdmissionReceipt{}, err
	}
	receipt, err := a.service.InstallGuard(bridge.GuardInstallRequest{PreviewID: bindingID, Digest: digest, BindingID: bindingID, ExpectedGuardRevision: expected, InstanceID: a.instance, SourceID: sourceID, SpaceID: spaceID, Scopes: scopes, Confirmed: true, IdempotencyKey: key})
	if err != nil {
		return AdmissionReceipt{}, err
	}
	return AdmissionReceipt{OperationID: receipt.OperationID, BindingID: receipt.Binding.BindingID, SourceID: sourceID, InstanceID: a.instance, Revision: receipt.Binding.Revision, State: receipt.Binding.State, UpdatedAt: receipt.Binding.UpdatedAt, Epoch: receipt.Epoch}, nil
}

func (a *BridgeAdmission) SetVisibility(root, bindingID string, expected uint64, hidden bool) (AdmissionReceipt, error) {
	_, sourceID, err := a.resolve(root)
	if err != nil {
		return AdmissionReceipt{}, err
	}
	receipt, err := a.service.SetGuardVisibility(bridge.GuardRequest{BindingID: bindingID, ExpectedRevision: expected, Hidden: hidden, IdempotencyKey: "visibility:" + bindingID + ":" + itoaRevision(expected)})
	if err != nil {
		return AdmissionReceipt{}, err
	}
	return AdmissionReceipt{OperationID: receipt.OperationID, BindingID: receipt.Binding.BindingID, SourceID: sourceID, InstanceID: a.instance, Revision: receipt.Binding.Revision, State: receipt.Binding.State, UpdatedAt: receipt.Binding.UpdatedAt}, nil
}

func (a *BridgeAdmission) Release(root, bindingID string, expected uint64, key string) (AdmissionReceipt, error) {
	_, sourceID, err := a.resolve(root)
	if err != nil {
		return AdmissionReceipt{}, err
	}
	receipt, err := a.service.ReleaseGuard(bridge.GuardRequest{BindingID: bindingID, ExpectedRevision: expected, Confirmed: true, IdempotencyKey: key})
	if err != nil {
		return AdmissionReceipt{}, err
	}
	return AdmissionReceipt{OperationID: receipt.OperationID, BindingID: receipt.Binding.BindingID, SourceID: sourceID, InstanceID: a.instance, Revision: receipt.Binding.Revision, State: receipt.Binding.State, UpdatedAt: receipt.Binding.UpdatedAt}, nil
}

func (a *BridgeAdmission) Acquire(root, bindingID string, epoch uint64, scopes []string) (func() error, error) {
	_, sourceID, err := a.resolve(root)
	if err != nil {
		return nil, err
	}
	return a.service.AcquireGuardLease(bindingID, a.instance, sourceID, root, epoch, scopes)
}

func itoaRevision(value uint64) string {
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	pos := len(buf)
	for value > 0 {
		pos--
		buf[pos] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[pos:])
}
