import { api } from '../../../api';

export interface NotebookDriveEntry {
  kind: 'file' | 'directory';
  path: string;
  file?: { id: string; space_id: string; path: string; version_id: string; size_bytes: number; mime_type: string; trashed: boolean; updated_at: string };
}

export function readNotebookDirectoryPage(space: string, directory: string, cursor: string | null, signal: AbortSignal) {
  const query = new URLSearchParams({ space_id: space, path: directory, cursor: cursor ?? '' });
  return api<{ entries: NotebookDriveEntry[]; next_cursor?: string; total?: number }>(`/browser?${query}`, 'GET', undefined, {}, signal);
}

export async function readNotebookDirectory(space: string, directory: string, signal: AbortSignal) {
  const entries: NotebookDriveEntry[] = [];
  let cursor = '';
  for (let page = 0; page < 10; page += 1) {
    const result = await readNotebookDirectoryPage(space, directory, cursor || null, signal);
    entries.push(...(Array.isArray(result.entries) ? result.entries : []));
    if (!result.next_cursor) return { entries, partial: false, total: result.total ?? entries.length };
    cursor = result.next_cursor;
  }
  return { entries, partial: true, total: entries.length };
}
