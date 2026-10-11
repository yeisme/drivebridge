import { act, cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { afterEach, expect, it, vi } from 'vitest';
import { useEffect } from 'react';
import { FilePreviewDialog } from './FilePreviewDialog';
import { AppDialog } from '../../components/AppDialog';
import { useState } from 'react';

afterEach(() => { cleanup(); vi.restoreAllMocks(); });
function viewport() {
  let desktop = true;
  const listeners = new Set<() => void>();
  const original = window.matchMedia.bind(window);
  vi.spyOn(window, 'matchMedia').mockImplementation(query => query === '(min-width: 1280px)' ? {
    ...original(query), media: query, get matches() { return desktop; },
    addEventListener: (_type: string, notify: EventListenerOrEventListenerObject) => { listeners.add(notify as () => void); },
    removeEventListener: (_type: string, notify: EventListenerOrEventListenerObject) => { listeners.delete(notify as () => void); },
  } as MediaQueryList : original(query));
  return (next: boolean) => act(() => { desktop = next; listeners.forEach(notify => notify()); });
}

it('keeps reader and information state mounted while changing layout and panel visibility', async () => {
  const resize = viewport(), mounted = vi.fn(), unmounted = vi.fn(), close = vi.fn();
  function Reader() {
    useEffect(() => { mounted(); return unmounted; }, []);
    return <textarea aria-label="reader draft" defaultValue="original"/>;
  }
  render(<FilePreviewDialog title="Fixture" close={close} information={<input aria-label="info draft" defaultValue="info"/>}><Reader/></FilePreviewDialog>);
  const reader = screen.getByRole('textbox', { name: 'reader draft' });
  const info = screen.getByRole('textbox', { name: 'info draft' });
  fireEvent.change(reader, { target: { value: 'unsaved' } });
  fireEvent.change(info, { target: { value: 'unchanged info' } });
  fireEvent.click(screen.getByRole('button', { name: '收起信息栏' }));
  resize(false);
  fireEvent.click(screen.getByRole('button', { name: '展开信息栏' }));
  await waitFor(() => expect(document.activeElement).toBe(screen.getByRole('button', { name: '关闭信息栏' })));
  expect(reader.closest('section')?.hasAttribute('inert')).toBe(true);
  fireEvent.click(screen.getByRole('button', { name: '关闭信息栏' }));
  expect(document.activeElement).toBe(screen.getByRole('button', { name: '展开信息栏' }));
  resize(true);
  expect(screen.queryByRole('complementary', { name: '预览文件列表与信息' })).toBeNull();
  fireEvent.click(screen.getByRole('button', { name: '展开信息栏' }));
  expect(screen.getByRole('textbox', { name: 'info draft' })).toBe(info);
  expect((info as HTMLInputElement).value).toBe('unchanged info');
  expect(screen.getByRole('textbox', { name: 'reader draft' })).toBe(reader);
  expect((reader as HTMLTextAreaElement).value).toBe('unsaved');
  expect(mounted).toHaveBeenCalledTimes(1);
  expect(unmounted).not.toHaveBeenCalled();
  expect(close).not.toHaveBeenCalled();
});

it('returns focus to information control when resizing hides its focused field', () => {
  const resize = viewport();
  render(<FilePreviewDialog title="Fixture" close={vi.fn()} information={<input aria-label="info field"/>}>Reader</FilePreviewDialog>);
  screen.getByRole('textbox', { name: 'info field' }).focus();
  resize(false);
  expect(document.activeElement).toBe(screen.getByRole('button', { name: '展开信息栏' }));
});

it('closes the nested dialog, then overlay, then preview on successive Escape presses', async () => {
  const resize = viewport(); resize(false);
  const close = vi.fn();
  function Information() {
    const [open, setOpen] = useState(false);
    return <><button onClick={() => setOpen(true)}>Nested action</button>{open && <AppDialog title="Nested" close={() => setOpen(false)}>Nested content</AppDialog>}</>;
  }
  render(<FilePreviewDialog title="Fixture" close={close} information={<Information/>}>Reader</FilePreviewDialog>);
  fireEvent.click(screen.getByRole('button', { name: '展开信息栏' }));
  fireEvent.click(screen.getByRole('button', { name: 'Nested action' }));
  const nested = await screen.findByRole('dialog', { name: 'Nested' });
  fireEvent.keyDown(nested, { key: 'Escape' });
  await waitFor(() => expect(screen.queryByRole('dialog', { name: 'Nested' })).toBeNull());
  expect(screen.getByRole('complementary', { name: '预览文件列表与信息' })).toBeTruthy();
  fireEvent.keyDown(screen.getByRole('button', { name: '关闭信息栏' }), { key: 'Escape' });
  await waitFor(() => expect(screen.queryByRole('complementary', { name: '预览文件列表与信息' })).toBeNull());
  expect(close).not.toHaveBeenCalled();
  fireEvent.keyDown(screen.getByRole('button', { name: '展开信息栏' }), { key: 'Escape' });
  expect(close).toHaveBeenCalledTimes(1);
});
