package desktop

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

// The host may attach the owner guard only after the private bootstrap frame
// validated, and the attached admission must be what the loopback facade actually
// uses. This exercises the real host process loop, the real facade HTTP surface
// and a real fenced owner state — no mock guard and no injected ready state.
func TestDesktopHostAttachesOwnerAdmissionAfterBootstrap(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("managed guard identity is Linux-only until platform acceptance")
	}
	admission, root := newAdmissionService(t)
	binary := testBinary(t)
	name := "pinax"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	packaged := filepath.Join(filepath.Dir(binary.path), name)
	if err := os.Rename(binary.path, packaged); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(filepath.Dir(packaged), "drivebridge")
	input, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	reader, output, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	opened := make(chan struct{})
	go func() {
		done <- ServeHostWithAdmission(ctx, input, output, executable, func() (Admission, func(), error) {
			close(opened)
			return admission, nil, nil
		})
	}()
	frames := make(chan HostEvent, 8)
	go func() {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			var event HostEvent
			if json.Unmarshal(scanner.Bytes(), &event) == nil {
				frames <- event
			}
		}
		close(frames)
	}()
	read := func() HostEvent {
		t.Helper()
		select {
		case event := <-frames:
			return event
		case <-ctx.Done():
			t.Fatal("host event timeout")
			return HostEvent{}
		}
	}
	select {
	case <-opened:
		t.Fatal("owner admission was opened before the bootstrap frame")
	default:
	}
	token := strings.Repeat("b", 64)
	encoder := json.NewEncoder(writer)
	if err := encoder.Encode(HostBootstrap{SchemaVersion: HostVersion, SessionNonce: strings.Repeat("s", 32), Bearer: token, BinaryDigest: binary.sha256, ManifestDigest: binary.manifestDigest}); err != nil {
		t.Fatal(err)
	}
	ready := read()
	if ready.Type != "ready" {
		t.Fatalf("ready event: %#v", ready)
	}
	select {
	case <-opened:
	case <-ctx.Done():
		t.Fatal("owner admission was never opened after a valid bootstrap")
	}
	// Register the native selection through the private frame channel.
	if err := encoder.Encode(HostSelection{SchemaVersion: HostVersion, Action: "select", RequestID: "select-admission", NativePath: root}); err != nil {
		t.Fatal(err)
	}
	selection := read()
	if selection.Type != "selection" || selection.SelectionID == "" {
		t.Fatalf("selection event: %#v", selection)
	}
	// The real loopback facade must now answer a ready preview, which is only
	// possible because the host actually attached the owner admission.
	body, _ := json.Marshal(map[string]string{"native_selection_id": selection.SelectionID})
	request, err := http.NewRequest("POST", ready.Address+"/v1/desktop/bindings/preview", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 8 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var envelope struct {
		Data struct {
			Decision              string `json:"decision"`
			Reason                string `json:"reason"`
			ExpectedGuardRevision uint64 `json:"expected_guard_revision"`
			PreviewID             string `json:"preview_id"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	view := envelope.Data
	// The owner guard answered: the preview id is the owner's binding identity,
	// which a deny-only host can never produce. The fake owner binary cannot serve
	// ReaderSource, so the probe gate still reports owner_unavailable; the full
	// `ready` path with a real Pinax binary is covered by the integration test.
	if response.StatusCode != 200 || !strings.HasPrefix(view.PreviewID, "guard-") || view.Reason != "owner_unavailable" {
		t.Fatalf("admission was not attached before the owner probe: %d %#v", response.StatusCode, view)
	}
	writer.Close()
	for {
		event := read()
		if event.Type == "shutdown" {
			break
		}
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("host exit: %v", err)
		}
	case <-ctx.Done():
		t.Fatal("host did not exit")
	}
}

// Without a factory the host stays deny-only: the same facade request must come
// back blocked rather than borrowing an admission from anywhere else.
func TestDesktopHostWithoutAdmissionStaysDenyOnly(t *testing.T) {
	if runtime.GOOS != "linux" {
		t.Skip("managed guard identity is Linux-only until platform acceptance")
	}
	binary := testBinary(t)
	name := "pinax"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	packaged := filepath.Join(filepath.Dir(binary.path), name)
	if err := os.Rename(binary.path, packaged); err != nil {
		t.Fatal(err)
	}
	executable := filepath.Join(filepath.Dir(packaged), "drivebridge")
	vault := t.TempDir()
	input, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer writer.Close()
	reader, output, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- ServeHost(ctx, input, output, executable) }()
	frames := make(chan HostEvent, 8)
	go func() {
		scanner := bufio.NewScanner(reader)
		for scanner.Scan() {
			var event HostEvent
			if json.Unmarshal(scanner.Bytes(), &event) == nil {
				frames <- event
			}
		}
		close(frames)
	}()
	read := func() HostEvent {
		t.Helper()
		select {
		case event := <-frames:
			return event
		case <-ctx.Done():
			t.Fatal("host event timeout")
			return HostEvent{}
		}
	}
	token := strings.Repeat("c", 64)
	encoder := json.NewEncoder(writer)
	if err := encoder.Encode(HostBootstrap{SchemaVersion: HostVersion, SessionNonce: strings.Repeat("t", 32), Bearer: token, BinaryDigest: binary.sha256, ManifestDigest: binary.manifestDigest}); err != nil {
		t.Fatal(err)
	}
	ready := read()
	if ready.Type != "ready" {
		t.Fatalf("ready event: %#v", ready)
	}
	if err := encoder.Encode(HostSelection{SchemaVersion: HostVersion, Action: "select", RequestID: "select-deny", NativePath: vault}); err != nil {
		t.Fatal(err)
	}
	selection := read()
	if selection.Type != "selection" {
		t.Fatalf("selection event: %#v", selection)
	}
	body, _ := json.Marshal(map[string]string{"native_selection_id": selection.SelectionID})
	request, err := http.NewRequest("POST", ready.Address+"/v1/desktop/bindings/preview", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Authorization", "Bearer "+token)
	request.Header.Set("Content-Type", "application/json")
	client := &http.Client{Transport: &http.Transport{Proxy: nil}, Timeout: 8 * time.Second}
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var envelope struct {
		Data struct {
			Decision string `json:"decision"`
			Reason   string `json:"reason"`
		} `json:"data"`
	}
	if err := json.NewDecoder(response.Body).Decode(&envelope); err != nil {
		t.Fatal(err)
	}
	view := envelope.Data
	if view.Decision != "blocked" || view.Reason == "none" {
		t.Fatalf("deny-only host produced a ready preview: %#v", view)
	}
	writer.Close()
	for {
		if read().Type == "shutdown" {
			break
		}
	}
	<-done
}
