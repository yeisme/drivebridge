# 设计

客户端拥有 UI 与浏览器偏好；资料的权限、持久化与图片归属由 backend-server/drivebridge 拥有。本次资料新增字段与路径为增量兼容，不修改现有用户身份或登录 Key。

统一使用当前已安装的 shadcn Base UI 与 TanStack Table v9。SelectAdapter 仅把现有 option 声明转换为共享控件，不渲染备用原生 select。实体字段使用 Combobox，静态枚举使用 Select。表格用 DataTable；动态列与混合行使用 GridTable，同样复用 DataTable 行为层。表格有稳定行 ID、排序、搜索、列显示、选择、分页及窄屏卡片呈现，后端已有分页维持原分页边界，表内搜索明确针对当前结果。

资料显示名称保存后同步账户菜单。头像从本人个人网盘内选择固定版本，协议由后端 profile 变更定义。外观偏好保存在浏览器，恢复默认不修改服务端资料。所有下拉菜单、弹窗与确认浮层由 Base UI 管理焦点和键盘。

CSS 删除旧按钮、原生 select 覆盖、directory-table 与 transfer-card 规则。保留文件画布、树、图表和工作区布局 CSS，语义颜色来自 shadcn.css；媒体内容不反色。UI 静态检查解析 JSX 并检查 CSS，不能通过重命名旧类或藏一个原生控件绕过。

验证使用已有 Vitest、Playwright 和项目 evidence runner。真实浏览器必须在构建后运行，因为 fixture 服务的是 dist；普通 fixture 与 team demo 分别验证。回滚可退回旧客户端，新增资料字段继续保留，普通文件与旧客户端协议不受影响。
