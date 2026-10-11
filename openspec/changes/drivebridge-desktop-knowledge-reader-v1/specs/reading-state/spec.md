## ADDED Requirements

### Requirement: 应用服务持有本机阅读状态
DriveBridge SHALL 通过同一application service/GORM规则保存收藏、稍后读和阅读位置，仅持必要引用，不写Pinax metadata或复制正文索引；首片不承诺跨设备同步。

#### Scenario: 重启后继续阅读
- **WHEN** 用户保存稍后读/位置并退出重启
- **THEN** 本机state可回读原来源引用与revision，renderer不直接读数据库，也不要求云登录

### Requirement: CAS与未知结果恢复
阅读状态写入 SHALL 校验当前主体、来源、expected_revision及原幂等键；位置与收藏字段独立更新，未知结果先lookup原键。

#### Scenario: 保存响应丢失或并发冲突
- **WHEN** 两个窗口更新同一state或响应超时
- **THEN** 只有合法CAS成功，客户端保留意图并重读/lookup，不新键盲重试或最后写赢

### Requirement: 阅读位置与内容revision绑定
位置 SHALL 包含revision、anchor、offset，只有owner验证锚点后才可转到新版；缺失历史版必须说明。

#### Scenario: 原内容已移动或版本不可达
- **WHEN** 用户从继续阅读打开旧position
- **THEN** 按稳定ID解析，旧版不可达显示选择，不以同标题/路径另一笔记替换

### Requirement: 隔离与删除边界
state与偏好 SHALL 按用户/实例/来源隔离，清理阅读历史不删笔记；授权/session切换清除正文和过期异步结果。

#### Scenario: 换来源后迟到结果返回
- **WHEN** 原来源读取在切换后才完成
- **THEN** 结果丢弃，不重新显示旧正文/名称；用户清理历史只删阅读绑定

### Requirement: 视图状态与阅读进度边界

DriveBridge SHALL 区分浏览器私有视图状态和应用服务持有的阅读进度。标签页、面板宽度、折叠、字号和动效偏好属于私有视图状态；收藏、稍后读和带 revision/anchor/offset 的阅读进度遵循既有应用服务/CAS 合同。

#### Scenario: 视图偏好恢复
- **WHEN** 用户刷新或重新打开同一来源笔记
- **THEN** 恢复标签页、窗格、折叠、正文宽度和焦点等非秘密偏好，不写入 Markdown 或 Pinax metadata。

#### Scenario: 阅读进度保存
- **WHEN** 用户保存收藏、稍后读或阅读位置
- **THEN** 通过应用服务校验主体、来源、revision 和 expected_revision；响应未知先回查原操作，不用新幂等键盲重试。

#### Scenario: 来源或身份切换
- **WHEN** 用户切换 actor、space、source 或 revision
- **THEN** 清除正文和过期异步结果，不能把旧用户或旧来源的正文、名称、位置恢复到新上下文。
