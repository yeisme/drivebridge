## ADDED Requirements

### Requirement: 挑战应答打开不传输链接秘密

系统 MUST 提供按分享发放单次挑战 nonce、并由接收方以链接秘密派生的证明完成打开的路径。证明 MUST 是对（share id, nonce）的密钥化摘要（HMAC 族），链接秘密 MUST NOT 出现在任何请求体、Cookie、日志或错误正文；服务端以既有加密持久化的链接秘密本地校验。挑战 nonce MUST 单次使用且有效期不超过 5 分钟；证明失败 MUST 独立于提取码失败计数，连续失败达到既有锁定阈值后按原节流合同拒绝。成功打开 MUST 颁发不落地秘密的会话凭证（服务端存哈希的不透明令牌），既有明文 grant 请求体打开路径 MUST 继续可用并收敛到同一会话颁发。

#### Scenario: 证明打开颁发会话且秘密不出现在传输中

- **WHEN** 接收方以 `GET /api/v1/shared/{id}/challenge` 取得 nonce，并提交正确 HMAC 证明
- **THEN** 系统 MUST 颁发与既有打开同范围、同时效的会话凭证
- **AND** 该次交互的请求体、Set-Cookie 与日志 MUST NOT 包含链接秘密明文

#### Scenario: nonce 重放与过期拒绝

- **WHEN** 同一 nonce 被第二次提交，或提交时已超过 5 分钟
- **THEN** 系统 MUST 拒绝为 `share_unavailable` 或既有等价稳定码
- **AND** MUST NOT 消耗打开额度，MUST NOT 颁发会话

#### Scenario: 证明失败独立节流

- **WHEN** 同一分享连续提交错误证明达到既有锁定阈值
- **THEN** 系统 MUST 按既有节流合同返回 `throttled`
- **AND** 提取码失败计数 MUST NOT 因此变化，正确提取码打开不受影响

#### Scenario: 旧 grant 打开路径保持

- **WHEN** 旧客户端继续 `POST /api/v1/shared/{id}/open` 提交 grant
- **THEN** 系统 MUST 按原合同成功并颁发会话凭证
- **AND** 旧 Cookie 值（grant 本体）在过渡期 MUST 继续被既有受保护读取接受

### Requirement: 接收设备绑定与按设备撤销

系统 MUST 允许 `audience=link` 分享的接收方在完成一次完整打开（含要求时的提取码）后，于该会话内注册 Ed25519 设备公钥，之后凭设备签名挑战免链接秘密、免提取码再次打开。注册 MUST 以公钥指纹幂等；同分享设备数达到上限 MUST 拒绝注册。设备打开 MUST 对（share id, nonce）签名并复用同一挑战发放与失败节流合同；未知设备或坏签名 MUST NOT 泄露与已撤销分享可区分的信息。设备绑定 MUST 可由分享 owner 单独撤销，撤销后该设备既有会话在下次受保护读取时失效；分享撤销或到期 MUST 使全部绑定失效。`audience=users` 分享 MUST NOT 提供设备绑定（其接收方已有登录身份）。Agent MUST NOT 注册或使用接收设备绑定。

#### Scenario: 一次完整验证后设备免密打开

- **WHEN** 接收设备在有效会话内注册公钥，链接过期前以设备签名挑战再次打开
- **THEN** 系统 MUST 颁发标记设备指纹的会话，且不再要求链接秘密或提取码

#### Scenario: 重复注册幂等

- **WHEN** 同一公钥在同一分享下重复注册
- **THEN** 系统 MUST 返回原设备身份，不新增绑定、不重置撤销状态

#### Scenario: 按设备撤销只影响该设备

- **WHEN** owner 撤销分享下某设备后再以该设备签名打开
- **THEN** 该设备 MUST 被拒绝且其既有会话下次读取失效
- **AND** 其他设备绑定与它们的会话 MUST 不受影响

#### Scenario: 未知设备不泄露分享状态

- **WHEN** 请求对不存在的分享或未绑定设备做设备打开
- **THEN** 系统 MUST 返回与已撤销分享不可区分的稳定错误
- **AND** MUST NOT 确认设备曾存在

#### Scenario: 受众为用户时不提供设备绑定

- **WHEN** `audience=users` 分享的会话请求注册接收设备
- **THEN** 系统 MUST 拒绝，登录身份继续按既有成员校验打开

### Requirement: 打开与下载额度

分享创建 MUST 支持打开上限、下载上限与单次分享标记；单次分享 MUST 等价于打开上限 1 且下载上限 1。额度只对成功动作计数：打开额度按会话颁发计数，下载额度按（会话、文件、固定版本）的下载字节流首次建立计数，连续 Range 续传 MUST 只计一次。额度耗尽后的新打开与继续下载 MUST 被拒绝且对未持凭据者与已撤销分享不可区分；既有会话在下载额度耗尽前 MUST 可继续清单与缩略图读取。已传出的字节 MUST NOT 被声称收回；额度耗尽 MUST NOT 自动撤销或删除分享记录。未设置额度的分享 MUST 保持无自动上限的原行为。

#### Scenario: 打开额度耗尽

- **WHEN** 分享设置打开上限为 3 且第 4 次新会话请求到达
- **THEN** 系统 MUST 拒绝该次打开
- **AND** 已颁发且未过期的会话 MUST 在下载额度内继续可用

#### Scenario: 续传只计一次下载

- **WHEN** 同一会话对同一下载以多段 Range 续传完成
- **THEN** 下载额度 MUST 只消耗 1
- **AND** 断点重启后续传 MUST 不重复计数

#### Scenario: 额度耗尽不删除分享

- **WHEN** 下载额度耗尽
- **THEN** 后续下载 MUST 被拒绝，清单与缩略图在既有会话内 MUST 可读
- **AND** 分享记录与 owner 侧额度视图 MUST 保留

#### Scenario: 未设置额度保持原行为

- **WHEN** 分享未声明任何额度
- **THEN** 系统 MUST 保持既有无上限行为，旧客户端不受影响

### Requirement: 接收状态衔接与不虚报

设备签名会话 MUST 只支持把交付观测升级为设备可辨识的 `receiver_verified`，MUST NOT 自动产生 `receiver_confirmed`；匿名链接会话 MUST 保持既有 `opened`/`bytes_served` 上限。会话的接收类别（anonymous、device、user）MUST 由服务端从颁发路径推导，MUST NOT 接受请求体自报。设备指纹是假名标识，MUST NOT 在 owner 视图外暴露设备公钥以外的身份信息。固定交付清单未发布或前置包未实现时，本能力 MUST 独立工作且 MUST NOT 伪造交付回执。

#### Scenario: 设备会话升级观测但不自动确认

- **WHEN** 设备签名会话完成下载且 fixed-delivery 已实现
- **THEN** 交付回执 MUST 记录 `receiver_verified` 与接收类别 device
- **AND** MUST NOT 未经该设备或登录用户的明确确认动作记录 `receiver_confirmed`

#### Scenario: 前置未实现不虚报

- **WHEN** fixed-delivery 能力不可用而设备会话完成下载
- **THEN** 系统 MUST 只记录既有分享观测事实
- **AND** MUST NOT 生成冒充交付回执的记录

#### Scenario: 接收类别不可自报

- **WHEN** 请求体携带接收类别字段
- **THEN** 系统 MUST 忽略该字段并以服务端推导值为准

### Requirement: 兼容、权限与证据分层

系统 MUST 保留既有公开合同与稳定身份：旧 `open` 请求、Cookie 形态、错误码、URI、受众语义与媒体读取不变；新增能力仅在既有分享管理权限与打开凭据内执行。挑战、设备与额度字段对旧客户端 MUST 为可选加法；未启用或未实现时新入口 MUST 明确 unsupported/blocked。规格校验通过 MUST NOT 被当作实现、真实后端或部署验收；本包任务完成前就绪状态保持设计。

#### Scenario: 旧客户端与未启用能力

- **WHEN** 旧客户端使用既有参数或新客户端请求未启用能力
- **THEN** 旧行为保持；新能力明确 unsupported/blocked，不静默扩权、换身份或创建替代任务

#### Scenario: 仅规格检查通过

- **WHEN** 本包通过 OpenSpec strict 但实现任务尚未完成
- **THEN** 就绪状态保持设计，所有实施任务未勾选，不宣称真实文件、模型或部署已验收
