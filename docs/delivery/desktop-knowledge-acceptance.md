# 桌面知识阅读：追溯、验收与交接

2026-10-01 · **本页为设计验收规范，产品任务全部待实施**。主入口：[Electron架构](../architecture/electron-desktop.md)、[阅读UI](../web/ui/knowledge-reading-spec.md)、[本包任务](../../openspec/changes/drivebridge-desktop-knowledge-reader-v1/tasks.md)、[Pinax任务](../../../pinax/openspec/changes/pinax-human-reading-api-v1/tasks.md)。

## 1. 完成口径

- 规格有效不等于API实现；前端fixture不等于HTTP/DB可用；Electron自动化不等于三平台安装实测；本地开发包不等于已签名公开分发。
- 首片绑定后从首页最多两次导航进入正文，无Obsidian/VS Code、无手动服务启动、无云登录、无模型或网络依赖。首次绑定的选择/范围确认不计入后续两步指标。
- `desktop-host`、`knowledge-reader`、`managed-vault-boundary`、`reading-state`及Pinax两个spec分别验收，只有D10.mac/D10.win/D10.linux全部通过才可完成D11工程门。
- D12两周自然阅读与D13/P09后续写入合同不被D11顺带勾选。Pinax原五change的11条真实验收和continuity原窗口保持，不能用本包fixture替代。

## 2. 需求→合同→任务→验证矩阵

下表测试名是实施时必须补充的定位目标；尚不存在的用例/desktop/profile在对应任务建立后才可运行，不将零命中视为通过。

| AC | 行为/反例 | spec与合同真源 | 实施/集成任务 | 验证层级与证据 |
| --- | --- | --- | --- | --- |
| R01 | 一个客户端、同一Web UI，普通MD无需导入Pinax | desktop-host；架构§1/4 | F06、D01/D05/D06 | Vitest组件+renderer请求断言；D09真实切换来源 |
| R02 | 无云登录/预装CLI/网络的本机启动与绑定 | desktop-host；架构§3 | D02/D07、P05、D09/D10 | Go生命周期、testscript、打包后Electron及人工安装 |
| R03 | source/instance/vault对象隔离、同ID不误开 | knowledge-reader；人读API§2 | P02/P06、D01/D05 | 两vault fixture、cursor/scope/晚响应负例 |
| R04 | 超64KiB/512节点中文、代码、表格完整分页 | human-reading-api；人读API§2 | P03/P07、F06/D05/D09 | Go语义/字节断言+真实UI；超8MiB/65536节点明确拒绝全文 |
| R05 | 页间变化、旧版缺失、目录定位、反链水位 | knowledge-reader；人读API§2 | P03/P04、D05/D09 | 409/410、重复/遗漏、anchor与返回位置 |
| R06 | 本地图片/PDF/媒体，坏/越界/失权asset | human-reading-api；人读API§2/3 | P04/P07、F06/D09 | Range/416、符号链接替换、nosniff、禁止自动外链 |
| R07 | 列表/搜索limit、partial索引、无付费副作用 | knowledge-reader；UI§3 | P02/P06、F07/F09.2/D05 | 默认50/max200、无结果≠不可用、取消请求、回退定位 |
| R08 | guard确认原子、重叠/未知alias/在途拒绝 | managed-vault-boundary；架构§5 | D03/D08 | Go应用/HTTP/SDK/MCP真实入口反例、旧overlay兼容 |
| R09 | overwrite/move/delete/祖先/sync/worker无法旁路 | managed-vault-boundary；架构§5 | D03/D08、B09.2 | 实际mutation路径、发布前fence、绑定后新增连接检查 |
| R10 | 收藏/稍后读/位置CAS、重启与未知恢复 | reading-state；架构§4/6 | B09.1/F09.1、D04/D09 | GORM事务/lookup/重启回读，不写Pinax/不删笔记 |
| R11 | 隐藏不释放、降级/卸载不删源/保护 | managed-vault-boundary；架构§5/6/7 | D03/D07/D08/D10 | release确认/在途阻断、兼容writer admission、安装升级清单 |
| R12 | sandbox/IPC/sender/CSP/危险URL/secret保护 | desktop-host；架构§2 | D06/D07/D09 | Electron真实security负例，不能只测配置字符串 |
| R13 | 三屏两步、键盘/焦点、主题/200%/长中文 | knowledge-reader；UI§2–5 | F03/F06/F07/F09、D05/D09 | Vitest/Playwright；1024/1440/1920和320组件降级 |
| R14 | EOF退出/崩溃/休眠，Hermes与外部CLI不受伤 | supervised-local-session；架构§3 | P05/P07、D02/D07/D10 | 真实进程ID/生命周期证据，正常并发但不race |
| R15 | 三平台安装和离线完整闭环 | desktop-host；架构§7 | D10.mac/win/linux、D11 | 每平台真实OS/arch/版本/包摘要和人工+自动化结果 |
| R16 | 旧API/RPC/MCP/URI/渲染预算/Web/PWA不改 | 两owner兼容设计 | P06/P08、D11、原F/B回归 | 旧golden/contract+新unknown状态，不重写归档 |
| R17 | 自然阅读效果、后续能力保留 | 根K01–K08；本地D12/D13 | D12/D13、P09 | 14天脱敏反馈；后续spec，不修改原11条验收 |

## 3. 场景矩阵

当前所有新增场景均为 `exploratory`（目标first-support）；不能因本页已有矩阵标成熟。

| scenario_id | 用户与job | 必需产物 | gate/review | evidence/export | 验证入口 |
| --- | --- | --- | --- | --- | --- |
| local-note-reading | 个人离线打开笔记 | binding、ReadingRef、DocumentPage | 正文scope/固定版本/两步阅读 | D09/D10；复制固定Pinax引用 | Electron原生用例+owner API回读 |
| file-markdown-reading | 文件用户预览普通MD | 固定FileRef、共享reader | 原文件权限、脚本/外链隔离 | F06/D09；固定文件引用 | 既有PreviewFiles+新双来源用例 |
| research-reuse | 研究者搜索并回用来源 | 分页命中、双链、引用 | 权限/水位/歧义与版本 | D09；显式复制引用或source导出 | P02/P04 + D09 |
| long-document | 长文读者看大表格/代码 | continuation、outline、complete | 预算/UTF-8/节点/版本 | P07/D09；超限原文流 | TestReaderDocument+真实分页UI |
| resume-after-restart | 用户继续阅读 | 本机ReadingState | CAS/lookup/引用可达性 | D04/D09；无跨设备承诺 | TestReadingState+重启用例 |
| managed-vault-safety | 维护者避免旁路覆盖 | guard、binding revision | 全写入口、alias、在途、release | D08；owner-required指引 | TestManagedVaultIntegration |
| owner-unavailable | 用户遇到Pinax或权限故障 | 安全错误/原恢复身份 | 禁fallback/清敏感内容 | D09；重连/诊断 | 故障注入，不直读private文件 |
| mobile-hermes-handoff | 用户手机获取内容 | 既有grounding source | 已授权MCP与可达性 | 本轮不新增运行证据；Hermes原路径 | 单独获授权后核验，不计桌面门 |

## 4. 命令与工具归属

### 本轮文档检查（已存在的命令）

从 `cli/drivebridge`：

```bash
openspec validate drivebridge-desktop-knowledge-reader-v1 --strict --no-interactive
openspec validate drivebridge-personal-workspace-rebuild-v1 --strict --no-interactive
openspec validate drivebridge-personal-workspace-contracts-v1 --strict --no-interactive
git diff --check
```

根change与Pinax change在各自owner验证。只改文档不运行产品测试/构建，不把strict当业务证据。

### 后续实现复用的入口

```bash
python3 scripts/check.py -- pnpm --dir web exec vitest run
python3 scripts/check.py -- go test ./internal/bridge ./internal/remote ./cmd/drivebridge
python3 scripts/check.py -- go -C server test ./internal/service ./sdk
python3 scripts/check.py -- pnpm --dir web exec playwright test
```

先创建具名测试并确认非零命中，开发期使用任务内过滤；稳定后才全量。不要外层runner再套 `pnpm test` 等内层runner。已有package script/聚合依赖先静态确认无race。

规划命令（当前不可宣称存在）：D01扩展B01 `make contracts`/`make contracts-check` 的desktop profile；D06建立desktop Vitest/typecheck；D07建立Forge make/artifact检查；D09在既有Playwright中登记Electron native项目和具名用例，不新增测试框架。Pinax P01在既有integrationevidence登记human-reading-api profile，P07执行它。新增命令实际存在后回填本页，不在本轮安装或生成产品机器合同。

## 5. 平台实机清单与证据

| 门 | macOS | Windows | Linux |
| --- | --- | --- | --- |
| 基线 | 14+ arm64 | 11 x64 | Ubuntu24.04 LTS x64，X11与Wayland |
| 包 | DMG | Setup EXE（Squirrel.Windows） | DEB |
| 宿主差异 | 签名/Gatekeeper、app生命周期 | DACL、Unicode/大小写、JobObject | sandbox/权限、两显示协议、进程组 |
| 当前状态 | 未实施/未实测 | 未实施/未实测 | 未实施/未实测 |

每平台都做：安装前无CLI、断网启动、只读绑定、搜索与长文、双链/资产、阅读状态、重启/休眠、Pinax异常、并发外部CLI、关闭无孤儿、手动升级保state/vault、卸载保vault。记录OS/arch、Electron/配套binary版本、包digest、显示环境、签名状态、执行用例和未验项；不保存机器用户名/真实路径/正文/secret。

普通component/e2e由既有runner生成六件套（summary.json、command.txt、stdout.log、stderr.log、env.json、artifacts/），失败也保留原退出码。D07扩展同一runner的platform metadata输入，人工清单结果通过runner/application记录，不手写JSON回执。截图仅合成fixture；认证/secret步骤禁trace。跨平台缺机器/权限保持blocked或未完成，不用Linux结果填另外两列。

签名/公证及公开分发另行授权与验收。未签名开发构建可提供有限工程反馈，但不能据此勾完需要可信安装的门，更不能教用户关闭系统安全。

## 6. 自然使用与后续

D12在用户同意真实使用、工程可用后记录固定14天窗口，不自动采集正文或上传遥测。记录每天实际阅读会话、两步进入是否成功、找回/引用复用和主要阻塞；未读天也如实记录，不能补造样本。结论为Continue/Iterate/Stop及短理由，不假设高频使用证明整体Pinax生产就绪。

D13/P09把捕获、正文编辑、提案审阅、小批整理交接到后续owner合同，必须带scope/revision/snapshot/receipt/unknown恢复；本首片不开放写。DriveBridge手机客户端仍在原F18/B14要求内，Hermes只是本次Pinax手机入口选择。
