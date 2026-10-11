import type { GrantChangeAction } from './grant-client';

export const changeSlot = (scope: string, grantID: string) => 'drivebridge:grant-change:' + JSON.stringify([scope, grantID]);
export type PendingChange = { action: GrantChangeAction; key: string };
// A single slot serializes lifecycle intents across actions. No request body,
// expiry, name or credential is persisted. Only the app creates this record.
export function readChange(slot: string): PendingChange | null {
  const raw = localStorage.getItem(slot);
  if (raw === null) return null;
  const match = /^(disable|enable|revoke|expiry):(grant-[a-f0-9]{48})$/.exec(raw);
  if (!match) throw new Error('recovery_identity_invalid');
  return { action: match[1] as GrantChangeAction, key: match[2] };
}
const encoded = (pending: PendingChange) => pending.action + ':' + pending.key;
export async function claimChange(slot: string, action: GrantChangeAction, signal: AbortSignal) {
  if (!navigator.locks?.request) throw new Error('recovery_lock_unavailable');
  return navigator.locks.request(slot, { signal }, () => {
    signal.throwIfAborted();
    const existing = readChange(slot);
    if (existing) return { pending: existing, claimed: false };
    const key = 'grant-' + Array.from(crypto.getRandomValues(new Uint8Array(24)), b => b.toString(16).padStart(2, '0')).join('');
    const pending = { action, key };
    localStorage.setItem(slot, encoded(pending));
    return { pending, claimed: true };
  });
}
export async function clearChange(slot: string, pending: PendingChange) {
  if (!navigator.locks?.request) throw new Error('recovery_lock_unavailable');
  return navigator.locks.request(slot, () => {
    if (localStorage.getItem(slot) !== encoded(pending)) throw new Error('recovery_identity_changed');
    localStorage.removeItem(slot);
  });
}
export function sameChange(a: PendingChange | null, b: PendingChange) {
  return a?.action === b.action && a.key === b.key;
}
