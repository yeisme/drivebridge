import { api } from '../../api';
import type { components } from '../../generated/personal-workspace';
type Schemas = components['schemas'];
export type AgentOrganization = Schemas['AgentOrganizationView'];
export type AgentGroup = Schemas['AgentGroupView'];
export type OrganizationPatch = Schemas['PatchOrganizationRequest'];
export type GroupChange =
  | { action: 'create'; body: Schemas['CreateGroupRequest'] }
  | { action: 'rename'; id: string; body: Schemas['RenameGroupRequest'] }
  | { action: 'delete'; id: string; body: Schemas['RevisionRequest'] };
export const getGroup = (id: string, signal: AbortSignal) => api<AgentGroup>(`/agent-groups/${encodeURIComponent(id)}`, 'GET', undefined, {}, signal);
export function changeGroup(change: GroupChange, key: string, signal: AbortSignal) {
  const path = change.action === 'create' ? '/agent-groups' : '/agent-groups/' + encodeURIComponent(change.id);
  return api<AgentGroup | Schemas['DeletedGroupEnvelope']['data']>(path, change.action === 'create' ? 'POST' : change.action === 'rename' ? 'PATCH' : 'DELETE', change.body, { 'Idempotency-Key': key }, signal);
}
export const getOrganization = (agent: string, signal: AbortSignal) => api<AgentOrganization>(`/agent-organizations/${encodeURIComponent(agent)}`, 'GET', undefined, {}, signal);
export const patchOrganization = (agent: string, body: OrganizationPatch, key: string, signal: AbortSignal) => api<AgentOrganization>(`/agent-organizations/${encodeURIComponent(agent)}`, 'PATCH', body, { 'Idempotency-Key': key }, signal);
export function listGroups(cursor: string, signal: AbortSignal) {
  const query = new URLSearchParams({ limit: '50' });
  if (cursor) query.set('cursor', cursor);
  return api<Schemas['GroupPageEnvelope']['data']>('/agent-groups?' + query, 'GET', undefined, {}, signal);
}
