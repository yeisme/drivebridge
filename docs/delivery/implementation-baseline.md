# S0–S4 实施基线

本轮开始实现已确认的设计，不改S5边界。当前分支为develop；工作树包含大量已修改及未追踪的迁仓源码、文档、脚本和运行时Skill。保留这些文件，不reset/clean/stash，不凭HEAD复制旧版本覆盖当前内容。

## 范围与执行顺序

两套个人工作台change共74个任务，其中B14.1/B14.2/F18.1/F18.2为S5，保持待实施，不在本目标勾选。其余70项按实际依赖推进；目标里程碑不是这些任务的另一套完成证据。先B01.1合同盘点，再B01.2/B01.3生成与漂移门；前端F01保护清单和focused基线不能用设计检查替代。

当前源码定向查找未发现agent-grants、inbox-submissions、AgentGrant或personal_workspace_v1实现；server/internal/contract及contracts目录尚不存在。不能将设计字段与源码已有旧Agent/普通上传混为一谈。现有server SDK已有上传批次、组织、分析等类型；后续复用旧类型或显式适配，不全量重写。web/src/app尚不存在，新壳仍待实施。

已发现并保留并发更新：B14.2/F18.2文本追加Electron桌面和D任务交接。这些属于当前目标外的S5/桌面工作，不删除、不修改、不回退成“框架未定”，也不扩充本目标为桌面实现。后续任务CLI只修改本轮负责的单个ID。

## 工作区与运行边界

遵循当前owner docs/development.md明确的同checkout例外，避免独立worktree丢失尚未提交的迁仓文件与最新设计。CONTRACT阶段不启动数据库或服务；BACKEND/INTEGRATION使用独立可丢弃state及空闲loopback端口，绝不使用现有用户state或停止未知进程。进入真实运行前再记录所用端口与进程归属。

本轮不提交、推送、发布或运行race。普通实现先focused tests，最终稳定后才执行全量门。证据统一由根scripts/check.py生成；不把嵌套模块遗漏当全项目测试通过。

## 文件保护与责任

- server/internal/service及sdk：保留全部旧文件/身份/上传合同；新合同先新增窄模块，再按应用边界接线。
- web/src：旧main、API、阅读器、恢复存储与权限反例均先记录和测试，不能直接整体覆盖；独立分享/本机入口继续保护。
- docs及openspec：复用现有设计包，状态和任务正文只通过scripts/openspec-tasks.py更新；保留历史证据。
- 根internal/bridge、internal/remote：当前不修改；待明确S2适配与S4兼容任务进入时再做定向修改。
- desktop、其它change、运行时Skill：无本轮写入权限范围，保留并发工作。

这是实施前盘点，不是B01/F01或业务验收完成证明。下一步读取现有公共DTO、错误与上传/授权合同，创建共享合同生成器的focused测试。
