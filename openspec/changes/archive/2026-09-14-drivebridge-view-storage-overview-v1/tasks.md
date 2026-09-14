## 1. 实施与验证

- [x] 1.1 view.go `/api/v1/spaces` 投影增补 `backend`（`Space.Kind()` 归一）
- [x] 1.2 view_test.go 覆盖 OneDrive（Backend 空）与 generic（s3）两类空间的投影与只读边界
- [x] 1.3 `openspec validate drivebridge-view-storage-overview-v1 --strict --no-interactive`

## 2. 实施与验证

- [x] 2.1 client 统一存储总览消费 `backend` 字段（归 client owner change，本任务仅登记交接）

验证证据：`temp/integration-test-runs/20260914T153248Z-957211c5/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
