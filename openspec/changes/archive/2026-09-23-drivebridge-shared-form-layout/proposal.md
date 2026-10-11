## Why
机器人授权截图显示旧 checkbox 标签完全没有横向布局，嵌套字段又未被通用样式覆盖。这是多个表单共用样式缺失，需要统一修复控件语义布局与底部操作区域，而不是逐页重复补丁。

## What Changes
- 按控件类型统一弹窗字段和复选框布局，覆盖嵌套字段及触屏点击区域。
- 整理机器人授权、连接存储、新建 Key 和签发表单的取消及提交操作。

## Capabilities
### New Capabilities
### Modified Capabilities
- `shadcn-ui-primitives`: 共享表单布局。

## Impact
fit：客户端表单展示；复用 Base UI，不新增依赖或更改权限合同。
