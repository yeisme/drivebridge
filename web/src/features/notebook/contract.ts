import type { NotebookEntry } from './notebook-navigation';
import type { NoteIdentity } from './notebook-state';

export type NotebookLifecycle = 'loading' | 'empty' | 'ready' | 'partial' | 'stale' | 'forbidden' | 'offline' | 'conflict' | 'unknown' | 'unsupported' | 'error';

export interface NotebookScopeContract {
  actor: string;
  tenant?: string;
  space: string;
  source: string;
}

export interface NotebookFixedIdentity extends NotebookScopeContract {
  object: string;
  version: string;
  revision: string | number;
}

export interface NotebookListProjection {
  entries: NotebookEntry[];
  scanned: number;
  unavailable: number;
  stale: number;
  nextCursor: string | null;
  lifecycle: NotebookLifecycle;
  reason?: string;
}

export interface NotebookPageMeta {
  file_id: string;
  space_id: string;
  version_id: string;
  path: string;
  parent_file_id: string;
  icon: string;
  revision: number;
}

export type { NoteIdentity };
