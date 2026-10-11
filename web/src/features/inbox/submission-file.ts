import { api, contextURL, getDataContext, pinnedFile, type DriveFile } from '../../api';
import { getSubmission, type Submission } from './submission-client';

// Resolve through public owner contracts. Never substitute the current head or
// trust a caller-provided URL, derivative URL, MIME claim or storage path.
export async function readSubmissionFile(submission: Submission, signal: AbortSignal): Promise<DriveFile> {
  const revision = getDataContext().revision;
  const check = () => { signal.throwIfAborted(); if (revision !== getDataContext().revision) throw new DOMException('Context changed', 'AbortError'); };
  const latest = await getSubmission(submission.submission_id, signal);
  check();
  if (!submission.file_ref || !submission.version_id || latest.submission_id !== submission.submission_id || latest.space_id !== submission.space_id || latest.file_ref !== submission.file_ref || latest.version_id !== submission.version_id) throw new Error('fixed_source_changed');
  const current = await api<{ file: DriveFile }>('/files/' + encodeURIComponent(submission.file_ref), 'GET', undefined, {}, signal);
  check();
  if (!current.file || current.file.id !== submission.file_ref || current.file.space_id !== submission.space_id || current.file.trashed) throw new Error('fixed_source_unavailable');
  const pinned = await pinnedFile(current.file, submission.version_id, {}, signal);
  check();
  const file = pinned.file;
  if (!file || file.id !== submission.file_ref || file.space_id !== submission.space_id || file.version_id !== submission.version_id || typeof file.path !== 'string' || typeof file.mime_type !== 'string' || !Number.isSafeInteger(file.size_bytes) || file.size_bytes < 0) throw new Error('fixed_source_changed');
  return { id: file.id, space_id: file.space_id, version_id: file.version_id, path: latest.file_name || '文件名称未提供', mime_type: file.mime_type, size_bytes: file.size_bytes, trashed: false, updated_at: latest.updated_at };
}

export async function downloadSubmissionFile(submission: Submission, signal: AbortSignal): Promise<void> {
  const file = await readSubmissionFile(submission, signal);
  signal.throwIfAborted();
  const link = document.createElement('a');
  link.href = contextURL('/api/v1/files/' + encodeURIComponent(file.id) + '/content?version=' + encodeURIComponent(file.version_id));
  link.download = file.path;
  link.click();
}
