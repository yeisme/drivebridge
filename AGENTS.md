# DriveBridge

独立 Go MCP owner，负责 OneDrive 空间、文件引用、受控传输和短期交接。生成准入、Provider 凭据和任务状态属于创作 owner。接口为 pre-1.0 增量合同。

- 采用 Go、固定版本 rclone、MCP SDK；持久化使用 GORM 和 pure-Go SQLite，支持 `CGO_ENABLED=0`。
- 所有本机路径限制在配置的 roots；远端访问限制在配置的 drive/root。配置和回执只通过 CLI/application service 写入。
- 不在 MCP 文本、日志、回执或证据中输出 token、临时 URL、原始请求。禁止公共分享、永久删除（软删除+恢复仅限空间内）、默认覆盖、自动付费重试和真实账号操作。
- 新入口默认关闭。独立本地 Git 仓库；远程创建、发布及部署单独授权。
- 使用 `go test ./...`、`go vet ./...`、`CGO_ENABLED=0 go build ./cmd/drivebridge`；集成验证使用 `python3 scripts/check.py`，证据位于 `temp/integration-test-runs/`。
- 使用 owner OpenSpec 记录实现和验收；普通文档中文，命令与错误英文。根工作区中其他项目的改动必须保留。
