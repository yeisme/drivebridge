import { useState } from "react";
import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { SelectAdapter } from "./SelectAdapter";

afterEach(() => { cleanup(); vi.restoreAllMocks(); });

// Regression reproducer: all value assertions can complete while the full
// popup leaves the jsdom test run hanging. Do not substitute mocked primitives
// or increase the timeout to claim that the lifecycle issue is fixed.
it("selects a controlled value after options arrive with an empty placeholder", () => {
  function Fixture() {
    const [value, setValue] = useState("");
    const [loaded, setLoaded] = useState(false);
    return <>
      <button onClick={() => setLoaded(true)}>load fixture</button>
      <SelectAdapter value={value} onChange={event => setValue(event.target.value)} aria-label="选择分享">
        <option value="">选择分享…</option>
        {loaded && <option value="fixture-share">交付一批</option>}
      </SelectAdapter>
      <output>{value}</output>
    </>;
  }
  const view = render(<Fixture />);
  fireEvent.click(screen.getByRole("button", { name: "load fixture" }));
  const trigger = screen.getByLabelText("选择分享");
  trigger.focus();
  fireEvent.keyDown(trigger, { key: "ArrowDown" });
  const options = Array.from(document.querySelectorAll<HTMLElement>('[role="option"]'));
  expect(options.map(option => option.textContent)).toEqual(["选择分享…", "交付一批"]);
  fireEvent.pointerDown(options[1], { pointerType: "mouse", button: 0 });
  fireEvent.click(options[1]);
  expect(document.querySelector("output")!.textContent).toBe("fixture-share");
  expect(trigger.getAttribute("aria-expanded")).toBe("false");
  view.unmount();
});
