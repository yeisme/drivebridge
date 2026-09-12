## 1. 实施与验证

- [x] 1.1 internal/config（viper）：发现序/严格未知键/优先级解析/模板
- [x] 1.2 cobra 命令树重构（space add/list、config show/init、持久输出旗标、space-add 别名）
- [x] 1.3 doctor config_file 检查与文档更新（README 配置节+activation-local §5）
- [x] 1.4 全门禁：race/vet/CGO-free/stdio 冒烟/testscript 配置全链

验证证据：`temp/integration-test-runs/20260912T055022Z-428b9422/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
