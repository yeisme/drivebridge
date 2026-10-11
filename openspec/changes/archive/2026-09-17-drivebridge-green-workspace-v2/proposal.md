## Why

现有蓝色 Finder 与原绿色产品风格不一致；进入“我的文件”立即出现操作区，无法先辨别团队与存储。Dashboard 缺少常用入口，Key 的用户绑定和区分方式不直观。

## What Changes

- 统一绿色主题、边框、卡片、工具栏、表格和弹窗，参考 shadcn 的 Sidebar 分区和 icon collapse 交互，复用现有 React 组件。
- “我的文件”改为“我的网盘”：第一层按个人／团队／存储选择网盘，第二层才出现目录树与文件工作区；本地 view 同步两层入口。
- 主导航和目录树独立折叠并保存偏好；移动端为图标导航。
- Dashboard 展示实际容量、传输、Agent 状态和当前网盘最近文件；原分析面板继续提供日期筛选。
- 用户／团队独立入口；Key 搜索、状态筛选、稳定标识、用户映射、当前登录提示、命名、详情与已有撤销归档流程。
- 团队治理表单以可见用户选择器代替手填 ID。数据权限仍在服务端校验。

## Capabilities

### New Capabilities

- `green-workspace`: 网盘两层导航、统一绿色界面与身份关系展示。

## Impact

fit：视觉及交互归 client；split-owner：Key 元数据 API 归 backend-server/drivebridge。沿用 mcp/drivebridge 本地接口，不变更文件 ref 与 Provider。未启用团队模式时显示真实空状态，不能声称已配置团队。回滚只移除新入口和样式，原服务合同保持可用。
