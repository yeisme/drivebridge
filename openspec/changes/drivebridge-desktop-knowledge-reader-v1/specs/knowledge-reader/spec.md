## ADDED Requirements

### Requirement: 共享阅读体验保持来源身份
知识库 SHALL 使用普通文件与Pinax两个固定适配器，共用阅读组件但不复制Pinax关系或私有索引；每个目标带instance/source/object/revision。

#### Scenario: 两个vault含同名同ID笔记
- **WHEN** 用户从一个来源搜索或引用打开笔记
- **THEN** 固定该binding和revision，不跨来源猜测，不按路径或标题合并身份

### Requirement: 三屏与低摩擦阅读
客户端 SHALL 提供继续阅读/稍后读/最近更新首页、可恢复搜索和正文/目录/反链阅读页，可设置知识库为启动页，绑定后最多两次首页导航进入正文。

#### Scenario: 搜索后阅读并返回
- **WHEN** 用户筛选结果、打开一篇笔记再返回
- **THEN** 查询、筛选、分页位置、滚动、焦点与选中项恢复，不要求外部编辑器

### Requirement: 长文读取与固定版本
Pinax正文 SHALL 消费明确授权的人读分页合同，展示complete/cursor/预算/版本事实，不把旧64KiB/512节点投影当完整正文。

#### Scenario: 分页期间外部编辑
- **WHEN** 当前note变化而旧revision已不可达
- **THEN** 停止拼接并提示重新打开，不混入新版、不自动snapshot，也不静默截断后标完成

### Requirement: 关系附件与故障可分辨
客户端 SHALL 分别表达正文、反链、附件的partial/stale/forbidden/offline/unsupported/error，缺索引不等于空结果，owner不可用不能转私有文件读取。

#### Scenario: 本机笔记可读但云附件离线
- **WHEN** 网络断开且附件未下载
- **THEN** 本机正文仍可阅读，附件明确不可用，不自动同步、抓取外链或调用模型

### Requirement: 无障碍与不可信内容隔离
阅读器 SHALL 沿用现有tokens，支持键盘、焦点恢复、200%缩放、浅深主题和reduced-motion；转义不可信HTML并禁自动外网内容。

#### Scenario: 长代码与键盘目录导航
- **WHEN** 用户在200%缩放下用键盘打开长中文笔记并跳目录
- **THEN** 正文可读、横向滚动限于块内、焦点不丢失，内容不能执行宿主API

### Requirement: 多笔记会话与统一 Markdown 渲染

阅读器 SHALL 以 source/object/revision/anchor 作为笔记身份，复用同一解析结果服务源码、实时预览、阅读、大纲和引用跳转；不得按标题或路径猜测跨来源同一对象。

#### Scenario: 三模式切换
- **WHEN** 用户在阅读、实时预览和源码之间切换
- **THEN** 保留草稿、选区、滚动位置和 anchor；历史 revision 保持只读，不静默切到最新版。

#### Scenario: 渲染失败或不可信内容
- **WHEN** KaTeX、Mermaid、HTML、嵌入或外部资源解析失败
- **THEN** 保留原始 Markdown，显示明确错误/unsupported 占位和重试入口；不执行原始 HTML、不自动联网。

#### Scenario: 大纲、双链与反向链接
- **WHEN** 用户从正文、双链、大纲或反向链接跳转
- **THEN** 使用同一稳定 anchor 映射，目标来源和 revision 明确，失权或不可达目标不泄漏标题并提供返回入口。

### Requirement: Obsidian 式导航闭环

阅读器 SHALL 提供命令面板、快速打开、悬停预览、标题/列表折叠和区块引用，但不引入独立知识库产品或第二套权限状态机。

#### Scenario: 快速打开与命令
- **WHEN** 用户按 Cmd/Ctrl+P 或 Cmd/Ctrl+O
- **THEN** 分别进入命令面板或快速打开，显示当前范围，Escape 返回触发控件；搜索结果不扩大授权范围。

#### Scenario: 区块折叠和工具条
- **WHEN** 用户折叠标题/列表或打开区块工具条
- **THEN** 折叠状态按 object/revision 私有保存，桌面鼠标/键盘可复制块链接或引用；触控端不被悬停工具条遮挡且折叠触点至少44px。
