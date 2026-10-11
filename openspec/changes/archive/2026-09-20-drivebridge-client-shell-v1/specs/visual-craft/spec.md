## ADDED Requirements

### Requirement: Shared surface tokens

客户端 SHALL 在 `src/style.css` `:root` 提供表面、边框、投影与半径令牌；Insights、Finder 与表单 SHALL 引用这些令牌，不得为卡片或图表另起一套 hex。

#### Scenario: Insight card uses shell tokens

- **WHEN** 用户打开资源态势
- **THEN** 指标卡与图表卡的边框和背景来自 `--line` / `--surface` / `--shadow-sm` 一类令牌，而不是组件内写死的另一套灰绿

### Requirement: Status pills and numeric alignment

状态与用量数字 SHALL 使用共享胶囊与 tabular nums；状态点不得用 emoji。

#### Scenario: Transfer dock uses tabular counts

- **WHEN** 顶栏显示传输数量
- **THEN** 数字以等宽数字渲染，状态点来自 CSS 而不是 emoji
