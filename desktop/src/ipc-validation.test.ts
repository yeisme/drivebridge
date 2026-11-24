import { expect, it } from 'vitest';
import { validIPCRequest } from './ipc-validation';

it('validates fixed reader requests before any owner call', () => {
  const input = { object_id: 'note', revision: 'sha256:' + 'a'.repeat(64) };
  expect(validIPCRequest('readDocument', { source_id: 'source', input })).toBe(true);
  for (const request of [{ source_id: 'source', input: { ...input, path: '/private' } }, { source_id: 'source', input: { ...input, revision: 'latest' } }, { source_id: 'source', input, bearer: 'private' }]) {
    expect(validIPCRequest('readDocument', request)).toBe(false);
  }
  expect(validIPCRequest('searchNotes', { source_id: 'source', input: { q: 'needle', limit: 50 } })).toBe(true);
  expect(validIPCRequest('searchNotes', { source_id: 'source', input: { q: {}, limit: 50 } })).toBe(false);
  expect(validIPCRequest('shell', { input: {} })).toBe(false);
});
