## ADDED Requirements

### Requirement: 同仓 Electron 宿主复用既有客户端
DriveBridge SHALL 在同仓 desktop 模块使用 Electron/Forge，复用 web UI 与 Go application service，提供随包版本匹配的 binary，不要求云登录、预装 CLI 或手工启动服务器。

#### Scenario: 离线首次启动已安装应用
- **WHEN** 用户未安装 CLI、未登录云账户且网络断开
- **THEN** 应用可启动并选择已有本地 vault，不初始化或同步数据

### Requirement: 沙箱与最小 IPC
宿主 SHALL 禁 Node integration、开启 context isolation/sandbox/webSecurity，通过固定逐方法 contextBridge 与主 frame/origin/参数校验操作，不暴露通用 shell/fs/fetch/IPC。

#### Scenario: 不可信笔记尝试提权
- **WHEN** Markdown、HTML、SVG 或子 frame 发送任意命令/文件读取/危险导航请求
- **THEN** 请求被拒绝，不能接收 bearer、任意路径或执行代码；正式包不使用 no-sandbox

### Requirement: 前台会话与秘密引导
宿主 SHALL 使用固定随包程序、私有stdin bootstrap、脱敏stdout lifecycle和父EOF排空协议，仅管理自有进程，不注册daemon或系统自启。

#### Scenario: 窗口退出或父进程崩溃
- **WHEN** 会话管道关闭或退出超过排空时限
- **THEN** 自有子进程关闭或由自有进程树清理，用户另行运行的Hermes/Pinax不被终止，秘密不进入日志

### Requirement: 三平台安装与安全验收
首片 SHALL 在macOS 14+ arm64、Windows 11 x64、Ubuntu 24.04 LTS x64的真实系统验证安装、启动、离线阅读与数据保留，Linux覆盖X11/Wayland；签名发布独立报告。

#### Scenario: 只有构建或Web测试通过
- **WHEN** 任一平台尚无实机证据或须关闭系统安全才能运行
- **THEN** 对应平台门保持未完成，不以编译、截图或未签名开发运行冒充完整桌面/发布验收
