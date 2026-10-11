import type { GroupChange } from './organization-client';
export type GroupIntent = { key: string; change: GroupChange };
export const groupSlot = (scope: string) => 'drivebridge:group-change:' + scope;
const exact = (value: unknown, keys: string[]) => typeof value === 'object' && value !== null && !Array.isArray(value) && Object.keys(value).sort().join(',') === keys.sort().join(',');
export function readGroupIntent(slot: string): GroupIntent | null {
  const raw = localStorage.getItem(slot); if (raw === null) return null;
  if (raw.length > 4096) throw new Error('recovery_identity_invalid');
  let value: GroupIntent;
  try { value = JSON.parse(raw) as GroupIntent; } catch { throw new Error('recovery_identity_invalid'); }
  if (!exact(value,['key','change']) || !/^grant-[a-f0-9]{48}$/.test(value.key) || !value.change) throw new Error('recovery_identity_invalid');
  const change = value.change;
  if (!['create','rename','delete'].includes(change.action) || !exact(change, change.action === 'create' ? ['action','body'] : ['action','id','body'])) throw new Error('recovery_identity_invalid');
  if (!exact(change.body,change.action === 'create' ? ['name'] : change.action === 'rename' ? ['name','expected_revision'] : ['expected_revision'])) throw new Error('recovery_identity_invalid');
  if (change.action !== 'create' && (typeof change.id !== 'string' || !change.id || change.id.length > 512 || !Number.isSafeInteger(change.body.expected_revision) || change.body.expected_revision < 1)) throw new Error('recovery_identity_invalid');
  if (change.action !== 'delete' && (typeof change.body.name !== 'string' || !change.body.name.trim() || [...change.body.name].length > 80)) throw new Error('recovery_identity_invalid');
  return value;
}
export async function claimGroupIntent(slot: string, change: GroupChange, signal: AbortSignal) {
  if (!navigator.locks?.request) throw new Error('recovery_lock_unavailable');
  return navigator.locks.request(slot, { signal }, () => {
    signal.throwIfAborted(); const existing = readGroupIntent(slot);
    if (existing) return { intent: existing, claimed: false };
    const key = 'grant-' + Array.from(crypto.getRandomValues(new Uint8Array(24)), b => b.toString(16).padStart(2,'0')).join('');
    const intent = { key, change }; localStorage.setItem(slot,JSON.stringify(intent)); return { intent, claimed: true };
  });
}
export async function clearGroupIntent(slot: string, intent: GroupIntent) {
  if (!navigator.locks?.request) throw new Error('recovery_lock_unavailable');
  return navigator.locks.request(slot, () => {
    if (JSON.stringify(readGroupIntent(slot)) !== JSON.stringify(intent)) throw new Error('recovery_identity_changed');
    localStorage.removeItem(slot);
  });
}
