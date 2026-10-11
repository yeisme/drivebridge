## Decisions

复用 WorkspaceDialog 的 className 扩展，让管理弹窗在桌面宽至 960px，在移动端留 8px 边距；内部面板宽度为 100%，避免两个独立 vw 宽度造成溢出。

UserDirectory 复用现有身份目录入口，查询使用服务端分页并在筛选/租户变化时中止旧请求。Key 详情精确对应所选用户，不再让每行跳到同一个本人 Key 页面。Key 数量 null 明确显示信息受限。

每个危险动作先取影响与账号 revision。身份确认密钥只停留在输入状态，不写浏览器存储，关闭及成功后清除。409 阻止重复确认，要求重新查看影响。删除明确标为归档且保留文件，恢复不自动启用旧凭据。

## Validation

复用 Go 服务 fixture 和 Playwright，覆盖筛选、Key 详情、冻结会话失效、解冻、归档、恢复；截图只在秘密输入关闭后记录。1110/868/390px 检查对话框 scrollWidth 不超过 clientWidth、表单不超出边界。运行既有 Vitest 和构建，证据由 scripts/check.py 生成。
