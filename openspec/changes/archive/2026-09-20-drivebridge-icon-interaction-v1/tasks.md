# Tasks: drivebridge-icon-interaction-v1

范围：`client/drivebridge-client`。不变更 backend-server 或 MCP 合同。界面中文，字段名英文。

## 1. 令牌与设计文档

- [x] 1.1 在 `src/style.css` 收口 `:root` 令牌；`finder.css` 不再重写 `--green` 与字体；去掉全局 `filter: brightness` hover。
- [x] 1.2 撰写 `docs/ui/DESIGN.md` 并在 `docs/README.md` 链接；操作页移除英文 eyebrow DOM。

## 2. 产品标

- [x] 2.1 更新 `public/icon.svg`、新增 maskable 资产，重渲 `apple-touch-icon.png`，manifest 声明 maskable。
- [x] 2.2 实现 `<BrandMark />`，替换侧栏与登录页 lucide Files。

## 3. 图标语言

- [x] 3.1 主导航 12 个入口使用互不相同的 lucide 图标。
- [x] 3.2 合并 `FileIcon`/`MediaIcon`，补齐 audio/pdf/text/other。

## 4. Finder 菜单与选择

- [x] 4.1 FinderMenu 支持 icon、separator、快捷键排版；粗指针/窄屏为 action sheet。
- [x] 4.2 为文件/空白菜单补图标与分组；空状态提供上传主按钮。
- [x] 4.3 键盘：ArrowRight 进文件夹，ArrowLeft 回上级；checkbox 细指针不常驻。

## 5. 拖拽、动效与壳层

- [x] 5.1 文件夹与目录树 drop 高亮；`:active` 与 reduced-motion。
- [x] 5.2 身份条瘦身、条件显示空间选择器、移动端主操作溢出、底栏触控尺寸。

## 6. 验证

- [x] 6.1 单测：`finder-keys`、Media 类型图标、菜单渲染。
- [x] 6.2 Playwright：`finder-design`、`workspace-design`、`finder-actions`；证据写入 `temp/integration-test-runs/<run-id>/`。
  - 验证：`pnpm test`（39 通过）；`pnpm exec vite build`；`pnpm exec playwright test tests/finder-design.spec.ts tests/workspace-design.spec.ts tests/finder-actions.spec.ts`。
  - 证据：`temp/integration-test-runs/20260919T094941Z-00a4e356/`（vitest）、`temp/integration-test-runs/20260919T095718Z-d13997d9/`（finder-design）、`temp/integration-test-runs/20260919T095045Z-c22b4153/`（workspace-design + finder-actions）。`pnpm build` 的 `tsc --noEmit` 被并行未跟踪文件 `tests/key-disable.spec.ts` 语法错误挡住，非本切片引入。

## 7. 手感续作

- [x] 7.1 空目录作为上传落点；有文件时才显示拖入条；加载用静态骨架而非闪烁。
- [x] 7.2 字母定位当前目录项目；焦点行滚入视口。
- [x] 7.3 网盘卡片悬停、总览指标可点、筛选下拉不再裁切「按名称」。
