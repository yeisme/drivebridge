import { Checkbox } from "./components/ui/checkbox";
import { Input } from "./components/ui/input";
import { DataTable } from "./components/DataTable";
import { Button } from "./components/ui/button";
import { SelectAdapter } from "./components/SelectAdapter";
import React, { useEffect, useState } from "react";
import { api, errorText, type DriveFile } from "./api";
import { AppDialog as WorkspaceDialog } from "./components/AppDialog";
interface Copy {
  id: string;
  name: string;
  status: string;
  error?: string;
  mode: string;
  updated_at: string;
  remote_id?: string;
}
const request = <T,>(body: unknown) =>
  api<T>("/local/file/share", "POST", body);
export function LocalShareDialog({
  files,
  path,
  space,
  close,
}: {
  files: DriveFile[];
  path: string;
  space: string;
  close: () => void;
}) {
  const [spaces, setSpaces] = useState<Array<{ id: string; name: string }>>([]),
    [configure, setConfigure] = useState(false),
    [mode, setMode] = useState("fixed"),
    [audience, setAudience] = useState("link"),
    [error, setError] = useState(""),
    [busy, setBusy] = useState(false),
    [created, setCreated] = useState(false);
  const load = () =>
    request<typeof spaces>({ action: "spaces" })
      .then(setSpaces)
      .catch(() => setConfigure(true));
  useEffect(() => {
    void load();
  }, []);
  return (
    <WorkspaceDialog title="分享远程副本" close={close}>
      {created ? (
        <>
          <p>已加入本机后台。远程副本准备完成后，可在“分享副本”中复制链接。</p>
          <p className="field-help">
            固定交付在电脑离线后仍可访问；持续共享需要本机 owner
            在线才能同步更新。
          </p>
          <Button variant="default" type="submit"  onClick={close}>
            完成
          </Button>
        </>
      ) : configure ? (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            const f = new FormData(e.currentTarget);
            setBusy(true);
            setError("");
            void request({
              action: "configure",
              destination: { url: f.get("url"), token: f.get("token") },
            })
              .then(() => {
                setConfigure(false);
                return load();
              })
              .catch((e) => setError(errorText(e)))
              .finally(() => setBusy(false));
          }}
        >
          <p>
            先连接保存副本的 DriveBridge 服务。连接凭据由本机 owner 加密保存。
          </p>
          <label>
            服务地址
            <Input
              name="url"
              type="url"
              required
              placeholder="https://files.example.com"
            />
          </label>
          <label>
            用户访问密钥
            <Input name="token" type="password" autoComplete="off" required />
          </label>
          <p className="field-help">
            创建分享需要用户身份；Agent 文件授权不能创建分享。
          </p>
          {error && (
            <p className="error" role="alert">
              {error}
            </p>
          )}
          <Button variant="default" type="submit"  disabled={busy}>
            连接远程空间
          </Button>
        </form>
      ) : (
        <form
          onSubmit={(e) => {
            e.preventDefault();
            const f = new FormData(e.currentTarget);
            setBusy(true);
            setError("");
            void request({
              action: "create",
              policy: {
                audience,
                recipients: String(f.get("recipients") || "")
                  .split(/[\s,]+/)
                  .filter(Boolean),
                passcode: f.get("passcode") || "",
                allow_download: f.get("download") === "on",
              },
              refs: files.map((file) => file.ref),
              share: {
                name: f.get("name"),
                space_id: space,
                dir: path,
                target_space: f.get("target"),
                mode,
                days: Number(f.get("days")),
              },
            })
              .then(() => setCreated(true))
              .catch((e) => setError(errorText(e)))
              .finally(() => setBusy(false));
          }}
        >
          <label>
            分享名称
            <Input
              name="name"
              required
              maxLength={120}
              defaultValue={
                files[0]?.path.split("/").pop() ||
                path.split("/").pop() ||
                "素材交付"
              }
            />
          </label>
          <label>
            远程副本空间
            <SelectAdapter name="target" required>
              {spaces.map((s) => (
                <option key={s.id} value={s.id}>
                  {s.name}
                </option>
              ))}
            </SelectAdapter>
          </label>
          <label>
            内容模式
            <SelectAdapter value={mode} onChange={(e) => setMode(e.target.value)}>
              <option value="fixed">固定交付</option>
              {files.length === 0 && <option value="live">持续共享</option>}
            </SelectAdapter>
          </label>
          <label>
            接收方
            <SelectAdapter
              value={audience}
              onChange={(e) => setAudience(e.target.value)}
            >
              <option value="link">持链接的人</option>
              <option value="users">指定已登录用户</option>
            </SelectAdapter>
          </label>
          {audience === "users" && (
            <label>
              接收方用户 ID
              <Input name="recipients" required placeholder="多个用逗号分隔" />
            </label>
          )}
          <label>
            提取码（可选）
            <Input
              name="passcode"
              type="password"
              minLength={6}
              maxLength={72}
              autoComplete="new-password"
            />
          </label>
          <label className="checkbox-label">
            <Checkbox name="download"  defaultChecked />
            允许下载原文件
          </label>
          <label>
            有效天数
            <Input
              name="days"
              type="number"
              min={1}
              max={365}
              defaultValue={7}
              required
            />
          </label>
          <p className="field-help">
            将复制
            {files.length
              ? `${files.length} 个素材`
              : `目录 ${path || "根目录"}`}
            到远程空间，占用相应容量。
            {mode === "live"
              ? "目录新增和修改会同步到分享副本。"
              : "固定交付保留当前文件版本。"}
          </p>
          {error && (
            <p className="error" role="alert">
              {error}
            </p>
          )}
          <Button variant="default" type="submit"  disabled={busy || !spaces.length}>
            创建远程副本并分享
          </Button>
        </form>
      )}
    </WorkspaceDialog>
  );
}
export function LocalShareCopies() {
  const [copies, setCopies] = useState<Copy[]>([]),
    [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [revoking, setRevoking] = useState<Copy>();
  useEffect(() => {
    let active = true;
    const load = () =>
      request<Copy[]>({ action: "list" })
        .then((rows) => {
          if (active) setCopies(rows);
        })
        .catch(() => {});
    void load();
    const timer = setInterval(load, 4000);
    return () => {
      active = false;
      clearInterval(timer);
    };
  }, []);
  if (!copies.length) return null;
  return (
    <section className="local-transfers">
      <h2>分享副本</h2>
      <p className="field-help">
        链接访问由远程服务负责。同步失败时，已发布的远程副本仍保留上次内容。
      </p>
      {error && (
        <p role="alert" className="error">
          {error}
        </p>
      )}
      {notice && <p role="status">{notice}</p>}
      {<DataTable label="交付记录" data={copies} getRowId={(copy)=>String(copy.id)} columns={[{id:"type",header:"类型",sortable:false,cell:(copy)=><><div className="transfer-info">
            <strong>{copy.name}</strong>
            <small>
              {copy.mode === "fixed" ? "固定交付" : "持续共享"} ·{" "}
              {(
                {
                  queued: "等待复制",
                  syncing: "正在同步",
                  ready: "副本已就绪",
                  failed: "同步失败，内容可能滞后",
                  revoked: "已撤销",
                  revocation_pending: "撤销待重试",
                  expired: "已过期",
                } as Record<string, string>
              )[copy.status] || copy.status}
            </small>
            <small>
              状态更新时间：{new Date(copy.updated_at).toLocaleString("zh-CN")}
            </small>
            {copy.error && (
              <small className="error">
                {errorText(new Error(copy.error))}
              </small>
            )}
          </div></>},{id:"details",header:"内容与状态",cell:(copy)=><>{copy.remote_id && !["revoked", "expired"].includes(copy.status) && (
            <Button variant="outline" size="sm" type="submit"

              onClick={() =>
                void request<{ link: string }>({ action: "link", id: copy.id })
                  .then((out) => navigator.clipboard.writeText(out.link))
                  .then(() => setNotice("已复制远程分享链接"))
                  .catch((e) => setError(errorText(e)))
              }
            >
              复制链接
            </Button>
          )}</>},{id:"actions",header:"操作",sortable:false,cell:(copy)=><div className="flex flex-wrap gap-2">{copy.status === "failed" && (
            <Button variant="outline" size="sm" type="submit"

              onClick={() =>
                void request({ action: "retry", id: copy.id }).catch((e) =>
                  setError(errorText(e)),
                )
              }
            >
              恢复同步
            </Button>
          )}
{!["revoked", "expired"].includes(copy.status) && (
            <Button variant="destructive" size="sm" type="submit"

              onClick={() => setRevoking(copy)}
            >
              撤销分享
            </Button>
          )}</div>}]} />}
      {revoking && (
        <WorkspaceDialog title="撤销分享" close={() => setRevoking(undefined)}>
          <p>撤销「{revoking.name}」后，接收方将无法继续访问远程副本。</p>
          <div className="dialog-actions">
            <Button variant="outline" type="submit"  onClick={() => setRevoking(undefined)}>
              取消
            </Button>
            <Button variant="destructive" type="submit"

              onClick={() =>
                void request({ action: "revoke", id: revoking.id })
                  .then(() => setRevoking(undefined))
                  .catch((e) => setError(errorText(e)))
              }
            >
              确认撤销分享
            </Button>
          </div>
        </WorkspaceDialog>
      )}
    </section>
  );
}
