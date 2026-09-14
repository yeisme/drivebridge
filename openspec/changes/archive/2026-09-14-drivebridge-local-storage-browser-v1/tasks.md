## 1. 实施与验证

- [x] 1.1 socket `POST /list`（ListRequest、60s 超时、500 截断 partial、错误码映射）
- [x] 1.2 view 代理路由 `POST /api/v1/local/list` + `requestList` 委派（socket 缺席 503 不撞锁）
- [x] 1.3 `QueryInsights` files kind：`dir` 过滤（参数在且空=根）+ `dir/mime_type/observed_at/trash_path` 投影
- [x] 1.4 测试：insights_socket_test.go（实时+索引 remember、dir 投影、错误矩阵、截断）；view_test.go（CSRF 门、代理、owner 缺席 503）
- [x] 1.5 `go vet` + `go test ./...`（证据 temp/integration-test-runs/）+ `openspec validate drivebridge-local-storage-browser-v1 --strict --no-interactive`

## 2. 实施与验证

- [x] 2.1 client 存储浏览 tab 消费两级接口（归 client change `drivebridge-unified-storage-visualization-v1`，本任务仅登记交接）

验证证据：`temp/integration-test-runs/20260914T153248Z-957211c5/summary.json`（相对项目根目录）。由 repository evidence runner 生成，任务状态由本命令写入。
