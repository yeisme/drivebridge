import { describe, it, expect } from "vitest";
import { capacityOf, percentText } from "./capacity";
describe("space quota accounting", () => {
  it.each([
    [0,0,100,0,100,0], [20,5,100,25,75,25], [99,1,100,100,0,100], [110,20,100,130,0,130],
  ])("accounts for used %s and reserved %s against quota %s", (used,reserved,quota,occupied,free,percent) => {
    expect(capacityOf({used_bytes:used,reserved_bytes:reserved,max_bytes:quota})).toMatchObject({occupied,free,percent});
  });
  it("retains missing values instead of manufacturing zero or free capacity", () => {
    expect(capacityOf({used_bytes:10,reserved_bytes:0,max_bytes:0})).toMatchObject({quota:null,free:null,percent:null,used:10});
    expect(capacityOf({used_bytes:NaN,reserved_bytes:0,max_bytes:100})).toMatchObject({occupied:null,percent:null,free:null});
    expect(capacityOf({used_bytes:10,reserved_bytes:-1,max_bytes:100})).toMatchObject({occupied:null});
  });
  it("reports actual excess and tiny nonzero percentages", () => {
    expect(capacityOf({used_bytes:110,reserved_bytes:20,max_bytes:100}).overflow).toBe(30);
    expect(percentText(0.01)).toBe("<0.1%");
    expect(percentText(130)).toBe("130%");
  });
});
