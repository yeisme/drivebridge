import type { State, FileRecord } from "./model";
import type { DirectoryNode } from "./storage-model";
import { rootNode } from "./storage-model";
// Deterministic paged fixture. Only requested rows are materialized in prototype state.
export function largeCatalogPage(
  s: State,
  parent: number | null,
  page: number,
  query = "",
) {
  const root = rootNode(s, s.activeConnection)!;
  const scope = `${s.space}:${root.connectionId}`;
  const folder = (i: number): DirectoryNode => ({
    id: `large:${scope}:d${i}`,
    space: s.space,
    connectionId: root.connectionId,
    rootId: root.rootId,
    parentId: root.id,
    name: `项目 ${String(i + 1).padStart(4, "0")}`,
    path: `规模演示 / 项目 ${String(i + 1).padStart(4, "0")}`,
    status: "ready",
  });
  const file = (i: number): FileRecord => {
    const dir = folder(Math.floor(i / 10));
    return {
      id: `large:${scope}:f${i}`,
      space: s.space,
      connectionId: root.connectionId,
      rootId: root.rootId,
      directoryId: dir.id,
      name: `研究资料 ${String(i + 1).padStart(5, "0")}.pdf`,
      folder: dir.path,
      kind: "document",
      size: "240 KB",
      version: 1,
      origin: "cloud",
      favorite: false,
      tags: [],
      updatedAt: "2026-09-28",
    };
  };
  const matches: number[] = [];
  const q = query.toLowerCase();
  const type = parent !== null || q ? "file" : "directory";
  const start = parent !== null ? parent * 10 : 0,
    end = parent !== null ? start + 10 : q ? 10000 : 1000;
  for (let i = start; i < end; i++) {
    const name =
      type === "file"
        ? `研究资料 ${String(i + 1).padStart(5, "0")}.pdf 项目 ${String(Math.floor(i / 10) + 1).padStart(4, "0")}`
        : `项目 ${String(i + 1).padStart(4, "0")}`;
    if (!q || name.toLowerCase().includes(q)) matches.push(i);
  }
  const indices = matches.slice((page - 1) * 50, page * 50);
  return {
    total: matches.length,
    files: type === "file" ? indices.map(file) : [],
    directories:
      type === "directory"
        ? indices.map(folder)
        : [...new Set(indices.map((i) => Math.floor(i / 10)))].map(folder),
    parent: parent === null ? null : folder(parent),
  };
}
