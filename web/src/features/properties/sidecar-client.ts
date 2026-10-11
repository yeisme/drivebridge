import type { components } from '../../generated/personal-workspace';
import { validCatalog, type Catalog, type Definition, type Recovery } from './property-recovery';
export type Sidecar = components['schemas']['PropertySidecarViewEnvelope']['data'];
export type SidecarPatch = components['schemas']['PropertySidecarPatch'];
export type SidecarReceipt = components['schemas']['PropertySidecarReceiptEnvelope']['data'];
export type PropertyValue = SidecarPatch['updates'][string];
export const sidecarSlot = (scope: string, file: string, version: string) => 'drivebridge:property-sidecar:' + JSON.stringify([scope, file, version]);
const textValid = (value: unknown, max: number): value is string => typeof value === 'string' && !value.includes('\0') && new TextEncoder().encode(value).length <= max && new TextDecoder().decode(new TextEncoder().encode(value)) === value;
export function validValue(value: unknown, type: Definition['type']) {
  switch (type) {
    case 'text': return textValid(value, 8192);
    case 'number': return typeof value === 'number' && Number.isFinite(value) && Math.abs(value) <= Number.MAX_SAFE_INTEGER;
    case 'checkbox': return typeof value === 'boolean';
    case 'date': return typeof value === 'string' && /^(?!0000)\d{4}-\d{2}-\d{2}$/.test(value) && !Number.isNaN(Date.parse(value)) && new Date(value).toISOString().slice(0, 10) === value;
    case 'list': return Array.isArray(value) && value.length <= 100 && value.every(item => textValid(item, 1024));
    case 'reference': return textValid(value, 2048) && /^drivebridge:\/\/[^\s/@?#%]+\/file\/[a-f0-9]{48}\/version\/[a-f0-9]{48}$/.test(value);
  }
}
export function parseValue(raw: string, type: Definition['type']): PropertyValue {
  let value: unknown = raw;
  if (type === 'number' || type === 'list' || type === 'checkbox') {
    try { value = JSON.parse(raw); } catch { throw new Error('invalid_value'); }
    // Number editing explicitly uses browser JSON numbers; never silently
    // round a user-entered integer outside the safe range into a valid value.
    if (type === 'number' && JSON.stringify(value) !== raw) throw new Error('invalid_value');
  }
  if (!validValue(value, type) || new TextEncoder().encode(JSON.stringify(value)).length > 16 * 1024) throw new Error('invalid_value');
  return value as PropertyValue;
}
export function validSidecar(value: Sidecar, catalog: Catalog, space: string, file: string, version: string) {
  if (!validCatalog(catalog, space) || !value || Object.keys(value).sort().join(',') !== 'definition_revision,file_id,revision,storage_kind,unknown_keys,values,version_id' || value.file_id !== file || value.version_id !== version || value.storage_kind !== 'sidecar' || value.definition_revision !== catalog.revision || !Number.isSafeInteger(value.revision) || value.revision < 0 || value.revision >= Number.MAX_SAFE_INTEGER || !value.values || typeof value.values !== 'object' || Array.isArray(value.values) || !Array.isArray(value.unknown_keys) || value.unknown_keys.length > 200 || value.unknown_keys.some(key => typeof key !== 'string' || !/^[a-z][a-z0-9_]{0,63}$/.test(key)) || new Set(value.unknown_keys).size !== value.unknown_keys.length) return false;
  const entries = Object.entries(value.values);
  return entries.length + value.unknown_keys.length <= 200 && entries.every(([key, val]) => { const definition = catalog.definitions.find(row => row.key === key); return definition && validValue(val, definition.type) && !value.unknown_keys.includes(key); });
}
export async function sidecarDigest(file: string, input: SidecarPatch) {
  // The wire payload uses these same JS numbers. Go compacts RawMessage rather
  // than reformatting its number literals; unrelated stored values never enter it.
  const updates = Object.fromEntries(Object.entries(input.updates).sort(([a], [b]) => a < b ? -1 : a > b ? 1 : 0));
  const raw = JSON.stringify({ file_id: file, request: { base_version_id: input.base_version_id, definition_revision: input.definition_revision, expected_revision: input.expected_revision, updates, remove_keys: input.remove_keys } }).replace(/[<>&\u2028\u2029]/g, ch => '\\u' + ch.charCodeAt(0).toString(16).padStart(4, '0'));
  return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(raw))), byte => byte.toString(16).padStart(2, '0')).join('');
}
export function matchesSidecarReceipt(row: SidecarReceipt, record: Recovery, file: string, version: string, expected?: number) {
  return !!row && Object.keys(row).sort().join(',') === 'committed_at,file_id,request_digest,result_revision,version_id' && row.file_id === file && row.version_id === version && row.request_digest === record.digest && Number.isSafeInteger(row.result_revision) && row.result_revision >= 1 && (expected === undefined || row.result_revision === expected + 1) && typeof row.committed_at === 'string' && Number.isFinite(Date.parse(row.committed_at));
}
