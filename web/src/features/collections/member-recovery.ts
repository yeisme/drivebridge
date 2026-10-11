import type { components } from '../../generated/personal-workspace';
export type Decisions = components['schemas']['CollectionMemberDecisionsEnvelope']['data'];
export type Decision = Decisions['items'][number];
export type MemberChange = components['schemas']['CollectionMembersChangeRequest'];
export type MemberReceipt = components['schemas']['CollectionMembersReceiptEnvelope']['data'];
export type MemberRecovery = { key: string; digest: string };
export const memberSlot = (scope: string, id: string) => 'drivebridge:collection-members:' + JSON.stringify([scope, id]);
export const fileID = (value: unknown): value is string => typeof value === 'string' && /^[a-f0-9]{48}$/.test(value);
export function validDecisions(value: Decisions, id: string) {
  return !!value && value.collection_id === id && Number.isSafeInteger(value.revision) && value.revision > 0 && value.revision < Number.MAX_SAFE_INTEGER && Array.isArray(value.items) && value.items.length <= 10000 && value.items.every(row => row && fileID(row.file_id) && typeof row.space_id === 'string' && !!row.space_id && typeof row.path === 'string' && ['include', 'exclude'].includes(row.decision)) && new Set(value.items.map(row => row.file_id)).size === value.items.length;
}
export async function memberDigest(id: string, input: MemberChange) {
  const raw = JSON.stringify({ collection_id: id, request: { expected_revision: input.expected_revision, include: input.include, exclude: input.exclude } });
  return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(raw))), byte => byte.toString(16).padStart(2, '0')).join('');
}
export function readMemberRecovery(slot: string): MemberRecovery | null {
  const raw = localStorage.getItem(slot); if (raw === null) return null;
  if (raw.length > 256) throw new Error('invalid_recovery');
  let value: unknown; try { value = JSON.parse(raw); } catch { throw new Error('invalid_recovery'); }
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid_recovery');
  const row = value as MemberRecovery;
  if (Object.keys(row).sort().join(',') !== 'digest,key' || !/^members-[a-f0-9]{48}$/.test(row.key) || !/^[a-f0-9]{64}$/.test(row.digest)) throw new Error('invalid_recovery');
  return row;
}
export async function claimMembers(slot: string, digest: string, signal: AbortSignal) {
  if (!navigator.locks?.request) throw new Error('lock_unavailable');
  return navigator.locks.request(slot, { signal }, () => {
    signal.throwIfAborted(); const existing = readMemberRecovery(slot);
    if (existing) return { record: existing, claimed: false };
    const record = { key: 'members-' + Array.from(crypto.getRandomValues(new Uint8Array(24)), byte => byte.toString(16).padStart(2, '0')).join(''), digest };
    localStorage.setItem(slot, JSON.stringify(record)); return { record, claimed: true };
  });
}
export async function clearMembers(slot: string, record: MemberRecovery) {
  if (!navigator.locks?.request) throw new Error('lock_unavailable');
  return navigator.locks.request(slot, () => {
    const current = readMemberRecovery(slot);
    if (current?.key !== record.key || current.digest !== record.digest) throw new Error('recovery_changed');
    localStorage.removeItem(slot);
  });
}
export function matchesMemberReceipt(row: MemberReceipt, record: MemberRecovery, id: string, expected?: number) {
  return !!row && Object.keys(row).sort().join(',') === 'collection_id,committed_at,request_digest,result_revision' && row.collection_id === id && row.request_digest === record.digest && Number.isSafeInteger(row.result_revision) && row.result_revision >= 2 && (expected === undefined || row.result_revision === expected + 1) && typeof row.committed_at === 'string' && Number.isFinite(Date.parse(row.committed_at));
}
export function decisionDiff(before: Decision[], after: Decision[]) {
  const old = new Map(before.map(row => [row.file_id, row]));
  const changes = after.filter(row => old.get(row.file_id)?.decision !== row.decision).map(row => ({ ...row, before: old.get(row.file_id)?.decision ?? 'none', after: row.decision }));
  const remaining = new Set(after.map(row => row.file_id));
  return [...changes, ...before.filter(row => !remaining.has(row.file_id)).map(row => ({ ...row, before: row.decision, after: 'none' }))];
}
