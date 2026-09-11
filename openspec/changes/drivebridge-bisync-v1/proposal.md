## Why

用户真实工作流是"本地目录 ⇄ 云端空间"长期共存：创作素材在本地生成、Agent 上传到 OneDrive、成品回存，再人工对齐两边。缺少受控双向同步时，对齐靠记忆和手工复制。rclone 内嵌 bisync 引擎（v1.70.3 `cmd/bisync` 可作库），包上 DriveBridge 的操作日志与不可变版本语义即可安全提供。

## What Changes（设计，实现归后续波）

新增同步空间能力：`drivebridge sync` 命令对绑定 local root 与远端根的空间执行双向同步；引擎复用 rclone bisync，外裹 Operation 日志（kind=sync）；冲突=keep-both 加后缀；首跑强制 resync 守卫；MaxDelete 收紧；调度延期，首版手动触发。

## Capabilities

### New Capabilities
- `drivebridge-sync`：双向同步的合同（安全守卫、冲突、恢复、报告）。

### Modified Capabilities
无已发布合同删除；软链接行为引用 generic-backends-v1 的 Symbolic link policy。

## Impact

新命令与 Operation 新 kind；不改 upload/download 现有语义。同步产生的远端文件与手工上传同样进入稳定引用登记。跨进程不承诺增量续传（对齐现行"允许整文件重传"决策）。
