import type { OrganizationPlanInput } from './client';
export type OrganizationIntent = { key: string } & ({ action: 'organize_plan'; input: OrganizationPlanInput } | { action: 'organize_confirm'; plan: string; revision: number; digest: string } | { action: 'organize_apply'; plan: string; revision: number } | { action: 'organize_compensation_plan'; plan: string });
export function readOrganizationIntent(slot: string): OrganizationIntent | null {
  const raw = localStorage.getItem(slot); if (raw === null) return null;
  const value: unknown = JSON.parse(raw);
  if (!value || typeof value !== 'object' || !('key' in value) || typeof value.key !== 'string' || !/^organize-[a-f0-9]{48}$/.test(value.key) || !('action' in value)) throw new Error('invalid_recovery');
  if (value.action === 'organize_plan' && 'input' in value && value.input && typeof value.input === 'object') {
    const input = value.input;
    if (!('refs' in input) || !Array.isArray(input.refs) || !input.refs.length || input.refs.length > 200 || !input.refs.every(ref => typeof ref === 'string' && ref.length <= 2048) || !('actions' in input) || !Array.isArray(input.actions) || input.actions.length !== 1 || !('policy_id' in input) || typeof input.policy_id !== 'string') throw new Error('invalid_recovery');
    const action = input.actions[0];
    if (!action || typeof action !== 'object' || !('kind' in action) || (action.kind !== 'add_label' && action.kind !== 'move')) throw new Error('invalid_recovery');
    const actions = action.kind === 'move' ? [{ kind: 'move', target_prefix: '/整理后/' }] : [{ kind: 'add_label', namespace: 'rule', value: 'reference' }];
    if (JSON.stringify(input.actions) !== JSON.stringify(actions)) throw new Error('invalid_recovery');
    return { key: value.key, action: 'organize_plan', input: { refs: input.refs, actions, policy_id: input.policy_id } };
  }
  if ('plan' in value && typeof value.plan === 'string' && value.plan) {
    if (value.action === 'organize_compensation_plan') return { key: value.key, action: value.action, plan: value.plan };
    if (value.action === 'organize_confirm' && 'revision' in value && typeof value.revision === 'number' && Number.isSafeInteger(value.revision) && value.revision > 0 && 'digest' in value && typeof value.digest === 'string') return { key: value.key, action: value.action, plan: value.plan, revision: value.revision, digest: value.digest };
    if (value.action === 'organize_apply' && 'revision' in value && typeof value.revision === 'number' && Number.isSafeInteger(value.revision) && value.revision > 0) return { key: value.key, action: value.action, plan: value.plan, revision: value.revision };
  }
  throw new Error('invalid_recovery');
}
export type OrganizationIntentRequest = { action: 'organize_plan'; input: OrganizationPlanInput } | { action: 'organize_confirm'; plan: string; revision: number; digest: string } | { action: 'organize_apply'; plan: string; revision: number } | { action: 'organize_compensation_plan'; plan: string };
export async function claimOrganizationIntent(slot: string, intent: OrganizationIntentRequest, signal: AbortSignal) {
  if (!navigator.locks?.request) throw new Error('lock_unavailable');
  return navigator.locks.request(slot, { signal }, () => {
    signal.throwIfAborted(); const current = readOrganizationIntent(slot); if (current) return { intent: current, claimed: false };
    const key = 'organize-' + Array.from(crypto.getRandomValues(new Uint8Array(24)), byte => byte.toString(16).padStart(2, '0')).join('');
    localStorage.setItem(slot, JSON.stringify({ ...intent, key }));
    const saved = readOrganizationIntent(slot); if (!saved) throw new Error('storage_unavailable'); return { intent: saved, claimed: true };
  });
}
export async function clearOrganizationIntent(slot: string, key: string) {
  if (!navigator.locks?.request) throw new Error('lock_unavailable');
  await navigator.locks.request(slot, () => { if (readOrganizationIntent(slot)?.key !== key) throw new Error('recovery_changed'); localStorage.removeItem(slot); });
}
