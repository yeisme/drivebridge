## Why

绿色工作区已经能先选网盘再进文件，但产品标、侧栏图标和 Finder 操作语言还没对齐：安装图标是桥，应用里却是 lucide Files；折叠导航有三处相同的 UsersRound；右键菜单只有文字。用户每天盯着这些表面，现在收口视觉与交互，不改存储合同。

## What Changes

- 产品标（桥 + 文件签）进入侧栏、登录页、favicon 与 PWA（含 maskable 安全区），并按尺寸提供可辨认的光学简化。
- 主导航每个入口使用不同 lucide 图标；文件类型映射合并为一处，音频/PDF/文本不再都显示成文档。
- 收口 CSS 令牌，去掉 `style.css` 与 `finder.css` 互相覆盖的 `:root`；操作页去掉英文 eyebrow。
- Finder 菜单增加图标、分组、快捷键展示；粗指针改为底栏 action sheet。
- 列表选择不再依赖常驻 checkbox；拖拽目标可见；空状态带上传动作；移动端主操作只留上传。
- 键盘补齐：方向键进文件夹/回上级（现有复制剪切粘贴/F2/Delete 保留）。

## Capabilities

### New Capabilities

- `visual-craft`: 产品标、图标语言、令牌、Finder 菜单与选择/拖拽/键盘的可见交互。

### Modified Capabilities

- `green-workspace`: 折叠侧栏与网盘工作区继续成立，但品牌标、导航图标和进入网盘后的身份条密度按本 change 更新。

## Impact

fit：仅 `client/drivebridge-client` 视觉、静态图标资产、Finder/壳交互与 Playwright 截图。不改 backend-server/MCP API、文件 ref、权限或 Key 明文规则。无新 npm 依赖。回滚只还原前端资源，服务合同保持可用。
