import { api } from '../../api';
import type { components, operations } from '../../generated/personal-workspace';

type Schemas = components['schemas'];
export type Grant = Schemas['GrantView'];
export type GrantPage = Schemas['GrantPageEnvelope']['data'];
export type GrantChecks = Schemas['GrantChecksEnvelope']['data'];
export type CreateGrant = Schemas['CreateGrantRequest'];
export type GrantWrite = Schemas['GrantWriteEnvelope']['data'];
export type RotationReceipt = Schemas['RotationReplayEnvelope']['data'];
export type GrantChangeAction = Schemas['GrantChangeLookupRequest']['action'];
export type GrantChangeReceipt = Schemas['GrantChangeReceiptEnvelope']['data'];
export type GrantPatch = Schemas['PatchGrantRequest'];
export const getGrant = (id: string, signal: AbortSignal) => api<Grant>(`/agent-grants/${encodeURIComponent(id)}`, 'GET', undefined, {}, signal);
export const changeGrant = (id: string, action: GrantChangeAction, input: GrantPatch, key: string, signal: AbortSignal) => api<Grant>(`/agent-grants/${encodeURIComponent(id)}${action === 'revoke' ? '/revoke' : ''}`, action === 'revoke' ? 'POST' : 'PATCH', input, { 'Idempotency-Key': key }, signal);
export const lookupGrantChange = (id: string, action: GrantChangeAction, key: string, signal: AbortSignal) => api<GrantChangeReceipt>(`/agent-grants/${encodeURIComponent(id)}/changes/lookup`, 'POST', { action, idempotency_key: key } satisfies Schemas['GrantChangeLookupRequest'], {}, signal);
export const rotateGrant = (id: string, revision: number, key: string, signal: AbortSignal) => api<GrantWrite>(`/agent-grants/${encodeURIComponent(id)}/rotate`, 'POST', { expected_revision: revision } satisfies Schemas['RevisionRequest'], { 'Idempotency-Key': key }, signal);
export const lookupRotation = (id: string, key: string, signal: AbortSignal) => api<RotationReceipt>(`/agent-grants/${encodeURIComponent(id)}/rotate/lookup`, 'POST', { idempotency_key: key }, {}, signal);
export const createGrant = (input: CreateGrant, key: string, signal: AbortSignal) => api<GrantWrite>('/agent-grants', 'POST', input, { 'Idempotency-Key': key }, signal);
export const lookupGrant = (key: string, signal: AbortSignal) => api<Schemas['GrantReplayEnvelope']['data']>('/agent-grants/lookup', 'POST', { idempotency_key: key }, {}, signal);
export type GrantOrganizationFilter = Pick<NonNullable<operations['listAgentGrants']['parameters']['query']>, 'group_id' | 'ungrouped' | 'pinned' | 'order' | 'state'>;
export function listGrants(cursor: string, signal: AbortSignal, filter: GrantOrganizationFilter = {}) {
  const query = new URLSearchParams({ limit: '50' });
  for (const [key, value] of Object.entries(filter)) if (value !== undefined) query.set(key, String(value));
  if (cursor) query.set('cursor', cursor);
  return api<GrantPage>('/agent-grants?' + query, 'GET', undefined, {}, signal);
}
export function checkGrant(grant: Grant, signal: AbortSignal) {
  return api<GrantChecks>(`/agent-grants/${encodeURIComponent(grant.grant_id)}/checks`, 'POST',
    { expected_revision: grant.revision } satisfies Schemas['RevisionRequest'], {}, signal);
}
