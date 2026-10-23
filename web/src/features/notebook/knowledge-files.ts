import { queryFiles, statFile } from "../../info-workspace/client";
import { canOpenMarkdownNotebook } from "./notebook-capability";
import type { NotebookEntry } from "./notebook-navigation";
import { readNotebookDirectoryPage, type NotebookDriveEntry } from './clients/directory-tree';

export interface KnowledgeFilePage {
  entries: NotebookEntry[];
  nextCursor: string | null;
  scanned: number;
  unavailable: number;
  stale: number;
}


// The existing file query has no extension/OR filter. Examine one bounded page
// and disclose coverage rather than mislabelling a partial scan as all notes.
async function readLegacyKnowledgeFiles(space: string, cursor: string | null, signal: AbortSignal): Promise<KnowledgeFilePage> {
  const result = await queryFiles({ mode: "exact", space_ids: [space], sort: [{ field: "name", direction: "asc" }], limit: 50, cursor }, signal);
  const entries: NotebookEntry[] = [];
  let unavailable = 0, stale = 0;
  for (let offset = 0; offset < result.items.length; offset += 6) {
    signal.throwIfAborted();
    const batch = result.items.slice(offset, offset + 6);
    const observations = await Promise.allSettled(batch.map(item => statFile(item.file_id, signal)));
    signal.throwIfAborted();
    observations.forEach((observation, index) => {
      if (observation.status === "rejected") {
        const error = observation.reason as { status?: number; name?: string };
        if (error.status === 401 || error.status === 403 || error.name === "AbortError") throw observation.reason;
        unavailable++;
        return;
      }
      const file = observation.value;
      const fixed = batch[index];
      if (file.id !== fixed.file_id || file.space_id !== space || file.trashed) { unavailable++; return; }
      if (file.version_id !== fixed.version_id) { stale++; return; }
      if (!canOpenMarkdownNotebook(file, space)) return;
      entries.push({ title: file.path.split("/").pop() || file.path, path: file.path,
        ref: { sourceId: space, objectId: fixed.file_id, revision: fixed.version_id } });
    });
  }
  return { entries, nextCursor: result.next_cursor, scanned: result.items.length, unavailable, stale };
}

export async function readKnowledgeFiles(space: string, cursor: string | null, signal: AbortSignal, directoryPath?: string): Promise<KnowledgeFilePage> {
  if (directoryPath === undefined) return readLegacyKnowledgeFiles(space, cursor, signal);
  const result = await readNotebookDirectoryPage(space, directoryPath, cursor, signal);
  const entries: NotebookEntry[] = [];
  let unavailable = 0;
  for (const item of Array.isArray(result.entries) ? result.entries : []) {
    if (item.kind !== 'file' || !item.file) continue;
    const file = item.file as NotebookDriveEntry['file'];
    if (!file || file.space_id !== space || file.trashed) { unavailable += 1; continue; }
    if (!canOpenMarkdownNotebook(file, space)) continue;
    entries.push({ title: file.path.split('/').at(-1) || file.path, path: file.path, ref: { sourceId: space, objectId: file.id, revision: file.version_id } });
  }
  return { entries, nextCursor: result.next_cursor || null, scanned: Array.isArray(result.entries) ? result.entries.length : 0, unavailable, stale: 0 };
}
