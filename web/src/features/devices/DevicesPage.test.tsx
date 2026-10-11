import { cleanup, fireEvent, render, screen } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { DevicesPage } from './DevicesPage';

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

it('distinguishes browser connectivity from device and service readiness without probing', () => {
  const online = vi.spyOn(navigator, 'onLine', 'get').mockReturnValue(true);
  const fetch = vi.spyOn(globalThis, 'fetch');
  const navigate = vi.fn();
  render(<DevicesPage onNavigate={navigate}/>);
  expect(screen.getByText('浏览器报告网络可用')).toBeTruthy();
  expect(screen.getByText(/不证明服务、存储或其他设备在线/)).toBeTruthy();
  expect(screen.getByText(/普通下载是用户保存的独立文件/)).toBeTruthy();
  online.mockReturnValue(false); fireEvent(window, new Event('offline'));
  expect(screen.getByText('浏览器报告离线')).toBeTruthy();
  expect(fetch).not.toHaveBeenCalled();
  fireEvent.click(screen.getByRole('button', { name: '查看服务能力' }));
  fireEvent.click(screen.getByRole('button', { name: '前往网盘查看与下载' }));
  fireEvent.click(screen.getByRole('button', { name: '查看服务端整理规则' }));
  expect(navigate.mock.calls).toEqual([['capabilities'], ['files'], ['automation']]);
  expect(fetch).not.toHaveBeenCalled();
});
