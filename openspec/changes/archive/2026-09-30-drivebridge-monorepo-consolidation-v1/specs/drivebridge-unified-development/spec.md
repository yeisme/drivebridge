## ADDED Requirements

### Requirement: 单仓与保全
项目 MUST 将 CLI/MCP、Server、Web 合并到一个 Git owner，并保全未提交工作与原历史。

#### Scenario: 三个目录有未提交文件
- **WHEN** 执行迁移
- **THEN** 文件字节、原索引和 Git 历史均保留，迁移摘要可复核，不创建嵌套 Git owner。

### Requirement: 统一开发入口
项目 MUST 提供单入口启动、构建和测试，内部保留原公开合同。

#### Scenario: 开发者运行 make dev
- **WHEN** 指定 loopback 端口空闲
- **THEN** 启动 disposable Server fixture 与 Web，停止时仅清理自己启动的进程。

#### Scenario: 旧消费者使用 SDK
- **WHEN** 导入既有 Go module 或调用旧 HTTP/MCP
- **THEN** import 名称和 wire 合同保持不变，仓库内路径可独立构建。

### Requirement: 统一规格与证据
项目 MUST 统一 active OpenSpec 和文档入口，保留旧任务状态并区分迁移与功能实现。

#### Scenario: 合并规格后查看任务
- **WHEN** 运行 openspec list
- **THEN** 所有来源的 active change 可见，前端重建任务仍未执行，历史证据不因迁移被伪造更新。
