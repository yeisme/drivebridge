# 统一 MCP 文件交接与引导式恢复

## 目标

通过 MCP Gateway 统一管理 DriveBridge、Scaena、Eikona 和 Sonora。DriveBridge 允许多个用户共用一个实例和对象存储，但普通用户不能浏览全局文件；文件通过稳定引用和任务绑定授权交接给 owner。

## 交互原则

- 不可用能力返回可执行引导，不把所有配置问题都表现为拒绝。
- 只有安全、权限和数据完整性风险阻断。
- Agent 只对 `retryable=true` 的错误使用原幂等键重试。
- 没有上传回执时重新选择文件并创建新上传，禁止全局搜索。

## 非目标

- 不新增 Gateway 文件索引。
- 不接受任意公网 URL 作为输入。
- 不把对象存储 signed URL 暴露给 MCP 客户端。
- 不把 DriveBridge 业务状态复制到 Scaena、Eikona 或 Sonora。
