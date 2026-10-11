import { afterEach, expect, it, vi } from "vitest";
import { act, cleanup, render, screen } from "@testing-library/react";
import { setDataContext } from "../../app/data-context";
import { MarkdownFileDocument } from "./MarkdownFileDocument";
const identity = { sourceId: "space", objectId: "note", revision: "one" };
afterEach(() => { cleanup(); vi.unstubAllGlobals(); setDataContext(""); });

it("reads the pinned owner content but never grants editing from read permission", async () => {
  setDataContext("tenant", "space");
  vi.stubGlobal("fetch", vi.fn(async () => new Response("# Owner content")));
  render(<MarkdownFileDocument identity={identity}/>);
  expect(await screen.findByRole("heading", { name: "Owner content" })).toBeTruthy();
  expect(screen.getByRole("status").textContent).toBe("当前来源未授予编辑能力");
});

it("clears already loaded content when the active context changes", async () => {
  setDataContext("tenant", "space");
  const fetcher = vi.fn(async () => new Response("# Private content"));
  vi.stubGlobal("fetch", fetcher);
  render(<MarkdownFileDocument identity={identity}/>);
  await screen.findByRole("heading", { name: "Private content" });
  act(() => { setDataContext("other", "space"); });
  expect(screen.queryByRole("heading", { name: "Private content" })).toBeNull();
  expect(screen.getByRole("alert").textContent).toContain("访问权限已失效");
  expect(fetcher).toHaveBeenCalledOnce();
});

it("hides old content immediately on identity change and rejects its late response", async () => {
  setDataContext("tenant", "space");
  let resolve!: (response: Response) => void;
  vi.stubGlobal("fetch", vi.fn().mockImplementationOnce(() => new Promise<Response>(done => { resolve = done; })).mockResolvedValueOnce(new Response("", { status: 403 })));
  const view = render(<MarkdownFileDocument identity={identity}/>);
  view.rerender(<MarkdownFileDocument identity={{ ...identity, objectId: "other" }}/>);
  expect((await screen.findByRole("alert")).textContent).toContain("访问权限已失效");
  resolve(new Response("# Private old note"));
  await Promise.resolve();
  expect(screen.queryByRole("heading", { name: "Private old note" })).toBeNull();
});
