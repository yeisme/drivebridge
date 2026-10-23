import { useEffect, useRef, useState, type ReactNode } from 'react';
import { BookOpen, ShieldCheck, PanelLeft, RefreshCw, Monitor } from 'lucide-react';
import { BrandMark } from '../../BrandMark';
import { Button } from '../../components/ui/button';
import { ThemeToggle } from '../../components/ThemeToggle';
import { WorkspaceShell, WorkspaceMain } from '../../app/WorkspaceShell';
import { DesktopNavigation } from '../../app/DesktopNavigation';
import { useMobileViewport } from '../../app/useMobileViewport';
import { useSidebarPreference } from '../../app/useSidebarPreference';
import type { Page } from '../../app/navigation-state';
import { visibleNavGroups } from '../../nav-groups';
import { readDesktopCapabilities, type DesktopEntryBridge } from '../../app/desktop-entry';
import { PinaxReaderWorkspace } from './PinaxReaderWorkspace';

const navigation: readonly [Page, string, ReactNode][] = [['knowledge', '本机知识库', <BookOpen key="knowledge" />], ['settings', '桌面能力说明', <ShieldCheck key="settings" />]];

// The same production Web shell, without inventing a cloud identity, spaces,
// files or a fake source. Domain readers mount only after owner admission.
export function DesktopWorkspace({ bridge, boundSourceId }: { bridge?: DesktopEntryBridge; boundSourceId?: string }) {
  const mobile = useMobileViewport();
  // No instance identity exists yet. Layout preferences remain memory-only.
  const [collapsed, toggle, reset, explicit] = useSidebarPreference('', '');
  const [page, setPage] = useState<Page>('knowledge');
  const [expanded, setExpanded] = useState<Record<string, boolean>>({});
  const [attempt, setAttempt] = useState(0);
  const [state, setState] = useState<'loading' | 'ready' | 'error'>('loading');
  const [available, setAvailable] = useState<string[]>([]);
  const [selecting, setSelecting] = useState(false);
  const [selectionStatus, setSelectionStatus] = useState('');
  const generation = useRef(0);
  useEffect(() => { generation.current++; setSelectionStatus(''); setSelecting(false); return () => { generation.current++; }; }, [bridge]);
  async function chooseVault() {
    if (selecting || !bridge?.chooseVault) return;
    const epoch = generation.current;
    setSelecting(true); setSelectionStatus('');
    try {
      const result = await bridge.chooseVault({ input: {} });
      if (generation.current !== epoch) return;
      if ('error' in result) setSelectionStatus(result.error.code === 'native_selection_cancelled' ? '已取消选择，未连接来源。' : '无法完成来源检查，未连接来源。');
      else if (result.data.decision === 'blocked') setSelectionStatus(result.data.reason === 'managed_scope_unverifiable' ? '受管范围保护尚未通过验证，来源未连接；正文和编辑均未开放。' : '所选目录未通过只读检查，来源未连接。');
      else setSelectionStatus('宿主返回了当前不支持的绑定状态，来源未连接。');
    } catch { if (generation.current === epoch) setSelectionStatus('来源检查失败，未读取正文。'); }
    finally { if (generation.current === epoch) setSelecting(false); }
  }
  useEffect(() => {
    let active = true;
    setState('loading'); setAvailable([]);
    const deadline = window.setTimeout(() => { if (active) { active = false; setState('error'); } }, 20000);
    Promise.resolve().then(() => {
      if (!bridge) throw new Error('desktop_host_missing');
      return bridge.capabilities({ input: {} });
    }).then(value => {
      if (!active) return;
      setAvailable(readDesktopCapabilities(value)); setState('ready');
    }).catch(() => { if (active) setState('error'); }).finally(() => window.clearTimeout(deadline));
    return () => { active = false; window.clearTimeout(deadline); };
  }, [bridge, attempt]);
  const title = page === 'knowledge' ? '本机知识库' : '桌面能力说明';
  const reader = bridge?.listNotes && bridge.searchNotes && bridge.readDocument && bridge.readLinks
    ? { listNotes: bridge.listNotes, searchNotes: bridge.searchNotes, readDocument: bridge.readDocument, readLinks: bridge.readLinks, syncStatus: bridge.syncStatus, syncDiff: bridge.syncDiff, listSyncConflicts: bridge.listSyncConflicts }
    : undefined;
  return <WorkspaceShell mobile={mobile} collapsed={collapsed} focused={false}>
    {!mobile && <aside className="sidebar shrink-0 overflow-hidden border-0 p-0 group-data-[collapsed=true]/shell:p-0">
      <a href="#knowledge" aria-label="DriveBridge 本机工作区" className="brand flex h-16 shrink-0 items-center gap-2 px-3 text-[15px] group-data-[collapsed=true]/shell:justify-center group-data-[collapsed=true]/shell:px-0" onClick={() => setPage('knowledge')}><span className="brandmark"><BrandMark size={32}/></span><span className="brand-name">DriveBridge</span></a>
      <DesktopNavigation groups={visibleNavGroups(['knowledge', 'settings'], expanded)} items={navigation} page={page} title={title} pendingCount={0} inviteCount={0} onToggle={(id, open) => setExpanded(current => ({ ...current, [id]: open }))} onNavigate={setPage}/>
      {explicit && <Button variant="ghost" className="mx-2 mb-2 min-h-11" aria-label="恢复自动导航布局" onClick={reset}><PanelLeft/><span className="group-data-[collapsed=true]/shell:hidden">自动布局</span></Button>}
      <p className="m-3 flex items-center gap-2 text-xs text-muted-foreground"><Monitor className="size-4 shrink-0"/><span className="group-data-[collapsed=true]/shell:hidden">本机模式 · 不需要云登录</span></p>
    </aside>}
    <WorkspaceMain>
      <header className="topbar"><ThemeToggle/>{!mobile && <Button variant="ghost" size="icon" className="min-h-11 min-w-11" aria-label={collapsed ? '展开侧边栏' : '折叠侧边栏'} aria-expanded={!collapsed} onClick={toggle}><PanelLeft/></Button>}<h1 className="m-0 text-base font-semibold">{title}</h1></header>
      {mobile && <nav aria-label="本机导航" className="flex flex-wrap gap-2 border-b border-border p-3">{navigation.map(([target, label, icon]) => <Button key={target} variant={page === target ? 'secondary' : 'ghost'} className="min-h-11" aria-current={page === target ? 'page' : undefined} onClick={() => setPage(target)}>{icon}{label}</Button>)}</nav>}
      <section aria-label={title} className="m-4 flex min-w-0 flex-col gap-4 rounded-lg border border-border bg-card p-4 md:m-6 md:p-6">
        <div className="flex flex-wrap items-center gap-3"><BookOpen className="size-5 text-primary"/><h2 className="m-0 text-lg font-semibold">{page === 'knowledge' ? '连接前的本机检查' : '当前交付边界'}</h2><Button variant="outline" className="ml-auto min-h-11" disabled={state === 'loading' || selecting} onClick={() => setAttempt(value => value + 1)}><RefreshCw className="size-4"/>重新检查</Button></div>
        {state === 'loading' && <p role="status" className="text-sm text-muted-foreground">正在检查桌面宿主能力…</p>}
        {state === 'error' && <p role="alert" className="text-sm text-destructive">桌面宿主未响应或合同不兼容。未读取本机笔记，请重新检查。</p>}
        {state === 'ready' && <p role="status" className="text-sm text-muted-foreground">{available.includes('chooseVault') && bridge?.chooseVault ? '可以选择已有 vault 进行只读预检。' : '桌面宿主已连接，尚未开放来源操作。'}</p>}
        {state === 'ready' && available.includes('chooseVault') && bridge?.chooseVault && <Button className="min-h-11 self-start" disabled={selecting} onClick={() => void chooseVault()}>{selecting ? '正在选择与检查…' : '选择本机 Pinax vault'}</Button>}
        {selectionStatus && <p role="status" className="text-sm text-muted-foreground">{selectionStatus}</p>}
        {state === 'ready' && boundSourceId && reader && <PinaxReaderWorkspace bridge={reader} sourceId={boundSourceId}/>}
        <div className="rounded-md border border-border bg-muted/40 p-4 text-sm leading-7"><strong className="font-medium">尚未建立 Pinax 来源</strong><p className="m-0">预检不会建立绑定。受管范围保护通过验证之前，不开放正文阅读或编辑。</p></div>
        {page === 'settings' && <ul className="m-0 space-y-2 pl-5 text-sm leading-7"><li>此窗口复用正式 Web 的导航、主题和阅读工作区组件。</li><li>当前不连接云文件服务，不创建云用户、空间或虚构笔记。</li><li>桌面文件、全文阅读、编辑及三平台实机验收仍未完成；原浏览器功能保持独立可用。</li></ul>}
      </section>
    </WorkspaceMain>
  </WorkspaceShell>;
}
