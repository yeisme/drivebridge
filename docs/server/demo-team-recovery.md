# 本地团队体验恢复

本工具只处理明确授权的 loopback 开发体验实例，不能用于生产。当前升级保留原账号和个人空间，yeisme 测试网盘配额为 1 GiB。平台管理权不替代显式文件访问授权。

## 当前运行

前端预览入口为 `http://localhost:62044/`，代理后端 `127.0.0.1:31949`。后端状态目录在用户本地数据目录 `~/.local/share/drivebridge/yeisme-team-test-20260921`，包含私有 metadata、实例密钥及磁盘 objects，不应上传或提交到 Git。原体验进程及独立备份保留。

从 Server 项目目录恢复当前体验：

```bash
go build -o temp/bin/drivebridge-demo-resume ./cmd/drivebridge-demo-resume
./temp/bin/drivebridge-demo-resume \
  --state /home/yeisme-agent-dev/.local/share/drivebridge/yeisme-team-test-20260921 \
  --bootstrap-user fc49fd573b2cabb16a002085677f42486f7adbd09a776b2c \
  --listen 127.0.0.1:31949 \
  --web-dir /workspaces/yeisme-agent/cli/drivebridge/web/dist
```

必须先正常停止占用同一状态目录的恢复进程；不停止原有内存 fixture 来获取状态。恢复命令校验现有账号 ID，只操作已制作的副本；启动期间请求返回 503，直到所有版本校验及治理初始化完成。

## 初次升级流程

使用 `scripts/checkpoint_demo.py --source <当前状态目录> --output <新的私有副本目录>` 执行 SQLite 在线备份。工具检查无在途任务、不覆盖已有副本、不修改源数据。源 fixture 保持运行，以便读取其内存对象。恢复命令逐版本复制原存储字节，并核对解密后内容摘要和大小；失败不切换浏览器入口。所有版本通过后才修改副本的平台连接并写入恢复标记。

第一次切换前须再次比较源和副本的版本集合；源在复制期间新增文件时重新制作副本，不应忽略差异。完成切换后，后续恢复使用私有 objects 目录，不再依赖旧内存进程。当前已有七个原文件版本与一个授权上传验证文件，重启校验通过。

## 兼容与回滚

空间创建可选 max_bytes；省略保留实例默认值。space-quota 动作仅管理员可用，禁止低于 used + reserved；新增错误为 invalid_quota、quota_below_occupied。未启用显式管理员读取策略的旧部署保持原行为；本恢复命令明确启用该策略。

切换后如需回退，先对新状态（包括 objects）备份，不能直接回指旧实例而忽略新写入。仅支持平台存储的开发快照；不自动迁移其他连接。账号、密钥及内容不能进入日志和测试证据。
