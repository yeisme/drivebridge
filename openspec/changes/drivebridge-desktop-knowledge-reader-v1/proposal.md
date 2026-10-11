## Why

用户需要在日常打开的 DriveBridge 内阅读和找回 Pinax 笔记，而非另装 Reader 或依赖编辑器。现有工作台已计划 Markdown/知识库/客户端，应该复用阅读器和壳层，避免重复建设。

## What Changes

- 新增同仓 `desktop/` Electron main/preload/Forge，复用 `web/`；三平台本机离线阅读首片，不要求云登录或预装 CLI。
- 新增 Pinax 固定 adapter、显式 vault binding、跨文件入口保护和本机阅读状态；Pinax 仍是笔记领域 owner。
- 复用原 F06/F07/F09/B09 的组件和规则，仅增补独立阅读纵切、宿主安全与平台验收；不等待完整 Agent/团队工作台。
- 普通文件和 Pinax 共用 UI，不复制解析领域逻辑；长文按 Pinax 人读合同分段，不把旧 Agent 截断树当全文。
- 编辑/捕获/提案/整理保留后续合同任务；移动客户端需求保留。本次只交付规格，不创建源码/依赖或执行真实迁移。

## Capabilities

### New Capabilities

- `desktop-host`：Electron 宿主、安全 IPC、受控进程、安装/升级与三平台门。
- `knowledge-reader`：来源适配、三屏、全文分页、关系/资产及无障碍。
- `managed-vault-boundary`：显式绑定、所有应用入口保护、旧挂载兼容及安全回退。
- `reading-state`：本机收藏/稍后读/位置的 owner 状态、CAS和身份隔离。

### Modified Capabilities

无既有 API 删除或重命名。旧“不做 Electron”设计由当前选择取代；旧 CLI/Web/PWA/mount 合同保留。新绑定是显式 opt-in，不自动修改旧用户数据。

## Impact

- 未来实现范围：`desktop/`、现有 `web/`、根本机 application service及同仓共享 guard/reading-state、必要 server 接入、合同生成和既有测试工具。
- 外部依赖只通过 [Pinax 人读 API](../../../../pinax/docs/interfaces/human-reading-api.md)；不导入 Pinax internal 包或私有存储，不依赖通用 client-runtime。
- 文档真源：[Electron](../../../docs/architecture/electron-desktop.md)、[UI](../../../docs/web/ui/knowledge-reading-spec.md)、[验收](../../../docs/delivery/desktop-knowledge-acceptance.md)。
- 实现任务全部待办；构建、签名/公证、发布、真实使用及三平台实测分开报告，禁止 race。
