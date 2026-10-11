# Design: drivebridge-error-contract-v1

## 决策

1. **message 静态注册表**（error_messages.go）：码→固定英文句，永不插值请求/错误文本（脱敏红线）；未注册码兜底通用句。writeError 信封 `{"error":{"code","message"}}`。
2. **状态码只加不改语义**：8 个码归位（401/404/409/429/503），码字符串零变更——纯映射修正，无断代。
3. **过载码拆而非加**：user_import_failed 从未文档化且两义并存，直接替换为 user_key_duplicate + database_unavailable（CLI/MCP 透传，无消费方依赖旧码）；runExport 复用 storage_read_failed 属误标，改 transfer_failed 而非新造码。
4. **SDK typed error 双轨**：`Error()` 字节兼容裸码（既有字符串比较不破）；`Message` 字段透传服务端静态句，缺席为空串。
5. **schema 漂移治理**：防漂移测试声明"CallTool 接受键 ⊆ schema 声明"（trash/restore 的 ref）+ "insight 工具声明字段=CallTool 实读 11 键且均有描述"；同文件在途改动（input.bind_reference lane）随其落地。
6. **platform-connect 单信封**：隔离结果并入 `data.storage_isolation`，over_scoped→status=partial（契约枚举既有值）；原双对象输出违反 --json 单对象规则，属合规修复。
7. **退出码分层**：0 成功、1 失败、2 doctor not-ready（投影已完整但部署未就绪）；消费方按非零判断不受影响。
8. **doctor 只读纪律**：login_identities 经 mode=ro 快照统计 user_keys，过期判定在 Go 侧宽容解析（driver 时间格式差异），零 key 无 issuer→fail 带恢复命令，有 issuer→info。
