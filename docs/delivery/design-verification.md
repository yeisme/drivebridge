# 本轮设计核验

本页记录设计交付的实际核验范围，不是产品测试报告。

## 已执行

- 冻结 v39 本地快照完整性：入口与 12 个资源共 13 项，字节长度及 SHA-256 全部匹配既有 manifest。证据：`temp/integration-test-runs/20261001T081500Z-e0668947/`。
- 前端 OpenSpec strict：通过。证据：`temp/integration-test-runs/20261001T082012Z-e94fec4e/`。
- 后端 OpenSpec strict：通过。证据：`temp/integration-test-runs/20261001T082014Z-2d3e4994/`。

- 设计包内容与依赖检查：33 个 Markdown 文件、174 个本地链接、35 条 Requirement、83 条 Scenario、73 个业务任务、164 条依赖边；无缺失引用或依赖环，R01–R15/AC01–AC30 完整，全部业务任务保持未勾选。证据：`temp/integration-test-runs/20261001T082106Z-ef33da3f/artifacts/design-check.json`。
- Mermaid 实际渲染：7 张图全部生成 SVG，包含本轮新增运行拓扑与发布撤权时序。证据：`temp/integration-test-runs/20261001T082152Z-e05c00d5/artifacts/`。
- 定向 `git diff --check` 通过；大量文件为既有未追踪迁仓材料，因此额外用上述内容校验检查链接、围栏、表格和尾部空白，不把 Git 空 diff 当成已覆盖所有文件。

这些检查由既有 `scripts/check.py` 生成六件套。runner 的通用 component 分类不能将文档校验解释为真实业务通过。

## 审计返修与最终复验

第一次完成审计指出遗漏 server 直连 `/mcp` 路径及新增跨产品面场景追踪不足，首次结构通过不能代表这些内容已齐备。现已读取 `server/internal/service/mcp.go` 与 `http.go`，在统一架构中区分 Streamable HTTP 直连、根 stdio 远程 adapter 和本机模式；明确 create_upload/part_path、同范围 HTTP token、received_parts 恢复、complete_upload 与固定版本内容读取。

S2 B12.2 经任务 CLI 补充直连首次验收，S4 B12.1 保留兼容回归。新增 F24 保护本机/generic/sync/relay，F17.3 依赖它。场景文档新增逐项设计→页面/协议流程→任务→具体方法追踪；没有勾完未来业务测试。

最终复验：两份 strict 通过，证据分别为 `temp/integration-test-runs/20261001T082932Z-767e95fa/`、`temp/integration-test-runs/20261001T082934Z-3363b3c7/`；内容检查 33 文件、176 链接、74 未完成任务、167 依赖边，无问题，见 `temp/integration-test-runs/20261001T082936Z-18eae49a/`；7 张图重新实际渲染成功，见 `temp/integration-test-runs/20261001T082936Z-d064f45f/`。上节 73 任务统计是返修前检查，保留历史而不冒充最终数量。

## 第二次审计协议缺口修复

收件与普通上传不能混同。现明确新 grant inbox.submit/labels.suggest 走专用 HTTP，MCP只发现这些HTTP合同；新grant旧create_upload拒绝，旧write普通上传保持。合同 §4.2.1补齐请求/返回/幂等域/lookup/部分上传/complete/正文权限，统一架构、场景追踪、B07/B12.2/B16.1、OpenSpec design与四个Scenario同步。S2最小链不依赖标签实现，完整三动作由B16/F23验收。

再次验证：两份strict通过（`temp/integration-test-runs/20261001T083631Z-fc22a105/`、`temp/integration-test-runs/20261001T083633Z-1be64a43/`）；33文件、177链接、35 Requirement、87 Scenario、74未勾选业务任务、167依赖边无问题（`temp/integration-test-runs/20261001T083634Z-123dc384/`）。本次只补设计，没有实现新HTTP/MCP能力。

## 交付与边界

本轮复用既有 R01–R15、AC01–AC30 和两套同仓任务，新增跨产品面场景、统一运行架构、原型审阅、交接入口；补充技术选型权衡，通过任务 CLI 关联 B12.1/B13.2/F17.2，修正验收表格中断。未勾选任何业务实现任务。

源码只读核对涉及依赖声明、数据库初始化与 MCP 工具注册，不等于运行时调用通过。截图为冻结快照历史产物，本轮只直接视觉阅读并复核资源完整性，不声称重新运行浏览器。异常状态采用静态线框和 UI Spec 定义，未实现新的可运行原型。

未执行业务代码修改、产品测试/构建、数据库迁移、真实 API/云/模型/生产探测、race、子 agent、付费、凭据修改、提交、推送或部署。原生宿主/runtime、真实云与生产目标仍在对应后续切片决定；当前首期 Web 范围及核心技术选型明确。
