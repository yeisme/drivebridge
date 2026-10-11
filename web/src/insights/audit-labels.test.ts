import {it,expect} from "vitest";
import {auditActionLabel,auditActorName,shortIdentifier} from "./audit-labels";

it("uses Chinese presentation while preserving unknown actions for the caller",()=>{
 expect(auditActionLabel("governance.overview")).toBe("查看管理概览");
 expect(auditActionLabel("operations.inbox")).toBe("查看待归档产物");
 expect(auditActionLabel("future.action")).toBe("其他操作");
});
it("resolves only the matching identity kind and current visible names",()=>{
 const names={users:{same:"用户名称"},agents:{same:"机器人名称"}};
 expect(auditActorName("user","same",names)).toBe("用户名称");
 expect(auditActorName("agent","same",names)).toBe("机器人名称");
 expect(auditActorName("user","hidden",names)).toBe("用户");
 expect(auditActorName("local","same",names)).toBe("本机身份");
});
it("shortens long IDs with both ends and keeps short IDs intact",()=>{
 expect(shortIdentifier("1234567890abcdefghijklmn")).toBe("12345678…ijklmn");
 expect(shortIdentifier("short-id")).toBe("short-id");
});
