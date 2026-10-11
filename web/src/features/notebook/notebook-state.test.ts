import { describe, expect, it } from "vitest";
import { createNotebookState, notebookReducer, noteKey, type NoteIdentity } from "./notebook-state";

const a: NoteIdentity = { sourceId: "source", objectId: "a", revision: "1" };
const b = { ...a, objectId: "b" };
const open = (ref: NoteIdentity, pinned = false) => ({ type: "open" as const, ref, pinned });

describe("notebook private session state", () => {
  it("reuses only an unpinned clean preview tab", () => {
    let state = notebookReducer(createNotebookState(), open(a));
    state = notebookReducer(state, open(b));
    expect(state.tabs.map(tab => tab.ref)).toEqual([b]);
    state = notebookReducer(state, { type: "pin", key: noteKey(b), pinned: true });
    state = notebookReducer(state, open(a));
    expect(state.tabs).toHaveLength(2);
  });

  it("pins dirty tabs, refuses unconfirmed close, and never reopens discarded drafts", () => {
    let state = notebookReducer(createNotebookState(), open(a));
    state = notebookReducer(state, { type: "draft", key: noteKey(a), body: "private draft" });
    expect(state.tabs[0].pinned).toBe(true);
    expect(notebookReducer(state, { type: "close", key: noteKey(a) })).toBe(state);
    state = notebookReducer(state, open(b));
    expect(state.tabs).toHaveLength(2);
    state = notebookReducer(state, { type: "close", key: noteKey(a), discard: true });
    expect(state.drafts[noteKey(a)]).toBeUndefined();
    state = notebookReducer(state, { type: "reopen" });
    expect(state.tabs.find(tab => tab.key === noteKey(a))?.dirty).toBe(false);
  });

  it("separates revisions and source identities without delimiter collisions", () => {
    expect(noteKey({ ...a, sourceId: "a:b", objectId: "c" })).not.toBe(noteKey({ ...a, sourceId: "a", objectId: "b:c" }));
    let state = notebookReducer(createNotebookState(), open(a, true));
    state = notebookReducer(state, { type: "draft", key: noteKey(a), body: "draft" });
    state = notebookReducer(state, open({ ...a, revision: "2" }, true));
    expect(state.tabs).toHaveLength(2);
    expect(state.drafts[noteKey({ ...a, revision: "2" })]).toBeUndefined();
  });

  it("shares a draft but keeps pane positions independent", () => {
    let state = notebookReducer(createNotebookState(), open(a));
    state = notebookReducer(state, { type: "split", key: noteKey(a) });
    state = notebookReducer(state, { type: "position", pane: "primary", key: noteKey(a), scrollTop: 120, selection: { from: 2, to: 5 } });
    state = notebookReducer(state, { type: "position", pane: "secondary", key: noteKey(a), scrollTop: 800 });
    state = notebookReducer(state, { type: "draft", key: noteKey(a), body: "shared" });
    expect(state.positions.primary[noteKey(a)].scrollTop).toBe(120);
    expect(state.positions.secondary[noteKey(a)].scrollTop).toBe(800);
    expect(Object.values(state.drafts)).toEqual(["shared"]);
  });

  it("never replaces a preview visible in the other pane", () => {
    let state = notebookReducer(createNotebookState(), open(a));
    state = notebookReducer(state, { type: "split", key: noteKey(a) });
    state = notebookReducer(state, open(b));
    expect(state.tabs).toHaveLength(2);
    expect(state.secondTab).toBe(noteKey(a));
  });

  it("clears sensitive sessions and pane positions on revocation", () => {
    let state = notebookReducer(createNotebookState(), open(a));
    state = notebookReducer(state, { type: "draft", key: noteKey(a), body: "secret" });
    state = notebookReducer(state, { type: "position", pane: "primary", key: noteKey(a), scrollTop: 100 });
    state = notebookReducer(state, { type: "revoke", sourceId: a.sourceId });
    expect(state.tabs).toEqual([]);
    expect(state.closed).toEqual([]);
    expect(state.drafts).toEqual({});
    expect(state.positions.primary).toEqual({});
  });

  it("moves both panes to the confirmed revision and clears only its old draft", () => {
    let state = notebookReducer(createNotebookState(), open(a, true));
    state = notebookReducer(state, { type: "draft", key: noteKey(a), body: "draft" });
    state = notebookReducer(state, { type: "split" });
    state = notebookReducer(state, { type: "position", pane: "primary", key: noteKey(a), scrollTop: 70, selection: { from: 2, to: 4 } });
    state = notebookReducer(state, { type: "position", pane: "secondary", key: noteKey(a), scrollTop: 900 });
    const next = { ...a, revision: "2" };
    state = notebookReducer(state, { type: "saved", key: noteKey(a), ref: next });
    expect(state.activeTab).toBe(noteKey(next));
    expect(state.secondTab).toBe(noteKey(next));
    expect(state.split).toBe(true);
    expect(state.drafts).toEqual({});
    expect(state.tabs[0].dirty).toBe(false);
    expect(state.positions.primary[noteKey(next)]).toEqual({ scrollTop: 70, selection: { from: 2, to: 4 } });
    expect(state.positions.secondary[noteKey(next)].scrollTop).toBe(900);
    expect(notebookReducer(state, { type: "saved", key: noteKey(next), ref: b })).toBe(state);
  });

  it("keeps a draft already based on the recovered revision", () => {
    const next = { ...a, revision: "2" };
    let state = notebookReducer(createNotebookState(), open(a, true));
    state = notebookReducer(state, open(next, true));
    state = notebookReducer(state, { type: "draft", key: noteKey(next), body: "newer draft" });
    state = notebookReducer(state, { type: "split" });
    state = notebookReducer(state, { type: "saved", key: noteKey(a), ref: next });
    expect(state.tabs).toHaveLength(1);
    expect(state.tabs[0].dirty).toBe(true);
    expect(state.drafts[noteKey(next)]).toBe("newer draft");
    expect(state.secondTab).toBe(noteKey(next));
  });

  it("bounds split ratios and cycles existing tabs", () => {
    let state = notebookReducer(notebookReducer(createNotebookState(), open(a, true)), open(b, true));
    state = notebookReducer(state, { type: "cycle", direction: 1 });
    expect(state.activeTab).toBe(noteKey(a));
    state = notebookReducer(state, { type: "ratio", value: 99 });
    expect(state.splitRatio).toBe(0.75);
    expect(notebookReducer(state, { type: "ratio", value: NaN })).toBe(state);
  });
});
