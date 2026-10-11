## 6. 实施与验证

- [x] 6.1 users verify --key：Service.VerifyUserKey 本地哈希查（零网络零副作用）+ CLI 状态报告（live/revoked/expired）+ 服务级矩阵测试
- [x] 6.2 serve --events：net.Listener 先 bind 再发 ready 事件、优雅退出 end、失败 error；--json/--agent/--explain 仍拒；流形态测试

验证证据：`temp/integration-test-runs/20260916T021414Z-5aaf074c/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
