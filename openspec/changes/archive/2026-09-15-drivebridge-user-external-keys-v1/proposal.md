## Why

外部工具预生成的密钥（任意前缀/字母表）无法导入：校验强制 `dbu_[0-9a-f]{96}`，bearer 路由也按 `dbu_` 前缀分表。运营方需要"自生成（已有）或导入外部预生成 key"两条路都通，并彻底移除 LiteLLM 耦合。

## What Changes

- 导入格式放宽：32–128 可打印 ASCII（无空白）即可导入，任意前缀；自生成仍 `dbu_`+96hex（一眼可辨来源）。
- bearer 路由改哈希优先：先查 UserKey，未中再查 Agent（删除前缀分支）。
- 跨表守卫：导入 key 哈希与 agents 表冲突时整批拒绝。
- `users rotate --id [--days]`：同用户原子换发（铸新+吊销旧，明文一次）。
- 去 LiteLLM：`scripts/issue_users.py` 删可选列与 env 门控；runbook 同步。

## Capabilities

### New Capabilities
无。

### Modified Capabilities
- `drivebridge-personal-service`：新增外部预生成密钥导入与密钥轮换需求。

## Impact

backend-server 拥有实现；SDK/客户端零变化（纯 Bearer 透传）。熵下限 32 字符为防爆破底线；两表查询各走唯一索引。
