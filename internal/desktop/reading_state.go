package desktop

import (
	"context"
	"errors"
	"github.com/yeisme/drivebridge-server/pkg/desktopcontract"
	"github.com/yeisme/drivebridge-server/pkg/personalstate"
	"github.com/yeisme/pinax/pkg/pinaxclient"
	"net/http"
)

// The local store owns CAS and immutable receipts. A single-target request
// holds its guard until all authorizations, storage work and postchecks finish.
func (f *facade) readingOperation(ctx context.Context, call func(*personalstate.ReadingStore, personalstate.Scope) (any, error)) (out any, result error) {
	f.preflight.mu.Lock()
	adapter, ok := f.preflight.admission.(*BridgeAdmission)
	closed := f.preflight.closed
	f.preflight.mu.Unlock()
	if !ok || closed {
		return nil, errors.New("desktop_contract_unsupported")
	}
	scope := personalstate.Scope{Subject: "desktop-local", Instance: adapter.instance}
	var release func() error
	var heldSource string
	defer func() {
		if release != nil {
			if err := release(); result == nil {
				result = err
			}
		}
	}()
	authorize := func(ctx context.Context, current personalstate.Scope, ref desktopcontract.ReadingRef) error {
		if current != scope || ref.InstanceID != scope.Instance || ref.Kind != "pinax" || ref.Validate() != nil {
			return errors.New("reading_target_unavailable")
		}
		bound, admission, err := f.preflight.sourceBinding(ref.SourceID)
		if err != nil {
			return err
		}
		if release == nil {
			release, err = admission.Acquire(bound.Root, bound.BindingID, bound.Epoch, []string{"reader.metadata", "reader.body"})
			if err != nil {
				return err
			}
			heldSource = ref.SourceID
		} else if heldSource != ref.SourceID {
			return errors.New("reading_target_unavailable")
		}
		return f.withOwnerSession(ctx, bound, []string{"reader.metadata", "reader.body"}, true, func(ctx context.Context, client *pinaxclient.Client) error {
			doc, err := client.ReaderDocument(ctx, pinaxclient.ReaderDocumentRequest{ObjectID: ref.ObjectID, Revision: ref.Revision})
			if err != nil {
				return err
			}
			if doc.ObjectID != ref.ObjectID || doc.Revision != ref.Revision {
				return errors.New("reading_target_unavailable")
			}
			return nil
		})
	}
	store, err := adapter.service.ReadingStatesForPinax(authorize)
	if err != nil {
		return nil, err
	}
	return call(store, scope)
}
func (f *facade) readingState(w http.ResponseWriter, r *http.Request) {
	var call func(*personalstate.ReadingStore, personalstate.Scope) (any, error)
	switch r.URL.Path {
	case "/v1/desktop/reading-state/read":
		var input desktopcontract.ReadReadingState
		if decodeFacadeRequest(w, r, &input, []string{"target_ref"}) != nil || input.TargetRef.Validate() != nil {
			facadeError(w, "request_invalid")
			return
		}
		call = func(store *personalstate.ReadingStore, scope personalstate.Scope) (any, error) {
			state, found, err := store.Get(r.Context(), scope, input.TargetRef)
			out := desktopcontract.ReadingMutationResult{Found: found}
			if found {
				out.State = &state
			}
			return out, err
		}
	case "/v1/desktop/reading-state":
		var input desktopcontract.UpdateReadingState
		// Optional update fields retain the existing closed Go DTO validation.
		if decodeFacadeOptionalRequest(w, r, &input, []string{"target_ref", "expected_revision"}, []string{"favorite", "read_later", "position", "clear_position"}) != nil || input.Validate() != nil || len(r.Header.Values("Idempotency-Key")) != 1 || !facadeKey.MatchString(r.Header.Get("Idempotency-Key")) {
			facadeError(w, "request_invalid")
			return
		}
		call = func(store *personalstate.ReadingStore, scope personalstate.Scope) (any, error) {
			return store.Update(r.Context(), scope, r.Header.Get("Idempotency-Key"), input)
		}
	case "/v1/desktop/reading-state/lookup":
		var input desktopcontract.LookupReadingMutation
		if decodeFacadeRequest(w, r, &input, []string{"idempotency_key"}) != nil || !facadeKey.MatchString(input.IdempotencyKey) {
			facadeError(w, "request_invalid")
			return
		}
		call = func(store *personalstate.ReadingStore, scope personalstate.Scope) (any, error) {
			return store.Lookup(r.Context(), scope, input.IdempotencyKey)
		}
	default:
		facadeError(w, "desktop_contract_unsupported")
		return
	}
	out, err := f.readingOperation(r.Context(), call)
	if err != nil {
		facadeCallError(w, err)
		return
	}
	facadeData(w, out)
}
