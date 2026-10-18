import { describe, expect, it } from 'vitest';
import { notebookLifecycleFromError, projectKnowledgeFilePage } from '../projection';

describe('notebook page projection', () => {
  it('keeps an empty page with a cursor partial', () => {
    const view = projectKnowledgeFilePage({ entries: [], nextCursor: 'next', scanned: 50, unavailable: 2, stale: 1 });
    expect(view.lifecycle).toBe('partial');
    expect(view.scanned).toBe(50);
  });

  it('maps permissions and unsupported contracts explicitly', () => {
    expect(notebookLifecycleFromError({ status: 403 })).toBe('forbidden');
    expect(notebookLifecycleFromError({ status: 405 })).toBe('unsupported');
  });
});
