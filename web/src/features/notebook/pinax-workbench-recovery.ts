import { claimNotebookRecovery, clearNotebookRecovery, inspectNotebookRecovery, NotebookRecoveryError } from "./notebook-recovery";

export interface PinaxRecoveryScope { actor: string; sourceId: string; bindingId: string }

function opaqueKey(value: string) {
  if (!/^[A-Za-z0-9][A-Za-z0-9._:-]{0,127}$/.test(value)) throw new NotebookRecoveryError("invalid_recovery");
  return value;
}

export function pinaxRecoverySlot(scope: PinaxRecoveryScope) {
  return "drivebridge:pinax-operation:" + JSON.stringify([opaqueKey(scope.actor), opaqueKey(scope.sourceId), opaqueKey(scope.bindingId)]);
}

// Reuse the existing Web Lock/storage admission. Persist exactly the opaque
// operation key; draft text, revisions, file paths and credentials never enter
// this slot. A stale or malformed slot must not be replaced automatically.
export function inspectPinaxRecovery(scope: PinaxRecoveryScope) {
  return inspectNotebookRecovery(pinaxRecoverySlot(scope), opaqueKey);
}

export function claimPinaxRecovery(scope: PinaxRecoveryScope, operationKey: string, signal: AbortSignal) {
  return claimNotebookRecovery(pinaxRecoverySlot(scope), opaqueKey, opaqueKey(operationKey), opaqueKey, signal);
}

// The host calls this only after an original-operation terminal readback or
// explicit revocation/cleanup. CAS prevents clearing another tab's operation.
export function clearPinaxRecovery(scope: PinaxRecoveryScope, operationKey: string) {
  return clearNotebookRecovery(pinaxRecoverySlot(scope), opaqueKey, opaqueKey(operationKey), value => value);
}
