# DriveBridge Electron 宿主

同仓模块，遵循 `../AGENTS.md`；不建第二Git或独立OpenSpec。main/preload使用TypeScript、Electron/Forge；renderer只加载 `../web` 的正式构建，不另建页面。文件、guard、阅读状态归Go owner，Pinax正文和身份归Pinax。

- sandbox/contextIsolation/webSecurity启用，Node/webview关闭；拒绝任意shell/fs/fetch IPC、子frame请求、远程导航、自动权限与窗口。
- token/root只允许main私有内存/管道，不进renderer、storage、日志或截图。新增方法逐个校验sender和参数，未实现不宣称available。
- 使用pnpm；配置由 `scripts/configure.mjs` 生成，不能手改生成配置。版本升级先核对官方发行与安全文档。
- 先定向 `pnpm --dir desktop test`、`pnpm --dir desktop typecheck`；根目录 `python3 scripts/check.py -- pnpm --dir desktop ...` 留六件套。
- `pnpm --dir desktop build`复用Web构建并打包main/preload；不证明三平台运行。
- 禁止race、个人vault/真实云账号、自动提交/推送/发布、签名公证、关闭sandbox。实机门独立保留，headless不替代。
- 本模块现阶段是FRONTEND宿主切片；Go真实集成另登记INTEGRATION。保持现有dirty改动。
