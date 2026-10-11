package desktop

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yeisme/pinax/pkg/localsession"
)

// ownerBinary is composition-owned package metadata, never request input.
// The containing installation must be immutable to untrusted writers: checking
// a path hash is not an OS-level executable lease. Packaging enforces that
// trust boundary separately; no PATH lookup or caller argv/env is supported.
type ownerBinary struct{ path, sha256, manifestDigest string }

// readerProcess owns exactly one metadata-only child. It is not serializable
// and never returns its bearer or native root to a renderer.
type readerProcess struct {
	cmd          *exec.Cmd
	stdin        *os.File
	stdout       *os.File
	done         chan struct{}
	waitErr      error // published by closing done
	ready        localsession.Event
	bearer       string
	closeOnce    sync.Once
	closeErr     error
	drainTimeout time.Duration
}

func startMetadataProcess(ctx context.Context, binary ownerBinary, vault string) (*readerProcess, error) {
	return startMetadataProcessWithin(ctx, binary, vault, 15*time.Second, 5*time.Second)
}

// Shortened budgets are private test inputs, never caller-controlled protocol.
func startMetadataProcessWithin(ctx context.Context, binary ownerBinary, vault string, readyBudget, drainBudget time.Duration) (*readerProcess, error) {
	if ctx == nil || readyBudget <= 0 || readyBudget > 15*time.Second || drainBudget <= 0 || drainBudget > 5*time.Second {
		return nil, errors.New("session_request_invalid")
	}
	if ctx.Err() != nil {
		return nil, errors.New("session_cancelled")
	}
	if err := verifyOwnerBinary(binary); err != nil {
		return nil, err
	}
	if _, err := directoryIdentity(vault); err != nil {
		return nil, err
	}
	nonce, err := opaqueID()
	if err != nil {
		return nil, err
	}
	bearer, err := opaqueID()
	if err != nil {
		return nil, err
	}
	binding, err := opaqueID()
	if err != nil {
		return nil, err
	}
	frame, err := json.Marshal(localsession.Bootstrap{SchemaVersion: localsession.BootstrapVersion, SessionNonce: nonce, Bearer: bearer, BindingID: binding, VaultRoot: vault, Scopes: []string{"reader.metadata"}, Readonly: true})
	if err != nil || len(frame)+1 > localsession.MaxFrameBytes {
		return nil, errors.New("session_request_invalid")
	}
	input, parentInput, err := os.Pipe()
	if err != nil {
		return nil, errors.New("session_start_failed")
	}
	defer input.Close()
	parentOutput, output, err := os.Pipe()
	if err != nil {
		parentInput.Close()
		return nil, errors.New("session_start_failed")
	}
	defer output.Close()
	// Direct OS handles, not os/exec copy goroutines: an inherited output handle
	// cannot keep Wait blocked after the owned child exits.
	null, err := os.OpenFile(os.DevNull, os.O_WRONLY, 0)
	if err != nil {
		parentInput.Close()
		parentOutput.Close()
		return nil, errors.New("session_start_failed")
	}
	defer null.Close()
	cmd := exec.Command(binary.path, "api", "serve", "--session-stdin")
	cmd.Dir = filepath.Dir(binary.path)
	cmd.Env = []string{}
	// Windows loader needs SystemRoot. No PINAX_*, tokens or user config is inherited.
	if root := os.Getenv("SystemRoot"); root != "" {
		cmd.Env = append(cmd.Env, "SystemRoot="+root)
	}
	cmd.Stdin, cmd.Stdout, cmd.Stderr = input, output, null
	if err = cmd.Start(); err != nil {
		parentInput.Close()
		parentOutput.Close()
		return nil, errors.New("session_start_failed")
	}
	input.Close()
	output.Close()
	p := &readerProcess{cmd: cmd, stdin: parentInput, stdout: parentOutput, done: make(chan struct{}), bearer: bearer, drainTimeout: drainBudget}
	go func() { p.waitErr = cmd.Wait(); close(p.done) }()
	startup, cancel := context.WithTimeout(ctx, readyBudget)
	defer cancel()
	fail := func(code string) (*readerProcess, error) {
		// A failed handshake is not a graceful session: kill only our child now.
		_ = p.cmd.Process.Kill()
		if err := p.Close(); err != nil && err.Error() == "session_cleanup_timeout" {
			return nil, errors.New("session_cleanup_failed")
		}
		return nil, errors.New(code)
	}
	wrote := make(chan error, 1)
	go func() { _, err := parentInput.Write(append(frame, '\n')); wrote <- err }()
	select {
	case err := <-wrote:
		if err != nil {
			return fail("session_bootstrap_failed")
		}
	case <-startup.Done():
		return fail("session_ready_timeout")
	case <-p.done:
		return fail("session_exited")
	}
	type result struct {
		event localsession.Event
		err   error
	}
	received := make(chan result, 1)
	go func() {
		scanner := bufio.NewScanner(parentOutput)
		scanner.Buffer(make([]byte, 1024), localsession.MaxFrameBytes+1)
		if !scanner.Scan() {
			received <- result{err: errors.New("session_ready_invalid")}
			return
		}
		event, err := localsession.DecodeReady(scanner.Bytes(), nonce, binding, binary.manifestDigest, true)
		received <- result{event: event, err: err}
		if err != nil {
			return
		}
		// The remaining stream is never forwarded or persisted. Bound both each
		// frame and total output; EOF/exit are monitored independently.
		total := 0
		for scanner.Scan() {
			total += len(scanner.Bytes()) + 1
			if total > 64*1024 {
				_ = cmd.Process.Kill()
				return
			}
		}
		// Losing the stdout protocol while still alive is terminal too.
		// Close gives an orderly EOF opportunity before force-stopping it.
		_ = p.Close()
	}()
	select {
	case got := <-received:
		if got.err != nil {
			return fail("session_ready_invalid")
		}
		p.ready = got.event
	case <-startup.Done():
		return fail("session_ready_timeout")
	case <-p.done:
		return fail("session_exited")
	}
	if ctx.Err() != nil {
		return fail("session_cancelled")
	}
	select {
	case <-p.done:
		return fail("session_exited")
	default:
	}
	go func() {
		select {
		case <-ctx.Done():
			_ = p.Close()
		case <-p.done:
			_ = p.Close()
		}
	}()
	return p, nil
}

// Close first sends EOF, then force-stops only the recorded child if it does
// not drain. Concurrent calls share one cleanup. Descendant process-tree
// containment belongs to the platform host and is not claimed here.
func (p *readerProcess) Close() error {
	p.closeOnce.Do(func() {
		_ = p.stdin.Close()
		timer := time.NewTimer(p.drainTimeout)
		defer timer.Stop()
		forced := false
		select {
		case <-p.done:
		case <-timer.C:
			forced = true
			_ = p.cmd.Process.Kill()
		}
		_ = p.stdout.Close()
		select {
		case <-p.done:
			if forced {
				p.closeErr = errors.New("session_forced_stop")
			} else if p.waitErr != nil {
				p.closeErr = errors.New("session_exited")
			}
		case <-time.After(time.Second):
			p.closeErr = errors.New("session_cleanup_timeout")
		}
	})
	return p.closeErr
}

func verifyOwnerBinary(binary ownerBinary) error {
	fail := func() error { return errors.New("owner_binary_invalid") }
	if !filepath.IsAbs(binary.path) || filepath.Clean(binary.path) != binary.path || !fixedDigest(binary.sha256) || !fixedDigest(binary.manifestDigest) {
		return fail()
	}
	if _, err := directoryIdentity(filepath.Dir(binary.path)); err != nil {
		return fail()
	}
	before, err := os.Lstat(binary.path)
	if err != nil || !before.Mode().IsRegular() || before.Size() > 512*1024*1024 || before.Mode().Perm()&0022 != 0 {
		return fail()
	}
	file, err := os.Open(binary.path)
	if err != nil {
		return fail()
	}
	defer file.Close()
	opened, err := file.Stat()
	if err != nil || !os.SameFile(before, opened) {
		return fail()
	}
	digest := sha256.New()
	n, err := io.Copy(digest, io.LimitReader(file, 512*1024*1024+1))
	after, statErr := os.Lstat(binary.path)
	if err != nil || statErr != nil || n != before.Size() || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) || "sha256:"+hex.EncodeToString(digest.Sum(nil)) != binary.sha256 {
		return fail()
	}
	return nil
}
func fixedDigest(value string) bool {
	if len(value) != 71 || !strings.HasPrefix(value, "sha256:") {
		return false
	}
	for _, c := range value[7:] {
		if !(c >= 'a' && c <= 'f' || c >= '0' && c <= '9') {
			return false
		}
	}
	return true
}
