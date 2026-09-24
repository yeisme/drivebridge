# 楔子试用操作卡

定位：个人把已有文件交给 Agent，再收回自己的存储。本卡只准备本机往返，不邀请外部用户、不采集遥测、不写机器状态文件。邀请 5–10 人仍按 [market-validation.md](market-validation.md)，需单独授权。

`setup` 完成、`doctor` 绿、`roundtrip_status=not_verified` **都不算**往返成功。成功只认：同一空间里上传 → `stat` 固定引用 → 下载校验 SHA-256。

## 本机闭环（必做）

替换尖括号路径。命令在用户终端执行，不要通过 MCP 打印凭据。

```bash
drivebridge auth
drivebridge config init
drivebridge setup --remote <rclone-remote> --root-path <已有目录> --local-root /absolute/local/project
drivebridge doctor --state <state> --rclone-config <rclone.conf>
drivebridge install --client claude-code
drivebridge mkdir --space personal --path outputs
drivebridge upload --space personal --file report.md --dir outputs --caller trial --task-ref task:trial-001 --json
drivebridge recent --space personal
drivebridge stat --ref 'drivebridge://file/<returned-id>' --json
drivebridge download --ref 'drivebridge://file/<returned-id>' --out report-copy.md
```

中断后查原操作再恢复，不换幂等键、不重新生成：

```bash
drivebridge operation --id '<operation-id>' --json
drivebridge resume '<operation-id>'
```

命名连接（消费方按名字引用；空注册表时 list 仍投影 synthetic 默认连接）：

```bash
drivebridge connection list --json
```

## Web 只核对这四页

服务端或 fixture 打开后，只把下面四页当楔子面。团队/治理页可以存在，本卡不测新能力。

- `#/files`：网盘与 Finder，核对版本与引用
- `#/transfers`：原操作进度、取消、恢复
- `#/inbox`：产物是否按版本出现
- `#/agents`：签发与撤销，默认只读期限

本机只读视图：`drivebridge view`（loopback，无写入口）。

## 记录什么

手写笔记即可，不要把文件正文、token、临时 URL 写进仓库或证据目录。

- 安装到首次 `setup` 的时间
- OAuth / rclone 等待
- 第一次 upload→stat→download 是否在 10 分钟内
- 人工搬运次数（拷到对话、另存为）
- 停在哪一步、错误码是什么

阻断扩大试用：越权、静默覆盖、错误报告成功。
