import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem } from "./components/ui/dropdown-menu";
import { Popover, PopoverTrigger, PopoverContent } from "./components/ui/popover";
import { Textarea } from "./components/ui/textarea";
import { Checkbox } from "./components/ui/checkbox";
import { Input } from "./components/ui/input";
import { SearchField } from "./components/SearchField";
import { Button } from "./components/ui/button";
import { SelectAdapter } from "./components/SelectAdapter";
import { AppDialog as WorkspaceDialog } from "./components/AppDialog";
import React, { useEffect, useId, useRef, useState } from "react";
import { useMobileViewport } from "./app/useMobileViewport";
import { FinderDirectoryPanel } from "./features/files/FinderDirectoryPanel";
import {
  SlidersHorizontal,
  Plus,
  PanelLeft,
  ArrowLeft,
  ArrowRight,
  ChevronDown,
  ChevronRight,
  Folder,
  FolderPlus,
  FolderOpen,
  FolderInput,
  FilePlus2,
  Eye,
  Info,
  LayoutGrid,
  Columns3,
  GalleryHorizontal,
  List,
  Search,
  Upload,
  X,
  Share2,
  Copy,
  Scissors,
  CopyPlus,
  Pencil,
  Trash2,
  ArrowDownToLine,
  MoreHorizontal,
  Star,
  RotateCw,
  Cloud,
} from "lucide-react";
import {
  contextURL,
  upload as saveFile,
  type Operation,
  api,
  bytes,
  copyReference,
  download,
  errorText,
  type DriveFile,
  type Space,
} from "./api";
import { MediaPreview, MediaThumbnail } from "./Media";
import { FinderMenu, newEntryError, type MenuItem } from "./FinderMenu";
import { FolderPicker } from "./FolderPicker";
import { moveNotice, type LastMove } from "./finder-undo";
import {
  clipboardFromFiles,
  clipboardNotice,
  copyDisabledReason,
  cutDisabledReason,
  isCutItem,
  joinPath,
  listedNames,
  needsPasteConfirm,
  pasteDisabledReason,
  type FileClipboard,
} from "./finder-clipboard";
import {
  finderCommand,
  isTypeaheadKey,
  shortcutLabel,
  typeaheadIndex,
  isTypingTarget,
  isMediaTarget,
} from "./finder-keys";

export { AppDialog as WorkspaceDialog } from "./components/AppDialog";
import { useDirectoryRead, type BrowserEntry, type DirectoryPage } from "./features/files/useDirectoryRead";
export type { BrowserEntry } from "./features/files/useDirectoryRead";
export interface FinderAdapter {
  directory: typeof directory;
  createFile?: (
    space: string,
    path: string,
    file: File,
    signal: AbortSignal,
  ) => Promise<void>;
  mkdir: (space: string, path: string) => Promise<unknown>;
  move: (file: DriveFile, path: string) => Promise<unknown>;
  trash: (file: DriveFile) => Promise<unknown>;
  copy?: (file: DriveFile, space: string, path: string) => Promise<unknown>;
  copyDirectory?: (
    space: string,
    path: string,
    destSpace: string,
    destPath: string,
  ) => Promise<unknown>;
  moveDirectory?: (space: string, path: string, destPath: string) => Promise<unknown>;
  trashDirectory?: (space: string, path: string) => Promise<unknown>;
}
async function directory(
  space: string,
  path: string,
  query: string,
  signal: AbortSignal,
): Promise<BrowserEntry[]> {
  const entries: BrowserEntry[] = [];
  let cursor = "";
  do {
    const result = await api<DirectoryPage>(
      `/browser?${new URLSearchParams({ space_id: space, path, q: query, cursor })}`,
      "GET",
      undefined,
      {},
      signal,
    );
    entries.push(...result.entries);
    cursor = result.next_cursor;
  } while (cursor && !signal.aborted);
  return entries;
}
function TreeNode({
  space,
  path,
  selected,
  navigate,
  revision,
  adapter,
  onContext,
  onDropFiles,
}: {
  space: string;
  path: string;
  selected: string;
  navigate: (path: string) => void;
  revision: unknown;
  adapter?: FinderAdapter;
  onContext?: (e: React.MouseEvent, path: string) => void;
  onDropFiles?: (e: React.DragEvent, path: string) => void;
}) {
  const [expanded, setExpanded] = useState(
    path === "" || selected.startsWith(path + "/"),
  );
  const [children, setChildren] = useState<BrowserEntry[]>([]);
  const [error, setError] = useState("");
  const [over, setOver] = useState(false);
  useEffect(() => {
    if (!expanded) return;
    const controller = new AbortController();
    (adapter?.directory || directory)(space, path, "", controller.signal)
      .then((rows) => {
        if (!controller.signal.aborted) {
          setChildren(rows.filter((row) => row.kind === "directory"));
          setError("");
        }
      })
      .catch((e) => {
        if (!controller.signal.aborted) setError(errorText(e));
      });
    return () => controller.abort();
  }, [expanded, space, path, revision]);
  return (
    <li>
      <div
        className={`tree-row ${selected === path ? "selected" : ""} ${over ? "drop-target" : ""}`}
        onContextMenu={(e) => onContext?.(e, path)}
        onDragOver={(e) => {
          if (onDropFiles && e.dataTransfer.types.includes("application/x-drivebridge-files")) {
            e.preventDefault();
            e.stopPropagation();
            e.dataTransfer.dropEffect = "move";
            setOver(true);
          }
        }}
        onDragLeave={(e) => {
          if (!e.currentTarget.contains(e.relatedTarget as Node | null))
            setOver(false);
        }}
        onDrop={(e) => {
          setOver(false);
          onDropFiles?.(e, path);
        }}
      >
        <Button layout="surface" variant="ghost" type="submit"
          className="tree-expander"
          aria-label={(expanded ? "收起 " : "展开 ") + (path || "根目录")}
          aria-expanded={expanded}
          onClick={() => setExpanded(!expanded)}
        >
          {expanded ? <ChevronDown size={13} /> : <ChevronRight size={13} />}
        </Button>
        <Button layout="surface" variant="ghost" type="submit"
          onClick={() => navigate(path)}
          aria-current={selected === path ? "location" : undefined}
        >
          <Folder size={15} />
          <span>{path.split("/").pop() || "全部文件"}</span>
        </Button>
      </div>
      {expanded && (
        <ul>
          {error && <li className="error">{error}</li>}
          {children.map((row) => (
            <TreeNode
              key={row.path}
              {...{
                space,
                selected,
                navigate,
                revision,
                adapter,
                onContext,
                onDropFiles,
              }}
              path={row.path}
            />
          ))}
        </ul>
      )}
    </li>
  );
}
export function Finder({
  spaces,
  space,
  setSpace,
  prefix,
  setPrefix,
  revision,
  reload,
  upload,
  onPreview,
  onShare,
  onOperation,
  backendLabel,
  focusAction,
  adapter,
  readOnly = false,
  managementRestricted = false,
  showSpaceSwitcher = true,
  onedriveConnections = [],
}: {
  focusAction?: React.ReactNode;
  readOnly?: boolean;
  managementRestricted?: boolean;
  showSpaceSwitcher?: boolean;
  onedriveConnections?: { id: string; name: string }[];
  spaces: Space[];
  space: string;
  setSpace: (id: string) => void;
  prefix: string;
  setPrefix: (path: string) => void;
  revision: unknown;
  adapter?: FinderAdapter;
  reload: () => Promise<void>;
  upload: (files?: File[]) => void;
  onPreview: (file: DriveFile, siblings: DriveFile[]) => void;
  onShare: (files: DriveFile[], path: string) => void;
  onOperation?: (op: Operation) => void;
  backendLabel: string;
}) {
  const [entries, setEntries] = useState<BrowserEntry[]>([]),
    [query, setQuery] = useState(""),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false);
  const [cursor, setCursor] = useState(""),
    [nextCursor, setNextCursor] = useState(""),
    [total, setTotal] = useState(0);
  useEffect(() => setCursor(""), [space, prefix, query]);
  const [selection, setSelection] = useState<string[]>([]),
    [folderSelection, setFolderSelection] = useState<string[]>([]),
    [sort, setSort] = useState("name"),
    [sortOrder, setSortOrder] = useState<"asc" | "desc">("asc"),
    [grid, setGrid] = useState(false),
    [viewMode, setViewMode] = useState<"list" | "grid" | "columns" | "gallery">("list"),
    [filter, setFilter] = useState("all");
  const filterControls = <>
          <label className="grid min-w-0 gap-1.5"><span className="text-xs text-muted-foreground">文件类型</span><SelectAdapter
            className="w-full"
            aria-label="文件类型"
            value={filter}
            onChange={(e) => setFilter(e.target.value)}
          >
            <option value="all">全部类型</option>
            <option value="image">图片</option>
            <option value="video">视频</option>
            <option value="audio">音频</option>
            <option value="text">文本</option>
          </SelectAdapter></label>
          <label className="grid min-w-0 gap-1.5"><span className="text-xs text-muted-foreground">文件排序</span><SelectAdapter
            className="w-full"
            aria-label="文件排序"
            value={sort}
            onChange={(e) => { const next = e.target.value; setCursor(""); setSort(next); setSortOrder(next === sort ? (sortOrder === "asc" ? "desc" : "asc") : "asc"); }}
          >
            <option value="name">按名称</option>
            <option value="date">最近修改</option>
            <option value="size">按大小</option>
          </SelectAdapter></label>
  </>;
  const [action, setAction] = useState(""),
    [value, setValue] = useState(""),
    [notice, setNotice] = useState("");
  const [clipboard, setClipboard] = useState<FileClipboard>(),
    [focusIndex, setFocusIndex] = useState(0),
    [anchorIndex, setAnchorIndex] = useState(0),
    [helpOpen, setHelpOpen] = useState(false),
    [lastMove, setLastMove] = useState<LastMove>(),
    [exportConnection, setExportConnection] = useState("");
  const [menu, setMenu] = useState<{
    x: number;
    y: number;
    entry?: BrowserEntry;
  }>();
  const [creationDir, setCreationDir] = useState(prefix),
    [multipleDirectories, setMultipleDirectories] = useState(false),
    [openCreated, setOpenCreated] = useState(false);
  const [textContent, setTextContent] = useState(""),
    [fileType, setFileType] = useState("txt");
  const [folderInfo, setFolderInfo] = useState<string>();
  const [folderEntries, setFolderEntries] = useState<
    BrowserEntry[] | undefined
  >();
  const [folderError, setFolderError] = useState("");
  const [dragging, setDragging] = useState(false);
  const operationAbort = useRef<AbortController | undefined>(undefined);
  const typeBuffer = useRef("");
  const typeAt = useRef(0);
  const listRef = useRef<HTMLDivElement>(null);
  const scope = useRef({ space, prefix });
  scope.current = { space, prefix };
  useEffect(() => {
    setMenu(undefined);
    setAction("");
    setFolderInfo(undefined);
    setDragging(false);
    setError("");
    operationAbort.current?.abort();
    return () => operationAbort.current?.abort();
  }, [space, prefix]);
  useEffect(() => {
    if (folderInfo === undefined) return;
    const abort = new AbortController();
    setFolderEntries(undefined);
    setFolderError("");
    const request = adapter
      ? adapter.directory(space, folderInfo, "", abort.signal)
      : api<DirectoryPage>(
          `/browser?${new URLSearchParams({ space_id: space, path: folderInfo })}`,
          "GET",
          undefined,
          {},
          abort.signal,
        ).then((r) => r.entries);
    void request
      .then((rows) => {
        if (!abort.signal.aborted) setFolderEntries(rows);
      })
      .catch((e) => {
        if (!abort.signal.aborted) setFolderError(errorText(e));
      });
    return () => abort.abort();
  }, [folderInfo, space]);
  const beginCreate = (kind: "folder" | "file", parent = prefix) => {
    setCreationDir(parent);
    setMultipleDirectories(false);
    setOpenCreated(false);
    setTextContent("");
    setFileType("txt");
    setError("");
    const base = kind === "folder" ? "新建文件夹" : "新建文档";
    const ext = kind === "folder" ? "" : ".txt";
    let name = base + ext,
      index = 2;
    while (
      entries.some((e) => e.path === [parent, name].filter(Boolean).join("/"))
    )
      name = `${base} (${index++})${ext}`;
    setValue(name);
    setAction(kind);
  };
  const creating = action === "folder" || action === "file";
  const nameError =
    creating || action === "rename"
      ? newEntryError(value, action === "folder" && multipleDirectories) || (action === "file" && adapter && !textContent.length ? "本机连接暂不支持空文件，请填写文件内容。" : "")
      : "";
  const contextMenu = (e: React.MouseEvent, entry?: BrowserEntry) => {
    e.preventDefault();
    e.stopPropagation();
    if (entry?.file && !selection.includes(entry.file.id))
      setSelection([entry.file.id]);
    if (entry?.kind === "directory" && !folderSelection.includes(entry.path))
      setFolderSelection([entry.path]);
    const rect = e.currentTarget.getBoundingClientRect();
    setMenu({
      x: e.clientX || rect.left + 12,
      y: e.clientY || rect.top + 12,
      entry,
    });
  };
  const [navigation, setNavigation] = useState<string[]>([""]),
    [position, setPosition] = useState(0);
  const mobile = useMobileViewport();
  const [mobileTreeOpen, setMobileTreeOpen] = useState(false);
  const treeToggle = useRef<HTMLButtonElement>(null);
  const treeID = useId();
  const [treeCollapsed, setTreeCollapsed] = useState(() => {
    try {
      return localStorage.getItem("drivebridge:tree-collapsed") === "true";
    } catch {
      return false;
    }
  });
  const treeOpen = mobile ? mobileTreeOpen : !treeCollapsed;
  useEffect(() => { setMobileTreeOpen(false); }, [mobile, space, prefix]);
  useEffect(() => {
    try {
      localStorage.setItem("drivebridge:tree-collapsed", String(treeCollapsed));
    } catch {
      // Optional preference only; the selected layout remains in React state.
    }
  }, [treeCollapsed]);
  const [favorites, setFavorites] = useState<string[]>([]);
  const preferenceKey = `drivebridge:view:${space}:${prefix}`;
  const navigate = (path: string) => {
    setMobileTreeOpen(false);
    setPrefix(path);
    setSelection([]);
    setFolderSelection([]);
    setQuery("");
    setNavigation([...navigation.slice(0, position + 1), path]);
    setPosition(position + 1);
  };
  useEffect(() => {
    setNavigation([""]);
    setPosition(0);
    setSelection([]);
    setFolderSelection([]);
    setEntries([]);
    setQuery("");
  }, [space]);
  useEffect(() => {
    try {
      const saved = localStorage.getItem(preferenceKey);
      setGrid(saved === "grid");
      setViewMode(saved === "grid" ? "grid" : saved === "columns" || saved === "gallery" ? saved : "list");
      setFavorites(
        JSON.parse(
          localStorage.getItem(`drivebridge:favorites:${space}`) || "[]",
        ),
      );
    } catch {
      setGrid(false);
      setFavorites([]);
    }
  }, [preferenceKey, space]);
  useDirectoryRead({ space, prefix, query, cursor, sort, order: sortOrder, revision, reader: adapter?.directory,
    onStart: () => { setBusy(true); setError(""); setEntries([]); setSelection([]); },
    onPage: result => { setEntries(result.entries); setNextCursor(result.next_cursor); setTotal(result.total); },
    onError: setError,
    onFinish: () => setBusy(false),
  });
  const chooseMode = (mode: "list" | "grid" | "columns" | "gallery") => {
    setViewMode(mode);
    setGrid(mode === "grid");
    try { localStorage.setItem(preferenceKey, mode); } catch {
      // Optional preference only; the selected view remains in React state.
    }
  };
  const toggleFavorite = () => {
    const next = favorites.includes(prefix)
      ? favorites.filter((p) => p !== prefix)
      : [...favorites, prefix];
    setFavorites(next);
    try {
      localStorage.setItem(
        `drivebridge:favorites:${space}`,
        JSON.stringify(next),
      );
    } catch {
      /* Preferences are optional. */
    }
  };
  const files = entries
    .flatMap((row) => (row.file ? [row.file] : []))
    .filter((f) => filter === "all" || f.mime_type.startsWith(filter + "/"));
  if (adapter) files.sort((a, b) => {
    const difference = sort === "size" ? a.size_bytes - b.size_bytes : sort === "date" ? a.updated_at.localeCompare(b.updated_at) : a.path.localeCompare(b.path, "zh-CN");
    return difference * (sortOrder === "desc" ? -1 : 1);
  });
  const selected = files.filter((file) => selection.includes(file.id));
  const [inspectorOpen, setInspectorOpen] = useState(false);
  const inspected = inspectorOpen && selected.length === 1 ? selected[0] : undefined;
  useEffect(() => setInspectorOpen(false), [space, prefix]);
  const act = async (fn: () => Promise<void>) => {
    setBusy(true);
    setError("");
    try {
      await fn();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  };
  const select = (file: DriveFile, multiple: boolean) =>
    setSelection((ids) =>
      multiple
        ? ids.includes(file.id)
          ? ids.filter((id) => id !== file.id)
          : [...ids, file.id]
        : [file.id],
    );
  const reportCopy = (op: Operation) => onOperation?.(op);
  const moveFiles = async (target: string, chosen = selected) => {
    if (readOnly) throw new Error("approval_required");
    const filesMoved = [];
    for (const f of chosen) {
      const destination =
        (target ? target + "/" : "") + f.path.split("/").pop();
      if (destination === f.path) continue;
      if (adapter) await adapter.move(f, destination);
      else
        await api(`/files/${f.id}/move`, "POST", {
          path: destination,
          expected_version: f.version_id,
        });
      filesMoved.push({
        id: f.id,
        from: f.path,
        to: destination,
        version_id: f.version_id,
      });
    }
    if (filesMoved.length)
      setLastMove({ space_id: space, files: filesMoved, folders: [] });
    await reload();
  };
  const moveSelection = async (target: string) => {
    if (readOnly) throw new Error("approval_required");
    const filesMoved: LastMove["files"] = [];
    const foldersMoved: LastMove["folders"] = [];
    for (const folder of chosenFolders()) {
      const destination = joinPath(target, folder.split("/").pop() || folder);
      if (destination === folder || destination.startsWith(folder + "/"))
        continue;
      if (adapter?.moveDirectory)
        await adapter.moveDirectory(space, folder, destination);
      else {
        const out = await api<{ path: string }>("/directories/move", "POST", {
          space_id: space,
          path: folder,
          target_path: destination,
          conflict: "rename",
        });
        foldersMoved.push({ from: folder, to: out.path || destination });
        continue;
      }
      foldersMoved.push({ from: folder, to: destination });
    }
    for (const f of chosenFiles()) {
      const destination = joinPath(target, f.path.split("/").pop() || f.path);
      if (destination === f.path) continue;
      if (adapter) await adapter.move(f, destination);
      else
        await api(`/files/${f.id}/move`, "POST", {
          path: destination,
          expected_version: f.version_id,
        });
      filesMoved.push({
        id: f.id,
        from: f.path,
        to: destination,
        version_id: f.version_id,
      });
    }
    if (filesMoved.length || foldersMoved.length) {
      const move = { space_id: space, files: filesMoved, folders: foldersMoved };
      setLastMove(move);
      setNotice(moveNotice(move));
    }
    await reload();
  };
  const beginExport = () => {
    if (!onedriveConnections.length) {
      setError("请先在存储连接中授权 OneDrive");
      return;
    }
    setExportConnection(onedriveConnections[0].id);
    setAction("export");
  };
  const runExport = async (connection: string) => {
    for (const folder of chosenFolders()) {
      const op = await api<Operation>(
        "/directories/export",
        "POST",
        { space_id: space, path: folder, connection_id: connection },
        { "Idempotency-Key": crypto.randomUUID() },
      );
      reportCopy(op);
    }
    for (const f of chosenFiles()) {
      const op = await api<Operation>(
        "/connections/" + connection + "/export",
        "POST",
        { file_id: f.id },
        { "Idempotency-Key": crypto.randomUUID() },
      );
      reportCopy(op);
    }
    setNotice("已提交复制到 OneDrive，可在传输中心查看进度");
  };
  const undoMove = async () => {
    if (!lastMove || lastMove.space_id !== space) return;
    const undo = lastMove;
    setLastMove(undefined);
    try {
      for (const folder of [...undo.folders].reverse()) {
        if (adapter?.moveDirectory)
          await adapter.moveDirectory(space, folder.to, folder.from);
        else
          await api("/directories/move", "POST", {
            space_id: space,
            path: folder.to,
            target_path: folder.from,
            conflict: "fail",
          });
      }
      for (const f of undo.files) {
        if (adapter)
          await adapter.move(
            { id: f.id, path: f.to, version_id: f.version_id } as DriveFile,
            f.from,
          );
        else
          await api(`/files/${f.id}/move`, "POST", {
            path: f.from,
            expected_version: f.version_id,
          });
      }
    } catch (e) {
      setLastMove(undo);
      throw e;
    }
    await reload();
    setNotice("已撤销移动");
  };
  const canCopy = !adapter || !!adapter.copy;
  const canCopyFolders = !adapter || !!adapter.copyDirectory;
  const canMoveFolders = !adapter || !!adapter.moveDirectory;
  const folders = query
    ? []
    : entries.filter((row) => row.kind === "directory");
  const listed: BrowserEntry[] = [
    ...folders,
    ...files.map((f) => ({ kind: "file" as const, path: f.path, file: f })),
  ];
  const focused = listed[Math.min(focusIndex, Math.max(listed.length - 1, 0))];
  const chosenFiles = () => {
    if (selected.length) return selected;
    if (folderSelection.length) return [];
    return focused?.file ? [focused.file] : [];
  };
  const chosenFolders = () => {
    if (folderSelection.length) return folderSelection;
    if (selected.length) return [];
    return focused?.kind === "directory" ? [focused.path] : [];
  };
  const rememberClipboard = (mode: "copy" | "cut") => {
    const board = clipboardFromFiles(
      mode,
      space,
      chosenFiles(),
      chosenFolders(),
    );
    if (!board) return;
    setClipboard(board);
    setNotice(clipboardNotice(board));
    void navigator.clipboard
      .writeText(board.items.map((item) => item.name).join("\n"))
      .catch(() => {});
  };
  const runPaste = async (dir: string) => {
    if (!clipboard) return;
    const blocked = pasteDisabledReason(
      clipboard,
      space,
      readOnly,
      canCopy,
      canCopyFolders,
      canMoveFolders,
    );
    if (blocked) throw new Error(blocked === "当前网盘仅供浏览" ? "approval_required" : "unsupported");
    for (const item of clipboard.items) {
      const destination = joinPath(dir, item.name);
      if (item.kind === "directory") {
        if (clipboard.mode === "cut") {
          if (item.path === destination) continue;
          if (destination.startsWith(item.path + "/"))
            throw new Error("invalid_directory");
          if (adapter?.moveDirectory)
            await adapter.moveDirectory(clipboard.space_id, item.path, destination);
          else
            await api("/directories/move", "POST", {
              space_id: clipboard.space_id,
              path: item.path,
              target_path: destination,
              conflict: "rename",
            });
        } else if (adapter?.copyDirectory) {
          await adapter.copyDirectory(clipboard.space_id, item.path, space, destination);
        } else {
          const op = await api<Operation>(
            "/directories/copy",
            "POST",
            {
              space_id: clipboard.space_id,
              path: item.path,
              target_space_id: space,
              target_path: destination,
              conflict: "rename",
            },
            { "Idempotency-Key": crypto.randomUUID() },
          );
          reportCopy(op);
        }
        continue;
      }
      if (clipboard.mode === "cut") {
        if (item.path === destination) continue;
        const file = files.find((f) => f.id === item.id) || {
          id: item.id,
          path: item.path,
          version_id: item.version_id,
        };
        if (adapter) await adapter.move(file as DriveFile, destination);
        else
          await api(`/files/${item.id}/move`, "POST", {
            path: destination,
            expected_version: item.version_id,
          });
      } else if (adapter?.copy) {
        const file = files.find((f) => f.id === item.id) || {
          id: item.id,
          ref: item.id,
          path: item.path,
          version_id: item.version_id,
        };
        await adapter.copy(file as DriveFile, space, destination);
      } else {
        const op = await api<Operation>(
          `/files/${item.id}/copy`,
          "POST",
          {
            target_space_id: space,
            target_path: destination,
            expected_version: item.version_id,
            conflict: "rename",
          },
          { "Idempotency-Key": crypto.randomUUID() },
        );
        reportCopy(op);
      }
    }
    if (clipboard.mode === "cut") {
      setClipboard(undefined);
      setLastMove({
        space_id: space,
        files: clipboard.items
          .filter(
            (item) =>
              item.kind !== "directory" && item.path !== joinPath(dir, item.name),
          )
          .map((item) => ({
            id: item.id,
            from: item.path,
            to: joinPath(dir, item.name),
            version_id: item.version_id,
          })),
        folders: clipboard.items
          .filter(
            (item) =>
              item.kind === "directory" && item.path !== joinPath(dir, item.name),
          )
          .map((item) => ({
            from: item.path,
            to: joinPath(dir, item.name),
          })),
      });
    }
    await reload();
    setNotice(
      clipboard.mode === "cut"
        ? `已移动 ${clipboard.items.length} 项`
        : `已粘贴 ${clipboard.items.length} 项`,
    );
  };
  const beginPaste = (dir = prefix) => {
    const blocked = pasteDisabledReason(
      clipboard,
      space,
      readOnly,
      canCopy,
      canCopyFolders,
      canMoveFolders,
    );
    if (blocked) {
      setError(blocked);
      return;
    }
    if (needsPasteConfirm(clipboard!.items.length)) {
      setValue(dir);
      setAction("paste");
      return;
    }
    void act(() => runPaste(dir));
  };
  const beginDuplicate = () => {
    const filesToCopy = chosenFiles();
    const foldersToCopy = chosenFolders();
    if (!filesToCopy.length && !foldersToCopy.length) return;
    const blocked = copyDisabledReason(
      readOnly,
      canCopy,
      foldersToCopy.length > 0,
      canCopyFolders,
    );
    if (blocked) {
      setError(blocked);
      return;
    }
    void act(async () => {
      for (const folder of foldersToCopy) {
        const destination = joinPath(prefix, folder.split("/").pop() || folder);
        if (adapter?.copyDirectory)
          await adapter.copyDirectory(space, folder, space, destination);
        else {
          const op = await api<Operation>(
            "/directories/copy",
            "POST",
            {
              space_id: space,
              path: folder,
              target_space_id: space,
              target_path: destination,
              conflict: "rename",
            },
            { "Idempotency-Key": crypto.randomUUID() },
          );
          reportCopy(op);
        }
      }
      for (const file of filesToCopy) {
        const destination = joinPath(prefix, file.path.split("/").pop() || file.path);
        if (adapter?.copy) await adapter.copy(file, space, destination);
        else {
          const op = await api<Operation>(
            `/files/${file.id}/copy`,
            "POST",
            {
              target_space_id: space,
              target_path: destination,
              expected_version: file.version_id,
              conflict: "rename",
            },
            { "Idempotency-Key": crypto.randomUUID() },
          );
          reportCopy(op);
        }
      }
      await reload();
      setNotice(`已创建 ${filesToCopy.length + foldersToCopy.length} 个副本`);
    });
  };
  const selectRange = (to: number) => {
    const from = anchorIndex;
    const start = Math.min(from, to);
    const end = Math.max(from, to);
    const slice = listed.slice(start, end + 1);
    setSelection(slice.flatMap((row) => (row.file ? [row.file.id] : [])));
    setFolderSelection(
      slice.flatMap((row) => (row.kind === "directory" ? [row.path] : [])),
    );
    setFocusIndex(to);
  };
  const selectFolder = (path: string, multiple: boolean) =>
    setFolderSelection((paths) =>
      multiple
        ? paths.includes(path)
          ? paths.filter((p) => p !== path)
          : [...paths, path]
        : [path],
    );
  useEffect(() => {
    listRef.current
      ?.querySelector<HTMLElement>(".focused")
      ?.scrollIntoView({ block: "nearest" });
  }, [focusIndex, listed.length]);
  const fileDragType = "application/x-drivebridge-files";
  const internalFileDrag = (event: React.DragEvent) => event.dataTransfer.types.includes(fileDragType);
  const startFileDrag = (event: React.DragEvent, id: string) => {
    if (readOnly || busy) { event.preventDefault(); return; }
    const ids = selection.includes(id) ? selection : [id];
    setMenu(undefined);
    event.dataTransfer.effectAllowed = "move";
    event.dataTransfer.setData(fileDragType, JSON.stringify({ space, ids }));
    const preview = document.createElement("div");
    preview.className = "finder-drag-preview";
    preview.textContent = ids.length > 1 ? `移动 ${ids.length} 个文件` : files.find(file => file.id === id)?.path.split("/").pop() || "移动文件";
    document.body.appendChild(preview);
    event.dataTransfer.setDragImage(preview, 16, 16);
    // Native drag captures this compact surface before the next frame.
    requestAnimationFrame(() => preview.remove());
  };
  const acceptFileMove = (event: React.DragEvent) => {
    if (!internalFileDrag(event)) return;
    event.preventDefault();
    event.stopPropagation();
    event.dataTransfer.dropEffect = readOnly || busy ? "none" : "move";
  };
  const dropMovedFiles = (event: React.DragEvent, target: string) => {
    if (!internalFileDrag(event)) return;
    event.preventDefault();
    event.stopPropagation();
    if (readOnly || busy) return;
    try {
      const payload = JSON.parse(event.dataTransfer.getData(fileDragType));
      if (payload.space !== space || !Array.isArray(payload.ids) || !payload.ids.every((id: unknown) => typeof id === "string")) {
        setError("请在同一网盘中拖拽移动文件。"); return;
      }
      const chosen = files.filter(file => payload.ids.includes(file.id));
      if (!chosen.length) { setError("拖拽文件已不在当前列表，请刷新后重试。"); return; }
      void act(() => moveFiles(target, chosen));
    } catch { setError("无法读取拖拽项目。"); }
  };
  const acceptOsFiles = (e: React.DragEvent) => {
    e.preventDefault();
    if (!readOnly && e.dataTransfer.types.includes("Files")) setDragging(true);
  };
  const dropOsFiles = (e: React.DragEvent) => {
    e.preventDefault();
    e.stopPropagation();
    setDragging(false);
    if (readOnly) return;
    if (e.dataTransfer.files.length) upload(Array.from(e.dataTransfer.files));
  };
  const menuFile = menu?.entry?.file;
  const menuPath = menu?.entry?.kind === "directory" ? menu.entry.path : prefix;
  const icon = (node: React.ReactNode) => node;
  const menuItems: MenuItem[] = menuFile
    ? [
        {
          label: "预览",
          icon: icon(<Eye size={16} />),
          run: () => onPreview(menuFile, files),
        },
        {
          label: "查看元数据",
          icon: icon(<Info size={16} />),
          run: () => {
            setSelection([menuFile.id]);
            setInspectorOpen(true);
          },
        },
        { type: "separator" },
        {
          label: "下载原文件",
          icon: icon(<ArrowDownToLine size={16} />),
          run: () =>
            void act(async () => {
              if (menuFile.preview_url) {
                const a = document.createElement("a");
                a.href = menuFile.preview_url;
                a.download = menuFile.path.split("/").pop() || "file";
                a.click();
              } else await download(menuFile, menuFile.version_id);
            }),
        },
        {
          label: "复制固定版本引用",
          icon: icon(<Copy size={16} />),
          run: () =>
            void act(async () => {
              if (menuFile.ref)
                await navigator.clipboard.writeText(menuFile.ref);
              else await copyReference(menuFile);
              setNotice("已复制固定版本引用");
            }),
        },
        {
          label: "复制",
          icon: icon(<Copy size={16} />),
          shortcut: shortcutLabel("C"),
          disabled: !!copyDisabledReason(readOnly, canCopy, chosenFolders().length > 0, canCopyFolders) || busy,
          run: () => rememberClipboard("copy"),
        },
        {
          label: "剪切",
          icon: icon(<Scissors size={16} />),
          shortcut: shortcutLabel("X"),
          disabled: !!cutDisabledReason(readOnly) || busy,
          run: () => rememberClipboard("cut"),
        },
        {
          label: "创建副本",
          icon: icon(<CopyPlus size={16} />),
          shortcut: shortcutLabel("D"),
          disabled: !!copyDisabledReason(readOnly, canCopy, chosenFolders().length > 0, canCopyFolders) || busy,
          run: () => beginDuplicate(),
        },
        ...(onedriveConnections.length
          ? [
              {
                label: "复制到 OneDrive",
                icon: icon(<Cloud size={16} />),
                disabled: busy || readOnly,
                run: () => beginExport(),
              } satisfies MenuItem,
            ]
          : []),
        { type: "separator" },
        {
          label: "重命名",
          icon: icon(<Pencil size={16} />),
          shortcut: "F2",
          disabled: readOnly || busy || selected.length > 1,
          run: () => {
            setSelection([menuFile.id]);
            setValue(menuFile.path.split("/").pop() || "");
            setError("");
            setAction("rename");
          },
        },
        {
          label: "移动",
          icon: icon(<FolderInput size={16} />),
          disabled: readOnly || busy,
          run: () => {
            setValue(prefix);
            setError("");
            setAction("move");
          },
        },
        {
          label: "分享",
          icon: icon(<Share2 size={16} />),
          disabled: readOnly || managementRestricted,
          run: () => onShare(selected.length ? selected : [menuFile], prefix),
        },
        { type: "separator" },
        {
          label: "移入回收站",
          icon: icon(<Trash2 size={16} />),
          shortcut: "Delete",
          disabled: readOnly || managementRestricted || busy,
          danger: true,
          run: () => {
            setError("");
            setAction("trash");
          },
        },
      ]
    : [
        ...(menu?.entry
          ? [
              {
                label: "打开文件夹",
                icon: icon(<FolderOpen size={16} />),
                run: () => navigate(menuPath),
              } satisfies MenuItem,
              {
                label: "复制",
                icon: icon(<Copy size={16} />),
                shortcut: shortcutLabel("C"),
                disabled:
                  !!copyDisabledReason(readOnly, canCopy, true, canCopyFolders) ||
                  busy,
                run: () => rememberClipboard("copy"),
              } satisfies MenuItem,
              {
                label: "剪切",
                icon: icon(<Scissors size={16} />),
                shortcut: shortcutLabel("X"),
                disabled: !!cutDisabledReason(readOnly) || busy,
                run: () => rememberClipboard("cut"),
              } satisfies MenuItem,
              {
                label: "创建副本",
                icon: icon(<CopyPlus size={16} />),
                shortcut: shortcutLabel("D"),
                disabled:
                  !!copyDisabledReason(readOnly, canCopy, true, canCopyFolders) ||
                  busy,
                run: () => beginDuplicate(),
              } satisfies MenuItem,
              ...(onedriveConnections.length
                ? [
                    {
                      label: "复制到 OneDrive",
                      icon: icon(<Cloud size={16} />),
                      disabled: busy || readOnly,
                      run: () => beginExport(),
                    } satisfies MenuItem,
                  ]
                : []),
            ]
          : []),
        {
          label: "粘贴",
          icon: icon(<Copy size={16} />),
          shortcut: shortcutLabel("V"),
          disabled:
            !!pasteDisabledReason(clipboard, space, readOnly, canCopy, canCopyFolders, canMoveFolders) || busy,
          run: () => beginPaste(menuPath),
        },
        { type: "separator" },
        {
          label: "新建文件夹",
          icon: icon(<FolderPlus size={16} />),
          disabled: readOnly || busy,
          run: () => beginCreate("folder", menuPath),
        },
        {
          label: "新建文本文件",
          icon: icon(<FilePlus2 size={16} />),
          disabled: readOnly || busy || !!(adapter && !adapter.createFile),
          run: () => beginCreate("file", menuPath),
        },
        ...(!menu?.entry
          ? [
              {
                label: "上传文件",
                icon: icon(<Upload size={16} />),
                disabled: readOnly,
                run: () => upload(),
              } satisfies MenuItem,
            ]
          : []),
        {
          label: "分享目录",
          icon: icon(<Share2 size={16} />),
          disabled: readOnly || managementRestricted,
          run: () => onShare([], menuPath),
        },
        { type: "separator" },
        {
          label: "复制目录路径",
          icon: icon(<Copy size={16} />),
          run: () =>
            void act(async () => {
              await navigator.clipboard.writeText(menuPath || "/");
              setNotice("已复制目录路径");
            }),
        },
        {
          label: "目录信息",
          icon: icon(<Info size={16} />),
          run: () => setFolderInfo(menuPath),
        },
        {
          label: "刷新",
          icon: icon(<RotateCw size={16} />),
          run: () => void reload(),
        },
        ...(menu?.entry
          ? [
              { type: "separator" } as const,
              {
                label: "移入回收站",
                icon: icon(<Trash2 size={16} />),
                shortcut: "Delete",
                disabled: readOnly || managementRestricted || busy,
                danger: true,
                run: () => {
                  setError("");
                  setAction("trash");
                },
              } satisfies MenuItem,
            ]
          : []),
      ];
  return (
    <div
      className={`finder ${inspected ? "has-inspector" : ""} ${mobile || treeCollapsed ? "tree-collapsed" : ""}`}
    >
      <FinderDirectoryPanel mobile={mobile} open={treeOpen} onOpenChange={setMobileTreeOpen} returnFocus={treeToggle} id={treeID}>
          {showSpaceSwitcher && (
            <>
              <label className="finder-label">位置</label>
              <SelectAdapter
                aria-label="选择空间"
                value={space}
                onChange={(e) => {
                  setMobileTreeOpen(false);
                  setSpace(e.target.value);
                  setPrefix("");
                }}
              >
                {spaces.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.name}
                  </option>
                ))}
              </SelectAdapter>
            </>
          )}
          <small>{backendLabel}</small>
          <label className="finder-label">收藏</label>
          {favorites.length ? (
            favorites.map((p) => (
              <Button variant="ghost" type="submit"
                className="favorite-row"
                key={p}
                onClick={() => navigate(p)}
              >
                <Star size={13} />
                {p || "全部文件"}
              </Button>
            ))
          ) : (
            <small>收藏常用目录，随时返回</small>
          )}
          <label className="finder-label">文件夹</label>
          <ul>
            {space && (
              <TreeNode
                key={space}
                space={space}
                path=""
                selected={prefix}
                navigate={navigate}
                revision={revision}
                adapter={adapter}
                onContext={(e, path) =>
                  contextMenu(e, { kind: "directory", path })
                }
                onDropFiles={readOnly || busy ? undefined : (e, path) => dropMovedFiles(e, path)}
              />
            )}
          </ul>
      </FinderDirectoryPanel>
      <section
        className={`finder-body ${dragging ? "dragging" : ""}`}
        onDragOver={e => { if (internalFileDrag(e)) acceptFileMove(e); else if (e.dataTransfer.types.includes("Files")) acceptOsFiles(e); }}
        onDragLeave={e => { if (!e.currentTarget.contains(e.relatedTarget as Node | null)) setDragging(false); }}
        onDrop={e => { if (internalFileDrag(e)) dropMovedFiles(e, prefix); else if (e.dataTransfer.files.length) dropOsFiles(e); }}
        aria-label="文件浏览"
        tabIndex={0}
        onContextMenu={(e) => {
          if (
            !(e.target as HTMLElement).closest("input,textarea,select,button,a")
          )
            contextMenu(e);
        }}
        onKeyDown={(e) => {
          if (e.defaultPrevented) return;
          const target = e.target as HTMLElement;
          // Toolbar and popup controls own their keyboard behavior. File-row
          // names retain list navigation; independent actions retain activation.
          if (target !== e.currentTarget && !target.closest(".finder-scroll")) return;
          if (target.closest("button,a,[role=checkbox],[role=combobox]") && !target.closest(".finder-name,.finder-column-row,.finder-gallery-strip")) return;
          if (
            (e.key === "ContextMenu" || (e.shiftKey && e.key === "F10")) &&
            !/INPUT|TEXTAREA|SELECT/.test((e.target as HTMLElement).tagName)
          ) {
            e.preventDefault();
            const rect = (e.target as HTMLElement).getBoundingClientRect();
            const row = (e.target as HTMLElement).closest<HTMLElement>(
              "[data-file-id]",
            );
            const file = row
              ? files.find((f) => f.id === row.dataset.fileId)
              : undefined;
            const folder = (e.target as HTMLElement).closest<HTMLElement>(
              "[data-directory-path]",
            );
            if (file) setSelection([file.id]);
            setMenu({
              x: rect.left + 12,
              y: rect.top + 12,
              entry: file
                ? { kind: "file", path: file.path, file }
                : folder
                  ? { kind: "directory", path: folder.dataset.directoryPath! }
                  : undefined,
            });
            return;
          }
          if (action || menu || helpOpen) return;
          if ((viewMode === "gallery") && (e.key === "ArrowLeft" || e.key === "ArrowRight") && !isTypingTarget(e.target) && !isMediaTarget(e.target) && !(e.target as HTMLElement).closest("[data-slot=dialog-content],[data-slot=sheet-content],.media-preview,[role=combobox]")) {
            e.preventDefault();
            const index = files.findIndex(f=>selection.includes(f.id));
            const next = files[Math.max(0,Math.min(files.length-1,index+(e.key==="ArrowRight" ? 1 : -1)))];
            if(next){select(next,false);setFocusIndex(listed.findIndex(row=>row.file?.id===next.id));}
            return;
          }
          if (isTypeaheadKey(e.nativeEvent)) {
            e.preventDefault();
            const now = Date.now();
            if (now - typeAt.current > 700) typeBuffer.current = "";
            typeAt.current = now;
            typeBuffer.current += e.key;
            const names = listed.map(
              (row) => row.path.split("/").pop() || row.path,
            );
            const next = typeaheadIndex(names, typeBuffer.current, focusIndex);
            if (next < 0) return;
            setFocusIndex(next);
            setAnchorIndex(next);
            const row = listed[next];
            if (row?.file) {
              setSelection([row.file.id]);
              setFolderSelection([]);
            } else if (row?.kind === "directory") {
              setFolderSelection([row.path]);
              setSelection([]);
            }
            return;
          }
          const command = finderCommand(e.nativeEvent);
          if (!command) return;
          const nextIndex = (delta: number) =>
            Math.max(0, Math.min(listed.length - 1, focusIndex + delta));
          if (command === "copy") {
            e.preventDefault();
            if (!copyDisabledReason(readOnly, canCopy, chosenFolders().length > 0, canCopyFolders)) rememberClipboard("copy");
          } else if (command === "cut") {
            e.preventDefault();
            if (!cutDisabledReason(readOnly)) rememberClipboard("cut");
          } else if (command === "paste") {
            e.preventDefault();
            beginPaste(prefix);
          } else if (command === "undo") {
            if (!lastMove) return;
            e.preventDefault();
            void act(() => undoMove());
          } else if (command === "duplicate") {
            e.preventDefault();
            beginDuplicate();
          } else if (command === "selectAll") {
            e.preventDefault();
            setSelection(files.map((f) => f.id));
            setFolderSelection(folders.map((row) => row.path));
          } else if (command === "trash") {
            if (
              (!chosenFiles().length && !chosenFolders().length) ||
              readOnly ||
              managementRestricted
            )
              return;
            e.preventDefault();
            setError("");
            setAction("trash");
          } else if (command === "rename") {
            const file = chosenFiles()[0];
            if (!file || readOnly) return;
            e.preventDefault();
            setSelection([file.id]);
            setValue(file.path.split("/").pop() || "");
            setAction("rename");
          } else if (command === "open" || command === "preview") {
            if (focused?.kind === "directory") {
              e.preventDefault();
              navigate(focused.path);
            } else if (focused?.file) {
              e.preventDefault();
              onPreview(focused.file, files);
            }
          } else if (command === "escape") {
            e.preventDefault();
            if (clipboard) setClipboard(undefined);
            else {
              setSelection([]);
              setFolderSelection([]);
            }
          } else if (command === "help") {
            e.preventDefault();
            setHelpOpen(true);
          } else if (command === "goParent") {
            e.preventDefault();
            if (!prefix) return;
            navigate(prefix.split("/").slice(0, -1).join("/"));
          } else if (command === "focusNext" || command === "focusPrev") {
            e.preventDefault();
            const next = nextIndex(command === "focusNext" ? 1 : -1);
            setFocusIndex(next);
            setAnchorIndex(next);
            const row = listed[next];
            if (row?.file) {
              setSelection([row.file.id]);
              setFolderSelection([]);
            } else if (row?.kind === "directory") {
              setFolderSelection([row.path]);
              setSelection([]);
            }
          } else if (command === "extendNext" || command === "extendPrev") {
            e.preventDefault();
            selectRange(nextIndex(command === "extendNext" ? 1 : -1));
          }
        }}
      >
        <div className="finder-toolbar max-md:grid-cols-[auto_minmax(0,1fr)_auto_auto] max-md:[&_button]:min-h-11 max-md:[&_button]:min-w-11">
          <Button variant="ghost" size="icon" type="button" ref={treeToggle}
            className="icon max-md:size-11"
            aria-label={treeOpen ? "折叠目录树" : "展开目录树"}
            aria-expanded={treeOpen}
            aria-controls={treeID}
            onClick={() => mobile ? setMobileTreeOpen(!mobileTreeOpen) : setTreeCollapsed(!treeCollapsed)}
          >
            <PanelLeft size={17} />
          </Button>
          <div className="history-buttons max-md:hidden">
            <Button variant="ghost" size="icon" type="button"
              className="icon"
              aria-label="返回上一目录"
              disabled={position === 0}
              onClick={() => {
                setPosition(position - 1);
                setPrefix(navigation[position - 1]);
              }}
            >
              <ArrowLeft size={17} />
            </Button>
            <Button variant="ghost" size="icon" type="button"
              className="icon"
              aria-label="前进下一目录"
              disabled={position >= navigation.length - 1}
              onClick={() => {
                setPosition(position + 1);
                setPrefix(navigation[position + 1]);
              }}
            >
              <ArrowRight size={17} />
            </Button>
          </div>
        <nav className="finder-breadcrumb" aria-label="目录路径">
          <Button variant="ghost" type="button" onClick={() => navigate("")}>全部文件</Button>
          {prefix
            .split("/")
            .filter(Boolean)
            .map((name, i) => (
              <React.Fragment key={i}>
                <ChevronRight size={12} />
                <Button variant="ghost" type="button"
                  onClick={() =>
                    navigate(
                      prefix
                        .split("/")
                        .slice(0, i + 1)
                        .join("/"),
                    )
                  }
                >
                  {name}
                </Button>
              </React.Fragment>
            ))}
          <Button variant="ghost" size="icon" type="button"
            className="icon"
            aria-label="收藏当前目录"
            aria-pressed={favorites.includes(prefix)}
            onClick={toggleFavorite}
          >
            <Star
              size={14}
              fill={favorites.includes(prefix) ? "currentColor" : "none"}
            />
          </Button>
        </nav>
          {mobile ? <DropdownMenu>
            <DropdownMenuTrigger render={<Button type="button" variant="outline" size="icon" className="size-11" aria-label="文件视图" />}>
              {viewMode === "grid" ? <LayoutGrid /> : viewMode === "columns" ? <Columns3 /> : viewMode === "gallery" ? <GalleryHorizontal /> : <List />}
            </DropdownMenuTrigger>
            <DropdownMenuContent align="end">
              {([['list', '列表视图', List], ['grid', '网格视图', LayoutGrid], ['columns', '分栏视图', Columns3], ['gallery', '画廊视图', GalleryHorizontal]] as const).map(([mode, label, Icon]) =>
                <DropdownMenuItem key={mode} aria-label={label} className="min-h-11" onClick={() => chooseMode(mode)}><Icon />{label}{viewMode === mode && <span className="ml-auto text-xs">当前</span>}</DropdownMenuItem>)}
            </DropdownMenuContent>
          </DropdownMenu> : <div className="view-switch" role="group" aria-label="文件视图">
            <Button variant="ghost" size="icon" type="button"
              aria-label="列表视图"
              aria-pressed={viewMode === "list"}
              onClick={() => chooseMode("list")}
            >
              <List size={17} />
            </Button>
            <Button variant="ghost" size="icon" type="button"
              aria-label="网格视图"
              aria-pressed={viewMode === "grid"}
              onClick={() => chooseMode("grid")}
            >
              <LayoutGrid size={17} />
            </Button>
            <Button variant="ghost" size="icon" type="button" aria-label="分栏视图" aria-pressed={viewMode === "columns"} onClick={() => chooseMode("columns")}><Columns3 size={17} /></Button>
            <Button variant="ghost" size="icon" type="button" aria-label="画廊视图" aria-pressed={viewMode === "gallery"} onClick={() => chooseMode("gallery")}><GalleryHorizontal size={17} /></Button>
          </div>}
          {focusAction && <div className="finder-focus border-l border-border pl-2 max-md:[&>a]:size-11">{focusAction}</div>}
        </div>
        <div className="finder-search px-3 pb-2 max-md:[&_[data-slot=input-group]]:min-h-11">          <SearchField
              aria-label="搜索文件"
              placeholder="搜索当前目录及子目录"
              value={query}
              onChange={(e) => setQuery(e.target.value)}
            />
</div>
        <div className="finder-actions max-md:[&_button]:min-h-11 max-md:[&_button]:min-w-11">
          <Button variant="default" type="button" className="finder-upload" aria-label="上传文件" disabled={readOnly} onClick={() => upload()}>
            <Upload size={17} /><span className="max-md:hidden">上传文件</span><span className="md:hidden">上传</span>
          </Button>
          <DropdownMenu>
            <DropdownMenuTrigger render={<Button variant="outline" type="button" disabled={readOnly} aria-label="新建"><Plus />新建<ChevronDown className="max-md:hidden" /></Button>} />
            <DropdownMenuContent align="start">
              <DropdownMenuItem disabled={readOnly} onClick={() => beginCreate("folder")}><FolderPlus />新建文件夹</DropdownMenuItem>
              <DropdownMenuItem disabled={readOnly || !!(adapter && !adapter.createFile)} onClick={() => beginCreate("file")}><FilePlus2 />新建文件</DropdownMenuItem>
            </DropdownMenuContent>
          </DropdownMenu>
          <Button variant="outline" size="sm" type="button"
            className="finder-secondary"
            disabled={readOnly || managementRestricted}
            onClick={() => onShare(selected, prefix)}
          >
            <Share2 size={15} />
            分享{selected.length ? `（${selected.length}）` : "目录"}
          </Button>
          <div className="finder-inline-filters ml-auto flex min-w-0 gap-2 [&_label>span]:hidden">{filterControls}</div>
          <div className="finder-compact-filters ml-auto max-md:[&_button]:w-11 max-md:[&_button]:p-0 max-md:[&_button]:text-[0px] max-md:[&_button]:gap-0">
            <Popover><PopoverTrigger render={<Button variant="outline" type="button"><SlidersHorizontal />筛选与排序</Button>} />
              <PopoverContent align="end" className="grid w-64 gap-4">{filterControls}</PopoverContent>
            </Popover>
          </div>
          <Button variant="outline" size="sm" type="button"
            className="finder-more"
            aria-label="更多操作"
            onClick={(e) => contextMenu(e, entries.find(entry => entry.file ? entry.file.id === selected[0]?.id : folderSelection.length === 1 && entry.path === folderSelection[0]))}
          >
            <MoreHorizontal size={15} />
            更多
          </Button>
        </div>
        {error && (
          <p className="banner error m-0 mx-3 mb-2 flex-wrap px-3 py-1.5" role="alert">
            {error}
            <Button variant="ghost" type="submit" onClick={() => void reload()}>重新读取</Button>
          </p>
        )}
        {notice && (
          <p className="banner m-0 mx-3 mb-2 flex-wrap px-0 py-1.5" role="status">
            {notice}
            {lastMove && notice.startsWith("已移动") && (
              <Button variant="ghost" type="submit" className="text-button" onClick={() => void act(() => undoMove())}>
                撤销
              </Button>
            )}
          </p>
        )}
        {clipboard && (
          <div className="clipboard-bar" role="status">
            <span>{clipboardNotice(clipboard)}</span>
            <Button variant="ghost" type="submit"
              disabled={!!pasteDisabledReason(clipboard, space, readOnly, canCopy, canCopyFolders, canMoveFolders) || busy}
              onClick={() => beginPaste(prefix)}
            >
              粘贴到当前目录
            </Button>
            <Button variant="ghost" type="submit"
              disabled={!!pasteDisabledReason(clipboard, space, readOnly, canCopy, canCopyFolders, canMoveFolders) || busy}
              onClick={() => {
                setValue(prefix);
                setAction("paste");
              }}
            >
              粘贴到…
            </Button>
            <Button variant="ghost" type="submit" onClick={() => setClipboard(undefined)}>清除</Button>
          </div>
        )}
        <div className="finder-scroll" ref={listRef}>
        {busy && !entries.length && (
          <div className="finder-skeleton" aria-hidden="true">
            <i /><i /><i /><i />
          </div>
        )}
        <div className={`finder-content-view view-${viewMode}`} aria-busy={busy}>
        {viewMode === "columns" && (
          <div className="finder-columns" aria-label="分栏浏览">
            {prefix.split("/").filter(Boolean).map((part, index, parts) => {
              const path = parts.slice(0, index + 1).join("/");
              return <div className="finder-column" key={path}><Button layout="surface" type="button" className="finder-column-title" title={path} onClick={() => navigate(path)}>{part}</Button><span>目录路径</span></div>;
            })}
            <div className="finder-column finder-column-current" aria-label="当前文件列">
              {listed.map((row,index) => <Button layout="surface" type="button" key={row.path} title={row.path}
                className={`finder-column-row ${(row.file ? selection.includes(row.file.id) : folderSelection.includes(row.path)) ? "selected" : ""} ${focusIndex===index ? "focused" : ""}`}
                aria-label={`选择 ${row.path.split("/").pop()}`} aria-pressed={row.file ? selection.includes(row.file.id) : folderSelection.includes(row.path)}
                data-file-id={row.file?.id} data-directory-path={row.kind==="directory" ? row.path : undefined}
                onClick={() => {setFocusIndex(index);setAnchorIndex(index);if(row.file){select(row.file,false);setFolderSelection([]);}else{selectFolder(row.path,false);setSelection([]);}}}
                onDoubleClick={() => row.file ? onPreview(row.file,files) : navigate(row.path)}
                onKeyDown={e=>{if(e.key==="Enter"){e.preventDefault();e.stopPropagation();if(row.file) onPreview(row.file,files); else navigate(row.path);}}}
                draggable={!!row.file && !readOnly && !busy} onDragStart={event => { if (row.file) startFileDrag(event, row.file.id); }} onDragOver={event => { if (!row.file) acceptFileMove(event); }} onDrop={event => { if (!row.file) dropMovedFiles(event, row.path); }}>
                {row.file ? <MediaThumbnail file={row.file}/> : <Folder size={24}/>}
                <span className="finder-readable-name"><span>{row.file ? row.path.split("/").pop()?.replace(/\.[^.]+$/,"") : row.path.split("/").pop()}</span>{row.file && <small>{row.path.includes(".") ? "."+row.path.split(".").pop() : row.file.mime_type}</small>}</span>
              </Button>)}
              {selected[0] && <Button type="button" variant="outline" className="m-3" onClick={()=>onPreview(selected[0],files)}><Eye/>打开阅读器</Button>}
            </div>
            {selected[0] && <aside className="finder-column-preview" aria-label="所选文件预览"><h3 className="break-words text-base font-semibold">{selected[0].path.split("/").pop()}</h3><MediaPreview file={selected[0]} compact/><Button type="button" onClick={()=>onPreview(selected[0],files)}><Eye/>完整阅读</Button></aside>}
          </div>
        )}
        {viewMode === "gallery" && (
          <div className="finder-gallery">
            <header className="flex min-w-0 flex-wrap items-center gap-3 border-b border-border p-4">
              <div className="min-w-0 flex-1"><h3 className="truncate text-base font-semibold" title={selected[0]?.path}>{selected[0]?.path.split("/").pop() || "选择文件"}</h3>{selected[0] && <p className="text-xs text-muted-foreground">{selected[0].mime_type || "未知类型"} · {bytes(selected[0].size_bytes)}</p>}</div>
              <Button type="button" disabled={!selected[0]} onClick={()=>selected[0] && onPreview(selected[0],files)}><Eye/>打开阅读器</Button>
            </header>
            <div className="finder-gallery-preview">{selected[0] ? <MediaPreview file={selected[0]} /> : <><GalleryHorizontal size={44}/><p>选择文件以查看预览</p></>}</div>
            <div className="finder-gallery-strip" aria-label="画廊文件">
              {files.map(f=><Button layout="surface" type="button" key={f.id} title={f.path} aria-label={`选择 ${f.path.split("/").pop()}`} aria-pressed={selection.includes(f.id)} className={selection.includes(f.id) ? "selected" : ""}
                onClick={()=>{select(f,false);setFolderSelection([]);setFocusIndex(listed.findIndex(row=>row.file?.id===f.id));}}
                onDoubleClick={()=>onPreview(f,files)} onKeyDown={e=>{if(e.key==="Enter"){e.preventDefault();e.stopPropagation();onPreview(f,files);}}}
                draggable={!readOnly && !busy} onDragStart={event => startFileDrag(event, f.id)}>
                <MediaThumbnail file={f}/><span>{f.path.split("/").pop()}</span>
              </Button>)}
            </div>
          </div>
        )}
        {(viewMode === "list" || viewMode === "grid") && <div className={grid ? "finder-grid" : "finder-list [&_.finder-file]:min-h-9 [&_.finder-file]:rounded-none [&_.finder-file]:py-1 [&_.finder-name]:gap-2 [&_.media-thumbnail]:size-6 [&_.finder-folder]:min-h-9 [&_.finder-folder]:rounded-none [&_.finder-folder]:p-1 [&_.finder-folder_.finder-name>svg]:size-5 md:[&_.finder-row-actions_.icon]:size-7 md:[&_.finder-folder_.icon]:size-7 max-md:[&_.finder-file]:min-h-12 max-md:px-2 max-md:[&_.finder-file]:grid-cols-[24px_minmax(0,1fr)_90px] max-md:[&_.finder-folder]:min-h-12 max-md:[&_.finder-folder_.icon]:size-11 max-md:[&_.finder-file_[data-slot=checkbox]]:opacity-100 max-md:[&_.finder-name]:min-h-11 max-md:[&_.finder-name_strong]:whitespace-normal max-md:[&_.finder-name_strong]:line-clamp-2 max-md:[&_.finder-name_strong]:break-all"} aria-busy={busy}>
          {!grid && <div className="finder-list-header max-md:grid-cols-[24px_minmax(0,1fr)_90px] max-md:px-0 [&_button]:min-h-8 max-md:[&_button]:min-h-11" role="group" aria-label="文件列表表头">
            <span />{[["name", "名称"], ["date", "修改时间"], ["size", "大小"]].map(([key, label]) => <Button layout="surface" variant="ghost" type="submit" key={key} aria-label={`按${label}排序`} aria-pressed={sort === key} onClick={() => { setCursor(""); setSort(key); setSortOrder(sort === key && sortOrder === "asc" ? "desc" : "asc"); }}>{label}{sort === key ? (sortOrder === "asc" ? " ↑" : " ↓") : ""}</Button>)}<span />
          </div>}
          {!query &&
            entries
              .filter((row) => row.kind === "directory")
              .map((row) => (
                <article
                  key={row.path}
                  className={`finder-folder ${folderSelection.includes(row.path) ? "selected" : ""} ${isCutItem(clipboard, "dir:" + row.path, row.path) ? "cut" : ""} ${listed[focusIndex]?.path === row.path ? "focused" : ""}`}
                  title={row.path}
                  tabIndex={listed[focusIndex]?.path === row.path ? 0 : -1}
                  data-directory-path={row.path}
                  onContextMenu={(e) => contextMenu(e, row)}
                  onDragOver={(e) => { acceptFileMove(e); if (internalFileDrag(e) && !readOnly && !busy) e.currentTarget.classList.add("drop-target"); }}
                  onDragLeave={(e) => {
                    e.currentTarget.classList.remove("drop-target");
                  }}
                  onDrop={(e) => { e.currentTarget.classList.remove("drop-target"); dropMovedFiles(e, row.path); }}
                >
                  <Checkbox

                    aria-label={"选择文件夹 " + row.path}
                    checked={folderSelection.includes(row.path)}
                    onCheckedChange={() => {
                      const index = listed.findIndex((item) => item.path === row.path);
                      setFocusIndex(index);
                      setAnchorIndex(index);
                      selectFolder(row.path, true);
                    }}
                  />
                  <Button layout="surface" variant="ghost" type="submit"
                    className="finder-name"
                    title={row.path}
                    onClick={(e) => {
                      const index = listed.findIndex((item) => item.path === row.path);
                      if (e.shiftKey) selectRange(index);
                      else {
                        setFocusIndex(index);
                        setAnchorIndex(index);
                        if (e.ctrlKey || e.metaKey) selectFolder(row.path, true);
                        else {
                          setFolderSelection([row.path]);
                          setSelection([]);
                        }
                      }
                    }}
                    onDoubleClick={() => navigate(row.path)}
                  >
                    <Folder size={32} />
                    <strong>{row.path.split("/").pop()}</strong>
                  </Button>
                  <Button variant="ghost" size="icon" type="submit"
                    className="icon"
                    aria-label={"打开文件夹 " + row.path}
                    onClick={() => navigate(row.path)}
                  >
                    <ChevronRight size={16} />
                  </Button>
                </article>
              ))}
          {files.map((f) => (
            <article
              className={`finder-file ${selection.includes(f.id) ? "selected" : ""} ${isCutItem(clipboard, f.id) ? "cut" : ""} ${listed[focusIndex]?.file?.id === f.id ? "focused" : ""}`}
              key={f.id}
              tabIndex={listed[focusIndex]?.file?.id === f.id ? 0 : -1}
              data-file-id={f.id}
              onContextMenu={(e) =>
                contextMenu(e, { kind: "file", path: f.path, file: f })
              }
              draggable={!readOnly && !busy}
              onDragStart={(e) => startFileDrag(e, f.id)}
            >
              <Checkbox

                aria-label={"选择 " + f.path}
                checked={selection.includes(f.id)}
                onCheckedChange={() => select(f, true)}
              />
              <Button layout="surface" variant="ghost" type="submit"
                className="finder-name"
                title={f.path}
                onClick={(e) => {
                  const index = listed.findIndex((item) => item.file?.id === f.id);
                  if (e.shiftKey) selectRange(index);
                  else {
                    setFocusIndex(index);
                    setAnchorIndex(index);
                    select(f, e.ctrlKey || e.metaKey);
                  }
                }}
                onDoubleClick={() => onPreview(f, files)}
                onKeyDown={(e) => {
                  if (e.key === " " || e.key === "Enter") {
                    e.preventDefault();
                    onPreview(f, files);
                  }
                }}
              >
                <MediaThumbnail file={f} />
                <span>
                  <strong>{query ? f.path : f.path.split("/").pop()}</strong>
                  <small className={grid || f.review_pending ? undefined : "md:hidden"}>
                    {f.review_pending ? "待审核 · " : ""}
                    {f.mime_type || "文件"}
                  </small>
                </span>
              </Button>
              <time>{new Date(f.updated_at).toLocaleDateString("zh-CN")}</time>
              <span className="file-size">{bytes(f.size_bytes)}</span>
              <div className="finder-row-actions max-md:[&_button]:min-h-11 max-md:[&_button]:min-w-11">
                <Button variant="ghost" size="icon" type="submit"
                  className="icon"
                  aria-label={"详情 " + f.path}
                  onClick={() => onPreview(f, files)}
                >
                  <Eye size={17} />
                </Button>
                <Button variant="ghost" size="icon" type="submit"
                  className="icon"
                  aria-label={"更多操作 " + f.path}
                  onClick={(e) =>
                    contextMenu(e, { kind: "file", path: f.path, file: f })
                  }
                >
                  <MoreHorizontal size={17} />
                </Button>
              </div>
            </article>
          ))}
        </div>}
        </div>
        {!busy && !entries.length && !error && (
          <div
            className={`empty ${dragging ? "drop-target" : ""}`}
            onDragOver={acceptOsFiles}
            onDragLeave={(e) => {
              if (!e.currentTarget.contains(e.relatedTarget as Node | null))
                setDragging(false);
            }}
            onDrop={dropOsFiles}
          >
            <Folder size={36} />
            <h3>{query ? "没有匹配的文件" : "把文件放到这里"}</h3>
            <p>
              {query
                ? "试试其他关键词。"
                : readOnly
                  ? "可预览与下载，当前权限不允许修改。"
                  : dragging
                    ? "松开即可上传到这个目录。"
                    : "上传图片、视频或文档，开始整理这个目录。"}
            </p>
          </div>
        )}
        {(cursor || nextCursor) && (
          <div className="finder-pagination">
            <Button variant="outline" size="sm" type="submit"

              disabled={!cursor || busy}
              onClick={() =>
                setCursor(
                  Number(cursor) > 200 ? String(Number(cursor) - 200) : "",
                )
              }
            >
              上一页
            </Button>
            <span>
              第 {Math.floor(Number(cursor || 0) / 200) + 1} 页 · 共 {total} 项
            </span>
            <Button variant="outline" size="sm" type="submit"

              disabled={!nextCursor || busy}
              onClick={() => setCursor(nextCursor)}
            >
              下一页
            </Button>
          </div>
        )}
        </div>
        {selection.length + folderSelection.length > 0 && (
          <div className="selection-bar">
            <span>已选择 {selection.length + folderSelection.length} 项</span>
            {!adapter && (
              <Button variant="ghost" type="submit"
                onClick={() => {
                  const a = document.createElement("a");
                  a.href = contextURL(
                    "/api/v1/files-export?files=" +
                      encodeURIComponent(selection.join(",")),
                  );
                  a.download = "drivebridge-export.zip";
                  a.click();
                }}
              >
                批量下载 ZIP
              </Button>
            )}
            <Button variant="ghost" type="submit"
              disabled={!!copyDisabledReason(readOnly, canCopy, chosenFolders().length > 0, canCopyFolders) || busy}
              onClick={() => rememberClipboard("copy")}
            >
              复制
            </Button>
            <Button variant="ghost" type="submit"
              disabled={!!cutDisabledReason(readOnly) || busy}
              onClick={() => rememberClipboard("cut")}
            >
              剪切
            </Button>
            <Button variant="ghost" type="submit"
              disabled={!!copyDisabledReason(readOnly, canCopy, chosenFolders().length > 0, canCopyFolders) || busy}
              onClick={() => beginDuplicate()}
            >
              创建副本
            </Button>
            {onedriveConnections.length > 0 && (
              <Button variant="ghost" type="submit" disabled={busy || readOnly} onClick={() => beginExport()}>
                复制到 OneDrive
              </Button>
            )}
            <Button variant="ghost" type="submit"
              disabled={readOnly}
              onClick={() => {
                setAction("move");
                setValue(prefix);
              }}
            >
              移动到…
            </Button>
            <Button variant="ghost" type="submit"
              disabled={readOnly || managementRestricted}
              onClick={() => setAction("trash")}
            >
              移入回收站
            </Button>
            <Button variant="ghost" type="submit"
              onClick={() => {
                setSelection([]);
                setFolderSelection([]);
              }}
            >
              取消选择
            </Button>
          </div>
        )}
        <footer className="finder-status">
          {readOnly && <span>当前网盘仅供浏览 · 移动文件需要编辑权限，请在「管理与授权」中申请</span>}
          <span>
            {busy
              ? "正在读取…"
              : `${files.length} 个文件 · ${entries.filter((e) => e.kind === "directory").length} 个文件夹`}
          </span>
          <span>
            {bytes(files.reduce((sum, f) => sum + f.size_bytes, 0))} · 当前页 ·
            右键操作
          </span>
        </footer>
      </section>
      {inspected && (
        <aside className="finder-inspector" aria-label="文件检查器">
          <header>
            <strong>文件信息</strong>
            <Button variant="ghost" size="icon" type="submit"
              className="icon"
              aria-label="关闭检查器"
              onClick={() => setInspectorOpen(false)}
            >
              <X size={17} />
            </Button>
          </header>
          <MediaPreview key={inspected.version_id} file={inspected} compact />
          <h3>{inspected.path.split("/").pop()}</h3>
          <dl>
            <dt>位置</dt>
            <dd>{inspected.path}</dd>
            <dt>大小</dt>
            <dd>{bytes(inspected.size_bytes)}</dd>
            <dt>类型</dt>
            <dd>{inspected.mime_type}</dd>
            <dt>更新时间</dt>
            <dd>{new Date(inspected.updated_at).toLocaleString("zh-CN")}</dd>
            <dt>版本标识</dt>
            <dd className="metadata-id">{inspected.version_id}</dd>
            <dt>文件标识</dt>
            <dd className="metadata-id">{inspected.id}</dd>
            <dt>所在网盘</dt>
            <dd>{spaces.find((s) => s.id === space)?.name || space}</dd>
            <dt>来源</dt>
            <dd>{inspected.caller || "用户上传"}</dd>
          </dl>
          <Button variant="outline" type="submit"

            onClick={() => onPreview(inspected, files)}
          >
            打开预览与版本
          </Button>
          <Button variant="outline" type="submit"

            onClick={() =>
              void act(async () => {
                if (inspected.ref)
                  await navigator.clipboard.writeText(inspected.ref);
                else await copyReference(inspected);
                setNotice("已复制固定版本引用");
              })
            }
          >
            <Copy size={15} />
            复制引用
          </Button>
          <Button variant="outline" type="submit"

            disabled={readOnly || managementRestricted}
            onClick={() => onShare([inspected], prefix)}
          >
            <Share2 size={15} />
            分享
          </Button>
          <Button variant="outline" type="submit"

            onClick={() =>
              void act(async () => {
                if (inspected.preview_url) {
                  const a = document.createElement("a");
                  a.href = inspected.preview_url;
                  a.download = inspected.path.split("/").pop() || "file";
                  a.click();
                } else await download(inspected, inspected.version_id);
              })
            }
          >
            <ArrowDownToLine size={15} />
            下载原文件
          </Button>
          <Button variant="outline" type="submit"

            disabled={!!copyDisabledReason(readOnly, canCopy, chosenFolders().length > 0, canCopyFolders)}
            onClick={() => rememberClipboard("copy")}
          >
            复制
          </Button>
          <Button variant="outline" type="submit"

            disabled={readOnly}
            onClick={() => {
              setAction("rename");
              setValue(inspected.path.split("/").pop() || "");
            }}
          >
            重命名
          </Button>
          <Button variant="destructive" type="submit"

            disabled={readOnly || managementRestricted}
            onClick={() => setAction("trash")}
          >
            <Trash2 size={15} />
            移入回收站
          </Button>
        </aside>
      )}
      {helpOpen && (
        <WorkspaceDialog title="键盘快捷键" close={() => setHelpOpen(false)}>
          <dl className="shortcut-list">
            <dt>{shortcutLabel("C")} / {shortcutLabel("X")} / {shortcutLabel("V")}</dt>
            <dd>复制、剪切、粘贴</dd>
            <dt>{shortcutLabel("D")}</dt>
            <dd>在当前目录创建副本</dd>
            <dt>{shortcutLabel("A")}</dt>
            <dd>全选当前页文件</dd>
            <dt>Delete</dt>
            <dd>移入回收站（需确认）</dd>
            <dt>F2</dt>
            <dd>重命名</dd>
            <dt>↑ ↓</dt>
            <dd>移动焦点；按住 Shift 扩展选择</dd>
            <dt>→ / ←</dt>
            <dd>进入文件夹 / 返回上级</dd>
            <dt>字母</dt>
            <dd>按名称定位当前目录中的项目</dd>
            <dt>{shortcutLabel("Z")}</dt>
            <dd>撤销上一次移动或剪切粘贴</dd>
            <dt>Esc</dt>
            <dd>关闭菜单、清除剪贴板或取消选择</dd>
          </dl>
        </WorkspaceDialog>
      )}
      {menu && (
        <FinderMenu
          x={menu.x}
          y={menu.y}
          items={menuItems}
          close={() => setMenu(undefined)}
        />
      )}
      {folderInfo !== undefined && (
        <WorkspaceDialog
          title="目录信息"
          close={() => setFolderInfo(undefined)}
        >
          <dl className="metadata-list">
            <dt>目录</dt>
            <dd>{folderInfo || "根目录"}</dd>
            <dt>网盘</dt>
            <dd>{spaces.find((s) => s.id === space)?.name || space}</dd>
            <dt>存储</dt>
            <dd>{backendLabel}</dd>
            <dt>当前访问</dt>
            <dd>{readOnly ? "只读" : "可编辑"}</dd>
            {folderEntries && (
              <>
                <dt>直属文件</dt>
                <dd>{folderEntries.filter((e) => e.file).length}</dd>
                <dt>直属子目录</dt>
                <dd>
                  {folderEntries.filter((e) => e.kind === "directory").length}
                </dd>
                <dt>文件大小</dt>
                <dd>
                  {bytes(
                    folderEntries.reduce(
                      (sum, e) => sum + (e.file?.size_bytes || 0),
                      0,
                    ),
                  )}
                </dd>
              </>
            )}
          </dl>
          {!folderEntries && !folderError && (
            <p role="status">正在读取目录信息…</p>
          )}
          {folderError && (
            <p className="error" role="alert">
              {folderError}
            </p>
          )}
          <p>
            数量与大小来自本次目录页，不包含子目录内文件；目录内容较多时请翻页查看。
          </p>
        </WorkspaceDialog>
      )}
      {action && (
        <WorkspaceDialog
          className={creating ? "sm:max-w-[480px] [&_.dialog-actions]:mt-4 [&_.dialog-actions]:pt-4" : undefined}
          title={
            {
              folder: "新建文件夹",
              file: "新建文本文件",
              rename: "重命名",
              move: "移动文件",
              trash: "移入回收站",
              paste: "粘贴文件",
              export: "复制到 OneDrive",
            }[action] || "文件操作"
          }
          close={() => setAction("")}
        >
          <form
            onSubmit={(e) => {
              e.preventDefault();
              if (busy || nameError) return;
              const currentScope = scope.current;
              const controller = new AbortController();
              operationAbort.current = controller;
              void act(async () => {
                if (readOnly) throw new Error("approval_required");
                const target = [creationDir, value.trim()]
                  .filter(Boolean)
                  .join("/");
                if (action === "folder") {
                  if (adapter) await adapter.mkdir(space, target);
                  else
                    await api(
                      "/directories",
                      "POST",
                      {
                        space_id: space,
                        path: target,
                      },
                      {},
                      controller.signal,
                    );
                }
                if (action === "file") {
                  const file = new File([textContent], value.trim(), {
                    type:
                      fileType === "json"
                        ? "application/json"
                        : fileType === "md"
                          ? "text/markdown"
                          : "text/plain",
                  });
                  if (adapter) {
                    if (!adapter.createFile) throw new Error("unsupported");
                    await adapter.createFile(
                      space,
                      target,
                      file,
                      controller.signal,
                    );
                  } else
                    await saveFile(
                      file,
                      space,
                      target,
                      (op) => onOperation?.(op),
                      controller.signal,
                    );
                }
                if (action === "rename" && selected[0]) {
                  const target = [
                    ...selected[0].path.split("/").slice(0, -1),
                    value,
                  ].join("/");
                  if (adapter) await adapter.move(selected[0], target);
                  else
                    await api(`/files/${selected[0].id}/move`, "POST", {
                      path: target,
                      expected_version: selected[0].version_id,
                    });
                }
                if (action === "move") await moveSelection(value);
                if (action === "paste") await runPaste(value);
                if (action === "export") await runExport(exportConnection);
                if (action === "trash") {
                  for (const folder of chosenFolders()) {
                    if (adapter?.trashDirectory)
                      await adapter.trashDirectory(space, folder);
                    else
                      await api("/directories/trash", "POST", {
                        space_id: space,
                        path: folder,
                      });
                  }
                  for (const f of chosenFiles()) {
                    if (adapter) await adapter.trash(f);
                    else await api(`/files/${f.id}/trash`, "POST", {});
                  }
                }
                if (
                  scope.current.space !== currentScope.space ||
                  scope.current.prefix !== currentScope.prefix
                )
                  return;
                setAction("");
                await reload();
                if (creating) {
                  setNotice(adapter && action === "file" ? `已提交本机后台保存：${target}，完成后显示在目录中。` : `已创建：${target}`);
                  if (action === "folder" && openCreated) navigate(target);
                  else if (creationDir !== prefix) navigate(creationDir);
                }
              });
            }}
          >
            {action === "export" ? (
              <>
                <p>
                  将{" "}
                  {listedNames([
                    ...chosenFolders().map((path) => ({ path })),
                    ...chosenFiles(),
                  ])}{" "}
                  复制到 OneDrive。源网盘文件不变；进度在传输中心。
                </p>
                <label>
                  OneDrive 连接
                  <SelectAdapter
                    aria-label="OneDrive 连接"
                    value={exportConnection}
                    onChange={(e) => setExportConnection(e.target.value)}
                  >
                    {onedriveConnections.map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.name}
                      </option>
                    ))}
                  </SelectAdapter>
                </label>
              </>
            ) : action === "trash" ? (
              <p>
                将{" "}
                {listedNames([
                  ...chosenFolders().map((path) => ({ path })),
                  ...chosenFiles(),
                ])}{" "}
                移入回收站？可恢复；分享中的文件会暂时不可访问。文件夹内文件一并进入回收站。
              </p>
            ) : action === "paste" || action === "move" ? (
              <>
                <p>
                  {action === "paste"
                    ? `将 ${clipboard?.items.length || 0} 项${clipboard?.mode === "cut" ? "移动" : "复制"}到「${value || "根目录"}」。同名会自动改名，不会覆盖。`
                    : `将 ${chosenFiles().length + chosenFolders().length} 项移动到「${value || "根目录"}」。`}
                </p>
                <FolderPicker
                  space={space}
                  adapter={adapter}
                  value={value}
                  onChange={setValue}
                  exclude={
                    action === "move"
                      ? chosenFolders()
                      : clipboard?.items
                          .filter((item) => item.kind === "directory")
                          .map((item) => item.path)
                  }
                />
              </>
            ) : (
              <label>
                名称
                <Input
                  autoFocus
                  aria-label="名称"
                  required
                  value={value}
                  onFocus={(e) => e.target.select()}
                  aria-invalid={!!nameError}
                  onChange={(e) => setValue(e.target.value)}
                />
              </label>
            )}
            {creating && (
              <div className="creation-destination flex min-w-0 items-start gap-2 text-sm">
                <Folder size={16} className="mt-0.5 shrink-0" aria-hidden="true" />
                <div className="min-w-0">
                  <span className="text-xs text-muted-foreground">保存位置</span>
                  <p className="mt-1 break-words">
                    {spaces.find((s) => s.id === space)?.name || "当前网盘"} /{" "}
                    {[creationDir, value.trim()].filter(Boolean).join("/") || "根目录"}
                  </p>
                </div>
              </div>
            )}
            {action === "folder" && (
              <fieldset className="grid min-w-0 gap-4 border-0 p-0">
                <legend className="sr-only">创建选项</legend>
                <label className="flex cursor-pointer items-start gap-3 text-sm">
                  <Checkbox
                    className="mt-0.5 shrink-0"
                    aria-label="创建多级目录（例如：2026/参考资料）"
                    checked={multipleDirectories}
                    onCheckedChange={(e) => setMultipleDirectories(e)}
                  />
                  <span className="min-w-0">
                    创建多级目录
                    <span className="mt-1 block text-xs text-muted-foreground">例如：2026/参考资料</span>
                  </span>
                </label>
                <label className="flex cursor-pointer items-center gap-3 text-sm">
                  <Checkbox
                    className="shrink-0"
                    aria-label="创建后打开文件夹"
                    checked={openCreated}
                    onCheckedChange={(e) => setOpenCreated(e)}
                  />
                  <span>创建后打开文件夹</span>
                </label>
              </fieldset>
            )}
            {action === "file" && (
              <>
                <label>
                  文件格式
                  <SelectAdapter
                    value={fileType}
                    onChange={(e) => {
                      const type = e.target.value;
                      setFileType(type);
                      setValue((v) => v.replace(/\.[^.]*$/, "") + "." + type);
                      if (!textContent)
                        setTextContent(
                          type === "json"
                            ? "{}\n"
                            : type === "md"
                              ? "# 新文档\n"
                              : "",
                        );
                    }}
                  >
                    <option value="txt">纯文本 .txt</option>
                    <option value="md">Markdown .md</option>
                    <option value="json">JSON .json</option>
                  </SelectAdapter>
                </label>
                <label>
                  文件内容
                  <Textarea
                    rows={8}
                    value={textContent}
                    onChange={(e) => setTextContent(e.target.value)}
                    spellCheck={false}
                    placeholder="可留空，也可直接输入文件内容"
                  />
                </label>
                <small>
                  UTF-8 · {bytes(new TextEncoder().encode(textContent).length)}{" "}
                  · 同名文件不会被覆盖
                </small>
              </>
            )}
            {nameError && (
              <p role="alert" className="error">
                {nameError}
              </p>
            )}
            {error && (
              <p role="alert" className="error">
                {error}
              </p>
            )}
            <div className="dialog-actions">
              <Button variant="outline"
                type="button"

                onClick={() => setAction("")}
              >
                取消
              </Button>
              <Button type="submit"
                variant={action === "trash" ? "destructive" : "default"}
                disabled={busy || !!nameError}
              >
                {busy
                  ? "处理中…"
                  : action === "trash"
                    ? "移入回收站"
                    : action === "paste"
                      ? "确认粘贴"
                      : action === "move"
                        ? "确认移动"
                        : action === "export"
                          ? "开始复制"
                          : "确认"}
              </Button>
            </div>
          </form>
        </WorkspaceDialog>
      )}
    </div>
  );
}
