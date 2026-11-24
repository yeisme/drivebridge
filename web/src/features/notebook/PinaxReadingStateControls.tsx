import { useEffect, useRef, useState } from 'react';
import { Button } from '../../components/ui/button';
import type { DesktopBridge } from '../../generated/desktop-ipc';
import type { components } from '../../generated/desktop';

type Ref = components['schemas']['PinaxRef'];
type Position = NonNullable<components['schemas']['ReadingState']['position']>;
type State = components['schemas']['ReadingState'];
type Bridge = Pick<DesktopBridge, 'readReadingState' | 'updateReadingState' | 'lookupReadingMutation'>;
const revoked = new Set(['protection_released', 'source_unknown', 'desktop_session_invalid', 'reading_target_unavailable', 'insufficient_scope']);

export function PinaxReadingStateControls({ bridge, target, onRevoked, capturePosition, restorePosition }: { bridge: Bridge; target: Ref; onRevoked: () => void; capturePosition?: () => Position | undefined; restorePosition?: (position: Position) => void }) {
  const [state, setState] = useState<State>();
  const [busy, setBusy] = useState(true);
  const [pending, setPending] = useState('');
  const [message, setMessage] = useState('');
  const [attempt, setAttempt] = useState(0);
  const epoch = useRef(0);
  const revoke = useRef(onRevoked); revoke.current = onRevoked;
  useEffect(() => {
    const generation = ++epoch.current;
    setState(undefined); setPending(''); setBusy(true); setMessage('');
    bridge.readReadingState({ input: { target_ref: target } }).then(result => {
      if (epoch.current !== generation) return;
      if ('error' in result) { setMessage('无法读取本机阅读状态。'); if (revoked.has(result.error.code)) revoke.current(); return; }
      setState(result.data.found ? result.data.state! : undefined); setBusy(false);
    }).catch(() => { if (epoch.current === generation) setMessage('无法读取本机阅读状态。'); });
    return () => { epoch.current++; };
  }, [bridge, target, attempt]);
  async function update(field: 'favorite' | 'read_later' | 'position') {
    if (busy || pending) return;
    const position = field === 'position' ? capturePosition?.() : undefined;
    if (field === 'position' && !position) return;
    const generation = epoch.current, key = crypto.randomUUID();
    setBusy(true); setPending(key); setMessage('');
    try {
      const result = await bridge.updateReadingState({ input: { target_ref: target, expected_revision: state?.revision ?? 0, ...(field === 'position' ? { position: position! } : { [field]: !(field === 'favorite' ? state?.favorite : state?.read_later) }) }, idempotency_key: key });
      if (epoch.current !== generation) return;
      if ('error' in result) {
        if (revoked.has(result.error.code)) revoke.current();
        setMessage(result.error.code === 'revision_conflict' ? '阅读状态已在其他窗口改变，请重新打开笔记。' : '结果尚未确认，请查询原操作。');
        return;
      }
      setState(result.data); setPending(''); setMessage('已保存在本机。');
    } catch { if (epoch.current === generation) setMessage('结果尚未确认，请查询原操作。'); }
    finally { if (epoch.current === generation) setBusy(false); }
  }
  async function reconcile() {
    if (!pending || busy) return;
    const generation = epoch.current; setBusy(true);
    try {
      const result = await bridge.lookupReadingMutation({ input: { idempotency_key: pending } });
      if (epoch.current !== generation) return;
      if ('error' in result) { if (revoked.has(result.error.code)) revoke.current(); setMessage('暂时无法查询原操作。'); return; }
      if (!result.data.found) { setMessage('尚未找到原操作回执，未重新提交。'); return; }
      setState(result.data.state!); setPending(''); setMessage('已恢复原操作的保存结果。');
    } catch { if (epoch.current === generation) setMessage('暂时无法查询原操作。'); }
    finally { if (epoch.current === generation) setBusy(false); }
  }
  return <section aria-label="本机阅读状态" className="flex flex-wrap items-center gap-2 border-b py-2">
    <Button variant="outline" disabled={busy || !!pending} aria-pressed={state?.favorite ?? false} onClick={() => void update('favorite')}>{state?.favorite ? '已收藏' : '收藏'}</Button>
    <Button variant="outline" disabled={busy || !!pending} aria-pressed={state?.read_later ?? false} onClick={() => void update('read_later')}>{state?.read_later ? '已加入稍后读' : '稍后读'}</Button>
    {capturePosition && <Button variant="outline" disabled={busy || !!pending} onClick={() => void update('position')}>记住当前位置</Button>}
    {state?.position && state.position.revision === target.revision && restorePosition && <Button variant="outline" disabled={busy} onClick={() => restorePosition(state.position!)}>继续上次阅读</Button>}
    {pending && <Button variant="outline" disabled={busy} onClick={() => void reconcile()}>查询原保存操作</Button>}
    {message && !pending && busy && <Button variant="outline" onClick={() => setAttempt(value => value + 1)}>重新读取阅读状态</Button>}
    {message && <p role="status" className="m-0 text-xs text-muted-foreground">{message}</p>}
  </section>;
}
