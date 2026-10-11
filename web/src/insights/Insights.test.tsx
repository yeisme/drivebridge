import React from "react";
import { render, screen, fireEvent, cleanup } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { MeterBar, Trend } from "./charts";
afterEach(cleanup);
it("renders observed zero on a baseline without fractional byte ticks or fake missing-day dots", () => {
  const { container } = render(<Trend title="零容量" rows={[
    { day: "2026-09-01", values: [null] }, { day: "2026-09-02", values: [0] },
  ]} fields={["文件"]} format={n => n + " B"} onDay={() => {}} />);
  expect(container.querySelectorAll(".chart-zero")).toHaveLength(1);
  expect(container.querySelectorAll(".chart-gap")).toHaveLength(0);
  expect(container.querySelector("svg")?.textContent).not.toContain("0.5 B");
  expect(screen.getByRole("button", {name:/2026-09-01 文件:未采集/})).toBeTruthy();
  expect(screen.getByRole("button", {name:/2026-09-02 文件:0 B/})).toBeTruthy();
});
it("keeps missing days distinct from zero and exposes keyboard drilldown", () => {
  const pick = vi.fn();
  render(
    <Trend
      title="容量趋势"
      rows={[
        { day: "2026-09-01", values: [null] },
        { day: "2026-09-02", values: [0] },
      ]}
      fields={["当前版本"]}
      onDay={pick}
    />,
  );
  expect(screen.getAllByText("未采集").length).toBeGreaterThan(0);
  fireEvent.keyDown(
    screen.getByRole("button", { name: /2026-09-01 当前版本:未采集/ }),
    { key: "Enter" },
  );
  expect(pick).toHaveBeenCalledWith("2026-09-01");
});
it("renders different byte definitions side by side, not as a combined stack", () => {
  const { container } = render(
    <Trend
      title="传输"
      rows={[{ day: "2026-09-02", values: [100, 200] }]}
      fields={["有效量", "观测量"]}
      stacked={false}
      onDay={() => {}}
    />,
  );
  const bars = container.querySelectorAll("svg rect[class]");
  expect(bars.length).toBe(2);
  expect(bars[0].getAttribute("x")).not.toBe(bars[1].getAttribute("x"));
});
it("formats audit count charts as events, never as byte totals", () => {
  render(
    <Trend
      title="安全事件"
      rows={[{ day: "2026-09-02", values: [4] }]}
      fields={["拒绝"]}
      format={(n) => n + " 次"}
      onDay={() => {}}
    />,
  );
  expect(screen.getByRole("button", { name: /拒绝:4 次/ })).toBeTruthy();
});
it("shows a follow tooltip on keyboard focus while keeping the data table", () => {
  render(
    <Trend
      title="容量趋势"
      rows={[{ day: "2026-09-02", values: [4] }]}
      fields={["当前版本"]}
      onDay={() => {}}
    />,
  );
  fireEvent.focus(screen.getByRole("button", { name: /2026-09-02 当前版本:4/ }));
  expect(screen.getByRole("status").textContent).toContain("当前版本 4");
  expect(screen.getByText("查看数据表")).toBeTruthy();
});
it("does not draw a full quota bar when quota is unknown", () => {
  const { container } = render(
    <MeterBar current={10} history={null} trash={1} reserved={0} quota={null} />,
  );
  const bar = container.querySelector(".meter-bar");
  expect(bar?.getAttribute("data-quota")).toBe("unknown");
  expect(container.querySelector("[data-filled=false]")).toBeTruthy();
  expect(screen.getByText("配额未知")).toBeTruthy();
});
it("keeps a null day as a gap instead of a zero-height bar", () => {
  const { container } = render(
    <Trend
      title="容量趋势"
      rows={[
        { day: "2026-09-01", values: [null] },
        { day: "2026-09-02", values: [0] },
      ]}
      fields={["当前版本"]}
      onDay={() => {}}
    />,
  );
  const groups = container.querySelectorAll("svg g[role='button']");
  expect(groups[0].querySelectorAll("rect[class]").length).toBe(0);
  expect(groups[1].querySelectorAll("rect[class]").length).toBe(1);
});

it("keeps the data table available when every day is uncollected", () => {
  const {container} = render(<Trend title="缺口" rows={[{day:"2026-09-01",values:[null]}]} fields={["容量"]} onDay={() => {}} />);
  expect(container.querySelector("svg.insight-chart")).toBeNull();
  expect(screen.getByText("查看数据表")).toBeTruthy();
  expect(container.querySelector("tbody")?.textContent).toContain("未采集");
});
