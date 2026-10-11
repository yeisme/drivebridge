import { Progress } from "./components/ui/progress";
import { DataTable } from "./components/DataTable";
import { Button } from "./components/ui/button";
import { DriveHome } from "./features/files/DriveHome";
import { LocalShareDialog, LocalShareCopies } from "./LocalSharing";
import React, { useEffect, useRef, useState } from "react";
import { api, bytes, errorText, type DriveFile, type Space } from "./api";
import {
  Finder,
  WorkspaceDialog,
  type FinderAdapter,
  type BrowserEntry,
} from "./Finder";
import { MediaPreview } from "./Media";

interface LocalFile {
  ref: string;
  name: string;
  dir?: string;
  size_bytes: number;
  mime_type: string;
  version: string;
  observed_at?: string;
  is_directory?: boolean;
}
function projection(f: LocalFile, space: string): DriveFile {
  const ext = f.name.split(".").pop()?.toLowerCase() || "";
  const types: Record<string, string> = {
    png: "image/png",
    jpg: "image/jpeg",
    jpeg: "image/jpeg",
    gif: "image/gif",
    webp: "image/webp",
    mp4: "video/mp4",
    mov: "video/quicktime",
    webm: "video/webm",
    mp3: "audio/mpeg",
    wav: "audio/wav",
    pdf: "application/pdf",
    txt: "text/plain",
    md: "text/markdown",
  };
  return {
    id: f.ref,
    ref: f.ref,
    version_id: f.version,
    path: [f.dir, f.name].filter(Boolean).join("/"),
    size_bytes: f.size_bytes,
    mime_type: f.mime_type || types[ext] || "application/octet-stream",
    space_id: space,
    trashed: false,
    updated_at: f.observed_at || new Date().toISOString(),
    thumbnail_url:
      "/api/v1/local/file/preview?kind=thumbnail&ref=" +
      encodeURIComponent(f.ref),
    compatibility_url:
      "/api/v1/local/file/preview?kind=compatible&ref=" +
      encodeURIComponent(f.ref),
    preview_url: "/api/v1/local/file/content?ref=" + encodeURIComponent(f.ref),
  };
}
const action = (q: Record<string, unknown>) =>
  api("/local/file/action", "POST", q);
export function LocalWorkspace({ csrf }: { csrf: string }) {
  const [spaces, setSpaces] = useState<Space[]>([]),
    [space, setSpace] = useState(""),
    [prefix, setPrefix] = useState(""),
    [revision, setRevision] = useState(0),
    [selected, setSelected] = useState<DriveFile>(),
    [error, setError] = useState(""),
    [notice, setNotice] = useState("");
  const [driveOpen, setDriveOpen] = useState(false);
  const [offline, setOffline] = useState(false);
  const [operations, setOperations] = useState<
    Array<{
      id: string;
      kind: string;
      status: string;
      bytes_total: number;
      bytes_transferred: number;
      error?: string;
    }>
  >([]);
  const [share, setShare] = useState<{ files: DriveFile[]; path: string }>();
  const input = useRef<HTMLInputElement>(null);
  useEffect(() => {
    void api<Space[]>("/spaces")
      .then((rows) => {
        setSpaces(rows);
        setSpace(rows[0]?.id || "");
      })
      .catch((e) => setError(errorText(e)));
  }, []);
  useEffect(() => {
    if (!space) return;
    let stopped = false;
    const poll = () =>
      api<typeof operations>("/local/file/action", "POST", {
        action: "operations",
        space_id: space,
      })
        .then((rows) => {
          if (!stopped) setOperations(rows);
        })
        .catch(() => {});
    void poll();
    const timer = setInterval(poll, 4000);
    return () => {
      stopped = true;
      clearInterval(timer);
    };
  }, [space]);
  const reload = async () => {
    setRevision((n) => n + 1);
  };
  const adapter: FinderAdapter = {
    directory: async (space, path, query, signal) => {
      try {
        const out = await api<{
          items: LocalFile[];
          directories?: LocalFile[];
          partial: boolean;
        }>("/local/list", "POST", { space_id: space, path }, {}, signal);
        if (signal.aborted) throw new DOMException("Aborted", "AbortError");
        setOffline(false);
        setNotice(out.partial ? "当前目录仅显示前 500 项，结果不完整。" : "");
        return [...(out.directories || []), ...out.items]
          .filter(
            (f) => !query || f.name.toLowerCase().includes(query.toLowerCase()),
          )
          .map(
            (f) =>
              ({
                kind: f.is_directory ? "directory" : "file",
                path: [path, f.name].filter(Boolean).join("/"),
                file: f.is_directory ? undefined : projection(f, space),
              }) as BrowserEntry,
          );
      } catch (error) {
        if (
          signal.aborted ||
          ![
            "insights_owner_unavailable",
            "listing_disabled",
            "list_failed",
          ].includes((error as Error).message)
        )
          throw error;
        const out = await api<{ items: LocalFile[]; partial: boolean }>(
          "/usage/files?space_id=" +
            encodeURIComponent(space) +
            "&category=current",
          "GET",
          undefined,
          {},
          signal,
        );
        if (signal.aborted) throw new DOMException("Aborted", "AbortError");
        setOffline(true);
        setNotice(
          "当前显示已观察索引，非全盘清单；连接恢复后可预览和操作。" +
            (out.partial ? " 索引结果不完整。" : ""),
        );
        const prefix = path ? path + "/" : "",
          dirs = new Set<string>();
        for (const file of out.items) {
          const dir = file.dir || "";
          if (dir.startsWith(prefix) && dir.length > prefix.length)
            dirs.add(prefix + dir.slice(prefix.length).split("/")[0]);
        }
        return [
          ...[...dirs].map(
            (path) => ({ kind: "directory", path }) as BrowserEntry,
          ),
          ...out.items
            .filter(
              (f) =>
                (f.dir || "") === path &&
                (!query || f.name.toLowerCase().includes(query.toLowerCase())),
            )
            .map(
              (f) =>
                ({
                  kind: "file",
                  path: [f.dir, f.name].filter(Boolean).join("/"),
                  file: {
                    ...projection(f, space),
                    preview_reason: "当前为已观察索引，连接恢复后才能预览。",
                  },
                }) as BrowserEntry,
            ),
        ];
      }
    },
    createFile: async (targetSpace, path, file, signal) => {
      const parts = path.split("/"); const name = parts.pop()!;
      const response = await fetch(`/api/v1/local/file/upload?${new URLSearchParams({space_id:targetSpace,dir:parts.join("/"),name})}`, {method:"PUT", headers:{"X-CSRF-Token":csrf,"Idempotency-Key":crypto.randomUUID()},body:file,signal});
      if (!response.ok) {const result=await response.json();throw new Error(result.error?.code || "upload_failed");}
    },
    mkdir: (space, path) => action({ action: "mkdir", space_id: space, path }),
    move: (file, target) =>
      action({
        action: "move",
        ref: file.ref || file.id,
        path: target.split("/").slice(0, -1).join("/"),
        name: target.split("/").pop(),
      }),
    trash: (file) => action({ action: "trash", ref: file.ref }),
    copy: (file, _space, destPath) => {
      const parts = destPath.split("/");
      const name = parts.pop() || file.path.split("/").pop() || "file";
      return action({
        action: "copy",
        ref: file.ref,
        path: parts.join("/"),
        name,
      });
    },
    copyDirectory: (space, src, destSpace, destPath) => {
      if (destSpace && destSpace !== space)
        return Promise.reject(new Error("unsupported"));
      return action({
        action: "copy_directory",
        space_id: space,
        path: src,
        name: destPath,
      });
    },
    moveDirectory: (space, src, destPath) =>
      action({
        action: "move_directory",
        space_id: space,
        path: src,
        name: destPath,
      }),
    trashDirectory: (space, path) =>
      action({ action: "trash_directory", space_id: space, path }),
  };
  const upload = async (files?: File[]) => {
    if (!files) {
      input.current?.click();
      return;
    }
    setError("");
    for (const file of files) {
      try {
        const r = await fetch(
          `/api/v1/local/file/upload?${new URLSearchParams({ space_id: space, dir: prefix, name: file.name })}`,
          {
            method: "PUT",
            headers: {
              "X-CSRF-Token": csrf,
              "Idempotency-Key": crypto.randomUUID(),
            },
            body: file,
          },
        );
        const result = await r.json();
        if (!r.ok) throw new Error(result.error?.code || "upload_failed");
        setNotice("文件已交给本机后台传输；可在下方查看进度。");
        await reload();
      } catch (e) {
        setError(errorText(e));
        break;
      }
    }
  };
  return (
    <>
      <input
        ref={input}
        type="file"
        multiple
        hidden
        onChange={(e) => {
          void upload(Array.from(e.target.files || []));
          e.target.value = "";
        }}
      />
      {error && (
        <p role="alert" className="banner error">
          {error}
        </p>
      )}
      {notice && (
        <p role="status" className="banner">
          {notice}
        </p>
      )}
      {!driveOpen && (
        <DriveHome
          spaces={spaces}
          connections={[]}
          open={(s) => {
            setSpace(s.id);
            setPrefix("");
            setDriveOpen(true);
          }}
        />
      )}
      {driveOpen && (
        <>
          <div className="drive-location">
            <Button variant="outline" size="sm" type="submit"

              onClick={() => setDriveOpen(false)}
            >
              ← 全部网盘
            </Button>
            <div className="drive-identity"><h2>{spaces.find((s) => s.id === space)?.name}</h2><span>本机授权空间</span></div>
          </div>
          <Finder
            spaces={spaces}
            space={space}
            setSpace={setSpace}
            prefix={prefix}
            setPrefix={setPrefix}
            revision={revision}
            reload={reload}
            adapter={adapter}
            readOnly={offline}
            backendLabel="本机授权空间"
            upload={(files) => void upload(files)}
            onPreview={setSelected}
            onShare={(files, path) => setShare({ files, path })}
          />
          <section className="local-transfers">
            <h2>传输中心</h2>
            <p className="field-help">
              已交给 owner
              的任务可在关页后继续；浏览器仍在发送的文件中断后需重新选择。
            </p>
            {<DataTable label="交付记录" data={operations} getRowId={(op)=>String(op.id)} columns={[{id:"type",header:"类型",sortable:false,cell:(op)=><><div className="transfer-info">
                  <strong>
                    {op.kind} · {op.status}
                  </strong>
                  <small>
                    {bytes(op.bytes_transferred)} / {bytes(op.bytes_total)}
                  </small>
                  <Progress
                    value={op.bytes_transferred}
                    max={op.bytes_total || 1}
                  />
                  {op.error && (
                    <small className="error">
                      {errorText(new Error(op.error))}
                    </small>
                  )}
                </div></>},{id:"details",header:"内容与状态",cell:(op)=><>{op.status !== "completed" && op.status !== "cancelled" && (
                  <>
                    <Button variant="outline" size="sm" type="submit"

                      onClick={() =>
                        void action({
                          action: "resume",
                          operation_id: op.id,
                        }).catch((e) => setError(errorText(e)))
                      }
                    >
                      恢复
                    </Button>
                    <Button variant="outline" size="sm" type="submit"

                      onClick={() =>
                        void action({
                          action: "cancel",
                          operation_id: op.id,
                        }).catch((e) => setError(errorText(e)))
                      }
                    >
                      取消
                    </Button>
                  </>
                )}</>},{id:"actions",header:"操作",sortable:false,cell:(op)=><div className="flex flex-wrap gap-2"></div>}]} />}
          </section>
          <LocalShareCopies />
        </>
      )}
      {share && (
        <LocalShareDialog
          {...share}
          space={space}
          close={() => setShare(undefined)}
        />
      )}
      {selected && (
        <WorkspaceDialog
          title={selected.path}
          close={() => setSelected(undefined)}
        >
          <MediaPreview file={selected} />
          <Button variant="outline" type="submit"

            onClick={() =>
              void navigator.clipboard
                .writeText(selected.ref!)
                .catch((e) => setError(errorText(e)))
            }
          >
            复制固定引用
          </Button>
        </WorkspaceDialog>
      )}
    </>
  );
}
