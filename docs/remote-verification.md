# 远程模式验证

原有本机模式回归/race：`temp/integration-test-runs/20260911T071552Z-dcce1787/`。

真实 stdio 远程闭环：`temp/integration-test-runs/20260911T073929Z-962e8635/`。测试启动本机文件服务与 OIDC/S3 fixture，再通过真实 stdio MCP 上传、下载、重放、拒绝路径逃逸、验证撤销。没有输出 Agent token 或临时 S3 URL。

vet 与 pure-Go 构建分别见 `temp/integration-test-runs/20260911T071557Z-33c9e026/`、`temp/integration-test-runs/20260911T071558Z-f7fe533e/`。远程 SDK 当前使用相邻 owner 的本地 replace，尚未单独发布。
