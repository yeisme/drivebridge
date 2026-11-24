package desktop

import (
	"context"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/yeisme/pinax/pkg/pinaxclient"
)

func TestDesktopOwnerCallRefusesExternallyReleasedBindingBeforeSpawn(t *testing.T) {
	admission, root := newAdmissionService(t)
	p := NewPreflightWithAdmission(admission)
	defer p.Close()
	f := &admissionFixture{root: root, preflight: p}
	view := f.preview(t)
	confirmed, err := p.Confirm(confirmation(view), "confirm")
	if err != nil {
		t.Fatal(err)
	}
	// 另一 owner 会话释放后，host 内存仍有 source；必须在启动任何 Pinax 子进程前拒绝。
	if _, err := admission.Release(root, confirmed.Binding.BindingID, 1, "external-release"); err != nil {
		t.Fatal(err)
	}
	called := false
	facade := &facade{preflight: p} // no owner binary: reaching spawn would be a different failure
	err = facade.withOwner(context.Background(), confirmed.Binding.SourceID, []string{"reader.metadata"}, true, func(context.Context, *pinaxclient.Client) error {
		called = true
		return nil
	})
	if err == nil || err.Error() != "protection_released" || called {
		t.Fatalf("revoked call: %v, called=%v", err, called)
	}
	response := httptest.NewRecorder()
	facadeCallError(response, err)
	if response.Code != 409 || !strings.Contains(response.Body.String(), `"origin":"drivebridge"`) || !strings.Contains(response.Body.String(), `"code":"protection_released"`) {
		t.Fatalf("guard error lost its origin: %d %s", response.Code, response.Body.String())
	}
}

func TestDesktopOwnerCallRejectsUnboundWriterScope(t *testing.T) {
	admission, root := newAdmissionService(t)
	p := NewPreflightWithAdmission(admission)
	defer p.Close()
	f := &admissionFixture{root: root, preflight: p}
	view := f.preview(t)
	confirmed, err := p.Confirm(confirmation(view), "confirm")
	if err != nil {
		t.Fatal(err)
	}
	called := false
	facade := &facade{preflight: p}
	err = facade.withOwner(context.Background(), confirmed.Binding.SourceID, []string{"writer.editor"}, false, func(context.Context, *pinaxclient.Client) error {
		called = true
		return nil
	})
	if err == nil || err.Error() != "session_request_invalid" || called {
		t.Fatalf("writer upgrade: %v, called=%v", err, called)
	}
}

// Existing owner conflict codes must keep their Pinax origin after adding the
// host-side lease gate.
func TestDesktopOwnerCallErrorKeepsOwnerOrigin(t *testing.T) {
	response := httptest.NewRecorder()
	facadeCallError(response, &pinaxclient.Error{Code: "revision_conflict"})
	if response.Code != 409 || !strings.Contains(response.Body.String(), `"origin":"pinax"`) || !strings.Contains(response.Body.String(), `"code":"revision_conflict"`) {
		t.Fatalf("owner error: %d %s", response.Code, response.Body.String())
	}
}
