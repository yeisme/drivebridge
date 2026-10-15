import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { AppearanceProvider } from './appearance';
import { PersonalSettings } from './PersonalSettings';
import type { UserIdentity } from './api';

const user: UserIdentity = { id: 'user-alice', name: 'alice', display_name: 'Alice' };
const json = (data: unknown, status = 200) => new Response(JSON.stringify({ data }), { status });

afterEach(() => { cleanup(); vi.restoreAllMocks(); vi.unstubAllGlobals(); localStorage.clear(); });

it('keeps appearance local, saves profile through the owner API, and exposes account credentials', async () => {
  const writeText = vi.fn().mockResolvedValue(undefined);
  vi.stubGlobal('navigator', Object.assign(Object.create(navigator), { clipboard: { writeText } }));
  const updated = { ...user, display_name: 'Alice Updated' };
  const fetch = vi.spyOn(globalThis, 'fetch').mockImplementation(async input => {
    if (String(input).endsWith('/me/avatar-options?q=')) return json({ items: [] });
    return json({ user: updated });
  });
  const onUserChange = vi.fn();
  render(<AppearanceProvider><PersonalSettings user={user} teamEnabled={false} onUserChange={onUserChange} openKeys={vi.fn()} /></AppearanceProvider>);
  fireEvent.click(screen.getByRole('radio', { name: /深色/ }));
  expect(localStorage.getItem('drivebridge:theme')).toBe('dark');
  fireEvent.change(screen.getByLabelText('显示名称'), { target: { value: 'Alice Updated' } });
  fireEvent.click(screen.getByRole('button', { name: '保存资料' }));
  await screen.findByText('个人资料已保存。');
  expect(onUserChange).toHaveBeenCalledWith(updated);
  expect(fetch.mock.calls.some(([url, options]) => String(url).endsWith('/me') && options?.method === 'PATCH')).toBe(true);
  fireEvent.click(screen.getByRole('button', { name: '复制用户 ID' }));
  await waitFor(() => expect(writeText).toHaveBeenCalledWith(user.id));
  expect(screen.getByRole('button', { name: '访问凭据' })).toBeTruthy();
});
