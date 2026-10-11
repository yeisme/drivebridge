# 本轮网页验证

- Vitest：`temp/integration-test-runs/20260911T074412Z-5d3b9f0a/`，API、CSRF 和安全错误显示通过。
- Playwright：`temp/integration-test-runs/20260911T074417Z-9fb7a47c/`，两条完整浏览器流程通过。
- 同轮 TypeScript/Vite 构建通过；源码格式检查通过。
- 桌面截图：`temp/integration-test-runs/20260911T074417Z-9fb7a47c/artifacts/desktop-files.png`。
- 移动端截图：`temp/integration-test-runs/20260911T074417Z-9fb7a47c/artifacts/mobile-files.png`。

实际浏览器覆盖登录、文件上传/拖拽、新版本、Agent 创建与撤销、只读拒绝、跨用户拒绝、回收站恢复、刷新后的同一操作恢复和移动端无横向溢出。对话框使用原生 dialog 管理键盘焦点；下载交给浏览器原生管理器，避免大文件累积在 JS Blob 内存。

初次浏览器失败源于测试身份页未声明 UTF-8，修复 fixture 后通过，没有降低真实服务认证要求。初次构建缺少 Node 类型定义，补齐开发依赖后通过。失败证据保留。

OIDC/S3 为真实本机 HTTP fixture；没有真实账户授权或公网部署。截图只在 token 对话框关闭后采集；trace/video/自动失败截图关闭，文本测试产物经脱敏。

## personal-agent-loop-v1（2026-09-12）

- Vitest 4 tests：`temp/integration-test-runs/20260912T161246Z-3f08f36b/`，覆盖复制过程中并发新版本不能替换所选引用。
- Playwright 3 tests：`temp/integration-test-runs/20260912T162055Z-9af9d095/`，覆盖真实 fixture 登录、拖拽、版本、限定授权、撤销、双用户隔离、刷新恢复，以及来源展示/引用复制/结果定位。
- TypeScript 检查与生产构建通过。
- 已检查该次 artifacts 下的手机列表和文件来源详情截图；没有拍摄 token 对话框。
- 本 owner OpenSpec 严格校验通过；真实账号、发布、部署和市场试用仍未执行。

## 2026-09-16 多租户管理与授权

- Vitest 21 项通过：`temp/integration-test-runs/20260916T112038Z-8ac3e977/`。包含标签页上下文、陈旧响应拒绝、只读呈现与摘要计算期间切换租户不发起上传。
- `pnpm build` 通过。普通 fixture 浏览器回归 8 项通过：`temp/integration-test-runs/20260916T112353Z-c5bd11f1/`；该配置按设计跳过治理专用的两项。
- 治理 fixture 浏览器验证 2 项通过：`temp/integration-test-runs/20260916T112417Z-9dbabd40/`。覆盖双用户审批与撤销、管理只读、通知、HTTP 拒绝、移动端恢复入口和登出。
- 早期旧首页预期差异及媒体文件并行写入导致的启动失败保留在原失败 evidence 中；在工作区恢复稳定后重新验证通过。没有回退并行 Finder、本机工作区或媒体改动。
- OpenSpec `drivebridge-multitenant-governance-v1` 验证通过。未使用真实账号、未部署、未发布。
