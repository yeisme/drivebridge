import type { components } from '../../generated/personal-workspace';

type Schemas = components['schemas'];
export type AutomationChange = Schemas['AutomationChangeRequest'];
export type AutomationExecute = Schemas['AutomationExecuteRequest'];
export type AutomationReceipt = Schemas['AutomationReceiptEnvelope']['data'];
export type AutomationPreview = Schemas['AutomationPreviewEnvelope']['data'];
export type AutomationAction = 'change' | 'execute';
export type AutomationRecovery = { key: string; digest: string };
export const automationSlot = (scope: string, space: string, action: AutomationAction, id = '') => 'drivebridge:automation:' + JSON.stringify([scope, space, action, id]);

export async function automationDigest(target: string, action: AutomationAction, input: AutomationChange | AutomationExecute) {
  let value: object;
  if (action === 'change') {
    const q = input as AutomationChange;
    value = { space: target, input: { action: q.action, action_detail: q.action_detail, prefix: q.prefix, ...(q.policy_id ? { policy_id: q.policy_id } : {}), budget_items: q.budget_items, enabled: q.enabled, clear_pause: q.clear_pause, expected_revision: q.expected_revision } };
  } else {
    const q = input as AutomationExecute;
    value = { rule: target, input: { expected_revision: q.expected_revision, preview_digest: q.preview_digest } };
  }
  // Mirror Go encoding/json's HTML and line-separator escaping, not user prose.
  const raw = JSON.stringify(value).replace(/[<>&\u2028\u2029]/g, character => '\\u' + character.charCodeAt(0).toString(16).padStart(4, '0'));
  return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(raw))), byte => byte.toString(16).padStart(2, '0')).join('');
}
export function readAutomationRecovery(slot: string): AutomationRecovery | null {
  const raw = localStorage.getItem(slot);
  if (raw === null) return null;
  if (raw.length > 256) throw new Error('invalid_recovery');
  let value: unknown;
  try { value = JSON.parse(raw); } catch { throw new Error('invalid_recovery'); }
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid_recovery');
  const record = value as AutomationRecovery;
  if (Object.keys(record).sort().join(',') !== 'digest,key' || !/^automation-[a-f0-9]{48}$/.test(record.key) || !/^[a-f0-9]{64}$/.test(record.digest)) throw new Error('invalid_recovery');
  return record;
}
export async function claimAutomation(slot: string, digest: string, signal: AbortSignal) {
  if (!navigator.locks?.request) throw new Error('lock_unavailable');
  return navigator.locks.request(slot, { signal }, () => {
    signal.throwIfAborted();
    const existing = readAutomationRecovery(slot);
    if (existing) return { record: existing, claimed: false };
    const record = { key: 'automation-' + Array.from(crypto.getRandomValues(new Uint8Array(24)), byte => byte.toString(16).padStart(2, '0')).join(''), digest };
    localStorage.setItem(slot, JSON.stringify(record));
    return { record, claimed: true };
  });
}
export async function clearAutomation(slot: string, record: AutomationRecovery) {
  if (!navigator.locks?.request) throw new Error('lock_unavailable');
  return navigator.locks.request(slot, () => {
    const current = readAutomationRecovery(slot);
    if (current?.key !== record.key || current.digest !== record.digest) throw new Error('recovery_changed');
    localStorage.removeItem(slot);
  });
}
export function matchesAutomation(receipt: AutomationReceipt, record: AutomationRecovery, space: string, action: AutomationAction, id?: string) {
  return !!receipt && Object.keys(receipt).sort().join(',') === 'action,committed_at,request_digest,result_revision,rule_id,space_id' && receipt.action === 'automation.' + action && receipt.space_id === space && /^[a-f0-9]{48}$/.test(receipt.rule_id) && (!id || receipt.rule_id === id) && receipt.request_digest === record.digest && Number.isSafeInteger(receipt.result_revision) && receipt.result_revision > 0 && typeof receipt.committed_at === 'string' && Number.isFinite(Date.parse(receipt.committed_at));
}
export function validatePreview(value: AutomationPreview, space: string, id: string) {
  if (!value || value.rule_id !== id || value.space_id !== space || !Number.isSafeInteger(value.revision) || value.revision < 1 || !Number.isSafeInteger(value.checkpoint) || value.checkpoint < 0 || !Number.isSafeInteger(value.scanned) || value.scanned < 0 || value.scanned > 200 || !['complete', 'partial'].includes(value.coverage) || !/^[a-f0-9]{64}$/.test(value.preview_digest) || typeof value.executable !== 'boolean' || !Array.isArray(value.items) || value.items.length > 200 || value.items.length > value.scanned || !Number.isSafeInteger(value.policy_max_bytes) || value.policy_max_bytes < 0 || (value.policy_expires_at !== null && (typeof value.policy_expires_at !== 'string' || !Number.isFinite(Date.parse(value.policy_expires_at))))) throw new Error('invalid_preview');
  for (const item of value.items) if (!item || !/^[a-f0-9]{48}$/.test(item.file_id) || !/^[a-f0-9]{48}$/.test(item.version_id) || typeof item.path !== 'string' || item.path.length > 4096 || !Number.isSafeInteger(item.size_bytes) || item.size_bytes < 0) throw new Error('invalid_preview');
  return value;
}
