# Changelog

## [0.1.2] - Unreleased

CI、fresh-clone 构建、版本一致性和安全边界修复进行中；本版本尚未发布。

## [0.1.0] - 2026-10-15

DriveBridge 首个正式的 pre-1.0 版本，包含个人文件空间与 Agent 工作台的完整本地体验。

### Included

- Pine 风格深森林绿应用壳层、响应式导航、移动端布局、主题和无障碍交互。
- 我的网盘目录树、面包屑、搜索、筛选、列表/网格视图、拖拽、预览和只读/错误状态。
- 知识库 Markdown 阅读、分屏编辑、页面树、保存冲突恢复和 Pinax owner contract 读取。
- Agent 授权范围、检查、一次显示凭据、禁用、撤销与调用记录。
- 上传、检索、整理、交付流程中心及 mock replica 场景。
- Go CLI/MCP、HTTP/GORM Server 与 React/Vite Web 的统一仓库开发入口。

### Verification

- `make check`：CLI、Server、Web 测试、静态检查和构建通过。
- Web：176 个测试文件、1287 个测试通过。
- 正式 fixture 浏览器路径和 replica 1440px/390px 回读已通过。
- 不运行 race；不连接真实账号、云存储、Pinax vault、付费 Provider 或生产服务。

### Scope

这是 pre-1.0 本地候选版本。真实 OIDC、云存储、生产部署、Electron 三平台实机和签名分发继续单独验收。
