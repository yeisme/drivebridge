package desktop

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"path/filepath"
	"runtime"
	"time"

	"github.com/invopop/jsonschema"
)

const HostVersion = "drivebridge.desktop.host.v1alpha1"

// HostBootstrap is private stdin input from the trusted native main process.
// Digests are trusted package metadata, not claims from renderer requests.
type HostBootstrap struct {
	SchemaVersion  string `json:"schema_version" jsonschema:"enum=drivebridge.desktop.host.v1alpha1"`
	SessionNonce   string `json:"session_nonce" jsonschema:"minLength=16,maxLength=128"`
	Bearer         string `json:"bearer" jsonschema:"pattern=^[a-f0-9]{64}$"`
	BinaryDigest   string `json:"binary_digest" jsonschema:"pattern=^sha256:[a-f0-9]{64}$"`
	ManifestDigest string `json:"manifest_digest" jsonschema:"pattern=^sha256:[a-f0-9]{64}$"`
}
type HostSelection struct {
	SchemaVersion string `json:"schema_version" jsonschema:"enum=drivebridge.desktop.host.v1alpha1"`
	Action        string `json:"action" jsonschema:"enum=select"`
	RequestID     string `json:"request_id" jsonschema:"minLength=1,maxLength=128"`
	NativePath    string `json:"native_path" jsonschema:"minLength=1,maxLength=32768"`
}
type HostCancel struct {
	SchemaVersion string `json:"schema_version" jsonschema:"enum=drivebridge.desktop.host.v1alpha1"`
	Action        string `json:"action" jsonschema:"enum=cancel"`
	RequestID     string `json:"request_id" jsonschema:"minLength=1,maxLength=128"`
	SelectionID   string `json:"selection_id" jsonschema:"minLength=1,maxLength=128"`
}
type HostEvent struct {
	SchemaVersion string `json:"schema_version"`
	Type          string `json:"type"`
	SessionNonce  string `json:"session_nonce,omitempty"`
	Address       string `json:"address,omitempty"`
	RequestID     string `json:"request_id,omitempty"`
	SelectionID   string `json:"selection_id,omitempty"`
	Code          string `json:"code,omitempty"`
}

// HostSchemas is a separate private host contract export. It is intentionally
// absent from web generated artifacts and public desktop DTO profiles.
func HostSchemas() map[string]*jsonschema.Schema {
	r := jsonschema.Reflector{Anonymous: true, DoNotReference: true}
	return map[string]*jsonschema.Schema{"HostBootstrap": r.Reflect(HostBootstrap{}), "HostSelection": r.Reflect(HostSelection{}), "HostCancel": r.Reflect(HostCancel{}), "HostEvent": r.Reflect(HostEvent{})}
}

// ServeHost owns these private pipes until EOF/cancellation. executable is the
// DriveBridge executable path supplied by the CLI composition root. Only the
// fixed sibling pinax binary may be started, never a frame-specified command.
func ServeHost(ctx context.Context, input io.ReadCloser, output io.WriteCloser, executable string) error {
	if ctx == nil {
		return errors.New("desktop_session_invalid")
	}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	go func() { <-ctx.Done(); _ = input.Close(); _ = output.Close() }()
	emit := func(event HostEvent) error {
		event.SchemaVersion = HostVersion
		// A stalled host reader must not keep the facade alive indefinitely.
		deadline := time.AfterFunc(3*time.Second, func() { cancel(); _ = output.Close() })
		defer deadline.Stop()
		return json.NewEncoder(output).Encode(event)
	}
	failure := func() error {
		_ = emit(HostEvent{Type: "error", Code: "desktop_host_invalid"})
		return errors.New("desktop_host_invalid")
	}
	scanner := bufio.NewScanner(input)
	scanner.Buffer(make([]byte, 1024), 64*1024)
	type frame struct {
		data []byte
		err  error
	}
	frames := make(chan frame)
	go func() {
		defer close(frames)
		for scanner.Scan() {
			select {
			case frames <- frame{data: append([]byte(nil), scanner.Bytes()...)}:
			case <-ctx.Done():
				return
			}
		}
		if scanner.Err() != nil {
			select {
			case frames <- frame{err: errors.New("desktop_host_invalid")}:
			case <-ctx.Done():
			}
		}
	}()
	var first frame
	timer := time.NewTimer(10 * time.Second)
	defer timer.Stop()
	select {
	case value, ok := <-frames:
		if !ok {
			return failure()
		}
		first = value
	case <-timer.C:
		return failure()
	case <-ctx.Done():
		return errors.New("desktop_session_invalid")
	}
	var bootstrap HostBootstrap
	if first.err != nil || closedHostJSON(first.data, &bootstrap, []string{"schema_version", "session_nonce", "bearer", "binary_digest", "manifest_digest"}) != nil || bootstrap.SchemaVersion != HostVersion || len(bootstrap.SessionNonce) < 16 || len(bootstrap.SessionNonce) > 128 || !facadeKey.MatchString(bootstrap.SessionNonce) || !fixedDigest("sha256:"+bootstrap.Bearer) || !fixedDigest(bootstrap.BinaryDigest) || !fixedDigest(bootstrap.ManifestDigest) {
		return failure()
	}
	name := "pinax"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	binary := ownerBinary{path: filepath.Join(filepath.Dir(executable), name), sha256: bootstrap.BinaryDigest, manifestDigest: bootstrap.ManifestDigest}
	f, err := startFacadeWithToken(ctx, binary, bootstrap.Bearer)
	if err != nil {
		return failure()
	}
	defer f.Close()
	if emit(HostEvent{Type: "ready", SessionNonce: bootstrap.SessionNonce, Address: f.address}) != nil {
		return errors.New("desktop_host_closed")
	}
	for {
		select {
		case <-ctx.Done():
			return errors.New("desktop_session_invalid")
		case next, ok := <-frames:
			if !ok {
				f.Close()
				return emit(HostEvent{Type: "shutdown", SessionNonce: bootstrap.SessionNonce})
			}
			if next.err != nil {
				return failure()
			}
			var action struct {
				Action string `json:"action"`
			}
			if json.Unmarshal(next.data, &action) != nil {
				return failure()
			}
			switch action.Action {
			case "select":
				var request HostSelection
				if closedHostJSON(next.data, &request, []string{"schema_version", "action", "request_id", "native_path"}) != nil || request.SchemaVersion != HostVersion || !facadeKey.MatchString(request.RequestID) {
					return failure()
				}
				ticket, err := f.preflight.Select(request.NativePath)
				event := HostEvent{Type: "selection", RequestID: request.RequestID, SelectionID: ticket.NativeSelectionID}
				if err != nil {
					event.Type = "error"
					event.Code = "native_selection_invalid"
				}
				if emit(event) != nil {
					return errors.New("desktop_host_closed")
				}
			case "cancel":
				var request HostCancel
				if closedHostJSON(next.data, &request, []string{"schema_version", "action", "request_id", "selection_id"}) != nil || request.SchemaVersion != HostVersion || !facadeKey.MatchString(request.RequestID) || !facadeKey.MatchString(request.SelectionID) {
					return failure()
				}
				f.preflight.Cancel(request.SelectionID)
				if emit(HostEvent{Type: "cancelled", RequestID: request.RequestID}) != nil {
					return errors.New("desktop_host_closed")
				}
			default:
				return failure()
			}
		}
	}
}

func closedHostJSON(data []byte, target any, fields []string) error {
	fail := func() error { return errors.New("desktop_host_invalid") }
	d := json.NewDecoder(bytes.NewReader(data))
	token, err := d.Token()
	if err != nil || token != json.Delim('{') {
		return fail()
	}
	allowed := map[string]bool{}
	for _, field := range fields {
		allowed[field] = true
	}
	seen := map[string]bool{}
	for d.More() {
		key, err := d.Token()
		if err != nil {
			return fail()
		}
		name, ok := key.(string)
		if !ok || !allowed[name] || seen[name] {
			return fail()
		}
		seen[name] = true
		var value json.RawMessage
		if d.Decode(&value) != nil || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return fail()
		}
	}
	if len(seen) != len(fields) {
		return fail()
	}
	if _, err := d.Token(); err != nil {
		return fail()
	}
	if _, err := d.Token(); err != io.EOF {
		return fail()
	}
	return json.Unmarshal(data, target)
}
