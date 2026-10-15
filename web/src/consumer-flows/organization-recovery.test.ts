import { afterEach, expect, it } from 'vitest';
import { clearOrganizationIntent, readOrganizationIntent } from './organization-recovery';
const intent = { key: 'organize-' + 'a'.repeat(48), action: 'organize_plan', input: { refs: ['drivebridge://instance/file/f/version/v'], actions: [{ kind: 'add_label', namespace: 'rule', value: 'reference' }], policy_id: 'policy' } } as const;
Object.defineProperty(navigator, 'locks', { configurable: true, value: { request: async (_name: string, options: LockOptions | (() => unknown), callback?: () => unknown) => typeof options === 'function' ? options() : callback!() } });
afterEach(() => localStorage.clear());
it('keeps fixed refs and intent identity recoverable', () => { localStorage.setItem('slot', JSON.stringify(intent)); expect(readOrganizationIntent('slot')).toEqual(intent); });
it('clears only the matching original key', async () => { localStorage.setItem('slot', JSON.stringify(intent)); await clearOrganizationIntent('slot', intent.key); expect(localStorage.getItem('slot')).toBeNull(); });
