# 用量统计与安全审计

本版本以资源、传输、Agent 使用、安全审计四个视图帮助用户解释文件使用情况。数据保存在各 owner 的数据库中，本机与远程分别查询，不合并为一个总量。所有统计从采集开始生效，不补造旧数据。

## 指标与边界

托管逻辑容量区分当前文件、历史版本、回收站和上传预留；目录观测容量来自明确授权范围内的扫描。二者不能相加。重叠根目录或多个范围不输出误导性的观测总计。本机索引不是全盘清单，本机历史版本容量和配额未知时返回空值；单文件上传上限不是空间配额。

有效传输字节按成功操作去重。观测字节记录应用可见的读写边界，区分方向、用途和验证流量；SDK 内部重试、签名直链下载和云商计费不可完整观测。因此观测流量不是网络账单，也不能与有效流量相加。caller 是调用者声明的标签；远程 Agent 身份来自认证，本机标签不冒充独立身份。

图表使用 UTC，默认最近 30 天，查询窗口最长 366 天；缺失数据保留为空，不能显示为零。明细和审计事件保留 90 天，日汇总保留一年；保留最近完整目录快照。按操作和资源过滤的日汇总仍可能有较高基数，当前适用于个人实例，未宣称完成大租户负载验收。

## 扫描与告警

扫描默认关闭。用户明确开启后，owner 进程运行期间执行每日调度，也可手动扫描。默认预算 100000 个条目、600 秒，同一实例同时最多一个扫描。取消、超时、失败或条目超限会保留不完整状态，不覆盖最近完整结果。S3 分页检查点支持满足条件的续扫；普通目录扫描不承诺跨运行续扫。远程仅列举当前实例、用户、空间的对象前缀，不扫描共享 bucket 全量，也不统计云端隐藏版本；列表不是原子快照。

已知配额达到 80% / 95%、完整扫描超过 48 小时未更新、可比完整扫描增长同时超过 20% 和 1 GiB 时提示。还展示采集缺口、审计不可用和不完整扫描。告警进入页面与 CLI，不发送聊天或桌面通知。

## 审计与权限

事件保存身份、动作、结果、资源/版本/操作 ID、关联 ID、入口和时间，不保存文件名、路径、凭据、原始工具参数或文件内容。界面按当前权限解析资源名称；失权后不能靠历史审计绕过授权。普通用户看自己的事件，Agent 查询限制在自己的身份与当前允许范围；匿名认证失败只供实例 operator 查看。

关键授权、删除和覆盖需要先成功写入审计意图；失败时阻止操作。普通读取可以降级并显示缺口。紧急撤销不因审计故障被阻止，但权限数据库失败时必须报告失败。意图与结果是不同事件，事件数量不等于请求数量。审计是本地可运营记录，不是不可篡改证明。

查询不会递归制造 usage/audit 事件。导出返回 audit_recorded，调用者必须保留该字段，不能把审计写入失败的导出描述为已留痕。分页明细默认有上限，应依据 next_cursor 继续读取；网页审计导出明确为当前页，汇总导出保留筛选和口径。

## MCP 无 CLI 客户端

连接后先调用 tools/list 读取 inputSchema。usage_summary、usage_series、usage_breakdown、usage_scans、audit_events 为查询；usage_scan 只能执行 owner 已开启的预算扫描，不能修改策略。远程扫描要求整个空间的 read 权限，目录受限 Agent 被拒绝。无需在 MCP 客户端安装 CLI；配置必须在数据 owner 所在机器完成。MCP 不传文件 base64，也不把客户端路径当作远程文件路径。

## 本机操作

使用 `drivebridge usage summary --json`、`drivebridge usage series --json`、`drivebridge usage breakdown --json`、`drivebridge usage events --json`、`drivebridge usage scans --json` 和 `drivebridge audit list --json` 读取当前配置对应的数据。本机与远程模式分别执行，输出不混合。

在 owner 机器用 `drivebridge usage scan --space SPACE_ID --enable --configure-only` 开启扫描，用 `drivebridge usage scan --space SPACE_ID` 手动执行，用 `--cancel-id SCAN_ID` 取消；`--disable --configure-only` 停止后续调度。用 `drivebridge mcp --enabled --insights` 启动带统计调度和私有控制 socket 的 owner。

`drivebridge view` 使用发行包附带的 web 资产；开发时用 `drivebridge view --web-dir ../../client/drivebridge-client/dist`。视图仅监听 127.0.0.1，一次性启动链接保存在 0600 文件，默认打开浏览器，敏感链接不打印到 stdout。可使用 `--no-open` 和 `--launch-file` 指定私有交接文件。会话通过一次性 fragment 换取 HttpOnly/SameSite cookie，fragment 随即移除；校验 Host、Origin、CSRF。文件操作只读，扫描和导出通过 owner 的 0600 Unix socket 执行。

`drivebridge usage export --json` 和 `drivebridge audit export --json` 输出带 audit_recorded 的结果。普通查询使用只读数据库连接，不夺取正在运行的 owner 锁；离线扫描遵守单进程状态锁。发行脚本同时构建并打包前端资产，保留 CLI 无图形环境路径。

## 本地验收记录

- Go 全量 race，包含完成状态/容量时序修复：[证据](../temp/integration-test-runs/20260912T182329Z-50e00ebe/summary.json)。
- go vet：[证据](../temp/integration-test-runs/20260912T182330Z-58063850/summary.json)。
- CLI/远程 fixture 系统回归：[证据](../temp/integration-test-runs/20260912T182420Z-7beca3c5/summary.json)。
- 四平台纯 Go 构建及前端资产：[证据](../temp/integration-test-runs/20260912T182330Z-1a3be890/summary.json)。
- 发行包校验及本机二进制运行：[证据](../temp/integration-test-runs/20260912T182435Z-97574729/summary.json)。

本轮未提交、推送或部署，未访问真实云账号。失败运行保留在同一证据目录；跨平台包除当前 Linux 架构外仅做编译与内容校验，不代表已在每个平台运行。
