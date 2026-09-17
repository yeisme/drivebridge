## ADDED Requirements

### Requirement: 存储接入引导命令

CLI SHALL 提供 `storage kinds`（能力目录，含每类必填旗标与能力真值）、`storage probe`（只读可达性 + 根层列举样本，绝不写探针）与 `storage add --kind`（校验缺旗标 → 预检 local-root/max-bytes → 自动创建 alias/webdav remote → 探测 → 委托既有 space add 合同）。能力真值 SHALL 与 `backend_capabilities` 同源：generic 类永远不宣称 direct_url / reference_handoff，后端相关能力（server-side copy）以键缺席表达。`--remote-path` 对 gdrive/webdav/smb/sftp/alias/openlist 可空即远端根。失败与成功信封 command SHALL 一致。

#### Scenario: local kind 零配置接入
- **WHEN** `storage add --kind local --space docs --remote-path <绝对目录> --local-root <目录>`
- **THEN** 自动创建 alias remote，探测根层，注册空间，回执 `command=drivebridge.storage-add` 且 `roundtrip_status=not_verified`

#### Scenario: 缺旗标点名
- **WHEN** 必填旗标缺失
- **THEN** 以 `storage_missing_flag` 失败并列出旗标名

#### Scenario: 远端根接入
- **WHEN** `storage add --kind gdrive --space g --remote mygd --local-root <目录>` 省略 --remote-path
- **THEN** 以远端根注册空间并可 `ls`

#### Scenario: 失败回滚
- **WHEN** 自动创建 remote 后探测或落库失败
- **THEN** 该 remote 被删除；修正输入后同一 space 重试成功，不触发 remote_exists

#### Scenario: 凭据轮换不静默复用
- **WHEN** 同名 webdav remote 的 url/user 一致但密码不同
- **THEN** 以 `remote_exists` 显式拒绝，不用旧密码探测

#### Scenario: 失败信封标识
- **WHEN** `storage probe/add` 机器模式失败
- **THEN** 信封 command 为 `drivebridge.storage.probe`/`drivebridge.storage-add`


### Requirement: 接入引导的错误码与文档同步

新码 `storage_kind_unknown`、`storage_missing_flag`、`probe_failed`、`credential_file_not_private` SHALL 进入 recovery registry，并由同步测试保证 `docs/error-codes.md` 表格不漂移。

#### Scenario: registry 与文档一致
- **WHEN** registry 修改任一新码
- **THEN** 同步测试要求 docs 表同步更新，否则测试失败
