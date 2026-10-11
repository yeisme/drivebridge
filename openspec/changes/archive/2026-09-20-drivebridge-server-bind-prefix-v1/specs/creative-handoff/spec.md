## ADDED Requirements

### Requirement: 内容读取与 GetHandoff 共用当前前缀检查

系统 SHALL 在 `OpenHandoff` 与 `readableHandoff` 使用与 `GetHandoff` 相同的当前文件 path/space 对照绑定 prefix/space。SHALL NOT 将越界码从 `forbidden` 重命名。SHALL 继续接受 purpose `reference`。

#### Scenario: OpenHandoff 拒绝已移出前缀的文件
- **WHEN** 绑定前缀内取得 grant 后文件被移出该前缀
- **THEN** `OpenHandoff` 返回 `forbidden`，不返回跨范围字节

#### Scenario: reference purpose 仍然有效
- **WHEN** owner 以 purpose `reference` 创建 handoff-binding
- **THEN** 绑定成功，既有 Eikona 接收路径可继续使用该 purpose
