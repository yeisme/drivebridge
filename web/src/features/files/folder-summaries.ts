import { useEffect, useState, useSyncExternalStore, type RefObject } from "react";
import { api } from "../../api";
import { summarizeFolder, type FolderSummary } from "../../components/folder-art/folder-cover";
import type { DirectoryPage, DirectoryReader } from "./useDirectoryRead";

export type FolderSummaryState =
  | { status: "idle" | "loading" | "error" }
  | { status: "ready"; summary: FolderSummary };

const IDLE: FolderSummaryState = { status: "idle" };
const LOADING: FolderSummaryState = { status: "loading" };
const ERROR: FolderSummaryState = { status: "error" };

export interface FolderSummaryStore {
  get(path: string): FolderSummaryState;
  want(path: string): void;
  subscribe(listener: () => void): () => void;
  dispose(): void;
}

// Every /browser read scans the space, so summaries are lazy, deduplicated per
// Finder scope and limited to `limit` reads in flight. Disposal aborts them.
export function createFolderSummaryStore(
  space: string,
  reader?: DirectoryReader,
  limit = 2,
): FolderSummaryStore {
  const states = new Map<string, FolderSummaryState>();
  const queue: string[] = [];
  const listeners = new Set<() => void>();
  const controller = new AbortController();
  let active = 0;
  const publish = (path: string, state: FolderSummaryState) => {
    if (controller.signal.aborted) return;
    states.set(path, state);
    for (const listener of listeners) listener();
  };
  const read = async (path: string) => {
    if (reader) return summarizeFolder(await reader(space, path, "", controller.signal));
    const page = await api<DirectoryPage>(
      `/browser?${new URLSearchParams({ space_id: space, path, sort: "date", order: "desc" })}`,
      "GET",
      undefined,
      {},
      controller.signal,
    );
    return summarizeFolder(page.entries, page.total, !page.next_cursor);
  };
  const pump = () => {
    while (active < limit && queue.length && !controller.signal.aborted) {
      const path = queue.shift()!;
      active++;
      publish(path, LOADING);
      read(path)
        .then((summary) => publish(path, { status: "ready", summary }))
        .catch(() => publish(path, ERROR))
        .finally(() => {
          active--;
          pump();
        });
    }
  };
  return {
    get: (path) => states.get(path) ?? IDLE,
    want(path) {
      if (states.has(path) || queue.includes(path)) return;
      queue.push(path);
      pump();
    },
    subscribe(listener) {
      listeners.add(listener);
      return () => listeners.delete(listener);
    },
    dispose() {
      controller.abort();
      queue.length = 0;
      listeners.clear();
    },
  };
}

/** A fresh store per scope; created in an effect so StrictMode never reuses a disposed one. */
export function useFolderSummaryStore(
  enabled: boolean,
  space: string,
  prefix: string,
  revision: unknown,
  reader?: DirectoryReader,
) {
  const [store, setStore] = useState<FolderSummaryStore>();
  useEffect(() => {
    if (!enabled || !space) {
      setStore(undefined);
      return;
    }
    const next = createFolderSummaryStore(space, reader);
    setStore(next);
    return () => next.dispose();
  }, [enabled, space, prefix, revision, reader]);
  return store;
}

const unsubscribed = () => () => {};

export function useFolderSummary(
  store: FolderSummaryStore | undefined,
  path: string,
  host: RefObject<Element | null>,
) {
  const state = useSyncExternalStore(
    store ? store.subscribe : unsubscribed,
    () => store?.get(path) ?? IDLE,
    () => IDLE,
  );
  useEffect(() => {
    if (!store) return;
    const element = host.current;
    if (!element || typeof IntersectionObserver === "undefined") {
      store.want(path);
      return;
    }
    const observer = new IntersectionObserver(
      (entries) => {
        if (entries.some((entry) => entry.isIntersecting)) {
          store.want(path);
          observer.disconnect();
        }
      },
      { rootMargin: "200px" },
    );
    observer.observe(element);
    return () => observer.disconnect();
  }, [store, path, host]);
  return state;
}
