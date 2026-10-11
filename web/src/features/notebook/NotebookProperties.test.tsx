import { afterEach, expect, it } from "vitest";
import { cleanup, render, screen } from "@testing-library/react";
import { NotebookProperties } from "./NotebookProperties";
afterEach(cleanup);

it("shows the exact revision and distinguishes unsupported relations from no links", () => {
  const view = render(<NotebookProperties identity={{ sourceId: "source", objectId: "object", revision: "pinned-version" }}/>);
  expect(screen.getByText("pinned-version")).toBeTruthy();
  expect(screen.getByLabelText("笔记关系能力").textContent).toContain("暂不支持");
  view.rerender(<NotebookProperties/>);
  expect(screen.queryByText("pinned-version")).toBeNull();
  expect(screen.queryByLabelText("笔记关系能力")).toBeNull();
});
