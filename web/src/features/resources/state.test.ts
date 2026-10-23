import { expect, it } from 'vitest';
import { initialResourcePageUiState, resourcePageUiReducer } from './state';

it('switches views without clearing the selected resource', () => {
  const selected = resourcePageUiReducer(initialResourcePageUiState, { type: 'select', descriptor: 'resource-1' });
  const grid = resourcePageUiReducer(selected, { type: 'set-view', mode: 'grid' });
  expect(grid).toMatchObject({ viewMode: 'grid', selectedDescriptor: 'resource-1', inspectorOpen: true });
});

it('clears inspector state when the resource scope is invalidated', () => {
  const selected = resourcePageUiReducer(initialResourcePageUiState, { type: 'select', descriptor: 'resource-1' });
  expect(resourcePageUiReducer(selected, { type: 'close-inspector' })).toMatchObject({ selectedDescriptor: null, inspectorOpen: false, checking: false, snapshotOpen: false });
});
