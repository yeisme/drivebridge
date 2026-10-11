import { Input } from "./components/ui/input";
import { Button } from "./components/ui/button";
import { SelectAdapter } from "./components/SelectAdapter";
import React, { useState } from "react";
import { Copy, Plug } from "lucide-react";

export function RobotSetup({ expanded = false }: { expanded?: boolean }) {
  const [mode, setMode] = useState<"mcp" | "cli">("mcp");
  const [origin, setOrigin] = useState(window.location.origin);
  const [notice, setNotice] = useState("");
  const [client, setClient] = useState("generic");
  let base = "";
  try { const url = new URL(origin); if (["http:", "https:"].includes(url.protocol) && !url.username && !url.password) base = url.origin; } catch {}
  const quote = (value: string) => "'" + value.replaceAll("'", "'\\''") + "'";
  const content = mode === "mcp" ? JSON.stringify({ mcpServers: { drivebridge: {
    type: "http", url: `${base}/mcp`, headers: { Authorization: "Bearer <ROBOT_TOKEN>" },
  } } }, null, 2) : [
    `drivebridge config set remote_url ${quote(base)}`,
    "drivebridge config set remote_token_file '/absolute/private/robot-token'",
    "drivebridge config set local_root '/absolute/local/project'",
    "drivebridge space list --json",
    `drivebridge install --client ${client}${client === "generic" ? " --json" : ""}`,
  ].join("\n");
  return <details className="robot-setup" open={expanded || undefined}>
    <summary><Plug size={18} />连接机器人：MCP / CLI 配置引导</summary>
    <ol><li>创建机器人授权，选择网盘、目录、权限和有效期。</li><li>保存仅显示一次的 token。已有授权无法再次取回 token，遗失后需撤销并重新创建。</li><li>选择接入方式，在机器人运行的设备上完成配置。</li></ol>
    <div className="robot-setup-tabs"><Button variant="outline" type="submit"  aria-pressed={mode === "mcp"} onClick={() => setMode("mcp")}>远程 MCP（无需 CLI）</Button><Button variant="outline" type="submit"  aria-pressed={mode === "cli"} onClick={() => setMode("cli")}>本机 CLI</Button></div>
    <label>服务地址<Input aria-label="机器人连接服务地址" value={origin} onChange={e => { setOrigin(e.target.value); setNotice(""); }} /></label>
    {mode === "cli" && <label>机器人客户端<SelectAdapter aria-label="机器人客户端" value={client} onChange={e => setClient(e.target.value)}><option value="generic">通用配置模板</option><option value="codex">Codex</option><option value="claude-code">Claude Code</option></SelectAdapter></label>}
    <p>这是机器人能访问的 DriveBridge 地址。localhost 仅适用于同一设备；远程机器人需要可达的 HTTPS 服务地址。</p>
    {mode === "mcp" ? <p>在支持 Streamable HTTP 的 MCP 客户端添加下方地址与 Authorization 请求头。模板中的 token 占位符只在客户端私有配置中替换；不同客户端的外层配置格式可能不同。</p> : <p>先安装 DriveBridge CLI。用本机编辑器将 token 保存到仅本人可读的私有文件，再替换下列文件与目录路径（POSIX shell 示例）。无需把 token 写进命令历史。最后一条命令输出通用 MCP 注册配置；复制到客户端后生效。</p>}
    {base ? <pre>{content}</pre> : <p role="alert">请输入有效的 HTTP 或 HTTPS 服务地址。</p>}
    <Button variant="outline" type="submit"  disabled={!base} onClick={async () => { try { await navigator.clipboard.writeText(content); setNotice("已复制配置模板；请替换占位符并在客户端完成配置。"); } catch { setNotice("无法访问剪贴板，请手动选择并复制模板。"); } }}><Copy size={16} />复制{mode === "mcp" ? "配置模板" : "配置命令"}</Button>
    {notice && <p role="status">{notice}</p>}
    {mode === "cli" && client !== "generic" && <p>最后一条安装命令会在本机注册对应客户端；执行前确认私有 token 文件与目录路径。复制命令不会自动执行安装。</p>}
    <p>验证：MCP 连接后查看 tools/list 与 inputSchema，再查询 capabilities；CLI 使用 space list 确认可见网盘。未安装 CLI 的机器人也可通过远程 MCP 查询工具与资源；本机路径不会自动变成服务端可读文件。</p>
  </details>;
}
