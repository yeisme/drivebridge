## Context

实现固定版本 rclone、Graph、GORM 状态和 stdio 工具，所有写入口默认关闭。

## Goals / Non-Goals

本地完整闭环、稳定文件引用、敏感 URL 不进入 MCP 文本、幂等恢复。无 Ordo、TOS、独立页面、长期公开分享、自动付费重试或部署。

## Decisions

使用固定版本 rclone 作为 OAuth 刷新唯一权威；Graph 从同一配置借用 access token。工具注册表生成 schema。状态和 space 只通过 CLI/service 修改。

引用固定 space、drive、item 和 version。上传名称带 operation ID，完成后计算远端内容 SHA-256。Graph ETag 仅用于版本检查。未知下载有效期保持未知。OneDrive global 云首版支持；其他云区域明确拒绝。

MCP prepare_reference 只检查就绪状态；实际 URL 只在 Scaena 通过原准入后经本机 socket 获取。socket 默认关闭且权限 0600。MCP Gateway 无需搬运 URL，但 Scaena 必须能够访问受限本机 socket；未部署 Gateway 不标记验收通过。

回存只读取 succeeded/imported 原任务及其 artifact grant。下载摘要验证后复用同一个上传 operation；任何回存失败都不能创建生成任务。临时文件保留用于恢复，不自动删除用户成品。

## Risks / Trade-offs

远端既有素材需要一次受控下载进行内容校验。rclone 跨进程不能承诺分片续传，因此恢复允许整文件重传。远端上传使用不可预测的 operation 名称及既有对象检查，不替换指定用户文件。

本地下载成功不能证明 Provider 区域可达；链接可能在排队时过期。真实账号、渠道及排队 canary 属运行时验收，独立于本地实现。当前不支持本机 socket 跨远程机器访问。

## Migration / Rollback

全部增量、pre-1.0、默认关闭。关闭新入口即可回滚，保留文件、state 与任务回执；不部署、不创建远程仓库、不移动旧资产。

## Verification

使用 owner 的 Go 测试体系、fake HTTP/Unix socket、真实 stdio、race 和 pure-Go 构建。每次集成证据写入本项目 temp/integration-test-runs/；严禁输出 URL、token 或原始 Provider 请求。
