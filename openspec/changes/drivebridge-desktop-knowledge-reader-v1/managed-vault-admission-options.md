# Managed vault 准入方案与本地验证

2026-10-24。本文记录 D03 的设计证据，未安装 guard，未允许 binding ready；D03、D08 和真实 Electron 编辑验收仍未完成。

## 当前合同

保护对象是显式登记的 DriveBridge 实例及参与该保护域的 CLI/MCP/HTTP/worker、连接与别名。域内旧 writer 必须在写入前被拒绝，不能通过自动换 state、关闭 UI 或删除导航解除保护。无法证明覆盖的已知连接、旧挂载和多 state writer 必须拒绝绑定。

现有架构同时保留普通外部编辑器，明确不承诺阻止用户自行运行的未登记外部程序。实现不能把已登记的旧 writer 临时归类为“外部程序”来通过验收，也不能未经决定将产品改成普遍禁止普通编辑器的 OS 保护系统。scope 依据仍为本 change 的 `managed-vault-boundary/spec.md` 和 `docs/architecture/electron-desktop.md`。

## 实测结果

`python3 scripts/check.py -- python3 scripts/managed_vault_os_probe.py` 只使用本机已有 Python 镜像，先解析固定 image ID，禁止拉取、禁网络、只读根文件系统，无宿主目录挂载。容器内 `/proof` 是可丢弃 tmpfs，模拟两个 UID；仅容器迁移进程具有 CHOWN/SETUID/SETGID，退出或超时后清理本次容器。不改变宿主权限、系统账号、真实 vault 或服务。

| 检验 | 实测结果 | 对准入的约束 |
| --- | --- | --- |
| 同 UID 另一进程写 0700 vault/0600 文件 | 可以写 | chmod 不区分普通编辑器与同 UID 旧 writer |
| 改属主前已打开的可写句柄 | 改属主后仍可写 | 迁移需要排空已登记 writer 和旧句柄，单纯 chown 不算完成 |
| 改属主后重新打开文件 | 被拒绝 | 独立身份能限制后续 open，但不能证明旧句柄已失效 |
| vault 属于 broker、父目录可由旧用户写 | 父目录用户能重命名 vault | 只保护叶子目录不够，祖先目录也必须受控 |
| 受控祖先 + 独立 UID | 直接写、符号链接写和重命名均拒绝；broker 可写原文件 | 在这个 Linux fixture 中建立了可用的权限隔离条件 |
| 普通用户编辑器访问同一保护根 | 被拒绝 | broker 方案会改变编辑器和 Hermes/CLI 的启动/访问方式 |

证据：`temp/integration-test-runs/20261023T163426Z-124e55e2/`，10 个断言全部通过。镜像 ID 为 `sha256:febd0be41adb897a0ab8f1f1c693d8912669ea60c4940e076e9946b60e210ef0`。这是 Linux 容器 DAC 验证，不是 Linux 实机安装、macOS/Windows ACL、Pinax/Electron 链路或生产 broker 证据。首次 probe 因容器迁移进程无权遍历旧 UID 目录而失败；修正为先取得临时目录所有权再迁移文件，失败记录 `20261023T163319Z-04a8e324/` 保留。

另外，真实 `bridge.New` 的候选布局试验证明：保留原 SQLite 字节，将旧 `state.sqlite` 路径保留为目录后，本 checkout 的旧构造路径返回 `state_unavailable`，未构造 service、未恢复 worker，也未改动候选数据库。已有反例同时通过，证明新增未知表仍不能拒绝旧启动、另一个 state 仍能旁路 registry。

命令：`python3 scripts/check.py -- go test ./internal/bridge -run '^TestManagedVaultLegacy' -count=1 -v`。证据：`temp/integration-test-runs/20261023T163702Z-2ae6d3c4/`，3 项通过。当前生产代码既不安装也不识别此候选布局；没有证明所有历史 release，也没有证明跨 state 保护。

## 优先验证的方案：受控 state 兼容栅栏

下一工程片优先验证与当前产品边界相容的 state 格式升级。保留普通编辑器直接访问原 vault 的体验，使用原 process lock 排空参与者，在旧二进制必经的存储打开路径建立可验证的拒绝条件，新版本使用明确的受管布局。不得仅新增 version 字段或新的 registry 表就宣称旧版已拒绝。

该方案必须满足以下条件才能进入产品入口：

- 同一 state 的全部实际入口持有同一 canonical process lock。现有 CLI `openService` 和 Server `New` 均使用 `process.lock`，但两个模块数据库路径不同，必须分别验证；direct library 调用与已有 worker 不可漏列。
- 迁移先关闭并排空受控 DB/worker/挂载句柄，保留实例身份、原 operation/key、未决 mutation 和 WAL 数据。锁冲突或未知参与者立即拒绝，不能停止别人的进程来强行通过。
- 用可丢弃数据证明各崩溃点均可恢复到完整旧布局或完整新布局，不能出现旧路径缺失后旧程序自动创建空库的窗口；读写会话均不得在半迁移状态启动。
- 为支持的旧 release 固定版本和摘要，运行真实旧 binary，验证服务创建前拒绝。未测试旧版本只能报告 unsupported，不能声称全部 downgrade 已受控。
- 枚举已登记的全部 state/连接/别名及在途操作；每个参与者必须进入同一 protection epoch。另一个已登记 state 未完成升级、TeamDSN/远程挂载无法纳入或 canonical root 无法验证时，binding 仍拒绝。不能利用外部程序例外缩小已登记范围。
- 升级后的应用层 guard 仍须覆盖所有 mutation 入口及发布前复查、祖先路径与 TOCTOU，隐藏不释放、原 key 恢复、guard CAS 与独立 reader/writer 会话。存储栅栏只解决旧启动拒绝条件。
- release 必须确认、复查 epoch 与未决 mutation，并保留恢复证据。不得在 guard active 时恢复旧 DB、删除栅栏或为调用者自动新建替代 state。

这是候选实现方向，不修改原验收标准。原有无 guard 的 state 继续按旧布局工作；真实 state 迁移另行明确授权，当前只允许临时 fixture 上实现和验证。

## 备选方案：独立 OS 身份 broker

若受控 state 栅栏无法覆盖已登记保护域，再评审独立身份 broker。它需要域专用服务身份、受保护祖先、经过验证的旧句柄排空、受控 Pinax/Hermes/编辑器入口和平台安装机制。原 vault 继续是真源，不能改用私有副本；broker 必须校验方法、作用域、revision 和原 operation identity，不提供通用 shell 或任意文件写接口。

此方案需要先决定是否接受编辑器访问方式变化及具体安装/迁移范围。Linux 容器证明不提供 macOS/Windows 实现，也不授权系统服务安装、凭据变更或真实数据权限迁移。未作决定前保持现有 metadata-only/blocked 行为。

## 下一验收前沿

先完成受控 state 的迁移/恢复协议与旧 binary 矩阵，再接应用 guard 与 D08 跨入口验证。随后才允许 D09/D14.3 在真实 Electron、两个 owner binary 和可丢弃 vault 中打开 reader/writer 会话。上述每一步继续使用本项目现有六件套证据，不运行 race。
