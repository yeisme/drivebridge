## 2. 实施与验证

- [x] 2.1 HTTP 错误信封 message 注册表 + 8 码状态映射修正；writeError 表驱动测试
- [x] 2.2 拆 user_import_failed（user_key_duplicate/database_unavailable）+ runExport 改 transfer_failed
- [x] 2.3 MCP schema 真实化（trash/restore 声明 ref、insight 工具逐字段描述、防漂移测试）——与并行 lane 同文件，2026-09-16 已随 f276b55 落地
- [x] 2.4 SDK typed Error{Code,Message} 裸码兼容 + client_test 三用例
- [x] 2.5 CLI：emitFailure 真码信封、audit 人读行输出、platform-connect 单信封（over_scoped→partial）、doctor 未就绪退出码 2
- [x] 2.6 doctor login_identities 预检（零可登录 key 无 issuer→fail 带恢复命令；Go 侧宽容过期判定）

验证证据：`temp/integration-test-runs/20260915T150629Z-aefe32a9/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
