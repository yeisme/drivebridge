import { describe, expect, it } from 'vitest';
import { initialResourcePageUiState, resourcePageUiReducer } from '../state';

describe('resource page UI state', () => {
  it('opens the inspector for a fixed descriptor', () => {
    const next = resourcePageUiReducer(initialResourcePageUiState, { type: 'select', descriptor: 'descriptor-a' });
    expect(next.inspectorOpen).toBe(true);
    expect(next.selectedDescriptor).toBe('descriptor-a');
  });
});
