# consumer-cloud-drive-shell Specification

## Purpose
TBD - created by archiving change drivebridge-consumer-cloud-drive-ux-v1. Update Purpose after archive.
## Requirements
### Requirement: Light frosted cloud-drive shell and accessible contrast
客户端壳层 SHALL 采用清透浅色侧栏与纯净白内容画布（深色模式同步柔和化），正文与导航项对比度 SHALL 至少达到 4.5:1，业务组件不得硬编码颜色值。

#### Scenario: Light and dark sidebar items remain readable
- **WHEN** 用户在浅色或深色模式下浏览桌面侧栏与主文件区
- **THEN** 所有可见导航项与主内容文字对比度均 ≥ 4.5:1，侧栏与主画布具有清晰表面区分

### Requirement: Default landing opens primary personal drive directly
已登录用户访问空 hash、`#/` 或 `#/files` 时，若存在可访问空间，客户端 SHALL 自动打开首个个人空间（无个人空间时回退首个可访问空间）的文件浏览器；访问 `#/drives` SHALL 展示全量网盘卡片目录。

#### Scenario: Login lands directly in personal files
- **WHEN** 拥有至少一个个人空间的用户完成登录进入默认首页
- **THEN** 页面直接展示该个人网盘的 `Finder` 文件浏览区域，无需先点击选盘实例表

#### Scenario: Switch between multiple drives from sidebar
- **WHEN** 用户拥有多个网盘并在侧栏云盘切换区点击 `打开网盘 <名称>`
- **THEN** 主文件画布立即切换至所选网盘根目录并保持对应空间权限约束

### Requirement: Streamlined 7-item consumer navigation and unified settings hub
桌面主导航 SHALL 提供醒目的「新建/上传」主入口与 7 项高频 C 端入口（全部文件、最近使用、收藏与图集、知识笔记、AI 投递箱、我的分享、回收站）；低频运维能力 SHALL 统一收口于「设置与空间管理」中心，且全部历史 `#/` 深链继续可达。

#### Scenario: Access consolidated management surfaces from Settings Hub
- **WHEN** 用户进入 `#/settings` 或点击侧栏底部的设置与空间管理入口
- **THEN** 可按分类访问外观资料、存储连接、AI 助手授权、自动整理及按权限可见的凭据与团队治理入口

### Requirement: Direct URL ?token= auto-login and single-use invitation tokens
客户端 SHALL 支持在 URL 查询或锚点参数中携带 `?token=`（或 `?key=`）自动完成密钥会话兑换，并在兑换前立即通过 `history.replaceState` 从地址栏擦除密钥参数；服务端与客户端 SHALL 支持签发 `one_time` 一次性邀请密钥，首次浏览器登录成功后原子标记 `consumed` 并擦除密文，拒绝二次复用，同时保持该次已换发的浏览器 Cookie 会话在有效期内正常可用（直至过期或被吊销）。密钥登录框 SHALL 支持粘贴整段邀请口令或完整直连 URL 自动提取密钥。

#### Scenario: One-time ?token= link logs in once and rejects forwarding
- **WHEN** 受邀人首次打开 `/?token=<one_time_key>`
- **THEN** 地址栏立即移除 `token` 参数，自动登录进入其个人云盘，刷新页面保持会话；当第二台设备再次打开同一 `?token=` 链接时服务端拒绝兑换并提示链接已失效

