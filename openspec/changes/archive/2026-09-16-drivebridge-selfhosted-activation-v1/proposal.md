# drivebridge-selfhosted-activation-v1

## Why

全部能力仍是本地候选：真实部署、真实存储、真实账号均未验收。操作者已决策首验形态——Tailscale 私网暴露、key-only（无 OIDC）、仅平台 S3、长期运行服务器、实例即租户的多租户分片。但 `docs/deployment.md` §6 的四步 key-only 序列**无法用真实二进制离线彩排**：fixture 的 S3 只活在进程内 httptest 上，`platform-connect` 又固定以 `development=false` 打开服务、拒绝回环 HTTP 端点。直接拿真实服务器当第一次试运行，等于把可离线发现的配置/序列错误留到最贵的地方暴露。

## What Changes

- `cmd/drivebridge-fixture` 新增 `--s3-listen <addr>`：把内置 S3 fixture 以独立回环监听暴露（endpoint 打 stderr），仍为纯测试命令，不进 serve 路径。
- `platform-connect` 新增显式 `--development` 旗标：与 `serve --development` 同一纪律（只能显式命令行、不进配置文件/环境变量），允许回环 HTTP fixture 端点，供离线彩排；生产路径行为不变。
- 新增 `scripts/selfhosted_rehearsal.py`：用真实 `drivebridge-server` 二进制 + 真实 client `dist` + 回环 S3 fixture 走一遍四步 key-only 激活（platform-connect → users import → serve --enabled → status 远程核对），再做浏览器等价 API 冒烟（`/auth/mode`、`/auth/key` 换会话、建空间、上传/下载小文件、配额口径）与备份只读校验；证据按仓库标准落 `temp/integration-test-runs/<run-id>/`。
- `docs/deployment.md` 增补「自托管激活（Tailscale + key-only + S3-only）」章节：部署形态决策、首验门与回滚，真实部署执行步骤单列并标注需要操作者输入（服务器访问、Tailscale、S3 凭据）。

## Non-goals

- 不接 OIDC/issuer，不接 OneDrive，不公网暴露，不设计费与组织后台。
- 不改动上传/下载/配额/加密等业务合同；不引入新依赖。
- 本 change 不执行真实服务器部署（需要操作者提供访问与凭据，单独授权）。

## Capabilities

### Modified Capabilities

- `drivebridge-personal-service`：ADDED 需求 Key-only activation rehearsal；MODIFIED 需求 Configured deployment surface（platform-connect 显式 development 场景）。

## Impact

backend-server/drivebridge（fixture 测试命令、platform-connect 旗标、scripts、docs）；client/drivebridge-client 只作为彩排的被构建产物消费，不改代码。默认行为不变：不加 `--development` 时 platform-connect 仍拒绝回环/非 HTTPS 端点。
