import { useEffect, useRef, useState } from 'react';
import { api, getDataContext, pinnedFile } from '../../api';
import { statFile } from '../../info-workspace/client';
import type { components } from '../../generated/personal-workspace';
import { Button } from '../../components/ui/button';
import { noteKey, type NoteIdentity } from './notebook-state';

type Schemas = components['schemas'];
type FavoritePage = Schemas['FavoriteListEnvelope']['data'];
type Favorite = FavoritePage['items'][number];
type Receipt = Schemas['FavoriteReceiptEnvelope']['data'];
type Action = Schemas['FavoriteLookup']['action'];
type Recovery = { key: string; digest: string; action: Action; expected: number; favorite: string };
const hexID = (value: string) => /^[a-f0-9]{48}$/.test(value);
const sameRecovery = (a: Recovery | null, b: Recovery) => !!a && a.key === b.key && a.digest === b.digest && a.action === b.action && a.expected === b.expected && a.favorite === b.favorite;
function readRecovery(slot: string): Recovery | null {
  const raw = localStorage.getItem(slot);
  if (raw === null) return null;
  if (raw.length > 512) throw new Error('invalid_recovery');
  const row = JSON.parse(raw) as Recovery;
  if (!row || Object.keys(row).sort().join(',') !== 'action,digest,expected,favorite,key' || !/^favorite-[a-f0-9]{48}$/.test(row.key) || !/^[a-f0-9]{64}$/.test(row.digest) || !['create', 'delete'].includes(row.action) || !Number.isSafeInteger(row.expected) || row.expected < 0 || row.expected >= Number.MAX_SAFE_INTEGER || typeof row.favorite !== 'string' || (row.favorite !== '' && !hexID(row.favorite)) || (row.action === 'delete' && !row.favorite)) throw new Error('invalid_recovery');
  return row;
}
async function clearRecovery(slot: string, record: Recovery, signal: AbortSignal) {
  if (!navigator.locks?.request) throw new Error('lock_unavailable');
  await navigator.locks.request(slot, { signal }, () => {
    signal.throwIfAborted();
    if (!sameRecovery(readRecovery(slot), record)) throw new Error('recovery_changed');
    localStorage.removeItem(slot);
  });
}
async function requestDigest(space: string, id: string, input: Schemas['FavoriteCreate']) {
  // Go's exported wrapper field names and declaration order are the owner digest contract.
  const raw = JSON.stringify({ SpaceID: space, ID: id, Request: input }).replace(/[<>&\u2028\u2029]/g, ch => '\\u' + ch.charCodeAt(0).toString(16).padStart(4, '0'));
  return Array.from(new Uint8Array(await crypto.subtle.digest('SHA-256', new TextEncoder().encode(raw))), byte => byte.toString(16).padStart(2, '0')).join('');
}

export function NotebookFavorite({ identity, actor }: { identity: NoteIdentity; actor: string }) {
  const context = getDataContext();
  return <FavoriteSession key={JSON.stringify([actor, context.tenant, context.space, context.revision, noteKey(identity)])} identity={identity} actor={actor}/>;
}
function FavoriteSession({ identity, actor }: { identity: NoteIdentity; actor: string }) {
  const context = getDataContext();
  const slot = 'drivebridge:note-favorite:' + JSON.stringify([actor, context.tenant, context.space, identity.sourceId, identity.objectId]);
  const ordinary = hexID(identity.sourceId) && hexID(identity.objectId) && hexID(identity.revision) && (!context.space || context.space === identity.sourceId);
  const [state, setState] = useState('loading'), [attempt, setAttempt] = useState(0), [busy, setBusy] = useState(false);
  const [favorite, setFavorite] = useState<Favorite | null>(null), [target, setTarget] = useState('');
  const [pending, setPending] = useState<Recovery | null>(null), [intent, setIntent] = useState<Action | null>(null), [notice, setNotice] = useState('');
  const controller = useRef<AbortController | null>(null), running = useRef(false);
  const deny = (signal: AbortController) => { signal.abort(); setFavorite(null); setTarget(''); setPending(null); setIntent(null); setNotice(''); setState('denied'); };
  useEffect(() => {
    const abort = new AbortController(); controller.current = abort;
    setFavorite(null); setTarget(''); setPending(null); setState('loading');
    const invalidate = () => deny(abort);
    const storage = (event: StorageEvent) => { if (event.key === null || event.key === slot) invalidate(); };
    window.addEventListener('drivebridge-context', invalidate); window.addEventListener('offline', invalidate); window.addEventListener('storage', storage);
    async function load() {
      if (!ordinary) { setState('unsupported'); return; }
      if (!navigator.onLine) { invalidate(); return; }
      try {
        const stored = readRecovery(slot);
        const file = await statFile(identity.objectId, abort.signal);
        abort.signal.throwIfAborted();
        if (file.id !== identity.objectId || file.space_id !== identity.sourceId || file.trashed || file.preview_url || file.preview_reason) throw new Error('invalid_file_identity');
        const pinned = await pinnedFile(file, identity.revision, {}, abort.signal);
        abort.signal.throwIfAborted();
        const ref = new URL(pinned.ref);
        if (ref.protocol !== 'drivebridge:' || !ref.hostname || ref.username || ref.password || ref.port || ref.search || ref.hash || pinned.ref !== ref.href || ref.pathname !== `/file/${identity.objectId}/version/${identity.revision}` || pinned.file.id !== identity.objectId || pinned.file.space_id !== identity.sourceId || pinned.file.version_id !== identity.revision || pinned.file.trashed) throw new Error('invalid_fixed_reference');
        const logical = pinned.ref.slice(0, -('/version/' + identity.revision).length);
        setTarget(logical);
        if (stored) { setPending(stored); setState('unknown'); return; }
        let cursor = '', found: Favorite | null = null, count = 0;
        const seenCursors = new Set<string>(), seenIDs = new Set<string>();
        for (let pageNumber = 0; pageNumber < 20; pageNumber++) {
          const query = new URLSearchParams({ space_id: identity.sourceId, limit: '50' });
          if (cursor) query.set('cursor', cursor);
          const page = await api<FavoritePage>('/favorites?' + query, 'GET', undefined, {}, abort.signal);
          abort.signal.throwIfAborted();
          if (!page || !Array.isArray(page.items) || page.items.length > 50 || typeof page.next_cursor !== 'string') throw new Error('invalid_favorite_page');
          count += page.items.length;
          if (count > 1000) throw new Error('favorite_budget_exceeded');
          for (const row of page.items) {
            if (!row || !hexID(row.favorite_id) || row.space_id !== identity.sourceId || !['file', 'directory', 'tag', 'collection'].includes(row.target_kind) || typeof row.target_ref !== 'string' || typeof row.deleted !== 'boolean' || !Number.isSafeInteger(row.revision) || row.revision < 1 || row.revision >= Number.MAX_SAFE_INTEGER || seenIDs.has(row.favorite_id)) throw new Error('invalid_favorite');
            seenIDs.add(row.favorite_id);
            if (row.target_kind === 'file' && row.target_ref === logical) { if (found) throw new Error('duplicate_favorite'); found = row; }
          }
          if (!page.next_cursor) { setFavorite(found); setState('ready'); return; }
          if (seenCursors.has(page.next_cursor)) throw new Error('favorite_cursor_loop');
          cursor = page.next_cursor; seenCursors.add(cursor);
        }
        throw new Error('favorite_budget_exceeded');
      } catch (error) {
        if (abort.signal.aborted) return;
        const status = error && typeof error === 'object' && 'status' in error ? error.status : undefined;
        if (status === 401 || status === 403) invalidate();
        else { setFavorite(null); setTarget(''); setState('error'); }
      }
    }
    void load();
    return () => { abort.abort(); window.removeEventListener('drivebridge-context', invalidate); window.removeEventListener('offline', invalidate); window.removeEventListener('storage', storage); };
  }, [slot, ordinary, identity.sourceId, identity.objectId, identity.revision, attempt]);

  async function change(action: Action | 'lookup') {
    const abort = controller.current;
    if (!abort || abort.signal.aborted || running.current || !navigator.onLine || (action !== 'lookup' && (state !== 'ready' || pending)) || (action === 'lookup' && !pending)) return;
    running.current = true; setBusy(true); setNotice('');
    let record = pending, submitted = false;
    try {
      if (action !== 'lookup') {
        if (!target || (action === 'delete' && (!favorite || favorite.deleted))) return;
        setIntent(action);
        const expected = favorite?.revision ?? 0;
        const id = action === 'delete' ? favorite!.favorite_id : '';
        const input: Schemas['FavoriteCreate'] = { space_id: action === 'delete' ? '' : identity.sourceId, target_kind: action === 'delete' ? '' : 'file', target_ref: action === 'delete' ? '' : target, expected_revision: expected };
        const digest = await requestDigest(identity.sourceId, id, input);
        abort.signal.throwIfAborted();
        if (!navigator.locks?.request) throw new Error('lock_unavailable');
        const claim = await navigator.locks.request(slot, { signal: abort.signal }, () => {
          abort.signal.throwIfAborted();
          const existing = readRecovery(slot);
          if (existing) return { record: existing, claimed: false };
          const next: Recovery = { key: 'favorite-' + Array.from(crypto.getRandomValues(new Uint8Array(24)), byte => byte.toString(16).padStart(2, '0')).join(''), digest, action, expected, favorite: favorite?.favorite_id ?? '' };
          localStorage.setItem(slot, JSON.stringify(next)); return { record: next, claimed: true };
        });
        abort.signal.throwIfAborted(); record = claim.record; setPending(record); setState('unknown');
        if (!claim.claimed) { setNotice('已有未确认的收藏请求，只能查询原结果。'); return; }
        if (!sameRecovery(readRecovery(slot), record)) throw new Error('recovery_changed');
        submitted = true;
        const result = action === 'delete'
          ? await api<Receipt>('/favorites/' + encodeURIComponent(id), 'DELETE', { expected_revision: expected } satisfies Schemas['FavoriteDelete'], { 'Idempotency-Key': record.key }, abort.signal)
          : await api<Receipt>('/favorites', 'POST', input, { 'Idempotency-Key': record.key }, abort.signal);
        await accept(result, record, abort.signal);
      } else {
        if (!record || !sameRecovery(readRecovery(slot), record)) throw new Error('recovery_changed');
        const result = await api<Receipt>('/favorites/lookup', 'POST', { space_id: identity.sourceId, action: record.action, idempotency_key: record.key } satisfies Schemas['FavoriteLookup'], {}, abort.signal);
        await accept(result, record, abort.signal);
      }
    } catch (error) {
      if (abort.signal.aborted) return;
      const status = error && typeof error === 'object' && 'status' in error ? error.status : undefined;
      if (status === 401 || status === 403) deny(abort);
      else if (submitted && status === 409 && error instanceof Error && error.message === 'revision_conflict' && record) {
        try { await clearRecovery(slot, record, abort.signal); if (!abort.signal.aborted) { setPending(null); setFavorite(null); setState('conflict'); setNotice('收藏修订冲突，已保留操作意图。请重读后再次确认，不会自动覆盖。'); } }
        catch { if (!abort.signal.aborted) { setState('unknown'); setNotice('原恢复身份无法安全清除，只能查询原结果。'); } }
      } else { setState(record ? 'unknown' : 'ready'); setNotice(record ? '原结果未确认；保留原键，只查询原结果。未找到不代表从未提交。' : '恢复存储或跨标签锁不可用，未发送收藏修改。'); }
    } finally { running.current = false; if (!abort.signal.aborted) setBusy(false); }
  }
  async function accept(receipt: Receipt, record: Recovery, signal: AbortSignal) {
    signal.throwIfAborted();
    if (!receipt || !hexID(receipt.favorite_id) || receipt.space_id !== identity.sourceId || receipt.request_digest !== record.digest || receipt.result_revision !== record.expected + 1 || receipt.deleted !== (record.action === 'delete') || (record.favorite && receipt.favorite_id !== record.favorite) || typeof receipt.committed_at !== 'string' || !Number.isFinite(Date.parse(receipt.committed_at))) throw new Error('favorite_receipt_mismatch');
    await clearRecovery(slot, record, signal);
    signal.throwIfAborted(); setPending(null); setIntent(null); setNotice('已确认原收藏操作；收藏不授予额外权限，取消收藏不会删除笔记。'); setAttempt(value => value + 1);
  }
  const currentAction: Action = favorite && !favorite.deleted ? 'delete' : 'create';
  const fulfilled = state === 'ready' && !!intent && intent !== currentAction;
  return <section aria-label="笔记收藏" className="space-y-2 border-t pt-3">
    {state === 'loading' && <p role="status" className="text-xs">正在读取收藏状态…</p>}
    {state === 'unsupported' && <p role="status" className="text-xs">此收藏入口仅支持普通文件，不操作 Pinax 或外部来源。</p>}
    {state === 'denied' && <p role="alert" className="text-xs">权限、网络或上下文已变化，收藏数据已清除。</p>}
    {state === 'error' && <p role="alert" className="text-xs">无法确认收藏状态，不能当作未收藏。请重读完整授权列表。</p>}
    {state === 'ready' && <>
      <p className="text-xs text-muted-foreground">{favorite && !favorite.deleted ? '已收藏此笔记' : '尚未收藏此笔记'}{favorite ? ` · 修订 ${favorite.revision}` : ''}</p>
      {fulfilled ? <><p className="text-xs">当前状态已符合原操作意图，但这不是原请求成功回执。</p><Button variant="outline" className="min-h-11 w-full text-xs" disabled={busy} onClick={() => { setIntent(null); setNotice(''); }}>接受当前收藏状态</Button></> : <Button variant="outline" className="min-h-11 w-full text-xs" disabled={busy} onClick={() => void change(intent ?? currentAction)}>{intent ? intent === 'create' ? '确认重新收藏' : '确认重新取消收藏' : currentAction === 'create' ? '收藏笔记' : '取消收藏'}</Button>}
    </>}
    {state === 'unknown' && pending && <Button variant="outline" className="min-h-11 w-full text-xs" disabled={busy} onClick={() => void change('lookup')}>查询原收藏结果</Button>}
    {['ready', 'error', 'conflict'].includes(state) && <Button variant="ghost" className="min-h-11 w-full text-xs" disabled={busy} onClick={() => setAttempt(value => value + 1)}>重读收藏状态</Button>}
    {notice && <p role="status" className="text-xs leading-5">{notice}</p>}
  </section>;
}
