import { describe, expect, it } from 'vitest';
import { initialNotebookPageUiState, notebookPageUiReducer } from '../state';

describe('notebook page UI state', () => {
  it('keeps directory and reading mode local', () => {
    const next = notebookPageUiReducer(initialNotebookPageUiState, { type: 'set-reading-mode', mode: 'source' });
    expect(next.readingMode).toBe('source');
    expect(next).not.toHaveProperty('body');
  });
});
