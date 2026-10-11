import type { components } from '../../generated/personal-workspace';
export type RegisterResource = components['schemas']['RegisterResourceRequest'];
export type ResourceReceipt = components['schemas']['ResourceRegistrationReceiptEnvelope']['data'];
export type Recovery = { key: string; digest: string };
export const resourceSlot = (scope: string) => 'drivebridge:resource-register:' + scope;
export async function resourceDigest(input: RegisterResource): Promise<string> {
  const bytes = new TextEncoder().encode(JSON.stringify([input.space_id, input.kind, input.display_name, input.file_ref ?? '', input.collection_snapshot_id ?? '']));
  return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', bytes)), b => b.toString(16).padStart(2, '0')).join('');
}
export function readResourceRecovery(slot: string): Recovery | null {
  const raw = localStorage.getItem(slot); if (raw === null) return null;
  if (raw.length > 256) throw new Error('invalid_recovery');
  let value: unknown;
  try { value = JSON.parse(raw); } catch { throw new Error('invalid_recovery'); }
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid_recovery');
  const row = value as Recovery;
  if (Object.keys(row).sort().join(',') !== 'digest,key' || !/^resource-[a-f0-9]{48}$/.test(row.key) || !/^[a-f0-9]{64}$/.test(row.digest)) throw new Error('invalid_recovery');
  return row;
}
export async function claimResource(slot: string, digest: string, signal: AbortSignal) {
  if (!navigator.locks?.request) throw new Error('lock_unavailable');
  return navigator.locks.request(slot, { signal }, () => {
    signal.throwIfAborted();
    const existing = readResourceRecovery(slot);
    if (existing) return { record: existing, claimed: false };
    const record = { key: 'resource-' + Array.from(crypto.getRandomValues(new Uint8Array(24)), b => b.toString(16).padStart(2, '0')).join(''), digest };
    localStorage.setItem(slot, JSON.stringify(record));
    return { record, claimed: true };
  });
}
export async function clearResource(slot: string, record: Recovery) {
  if (!navigator.locks?.request) throw new Error('lock_unavailable');
  return navigator.locks.request(slot, () => {
    const current = readResourceRecovery(slot);
    if (current?.key !== record.key || current.digest !== record.digest) throw new Error('recovery_changed');
    localStorage.removeItem(slot);
  });
}
export async function matchesResourceReceipt(receipt: ResourceReceipt, record: Recovery) {
  const row = receipt?.resource;
  if (!row || typeof row.descriptor_id !== 'string' || !row.descriptor_id || !Number.isSafeInteger(receipt.result_revision) || receipt.result_revision < 1 || row.descriptor_revision !== receipt.result_revision || row.check_state !== 'unknown' || !Number.isFinite(Date.parse(receipt.committed_at)) || !Number.isFinite(Date.parse(row.created_at)) || !['skill', 'template', 'reference_pack'].includes(row.kind) || typeof row.space_id !== 'string' || typeof row.display_name !== 'string') return false;
  if (!(typeof row.file_ref === 'string' && row.file_ref.length > 0 && row.collection_snapshot_id === undefined && row.kind !== 'reference_pack' || typeof row.collection_snapshot_id === 'string' && row.collection_snapshot_id.length > 0 && row.file_ref === undefined)) return false;
  return await resourceDigest(row) === record.digest;
}
