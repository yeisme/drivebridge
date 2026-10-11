## Context
此次补齐两处浏览器反馈，沿用现有 Base UI 和审计动作字典。

## Goals / Non-Goals
中文活动可理解，勾选项横向对齐，窄屏弹窗不溢出。不新增事件、不把后台读取解释成用户打开页面。

## Decisions
activityLabel 复用 auditActionLabel，未知动作显示其他操作并保留原值提示。结果单独映射，未知结果不宣称完成。新建弹窗收窄，fieldset 分组；共享表单 grid 排除 checkbox-label。

## Risks / Trade-offs
复选框样式影响共用弹窗，需回归 Finder 创建与通用表单。原始动作仍可通过 title 查阅。

## Validation
Vitest 验证动作与结果；Playwright 使用隔离 fixture 覆盖两主题、1213/959/390px、校验和实际创建流程；证据由既有 runner 保存。
