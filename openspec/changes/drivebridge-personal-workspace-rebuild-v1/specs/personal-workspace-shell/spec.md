## ADDED Requirements

### Requirement: 旧前端可恢复清理
客户端 MUST 在记录 dirty 工作树和旧行为基线后拆除旧个人页面编排，并使新版入口与原型模拟状态隔离。

#### Scenario: 先清理再建立新默认入口
- **WHEN** F02 完成旧 main/Finder 页面与布局依赖拆除
- **THEN** 新入口保持可构建；登录、CSRF、上传恢复和阅读器有明确承接路径，不能删除用户数据或未提交的他人工作。

#### Scenario: 原型模型误入生产
- **WHEN** 构建或依赖检查发现生产 import prototypes 内存模型
- **THEN** 阻止验收并改为真实合同或显式测试 fixture，不把演示样本作为用户文件。

### Requirement: 双端壳层与设计系统
客户端 MUST 使用当前原型的三组导航、统一 Precision Pine tokens 和手机五栏结构，并遵循 UI Spec 的断点与详情布局。

#### Scenario: 手机和窄屏
- **WHEN** 在 320px 或 390px 访问主要页面
- **THEN** 无页面横向溢出，目录与详情全宽可关闭，主操作不被底栏或键盘高度遮挡。

#### Scenario: 桌面详情缩窄
- **WHEN** 固定详情后缩小可用容器
- **THEN** 详情按可用宽度降级，不将正文压到零，不覆盖已保存的桌面宽度偏好。

### Requirement: 控件可访问与状态完整
客户端 MUST 为每个交互提供键盘、触摸、焦点与加载/空/权限/错误/恢复状态。

#### Scenario: 键盘调整详情
- **WHEN** 使用方向键、Shift、Home/End、Escape 操作分隔条
- **THEN** 按 UI Spec 调整并朗读数值，Escape 取消本次操作，焦点保持可见。

#### Scenario: 无权详情
- **WHEN** 已打开文件被撤权后再次读取
- **THEN** 移除旧正文与敏感元数据，显示明确权限状态与返回入口，不永久 loading。

### Requirement: 深链与持久化兼容
客户端 MUST 保持既有深链含义、租户空间范围和非秘密恢复身份，并隔离用户数据。

#### Scenario: 旧流程深链
- **WHEN** 用户访问旧 flows 或 overview URL
- **THEN** 按兼容表转到同义视图并保留 plan/operation/scope，不静默变成个人首页。

#### Scenario: 切身份晚响应
- **WHEN** 退出用户 A 并进入用户 B 后 A 请求完成
- **THEN** 丢弃 A 响应和缓存，不显示 A 的名称、数量、正文或上传恢复记录。

### Requirement: 设计与实现证据分层
客户端 MUST 分开报告原型参考、mock UI、本地真实 API、外部服务与发布状态。

#### Scenario: 只有 strict 和参考截图
- **WHEN** 本包设计检查通过但业务实现未完成
- **THEN** 实现任务保持未勾选，不称新版已重写或真实多 Agent 已验收。

#### Scenario: 单个切片完成
- **WHEN** 文件切片通过本地 e2e 而原生宿主不可用
- **THEN** 仅记录已验证切片，原生/内置助手保留后续任务和明确 unsupported 状态。

### Requirement: 多笔记工作台壳层与私有交互状态

客户端 MUST 在现有个人工作台壳层内提供笔记目录树、标签页、双窗格和关联面板；这些布局状态属于用户私有视图状态，不成为文件、Pinax 或权限真源。

#### Scenario: 标签页和预览页
- **WHEN** 用户单击、双击、固定、关闭或重新打开 Markdown 笔记
- **THEN** 未固定单击复用预览页，双击或编辑后固定；带草稿的关闭需要确认，焦点和滚动位置可恢复。

#### Scenario: 窗格宽度不足
- **WHEN** 当前容器不足以容纳两个最小 480px 窗格或关联面板
- **THEN** 先将关联面板转为覆盖，再只显示当前窗格；不让正文变为零宽，不重建草稿。

#### Scenario: 命令快捷键让位
- **WHEN** 用户在输入框、CodeMirror 或中文输入法组合期间按 Cmd/Ctrl+P、O、K 或 Tab
- **THEN** 工作台快捷键让位给编辑控件；组合结束后焦点和选区保持不变。

#### Scenario: 减少动态效果
- **WHEN** 系统或用户启用 reduced-motion
- **THEN** 取消非必要位移、缩放和连续动画，但保留焦点、选中、保存、错误和内容层级反馈。
