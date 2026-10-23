import { expect, it } from 'vitest';
import type { ResourceDescriptor } from './contract';
import { projectResourceList } from './projection';

const resource = (id: string, name: string, created_at: string): ResourceDescriptor => ({
  descriptor_id: id,
  space_id: 'space',
  display_name: name,
  kind: 'template',
  descriptor_revision: 1,
  check_state: 'unknown',
  created_at,
  file_ref: `drivebridge://instance/file/${id}/version/v1`,
  collection_snapshot_id: undefined,
});

it('filters the loaded page and sorts by display name', () => {
  const result = projectResourceList([resource('2', 'Zulu', '2026-01-01T00:00:00Z'), resource('1', 'Alpha', '2026-01-02T00:00:00Z')], '');
  expect(result.filteredItems.map(item => item.display_name)).toEqual(['Alpha', 'Zulu']);
});

it('sorts only the loaded page by most recent registration', () => {
  const result = projectResourceList([resource('2', 'Old', '2026-01-01T00:00:00Z'), resource('1', 'New', '2026-01-02T00:00:00Z')], '', 'created');
  expect(result.filteredItems.map(item => item.display_name)).toEqual(['New', 'Old']);
});
