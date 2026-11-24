// @vitest-environment jsdom
import { afterEach, expect, it, vi } from 'vitest';
import { cleanup, fireEvent, render, screen, waitFor } from '@testing-library/react';
import { PinaxReadingStateControls } from './PinaxReadingStateControls';
const target = { kind: 'pinax' as const, instance_id: 'cli', source_id: 'source', object_id: 'note', revision: 'sha256:' + 'a'.repeat(64), uri: 'pinax://note/note?v=sha256:' + 'a'.repeat(64) };
const state = { target_ref: target, favorite: true, read_later: false, revision: 1, position: null, updated_at: '2026-11-03T00:00:00Z' };
afterEach(cleanup);
it('lost save response only looks up the original key and never submits another update', async () => {
 const bridge = { readReadingState: vi.fn().mockResolvedValue({ data: { found: false, state: null } }), updateReadingState: vi.fn().mockRejectedValue(new Error('lost')), lookupReadingMutation: vi.fn().mockResolvedValue({ data: { found: true, state } }) };
 render(<PinaxReadingStateControls bridge={bridge} target={target} onRevoked={() => {}}/>);
 await waitFor(() => expect((screen.getByRole('button', { name: '收藏' }) as HTMLButtonElement).disabled).toBe(false));
 fireEvent.click(screen.getByRole('button', { name: '收藏' }));
 await screen.findByText('结果尚未确认，请查询原操作。');
 fireEvent.click(screen.getByRole('button', { name: '查询原保存操作' }));
 await screen.findByRole('button', { name: '已收藏' });
 expect(bridge.updateReadingState).toHaveBeenCalledTimes(1);
 expect(bridge.lookupReadingMutation).toHaveBeenCalledWith({ input: { idempotency_key: bridge.updateReadingState.mock.calls[0][0].idempotency_key } });
});
it('revoked reading state clears the parent reader and does not write', async () => {
 const onRevoked = vi.fn();
 const bridge = { readReadingState: vi.fn().mockResolvedValue({ error: { code: 'protection_released' } }), updateReadingState: vi.fn(), lookupReadingMutation: vi.fn() };
 render(<PinaxReadingStateControls bridge={bridge} target={target} onRevoked={onRevoked}/>);
 await waitFor(() => expect(onRevoked).toHaveBeenCalledTimes(1));
 expect(bridge.updateReadingState).not.toHaveBeenCalled();
});
