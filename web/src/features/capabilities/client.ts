import { api } from '../../api';

// Presentation projections of the legacy discovery API, not a second wire
// schema. Unknown fields (including URLs/provider data) never reach the UI.
const object = (value: unknown): Record<string, unknown> => {
  if (!value || typeof value !== 'object' || Array.isArray(value)) throw new Error('invalid_contract');
  return value as Record<string, unknown>;
};
const identifier = (value: unknown): value is string => typeof value === 'string' && /^[a-z][a-z0-9_.-]{0,95}$/.test(value);
const count = (value: unknown): value is number => typeof value === 'number' && Number.isSafeInteger(value) && value >= 0;
const boolean = (value: unknown): boolean => { if (typeof value !== 'boolean') throw new Error('invalid_contract'); return value; };

export function projectCapabilities(value: unknown) {
  const row = object(value);
  if (!Array.isArray(row.actions) || row.actions.length > 128 || !row.actions.every(identifier) || new Set(row.actions).size !== row.actions.length) throw new Error('invalid_contract');
  const limits = object(row.limits);
  const safeLimits = Object.fromEntries(['max_file_bytes', 'max_batch_items', 'space_quota_bytes', 'space_remaining_bytes'].map(key => {
    const value = limits[key];
    if (value !== undefined && value !== null && !count(value)) throw new Error('invalid_contract');
    return [key, value === undefined ? null : value];
  })) as Record<string, number | null>;
  return { actions: row.actions as string[], limits: safeLimits };
}

export function projectReadiness(value: unknown, expectedAction: string) {
  const row = object(value);
  if (row.action !== expectedAction) throw new Error('invalid_contract');
  const implemented = boolean(row.implemented), configured = boolean(row.configured), authorized = boolean(row.authorized);
  const availability = row.availability === 'available' || row.availability === 'blocked' ? row.availability : 'unknown';
  const dependency = ['none', 'ok', 'unavailable', 'not_probed'].includes(String(row.dependency_status)) ? String(row.dependency_status) : 'unknown';
  if (availability === 'available' && (!implemented || !authorized || !['none', 'ok'].includes(dependency))) throw new Error('invalid_contract');
  const observedAt = row.connection_observed_at === null ? null : typeof row.connection_observed_at === 'string' && Number.isFinite(Date.parse(row.connection_observed_at)) ? row.connection_observed_at : null;
  const reason = identifier(row.block_reason) ? row.block_reason : null;
  return { action: expectedAction, implemented, configured, authorized, availability, dependency, observedAt, reason };
}

export type Capabilities = ReturnType<typeof projectCapabilities>;
export type Readiness = ReturnType<typeof projectReadiness>;
export async function readCapabilities(space: string, signal: AbortSignal) {
  const query = space ? '?' + new URLSearchParams({ space_id: space }) : '';
  return projectCapabilities(await api<unknown>('/capabilities' + query, 'GET', undefined, {}, signal));
}
export async function readReadiness(action: string, space: string, signal: AbortSignal) {
  if (!identifier(action) || !space) throw new Error('invalid_request');
  return projectReadiness(await api<unknown>('/capabilities/check', 'POST', { action, space_id: space }, {}, signal), action);
}
