import { api, type Operation } from '../../api';
import { statFile } from '../../info-workspace/client';
import type { components } from '../../generated/personal-workspace';
type Schemas = components['schemas'];
export type Submission = Schemas['SubmissionEnvelope']['data'];
export type SubmissionPage = Schemas['SubmissionPageEnvelope']['data'];
export type SubmissionDecision = Schemas['SubmissionDecisionRequest'];
export type SubmissionArchive = Schemas['ArchiveSubmissionRequest'];
export const archiveSubmission = (id: string, body: SubmissionArchive, key: string, signal: AbortSignal) => api<Schemas['ArchiveSubmissionEnvelope']['data']>('/inbox-submissions/' + encodeURIComponent(id) + '/archive', 'POST', body, { 'Idempotency-Key': key }, signal);
export const getArchiveOperation = (id: string, signal: AbortSignal) => api<{ operation: Operation }>('/operations/' + encodeURIComponent(id), 'GET', undefined, {}, signal);
export const resumeArchiveOperation = (id: string, signal: AbortSignal) => api<Operation>('/operations/' + encodeURIComponent(id) + '/resume', 'POST', {}, {}, signal);
export const getArchiveFile = statFile;
export const decideSubmission = (id: string, body: SubmissionDecision, key: string, signal: AbortSignal) => api<Submission>('/inbox-submissions/' + encodeURIComponent(id) + '/decision', 'POST', body, { 'Idempotency-Key': key }, signal);
export function listSubmissions(state: Submission['state'] | '', cursor: string, signal: AbortSignal, space = '') {
  const query = new URLSearchParams({ limit: '50' });
  if (space) query.set('space_id', space);
  if (state) query.set('state', state);
  if (cursor) query.set('cursor', cursor);
  return api<SubmissionPage>('/inbox-submissions?' + query, 'GET', undefined, {}, signal);
}
export const getSubmission = (id: string, signal: AbortSignal) => api<Submission>('/inbox-submissions/' + encodeURIComponent(id), 'GET', undefined, {}, signal);
