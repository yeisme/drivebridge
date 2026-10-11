import { describe,it,expect } from "vitest";
import { newEntryError } from "./FinderMenu";
describe("new entry paths",()=>{
  it("requires explicit nested-directory mode and rejects traversal",()=>{
    expect(newEntryError("项目/资料",false)).not.toBe("");
    expect(newEntryError("项目/资料",true)).toBe("");
    for(const name of ["..","/root","a//b","a/../b","a\\b","a\u0000b"]){expect(newEntryError(name,true)).not.toBe("");}
    expect(newEntryError("笔记.txt",false)).toBe("");
  });
});
