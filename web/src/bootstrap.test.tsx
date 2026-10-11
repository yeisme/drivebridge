import { afterEach, expect, it, vi } from "vitest";

const mocks = vi.hoisted(() => ({ render: vi.fn(), createRoot: vi.fn(), initialize: vi.fn() }));
vi.mock("react-dom/client", () => ({ createRoot: mocks.createRoot }));
vi.mock("./appearance", () => ({ initializeAppearance: mocks.initialize, AppearanceProvider: "appearance-provider" }));
vi.mock("./Sharing", () => ({ SharedView: "shared-view" }));
vi.mock("./app/ScopedWorkspace", () => ({ ScopedWorkspace: "scoped-workspace" }));
vi.mock("./WorkspaceApp", () => ({ default: "workspace-app" }));

afterEach(() => {
  document.body.innerHTML = "";
  history.replaceState(null, "", "/");
  vi.resetModules();
  vi.clearAllMocks();
});

it.each([
  ["/#/drives", "scoped-workspace"],
  ["/#share/fixture/reference", "shared-view"],
])("bootstraps %s through the correct legacy boundary", async (url, expected) => {
  document.body.innerHTML = '<div id="root"></div>';
  history.replaceState(null, "", url);
  mocks.createRoot.mockReturnValue({ render: mocks.render });
  await import("./main");
  expect(mocks.initialize).toHaveBeenCalledTimes(1);
  expect(mocks.createRoot).toHaveBeenCalledWith(document.getElementById("root"));
  expect(mocks.render).toHaveBeenCalledTimes(1);
  const tree = mocks.render.mock.calls[0][0];
  expect(tree.type).toBe("appearance-provider");
  expect(tree.props.children.type).toBe(expected);
  if (expected === "scoped-workspace") {
    expect(tree.props.children.props.component).toBe("workspace-app");
  } else {
    expect(tree.props.children.props.component).toBeUndefined();
  }
});

it("does not initialize or mount when imported without a root", async () => {
  const module = await import("./main");
  expect(module.default).toBe("workspace-app");
  expect(mocks.initialize).not.toHaveBeenCalled();
  expect(mocks.createRoot).not.toHaveBeenCalled();
});
