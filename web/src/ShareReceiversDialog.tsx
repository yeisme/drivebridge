import { useEffect, useRef, useState } from 'react';
import { api } from './api';
import { AppDialog } from './components/AppDialog';
import { ConfirmDialog } from './components/ConfirmDialog';
import { Button } from './components/ui/button';
import { receiverError, type ShareReceivers, type ShareReceiverDevice } from './share-receiver';

export function ShareReceiversDialog({ id, close }: { id: string; close: () => void }) {
  const [data, setData] = useState<ShareReceivers>();
  const [error, setError] = useState('');
  const [busy, setBusy] = useState(false);
  const [revoking, setRevoking] = useState<ShareReceiverDevice>();
  const lifetime = useRef(new AbortController());
  const reading = useRef<AbortController | undefined>(undefined);
  async function reload() {
    if (lifetime.current.signal.aborted) return;
    reading.current?.abort();
    const controller = new AbortController(); reading.current = controller;
    setData(undefined); setError('');
    try {
      const result = await api<ShareReceivers>(`/shares/${encodeURIComponent(id)}/receivers`, 'GET', undefined, {}, controller.signal);
      if (!controller.signal.aborted && !lifetime.current.signal.aborted) {
        if (!Array.isArray(result?.devices)) throw new Error('invalid_contract');
        setData(result);
      }
    } catch (error) {
      if (!controller.signal.aborted && !lifetime.current.signal.aborted) {
        setError(receiverError(error));
        if (error instanceof Error && 'status' in error && (error.status === 401 || error.status === 403)) { lifetime.current.abort(); setRevoking(undefined); }
      }
    }
  }
  useEffect(() => {
    lifetime.current = new AbortController();
    void reload();
    const invalidate = () => { lifetime.current.abort(); reading.current?.abort(); setData(undefined); setRevoking(undefined); setError('身份或范围已变化，接收设备数据已清除。'); };
    window.addEventListener('drivebridge-context', invalidate);
    return () => { lifetime.current.abort(); reading.current?.abort(); window.removeEventListener('drivebridge-context', invalidate); };
  }, [id]);
  async function revoke(device: ShareReceiverDevice) {
    const signal = lifetime.current.signal;
    if (signal.aborted || busy) return;
    setBusy(true);
    try {
      await api(`/shares/${encodeURIComponent(id)}/receivers/${encodeURIComponent(device.id)}`, 'DELETE', undefined, {}, signal);
      if (signal.aborted) return;
      setRevoking(undefined); await reload();
    } catch (error) {
      if (!signal.aborted) {
        setData(undefined); setRevoking(undefined); setError(receiverError(error));
        if (error instanceof Error && 'status' in error && (error.status === 401 || error.status === 403)) lifetime.current.abort();
      }
    } finally { if (!signal.aborted) setBusy(false); }
  }
  return <AppDialog title="接收设备与额度" close={close}>
    <p>设备只是假名身份，不代表实名确认。撤销后该设备的后续访问与已有设备会话失效，已下载副本不会删除。</p>
    {error ? <p role="alert">{error}</p> : !data && <p role="status">正在读取接收设备…</p>}
    <Button type="button" variant="outline" disabled={busy || lifetime.current.signal.aborted} onClick={() => void reload()}>刷新接收设备</Button>
    {data && <>
      <p>打开次数：{data.opens_used} / {data.opens_max ?? '不限'}；下载次数：{data.downloads_used} / {data.downloads_max ?? '不限'}</p>
      {!data.devices.length && <p>尚未登记接收设备。</p>}
      <ul className="space-y-3">{data.devices.map(device => <li key={device.id} className="space-y-1 break-all">
        <strong>{device.label || '未命名设备'}</strong>
        <p>指纹：{device.fingerprint}</p>
        <p>登记：{new Date(device.created_at).toLocaleString('zh-CN')}；最近访问：{new Date(device.last_seen).getFullYear() > 1 ? new Date(device.last_seen).toLocaleString('zh-CN') : '尚未访问'}；打开 {device.opens} 次</p>
        {device.revoked ? <p>已撤销</p> : <Button type="button" variant="destructive" disabled={busy} onClick={() => setRevoking(device)}>撤销设备 {device.label || device.id}</Button>}
      </li>)}</ul>
    </>}
    {revoking && <ConfirmDialog open onOpenChange={open => { if (!open) setRevoking(undefined); }} title="撤销接收设备" description="停止此设备的后续免码访问。此操作不会撤销其他设备，也不会收回已下载副本。" destructive confirmLabel="确认撤销设备" onConfirm={() => revoke(revoking)}/>}
  </AppDialog>;
}
