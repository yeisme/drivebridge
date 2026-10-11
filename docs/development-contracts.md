# 共享合同生成：当前实现切片

当前完成授权/组织/收件创建/标签建议相关三十五个schema组件、九个Go类型化响应样例，以及二十七个规划操作（十九条path）。新增grant/分组/组织列表分页和分组CRUD合同。包括grant创建、查询/更新、撤销、轮转与metadata lookup，收件创建/lookup，标签建议及组织更新。它们尚未接入HTTP/SDK/MCP业务端点；OpenAPI路由明确标记planned，不代表B01已经完整交付。现有旧API与SDK保持不变。

从cli/drivebridge执行：

```bash
go -C server run ./cmd/drivebridge-contracts openapi
go -C server run ./cmd/drivebridge-contracts fixtures
make contracts
make contracts-check
python3 scripts/check.py -- node web/scripts/check-contract-schemas.mjs
python3 scripts/check.py -- go -C server test ./internal/contract ./cmd/drivebridge-contracts -count=1
```

Go DTO是真源，TypeScript输出至web/src/generated/personal-workspace.ts，不可手改。请求Field保留缺失/null/值三态；schema将其展开为公共类型而非泄漏Go内部Present/Null/Value字段。期限输入仅允许never或六档duration；响应expires_at和expiry_changed_at必返可空，never/duration与截止时间的组合也有约束。组织group_id显式null允许解绑，pinned的null拒绝。

Ajv正负例验证Go生成的schema与样例；openapi-typescript生成类型并可做只读比较。依赖通过pnpm新增固定开发版本，不进入生产业务代码。JS验证脚本里的负例是测试输入，不是第二套公共fixture真源。

已验证该切片：schema正负例、Go定向测试、生成一致性、生成TS独立strict检查。新增收件创建拒绝path/file_id/grant_id/purpose等客户端越权字段，标签建议限制条数与每项长度。它们目前仅为合同校验，HTTP强制限制仍待实现。创建/轮转响应使用issued或not_replayable的互斥信封；metadata重放携带secret会校验失败。路由登记区分human-only与inbox.submit/labels.suggest，以及201创建和200回查，不给无路径参数端点虚构id。DecodeRequest拒绝未知字段、null根及尾随JSON，错误不暴露部分解码结果；必填字段缺失或null、重复顶层JSON键、大小写字段别名现在也拒绝；合法revision=0保留。字段值校验和资源权限仍由应用验证负责。

漂移Vitest首跑因文件指定Node环境而既有全局setup使用window失败；已移除本测试的环境覆盖，复用项目默认jsdom，2个测试实际命中通过；未修改全局setup或删除断言。失败证据保留在temp/integration-test-runs/20261001T102443Z-3c69950b，修复通过在20261001T102511Z-d6c46a6a。没有运行全Web类型检查或业务集成，因此不能声称现有应用全绿。

分页合同区分unknown/null与exact/整数，默认limit=50、上限200，包含cursor失效410；列表里的grant同样保留期限互斥验证。已定向验证现有HTTP成功数组信封与错误信封可由新DTO读取，原错误状态/脱敏测试通过，证据temp/integration-test-runs/20261001T104523Z-a9d98724。新增真实本地fixture测试固定旧 /agents 默认7天、31天拒绝、成功包含agent/token以及GET列表保持数组，证据temp/integration-test-runs/20261001T105140Z-1d691b8f。这仍不是旧Agent全部行为或完整SDK/MCP兼容证明。

Go与Web测试开始直接读取同一份生成响应fixture；Go检查落盘字节与类型化样例一致，Web消费grant、不可重放凭据、staging与分页。首次新增用例分别遇到Vitest import.meta URL非file及Go相对路径多退一级，均已定向修复且保留失败记录；没有修改业务规则。最后Go与3个Web测试通过，见20261001T105537Z-324889cb、20261001T105538Z-0792da1b。分页请求解析新增边界、重复参数与opaque cursor保护测试。

新增CreateGrant/PatchGrant/PatchOrganization值校验：拒绝空白名称、重复/未知动作、禁止字段null及非法期限组合，保留false与空prefix的合法含义。此层只检查输入值；目录规范化、权限交集、CAS和生命周期仍须应用服务校验，尚未接到HTTP处理器。定向Go测试与生成/schema复验通过，见20261001T105843Z-de95ebf1及20261001T105842Z-542e2fc6。

B01.1基线盘点已通过任务CLI登记完成，清单见docs/architecture/workspace-contract-inventory.md；B01.2/B01.3仍未完成。invalid_expiry已在现有HTTP错误映射增量登记422，先失败后修复且旧测试保持；SDK现在通过httptest读取相同生成fixture并验证不可重放响应和typed error，证据20261001T111241Z-a1a657dc；合同/生成CLI/SDK包复验见20261001T111532Z-5ce9b887。以上不意味着新业务路由已上线。

收件/标签建议的读取路由已显式登记human-visible或Agent-own读策略，路由测试要求每个操作必须有human/action/read-policy之一。标签采纳请求新增Go与schema一致的accept非空子集/reject无标签、唯一值与长度约束；真正子集关系和人工权限仍由service验证。条件schema曾因缺少array类型被Ajv strict拒绝，补足类型后通过，未关闭strict；失败记录20261001T112400Z-e7f9bcff，正负例通过20261001T112534Z-1877e207。

归档合同现明确返回202及原operation_ref，目标file_ref必返可空，未发布不能当归档完成；设计HTTP表已同步。最新生成/schema/Go与SDK定向检查通过，证据20261001T112808Z-db5653ac与20261001T112808Z-76b6feaf。仍无新增业务HTTP处理器。

仍待B01补齐：完整DTO覆盖及其余路由/身份/错误登记、更多业务fixture及漂移负例、旧接口golden与HTTP/SDK/MCP同源消费验证。OpenAPI与fixture已由生成脚本落盘，make contracts/contracts-check已存在，不能将部分覆盖的命令通过视为B01全部完成。当前脚本已比较OpenAPI、Go响应fixture和TS三类受管文件；Vitest负例在临时目录验证缺失、手改各类产物会失败且不覆盖，恢复生成后通过。任务仍未完成，尤其完整DTO/路由覆盖和旧合同验证仍待补齐。
