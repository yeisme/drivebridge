import { expect, it } from 'vitest';
import { invocationDateBounds } from './invocation-dates';
it('includes both selected UTC dates but excludes the following midnight', () => {
  expect(invocationDateBounds('2024-02-28', '2024-02-29', 'utc')).toEqual({ after: '2024-02-28T00:00:00.000Z', before: '2024-03-01T00:00:00.000Z' });
  expect(invocationDateBounds('2026-12-31', '2026-12-31', 'utc')).toEqual({ after: '2026-12-31T00:00:00.000Z', before: '2027-01-01T00:00:00.000Z' });
});
it.each([['', '2026-01-01'], ['2026-02-29', '2026-03-01'], ['2026-01-02', '2026-01-01'], ['2026-01-01', ''], ['9999-12-31', '9999-12-31'], ['26-01-01', '2026-01-01']])('rejects invalid or reversed dates: %s / %s', (start, end) => expect(() => invocationDateBounds(start, end, 'utc')).toThrow());
it('uses native local calendar midnights rather than a fixed duration', () => {
  const expectedStart = new Date('2026-03-08T00:00:00').toISOString();
  const expectedEnd = new Date('2026-03-09T00:00:00').toISOString();
  expect(invocationDateBounds('2026-03-08', '2026-03-08', 'local')).toEqual({ after: expectedStart, before: expectedEnd });
});
