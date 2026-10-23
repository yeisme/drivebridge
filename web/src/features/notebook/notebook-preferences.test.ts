import { describe, expect, it } from "vitest";
import { clearNotebookPreferences, defaultNotebookPreferences, notebookPreferenceKey, readNotebookPreferences, writeNotebookPreferences } from "./notebook-preferences";

const scope = { actor: "actor", space: "space", source: "source" };
function storage() {
  const values = new Map<string, string>();
  return { values, getItem: (key: string) => values.get(key) ?? null, setItem: (key: string, value: string) => { values.set(key, value); }, removeItem: (key: string) => { values.delete(key); } };
}

describe("notebook private preferences", () => {
  it("isolates actor, space and source with collision-free keys", () => {
    const store = storage();
    const preferences = { ...defaultNotebookPreferences(), fontSize: 22 };
    expect(writeNotebookPreferences(store, scope, preferences)).toBe(true);
    expect(readNotebookPreferences(store, scope)).toEqual(preferences);
    for (const field of ["actor", "space", "source"] as const) {
      expect(readNotebookPreferences(store, { ...scope, [field]: "other" }).fontSize).toBe(16);
    }
    expect(notebookPreferenceKey({ ...scope, actor: "a:b", space: "c" })).not.toBe(notebookPreferenceKey({ ...scope, actor: "a", space: "b:c" }));
  });

  it("never persists session bodies, credentials, or arbitrary fields", () => {
    const store = storage();
    writeNotebookPreferences(store, scope, { ...defaultNotebookPreferences(), draft: "secret body", token: "secret token" } as ReturnType<typeof defaultNotebookPreferences>);
    const serialized = [...store.values.values()][0];
    expect(serialized).not.toContain("secret");
    expect(JSON.parse(serialized)).toEqual(defaultNotebookPreferences());
  });

  it("bounds untrusted values and rejects invalid enums", () => {
    const store = storage();
    store.setItem(notebookPreferenceKey(scope)!, JSON.stringify({ fontSize: 300, splitRatio: -5, leftWidth: "bad", rightOpen: "false", readingWidth: "huge", motion: "always" }));
    expect(readNotebookPreferences(store, scope)).toEqual({ ...defaultNotebookPreferences(), fontSize: 24, splitRatio: 0.25 });
    for (const invalid of ["{", "null", "[]", "123"]) {
      store.setItem(notebookPreferenceKey(scope)!, invalid);
      expect(readNotebookPreferences(store, scope)).toEqual(defaultNotebookPreferences());
    }
  });

  it("persists only bounded per-note modes", () => {
    const store = storage();
    writeNotebookPreferences(store, scope, { ...defaultNotebookPreferences(), modeByNote: { "[\"source\",\"note\",\"one\"]": "source", bad: "editor" } as never });
    expect(readNotebookPreferences(store, scope).modeByNote).toEqual({ "[\"source\",\"note\",\"one\"]": "source" });
  });

  it("handles denied storage and refuses anonymous scope writes", () => {
    const denied = { getItem: () => { throw new Error("denied"); }, setItem: () => { throw new Error("denied"); }, removeItem: () => { throw new Error("denied"); } };
    expect(readNotebookPreferences(denied, scope)).toEqual(defaultNotebookPreferences());
    expect(writeNotebookPreferences(denied, scope, defaultNotebookPreferences())).toBe(false);
    expect(clearNotebookPreferences(denied, scope)).toBe(false);
    const store = storage();
    expect(writeNotebookPreferences(store, { ...scope, actor: "" }, defaultNotebookPreferences())).toBe(false);
    expect(store.values.size).toBe(0);
  });

  it("clears only the current view preferences", () => {
    const store = storage();
    store.setItem("unrelated", "keep");
    writeNotebookPreferences(store, scope, defaultNotebookPreferences());
    expect(clearNotebookPreferences(store, scope)).toBe(true);
    expect([...store.values.entries()]).toEqual([["unrelated", "keep"]]);
  });
});
