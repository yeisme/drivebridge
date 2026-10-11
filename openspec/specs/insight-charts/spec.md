# insight-charts Specification

## Purpose
定义资源态势使用的轻量 SVG 图表、容量分段与数据表合同，区分已观测零值、未采集历史和未知配额。图例、坐标、浮层与快照根据可用内容宽度排版，并支持键盘读取和下钻。

## Requirements

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

### Requirement: Capacity layout follows available content width

容量趋势与历史快照 SHALL 在实际内容区达到 880px 才采用双栏，较窄时上下排列。快照 SHALL 分行显示名称、占用、额度和时间；图例可换行，坐标字号至少 12px，提示浮层不越界，数据表替代保持可用。

#### Scenario: Expanded sidebar at 957px viewport

- **WHEN** 957px 窗口展开主侧栏，容量内容区不足 880px
- **THEN** 趋势与快照单列排列，长名称和时间不互相覆盖，页面没有横向滚动

#### Scenario: Edge tooltip and long snapshot name

- **WHEN** 用户聚焦首末日期或查看长名称快照
- **THEN** 浮层保持在图表边界内，快照名称可取得完整文本，日期和数值可通过键盘及数据表读取

### Requirement: Observed zero is distinct from missing history

容量图 SHALL 区分真实零与未采集；已知全零范围显示零基线，不生成 `0.5 B` 一类没有解释价值的刻度，缺失日期保持空缺而非密集圆点。采集开始、保留规则与清理信息 SHALL 在统计说明中可查。

#### Scenario: Zero and null coexist

- **WHEN** 某日容量为 0、前一天容量为 null
- **THEN** 零值日有零基线及 0 B 读数，null 日显示未采集，不假装存在历史观测

#### Scenario: Collection details remain accessible

- **WHEN** 用户展开统计说明
- **THEN** 可查看采集和保留口径，收起时只保留简洁更新时间，不删除统计定义
