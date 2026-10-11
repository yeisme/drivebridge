package desktop

import (
	"bytes"
	"context"
	"crypto/subtle"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"regexp"
	"sync"
	"time"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"github.com/yeisme/pinax/pkg/pinaxclient"
)

// facade is private composition state, not a renderer-accessible native API.
// A transport for native selection registration is intentionally not exposed:
// only the trusted host may call preflight.Select directly at this stage.
type facade struct {
	preflight            *Preflight
	binary               ownerBinary
	token, address, host string
	server               *http.Server
	cancel               context.CancelFunc
	slot                 chan struct{}
	done                 chan struct{}
	once                 sync.Once
}

func startFacade(parent context.Context, binary ownerBinary) (*facade, error) {
	token, err := opaqueID()
	if err != nil {
		return nil, err
	}
	return startFacadeWithToken(parent, binary, token)
}

func startFacadeWithToken(parent context.Context, binary ownerBinary, token string) (*facade, error) {
	if !fixedDigest("sha256:" + token) {
		return nil, errors.New("desktop_session_invalid")
	}
	if parent == nil || parent.Err() != nil {
		return nil, errors.New("desktop_session_invalid")
	}
	if err := verifyOwnerBinary(binary); err != nil {
		return nil, err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		return nil, errors.New("owner_unavailable")
	}
	ctx, cancel := context.WithCancel(parent)
	f := &facade{preflight: NewPreflight(), binary: binary, token: token, host: listener.Addr().String(), address: "http://" + listener.Addr().String(), cancel: cancel, slot: make(chan struct{}, 1), done: make(chan struct{})}
	f.server = &http.Server{Handler: http.HandlerFunc(f.handle), ReadHeaderTimeout: 3 * time.Second, ReadTimeout: 5 * time.Second, WriteTimeout: 25 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 8 * 1024, BaseContext: func(net.Listener) context.Context { return ctx }}
	go func() { defer close(f.done); _ = f.server.Serve(listener) }()
	go func() {
		select {
		case <-ctx.Done():
			f.Close()
		case <-f.done:
			f.Close()
		}
	}()
	return f, nil
}
func (f *facade) Close() {
	f.once.Do(func() {
		f.cancel()
		f.preflight.Close()
		ctx, cancel := context.WithTimeout(context.Background(), 7*time.Second)
		defer cancel()
		if f.server.Shutdown(ctx) != nil {
			_ = f.server.Close()
		}
	})
}
func (f *facade) handle(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "private, no-store")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Type", "application/json")
	peer, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil || peer != "127.0.0.1" || r.Host != f.host || len(r.Header.Values("Origin")) != 0 || len(r.Header.Values("Sec-Fetch-Site")) != 0 || len(r.Header.Values("Authorization")) != 1 || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+f.token)) != 1 || r.Context().Err() != nil {
		facadeError(w, "desktop_session_invalid")
		return
	}
	if r.URL.RawQuery != "" || r.URL.ForceQuery || r.URL.RawPath != "" {
		facadeError(w, "request_invalid")
		return
	}
	switch {
	case r.Method == "GET" && r.URL.Path == "/v1/desktop/capabilities":
		if r.ContentLength != 0 || len(r.TransferEncoding) != 0 {
			facadeError(w, "request_invalid")
			return
		}
		// Confirmation and content reads are not usable until guard admission.
		facadeData(w, desktopcontract.DesktopCapabilities{SchemaVersion: desktopcontract.Version, Available: []string{"capabilities", "chooseVault"}})
	case r.Method == "POST" && r.URL.Path == "/v1/desktop/bindings/preview":
		var input desktopcontract.NativeSelection
		if decodeFacadeRequest(w, r, &input, []string{"native_selection_id"}) != nil || !facadeKey.MatchString(input.NativeSelectionID) {
			facadeError(w, "request_invalid")
			return
		}
		// Never let callers queue unbounded subprocess starts or consume a ticket
		// before the request has acquired the single metadata probe slot.
		select {
		case f.slot <- struct{}{}:
			defer func() { <-f.slot }()
		default:
			facadeError(w, "managed_scope_busy")
			return
		}
		view, err := f.preflight.Preview(input)
		if err != nil {
			facadeError(w, err.Error())
			return
		}
		probeCtx, cancel := context.WithDeadline(r.Context(), view.ExpiresAt)
		view = f.inspectOwner(probeCtx, view)
		cancel()
		f.preflight.mu.Lock()
		_, retained := f.preflight.previews[view.PreviewID]
		valid := retained && !f.preflight.closed && f.preflight.now().Before(view.ExpiresAt)
		f.preflight.mu.Unlock()
		if !valid || r.Context().Err() != nil {
			f.preflight.Cancel(view.PreviewID)
			facadeError(w, "binding_preview_expired")
			return
		}
		facadeData(w, view)
	case r.Method == "POST" && r.URL.Path == "/v1/desktop/bindings":
		if len(r.Header.Values("Idempotency-Key")) != 1 || !facadeKey.MatchString(r.Header.Get("Idempotency-Key")) {
			facadeError(w, "request_invalid")
			return
		}
		var input desktopcontract.ConfirmBinding
		if decodeFacadeRequest(w, r, &input, []string{"preview_id", "preview_digest", "expected_guard_revision", "confirmed"}) != nil {
			facadeError(w, "request_invalid")
			return
		}
		// No mutation is accepted or persisted. A denied confirmation cannot
		// become a receipt, even if its caller supplies a valid original key.
		err := f.preflight.ConfirmBlocked(input)
		if err == nil {
			facadeError(w, "guard_unavailable")
			return
		}
		facadeError(w, err.Error())
	default:
		facadeError(w, "desktop_contract_unsupported")
	}
}

func (f *facade) inspectOwner(ctx context.Context, view desktopcontract.BindingPreview) desktopcontract.BindingPreview {
	f.preflight.mu.Lock()
	pending, ok := f.preflight.previews[view.PreviewID]
	f.preflight.mu.Unlock()
	if !ok || pending.selection.recheck() != nil {
		view.Reason = "owner_unavailable"
		return view
	}
	child, err := startMetadataProcess(ctx, f.binary, pending.selection.path)
	if err != nil {
		view.Reason = "owner_unavailable"
		return view
	}
	client, err := pinaxclient.New(pinaxclient.Config{BaseURL: child.ready.Address, Token: child.bearer, Timeout: 3 * time.Second})
	if err == nil {
		var source pinaxclient.ReaderSourceDescriptor
		source, err = client.ReaderSource(ctx)
		if err == nil && (source.SessionID != child.ready.SessionID || source.BindingID != child.ready.BindingID) {
			err = errors.New("owner_unavailable")
		}
	}
	closeErr := child.Close()
	if err != nil || closeErr != nil || pending.selection.recheck() != nil {
		view.Reason = "owner_unavailable"
	}
	// Do not return owner display names, paths, diagnostics, or session tokens.
	// Success leaves the original guard refusal intact.
	f.preflight.mu.Lock()
	defer f.preflight.mu.Unlock()
	if current, exists := f.preflight.previews[view.PreviewID]; exists && !f.preflight.closed {
		current.view.Reason = view.Reason
		f.preflight.previews[view.PreviewID] = current
	}
	return view
}

var facadeKey = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$`)

// These flat host requests reject duplicate/case-variant/null keys as well as
// unknown fields. encoding/json's permissive last-key-wins behavior is not
// suitable for confirmation or private selection identities.
func decodeFacadeRequest(w http.ResponseWriter, r *http.Request, target any, fields []string) error {
	fail := func() error { return errors.New("request_invalid") }
	content, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || content != "application/json" || len(r.Header.Values("Content-Type")) != 1 || len(r.Header.Values("Content-Encoding")) != 0 {
		return fail()
	}
	data, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 128*1024))
	if err != nil {
		return fail()
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	token, err := decoder.Token()
	if err != nil || token != json.Delim('{') {
		return fail()
	}
	allowed := map[string]bool{}
	for _, field := range fields {
		allowed[field] = true
	}
	seen := map[string]bool{}
	for decoder.More() {
		key, err := decoder.Token()
		if err != nil {
			return fail()
		}
		name, ok := key.(string)
		if !ok || !allowed[name] || seen[name] {
			return fail()
		}
		seen[name] = true
		var value json.RawMessage
		if decoder.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fail()
		}
	}
	if len(seen) != len(fields) {
		return fail()
	}
	if _, err := decoder.Token(); err != nil {
		return fail()
	}
	if _, err := decoder.Token(); err != io.EOF {
		return fail()
	}
	decoder = json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	if decoder.Decode(target) != nil {
		return fail()
	}
	return nil
}
func facadeError(w http.ResponseWriter, code string) {
	value, status := desktopcontract.SafeError("drivebridge", code, "")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error desktopcontract.DesktopError `json:"error"`
	}{value})
}
func facadeData(w http.ResponseWriter, data any) {
	_ = json.NewEncoder(w).Encode(struct {
		Data any `json:"data"`
	}{data})
}
