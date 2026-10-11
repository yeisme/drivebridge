# 控件与资料设置验收

本轮统一共享 Button、Input、Textarea、Checkbox、Select/Combobox、Dialog/AlertDialog、Popover 和 DropdownMenu。旧按钮、原生 select 视觉层、原生 dialog 样式、directory-table 与 transfer-card 规则已移除。UI 检查使用 TypeScript AST，基础组件与测试替身外不允许重新引入旧控件。

用户、Key、团队成员/权限、资源态势、传输与分享列表复用 TanStack v9 DataTable；支持当前结果搜索、排序、列设置、稳定 ID 选择、分页、窄屏卡片。分享支持确认后批量撤销；失败保留未完成项。Finder 保留文件管理专有的目录树、拖拽、范围选择和视图交互，其控件已统一。

个人设置资料真实保存到服务端，头像必须为本人个人网盘的固定图片版本；外观偏好仍只保存在浏览器。后端为增量字段与新增路径，旧 id/name、文件、团队和 Key 合同不变。两个 owner 的 OpenSpec 分别为 drivebridge-ui-polish-v2 与 drivebridge-profile-v1。

## 验证证据

所有路径相对本项目根目录。

| 检查 | 结果 | 证据 |
| --- | --- | --- |
| 前端单测 | 98 项通过 | temp/integration-test-runs/20260921T072125Z-21fa8e44/summary.json |
| 资料跨会话与头像完整流程 | 2 项通过 | temp/integration-test-runs/20260921T072707Z-afa842f1/summary.json |
| 表格键盘、列设置、排序与选择 | 1 项通过 | temp/integration-test-runs/20260921T072414Z-205a85f7/summary.json |
| 默认 fixture E2E | 29 项通过；14 项需其他 fixture 模式 | temp/integration-test-runs/20260921T072127Z-1071da64/summary.json |
| 用户向导、团队与布局 | 6 项通过 | temp/integration-test-runs/20260921T070947Z-97b1b2f3/summary.json |
| 用户目录生命周期 | 1 项通过 | temp/integration-test-runs/20260921T071415Z-856cd2db/summary.json |
| 管理员职责交接 | 1 项通过 | temp/integration-test-runs/20260921T071456Z-6094457d/summary.json |
| 团队 Key 与身份切换 | 1 项通过 | temp/integration-test-runs/20260921T071456Z-c3017c0f/summary.json |
| 工作区访问范围 | 1 项通过 | temp/integration-test-runs/20260921T072559Z-1b769b5a/summary.json |
| 独立治理 fixture | 3 项通过 | temp/integration-test-runs/20260921T072304Z-044b8ba2/summary.json |
| 本地体验 fixture | 1 项通过 | temp/integration-test-runs/20260921T072304Z-2799902a/summary.json |
| localhost:11070 浏览器检查 | 八路由、深浅色、1440/768/390、1 GiB 与团队 | temp/integration-test-runs/20260921T071414Z-421984ef/summary.json |

后端完整 race 检查证据位于 cli/drivebridge/server/temp/integration-test-runs/20260921T065330Z-a9a16c0d/summary.json。头像测试扩充后的定向证据为该 owner 的 temp/integration-test-runs/20260921T070425Z-e7c27d8d/summary.json。Go vet 与 CGO_ENABLED=0 构建通过。

## 复验

```bash
pnpm typecheck
pnpm test
pnpm build
pnpm test:e2e
DRIVEBRIDGE_TEST_TEAM_DEMO=1 pnpm exec playwright test tests/teams.spec.ts tests/creation-wizard.spec.ts tests/teams-layout.spec.ts tests/direct-actions.spec.ts
DRIVEBRIDGE_TEST_TEAM_DEMO=1 pnpm exec playwright test tests/users-management.spec.ts
pnpm exec playwright test --config playwright.governance.config.ts
```

涉及生命周期变更的测试应使用独立临时 fixture，避免一个测试留下的管理员/数据负责人状态影响另一个测试。默认 E2E 的 skipped 不视作已验收，以上独立模式分别补充证据。

开发预览为临时本地 fixture，关闭服务即清除其数据，不是生产部署。构建仍提示主共享包约 757 kB（压缩前）；本次通过分页控制列表渲染，未宣称完成整个应用的路由代码拆分。
