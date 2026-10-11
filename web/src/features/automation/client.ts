import { api } from '../../api';

// Safe view of the existing rule response. Do not mirror owner persistence
// or expose arbitrary provider/error fields in the browser.
export function projectRules(value: unknown, space: string) {
  if (value === null) return [];
  if (!Array.isArray(value) || value.length > 200) throw new Error('invalid_contract');
  const rows = value.map(raw => {
    if (!raw || typeof raw !== 'object' || Array.isArray(raw)) throw new Error('invalid_contract');
    const row = raw as Record<string, unknown>;
    const text = (key: string, max: number) => {
      const field = row[key];
      if (typeof field !== 'string' || field.length > max) throw new Error('invalid_contract');
      return field;
    };
    const id = text('rule_id', 256);
    const action = text('action', 96);
    if (!id || !action || row.space_id !== space || typeof row.enabled !== 'boolean' || typeof row.paused !== 'boolean' || !Number.isSafeInteger(row.revision) || Number(row.revision) <= 0) throw new Error('invalid_contract');
    const budget = row.budget_items === undefined ? 0 : row.budget_items;
    if (typeof budget !== 'number' || !Number.isSafeInteger(budget) || budget < 0) throw new Error('invalid_contract');
    const revision = Number(row.revision);
    const updatedAt = text('updated_at', 64);
    if (!Number.isFinite(Date.parse(updatedAt))) throw new Error('invalid_contract');
    const policyID = row.policy_id === undefined ? '' : text('policy_id', 256);
    return { id, action, detail: text('action_detail', 2048), prefix: text('prefix', 4096), revision, enabled: row.enabled, paused: row.paused, budget, updatedAt, policyID };
  });
  if (new Set(rows.map(row => row.id)).size !== rows.length) throw new Error('invalid_contract');
  return rows;
}
export type AutomationRuleView = ReturnType<typeof projectRules>[number];
export async function readRules(space: string, signal: AbortSignal) {
  if (!space) throw new Error('invalid_request');
  return projectRules(await api<unknown>('/spaces/' + encodeURIComponent(space) + '/automation-rules', 'GET', undefined, {}, signal), space);
}
