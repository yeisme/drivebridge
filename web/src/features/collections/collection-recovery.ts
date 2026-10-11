import type { components } from '../../generated/personal-workspace';
export type CollectionReceipt = components['schemas']['CollectionCreationReceiptEnvelope']['data'];
export type Creation = { space_id: string; name: string; scope: 'private' } & ({ kind: 'manual' } | { kind: 'rule'; rule: components['schemas']['CollectionRule'] });
export type Recovery = { key: string; digest: string };
export const collectionSlot = (scope: string) => 'drivebridge:collection-create:' + scope;
export async function creationDigest(input: Creation) {
  // Match Go encoding/json's legacy field order and HTML/line-separator escaping.
  const raw = JSON.stringify({ space: input.space_id, name: input.name, kind: input.kind, scope: input.scope, rule: input.kind === 'rule' ? JSON.stringify(input.rule) : '' }).replace(/[<>&\u2028\u2029]/g, ch => '\\u' + ch.charCodeAt(0).toString(16).padStart(4, '0'));
  return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(raw))), b => b.toString(16).padStart(2, '0')).join('');
}
export function readCreation(slot: string): Recovery | null {
  const raw = localStorage.getItem(slot); if (raw === null) return null;
  if (raw.length > 256) throw new Error('invalid_recovery');
  let row: unknown;
  try { row = JSON.parse(raw); } catch { throw new Error('invalid_recovery'); }
  if (!row || typeof row !== 'object' || Array.isArray(row)) throw new Error('invalid_recovery');
  const value = row as Recovery;
  if (Object.keys(value).sort().join(',') !== 'digest,key' || !/^collection-[a-f0-9]{48}$/.test(value.key) || !/^[a-f0-9]{64}$/.test(value.digest)) throw new Error('invalid_recovery');
  return value;
}
export async function claimCreation(slot: string, digest: string, signal: AbortSignal) {
  if (!navigator.locks?.request) throw new Error('lock_unavailable');
  return navigator.locks.request(slot, { signal }, () => {
    signal.throwIfAborted(); const existing = readCreation(slot);
    if (existing) return { record: existing, claimed: false };
    const record = { key: 'collection-' + Array.from(crypto.getRandomValues(new Uint8Array(24)), b => b.toString(16).padStart(2, '0')).join(''), digest };
    localStorage.setItem(slot, JSON.stringify(record)); return { record, claimed: true };
  });
}
export async function clearCreation(slot: string, record: Recovery) {
  if (!navigator.locks?.request) throw new Error('lock_unavailable');
  return navigator.locks.request(slot, () => {
    const current = readCreation(slot);
    if (current?.key !== record.key || current.digest !== record.digest) throw new Error('recovery_changed');
    localStorage.removeItem(slot);
  });
}
export function matchesCreation(receipt: CollectionReceipt, record: Recovery, space: string) {
  return !!receipt && Object.keys(receipt).sort().join(',') === 'collection_id,committed_at,request_digest,result_revision,space_id' && /^[a-f0-9]{48}$/.test(receipt.collection_id) && receipt.space_id === space && receipt.request_digest === record.digest && receipt.result_revision === 1 && typeof receipt.committed_at === 'string' && Number.isFinite(Date.parse(receipt.committed_at));
}
