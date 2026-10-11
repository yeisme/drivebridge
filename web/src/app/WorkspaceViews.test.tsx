import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { WorkspaceViews } from './WorkspaceViews';
import { useRouteLeaveGuard } from './route-leave';

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it('shows one product surface at a time and leaves legacy functionality reachable', () => {
  const Legacy = vi.fn(() => <p>旧操作</p>);
  render(<WorkspaceViews label="收件视图" primaryLabel="提交收件" legacyLabel="历史传输收件" primary={<p>新收件</p>} legacy={<Legacy />} />);
  expect(screen.getByText('新收件')).toBeTruthy();
  expect(Legacy).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('tab', { name: '历史传输收件' }));
  expect(screen.getByText('旧操作')).toBeTruthy();
  expect(screen.queryByText('新收件')).toBeNull();
  expect(screen.getByRole('tab', { name: '历史传输收件' }).getAttribute('aria-selected')).toBe('true');
});

it('keeps shared actions outside the tablist and available in both views', () => {
  const action = vi.fn();
  render(<WorkspaceViews label="授权视图" primaryLabel="新授权" legacyLabel="旧授权" primary={<p>新版</p>} legacy={<p>兼容</p>} actions={<button onClick={action}>连接指南</button>} />);
  const button = screen.getByRole('button', { name: '连接指南' });
  expect(button.closest('[role=tablist]')).toBeNull();
  fireEvent.click(button);
  fireEvent.click(screen.getByRole('tab', { name: '旧授权' }));
  fireEvent.click(button);
  expect(action).toHaveBeenCalledTimes(2);
});

it('can preserve an explicit legacy deep-link entry without changing the default', () => {
  render(<WorkspaceViews label="工作台视图" primaryLabel="工作台" legacyLabel="观察视图" initialView="legacy" primary={<p>新版</p>} legacy={<p>兼容深链</p>} />);
  expect(screen.getByText('兼容深链')).toBeTruthy();
  expect(screen.queryByText('新版')).toBeNull();
});

it('does not discard an active draft when the view change is declined', () => {
  function Draft() { useRouteLeaveGuard(true, '保留草稿？'); return <p>草稿</p>; }
  render(<WorkspaceViews label="工作台视图" primaryLabel="工作台" legacyLabel="观察视图" primary={<Draft />} legacy={<p>观察</p>} />);
  const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
  fireEvent.click(screen.getByRole('tab', { name: '观察视图' }));
  expect(confirm).toHaveBeenCalledOnce();
  expect(screen.getByText('草稿')).toBeTruthy();
  expect(screen.queryByText('观察')).toBeNull();
});

it('a new scope starts with the primary view instead of retaining the previous actor selection', () => {
  const renderViews = (key: string) => <WorkspaceViews key={key} label="授权视图" primaryLabel="Agent" legacyLabel="旧版授权" primary={<p>默认授权</p>} legacy={<p>兼容授权</p>} />;
  const view = render(renderViews('alice'));
  fireEvent.click(screen.getByRole('tab', { name: '旧版授权' }));
  view.rerender(renderViews('bob'));
  expect(screen.getByText('默认授权')).toBeTruthy();
  expect(screen.queryByText('兼容授权')).toBeNull();
});
