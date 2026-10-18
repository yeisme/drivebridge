import type { KnowledgeFilePage } from './knowledge-files';
import type { NotebookLifecycle, NotebookListProjection } from './contract';

export function projectKnowledgeFilePage(page: KnowledgeFilePage, lifecycle: NotebookLifecycle = page.entries.length ? 'ready' : 'empty'): NotebookListProjection {
  return { ...page, lifecycle: page.nextCursor && !page.entries.length ? 'partial' : lifecycle };
}

export function notebookLifecycleFromError(error: { status?: number; message?: string }): NotebookLifecycle {
  if (error.status === 401 || error.status === 403) return 'forbidden';
  if (error.status === 409 || error.status === 410) return 'stale';
  if (error.status === 404 || error.status === 405 || error.message === 'unsupported_action') return 'unsupported';
  return 'error';
}
