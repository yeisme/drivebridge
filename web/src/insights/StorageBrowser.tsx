import { Input } from "../components/ui/input";
import { Checkbox } from "../components/ui/checkbox";
import { GridTable } from "../components/DataTable";
import { Button } from "../components/ui/button";
import { SelectAdapter } from "./../components/SelectAdapter";
import React, { useEffect, useState } from "react";
import { api, bytes, errorText } from "../api";
import { backendOf } from "../storage";

type BrowserSpace = {
  id: string;
  name: string;
  connection_id: string;
  backend?: string;
};
interface BrowserItem {
  ref?: string;
  name: string;
  dir?: string;
  size_bytes: number;
  mime_type?: string;
  observed_at?: string;
  caller?: string;
}

// Live listing needs the owner process; every other path falls back to the
// observed index. The wording must say which one is on screen.
const LIVE_REASONS: Record<string, string> = {
  insights_owner_unavailable:
    "未检测到运行中的 owner（drivebridge mcp --enabled --insights）",
  listing_disabled: "实时浏览需 owner 启用文件操作（--enabled）",
};

export function StorageBrowser({ spaces }: { spaces: BrowserSpace[] }) {
  const [spaceId, setSpaceId] = useState(spaces[0]?.id ?? "");
  const [dir, setDir] = useState("");
  const [trash, setTrash] = useState(false);
  const [items, setItems] = useState<BrowserItem[]>([]);
  const [subdirs, setSubdirs] = useState<string[]>([]);
  const [live, setLive] = useState(false);
  const [partial, setPartial] = useState(false);
  const [reason, setReason] = useState("");
  const [error, setError] = useState("");
  const [notice, setNotice] = useState("");
  const [busy, setBusy] = useState(false);
  const [goto, setGoto] = useState("");

  useEffect(() => {
    if (spaces.length && !spaces.some((s) => s.id === spaceId)) {
      setSpaceId(spaces[0].id);
      setDir("");
    }
  }, [spaces, spaceId]);

  useEffect(() => {
    const controller = new AbortController();
    (async () => {
      if (!spaceId) return;
      setBusy(true);
      setError("");
      setNotice("");
      try {
        let why = "";
        let liveItems: BrowserItem[] | null = null;
        let livePartial = false;
        if (trash) {
          why = "回收站按已观察索引显示";
        } else {
          try {
            const out = await api<{ items: BrowserItem[]; partial: boolean }>(
              "/local/list",
              "POST",
              { space_id: spaceId, path: dir },
              {},
              controller.signal,
            );
            liveItems = out.items;
            livePartial = out.partial;
          } catch (e) {
            const code = e instanceof Error ? e.message : "";
            why = LIVE_REASONS[code] || "实时列表暂不可用，显示已观察索引";
          }
        }
        const index = await api<{ items: BrowserItem[]; partial: boolean }>(
          "/usage/files?space_id=" +
            encodeURIComponent(spaceId) +
            "&category=" +
            (trash ? "trash" : "current"),
          "GET",
          undefined,
          {},
          controller.signal,
        );
        // Subdirectories can only come from the observed index: both
        // providers list files, never folders.
        const prefix = dir ? dir + "/" : "";
        const children = new Set<string>();
        for (const f of index.items) {
          const d = f.dir || "";
          if (d.startsWith(prefix) && d.length > prefix.length) {
            children.add(d.slice(prefix.length).split("/")[0]);
          }
        }
        setItems(
          liveItems ?? index.items.filter((f) => (f.dir || "") === dir),
        );
        setSubdirs([...children].sort());
        setLive(liveItems != null);
        setPartial(livePartial || index.partial);
        setReason(why);
      } catch (e) {
        if ((e as Error).name !== "AbortError") setError(errorText(e));
      } finally {
        setBusy(false);
      }
    })();
    return () => controller.abort();
  }, [spaceId, dir, trash]);

  const backend = backendOf(spaces.find((s) => s.id === spaceId)?.backend);
  const segments = dir ? dir.split("/") : [];
  const jump = () => {
    const raw = goto.trim().replace(/^\/+|\/+$/g, "");
    setGoto("");
    if (!raw) {
      setDir("");
      return;
    }
    if (raw.split("/").some((part) => !part || part === "." || part === "..")) {
      setError("路径无效：使用相对目录，不含 .. 或空段。");
      return;
    }
    setError("");
    setTrash(false);
    setDir(raw);
  };

  if (!spaces.length)
    return (
      <section className="insight-card" aria-label="存储浏览">
        <h3>存储浏览</h3>
        <p>尚无已配置空间。先在终端运行 drivebridge setup 配置空间后回来浏览。</p>
      </section>
    );
  return (
    <section className="insight-card storage-browser" aria-label="存储浏览">
      <div className="storage-toolbar">
        <label>
          空间
          <SelectAdapter
            aria-label="浏览空间"
            value={spaceId}
            onChange={(e) => {
              setSpaceId(e.target.value);
              setDir("");
              setTrash(false);
            }}
          >
            {spaces.map((s) => (
              <option key={s.id} value={s.id}>
                {s.name || s.id}
              </option>
            ))}
          </SelectAdapter>
        </label>
        <span
          className={
            "backend-chip" + (backend.unrecognized ? " unrecognized" : "")
          }
        >
          <strong>{backend.label}</strong>
          <small>{backend.directUrl ? "传输 · 临时直链 · 参考交接" : "仅传输"}</small>
        </span>
        <label className="storage-toggle">
          <Checkbox

            checked={trash}
            onCheckedChange={(e) => setTrash(e)}
          />
          查看回收站
        </label>
        <label className="storage-goto">
          前往目录
          <Input
            aria-label="前往目录"
            placeholder="如 refs/2026"
            value={goto}
            onChange={(e) => setGoto(e.target.value)}
            onKeyDown={(e) => {
              if (e.key === "Enter") jump();
            }}
          />
          <Button variant="outline" size="sm" type="submit"  onClick={jump}>
            前往
          </Button>
        </label>
      </div>
      <nav className="storage-breadcrumb" aria-label="目录路径">
        <Button variant="ghost" type="submit" className="text-button" onClick={() => setDir("")}>
          根目录
        </Button>
        {segments.map((segment, i) => (
          <React.Fragment key={i}>
            <span aria-hidden="true">/</span>
            <Button variant="ghost" type="submit"
              className="text-button"
              onClick={() => setDir(segments.slice(0, i + 1).join("/"))}
            >
              {segment}
            </Button>
          </React.Fragment>
        ))}
      </nav>
      <p className="storage-banner" role="status">
        {busy
          ? "正在读取…"
          : reason
            ? reason + "；子目录聚合来自已观察索引，非全盘。"
            : live
              ? "实时目录 · 列表会更新已观察索引。"
              : "已观察索引，非全盘；索引未命中不代表远端没有。"}
        {partial && " 当前仅显示前 500 条（索引聚合前 200 条）。"}
      </p>
      {error && (
        <p role="alert">
          {error}
        </p>
      )}
      {notice && <p role="status">{notice}</p>}
      <div className="insight-table-wrap">
        <GridTable headers={[<> 名称 </>,<> 大小 </>,<> 类型 </>,<> 观察时间（UTC） </>,<> 引用 </>]} rows={[...(subdirs.map((name) => (({id:String("d:" + name),cells:[<>
                  <Button variant="ghost" type="submit"
                    className="text-button"
                    onClick={() => setDir(dir ? dir + "/" + name : name)}
                  >
                    {name}/
                  </Button>
                 </>]})))),...(items.map((f) => (({id:String(f.ref || f.name),cells:[<> {f.name} </>,<> {bytes(f.size_bytes || 0)} </>,<> {f.mime_type || "未知"} </>,<>
                  {f.observed_at ? f.observed_at.slice(0, 19).replace("T", " ") : "未记录"}
                 </>,<>
                  {f.ref ? (
                    <Button variant="ghost" type="submit"
                      className="text-button"
                      onClick={() => {
                        navigator.clipboard.writeText(f.ref!);
                        setNotice("已复制固定引用，可在已连接的 Agent 中使用。");
                      }}
                    >
                      复制引用
                    </Button>
                  ) : (
                    "—"
                  )}
                 </>]}))))]}/>
      </div>
      {!subdirs.length && !items.length && !busy && (
        <p>{trash ? "回收站没有已观察记录。" : "此目录暂无已观察文件。"}</p>
      )}
      <p className="storage-note">
        浏览完全只读；未观察过的子目录不会出现在导航中，可用「前往目录」直接输入已知路径。
      </p>
    </section>
  );
}
