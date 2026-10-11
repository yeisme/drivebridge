import { afterEach, expect, it, vi } from "vitest";
import { cleanup, render, screen, waitFor } from "@testing-library/react";
import { NotebookDiagram } from "./NotebookDiagram";
const renderer = vi.hoisted(() => ({ initialize: vi.fn(), render: vi.fn() }));
vi.mock("mermaid", () => ({ default: renderer }));
afterEach(() => { cleanup(); vi.resetAllMocks(); });

it("rejects source security directives without loading a diagram", () => {
  render(<NotebookDiagram source={'%%{init: {"securityLevel":"loose"}}%%\nflowchart LR\n A --> B'}/>);
  expect(screen.getByRole("alert").textContent).toContain("无法安全渲染");
  expect(renderer.render).not.toHaveBeenCalled();
});

it.each(['flowchart LR\n classDef bad fill:url(https://tracker.invalid/pixel)', 'flowchart LR\n A@{ img: "https://tracker.invalid/pixel" }', 'flowchart LR\n A[<img src="/private">]'])("rejects active resources before Mermaid layout: %s", source => {
  render(<NotebookDiagram source={source}/>);
  expect(screen.getByRole('alert')).toBeTruthy();
  expect(renderer.render).not.toHaveBeenCalled();
});
it("removes active links and handlers from strict SVG output", async () => {
  renderer.render.mockResolvedValue({ svg: '<svg xmlns="http://www.w3.org/2000/svg"><a href="https://tracker.invalid"><text onclick="alert(1)">Node</text></a></svg>' });
  const { container } = render(<NotebookDiagram source={"flowchart LR\n A --> B"}/>);
  await waitFor(() => expect(container.querySelector('[aria-label="笔记图表"] svg')).not.toBeNull());
  expect(container.querySelector('[onclick], [href]')).toBeNull();
  expect(renderer.initialize).toHaveBeenCalledWith(expect.objectContaining({ securityLevel: "strict", htmlLabels: false }));
});
it("keeps unsafe or failed diagrams as source rather than importing HTML", async () => {
  renderer.render.mockResolvedValue({ svg: '<svg xmlns="http://www.w3.org/2000/svg"><foreignObject><div>unsafe</div></foreignObject></svg>' });
  const { container } = render(<NotebookDiagram source={"flowchart LR\n A --> B"}/>);
  await screen.findByRole("alert");
  expect(container.querySelector('foreignObject')).toBeNull();
  expect(container.querySelector('pre')?.textContent).toBe('flowchart LR\n A --> B');
});
