# Agent 文件闭环增量合同

HTTP、SDK、MCP 增量接入 caller/task_ref 两个可选短标识符。两者仅为调用方声明，不替代用户、Agent、空间或目录权限；不接受 URL、正文和路径。字段随上传操作、版本、当前文件保存；旧版本保留自己的来源。省略字段时旧请求、幂等指纹及响应字段保持兼容。

新增 `GET /api/v1/references?ref=...` 校验本实例固定版本引用，返回该版本文件、引用和摘要；复用 read 权限，拒绝跨实例、跨用户及不存在的版本。原生 MCP stat 的 ref 路径同样固定版本，不升级到当前版本。回收站引用允许查询，下载仍拒绝。

`GET /api/v1/files` 新增 mime_type 精确过滤和 RFC3339 after（文件更新时间）；q/prefix 沿用既有路径搜索。原生 MCP 新增 search/trash/restore，读写权限由既有服务执行。原生 MCP create_upload 的来源字段为可选；字节仍通过 HTTP，不在 MCP 中放 base64 或客户端路径。

`GET /api/v1/operations` 可按 space_id、pending 和 limit（1–200）过滤。先应用 owner 和 Agent 权限再返回；不会因客户端过滤而扩大访问。上传结果、恢复记录及回收站入口保留原合同。

这不是新的任务平台，不保存领域审核或收费状态。加密空间仍不能通过 prepare_reference 宣称可直接交给 Provider；其字节通过授权解密下载路径取得。

## 验证与回滚

```bash
python3 scripts/check.py
python3 scripts/check.py -- go vet ./...
python3 scripts/check.py -- env CGO_ENABLED=0 go build ./cmd/drivebridge-server
```

新测试覆盖当前与历史来源、固定版本解析、跨用户拒绝、来源 URL 拒绝以及授权删除恢复；原有身份、配额、恢复、加密和备份测试继续执行。完整客户端闭环由 cli/drivebridge 的 remote_system.py 启动本服务 fixture 验证。

回滚到旧程序会忽略新增可选列；保留数据库与密钥，不删列、不改引用或实例身份。新增客户端入口依赖此服务版本，旧服务未提供接口时明确失败，不静默改用最新版。没有发布、真实账号或部署验收。
