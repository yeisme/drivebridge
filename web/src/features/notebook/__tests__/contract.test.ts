import { describe, expect, it } from 'vitest';
import type { NotebookScopeContract } from '../contract';

describe('notebook page contract', () => {
  it('requires a fixed scope without carrying canonical body data', () => {
    const value: NotebookScopeContract = { actor: 'alice', tenant: 'tenant-a', space: 'space-a', source: 'space-a' };
    expect(value.source).toBe(value.space);
    expect(value).not.toHaveProperty('body');
  });
});
