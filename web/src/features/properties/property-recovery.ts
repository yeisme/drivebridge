import type { components } from '../../generated/personal-workspace';
export type Catalog = components['schemas']['PropertyDefinitionViewEnvelope']['data'];
export type Definition = Catalog['definitions'][number];
export type Update = components['schemas']['PropertyDefinitionUpdate'];
export type Receipt = components['schemas']['PropertyDefinitionReceiptEnvelope']['data'];
export type Recovery = { key: string; digest: string };
export const TYPES: Definition['type'][] = ['text', 'number', 'date', 'checkbox', 'list', 'reference'];
export const typeLabel: Record<Definition['type'], string> = { text: '文本', number: '数字', date: '日期', checkbox: '复选框', list: '列表', reference: '引用' };
export function validDefinition(row: Definition) {
  return !!row && Object.keys(row).sort().join(',') === 'display_name,group,key,type' && typeof row.key === 'string' && /^[a-z][a-z0-9_]{0,63}$/.test(row.key) && TYPES.includes(row.type) && typeof row.display_name === 'string' && !!row.display_name.trim() && [...row.display_name].length <= 120 && typeof row.group === 'string' && [...row.group].length <= 80 && !/[\p{Cc}]/u.test(row.display_name + row.group) && new TextDecoder().decode(new TextEncoder().encode(row.display_name + row.group)) === row.display_name + row.group;
}
export function validCatalog(row: Catalog, space: string) {
  return !!row && row.space_id === space && Number.isSafeInteger(row.revision) && row.revision >= 0 && Array.isArray(row.definitions) && row.definitions.length <= 200 && row.definitions.every(validDefinition) && new Set(row.definitions.map(item => item.key)).size === row.definitions.length;
}
export const propertySlot = (scope: string, space: string) => 'drivebridge:property-definitions:' + JSON.stringify([scope, space]);
export async function propertyDigest(space: string, input: Update) {
  const raw = JSON.stringify({ space_id: space, request: { expected_revision: input.expected_revision, definitions: input.definitions.map(row => ({ key: row.key, display_name: row.display_name, group: row.group, type: row.type })) } }).replace(/[<>&\u2028\u2029]/g, ch => '\\u' + ch.charCodeAt(0).toString(16).padStart(4, '0'));
  return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(raw))), byte => byte.toString(16).padStart(2, '0')).join('');
}
export function readRecovery(slot: string): Recovery | null {
  const raw = localStorage.getItem(slot); if (raw === null) return null;
  if (raw.length > 256) throw new Error('invalid_recovery');
  let value: unknown; try { value = JSON.parse(raw); } catch { throw new Error('invalid_recovery'); }
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid_recovery');
  const row = value as Recovery;
  if (Object.keys(row).sort().join(',') !== 'digest,key' || !/^properties-[a-f0-9]{48}$/.test(row.key) || !/^[a-f0-9]{64}$/.test(row.digest)) throw new Error('invalid_recovery');
  return row;
}
export async function claimUpdate(slot: string, digest: string, signal: AbortSignal) {
  if (!navigator.locks?.request) throw new Error('lock_unavailable');
  return navigator.locks.request(slot, { signal }, () => {
    signal.throwIfAborted(); const existing = readRecovery(slot);
    if (existing) return { record: existing, claimed: false };
    const record = { key: 'properties-' + Array.from(crypto.getRandomValues(new Uint8Array(24)), byte => byte.toString(16).padStart(2, '0')).join(''), digest };
    localStorage.setItem(slot, JSON.stringify(record)); return { record, claimed: true };
  });
}
export async function clearUpdate(slot: string, record: Recovery) {
  if (!navigator.locks?.request) throw new Error('lock_unavailable');
  return navigator.locks.request(slot, () => {
    const current = readRecovery(slot);
    if (current?.key !== record.key || current.digest !== record.digest) throw new Error('recovery_changed');
    localStorage.removeItem(slot);
  });
}
export function matchesReceipt(row: Receipt, record: Recovery, space: string, expected?: number) {
  return !!row && Object.keys(row).sort().join(',') === 'committed_at,request_digest,result_revision,space_id' && row.space_id === space && row.request_digest === record.digest && Number.isSafeInteger(row.result_revision) && row.result_revision >= 1 && (expected === undefined || row.result_revision === expected + 1) && typeof row.committed_at === 'string' && Number.isFinite(Date.parse(row.committed_at));
}
