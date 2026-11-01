## Why

Pinax 已能维护笔记，但用户需要打开 Obsidian 或 VS Code 才会阅读，导致找回与再利用不足。DriveBridge 已确定建设 Markdown 阅读器和安装客户端，再建独立 Pinax Reader 会重复窗口、导航、阅读与分发。

## What Changes

- 用户确认一个 DriveBridge 客户端，Electron 桌面宿主，阅读先行；macOS、Windows、Linux 均须实机验收。
- 准入为 `split-owner`：DriveBridge 拥有客户端和阅读状态应用服务，Pinax 保留 Markdown vault、笔记身份、索引、关系及写入门；Hermes 保留手机入口。
- 明确 `cli/drivebridge/desktop/` 是获批的同仓宿主，不新建 `client/pinax-reader`，不恢复 Workbench，不新增笔记后端。
- 建立跨项目合同与验收交接；人类完整阅读不改变现有 Agent bounded projection 的权限和脱敏默认值。
- 保留编辑、捕获、提案审阅、整理以及 DriveBridge 移动客户端需求，分期而非删除；本次只落规格与任务。

## Capabilities

### New Capabilities

- `drivebridge-pinax-reading-handoff`：单客户端消费、领域真源、受管范围、两种读取受众、三平台验收与后续能力保留。

### Modified Capabilities

无已有协议删除或重命名。旧 Workbench 与“不做 Electron”是被当前用户决定取代的设计路线，不重写归档。

## Impact

- 根仓只更新治理与交接；不承载客户端或 Pinax 产品实现进度。
- DriveBridge 实施包：[drivebridge-desktop-knowledge-reader-v1](../drivebridge-desktop-knowledge-reader-v1/README.md)。复用既有 F06/F07/F09/B09，增补宿主、适配与独立阅读验收，不复制任务。
- Pinax 实施包：[pinax-human-reading-api-v1](../../../../pinax/openspec/changes/pinax-human-reading-api-v1/README.md)。新增人读 API 与受监督本机会话，不增加内部 Web/Electron。
- 本轮无产品源码、依赖、用户数据、凭据、同步、构建、发布或部署变更；Pinax 既有 11 项真实验收与原观察窗口不变。
