import { useEffect } from "react";
import type { DriveFile } from "../../api";
import { api, errorText } from "../../api";

export interface BrowserEntry {
  kind: "file" | "directory";
  path: string;
  file?: DriveFile;
}
export interface DirectoryPage {
  entries: BrowserEntry[];
  next_cursor: string;
  coverage: string;
  total: number;
}
export type DirectoryReader = (space: string, path: string, query: string, signal: AbortSignal) => Promise<BrowserEntry[]>;

interface Options {
  space: string;
  prefix: string;
  query: string;
  cursor: string;
  sort: string;
  order: string;
  revision: unknown;
  reader?: DirectoryReader;
  onStart: () => void;
  onPage: (page: Pick<DirectoryPage, "entries" | "next_cursor" | "total">) => void;
  onError: (message: string) => void;
  onFinish: () => void;
}

// One directory observation per scope. A cancelled read never publishes state,
// including when an adapter resolves despite AbortSignal cancellation.
export function useDirectoryRead(options: Options) {
  const { space, prefix, query, cursor, sort, order, revision, reader,
    onStart, onPage, onError, onFinish } = options;
  useEffect(() => {
    if (!space) {
      onStart();
      onPage({ entries: [], next_cursor: "", total: 0 });
      onFinish();
      return;
    }
    const controller = new AbortController();
    onStart();
    const timer = setTimeout(() => {
      void (async () => {
        try {
          const result = reader
            ? await reader(space, prefix, query, controller.signal).then(entries => ({ entries, next_cursor: "", total: entries.length }))
            : await api<DirectoryPage>(`/browser?${new URLSearchParams({ space_id: space, path: prefix, q: query, cursor, sort, order })}`, "GET", undefined, {}, controller.signal);
          if (!controller.signal.aborted) onPage(result);
        } catch (error) {
          if (!controller.signal.aborted) onError(errorText(error));
        } finally {
          if (!controller.signal.aborted) onFinish();
        }
      })();
    }, query ? 200 : 0);
    return () => { clearTimeout(timer); controller.abort(); };
    // Callbacks publish into the consuming view; only request inputs restart a read.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [space, prefix, query, cursor, sort, order, revision, reader]);
}
