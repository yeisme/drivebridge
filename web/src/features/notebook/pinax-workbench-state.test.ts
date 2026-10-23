import { expect, it } from "vitest";
import { createPinaxWorkbenchState, transitionPinaxWorkbench, pinaxWorkbenchRecoveryKey, type PinaxWorkbenchState } from "./pinax-workbench-state";

it("keeps source identity and makes the recovery key opaque", () => {
  let state = createPinaxWorkbenchState("source");
  state = transitionPinaxWorkbench(state, { type: "read" });
  state = transitionPinaxWorkbench(state, { type: "draft", objectId: "note", revision: "sha256:rev" });
  state = transitionPinaxWorkbench(state, { type: "preview" });
  state = transitionPinaxWorkbench(state, { type: "apply", operationKey: "operation-opaque" });
  state = transitionPinaxWorkbench(state, { type: "unknown", operationKey: "operation-opaque" });
  expect(state).toEqual({ kind: "unknown", sourceId: "source", objectId: "note", revision: "sha256:rev", operationKey: "operation-opaque" });
});

it("stops all transitions after revocation", () => {
  const revoked = transitionPinaxWorkbench(createPinaxWorkbenchState("source"), { type: "revoke" });
  expect(transitionPinaxWorkbench(revoked, { type: "read" })).toEqual(revoked);
});

function applying(): PinaxWorkbenchState {
  let state = createPinaxWorkbenchState("source");
  state = transitionPinaxWorkbench(state, { type: "read" });
  state = transitionPinaxWorkbench(state, { type: "draft", objectId: "note", revision: "r1", body: "private draft" });
  state = transitionPinaxWorkbench(state, { type: "preview" });
  return transitionPinaxWorkbench(state, { type: "apply", operationKey: "original" });
}

it("preserves the original key until a same-operation recovery is terminal", () => {
  const pending = applying();
  expect(transitionPinaxWorkbench(pending, { type: "read" })).toBe(pending);
  expect(transitionPinaxWorkbench(pending, { type: "unknown", operationKey: "replacement" })).toBe(pending);
  const unknown = transitionPinaxWorkbench(pending, { type: "unknown", operationKey: "original" });
  expect(transitionPinaxWorkbench(unknown, { type: "read" })).toBe(unknown);
  expect(transitionPinaxWorkbench(unknown, { type: "apply", operationKey: "new" })).toBe(unknown);
  expect(transitionPinaxWorkbench(unknown, { type: "succeeded", revision: "r2" })).toBe(unknown);
  expect(transitionPinaxWorkbench(unknown, { type: "succeeded", revision: "r2", operationKey: "other" })).toBe(unknown);
  expect(pinaxWorkbenchRecoveryKey(unknown)).toBe("original");
  const recovered = transitionPinaxWorkbench(unknown, { type: "succeeded", revision: "r2", operationKey: "original" });
  expect(recovered).toMatchObject({ kind: "succeeded", revision: "r2", operationKey: "original" });
  expect("body" in recovered).toBe(false);
  expect(pinaxWorkbenchRecoveryKey(recovered)).toBeUndefined();
});

it("holds draft text in state, invalidates edited previews and clears it on revocation", () => {
  const preview = transitionPinaxWorkbench(transitionPinaxWorkbench(createPinaxWorkbenchState("source"), { type: "read" }), { type: "draft", objectId: "note", revision: "r1", body: "draft" });
  const edited = transitionPinaxWorkbench(transitionPinaxWorkbench(preview, { type: "preview" }), { type: "edit", body: "updated" });
  expect(edited).toMatchObject({ kind: "draft", body: "updated", revision: "r1" });
  expect(transitionPinaxWorkbench(edited, { type: "apply", operationKey: "premature" })).toBe(edited);
  const revoked = transitionPinaxWorkbench(applying(), { type: "revoke" });
  expect(revoked).toEqual({ kind: "revoked", sourceId: "source" });
  expect(pinaxWorkbenchRecoveryKey(revoked)).toBeUndefined();
  expect(createPinaxWorkbenchState("other")).toEqual({ kind: "bound", sourceId: "other" });
});

it.each(["", "../private", "https://example.invalid/token", "a".repeat(129)])("rejects a non-opaque operation key %s", operationKey => {
  const draft = transitionPinaxWorkbench(transitionPinaxWorkbench(createPinaxWorkbenchState("source"), { type: "read" }), { type: "draft", objectId: "note", revision: "r1" });
  const preview = transitionPinaxWorkbench(draft, { type: "preview" });
  expect(transitionPinaxWorkbench(preview, { type: "apply", operationKey })).toBe(preview);
});
