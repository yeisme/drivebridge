# DriveBridge Client

本目录是单仓 `cli/drivebridge` 的内部模块。日常开发从父目录执行 `make dev` / `make build` / `make test`；统一规格见 `../openspec`，开发说明见 [统一入口](../docs/development.md)。下文保留模块使用说明。

新版“我的网盘与 Agent 工作台”已形成 [正式 PRD](../docs/web/product/personal-agent-workspace-prd.md) 和 [前端开工包](../openspec/changes/drivebridge-personal-workspace-rebuild-v1/README.md)：以 2026-09-30 当前 OpenDesign 原型为准，先清理旧个人壳层，再重建双端页面。当前仅完成设计与任务拆分，下面的已实现功能属于旧客户端基线；新前端尚未删除或重写。详细顺序见 [清理与兼容计划](../docs/web/ui/personal-workspace-rebuild.md)。

上一轮 UI 改版已落实为 [紧凑工作区与可读性规范](../docs/web/ui/compact-workspace.md)：独立管理页、目录列表详情、可读表格与 Finder 四视图。验证范围与截图见 [该轮验收记录](../docs/web/ui/compact-workspace-validation.md)，其交付范围为本地开发预览。

个人文件空间网页，消费 DriveBridge 公共 HTTP API。不持有数据库、存储密钥或业务审核规则。团队与租户治理页已经交付，但是 committed 管理面，不是个人楔子的主叙事；个人往返核 `#/files`、`#/transfers`、`#/inbox`、`#/agents`。

## 构建与本地预览

```bash
pnpm install
pnpm build
```

随后在服务端项目启动隔离的本地 fixture：

```bash
cd ../server
go run ./cmd/drivebridge-fixture --listen 127.0.0.1:8787 --web-dir ../web/dist
```

打开 `http://127.0.0.1:8787`。Alice/Bob 是测试身份，S3 也是本机 fixture，不需要真实账号或凭据。程序不会把测试登录接入真实 `serve` 模式。

正式使用时由服务端配置 OIDC 和存储，并指定本项目 `dist`。`pnpm dev` 提供 Vite 热开发代理；此时后端 `--origin` 必须与浏览器的实际 origin 一致，不能靠关闭 CSRF 检查解决跨端口问题。

## 首版交互

- 文件页：列表/网格、搜索、虚拟目录导航、拖拽或键盘选文件、版本查看、下载和新版本上传。
- 预览：20 MiB 内图片/视频、2 MiB 内文本；其他文件提示下载后查看。HTML/XML 等文本以文本节点显示，不执行内容。
- 传输中心：进度、取消、错误与恢复；页面刷新后，从服务端操作记录选择原文件继续，不持久化文件字节或凭据到 localStorage。
- 存储连接：平台 S3、自带 S3、OneDrive 授权、验证、断开，以及固定绑定的空间。
- Agent：限定目录/操作、默认只读七天、一次显示 token、撤销与最近活动。
- 回收站：恢复，或经确认永久删除；Agent 不执行永久清理。

上传先按 8 MiB 块计算完整 SHA-256，再只发送缺少的分片。上传过程中更换源文件会导致校验失败。成功/失败反馈、模态对话框与移动端导航共用一套视觉样式；键盘焦点由原生 dialog 管理。

OneDrive 文件浏览不自动同步整个网盘。用户主动导入才产生受管副本；外部源变化不会改写已有副本。回存和导入只传文件，不触发生成、发布或 canonical 采纳。

## 验证

```bash
pnpm test
pnpm build
pnpm test:e2e
```

Vitest 验证 API/CSRF/错误边界，Playwright 启动真实本地 API + OIDC/S3 fixture 验证登录、上传、版本、拖拽、Agent 授权/撤销、跨用户拒绝、中断恢复、回收站与移动端。

Playwright 首选现有 Chromium，也支持 `PLAYWRIGHT_CHROMIUM` 指定本机可执行文件。首个验证失败时保留证据，不修改生产权限来适配测试。

证据在 `temp/integration-test-runs/<run-id>/`，桌面/移动端截图仅捕获不含 token 的页面。禁用自动 trace/video/失败截图，避免一次显示的 token 被录入证据；测试日志由 runner 脱敏。

当前只完成本地候选，真实 OIDC 应用、OneDrive 账号、存储服务与公网部署独立验收。

## Agent 文件闭环增量

文件与版本显示调用方来源标签；详情可复制当前选定版本引用，历史版本可分别复制。复制前由服务端解析并验证版本，不因并发上传切换到最新版。传输完成后可通过“查看结果”定位该次操作版本。网页上传标记 caller=drivebridge-client；这些来源是声明标签，不替代服务端身份与权限。

引用只有在连接 DriveBridge 的客户端中才可消费；加密文件与纯远程 MCP 的数据传输限制仍由服务端决定。使用新版 cli/drivebridge/server 的 references 接口；旧服务未提供时显示操作失败，不伪造引用。无服务部署、真实账号或用户试用验收。

用量、图表、扫描与安全审计见[使用说明](../docs/web/insights-audit.md)。
