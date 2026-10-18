import { readFileSync } from 'node:fs';
import { resolve } from 'node:path';
import { describe, expect, it } from 'vitest';

describe('feature boundary', () => {
  it('does not couple notebook and resources page entries directly', () => {
    const notebook = readFileSync(resolve(process.cwd(), 'src/features/notebook/KnowledgePage.tsx'), 'utf8');
    const resources = readFileSync(resolve(process.cwd(), 'src/features/resources/ResourcesPage.tsx'), 'utf8');
    expect(notebook).not.toMatch(/features\/resources/);
    expect(resources).not.toMatch(/features\/notebook/);
  });
});
