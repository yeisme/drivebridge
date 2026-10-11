import { cleanup, fireEvent, render, screen, waitFor, within } from '@testing-library/react';
import { afterEach, beforeEach, expect, it, vi } from 'vitest';
import { useState } from 'react';
import { Finder, type FinderAdapter } from '../../Finder';

const viewport = vi.hoisted(() => ({ mobile: false }));
vi.mock('../../app/useMobileViewport', () => ({ useMobileViewport: () => viewport.mobile }));
const scrollIntoView = HTMLElement.prototype.scrollIntoView;
// jsdom has no layout/scroll API; real scrolling is checked by Playwright.
beforeEach(() => { HTMLElement.prototype.scrollIntoView = vi.fn(); });
afterEach(() => {
  cleanup(); vi.restoreAllMocks(); localStorage.clear(); viewport.mobile = false;
  if (scrollIntoView) HTMLElement.prototype.scrollIntoView = scrollIntoView;
  else Reflect.deleteProperty(HTMLElement.prototype, 'scrollIntoView');
});
const space = { id: 'layout-space', name: 'Fixture', connection_id: 'platform', used_bytes: 0, reserved_bytes: 0, max_bytes: 100 };
const adapter: FinderAdapter = {
  directory: async (_space, prefix) => prefix ? [] : [{ kind: 'directory', path: '资料' }],
  mkdir: async () => {}, move: async () => {}, trash: async () => {},
};
const revision: unknown[] = [];
function Fixture() {
  const [prefix, setPrefix] = useState('');
  return <Finder spaces={[space]} space={space.id} setSpace={() => {}} prefix={prefix} setPrefix={setPrefix} revision={revision}
    reload={async () => {}} upload={() => {}} onPreview={() => {}} onShare={() => {}} backendLabel="Fixture" adapter={adapter} showSpaceSwitcher={false}/>;
}

it('keeps layout and view usable when optional localStorage is unavailable', async () => {
  vi.spyOn(Storage.prototype, 'getItem').mockImplementation(() => { throw new DOMException('Blocked', 'SecurityError'); });
  vi.spyOn(Storage.prototype, 'setItem').mockImplementation(() => { throw new DOMException('Blocked', 'SecurityError'); });
  render(<Fixture/>);
  await screen.findByRole('button', { name: '打开文件夹 资料' });
  fireEvent.click(screen.getByRole('button', { name: '折叠目录树' }));
  expect(screen.queryByRole('complementary', { name: '目录树' })).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: '网格视图' }));
  expect(screen.getByRole('button', { name: '网格视图' }).getAttribute('aria-pressed')).toBe('true');
  fireEvent.click(screen.getByRole('button', { name: '展开目录树' }));
  expect(screen.getByRole('complementary', { name: '目录树' })).toBeTruthy();
});

it('uses a mobile drawer without changing the desktop directory preference', async () => {
  const view = render(<Fixture/>);
  await screen.findByRole('button', { name: '打开文件夹 资料' });
  expect(localStorage.getItem('drivebridge:tree-collapsed')).toBe('false');
  viewport.mobile = true;
  view.rerender(<Fixture/>);
  expect(screen.queryByRole('complementary', { name: '目录树' })).toBeNull();
  const trigger = screen.getByRole('button', { name: '展开目录树' });
  fireEvent.keyDown(trigger, { key: 'Enter' });
  expect(within(screen.getByRole('navigation', { name: '目录路径' })).queryByRole('button', { name: '资料' })).toBeNull();
  fireEvent.click(trigger);
  const drawer = await screen.findByRole('dialog', { name: '目录树' });
  fireEvent.click(await within(drawer).findByRole('button', { name: '资料' }));
  await waitFor(() => expect(screen.queryByRole('dialog', { name: '目录树' })).toBeNull());
  expect(within(screen.getByRole('navigation', { name: '目录路径' })).getByRole('button', { name: '资料' })).toBeTruthy();
  expect(localStorage.getItem('drivebridge:tree-collapsed')).toBe('false');
  viewport.mobile = false;
  view.rerender(<Fixture/>);
  expect(screen.getByRole('complementary', { name: '目录树' })).toBeTruthy();
  fireEvent.click(screen.getByRole('button', { name: '折叠目录树' }));
  viewport.mobile = true; view.rerender(<Fixture/>);
  fireEvent.click(screen.getByRole('button', { name: '展开目录树' }));
  fireEvent.click(await screen.findByRole('button', { name: '关闭目录树' }));
  viewport.mobile = false; view.rerender(<Fixture/>);
  expect(screen.queryByRole('complementary', { name: '目录树' })).toBeNull();
  expect(localStorage.getItem('drivebridge:tree-collapsed')).toBe('true');
});
