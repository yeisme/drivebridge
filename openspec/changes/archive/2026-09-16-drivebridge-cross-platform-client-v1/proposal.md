## Why

用户设备横跨 Linux/macOS/Windows 与手机。client 的跨平台答案是「浏览器即 client」：不建桌面/原生壳，一份 web 资产覆盖服务端部署与本机 `drivebridge view`；缺口在 Windows 本机分发（连编译目标都没有）与移动端可安装体验。

## What Changes

- 设计文档 `docs/cross-platform-client.md`：平台矩阵、Windows 实机验收门（state 权限语义/AF_UNIX 隐私/浏览器启动/rclone+ffprobe/锁等价，全部如实未勾）、移动端定位（服务端网页唯一官方路径；本机 view 恒 loopback 不开 LAN）、桌面壳评估（不做）。
- **PWA 可安装**：`public/manifest.webmanifest` + SVG 图标 + `index.html` link——服务端部署后手机/桌面浏览器可安装为独立窗口应用，零新运行时；iOS `apple-touch-icon`（需 PNG）记为后续。
- 配套 mcp owner change `drivebridge-cross-platform-distribution-v1`（windows 分发目标、平台锁抽象、三平台浏览器启动）。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-personal-web`：ADDED「跨平台可安装」需求（manifest 如实描述、桌面/移动同资产、不引入新壳）。

## Impact

纯增量：一个静态 manifest 与图标，不改任何 API/会话/安全行为；本机 view（insights.html）不加 manifest（一次性启动语义与安装冲突）。
