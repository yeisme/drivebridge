# 实施与验收任务

由 scripts/openspec-tasks.py 维护状态。

- [x] F01 更新 DESIGN.md、UI Spec 与清透果味网盘语义 Token 及彩色文件可读类型工具 | evidence: tokens.css + file-display.test.ts passed
- [x] F02 实现默认直达主个人网盘路由、顶部新建上传菜单、7 项精简侧栏与多盘一键切换 | evidence: default-space + DesktopNavigation + WorkspaceApp typecheck passed
- [x] F03 升级设置与空间管理双栏中心及移动端五栏网盘壳层，保留全部 22+ 历史深链兼容 | evidence: PersonalSettings Hub + MobileNavigation tests passed
- [x] F04 重构 Finder 紧凑头部、鼠标框选、底部悬浮多选胶囊栏、全屏拖拽遮罩与右下角传输悬浮窗 | evidence: Finder compact header + marquee + SelectionActionBar + TransferDock passed
- [x] F05 重做沉浸式 Lightbox 预览器详情抽屉、卡片化 AI 投递箱与最近使用页并清理后台术语 | evidence: Lightbox + SubmissionInbox + WorkspaceHome updated
- [x] F06 完成类型门禁、Vitest 单测、六视口深浅色 Playwright E2E 截图与构建验收 | evidence: vitest 201 files (1371 tests), motion policy PASS, check:ui PASS, playwright E2E (temp/integration-test-runs/20261106T145022Z-52997120)
- [x] F07 支持 ?token= 免密直连自动注入脱敏、一次性单次销毁邀请 Token 生命周期与口令智能粘贴 | evidence: key_onetime_test.go + url-token.test.ts + token-url-login.spec.ts (temp/integration-test-runs/20261106T153009Z-51872480)
- [x] M01 接入 motion 物理弹簧引擎与 ReducedMotion 全局桥接，升级 Crystal Tactile Glass 光影材质令牌 | evidence: motion 14.9.1 + AppearanceProvider MotionConfig + Crystal Tactile Glass tokens
- [x] M02 构建微立体折角 FileArtwork、双层彩色 FolderGlyph、双色晶透 NavGlyph 与多图层空态插画 | evidence: FileArtwork.test.tsx + Media.test.ts passed
- [x] M03 实装侧栏与视图切换 layoutId 弹簧滑块、网格卡片微悬浮回弹、多选坞与传输窗弹性入场及晶透登录/预览面 | evidence: check:ui PASS + check_motion_policy.sh PASS
- [x] M04 通过 UI/动效/对比度/全量单测与六视口 E2E 门禁，热更新发布至内网 10.10.1.101:18786 | evidence: vitest 203 files (1375 tests), check:ui PASS, motion policy PASS, playwright E2E (temp/integration-test-runs/20261107T024057Z-3f6d3a77)
