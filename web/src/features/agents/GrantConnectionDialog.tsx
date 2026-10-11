import { useEffect, useRef, useState } from 'react';
import { Copy } from 'lucide-react';
import { AppDialog } from '../../components/AppDialog';
import { Button } from '../../components/ui/button';
import { Input } from '../../components/ui/input';
import type { Grant } from './grant-client';

const states: Record<Grant['state'], string> = { active: '有效', disabled: '已禁用', expired: '已到期', revoked: '已撤销' };
const actions: Record<string, string> = { 'files.read': '读取文件', 'labels.suggest': '建议标签', 'inbox.submit': '提交收件' };

// The template accepts only a service origin. Never silently strip credentials,
// a reverse-proxy path, query parameters, or fragments into another destination.
function serviceOrigin(value: string) {
  const input = value.trim();
  if (!input || /[\s?#]/.test(input)) return '';
  try {
    const url = new URL(input);
    if (!['http:', 'https:'].includes(url.protocol) || url.username || url.password || url.pathname !== '/') return '';
    return url.origin;
  } catch { return ''; }
}

export function GrantConnectionDialog({ grant, close }: { grant: Grant; close: () => void }) {
  const [address, setAddress] = useState(window.location.origin);
  const [notice, setNotice] = useState('');
  const [invalid, setInvalid] = useState(false);
  const [copying, setCopying] = useState(false);
  const generation = useRef(0);
  const live = useRef(true);
  useEffect(() => {
    live.current = true;
    const invalidate = () => { live.current = false; generation.current++; setInvalid(true); setAddress(''); setNotice(''); close(); };
    window.addEventListener('drivebridge-context', invalidate);
    return () => { live.current = false; generation.current++; window.removeEventListener('drivebridge-context', invalidate); };
  }, [close]);
  const base = serviceOrigin(address);
  const protocol = grant.protocol;
  const canRead = grant.actions.includes('files.read');
  const query = new URLSearchParams({ space_id: grant.space_id, prefix: grant.prefix });
  const httpPath = canRead ? 'files?' + query.toString() : 'capabilities';
  const template = protocol === 'mcp' ? JSON.stringify({ mcpServers: { drivebridge: {
    type: 'http', url: base + '/mcp', headers: { Authorization: 'Bearer <AGENT_TOKEN>' },
  } } }, null, 2) : `GET ${base}/api/v1/${httpPath}\nAuthorization: Bearer <AGENT_TOKEN>`;
  async function copy() {
    if (!base || !live.current || copying) return;
    const current = generation.current;
    setCopying(true); setNotice('');
    try {
      await navigator.clipboard.writeText(template);
      if (live.current && generation.current === current) setNotice('已复制占位配置；尚未配置客户端，也未验证连接。');
    } catch {
      if (live.current && generation.current === current) setNotice('无法访问剪贴板，请手动选择并复制占位配置。');
    } finally { if (live.current && generation.current === current) setCopying(false); }
  }
  if (invalid) return null;
  return <AppDialog title="Agent 接入配置" close={close}>
    <div className="min-w-0 space-y-3 text-[13px] leading-5">
      <div className="space-y-1 border-b border-border pb-3">
        <h3 className="m-0 line-clamp-2 break-words text-sm font-semibold" title={grant.name}>{grant.name}</h3>
        <p className="m-0 text-muted-foreground">{protocol.toUpperCase()} · {states[grant.state]} · 授权版本 {grant.revision}</p>
        {grant.state !== 'active' && <p role="status">此授权{states[grant.state]}，配置模板不会恢复其权限。请先在授权管理中确认状态。</p>}
        <p className="m-0 line-clamp-2 break-all" title={grant.prefix || '空间内全部目录'}>目录：{grant.prefix || '空间内全部目录'}</p>
        <p className="m-0 break-words text-muted-foreground">{grant.actions.map(action => actions[action] || action).join(' · ')}</p>
      </div>
      <label className="grid gap-2">Agent 服务地址<Input aria-label="Agent 服务地址" className="min-h-11" value={address} onChange={event => { generation.current++; setAddress(event.target.value); setNotice(''); setCopying(false); }} autoComplete="off" spellCheck={false}/></label>
      {base.startsWith('http:') && <p role="status" className="m-0 text-xs text-muted-foreground">未加密 HTTP 仅用于可信本地测试；远程使用 HTTPS。</p>}
      <Button variant="outline" className="min-h-11" disabled={!base || copying} onClick={() => void copy()}><Copy className="size-4"/>{copying ? '正在复制…' : '复制占位配置'}</Button>
      {notice && <p role="status">{notice}</p>}
      {base ? <pre aria-label="占位接入配置" className="m-0 max-w-full whitespace-pre-wrap break-all rounded-md border border-border bg-muted p-3 font-mono text-xs leading-5 text-foreground">{template}</pre> : <p role="alert">请输入不含凭据、路径、查询参数或片段的 HTTP / HTTPS 服务源地址。</p>}
      <p className="m-0 text-xs text-muted-foreground">仅在客户端私有配置中替换 &lt;AGENT_TOKEN&gt;；此页不存凭据、不发送请求。复制不等于已连接。</p>
      <details className="border-t border-border text-xs leading-6 text-muted-foreground">
        <summary className="min-h-11 cursor-pointer py-3 text-[13px] font-medium text-foreground focus-visible:outline-2 focus-visible:outline-ring">完整授权范围</summary>
        <dl className="m-0 space-y-2 break-all"><div><dt>名称</dt><dd className="m-0">{grant.name}</dd></div><div><dt>空间</dt><dd className="m-0">{grant.space_id}</dd></div><div><dt>限定目录</dt><dd className="m-0">{grant.prefix || '空间内全部目录'}</dd></div></dl>
        <p>这是打开时的授权快照，不是当前可用性证明；每次实际访问仍由服务端重新校验。</p>
      </details>
      <details className="border-t border-border text-xs leading-6 text-muted-foreground">
        <summary className="min-h-11 cursor-pointer py-3 text-[13px] font-medium text-foreground focus-visible:outline-2 focus-visible:outline-ring">发现能力与失败恢复</summary>
        <p>填写客户端可达的服务源地址（协议、主机和可选端口），不要附加路径、查询参数、片段或凭据。localhost 仅适用于同一设备；远程访问请使用 HTTPS。</p>
        {protocol === 'mcp' ? <p>远程 MCP 无需安装 DriveBridge CLI。此模板用于支持 Streamable HTTP 的客户端，外层格式请按客户端要求调整。连接后先读 tools/list、inputSchema 和 drivebridge://capabilities；文件参数使用本授权的空间与限定目录。</p> : <p>示例是 HTTP 请求，不会在网页中执行。完整动作和恢复参数从 GET /api/v1/capabilities 获取；不含 files.read 时不提供文件列表请求。</p>}
        <p>遗失凭据时使用原授权的“凭据管理”轮转，不要重复创建身份。</p>
        <p>HTTP 数据传输与 MCP 控制面分开。标签建议和产物提交仅在授权动作与能力合同允许时使用专用 HTTP 入口，不是新增 MCP 工具；不支持 HTTP 的客户端不能退回宽泛写权限。本机路径不会自动成为服务端可读文件。</p>
        <p>请求结果未知时先查询原 operation 或原幂等键的恢复入口，按能力合同处理，不自动创建新请求。配置复制、检查通过或调用返回都不等于产物已验收或归档。</p>
      </details>
    </div>
  </AppDialog>;
}
