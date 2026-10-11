# DriveBridge 语义视频生产交接

本域设计与任务真源：[drivebridge-semantic-video-handoff-v1](../openspec/changes/drivebridge-semantic-video-handoff-v1/proposal.md)。

补齐素材进入制作与成片回存的版本、摘要、受控传输和回执说明。引用不是远端可读路径；上传、生成和采用分别记录。版本漂移后重新准备，unknown 恢复原传输。

复用：稳定文件引用、prepare_reference、save_output 与原 transfer 恢复。交付：file ref/version/digest、transfer identity、permission/source facts 与 handoff receipt。

不复制生成任务或成片接受，不泄漏临时访问地址、宿主路径或凭据。

## 消费与验收

调用方先发现已执行能力，再绑定固定来源；结果以原任务身份查询。来源改变产生新的修订和影响说明；不能以同名文件、同一句台词或一次成功请求替代固定来源与接受。

检查命令：`go test ./internal/bridge/... -run 'Handoff|Version|Resume' -count=1`、`openspec validate drivebridge-semantic-video-handoff-v1 --strict`。工程验收、真实效果和下游采用分别记录；运行证据位于本仓 temp/integration-test-runs。

## 当前接入与恢复矩阵

唯一 owner 为本仓，server/web 是仓内模块。制作端先发现本会话可执行 action 与 inputSchema，再准备固定文件引用；没有本地 CLI 的客户端也从 MCP discovery 获取参数，不把自己的文件路径传给远端当可读路径。实际字节上传/下载走既有受控数据通道，MCP 投影不返回私有临时 URL。

| 场景 | 保留信息与恢复 |
| --- | --- |
| 参考文件进入制作 | 文件引用、版本、摘要和用途；prepare 不等于生成或采用 |
| 成片回存 | 已完成产物的固定版本及 save_output/transfer 回执；回存不等于正式成片接受 |
| 上传成功但回执丢失 | 用原 operation ID 核对并恢复；同请求同键返回原 operation，不能重复创建远端文件 |
| 文件版本变化、跨空间或路径越界 | 既有 CAS/空间边界拒绝，重新准备当前版本 |
| 取消与重启 | 取消结果持久化，终态不恢复上传；未完成任务沿原身份处理 |
| 本地目标已存在 | 拒绝覆盖，保留原文件 |

2026-10-01 经 `python3 scripts/check.py -- go test ./internal/bridge -run 'Handoff|Version|Resume|CancelIsDurable|UploadResume' -count=1` 验证通过；证据：`temp/integration-test-runs/20261001T092354Z-29fef154/`。使用本地受控传输替身，未修改真实 OneDrive、凭据或线上服务。

## 下一阶段：制作文件交接

[详细设计](../openspec/changes/drivebridge-semantic-video-handoff-v1/design.md) 和 4.x 任务补充固定文件版本、受控字节、grant 过期/撤权、取消晚到及回存核对。版本/ETag 与实际 SHA-256 分开报告；只核对 metadata 时不宣称已验真实字节。

prepare_reference 只做准备，save_output 只保存既有成功/导入产物；生成和正式成片接受仍由 Scaena 持有。断连后查询原 operation，不能换名重复上传。

复用当前单仓的固定交付和上传恢复实现及任务；不另建传输调度器、不恢复旧仓入口。新增 conformance 是待实施工作，真实 OneDrive 与规模验收不由本地替身结果替代。
