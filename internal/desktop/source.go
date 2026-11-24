package desktop

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"github.com/yeisme/pinax/pkg/pinaxclient"
)

// facadeOwnerError keeps the owner origin so the closed pinax error vocabulary
// applies; a drivebridge-origin code would be masked to owner_unavailable and the
// real recovery class would be lost.
func facadeOwnerError(w http.ResponseWriter, code string) {
	value, status := desktopcontract.SafeError("pinax", code, "")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(struct {
		Error desktopcontract.DesktopError `json:"error"`
	}{value})
}

func facadeCallError(w http.ResponseWriter, err error) {
	var ownerErr *pinaxclient.Error
	if errors.As(err, &ownerErr) {
		facadeOwnerError(w, ownerFailure(err))
		return
	}
	// Guard failures belong to DriveBridge. Keeping their origin preserves the
	// closed conflict/revocation recovery codes without exposing upstream text.
	facadeError(w, err.Error())
}

// ownerFailure maps an owner error to a stable, path-free code. Upstream messages
// can contain vault paths or provider text, so only the code family is kept.
func ownerFailure(err error) string {
	var ownerErr *pinaxclient.Error
	if !errors.As(err, &ownerErr) {
		return "owner_rejected"
	}
	switch ownerErr.Code {
	case pinaxclient.CodeMutationOutcomeUnknown, pinaxclient.CodeReconcileRequired:
		return "mutation_outcome_unknown"
	case pinaxclient.CodeRequestInvalid:
		return "request_invalid"
	}
	// Owner domain codes are already snake_case identifiers and never contain
	// paths; conflict codes keep their family so a renderer can offer recovery.
	code := ownerErr.Code
	if code == "" || len(code) > 128 {
		return "owner_rejected"
	}
	for _, r := range code {
		if !(r == '_' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9') {
			return "owner_rejected"
		}
	}
	return code
}

// withOwner runs one bounded call against the owner binary for a bound root. The
// child is digest-validated. Read sessions retain opaque cursors while each
// call rechecks its guard; write sessions close before returning.
func (f *facade) withOwner(ctx context.Context, sourceID string, scopes []string, readonly bool, call func(context.Context, *pinaxclient.Client) error) (result error) {
	bound, admission, err := f.preflight.sourceBinding(sourceID)
	if err != nil {
		return err
	}
	release, err := admission.Acquire(bound.Root, bound.BindingID, bound.Epoch, scopes)
	if err != nil {
		return err
	}
	defer func() {
		if err := release(); result == nil {
			result = err
		}
	}()
	// Lookup can wait behind a previous owner call. Closing the host while it
	// waits must not start a new child with the old session's authority.
	if _, _, err := f.preflight.sourceBinding(sourceID); err != nil {
		return err
	}
	return f.withOwnerSession(ctx, bound, scopes, readonly, call)
}

// Caller must hold the matching owner guard through this call and publication.
func (f *facade) withOwnerSession(ctx context.Context, bound boundSource, scopes []string, readonly bool, call func(context.Context, *pinaxclient.Client) error) error {
	var err error
	sourceID := bound.SourceID
	// Keep read sessions for opaque owner cursors. Every call still acquires
	// the current guard lease; release/suspend closes these private processes.
	f.ownerMu.Lock()
	defer f.ownerMu.Unlock()
	cacheKey := sourceID + ":" + strings.Join(scopes, ",")
	child := f.readers[cacheKey]
	if child != nil {
		select {
		case <-child.done:
			delete(f.readers, cacheKey)
			child = nil
		default:
		}
	}
	if child == nil {
		if readonly && len(f.readers) >= 32 {
			return errors.New("managed_scope_busy")
		}
		lifetime := f.ctx
		if lifetime == nil {
			lifetime = ctx
		}
		child, err = startOwnerSession(lifetime, f.binary, bound.Root, scopes, readonly)
		if err == nil && readonly {
			if f.readers == nil {
				f.readers = map[string]*readerProcess{}
			}
			f.readers[cacheKey] = child
		}
	}
	if err != nil {
		// A rejected bootstrap is a distinct, stable fact; only an unclassified
		// failure collapses to owner_unavailable.
		code := err.Error()
		// Every session_* code originates in this package and carries no path or
		// owner text, so it is safe to surface for recovery.
		if strings.HasPrefix(code, "session_") && len(code) <= 64 {
			return errors.New(code)
		}
		return errors.New("owner_unavailable")
	}
	if !readonly {
		defer child.Close()
	}
	client, err := pinaxclient.New(pinaxclient.Config{BaseURL: child.ready.Address, Token: child.bearer, Timeout: 5 * time.Second})
	if err != nil {
		return errors.New("owner_unavailable")
	}
	return call(ctx, client)
}

func (f *facade) closeReaders(sourceID string) {
	f.ownerMu.Lock()
	defer f.ownerMu.Unlock()
	for key, child := range f.readers {
		if sourceID == "" || strings.HasPrefix(key, sourceID+":") {
			_ = child.Close()
			delete(f.readers, key)
		}
	}
}

// listNotes projects the owner's bounded note page. The renderer receives opaque
// object identities and never a vault path.
func (f *facade) listNotes(ctx context.Context, w http.ResponseWriter, r *http.Request, sourceID string) {
	if r.ContentLength != 0 {
		facadeError(w, "request_invalid")
		return
	}
	var page pinaxclient.ReaderNoteList
	query, err := readerPageQuery(r, "limit", "cursor", "sort", "project_id", "status", "tags")
	if err != nil {
		facadeError(w, "request_invalid")
		return
	}
	err = f.withOwner(ctx, sourceID, []string{"reader.metadata"}, true, func(ctx context.Context, client *pinaxclient.Client) error {
		var callErr error
		page, callErr = client.ReaderNotes(ctx, query)
		return callErr
	})
	if err != nil {
		facadeCallError(w, err)
		return
	}
	facadeData(w, page)
}

// readEditorSource returns the fixed-revision Markdown source plus the owner's
// editability and budget facts. The body is what the editor needs; nothing here
// carries a vault path.
func (f *facade) readEditorSource(ctx context.Context, w http.ResponseWriter, r *http.Request, sourceID string) {
	if r.Method != "GET" {
		facadeError(w, "desktop_contract_unsupported")
		return
	}
	query, err := readerQuery(r, "object_id", "revision")
	objectID := query.Get("object_id")
	revision := query.Get("revision")
	if err != nil || !facadeKey.MatchString(objectID) || !fixedDigest(revision) {
		facadeError(w, "request_invalid")
		return
	}
	var source pinaxclient.WriterEditorSource
	err = f.withOwner(ctx, sourceID, []string{"writer.editor", "reader.body"}, false, func(ctx context.Context, client *pinaxclient.Client) error {
		var callErr error
		source, callErr = client.WriterEditorSource(ctx, objectID, revision)
		return callErr
	})
	if err != nil {
		facadeCallError(w, err)
		return
	}
	if len(source.Body) > 65536 {
		// Bounded: the editor never receives a truncated source.
		facadeError(w, "source_too_large")
		return
	}
	facadeData(w, source)
}

// proposeEditorDraft submits the renderer's in-memory draft as an owner proposal.
// The host derives the change kind and the approval context; the renderer supplies
// only the fixed object identity, the revision it read, and the Markdown body.
// Only the digest and state come back: the owner path and diff bodies stay in the
// owner.
func (f *facade) proposeEditorDraft(ctx context.Context, w http.ResponseWriter, r *http.Request, sourceID string) {
	var input desktopcontract.DesktopEditorDraftInput
	if decodeFacadeRequest(w, r, &input, []string{"object_id", "revision", "body"}) != nil {
		facadeError(w, "request_invalid")
		return
	}
	if !facadeKey.MatchString(input.ObjectID) || !strings.HasPrefix(input.Revision, "sha256:") || len(input.Revision) != 71 || len(input.Body) > 65536 {
		facadeError(w, "request_invalid")
		return
	}
	var preview pinaxclient.WriterProposalPreview
	// A draft proposal is a write: it needs an explicit writer session, never an
	// upgraded reader session.
	err := f.withOwner(ctx, sourceID, []string{"writer.editor"}, false, func(ctx context.Context, client *pinaxclient.Client) error {
		var callErr error
		preview, callErr = client.WriterPropose(ctx, pinaxclient.WriterChange{Action: "replace", NoteRef: input.ObjectID, ExpectedRevision: input.Revision, Body: input.Body})
		return callErr
	})
	if err != nil {
		facadeCallError(w, err)
		return
	}
	// Path, before and after are deliberately not projected.
	facadeData(w, struct {
		SchemaVersion string `json:"schema_version"`
		ObjectID      string `json:"object_id"`
		Revision      string `json:"revision"`
		PreviewDigest string `json:"preview_digest"`
		State         string `json:"state"`
	}{SchemaVersion: preview.SchemaVersion, ObjectID: input.ObjectID, Revision: input.Revision, PreviewDigest: preview.PreviewDigest, State: preview.State})
}

// decodeEditorRoute splits /v1/desktop/sources/{source_id}/{rest}.
func decodeEditorRoute(target, rest string) (string, bool) {
	const prefix = "/v1/desktop/sources/"
	if !strings.HasPrefix(target, prefix) {
		return "", false
	}
	value := strings.TrimPrefix(target, prefix)
	if !strings.HasSuffix(value, rest) {
		return "", false
	}
	id := strings.TrimSuffix(value, rest)
	if id == "" || strings.Contains(id, "/") || !facadeKey.MatchString(id) {
		return "", false
	}
	return id, true
}
