import { cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { useNavGroups } from "./useNavGroups";
import { navGroupsKey, readNavGroups } from "../nav-groups";

function Fixture({ instance = "instance-a", user = "user-a" }) {
  const [expanded, change] = useNavGroups(instance, user);
  return <button aria-expanded={expanded.workbench !== false} onClick={() => change({ ...expanded, workbench: expanded.workbench === false })}>workbench</button>;
}
afterEach(() => { cleanup(); localStorage.clear(); vi.restoreAllMocks(); });
it("restores only the matching user's explicit group preference", () => {
  localStorage.setItem("drivebridge:nav-groups", JSON.stringify({ workbench: false }));
  const view = render(<Fixture />);
  const button = screen.getByRole("button");
  expect(button.getAttribute("aria-expanded")).toBe("true");
  fireEvent.click(button);
  expect(readNavGroups(navGroupsKey("instance-a", "user-a"))).toEqual({ workbench: false });
  view.rerender(<Fixture user="user-b" />);
  expect(button.getAttribute("aria-expanded")).toBe("true");
  view.rerender(<Fixture instance="instance-b" />);
  expect(button.getAttribute("aria-expanded")).toBe("true");
  view.rerender(<Fixture />);
  expect(button.getAttribute("aria-expanded")).toBe("false");
});
it("does not persist anonymous changes and validates stored group names", () => {
  const save = vi.spyOn(Storage.prototype, "setItem");
  render(<Fixture user="" />);
  fireEvent.click(screen.getByRole("button"));
  expect(save).not.toHaveBeenCalled();
  const key = navGroupsKey("instance-a", "user-a");
  localStorage.setItem(key, JSON.stringify({ workbench: false, system: true, agents: "false", foreign: true }));
  expect(readNavGroups(key)).toEqual({ workbench: false, system: true });
  localStorage.setItem(key, "not-json");
  expect(readNavGroups(key)).toEqual({});
});
