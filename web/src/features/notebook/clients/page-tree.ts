import { api } from '../../../api';
import type { NotebookPageMeta } from '../contract';

export type NotebookPageTreePage = { items: NotebookPageMeta[]; next_cursor?: string | null };

export function readNotebookPageTree(space: string, cursor: string | null, signal: AbortSignal) {
  const suffix = cursor ? `&cursor=${encodeURIComponent(cursor)}` : '';
  return api<NotebookPageTreePage>(`/notebook/pages?space_id=${encodeURIComponent(space)}&limit=100${suffix}`, 'GET', undefined, {}, signal);
}
