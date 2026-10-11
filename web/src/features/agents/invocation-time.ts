// The UI accepts an explicit millisecond-precision RFC3339 subset. It never
// guesses a local zone or truncates a finer-precision boundary.
export function invocationTime(value: string): string {
  if (value === '') return '';
  const match = /^(\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2})(\.\d{1,3})?(Z|[+-]\d{2}:\d{2})$/.exec(value);
  if (!match || match[3] === '-00:00') throw new Error('invalid_time');
  const civil = match[1] + (match[2] || '') + 'Z';
  const time = Date.parse(civil);
  if (!Number.isFinite(time) || new Date(time).toISOString().slice(0, 19) !== match[1]) throw new Error('invalid_time');
  const zone = match[3];
  if (zone !== 'Z' && (Number(zone.slice(1, 3)) > 23 || Number(zone.slice(4, 6)) > 59)) throw new Error('invalid_time');
  const instant = Date.parse(value);
  if (!Number.isFinite(instant)) throw new Error('invalid_time');
  const result = new Date(instant).toISOString();
  if (!/^\d{4}-/.test(result)) throw new Error('invalid_time');
  return result;
}
export function invocationBounds(after: string, before: string) {
  const start = invocationTime(after), end = invocationTime(before);
  if (start && end && start >= end) throw new Error('invalid_range');
  return { after: start, before: end };
}
