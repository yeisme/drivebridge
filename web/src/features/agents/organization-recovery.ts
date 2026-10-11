import type { OrganizationPatch } from './organization-client';
export type OrganizationIntent = { key: string; body: OrganizationPatch };
export const organizationSlot = (scope: string, agent: string) => 'drivebridge:organization:' + JSON.stringify([scope, agent]);
const ownKeys = (value: unknown, keys: string[]) => typeof value === 'object' && value !== null && !Array.isArray(value) && Object.keys(value).every(key => keys.includes(key));
export function readOrganizationIntent(slot: string): OrganizationIntent | null {
  const raw = localStorage.getItem(slot);
  if (raw === null) return null;
  if (raw.length > 2048) throw new Error('recovery_identity_invalid');
  let value: OrganizationIntent;
  try { value = JSON.parse(raw) as OrganizationIntent; }
  catch { throw new Error('recovery_identity_invalid'); }
  if (!value || !ownKeys(value, ['key','body']) || !/^grant-[a-f0-9]{48}$/.test(value.key) || !value.body || !ownKeys(value.body, ['expected_revision','group_id','pinned'])) throw new Error('recovery_identity_invalid');
  const body = value.body;
  if (!Number.isSafeInteger(body.expected_revision) || body.expected_revision < 0 || (body.group_id === undefined && body.pinned === undefined) || (body.pinned !== undefined && typeof body.pinned !== 'boolean') || (body.group_id !== undefined && body.group_id !== null && (typeof body.group_id !== 'string' || body.group_id.length > 512 || !body.group_id.trim()))) throw new Error('recovery_identity_invalid');
  return value;
}
export async function claimOrganizationIntent(slot: string, body: OrganizationPatch, signal: AbortSignal) {
  if (!navigator.locks?.request) throw new Error('recovery_lock_unavailable');
  return navigator.locks.request(slot, { signal }, () => {
    signal.throwIfAborted(); const existing = readOrganizationIntent(slot);
    if (existing) return { intent: existing, claimed: false };
    const key = 'grant-' + Array.from(crypto.getRandomValues(new Uint8Array(24)), b => b.toString(16).padStart(2, '0')).join('');
    const intent = { key, body }; localStorage.setItem(slot, JSON.stringify(intent)); return { intent, claimed: true };
  });
}
export async function clearOrganizationIntent(slot: string, intent: OrganizationIntent) {
  if (!navigator.locks?.request) throw new Error('recovery_lock_unavailable');
  return navigator.locks.request(slot, () => {
    if (JSON.stringify(readOrganizationIntent(slot)) !== JSON.stringify(intent)) throw new Error('recovery_identity_changed');
    localStorage.removeItem(slot);
  });
}
