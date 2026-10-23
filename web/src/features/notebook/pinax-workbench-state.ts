export type PinaxWorkbenchState =
  | { kind: "bound"; sourceId: string }
  | { kind: "reading"; sourceId: string }
  | { kind: "draft"; sourceId: string; objectId: string; revision: string; body?: string }
  | { kind: "preview"; sourceId: string; objectId: string; revision: string; body?: string }
  | { kind: "applying"; sourceId: string; objectId: string; revision: string; operationKey: string; body?: string }
  | { kind: "succeeded"; sourceId: string; objectId: string; revision: string; operationKey?: string }
  | { kind: "conflict"; sourceId: string; objectId: string; revision: string; operationKey?: string; body?: string }
  | { kind: "unknown"; sourceId: string; objectId: string; revision: string; operationKey: string; body?: string }
  | { kind: "revoked"; sourceId: string };

export type PinaxWorkbenchEvent =
  | { type: "read" }
  | { type: "draft"; objectId: string; revision: string; body?: string }
  | { type: "edit"; body: string }
  | { type: "preview" }
  | { type: "apply"; operationKey: string }
  | { type: "succeeded"; revision: string; operationKey?: string }
  | { type: "conflict"; operationKey?: string }
  | { type: "unknown"; operationKey: string }
  | { type: "revoke" };

export function createPinaxWorkbenchState(sourceId: string): PinaxWorkbenchState {
  return { kind: "bound", sourceId };
}

export function transitionPinaxWorkbench(state: PinaxWorkbenchState, event: PinaxWorkbenchEvent): PinaxWorkbenchState {
  if (event.type === "revoke") return { kind: "revoked", sourceId: state.sourceId };
  if (state.kind === "revoked") return state;
  if (event.type === "read" && ["bound", "reading", "succeeded", "conflict"].includes(state.kind)) return { kind: "reading", sourceId: state.sourceId };
  if (event.type === "draft" && (state.kind === "reading" || state.kind === "succeeded" || state.kind === "conflict" || state.kind === "preview") && event.objectId && event.revision) return { kind: "draft", sourceId: state.sourceId, objectId: event.objectId, revision: event.revision, ...(event.body === undefined ? {} : { body: event.body }) };
  if (event.type === "edit" && (state.kind === "draft" || state.kind === "preview")) return { ...state, kind: "draft", body: event.body };
  if (event.type === "preview" && state.kind === "draft") return { ...state, kind: "preview" };
  if (event.type === "apply" && state.kind === "preview" && /^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(event.operationKey)) return { ...state, kind: "applying", operationKey: event.operationKey };
  if (state.kind === "applying" || state.kind === "unknown") {
    if (event.type === "succeeded" || event.type === "conflict") {
      // A recovery response must name the original operation. A pending local
      // completion may omit it, but can never substitute a different identity.
      if ((state.kind === "unknown" && event.operationKey !== state.operationKey) || (event.operationKey !== undefined && event.operationKey !== state.operationKey)) return state;
      if (event.type === "succeeded" && event.revision) return { kind: "succeeded", sourceId: state.sourceId, objectId: state.objectId, revision: event.revision, operationKey: state.operationKey };
      if (event.type === "conflict") return { ...state, kind: "conflict" };
    }
    if (event.type === "unknown" && event.operationKey === state.operationKey) return { ...state, kind: "unknown" };
  }
  return state;
}

// Only this opaque value belongs in a recovery slot; draft text stays in state
// owned by the current page session, never localStorage or sessionStorage.
export function pinaxWorkbenchRecoveryKey(state: PinaxWorkbenchState): string | undefined {
  return state.kind === "applying" || state.kind === "unknown" ? state.operationKey : undefined;
}
