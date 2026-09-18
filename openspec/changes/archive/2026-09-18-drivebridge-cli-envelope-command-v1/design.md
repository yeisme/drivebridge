# 实施注记

- 派生函数：`commandAction(cmd)` = TrimPrefix(CommandPath, "drivebridge ") 后空格转点，再过覆盖表 `{"space.add":"space-add","storage.add":"storage-add"}`（保持既有成功 action 字面量不动）。
- 隐藏叶 `space-add` 的 CommandPath 无空格，派生即 `space-add`，与成功 action 一致，无需映射。
- `remote` 单叶命令带 `--action`：失败身份留在命令级 `drivebridge.remote`（PreRun 阶段 action 未知），成功为 `drivebridge.remote.<action>`——按命令粒度诚实，不算漂移。
- mount 归一后 `testdata` 无需改动（无旧值断言）；storage.txt 的失败断言（`drivebridge.storage-add`/`drivebridge.storage.probe`）由派生+映射天然满足，storage.go 中手工 `c.command` 赋值删除以免双源。
- deviceauth 两处：token 落盘与 drive/root 选择落盘；`Data().Save()` 失败返回 `config_write_failed` 语义（沿用注册码）。
