import { expect, it } from 'vitest';
import { projectCapabilities, projectReadiness } from './client';

it('projects only safe discovery fields and preserves unknown limits', () => {
  const value = projectCapabilities({ actions: ['file_list'], limits: { max_file_bytes: 1024 }, public_base_url: 'do-not-copy', tools: [{ private: true }] });
  expect(value).toEqual({ actions: ['file_list'], limits: { max_file_bytes: 1024, max_batch_items: null, space_quota_bytes: null, space_remaining_bytes: null } });
});
it.each([
  null, {}, { actions: ['file_list', 'file_list'], limits: {} },
  { actions: ['https://invalid'], limits: {} }, { actions: ['file_list'], limits: { max_file_bytes: -1 } },
  { actions: ['file_list'], limits: { max_file_bytes: Number.MAX_SAFE_INTEGER + 1 } },
])('rejects malformed discovery without claiming no capabilities: %j', value => {
  expect(() => projectCapabilities(value)).toThrow('invalid_contract');
});
const readiness = { action: 'file_read', implemented: true, configured: true, authorized: true, dependency_status: 'ok', availability: 'available', connection_observed_at: '2026-10-07T00:00:00Z' };
it('does not treat unknown readiness enums as availability', () => {
  expect(projectReadiness({ ...readiness, availability: 'future', dependency_status: 'future' }, 'file_read')).toMatchObject({ availability: 'unknown', dependency: 'unknown' });
});
it.each([{ action: 'file_list' }, { authorized: false }, { implemented: false }, { dependency_status: 'not_probed' }, { authorized: 'yes' }])('rejects a substituted or contradictory readiness result: %j', patch => {
  expect(() => projectReadiness({ ...readiness, ...patch }, 'file_read')).toThrow('invalid_contract');
});
it('does not project arbitrary provider messages or URLs', () => {
  const value = projectReadiness({ ...readiness, blocked_reason: 'https://private.invalid', provider_payload: 'private', recovery: { token: 'private' } }, 'file_read');
  expect(value.reason).toBeNull();
  expect(JSON.stringify(value)).not.toContain('private');
});
