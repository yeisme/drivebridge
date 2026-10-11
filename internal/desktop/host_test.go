package desktop

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

func TestDesktopSessionHostPipeLifecycle(t *testing.T) {
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
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- ServeHost(ctx, input, output, executable) }()
	frames := make(chan HostEvent, 4)
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
	token := strings.Repeat("a", 64)
	encoder := json.NewEncoder(writer)
	if err := encoder.Encode(HostBootstrap{SchemaVersion: HostVersion, SessionNonce: strings.Repeat("n", 32), Bearer: token, BinaryDigest: binary.sha256, ManifestDigest: binary.manifestDigest}); err != nil {
		t.Fatal(err)
	}
	ready := read()
	if ready.Type != "ready" || ready.SessionNonce != strings.Repeat("n", 32) {
		t.Fatal("missing correlated ready")
	}
	f := &facade{address: ready.Address, token: token}
	status, _ := facadeRequest(t, f, "GET", "/v1/desktop/capabilities", "", nil)
	if status != 200 {
		t.Fatal("private bootstrap auth not usable")
	}
	vault := existingRoot(t)
	if err := encoder.Encode(HostSelection{SchemaVersion: HostVersion, Action: "select", RequestID: "select-one", NativePath: vault}); err != nil {
		t.Fatal(err)
	}
	selection := read()
	if selection.Type != "selection" || selection.SelectionID == "" || selection.RequestID != "select-one" {
		t.Fatal("selection not correlated")
	}
	wire, _ := json.Marshal(selection)
	if strings.Contains(string(wire), vault) || strings.Contains(string(wire), token) {
		t.Fatal("host event leaked private input")
	}
	if err := encoder.Encode(HostCancel{SchemaVersion: HostVersion, Action: "cancel", RequestID: "cancel-one", SelectionID: selection.SelectionID}); err != nil {
		t.Fatal(err)
	}
	if read().Type != "cancelled" {
		t.Fatal("cancel missing")
	}
	writer.Close()
	if read().Type != "shutdown" {
		t.Fatal("shutdown missing")
	}
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal("host did not exit")
	}
}
func TestDesktopSessionHostRejectsPrivateFrameAmbiguity(t *testing.T) {
	for _, frame := range []string{`{}`, `{"schema_version":null}`, `{"schema_version":"x","schema_version":"y"}`, `{"action":"exec","argv":["x"]}`, strings.Repeat("x", 65537)} {
		input, writer, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		reader, output, err := os.Pipe()
		if err != nil {
			t.Fatal(err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		done := make(chan error, 1)
		go func() { done <- ServeHost(ctx, input, output, "/not-used/drivebridge") }()
		go func() { _, _ = writer.WriteString(frame + "\n"); writer.Close() }()
		scanner := bufio.NewScanner(reader)
		if !scanner.Scan() || !strings.Contains(scanner.Text(), "desktop_host_invalid") {
			cancel()
			reader.Close()
			t.Fatal("invalid frame not rejected safely")
		}
		if err := <-done; err == nil {
			t.Fatal("bad host succeeded")
		}
		cancel()
		reader.Close()
	}
}
