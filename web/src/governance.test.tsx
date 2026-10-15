import React from "react";
import { afterEach, describe, it, expect, vi } from "vitest";
import { render, screen, fireEvent, cleanup, waitFor } from "@testing-library/react";
import { api, setDataContext, contextURL } from "./api";
import { GovernanceBar, type GovernanceState } from "./Governance";
import { governanceMode } from "./governance-context";

// Base UI popup positioning is covered by Playwright; this test verifies tenant state transitions.
vi.mock("./components/SelectField", () => ({ SelectField: ({ options, value, onValueChange, "aria-label": label }: any) =>
  <select aria-label={label} value={value} onChange={event => onValueChange(event.target.value)}>{options.map((option: any) => <option key={option.value} value={option.value}>{option.label}</option>)}</select>
}));
vi.mock("./components/ComboboxField", () => ({ ComboboxField: ({ options, value, onValueChange, "aria-label": label }: any) =>
  <select aria-label={label} value={value} onChange={event => onValueChange(event.target.value)}>{options.map((option: any) => <option key={option.value} value={option.value}>{option.label}</option>)}</select>
}));

vi.mock("./components/ui/popover", () => ({
  Popover: ({children}: {children:React.ReactNode}) => <div>{children}</div>,
  PopoverTrigger: ({children,render,...props}: any) => React.cloneElement(render,props,children),
  PopoverContent: ({children}: {children:React.ReactNode}) => <div>{children}</div>,
}));

afterEach(() => {
  cleanup();
  vi.restoreAllMocks();
  setDataContext("");
});
describe("tenant context", () => {
  it("uses administrator write access only when the server enables it", () => {
    setDataContext("a", "space");
    const data = { enabled: true, space_roles: { space: "admin" }, requests: [] } as unknown as GovernanceState;
    expect(governanceMode(data)).toBe("admin");
    expect(governanceMode({ ...data, admin_direct_write: true })).toBe("admin_editor");
    expect(governanceMode({ ...data, admin_direct_write: true, space_roles: { space: "viewer" } })).toBe("viewer");
  });
  it("attaches the selected context and leaves governance requests out of data scope", async () => {
    const fetcher = vi
      .spyOn(globalThis, "fetch")
      .mockImplementation(
        async () => new Response(JSON.stringify({ data: { ok: true } })),
      );
    setDataContext("tenant-a", "space-a");
    await api("/files");
    await api("/governance");
    expect(
      (fetcher.mock.calls[0][1]?.headers as Record<string, string>)[
        "X-DriveBridge-Tenant"
      ],
    ).toBe("tenant-a");
    expect(
      (fetcher.mock.calls[0][1]?.headers as Record<string, string>)[
        "X-DriveBridge-Space"
      ],
    ).toBe("space-a");
    expect(
      (fetcher.mock.calls[1][1]?.headers as Record<string, string>)[
        "X-DriveBridge-Space"
      ],
    ).toBeUndefined();
    expect(contextURL("/api/v1/files/f/media?version=v")).toContain(
      "tenant_context=tenant-a",
    );
  });
  it("rejects stale data after changing tenants", async () => {
    let resolve!: (r: Response) => void;
    vi.spyOn(globalThis, "fetch").mockImplementation(
      () => new Promise((r) => (resolve = r)),
    );
    setDataContext("tenant-a");
    const pending = api("/files");
    setDataContext("tenant-b");
    resolve(new Response(JSON.stringify({ data: [{ id: "private-a" }] })));
    await expect(pending).rejects.toMatchObject({ name: "AbortError" });
  });
  it("renders management as read-only and clears the target on tenant switch", async () => {
    setDataContext("a", "space");
    const changed = vi.fn();
    window.addEventListener("drivebridge-context", changed);
    const data = {
      enabled: true,
      tenant_id: "a",
      tenants: [
        { id: "a", name: "Alpha" },
        { id: "b", name: "Beta" },
      ],
      spaces: [{ id: "space", name: "Private files" }],
      space_roles: { space: "admin" },
      requests: [],
      notifications: [],
    } as unknown as GovernanceState;
    render(
      <GovernanceBar data={data} user={{ id: "admin", name: "管理员" }} />,
    );
    fireEvent.click(screen.getByLabelText("工作区选项"));
    expect(screen.getByRole("status").textContent).toContain("管理只读");
    fireEvent.change(screen.getByLabelText("切换租户"), {target: {value: "b"}});
    await waitFor(() => expect(changed).toHaveBeenCalledTimes(1));
    expect(contextURL("/api/v1/files")).not.toContain("space_context");
    window.removeEventListener("drivebridge-context", changed);
  });
});

it("does not start an upload in a different tenant after hashing", async () => {
  const { upload } = await import("./api");
  let finish!: (b: ArrayBuffer) => void;
  const file = {
    size: 1,
    name: "test.txt",
    slice: () => ({
      arrayBuffer: () =>
        new Promise<ArrayBuffer>((resolve) => (finish = resolve)),
    }),
  } as unknown as File;
  const fetcher = vi.spyOn(globalThis, "fetch");
  setDataContext("original");
  const pending = upload(
    file,
    "original-space",
    "test.txt",
    () => {},
    new AbortController().signal,
  );
  setDataContext("different");
  finish(new Uint8Array([1]).buffer);
  await expect(pending).rejects.toMatchObject({ name: "AbortError" });
  expect(fetcher).not.toHaveBeenCalled();
});
