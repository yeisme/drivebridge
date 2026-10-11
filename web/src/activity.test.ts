import { expect, it } from "vitest";
import { activityLabel, activityOutcomeLabel, agentState, filterAgents } from "./activity";

it("uses Chinese labels for known actions", () => {
  expect(activityLabel("keys.disable")).toBe("禁用登录 Key");
  expect(activityLabel("agents.list")).toBe("查看机器人授权");
  expect(activityLabel("operations.inbox")).toBe("查看待归档产物");
  expect(activityLabel("governance.users.list")).toBe("查看租户成员");
  expect(activityLabel("custom.event")).toBe("其他操作");
});

it("distinguishes intent, denial and completion without leaking unknown codes",()=>{
 expect(activityOutcomeLabel("intent")).toBe("操作请求");
 expect(activityOutcomeLabel("forbidden")).toBe("访问被拒绝");
 expect(activityOutcomeLabel("success")).toBe("已完成");
 expect(activityOutcomeLabel("future_outcome")).toBe("结果待确认");
});

it("filters agents by status and query", () => {
  const agents = [
    { name: "整理助手", revoked: false, expires_at: "2099-01-01" },
    { name: "旧助手", revoked: true, expires_at: "2099-01-01" },
    { name: "过期助手", revoked: false, expires_at: "2001-01-01" },
  ];
  expect(agentState("2099-01-01", true)).toBe("已撤销");
  expect(filterAgents(agents, "", "active").map((a) => a.name)).toEqual(["整理助手"]);
  expect(filterAgents(agents, "旧", "all").map((a) => a.name)).toEqual(["旧助手"]);
});
