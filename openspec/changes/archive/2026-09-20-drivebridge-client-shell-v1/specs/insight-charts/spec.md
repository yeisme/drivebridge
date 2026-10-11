## ADDED Requirements

### Requirement: Lightweight honest charts

用量图表 SHALL 用项目内 SVG 原语绘制，不引入图表库。柱顶可有小圆角，背景可有细网格；缺失日 SHALL 显示缺口而不是零值。有效量与观测量并排时不得叠成单一总量。

#### Scenario: Missing day stays a gap

- **WHEN** 某日值为 `null`
- **THEN** 该日不画成零高度柱，并在读数或表格中标为未采集

#### Scenario: Effective and observed bytes stay side by side

- **WHEN** 传输图同时展示有效量与观测量
- **THEN** 两根柱的 x 不同，不堆叠为一个高度

### Requirement: Follow tooltip without hiding the table

图表 SHALL 在指针悬停或键盘聚焦时显示跟随浮层，列出日期、分项与格式化数值；数据表替代 SHALL 仍然可用。浮层不是唯一读数通道。

#### Scenario: Keyboard focus shows the same numbers as hover

- **WHEN** 用户把焦点移到某日柱组
- **THEN** 浮层或等效 live region 读出该日各分项，Enter 仍触发既有钻取

#### Scenario: Data table remains

- **WHEN** 用户展开「查看数据表」
- **THEN** 可见与图表相同的日期与未采集/数值单元格

### Requirement: Segmented capacity without fake totals

容量展示 SHALL 能按当前文件、历史版本、回收站、预留分段；配额未知时不得把进度画成 100%。未知分段保持空缺文案。

#### Scenario: Unknown quota is not a full bar

- **WHEN** 空间 `quota_bytes` 为空
- **THEN** 分段条只显示已知分段比例或明确「配额未知」，不渲染满额进度
