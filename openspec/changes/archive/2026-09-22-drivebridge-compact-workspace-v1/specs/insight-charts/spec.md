## ADDED Requirements

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
