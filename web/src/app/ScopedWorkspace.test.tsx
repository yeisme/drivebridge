import { StrictMode, useEffect, useState } from "react";
import { act, cleanup, fireEvent, render, screen } from "@testing-library/react";
import { afterEach, expect, it, vi } from "vitest";
import { ScopedWorkspace } from "./ScopedWorkspace";
import { getDataContext, setDataContext } from "./data-context";
import { useWorkspaceScope } from "./WorkspaceScope";
import { useRouteLeaveGuard } from "./route-leave";

function Page() {
  const [count, setCount] = useState(0);
  return <button onClick={() => setCount(count + 1)}>Count {count}</button>;
}
afterEach(() => { cleanup(); history.replaceState(null, "", "/"); setDataContext(""); vi.restoreAllMocks(); });

it("rejects a hash scope change before page listeners, context changes or draft unmount", () => {
  history.replaceState({ marker: 'keep' }, '', '/#/knowledge?tenant=one');
  const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
  const unmount = vi.fn();
  const downstream = vi.fn();
  function DraftPage() {
    const [draft, setDraft] = useState('');
    useRouteLeaveGuard(!!draft, '放弃草稿？');
    useEffect(() => {
      window.addEventListener('hashchange', downstream);
      return () => { unmount(); window.removeEventListener('hashchange', downstream); };
    }, []);
    return <input aria-label="draft" value={draft} onChange={event => setDraft(event.target.value)} />;
  }
  render(<ScopedWorkspace component={DraftPage} />);
  fireEvent.change(screen.getByLabelText('draft'), { target: { value: 'memory only' } });
  const before = getDataContext();
  act(() => { history.replaceState(history.state, '', '/#/drive/foreign?tenant=two'); window.dispatchEvent(new HashChangeEvent('hashchange')); });
  expect(confirm).toHaveBeenCalledTimes(1);
  expect(getDataContext()).toBe(before);
  expect(location.hash).toBe('#/knowledge?tenant=one');
  expect(history.state).toEqual({ marker: 'keep' });
  expect((screen.getByLabelText('draft') as HTMLInputElement).value).toBe('memory only');
  expect(downstream).not.toHaveBeenCalled();
  expect(unmount).not.toHaveBeenCalled();
  // An accepted same-scope route does not erase the still-mounted draft or
  // its blocker. A later rejected route must restore that accepted location.
  confirm.mockReturnValue(true);
  act(() => { history.replaceState(history.state, '', '/#/knowledge?tenant=one&view=alternate'); window.dispatchEvent(new HashChangeEvent('hashchange')); });
  expect((screen.getByLabelText('draft') as HTMLInputElement).value).toBe('memory only');
  confirm.mockReturnValue(false);
  act(() => { history.replaceState(history.state, '', '/#/files?tenant=one'); window.dispatchEvent(new HashChangeEvent('hashchange')); });
  expect(location.hash).toBe('#/knowledge?tenant=one&view=alternate');
  expect(confirm).toHaveBeenCalledTimes(3);
  confirm.mockReturnValue(true);
  act(() => { history.replaceState(history.state, '', '/#/drive/foreign?tenant=two'); window.dispatchEvent(new HashChangeEvent('hashchange')); });
  expect(getDataContext()).toMatchObject({ tenant: 'two', space: '' });
  expect((screen.getByLabelText('draft') as HTMLInputElement).value).toBe('');
  expect(unmount).toHaveBeenCalledTimes(1);
});

it("does not let draft confirmation block forced identity invalidation", () => {
  const confirm = vi.spyOn(window, 'confirm').mockReturnValue(false);
  function DirtyPage() { useRouteLeaveGuard(true, 'unsaved'); return <Page />; }
  render(<ScopedWorkspace component={DirtyPage} />);
  fireEvent.click(screen.getByRole('button'));
  act(() => setDataContext('revoked-context', ''));
  expect(screen.getByRole('button').textContent).toBe('Count 0');
  expect(confirm).not.toHaveBeenCalled();
});

it("projects the current scope through context without carrying file or credential state", () => {
  function ScopePage() {
    const scope = useWorkspaceScope();
    return <output>{JSON.stringify(scope)}</output>;
  }
  const { container } = render(<ScopedWorkspace component={ScopePage} />);
  act(() => setDataContext("tenant-next", "space-next"));
  expect(JSON.parse(container.querySelector("output")!.textContent!)).toEqual({
    tenant: "tenant-next", space: "space-next", revision: getDataContext().revision,
  });
});

it("binds the initial deep link before any child effect can read stale scope", () => {
  setDataContext("old-tenant", "old-space");
  history.replaceState(null, "", "/#/drive/new-space?tenant=new-tenant&scope=managed");
  const observed = vi.fn();
  function RequestPage() {
    useEffect(() => { observed(getDataContext()); }, []);
    return null;
  }
  render(<StrictMode><ScopedWorkspace component={RequestPage} /></StrictMode>);
  expect(observed).toHaveBeenCalled();
  for (const [scope] of observed.mock.calls) {
    expect(scope).toMatchObject({ tenant: "new-tenant", space: "new-space" });
  }
});

it("resets page-local state after a scope change", () => {
  render(<ScopedWorkspace component={Page} />);
  fireEvent.click(screen.getByRole("button"));
  expect(screen.getByRole("button").textContent).toBe("Count 1");
  act(() => setDataContext("other", "space"));
  expect(screen.getByRole("button").textContent).toBe("Count 0");
});

it("binds initial managed deep links and preserves tenant on tenant-less routes", () => {
  history.replaceState(null, "", "/#/drive/space-a?tenant=tenant-a&scope=managed");
  render(<ScopedWorkspace component={Page} />);
  expect(getDataContext()).toMatchObject({ tenant: "tenant-a", space: "space-a" });
  act(() => {
    history.replaceState(null, "", "/#/drives");
    window.dispatchEvent(new HashChangeEvent("hashchange"));
  });
  expect(getDataContext()).toMatchObject({ tenant: "tenant-a", space: "" });
});

it("keeps page state for an unchanged scope and removes listeners on unmount", () => {
  const remove = vi.spyOn(window, "removeEventListener");
  const { unmount } = render(<ScopedWorkspace component={Page} />);
  fireEvent.click(screen.getByRole("button"));
  act(() => {
    history.replaceState(null, "", "/#/drives");
    window.dispatchEvent(new HashChangeEvent("hashchange"));
  });
  expect(screen.getByRole("button").textContent).toBe("Count 1");
  unmount();
  expect(remove).toHaveBeenCalledWith("hashchange", expect.any(Function), true);
  expect(remove).toHaveBeenCalledWith("drivebridge-context", expect.any(Function));
  const before = getDataContext();
  history.replaceState(null, "", "/#/drive/foreign?tenant=foreign&scope=managed");
  window.dispatchEvent(new HashChangeEvent("hashchange"));
  expect(getDataContext()).toBe(before);
});
