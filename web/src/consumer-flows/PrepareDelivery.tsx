import { useEffect, useRef, useState } from 'react';
import { api, getDataContext, type Space } from '../api';
import { Button } from '../components/ui/button';
import { SelectAdapter } from '../components/SelectAdapter';
import { validSnapshotIndex } from '../features/collections/SnapshotHistory';
import type { components } from '../generated/personal-workspace';
import { getDeliveryManifest, lookupOperation, prepareDeliveryManifest } from './client';
import type { DeliveryManifestView } from './model';

type Index = components['schemas']['SnapshotIndexEnvelope']['data'];
interface Share { id: string; name: string; revision: number; revoked: boolean }
interface Collection { collection_id: string; name: string; space_id: string }
interface Intent { key: string; share: string; snapshot: string; revision: number }
function readIntent(slot: string): Intent | null {
  const raw = localStorage.getItem(slot); if (raw === null) return null;
  const value: unknown = JSON.parse(raw);
  if (!value || typeof value !== 'object' || !('key' in value) || typeof value.key !== 'string' || !/^delivery-[a-f0-9]{48}$/.test(value.key) || !('share' in value) || typeof value.share !== 'string' || !value.share || !('snapshot' in value) || typeof value.snapshot !== 'string' || !value.snapshot || !('revision' in value) || typeof value.revision !== 'number' || !Number.isSafeInteger(value.revision) || value.revision < 1) throw new Error('invalid_recovery');
  return { key: value.key, share: value.share, snapshot: value.snapshot, revision: value.revision };
}
export function PrepareDelivery({ actor, spaces, onManifest }: { actor: string; spaces: Space[]; onManifest: (value: DeliveryManifestView) => void }) {
  const context = getDataContext();
  const slot = 'drivebridge:delivery-prepare:' + JSON.stringify([location.origin, actor, context.tenant, context.space]);
  const [shares, setShares] = useState<Share[]>([]), [collections, setCollections] = useState<Collection[]>([]), [index, setIndex] = useState<Index | null>(null);
  const [share, setShare] = useState(''), [collection, setCollection] = useState(''), [snapshot, setSnapshot] = useState(''), [intent, setIntent] = useState<Intent | null>(null);
  const [state, setState] = useState('idle'), [message, setMessage] = useState('');
  const controller = useRef<AbortController | null>(null), active = useRef(true), running = useRef(false);
  const changed = useRef(onManifest); changed.current = onManifest;
  const deny = () => { active.current = false; controller.current?.abort(); setShares([]); setCollections([]); setIndex(null); setShare(''); setCollection(''); setSnapshot(''); setState('denied'); setMessage(''); };
  useEffect(() => {
    active.current = true;
    const invalidate = () => { active.current = false; controller.current?.abort(); setShares([]); setCollections([]); setIndex(null); setState('denied'); };
    const offline = () => { controller.current?.abort(); setShares([]); setCollections([]); setIndex(null); setState('offline'); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', offline);
    try { setIntent(readIntent(slot)); if (!navigator.onLine) offline(); } catch { setState('blocked'); }
    return () => { active.current = false; controller.current?.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', offline); };
  }, [slot]);
  async function load() {
    if (!active.current || running.current || !navigator.onLine) return;
    running.current = true; const request = new AbortController(); controller.current = request; setState('loading'); setMessage('');
    try {
      const observed = await api<Share[]>('/shares', 'GET', undefined, {}, request.signal);
      if (!Array.isArray(observed) || !observed.every(row => row.id && row.name && Number.isSafeInteger(row.revision) && row.revision > 0 && typeof row.revoked === 'boolean')) throw new Error('share_contract');
      const rows: Collection[] = [];
      for (const space of spaces) { const items = await api<Collection[]>('/file-collections?' + new URLSearchParams({ space_id: space.id }), 'GET', undefined, {}, request.signal); if (!Array.isArray(items) || items.length > 1000 || !items.every(item => item.collection_id && item.name && item.space_id === space.id)) throw new Error('collection_contract'); rows.push(...items); }
      if (!request.signal.aborted) { setShares(observed.filter(row => !row.revoked)); setCollections(rows); setIndex(null); setState('ready'); }
    } catch (error) { if (!request.signal.aborted) { if (error && typeof error === 'object' && 'status' in error && (error.status === 401 || error.status === 403)) deny(); else { setShares([]); setCollections([]); setState('error'); setMessage('分享或集合读取失败，不代表没有可用来源。'); } } } finally { running.current = false; }
  }
  async function loadSnapshots(id: string) {
    if (!active.current || running.current || !navigator.onLine) return;
    running.current = true; setCollection(id); setSnapshot(''); setIndex(null); const request = new AbortController(); controller.current = request;
    try { const value = await api<Index>('/file-collections/' + encodeURIComponent(id) + '/snapshots', 'GET', undefined, {}, request.signal); if (!validSnapshotIndex(value, id)) throw new Error('snapshot_contract'); if (!request.signal.aborted) setIndex(value); } catch (error) { if (!request.signal.aborted) { if (error && typeof error === 'object' && 'status' in error && (error.status === 401 || error.status === 403)) deny(); else setMessage('历史快照读取失败或无权访问，不代表没有快照。'); } } finally { running.current = false; }
  }
  async function run(recover: boolean) {
    if (!active.current || running.current || !navigator.onLine || !navigator.locks?.request) { setMessage('无法安全锁定恢复身份或当前离线，未创建新清单。'); return; }
    running.current = true; setState('loading'); setMessage(''); const request = new AbortController(); controller.current = request;
    try {
      const original = await navigator.locks.request(slot, { signal: request.signal }, () => {
        const saved = readIntent(slot); if (saved) return saved;
        if (recover) throw new Error('missing_recovery');
        const selected = shares.find(row => row.id === share); if (!selected || !snapshot || !index?.items.some(row => row.snapshot_id === snapshot)) throw new Error('missing_source');
        const value = { key: 'delivery-' + Array.from(crypto.getRandomValues(new Uint8Array(24)), byte => byte.toString(16).padStart(2, '0')).join(''), share, snapshot, revision: selected.revision }; localStorage.setItem(slot, JSON.stringify(value)); return value;
      });
      setIntent(original);
      if (recover) await lookupOperation('delivery_prepare', original.key, { share_id: original.share, snapshot_id: original.snapshot, expected_share_revision: original.revision }, request.signal);
      const prepared = await prepareDeliveryManifest(original.share, { snapshot_id: original.snapshot, expected_share_revision: original.revision }, original.key, request.signal);
      const verified = await getDeliveryManifest(original.share, prepared.manifest_revision, request.signal);
      if (request.signal.aborted || verified.share_id !== original.share || verified.manifest_revision !== prepared.manifest_revision) return;
      await navigator.locks.request(slot, () => { if (readIntent(slot)?.key === original.key) localStorage.removeItem(slot); });
      if (request.signal.aborted) return;
      setIntent(null); setState('ready'); changed.current(verified); setMessage('原固定清单已回读；完整性与保留保护决定是否可发布。');
    } catch (error) { if (!request.signal.aborted) { if (error && typeof error === 'object' && 'status' in error && (error.status === 401 || error.status === 403)) deny(); else { setState('unknown'); setMessage('清单结果待回查，请使用原键恢复；不会使用新快照或新分享修订替代。'); } } } finally { running.current = false; }
  }
  return <div className="space-y-3"><p>先选择真实分享和集合历史快照，当前分享修订由服务端读取，不用占位快照。</p><Button className="min-h-11" disabled={state === 'loading' || state === 'denied' || state === 'blocked'} onClick={() => void load()}>读取分享与集合来源</Button>{state === 'loading' && <p role="status">正在读取或准备固定交付…</p>}{state === 'offline' && <p role="alert">当前离线，联网后请手动读取；不会自动准备清单。</p>}{state === 'denied' && <p role="alert">权限或范围已变化，交付来源已清除。</p>}{state === 'blocked' && <p role="alert">恢复身份无效，已停止准备新清单。</p>}{message && <p role="status">{message}</p>}{intent && <Button className="min-h-11" disabled={state === 'loading'} onClick={() => void run(true)}>按原键恢复清单</Button>}{state === 'ready' && !intent && <><label>选择分享<SelectAdapter aria-label="选择分享" value={share} onChange={event => setShare(event.target.value)}><option value="">选择分享</option>{shares.map(row => <option key={row.id} value={row.id}>{row.name} · 修订 {row.revision}</option>)}</SelectAdapter></label><label>来源集合<SelectAdapter aria-label="交付来源集合" value={collection} onChange={event => void loadSnapshots(event.target.value)}><option value="">选择集合</option>{collections.map(row => <option key={row.collection_id} value={row.collection_id}>{row.name}</option>)}</SelectAdapter></label>{index && <label>固定快照<SelectAdapter aria-label="交付固定快照" value={snapshot} onChange={event => setSnapshot(event.target.value)}><option value="">选择历史快照</option>{index.items.slice(0, 50).map(row => <option key={row.snapshot_id} value={row.snapshot_id}>{row.snapshot_id} · 修订 {row.collection_revision}</option>)}</SelectAdapter></label>}<Button className="min-h-11" disabled={!share || !snapshot} onClick={() => void run(false)}>准备清单</Button>{!shares.length && <p role="status">当前没有可管理的有效分享。</p>}{!collections.length && <p role="status">当前来源没有集合；请在项目集合建立固定快照后返回。</p>}</>}</div>;
}
