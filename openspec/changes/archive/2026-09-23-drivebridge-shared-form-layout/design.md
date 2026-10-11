## Context
旧 checkbox 类名不一致，表单直系 label 的选择器漏掉 fieldset 与双列内的字段。

## Goals / Non-Goals
统一布局并保留表单提交语义，不重写表单状态管理和权限。

## Decisions
AppDialog 引入局部 components 层样式，通过直接子控件识别普通字段与勾选标签；不依赖历史类名，不全局覆盖 label。嵌套输入上方显示标签，间隔 8px；选项横排间隔 12px，触屏至少 44px。复杂对话框保留原宽度，授权表单限制 480px。

## Risks / Trade-offs
共享样式影响分享、存储和用户表单，使用完整浏览器回归及专用主题/尺寸检查验证。协议、权限、默认只读范围不变。

## Validation
专用 fixture 检查复选框与文本几何、嵌套字段间距、取消焦点恢复及三种宽度两主题截图；实际授权流程继续验证只读范围。

样式文件必须首先声明项目统一的 theme/base/legacy/components/utilities 层顺序，避免模块导入先后改变全站优先级。回归已发现并修正该问题，使用既有概览行尺寸测试防止复发。
