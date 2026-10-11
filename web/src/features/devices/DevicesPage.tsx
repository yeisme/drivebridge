import { useEffect, useState } from 'react';
import { Monitor, HardDriveDownload, Smartphone, Wifi, WifiOff } from 'lucide-react';
import { WorkspacePageHeader } from '../../components/WorkspacePageHeader';
import { Button } from '../../components/ui/button';
import type { Page } from '../../app/navigation-state';

export function DevicesPage({ onNavigate }: { onNavigate: (page: Page) => void }) {
  const [online, setOnline] = useState(navigator.onLine);
  useEffect(() => {
    const update = () => setOnline(navigator.onLine);
    window.addEventListener('online', update); window.addEventListener('offline', update);
    return () => { window.removeEventListener('online', update); window.removeEventListener('offline', update); };
  }, []);
  return <section aria-label="设备与离线" className="min-w-0 space-y-5">
    <WorkspacePageHeader title="设备与离线" description="当前是网页客户端。设备登记、固定版本离线副本和后台任务需要受控宿主提供公开能力。"/>
    <section aria-label="当前客户端" className="flex flex-wrap items-start gap-3 border-y border-border bg-card px-4 py-4">
      <Monitor className="mt-1 size-5 shrink-0 text-muted-foreground"/>
      <div className="min-w-0 flex-1 space-y-2"><h3 className="text-sm font-semibold">当前浏览器</h3><p className="flex items-center gap-2 text-[13px]">{online ? <Wifi className="size-4"/> : <WifiOff className="size-4"/>}{online ? '浏览器报告网络可用' : '浏览器报告离线'}</p><p className="max-w-2xl text-xs leading-5 text-muted-foreground">仅为浏览器网络信号，不证明服务、存储或其他设备在线。此页面没有登记当前设备，也不会扫描本机文件。</p></div>
      <Button variant="outline" className="min-h-11 text-[13px] md:min-h-8" onClick={() => onNavigate('capabilities')}>查看服务能力</Button>
    </section>
    <section aria-label="离线副本能力" className="space-y-3"><h3 className="flex items-center gap-2 text-sm font-semibold"><HardDriveDownload className="size-4"/>固定版本离线副本</h3><p role="status" className="text-[13px] leading-6 text-muted-foreground">当前网页尚未接入受管离线副本合同，无法报告已缓存的文件或版本。普通下载是用户保存的独立文件，不算受管离线副本，也不能保证撤权后召回。</p><Button variant="outline" className="min-h-11 text-[13px] md:min-h-8" onClick={() => onNavigate('files')}>前往网盘查看与下载</Button></section>
    <section aria-label="原生设备能力" className="space-y-3 border-t border-border pt-4"><h3 className="flex items-center gap-2 text-sm font-semibold"><Smartphone className="size-4"/>相册、系统分享与后台传输</h3><p role="status" className="text-[13px] leading-6 text-muted-foreground">这些能力需要原生宿主与设备权限。本轮 Web 不显示演示设备、不承诺关闭网页后继续备份；Electron、手机原生及三平台验证分别交付。</p><Button variant="outline" className="min-h-11 text-[13px] md:min-h-8" onClick={() => onNavigate('automation')}>查看服务端整理规则</Button></section>
  </section>;
}
