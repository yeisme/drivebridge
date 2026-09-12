## 1. 实施与验证

- [x] 1.1 Provider 目录扩展（OneDrive Graph path 寻址/逐段 mkdir；generic rclone List/Mkdir）与 Service remoteDir 校验
- [x] 1.2 MCP 工具面：list path、mkdir、upload dir
- [x] 1.3 CLI 传输命令（upload/download/ls/stat/operation/mkdir，确定性幂等键）+ install --client 三态
- [x] 1.4 全门禁：fixture 目录全链+testscript 配置驱动 CLI+race/vet/CGO-free+stdio 冒烟

验证证据：`temp/integration-test-runs/20260912T133712Z-368e720a/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
