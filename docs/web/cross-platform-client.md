# 跨平台 Client 支持设计

## 当前桌面决定（2026-10-01）

用户明确选择 **Electron + 既有 React/Vite**，由 `cli/drivebridge/desktop/` 同仓维护；桌面内置 Pinax 阅读空间，首片无云登录、离线阅读优先，macOS/Windows/Linux 必须全部实机验收。详见 [Electron 架构](../architecture/electron-desktop.md)、[阅读 UI](ui/knowledge-reading-spec.md) 与 [平台验收](../delivery/desktop-knowledge-acceptance.md)。当前只交规格，未创建宿主或安装包；不能把下表旧 CLI/Web 构建证据用于新桌面。

Web/PWA 继续维护，DriveBridge 手机独立客户端需求保留；本次 Pinax 手机内容通过 Hermes，不新增手机阅读 UI。以下为历史 Web/PWA 阶段记录，所有“不建桌面壳/不做 Electron/移动唯一 Web”仅描述当时范围，不再是当前长期限制。

## 历史 Web/PWA 阶段

2026-09-27 产品目标修订：用户明确要求未来桌面与手机均提供独立安装客户端，且两端同等优先。当前 [双端产品原型](ui/personal-agent-prototype.md) 已纳入系统采集、设备续接、离线与后台状态设计；原生框架、运行时、签名分发和系统实测另行实施。下文“浏览器即 client / 本阶段不做桌面壳”记录既有实现阶段，不再作为未来客户端能力的永久排除项；现有 Web/PWA 继续维护。

DriveBridge client 的跨平台原则：**浏览器即 client**。不建独立桌面壳（Tauri/Electron）或原生移动应用；一个 web 资产同时服务服务端部署与 `drivebridge view` 本机只读视图，覆盖用户全部设备。Agent 面保持 CLI/MCP（`cli/drivebridge` 拥有），与人用面分离。

## 平台矩阵

| 平台 | 服务端网页 | 本机 `drivebridge view` | CLI / MCP | 状态 |
|---|---|---|---|---|
| Linux amd64/arm64 | ✓ 浏览器 | ✓ xdg-open | ✓ | 本仓 fixture/e2e 已验证 |
| macOS amd64/arm64 | ✓ 浏览器 | ✓ open | ✓ 分发包 | 构建验证；实机验收待记录 |
| Windows amd64/arm64 | ✓ 浏览器 | 编译候选 | 编译候选 | `local-candidate-windows-unverified-on-machine` |
| 移动端浏览器（iOS/Android） | ✓ 响应式 + 可安装 | ✗ 恒 loopback | ✗ | 服务端部署后可用 |

配套 mcp change：`drivebridge-cross-platform-distribution-v1`（平台锁抽象、三平台浏览器启动、windows zip 分发目标）。

## Windows 实机验收门（全部未勾，如实）

Windows 二进制目前**只保证编译通过与打包完整**，以下门任何一项未在真实 Windows 机器验证前不得宣称可用：

1. **state 目录权限语义**：`bridge.New` / `readInsights` 用 `Perm()&0077 != 0` 判私有；Windows 的 `os.Stat` 权限位由 ACL 合成（典型 0666/0777），检查会误报 `state_not_private`。方向：Windows 下改查 owner-only DACL，或显式 `--development` 类豁免并记录；实现前 Windows 本机模式不可用。
2. **socket 隐私**：`insights.sock`/`handoff.sock` 为 AF_UNIX（Win10 1803+，Go 支持），但 `os.Chmod(0600)` 在 Windows 无 Unix 语义；隐私依赖默认安全描述符，需实机验证「其他用户不可连」。
3. **浏览器启动**：`cmd /c start "" <url>` 分支已写，未实机跑过。
4. **rclone / ffprobe**：内嵌 rclone 在 Windows 的行为与 ffprobe 可用性未验证；`doctor` 的 ffprobe 检查逻辑通用。
5. **锁等价性**：LockFileEx FAIL_IMMEDIATELY 与 flock 非阻塞语义等价性需实机并发验证。

## 移动端定位

- **服务端部署 + 手机浏览器**是移动端唯一官方路径；网页已响应式（移动端导航/触控有 e2e 截图证据）。
- **PWA 可安装**（本轮落地）：`manifest.webmanifest` + SVG 图标，Chromium/桌面浏览器可「安装为应用」，无新壳、无新运行时。iOS 主屏图标需 PNG `apple-touch-icon` 资产——记为后续项。
- **本机 view 恒 loopback**：不为移动访问开 LAN 监听（一次性 fragment + loopback 是安全边界）；远程访问 = 部署服务端，或用户自建 VPN/Tailscale，DriveBridge 不新增监听面。

## 历史桌面壳评估（已由 2026-10-01 Electron 决定取代）

Tauri/Electron 会引入第二个运行时、独立更新通道、签名与打包面；浏览器 + PWA 安装已覆盖同等体验，且与服务端网页同一份资产。与「独立 Workbench 退役」（根仓 2026-09-07 决策）同向：不建平行壳。

客户端观感按三阶段写在 [客户端壳](ui/client-shell.md)：

1. **现在（`drivebridge-client-shell-v1`）**：同一份 web 资产，`standalone` + `window-controls-overlay` 回退，桌面 shortcuts 指向网盘与传输。视窗锁定与分组导航让安装后的窗口像文件客户端，而不是浏览器标签页里的后台。
2. **评估中**：轻量宿主（Tauri v2 / Wails）只允许复用 `dist/`，不得复制领域状态。未批准实现。
3. **以后**：托盘、file_handlers、本机文件系统事件。需单独 OpenSpec 与根边界审批。

当时未实施 Electron；当前由独立桌面 change 承接，不改造本机 `drivebridge view` 为安装客户端。

## 分发矩阵

| 产物 | 内容 | 平台 |
|---|---|---|
| `drivebridge-<os>-<arch>.tar.gz` | drivebridge + web/ + 文档 | linux/darwin（默认构建） |
| `drivebridge-windows-<arch>.zip` | drivebridge.exe + web/ + 文档 | windows（显式 `--target`） |
| 服务端 `dist/` | 双入口 SPA（index + insights）+ manifest | 全平台浏览器 |

web/ 资产与 `cli/drivebridge/web/dist` 同源构建，一次构建双面复用。
