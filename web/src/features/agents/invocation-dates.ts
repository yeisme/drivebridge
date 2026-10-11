export type InvocationDateZone = 'utc' | 'local';

function calendarDate(value: string) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) throw new Error('invalid_date');
  const date = new Date(value + 'T00:00:00.000Z');
  if (!Number.isFinite(date.getTime()) || date.toISOString().slice(0, 10) !== value) throw new Error('invalid_date');
  return date;
}
function midnight(day: string, zone: InvocationDateZone): string {
  if (zone === 'utc') return day + 'T00:00:00.000Z';
  // Native local calendar conversion chooses the earlier repeated midnight.
  // A skipped/adjusted midnight is rejected, never silently normalized.
  const date = new Date(day + 'T00:00:00');
  const [year, month, dateOfMonth] = day.split('-').map(Number);
  if (!Number.isFinite(date.getTime()) || date.getFullYear() !== year || date.getMonth() !== month - 1 || date.getDate() !== dateOfMonth || date.getHours() !== 0 || date.getMinutes() !== 0 || date.getSeconds() !== 0) throw new Error('unavailable_midnight');
  const result = date.toISOString();
  if (!/^\d{4}-/.test(result)) throw new Error('invalid_date');
  return result;
}
export function invocationDateBounds(start: string, end: string, zone: InvocationDateZone) {
  if (zone !== 'utc' && zone !== 'local') throw new Error('invalid_zone');
  calendarDate(start);
  const last = calendarDate(end);
  if (start > end) throw new Error('invalid_range');
  // Advance the civil date, not the elapsed duration of a zoned day.
  last.setUTCDate(last.getUTCDate() + 1);
  const following = last.toISOString();
  if (!/^\d{4}-/.test(following)) throw new Error('invalid_date');
  midnight(end, zone);
  const after = midnight(start, zone), before = midnight(following.slice(0, 10), zone);
  if (after >= before) throw new Error('invalid_range');
  return { after, before };
}
