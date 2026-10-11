package desktop

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/yeisme/pinax/pkg/localsession"
)

// TestMain lets the already-compiled test binary act as a protocol subprocess.
// It is not Pinax and proves lifecycle mechanics, not owner integration.
func TestMain(m *testing.M) {
	if len(os.Args) == 4 && os.Args[1] == "api" && os.Args[2] == "serve" && os.Args[3] == "--session-stdin" {
		reader := bufio.NewReader(os.Stdin)
		line, err := reader.ReadBytes('\n')
		if err != nil {
			os.Exit(2)
		}
		var frame localsession.Bootstrap
		if json.Unmarshal(line, &frame) != nil {
			os.Exit(3)
		}
		if !frame.Readonly || len(frame.Scopes) != 1 || frame.Scopes[0] != "reader.metadata" || os.Getenv("PINAX_PRIVATE_TEST") != "" {
			os.Exit(4)
		}
		mode := filepath.Base(frame.VaultRoot)
		if mode == "exit" {
			os.Exit(5)
		}
		if mode == "hang" {
			time.Sleep(time.Hour)
			os.Exit(0)
		}
		readonly := true
		event := localsession.Event{SchemaVersion: localsession.EventVersion, Type: "ready", SessionID: "rss_test", SessionNonce: frame.SessionNonce, BindingID: frame.BindingID, Readonly: &readonly, Address: "http://127.0.0.1:12345", ContractDigest: "sha256:" + strings.Repeat("a", 64)}
		if mode == "bad" {
			event.SessionNonce = "wrong"
		}
		if mode == "digest" {
			event.ContractDigest = "sha256:" + strings.Repeat("b", 64)
		}
		if mode == "address" {
			event.Address = "http://10.0.0.1:12345"
		}
		if mode == "huge" {
			_, _ = os.Stdout.WriteString(strings.Repeat("x", localsession.MaxFrameBytes+10) + "\n")
		} else {
			_ = json.NewEncoder(os.Stdout).Encode(event)
		}
		_, _ = io.Copy(io.Discard, reader)
		if mode == "stubborn" {
			time.Sleep(time.Hour)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
func testBinary(t *testing.T) ownerBinary {
	t.Helper()
	path, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	path, err = filepath.EvalSymlinks(path)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	// go test's build output permissions depend on the invoking umask. Model
	// a packaged private executable instead of weakening the trust check.
	path = filepath.Join(existingRoot(t), "owner-test")
	if err := os.WriteFile(path, data, 0700); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(data)
	return ownerBinary{path: path, sha256: "sha256:" + hex.EncodeToString(sum[:]), manifestDigest: "sha256:" + strings.Repeat("a", 64)}
}
func processVault(t *testing.T, mode string) string {
	t.Helper()
	path := filepath.Join(existingRoot(t), mode)
	if err := os.Mkdir(path, 0700); err != nil {
		t.Fatal(err)
	}
	return path
}
func TestDesktopSessionProcessMetadataAndEOF(t *testing.T) {
	t.Setenv("PINAX_PRIVATE_TEST", "must-not-inherit")
	p, err := startMetadataProcess(context.Background(), testBinary(t), processVault(t, "ok"))
	if err != nil {
		t.Fatal(err)
	}
	if p.ready.Readonly == nil || !*p.ready.Readonly || p.bearer == "" {
		t.Fatal("bad session")
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	if err := p.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-p.done:
	default:
		t.Fatal("child not reaped")
	}
	encoded, err := json.Marshal(p)
	if err != nil || string(encoded) != "{}" {
		t.Fatal("private state serializable")
	}
}
func TestDesktopSessionProcessRejectsBadHandshake(t *testing.T) {
	binary := testBinary(t)
	for _, mode := range []string{"bad", "huge", "exit", "hang", "digest", "address"} {
		t.Run(mode, func(t *testing.T) {
			p, err := startMetadataProcessWithin(context.Background(), binary, processVault(t, mode), 200*time.Millisecond, 100*time.Millisecond)
			if p != nil || err == nil {
				t.Fatal("invalid owner accepted")
			}
			if err.Error() == "owner_binary_invalid" || err.Error() == "session_cleanup_failed" {
				t.Fatal("test did not exercise handshake rejection")
			}
			if mode == "hang" && err.Error() != "session_ready_timeout" {
				t.Fatal("timeout mapping drift")
			}
			if strings.Contains(err.Error(), binary.path) {
				t.Fatal("private path leaked")
			}
		})
	}
}
func TestDesktopSessionProcessCancellationAndForcedStop(t *testing.T) {
	for _, mode := range []string{"ok", "stubborn"} {
		t.Run(mode, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			p, err := startMetadataProcessWithin(ctx, testBinary(t), processVault(t, mode), time.Second, 100*time.Millisecond)
			if err != nil {
				t.Fatal(err)
			}
			cancel()
			select {
			case <-p.done:
			case <-time.After(2 * time.Second):
				t.Fatal("cancel left child running")
			}
			closeErr := p.Close()
			if mode == "stubborn" && (closeErr == nil || closeErr.Error() != "session_forced_stop") {
				t.Fatal("forced stop reported as graceful")
			}
			if mode == "ok" && closeErr != nil {
				t.Fatal(closeErr)
			}
		})
	}
}
func TestDesktopSessionProcessBinaryTrust(t *testing.T) {
	binary := testBinary(t)
	vault := processVault(t, "ok")
	bad := binary
	bad.sha256 = "sha256:" + strings.Repeat("b", 64)
	if p, err := startMetadataProcess(context.Background(), bad, vault); p != nil || err == nil || err.Error() != "owner_binary_invalid" {
		t.Fatal("wrong binary accepted")
	}
	bad = binary
	bad.manifestDigest = "unverified"
	if _, err := startMetadataProcess(context.Background(), bad, vault); err == nil {
		t.Fatal("missing digest accepted")
	}
	bad = binary
	bad.path = "pinax"
	if _, err := startMetadataProcess(context.Background(), bad, vault); err == nil {
		t.Fatal("PATH lookup allowed")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := startMetadataProcess(ctx, binary, vault); err == nil || err.Error() != "session_cancelled" {
		t.Fatal("cancelled start accepted")
	}
}
