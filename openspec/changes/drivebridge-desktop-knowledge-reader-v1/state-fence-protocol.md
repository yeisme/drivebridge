# 受控 state 格式栅栏协议

2026-10-24，D03.4 本地实现。共享 owner 为 `server/pkg/managedstate`，供根 CLI 与 Server 后续共同消费；目前只有测试调用，未接 service startup、CLI 命令、binding 确认或真实 state 迁移。Linux 为当前实现验证范围，其他平台返回 `state_layout_unsupported`，不等同于最终产品平台范围缩减。

## 从目录原型改为原子文件替换

上轮目录原型只证明旧 `bridge.New` 遇到目录会拒绝。先移走数据库、再创建目录会产生旧路径缺失窗口，因此本实现不用该序列。

旧路径始终存在。升级先把完整 SQLite 字节保存到 `.managed-state-v1/<原数据库名>`，然后将已经 fsync 的非 SQLite 格式标记原子 rename 到旧路径。旧 GORM/SQLite 可能延迟到 AutoMigrate 才检查格式，因此旧版拒绝证据必须包含服务构造阶段，不能只观察 `gorm.Open`。

## API 与事实边界

- `Prepare(path, kind, migrationID)` 只读取已有状态，输出绑定目录 device/inode 身份、数据库 SHA-256/长度和原 migration ID 的 plan，不创建 registry 或锁文件。`kind` 只允许 `cli`/`server`，固定映射 `state.sqlite`/`metadata.sqlite`，不接任意数据库路径。
- `Install(path, plan)` 只适用于另行授权的 state 迁移。它要求原 `process.lock` 已存在、私有且不是链接，以相同内核锁非阻塞取得独占权；不杀进程，不等待无界时间，不删除 sidecar。
- `Inspect(path, kind)` 只报告 `legacy` 或 `fenced` 布局。缺少或损坏快照、非法 marker、重复 JSON key、目录别名不会自动修复，更不会创建替代数据库。
- `fenced` 是维护状态。调用方仍需证明整个保护域、安装应用 guard、CAS 登记 binding 和创建新的受权会话，才能开放正文或写入。该库不生成 ready/approved，也不启动 Pinax。
- 数据库路径只供 owner 内部使用，不能返回 renderer。marker 由应用生成，仅保存 kind、原迁移 identity、所选目录身份和原数据库 fingerprint，不保存用户正文、凭据或 provider 信息。

## 发布与崩溃恢复

| 阶段 | 旧数据库路径 | 同一 plan 重启后的行为 |
| --- | --- | --- |
| 快照临时文件写完、尚未发布 | 完整旧 SQLite | 重新复制，忽略非 canonical 临时文件 |
| 快照以不覆盖方式发布并完成目录 fsync | 完整旧 SQLite | 只在快照 digest/长度与原 plan 相同时复用 |
| 格式标记临时文件完成 fsync | 完整旧 SQLite | 再次复验 source/sidecar/目录身份后发布 |
| 格式标记原子 rename 后、目录 fsync 前 | 完整 marker | 原 plan 回读后补目录 fsync；失败为 `state_commit_unknown` |
| 目录 fsync 后 | 完整 marker | 同 identity 幂等回读；不同 identity 为 `state_migration_conflict` |

旧 SQLite 字节在发布前不会被移动、截断或覆盖；最终替换发生时已有独立、持久的完整快照。新路径只用固定 kind 映射，创建快照通过 hard link 发布以拒绝覆盖既有恢复文件；旧数据库路径与快照不共享 inode。缺乏所需 link/rename/directory-sync 语义的文件系统不能准入。

源数据库出现 WAL、SHM 或 rollback journal 时返回 `state_checkpoint_required`。排空 writer、通过 owner 正常关闭/检查点数据库属于上层迁移前置；此库不删除 WAL 来制造“干净”状态。原有 operation ID、idempotency key、未决结果及其他表通过整库字节保留。

旧进程在 marker 尚未发布时可能合法使用完整旧布局。如果源数据库随后变化，原 plan 恢复返回 `state_changed`；不覆盖旧源或既有快照，也不自动换迁移 key。需要上层明确 reconcile/撤销未发布计划后重新形成计划。此处不实现隐式回滚，更不在 guard active 时恢复旧 DB；完整 release/回滚仍依赖 D03 的 epoch、在途 mutation 和确认合同。

## 已验证与尚未证明

已用临时真实 SQLite/GORM 验证两种文件名的完整字节保留、原 operation/key/unknown 状态、原 plan replay、替换 key 拒绝、锁冲突、sidecar 保留、source 变化、目录替换、同字节跨 state、别名与坏 marker/快照拒绝。五个阶段通过真正的子进程 `os.Exit` 中断，并由父进程重新调用原 plan 恢复；helper 不作为普通独立通过项计数。根 `bridge.New` 的真实旧构造路径拒绝 marker，未改恢复数据库。

进程退出测试不等于断电/文件系统损坏注入。Linux fsync/rename 调用成功也不代表 macOS/Windows 已验。尚需固定已发布旧 binary 的版本/摘要及多入口矩阵、Server 真实构造路径、所有已登记 state/连接/worker/挂载准入、应用 guard 持久化与 release、真实 Electron 与三平台证明。当前 root `bridge.New` 和 Server `New` 均未识别新布局，不能把组件测试当作用户已可使用的新功能。

根模块复用 Server 已有 `gofrs/flock`；root 的依赖图选择 v0.13.0，Server 保持既有 v0.12.1。未增加新的锁实现或修改现有 process lock 的名字、模式和作用域。完整 `go mod tidy -diff` 遇到既有私有 `credentialctl` 校验不可用，未使用其结果改写依赖树；本片只通过 `go mod edit` 登记新共享包显式带入的 flock requirement。
