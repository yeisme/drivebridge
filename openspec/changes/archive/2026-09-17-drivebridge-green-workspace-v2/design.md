# 绿色统一工作区

一级为总览、我的网盘、传输、产物、分享、存储、Agent、用户与团队、Key、回收站。我的网盘只呈现可访问空间卡片，明确团队及存储；进入具体空间后显示“全部网盘”返回、位置、目录树、文件列表和预览。切换团队通过既有 tenant/space context 重新挂载隔离状态，不能用客户端筛选模拟权限。

主导航折叠保留图标与无障碍名称，Ctrl/Cmd+B 可切换；目录树独立折叠。两项只保存非敏感 UI 偏好。移动端保留图标导航和可展开目录树。图片、视频、文件和传输预览沿用现有固定版本 Media 组件。

关系顺序为 Key → 用户 → team 成员关系 → 网盘权限。Key 的 ID 是可复制公开标识，不能拿来登录；明文依旧只出现一次。标签不改变用户名称或权限，跨用户转移 Key 不进入本次范围。治理模式仅展示本人的 Key 元数据；其他用户的 Key 数量标为当前可见范围。

| 场景 | 用户目标 | 产物／状态 | 验收 |
| --- | --- | --- | --- |
| 个人多存储 | 先选网盘再管理文件 | 卡片、目录树、列表与预览 | workspace-design + finder |
| 团队协作 | 清楚看到成员与空间授权 | 真实 team/成员/空间上下文 | Workspace unit + governance |
| Key 日常管理 | 按用途辨识及改名 | 只读标识、用户映射、标签 | key directory service + browser |
| Dashboard | 看容量、任务与近期素材 | 实际计量与当前网盘预览 | workspace-design + insights |
| 窄屏／本机 view | 不挤压文件区 | 图标侧栏、两层导航、独立树折叠 | mobile + local-insights |

验证沿用现有 Vitest、Playwright、Go 测试和 scripts/check.py。每次集成与浏览器验收写入本项目 temp/integration-test-runs，失败同样留存。团队与账号未启用、接口不可用、空网盘、无匹配 Key、已归档及失效凭据都有明确状态。
