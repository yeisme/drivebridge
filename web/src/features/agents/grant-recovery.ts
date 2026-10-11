// Persist only an opaque recovery identity, never request content or a secret.
const keyPattern = /^grant-[a-f0-9]{48}$/;
export const grantRecoverySlot = (scope: string) => 'drivebridge:grant-create:' + scope;
export function readGrantRecovery(slot: string): string | null {
  const value = localStorage.getItem(slot);
  if (value !== null && !keyPattern.test(value)) throw new Error('recovery_identity_invalid');
  return value;
}
export async function claimGrantRecovery(slot: string, signal: AbortSignal) {
  if (!navigator.locks?.request) throw new Error('recovery_lock_unavailable');
  return navigator.locks.request(slot, { signal }, () => {
    signal.throwIfAborted();
    const existing = readGrantRecovery(slot);
    if (existing) return { key: existing, claimed: false };
    const key = 'grant-' + Array.from(crypto.getRandomValues(new Uint8Array(24)), b => b.toString(16).padStart(2, '0')).join('');
    localStorage.setItem(slot, key);
    return { key, claimed: true };
  });
}
export async function clearGrantRecovery(slot: string, key: string) {
  if (!navigator.locks?.request) throw new Error('recovery_lock_unavailable');
  return navigator.locks.request(slot, () => {
    if (readGrantRecovery(slot) === key) localStorage.removeItem(slot);
  });
}
