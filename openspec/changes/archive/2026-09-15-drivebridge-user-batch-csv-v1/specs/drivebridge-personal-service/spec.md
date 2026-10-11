## ADDED Requirements

### Requirement: Tabular batch issuance
`users import` SHALL 接受 CSV 批量输入并可选写出分发文件：带表头（name/days/admin/key）或按位无表头，单列即纯名单；分发文件只含签发输出且权限 0600；JSON 输入路径行为不变。

#### Scenario: Header CSV with per-user options
- **WHEN** 提交带 name,days,admin 表头的 CSV
- **THEN** 按列解析批量签发，格式或重复错误整批拒绝零落库

#### Scenario: Plain name list
- **WHEN** 提交单列名单（无表头、容忍空行与 BOM）
- **THEN** 每行作为一个用户签发，默认长期有效非 admin

#### Scenario: Distribution file
- **WHEN** 指定 --out 分发文件
- **THEN** 写出 name,user_id,key,admin,expires_at 的 CSV 且文件权限 0600

#### Scenario: JSON path unchanged
- **WHEN** 不带 --csv 提交 stdin JSON
- **THEN** 行为与本变更前一致
