## Why

LiteLLM 单钥桥接判定过重，运营回归最朴素的填表流：一张表进、一批 key 出。现有 `users import` 只收 stdin JSON，操作员要手工拼 JSON、再从终端抄 key——批量开通几十个用户时摩擦明显。

## What Changes

- `users import --csv`：stdin 收 CSV（带表头 `name,days,admin,key` 或无表头按位 `name[,days[,admin[,key]]]`；单列=纯名单），空行/BOM 容忍。
- `users import --out <file>`：把签发结果写成 0600 分发 CSV（`name,user_id,key,admin,expires_at`），明文仍只落在操作员指定的一处。
- `scripts/issue_users.py`：一表批量开通的可选包装——本地 CLI 发 dbu_ key；设 `LITELLM_URL`+`LITELLM_MASTER_KEY` 时同表加发 LiteLLM key（`POST /key/generate`，纯脚本调 Admin API，零运行时耦合），输出合并 CSV `name,drivebridge_key,litellm_key,expires_at`。
- 部署 runbook：填表批量开通配方 + CSV 模板。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-personal-service`：`users import` 输入面扩展 CSV 与分发文件输出（JSON 路径不变）；不新增在线面。

## Impact

backend-server 拥有实现；CLI 纯增量；LiteLLM 交互只存在于操作员脚本（stdlib urllib），产品代码零依赖。
