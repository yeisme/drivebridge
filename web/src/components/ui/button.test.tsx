// @vitest-environment jsdom
import React from "react";
import { render, screen, cleanup } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";
import { Button } from "./button";
afterEach(cleanup);
it("keeps composite surface geometry independent of control utilities", () => {
 render(<Button layout="surface" className="metric-card">容量</Button>);
 const button=screen.getByRole("button");
 expect(button.className).toContain("metric-card");
 for(const rule of ["inline-flex","h-8","whitespace-nowrap","justify-center","size-4"]) expect(button.className).not.toContain(rule);
 expect(button.className).toContain("focus-visible");
});
it("preserves standard control styling", () => {
 render(<Button>保存</Button>);
 expect(screen.getByRole("button").className).toContain("inline-flex");
});
