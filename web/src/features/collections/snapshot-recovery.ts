import type { components } from '../../generated/personal-workspace';
export type SnapshotReceipt = components['schemas']['SnapshotCreationReceiptEnvelope']['data'];
export type SnapshotRequest = components['schemas']['SnapshotCreationRequest'];
export type SnapshotRecovery = { key: string; digest: string };
export const snapshotSlot = (scope: string, id: string) => 'drivebridge:snapshot-create:' + JSON.stringify([scope, id]);
export async function snapshotDigest(id: string, revision: number) {
  const raw = JSON.stringify({ collection_id: id, expected_revision: revision }).replace(/[<>&\u2028\u2029]/g, ch => '\\u' + ch.charCodeAt(0).toString(16).padStart(4, '0'));
  return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(raw))), byte => byte.toString(16).padStart(2, '0')).join('');
}
export function readSnapshotRecovery(slot: string): SnapshotRecovery | null {
  const raw = localStorage.getItem(slot); if (raw === null) return null;
  if (raw.length > 256) throw new Error('invalid_recovery');
  let value: unknown; try { value = JSON.parse(raw); } catch { throw new Error('invalid_recovery'); }
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid_recovery');
  const row = value as SnapshotRecovery;
  if (Object.keys(row).sort().join(',') !== 'digest,key' || !/^snapshot-[a-f0-9]{48}$/.test(row.key) || !/^[a-f0-9]{64}$/.test(row.digest)) throw new Error('invalid_recovery');
  return row;
}
export async function claimSnapshot(slot: string, digest: string, signal: AbortSignal) {
  if (!navigator.locks?.request) throw new Error('lock_unavailable');
  return navigator.locks.request(slot, { signal }, () => {
    signal.throwIfAborted(); const existing = readSnapshotRecovery(slot);
    if (existing) return { record: existing, claimed: false };
    const record = { key: 'snapshot-' + Array.from(crypto.getRandomValues(new Uint8Array(24)), byte => byte.toString(16).padStart(2, '0')).join(''), digest };
    localStorage.setItem(slot, JSON.stringify(record)); return { record, claimed: true };
  });
}
export async function clearSnapshot(slot: string, record: SnapshotRecovery) {
  if (!navigator.locks?.request) throw new Error('lock_unavailable');
  return navigator.locks.request(slot, () => {
    const current = readSnapshotRecovery(slot);
    if (current?.key !== record.key || current.digest !== record.digest) throw new Error('recovery_changed');
    localStorage.removeItem(slot);
  });
}
export async function matchesSnapshot(row: SnapshotReceipt, record: SnapshotRecovery, id: string) {
  return !!row && Object.keys(row).sort().join(',') === 'collection_id,collection_revision,committed_at,request_digest,snapshot_id' && row.collection_id === id && /^[a-f0-9]{48}$/.test(row.snapshot_id) && Number.isSafeInteger(row.collection_revision) && row.collection_revision > 0 && row.request_digest === record.digest && await snapshotDigest(id, row.collection_revision) === record.digest && typeof row.committed_at === 'string' && Number.isFinite(Date.parse(row.committed_at));
}
