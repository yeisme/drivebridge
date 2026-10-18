import { describe, expect, it } from 'vitest';
import { projectResourceList } from '../projection';

describe('resource page projection', () => {
  it('filters only the loaded page', () => {
    const items = [{ display_name: 'Skill A' }, { display_name: 'Template B' }] as never[];
    expect(projectResourceList(items, 'skill').filteredItems).toHaveLength(1);
  });
});
