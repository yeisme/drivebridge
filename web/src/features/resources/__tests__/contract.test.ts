import { describe, expect, it } from 'vitest';
import { isResourceDescriptor, resourceKinds } from '../contract';

describe('resource page contract', () => {
  it('keeps the three supported descriptor kinds explicit', () => {
    expect(Object.keys(resourceKinds).sort()).toEqual(['reference_pack', 'skill', 'template']);
  });

  it('rejects a descriptor without a fixed target', () => {
    expect(isResourceDescriptor({ descriptor_id: 'd', space_id: 's', display_name: 'x', kind: 'skill', descriptor_revision: 1, check_state: 'unknown', created_at: new Date().toISOString(), file_ref: undefined, collection_snapshot_id: undefined } as never)).toBe(false);
  });
});
