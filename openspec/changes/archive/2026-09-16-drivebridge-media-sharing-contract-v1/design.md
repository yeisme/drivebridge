# 增量合同与恢复

新增 Directory、Preview、Share/ShareItem、CreativeConnection/Receipt/Asset 等 GORM 状态；已有字段和表不删除。原下载路径继续作为 attachment，新 media 路径强制 version 参数。公开分享每次访问校验授权，固定集合与实时范围分别处理。

缓存为可重建派生产物，服务端缓存加密。转换进程使用明确输入、资源限制与私有工作目录；Office/RAW/PDF 的解析在 OS 沙箱内完成。生成提示词仅转发给领域 owner，记录保存摘要、稳定输入与任务引用。

文件删除前查询活动分享、handoff 和生成交接。回存与生成分离，禁止以重新生成恢复传输。旧二进制可读取原文件与 revoked 标记；回滚不删除新增状态和实例密钥。消费者为 DriveBridge Web 与本机 MCP owner。

验证依据为本项目 evidence runner 的真实进程／HTTP／GORM 测试；mock owner 只证明本地合同与恢复，不证明实际 provider、账户配置或费用结算。详细接口与限制见 docs/media-sharing.md。
