# 创作素材本机中转

目标是在 Scaena、Eikona、Sonora 当前项目内引用 DriveBridge 素材：可达服务直接交接，只有个人电脑能读来源时由本机适配器主动上传。2026-09-16 已实现三个 owner 的本机中转（Eikona 图片、Scaena 图片/视频、Sonora 音频）并通过本地 fixture、进程中断恢复与 race 验证；跨机器真机验收、发布和部署保持独立。

## 现有能力不变

既有 upload/download、固定文件引用与操作恢复继续使用。[个人文件闭环](personal-agent-loop.md)介绍普通传输；已有 prepare_reference/save_output 仍是其原先的 Scaena 专用能力，不自动升级为三个 owner 通用导入和交付分组回存。

本机引用绑定远端版本，原文件被替换后旧引用可被拒绝，并不保证保留历史字节。远程受管版本引用只有在内容仍保留且权限有效时可读。目标项目成功导入固定副本后，其副本不随来源文件更新或删除。

## 中转流程与当前边界

1. 用户或 Agent 在目标创作项目提供文件引用；目标项目建立自己的输入请求并声明媒体和用途限制。
2. 本机适配器核对来源空间/允许根、固定版本与摘要，检查可信目标实例以及该项目允许的传输权限。
3. 使用目标输入会话的受限能力主动发送字节。仅需本机向外连接，不开放电脑公网 listener，不把本机路径提交给远端读取。
4. 目标项目执行原生媒体校验，完成后返回自己的输入/资产引用和来源关系。适配器只记录传输关联，不替代领域采纳或执行生成。
5. 网络中断或结果未知时先查询原输入与原传输；已完成就复用原结果，未完成则按实际通道能力续传或重传同一内容。不能承诺所有 rclone 后端都能断点续传。

本地[实施设计](../openspec/changes/drivebridge-creative-relay-v1/design.md)和[任务](../openspec/changes/drivebridge-creative-relay-v1/tasks.md)已建立，明确中转操作、持久关联和验收入口。中转复用现有 Operation、来源空间和恢复入口，不新增调度系统；owner 差异只体现在小型适配表（完成态、引用前缀、媒体预检），领域校验和采纳仍归各 owner 原输入服务：Scaena requires_finalize、Eikona unreviewed、Sonora requires_permission 保持原义。

## 无本机 CLI 的能力判断

客户端无需安装 Eikona、Sonora 或 Scaena CLI。先通过已连接 MCP 的 tools/list、inputSchema 和可读能力资源确认实际工具名称、限制与恢复规则。

- 已安装且可读取允许目录的 DriveBridge 本机适配器：新增中转能力实现并启用后，才可按上述流程执行。
- DriveBridge Server 与目标 owner 网络可达：目标 owner 可采用服务间字节交接，不要求经过用户电脑。
- 仅有远程 MCP 调用，不能读取本机文件或执行 HTTP：明确报告能力缺口，使用已启用的一次性上传页或先向可达存储上传。不能把这种情况描述成中转已成功，也不把安装所有产品 CLI 当作唯一解法。

本机路径只在固定允许根内解析。传输端点必须匹配目标实例和原会话，不跟随跨源重定向；不向对象存储或外部地址附带长期 owner 凭据。传输描述符只在必要的执行层使用，不进入普通回执、日志、提示词或清单。

## 回存与恢复职责

创作项目依据预先配置的目的目录，在选定交付版本后发起独立回存。DriveBridge 搬运文件并验证摘要；来源项目定义预期文件集合和最小来源清单。普通上传成功不等于整个交付分组保存成功。

回存失败不阻塞原交付、不重新生成；重复触发复用原分组，缺项补传。电脑离线时保留可恢复状态，由原项目继续呈现尚未完整回存。不能让一次内存通知成为唯一自动回存记录。

取消回存不会删除创作成品或已经保存的文件。清理与保留沿用 owner 规则；待回存源文件不可用时应明确阻塞，不伪造成功或重新生成替代品。

## 验收与兼容

沿用现有测试 runner，证据写入本项目 temp/integration-test-runs/<run-id>/。新增验证必须覆盖同机、跨机器、电脑断网/进程退出、完成响应丢失、过期传输能力、摘要变化、目录越界和无 CLI 客户端。实施设计已绑定现有可运行命令，本轮新增原输入复用、短期 grant 隔离及重复传输测试；全包 race 证据为 `temp/integration-test-runs/20260916T125223Z-5b2519fb/`，不代表真实跨机器部署。

保留旧本机和远程 URI、原 Scaena socket 合同及原传输命令；新增入口默认关闭，关闭后保留已完成副本与恢复记录。本机 state 单进程占用约束继续适用，不在本轮暗中引入新 daemon 或通用调度系统。

跨项目背景见根工作区 [设计](../../../openspec/changes/drivebridge-creative-resource-reuse-v1/design.md)、[场景矩阵](../../../openspec/changes/drivebridge-creative-resource-reuse-v1/scenarios.md) 和 [owner handoff](../../../openspec/changes/drivebridge-creative-resource-reuse-v1/handoffs.md)；独立 checkout 的实施和验收只以本项目本地 OpenSpec 为任务真源。

## 当前使用入口（Eikona/Scaena/Sonora）

先在目标 owner 上为本机适配器认证身份配置 local-relay connection，再在用户电脑注册可信目标（--owner 区分 eikona、scaena、sonora，默认 eikona）：

```bash
drivebridge relay target-set images --space LOCAL_SPACE --project project-a \
  --connection laptop --url https://images.example.com --key-file /absolute/private/input.key --owner eikona
drivebridge relay target-set shots --space LOCAL_SPACE --project project-b \
  --connection laptop --url https://shots.example.com --key-file /absolute/private/input.key --owner scaena
drivebridge relay target-set tracks --space LOCAL_SPACE --project project-c \
  --connection laptop --url https://music.example.com --key-file /absolute/private/input.key --owner sonora
drivebridge relay start 'drivebridge://file/FILE_REF' --target images --idempotency-key input-one --json
drivebridge operation OPERATION_ID --json
drivebridge resume OPERATION_ID --json
drivebridge relay cancel OPERATION_ID --json
drivebridge relay target-disable images --json
```

MCP 对应 `relay_start`、`relay_cancel`，查询仍用 `operation_get`。客户端无需任何创作 CLI；仅有远程 MCP 且无本机读取能力时继续使用一次性上传页。配置仅由本机 CLI/application service 修改，MCP 不接收任意目标 URL/header。旧单进程 state 锁约束保留。

本轮补充：三 owner（Eikona 图片、Scaena 视频、Sonora 音频）各自独立 origin 的原输入完成与同键重放不重传；过期/终止输入恢复显式报 `relay_input_terminal` 且不重建输入；来源内容变化恢复报 `source_changed`；真实子进程中断（kill 本机 relay 进程）后在新进程 resume 复用原 import 不重复创建；generic rclone 后端来源走同一合同。丢失完成响应后重开本机数据库的恢复验证仍复用原 input，未重传：`temp/integration-test-runs/20260916T134736Z-f1ea3190/`。真实本机二进制与 Eikona 服务组合由 Eikona system 场景验证，使用临时 rclone alias/local 后端。较广检查 `20260916T134130Z-cb8876cf` 的 config 包因并行 aigora_url 模板缺项失败；bridge/remote/CLI 包通过，整条检查不算通过。

## MCP 先选择已配置目标

新增只读 `relay_targets` 与 capabilities 中的 relay 区块：返回目标 ID、项目、源空间、connection_id、disabled。端点与 key-file 不返回，network_readiness 固定为 not_probed。配置存在不代表目标已启用或远端授权已验证。

顺序为 `relay_targets` → `stat` → `relay_start` → `operation_get`。断线先查询原 operation；失败后恢复原操作，取消用 `relay_cancel`。不要把 Server 的 binding_id 当作本机 target_id，也不要把 owner import_id 当作本机 operation_id。capabilities 的 relay 区块另返回 `owners` 列表（当前启用目标覆盖的 owner），仍不含端点或密钥。

CLI 配置命令会提前指出缺少的必要选项，避免先读取凭据再在网络端返回含糊错误。现有 `drivebridge relay targets --json` 的数组形状保持不变；MCP 新增自己的 advisory projection，不修改旧消费者输出。
