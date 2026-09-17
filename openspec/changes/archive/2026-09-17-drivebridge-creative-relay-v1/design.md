# 本机主动中转与恢复

## 现有入口

internal/remote/remote.go 的 Adapter 已用 os.Root 限制本机目录并通过 DriveBridge SDK 传输；internal/bridge/model.go 管理 Space、Operation 和 state；internal/bridge/handoff.go 与 host_scaena.go 是旧 Scaena 专用路径。配置允许键由 internal/config/config.go 显式列举，不能靠未知键静默启用写能力。

```mermaid
flowchart LR
  Q[目标项目输入请求] --> P[本机 Relay service]
  F[允许空间中的固定来源] --> P
  P --> T[已验证目标 HTTP 会话]
  T --> V[目标原生 complete]
  V --> R[原输入引用及来源回执]
  P --> J[持久传输关联]
  J --> P
```

## 最小实现

建立一个 Relay application service，复用既有 Operation 和来源读取，提供 Start、Status、Resume、Cancel typed 操作。CLI/MCP 仅做参数与安全输出适配；不同 owner 的输入元数据/外层参数转换使用小型显式 adapter，不解析人类 CLI 输出，不构造新通用工作流引擎。

Start 接收 source_ref、绑定的 target_connection、project_ref、用途、目标 import ID 与稳定幂等键；原 input ID 可暂缺。目标 project 和 origin 必须与本机用户预先配置的连接绑定一致，不接受调用者任意提交 URL 或 HTTP header。源身份、版本、摘要、目标身份和会话进入操作指纹；同键异内容拒绝。

本机来源不可由远端 stat 时采用两阶段握手：目标先持久化 awaiting_source 的 import ID；中转器固定文件版本及 name/mime/size/sha256，再以绑定身份调用目标 ProvideImportSource 语义接口；目标校验身份与元数据后创建/复用原 input ID，中转器再私下取得传输方案。相同元数据重放返回同一 input ID，不同元数据冲突；不能要求双方先持有对方尚未创建的会话。目标仅记录本机绑定所证明的来源，不虚称自己远程 stat 过本机文件。

目标输入由创作 owner 的 service 创建和授权；本机适配器通过其 typed HTTP/SDK 取得只用于该输入的上传方案。公开 MCP 只接收安全标识，不接收长期凭据。短期能力由传输进程私下取得且不持久化；恢复时用受限连接重取。目标 API 不支持安全方案获取时明确 unavailable，不能要求模型转述秘密。

本地来源先在允许根内固定内容摘要；网盘引用通过现有 provider/remote 读取固定版本。数据流必须有上限与背压。需要暂存时使用 owner 私有目录和随机文件名，按同一操作恢复/清理，不能写到调用方指定任意路径。摘要变化直接阻断旧操作，不能换内容继续。

三消费方的媒体验证留在原上传服务；中转成功以目标完成回执为准，HTTP PUT 成功只表示字节已送达。Scaena requires_finalize、Eikona unreviewed、Sonora requires_permission 保持原义。

## 恢复与运行边界

持久关联保存原 input ID、source_ref/version/digest、Operation ID、target binding ID 和安全进度，不保存临时 grant。重启先查询目标状态，完成则复用引用；未完成且仍有效时按原通道规则继续。后端不支持分片恢复时允许重传相同内容，但不得创建第二个领域输入。终止或过期输入不能靠 renew 复活。

权限/摘要/媒体/配额拒绝不是自动换通道条件。主动恢复只处理明确可重试的网络失败；并发 lease 保证单操作只有一个写入者。取消停止传输并按目标原合同取消未完成输入，不能删除已完成领域资产。

保留本机 state 单进程占用：持有 state 的当前 MCP/CLI 进程执行任务，进程退出后由下次显式 resume 恢复；不承诺电脑关机期间自动上传。目标项目保存等待中转的输入/回存记录。回存反向文件搬运只接受来源项目已授权的固定交付项，不由中转器选择版本或重新生成。

CLI 配置、启用、inspect/status/resume/cancel 与 MCP schema 在实施任务 1.1/1.4 中经现有 Cobra/config/registry 增量提供；本规格不展示未实现命令作为操作指南。

## 验收命令

以下入口均在本项目现有；实施后必须检查输出确实运行新增场景，当前未执行：

```bash
python3 scripts/check.py -- go test ./internal/bridge ./internal/remote ./internal/config ./cmd/drivebridge -count=1
python3 scripts/check.py -- go test -race ./internal/bridge ./internal/remote ./cmd/drivebridge -count=1
go vet ./...
CGO_ENABLED=0 go build ./cmd/drivebridge
openspec validate drivebridge-creative-relay-v1 --strict --no-interactive
```

使用 httptest 模拟三个 owner 的原输入请求、完成与恢复合同；使用真实子进程中断验证本机状态锁和恢复。CLI 过程测试沿用现有 testscript。集成证据由 scripts/check.py 写入 temp/integration-test-runs/<run-id>/ 六件套，失败脱敏并保持退出码。

tasks.md 的 1.1–1.4 每项运行首条命令并检查相应失败断言，1.5 做进程/race 和旧合同回归，1.6 做最终文档和质量门。仅新增 nullable/default 字段，pure-Go/GORM；禁用新增 relay 不迁移引用或删除旧操作。
