import { expect, it } from 'vitest';
import { invocationBounds, invocationTime } from './invocation-time';
it.each([
  ['', ''],
  ['2026-10-06T09:30:00+08:00', '2026-10-06T01:30:00.000Z'],
  ['2024-02-29T23:00:00-02:00', '2024-03-01T01:00:00.000Z'],
  ['2026-10-06T09:30:00.12Z', '2026-10-06T09:30:00.120Z'],
])('normalizes explicit instants without the browser zone: %s', (input, output) => expect(invocationTime(input)).toBe(output));
it.each(['2026-02-29T00:00:00Z', '2026-04-31T00:00:00Z', '2026-10-06', '2026-10-06T09:30', '2026-10-06T09:30:00', '2026-10-06T24:00:00Z', '2026-10-06T09:30:60Z', '2026-10-06T09:30:00+24:00', '2026-10-06T09:30:00+00:60', '2026-10-06T09:30:00-00:00', '2026-10-06T09:30:00.0001Z', ' 2026-10-06T09:30:00Z', '2026-10-06T09:30:00Z ', '9999-12-31T23:00:00-02:00'])('rejects invalid, ambiguous or unsupported input: %s', value => expect(() => invocationTime(value)).toThrow());
it('compares normalized instants, rejecting equal and reversed bounds', () => {
  expect(() => invocationBounds('2026-10-06T09:00:00+08:00', '2026-10-06T01:00:00Z')).toThrow();
  expect(() => invocationBounds('2026-10-06T09:00:00Z', '2026-10-06T10:00:00+08:00')).toThrow();
  expect(invocationBounds('', '2026-10-06T09:00:00Z')).toEqual({ after: '', before: '2026-10-06T09:00:00.000Z' });
  expect(invocationBounds('2026-10-06T09:00:00+08:00', '2026-10-06T02:00:00Z')).toEqual({ after: '2026-10-06T01:00:00.000Z', before: '2026-10-06T02:00:00.000Z' });
});
