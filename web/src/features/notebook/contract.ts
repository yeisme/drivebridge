import type { NotebookEntry } from './notebook-navigation';
import type { NoteIdentity } from './notebook-state';

export type NotebookLifecycle = 'loading' | 'empty' | 'ready' | 'partial' | 'stale' | 'forbidden' | 'offline' | 'conflict' | 'unknown' | 'unsupported' | 'error';

export interface NotebookScopeContract {
  actor: string;
  tenant?: string;
  space: string;
  source: string;
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

export type { NoteIdentity };
