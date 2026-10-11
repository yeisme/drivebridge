import type { ReactNode } from "react";
import { Input } from "./ui/input";
export function DataTableToolbar({
  label,
  query,
  onQueryChange,
  children,
  searchable = true,
}: {
  searchable?: boolean;
  label: string;
  query: string;
  onQueryChange: (query: string) => void;
  children: ReactNode;
}) {
  return (
    <div className={`flex flex-wrap items-center gap-2 ${searchable ? "" : "justify-end"}`}>
      {searchable && <Input
        aria-label={`搜索${label}`}
        type="search"
        placeholder="搜索当前结果…"
        className="min-w-36 flex-1"
        value={query}
        onChange={(event) => onQueryChange(event.target.value)}
      />}
      {children}
    </div>
  );
}
