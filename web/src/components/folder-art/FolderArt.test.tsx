import { cleanup, render } from "@testing-library/react";
import { afterEach, expect, it } from "vitest";
import { FolderArt } from "./FolderArt";

afterEach(cleanup);

it("stays decorative and keeps its badge out of the text content", () => {
  const { container } = render(
    <FolderArt tone={3} sheets={["image", "pdf", "doc", "video"]} badge="3小时前更新" />,
  );
  const art = container.querySelector(".folder-art")!;
  expect(art.getAttribute("aria-hidden")).toBe("true");
  expect(art.getAttribute("data-tone")).toBe("3");
  expect(art.getAttribute("data-sheets")).toBe("3");
  expect(Array.from(art.querySelectorAll(".folder-art-sheet"), (sheet) => sheet.getAttribute("data-kind"))).toEqual(["image", "pdf", "doc"]);
  expect(art.querySelector(".folder-art-front")!.getAttribute("data-badge")).toBe("3小时前更新");
  expect(art.textContent).toBe("");
  expect(art.hasAttribute("data-open")).toBe(false);
});

it("renders an empty folder that can be held open for a drop", () => {
  const { container } = render(<FolderArt open />);
  const art = container.querySelector(".folder-art")!;
  expect(art.getAttribute("data-tone")).toBe("1");
  expect(art.getAttribute("data-sheets")).toBe("0");
  expect(art.querySelector(".folder-art-sheet")).toBeNull();
  expect(art.querySelector(".folder-art-front")!.hasAttribute("data-badge")).toBe(false);
  expect(art.hasAttribute("data-open")).toBe(true);
});
