import { Button } from "./components/ui/button";
import React, { useEffect, useState } from "react";
import { ChevronDown, ChevronRight, Folder } from "lucide-react";
import { api, errorText } from "./api";

interface DirRow {
  kind: string;
  path: string;
}

async function listFolders(
  space: string,
  path: string,
  adapter?: { directory: (space: string, path: string, query: string, signal: AbortSignal) => Promise<DirRow[]> },
) {
  const rows = adapter
    ? await adapter.directory(space, path, "", new AbortController().signal)
    : (
        await api<{ entries: DirRow[] }>(
          `/browser?${new URLSearchParams({ space_id: space, path })}`,
        )
      ).entries;
  return rows.filter((row) => row.kind === "directory");
}

export function FolderPicker({
  space,
  adapter,
  value,
  onChange,
  exclude = [],
}: {
  space: string;
  adapter?: {
    directory: (
      space: string,
      path: string,
      query: string,
      signal: AbortSignal,
    ) => Promise<DirRow[]>;
  };
  value: string;
  onChange: (path: string) => void;
  exclude?: string[];
}) {
  const [tree, setTree] = useState<Record<string, DirRow[]>>({});
  const [error, setError] = useState("");
  const blocked = (path: string) =>
    exclude.some((src) => path === src || path.startsWith(src + "/"));
  const load = (path: string) => {
    void listFolders(space, path, adapter)
      .then((rows) => {
        setTree((cur) => ({ ...cur, [path]: rows }));
        setError("");
      })
      .catch((e) => setError(errorText(e)));
  };
  useEffect(() => {
    load("");
  }, [space]);
  const render = (path: string, depth: number) => {
    const rows = tree[path];
    if (!rows) return null;
    return rows.map((row) => {
      const open = tree[row.path] !== undefined;
      const disabled = blocked(row.path);
      return (
        <li key={row.path}>
          <div
            className={`picker-row ${value === row.path ? "selected" : ""} ${disabled ? "disabled" : ""}`}
            style={{ paddingLeft: 8 + depth * 16 }}
          >
            <Button variant="ghost"
              type="button"
              className="tree-expander"
              aria-label={(open ? "收起 " : "展开 ") + row.path}
              aria-expanded={open}
              onClick={() => (open ? setTree((cur) => {
                const next = { ...cur };
                delete next[row.path];
                return next;
              }) : load(row.path))}
            >
              {open ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
            </Button>
            <Button variant="ghost"
              type="button"
              disabled={disabled}
              onClick={() => onChange(row.path)}
            >
              <Folder size={15} />
              {row.path.split("/").pop()}
            </Button>
          </div>
          {open && <ul>{render(row.path, depth + 1)}</ul>}
        </li>
      );
    });
  };
  return (
    <div className="folder-picker" role="tree" aria-label="选择目标目录">
      <Button variant="ghost"
        type="button"
        className={`picker-row ${value === "" ? "selected" : ""}`}
        onClick={() => onChange("")}
      >
        <Folder size={15} />
        根目录
      </Button>
      {error && (
        <p className="error" role="alert">
          {error}
        </p>
      )}
      <ul>{render("", 1)}</ul>
    </div>
  );
}
