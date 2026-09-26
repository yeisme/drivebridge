# 本机文件操作、预览与远程交接

`drivebridge view` 仍只监听回环地址，启动链接只用于换取本地 HttpOnly 会话。文件修改由显式启用的 owner 经私有 0600 socket 执行；网页不打开第二份可写数据库，也不能传入任意远程目标。

```bash
drivebridge mcp --enabled --insights
drivebridge view
```

实时列举保留旧 `items` 文件数组，新增 `directories` 表示真实目录。旧 CLI List 行为保持文件列表；本机新页面根据实际目录显示树，不把已观察索引冒充全盘。缺少 owner 时显示索引并禁用写操作。

私有 `/files/action` 通道支持 mkdir、文件 move/copy、目录 copy/move/trash、restore、限定本机路径的上传和传输恢复／取消。复制优先走存储后端的服务端 Copy，不行则读取再上传；同名自动改名，不覆盖。目录作业最多 1000 个文件，不能把文件夹移进自己。浏览器文件以原始字节 PUT 进入私有暂存；交给 owner 后后台继续传输。未完成浏览器发送时关页仍需重新选择文件。

图片、视频、音频、PDF 通过固定引用读取，源版本改变时明确失败。缩略图与兼容预览复用 DriveBridge Server 的 `sdk/mediapreview`，不会自行维护另一套格式转换规则。转换组件缺失时保留原文件入口。Office、RAW 和 PDF 栅格化需要相应转换器与 Bubblewrap。

## 本机同步计划与冲突保护（新增）

状态：已实现并通过本地 fixture 定向验证（`TestFPSyncPlan` 与 CLI testscript）；真实 OneDrive、跨 owner Server 联调与发布部署未验证。

`drivebridge sync` 一键入口保持不变；新增显式可复核路径把扫描、确认与执行分开：

```bash
drivebridge sync plan --space nas --resync
drivebridge sync status PLAN_ID
drivebridge sync apply PLAN_ID --revision 1
drivebridge retention preview --space nas
```

- `sync plan` 冻结四分类计划与冲突项（SyncPlan/SyncBaseline/SyncItem 持久化，keyset 分页默认 50、最大 200）；计划阶段不写任何一端。首次计划需 `--resync`，同名远端差异以冲突预览，不伪装成无冲突初始化。
- `sync apply` 仅在用户终端执行：执行前重新核对单写入者（本机 host+state 身份）、两端根身份、基线摘要、双端扫描摘要与删除规模，任何变化返回 `plan_stale`，旧确认不扩展，需重新生成计划。跨设备不自动接管：`writer_handoff_required`，超时不是旧 writer 停止的证据；交接 = 在本机新建计划重建基线。
- 冲突保留两份并以原 operation 命名；默认 25% 删除阈值冻结进计划，调整需用户新建计划显式选择。执行中断重启后计划标记 `interrupted`：两侧未变可沿用原 operation 与幂等键恢复，已部分落地则 `plan_stale` 并提示先回读原操作。扫描不完整（`sync_scan_incomplete`）绝不传播删除并展示 coverage。
- `retention preview` 只读分类：浏览器暂存、本机回收站、rclone VFS 未刷写上传字节（受保护，永不列为可清理）、可重建缓存与远端引用分开预览；mount alive 不代表远端已保存；.pinax/数据库/Capsa 信封继续拒绝通用整理写入。
- MCP 只注册只读 `sync_plan_get` 与 `retention_preview`；无 apply 工具，Agent 不启动挂载、不执行同步写入、不 purge。

回滚：删除本包新增入口即回到旧行为；已持久化的计划、原 Operation、文件与版本保留，不回滚丢弃业务写入的数据库快照。新表为加法迁移（expand-then-contract），旧 `sync` 的输出、幂等键与快照文件语义不变。


## 远程副本分享

用户在网页明确选择远程服务、用户访问密钥与目标空间。连接凭据及分享策略保存在本机私有加密存储，浏览器不持久保存密钥。创建分享需要用户身份，Agent 文件 token 不具备分享管理权限。

固定交付在复制前固定素材引用；持续共享由在线 owner 周期同步。复制完成后调用远程服务的同一分享合同，支持链接／指定用户、提取码、有效期和下载设置。源文件不直接暴露到公网。重复分享创建使用稳定幂等键；源离线时保留已发布副本并显示同步失败，用户可恢复原同步。

活动分享期间不允许把目标静默改成另一个远程账户。撤销先停止同步，再撤销远程授权；远程撤销失败保留待重试状态，不能声称已彻底撤销。

## 创作交接

本机创作面板将明确选择的固定引用复制到远程素材库，再使用远程 DriveBridge 的 Eikona／Scaena 消费连接。原始媒体字节走 HTTP，引用与任务回执不包含服务商凭据。远程生成与结果回存继续由原 owner 执行，本机网页不成为生成服务。

## 验证

```bash
python3 scripts/check.py
go vet ./...
CGO_ENABLED=0 go build ./cmd/drivebridge
```

`TestLocalShareRemoteCopyAndRevocation` 使用真实本机目录 provider、私有 socket 和独立远程 fixture 进程，验证复制、固定版本、重放与撤销。其余测试覆盖 CLI、索引、权限、传输和原有协议。证据在本项目 `temp/integration-test-runs/`；没有使用真实云账户或付费模型。
