## 1. 实施与验证

- [x] 1.1 internal/config（viper server 变体）：发现序含 /etc、未知键与形状校验、优先级、Int/模板
- [x] 1.2 cobra 六命令树+config show/init+completion；AddCommand 指针化（值复制断 parent 链坑）
- [x] 1.3 doctor config_file 检查 + fixture 冒烟（config 驱动 serve→healthz 200）+ 文档（deployment.md 首启改 config 流）
- [x] 1.4 全门禁：race/vet/CGO-free/既有套件零改动/CLI 级配置测试

验证证据：`temp/integration-test-runs/20260912T060023Z-8bb25c77/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
