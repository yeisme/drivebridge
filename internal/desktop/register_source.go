package desktop

import (
	"context"
	"errors"
	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"github.com/yeisme/pinax/pkg/pinaxclient"
)

// Only the native private pipe reaches this method, after an explicit native
// confirmation. Metadata is obtained from the public owner before registration.
func (f *facade) registerSource(ctx context.Context, path string) (desktopcontract.NativeSelection, error) {
	empty := desktopcontract.NativeSelection{}
	identity, err := directoryIdentity(path)
	if err != nil {
		return empty, err
	}
	selected := selection{path: path, identity: identity}
	adapter, ok := f.preflight.admission.(*BridgeAdmission)
	if !ok {
		return empty, errors.New("owner_unavailable")
	}
	child, err := startMetadataProcess(ctx, f.binary, path)
	if err != nil {
		return empty, err
	}
	client, err := pinaxclient.New(pinaxclient.Config{BaseURL: child.ready.Address, Token: child.bearer})
	if err == nil {
		var source pinaxclient.ReaderSourceDescriptor
		source, err = client.ReaderSource(ctx)
		if err == nil && (source.SessionID != child.ready.SessionID || source.BindingID != child.ready.BindingID) {
			err = errors.New("owner_unavailable")
		}
	}
	closeErr := child.Close()
	if err != nil || closeErr != nil {
		return empty, errors.New("owner_unavailable")
	}
	if err = selected.recheck(); err != nil {
		return empty, err
	}
	if err = adapter.service.RegisterReaderRoot(path); err != nil {
		return empty, err
	}
	if err = selected.recheck(); err != nil {
		return empty, err
	}
	return f.preflight.Select(path)
}
