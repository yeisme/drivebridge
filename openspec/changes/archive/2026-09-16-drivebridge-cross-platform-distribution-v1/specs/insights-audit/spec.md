## ADDED Requirements

### Requirement: 跨平台启动与分发

本机视图与 CLI 的平台相关行为 SHALL 按操作系统分派，Windows 分发 SHALL 以「未实机验收的候选」诚实标注。

#### Scenario: 按平台启动浏览器
- **WHEN** 用户在 darwin/windows/linux 运行 `drivebridge view` 且未传 `--no-open`
- **THEN** 分别经 `open` / `cmd start` / `xdg-open` 打开私有启动链接；启动失败时回退提示使用私有启动文件，不打印 URL

#### Scenario: Windows 分发候选
- **WHEN** 发行脚本构建 Windows 目标
- **THEN** 产物为 zip（`drivebridge.exe` + web 资产 + 文档），manifest 状态标注未实机验收；Linux/macOS 产物与既有 tar.gz 完全一致

#### Scenario: 单进程锁跨平台等价
- **WHEN** 同一 state 已被持锁进程占用
- **THEN** 第二个进程的非阻塞加锁在 Unix（flock）与 Windows（LockFileEx FAIL_IMMEDIATELY）同样失败，不静默共享 state
