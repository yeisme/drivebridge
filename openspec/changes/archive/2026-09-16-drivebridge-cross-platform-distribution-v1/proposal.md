## Why

用户的工作环境横跨 Linux/macOS/Windows；本机 client（`drivebridge view`）此前只有 Linux/macOS 分发，Windows 连交叉编译目标都没有（`syscall.Flock`/`syscall.Access` 直接阻断编译）。跨平台 client 支持需要本机分发面先补齐编译与启动层。

## What Changes

- **平台抽象层**：单进程 state 锁与 doctor 可写探测抽为 `platform_unix.go` / `platform_windows.go`（Windows 用 `LockFileEx`/`UnlockFileEx` 字节范围锁；可写探测用瞬时探针文件，`syscall.Access` 不存在于 Windows）。Linux/macOS 行为零变化（既有 flock/Access 路径）。
- **view 浏览器启动三平台**：darwin `open`、windows `cmd /c start "" <url>`（start 的首引号参数是窗口标题）、其余 `xdg-open`。
- **分发目标**：`build.py` 新增 windows/amd64、windows/arm64，zip 打包（`drivebridge.exe` + web 资产），manifest 状态 `local-candidate-windows-unverified-on-machine`——**交叉编译通过≠实机验收**。
- 实机门保持未勾：state 目录权限语义（Windows ACL vs `Perm()&0077` 检查）、socket 隐私（`os.Chmod` 对 Unix socket 在 Windows 的语义）、rclone/ffprobe 在 Windows 的可用性，全部见 `client/drivebridge-client/docs/cross-platform-client.md`。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `insights-audit`：ADDED「跨平台启动与分发」需求（按平台启动浏览器、Windows 分发候选诚实标注、不宣称实机可用）。

## Impact

Linux/macOS 行为不变（同 API、同错误码）；Windows 是编译+打包候选，不进入默认构建列表；无新依赖（LockFileEx 经 LazyDLL 直调，不引入 x/sys）。
