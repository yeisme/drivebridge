# 旧项目入口清理验收

用户在单仓迁移后要求清理原项目。已移除工作区 mcp/drivebridge、backend-server/drivebridge、client/drivebridge-client 三个软链接，以及本地 Git 中对应的旧 submodule 注册。父仓索引记录旧入口删除和 cli/drivebridge 单一子模块；未提交、推送或部署。

当前代码仍全部在统一 owner：根 CLI/MCP、server、web。原 Server/Web Git 备份可读取 HEAD commit；模块没有嵌套 Git。当前路由、文档、命令、active 规格及任务 owner 路径已更新；任务文字通过 openspec-tasks.py amend 修改，原编号、勾选与 evidence 后缀保留。前端 37 项、后端 25 项新工作台实施任务仍全部未完成。

## 本轮证据

以下路径均相对统一仓库 temp/integration-test-runs/：

- `20260930T154402Z-7b21e84c`：5 项 workspace 编排普通测试通过。
- `20260930T154545Z-432404ee`：旧入口不存在、旧本地注册消失、同一 Git/Skills/OpenSpec、原 Git 备份可读、62 项新工作台任务状态保留，570 个当前项目相对链接无缺失。
- `20260930T154545Z-cfe307d6`：DriveBridge MCP 静态合同定向校验通过，发现 40 个工具；不代表实际云或远端运行验收。
- `20260930T154545Z-d9645b40`：当前 78 项 base spec/active change strict 校验通过。

MCP 校验首次失败记录 `20260930T154402Z-07f2eb75` 保留：旧脚本仍要求根 runtime 的 drivebridge-operator，以及当前 Skill 已不存在的 doctor/mkdir 文本。已改为检查统一 owner 的实际 Skill discovery 合同（tools/list、inputSchema、capabilities、operation_get）；CLI doctor、只读探测和 ready true/false 的源码检查继续保留。

根级非 DriveBridge 的 11 个历史死链未改动。本轮只涉及目录入口、管理配置、文档及源码注释；没有重跑产品全量测试，也没有修复或宣称修复上轮已复现的 Server 四处基线失败。未运行 race，前端重建仍待后续任务实施。
