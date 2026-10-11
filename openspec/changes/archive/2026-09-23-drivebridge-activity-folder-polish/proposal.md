## Why
最近活动直接展示协议动作，创建文件夹的勾选项又被通用表单 grid 样式拆成两行，影响理解和操作。

## What Changes
- 复用审计中文动作字典，区分请求、成功和拒绝，保留原始动作提示。
- 新建弹窗宽度 480px，保存位置和创建选项分组，修复通用复选框布局。

## Capabilities
### New Capabilities
### Modified Capabilities
- `insights-audit`: 最近活动共享中文语义。
- `finder-file-actions`: 新建表单可读性。

## Impact
fit：全部归客户端；服务端接口、权限和审计事件不变。
