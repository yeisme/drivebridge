import { DropdownMenu, DropdownMenuTrigger, DropdownMenuContent, DropdownMenuItem, DropdownMenuSeparator } from "./components/ui/dropdown-menu";
import { Input } from "./components/ui/input";
import { SearchField } from "./components/SearchField";
import "./key-directory.css";
import { DataTable } from "./components/DataTable";
import { Button } from "./components/ui/button";
import { SelectAdapter } from "./components/SelectAdapter";
import { SelectField } from "./components/SelectField";
import React, { useState } from "react";
import {
  KeyRound,
  MoreHorizontal,
  Search,
  Copy,
  Pencil,
  ChevronRight,
  ShieldCheck,
  Eye,
  EyeOff,
  RefreshCw,
  Check,
  Trash2,
  Pause,
  Play,
} from "lucide-react";
import { api, errorText, type UserKeyRow } from "./api";
import { AppDialog as WorkspaceDialog } from "./components/AppDialog";
import type { GovernanceState } from "./Governance";
export function keyState(k: UserKeyRow) {
  return k.archived
    ? "已归档"
    : k.revoked
      ? "已吊销"
      : k.disabled
        ? "已禁用"
        : k.expires_at &&
            !k.expires_at.startsWith("0001") &&
            new Date(k.expires_at) <= new Date()
          ? "已过期"
          : "有效";
}
export function KeyDirectory({
  keys,
  data,
  canManage,
  reload,
  revoke,
  archive,
}: {
  keys: UserKeyRow[];
  data?: GovernanceState;
  canManage: boolean;
  reload: () => Promise<void>;
  revoke: (k: UserKeyRow) => void;
  archive: (k: UserKeyRow) => void;
}) {
  const [query, setQuery] = useState(""),
    [filter, setFilter] = useState("all"),
    [detail, setDetail] = useState<UserKeyRow>(),
    [edit, setEdit] = useState<UserKeyRow>(),
    [name, setName] = useState(""),
    [busy, setBusy] = useState(false),
    [error, setError] = useState(""),
    [notice, setNotice] = useState("");
  const [creating,setCreating]=useState(false);
  const [createName,setCreateName]=useState("");
  const [days,setDays]=useState("90");
  const [revokeOwn, setRevokeOwn] = useState<UserKeyRow>();
  const [deleteOwn, setDeleteOwn] = useState<UserKeyRow>();
  const [disableOwn, setDisableOwn] = useState<UserKeyRow>();
  const [rotation, setRotation] = useState<UserKeyRow>();
  const [secret, setSecret] = useState<{
    id: string;
    name: string;
    key: string;
  }>();
  const [visible, setVisible] = useState(false);
  const [copied, setCopied] = useState("");
  const unavailable = (k: UserKeyRow) =>
    k.can_copy === undefined
      ? "当前服务不支持取回 Key，请更新服务端。"
      : k.copy_unavailable_reason === "key_material_unavailable"
      ? "旧 Key 需用原密钥登录一次或轮转后，才能再次复制。"
      : k.copy_unavailable_reason === "owner_only"
        ? "仅所属用户可查看、复制或轮转 Key。"
        : "此 Key 已失效，无法复制。";
  const rows = keys.filter(
    (k) =>
      `${k.name} ${k.id} ${k.user} ${k.user_name || ""}`
        .toLowerCase()
        .includes(query.toLowerCase()) &&
      (filter === "all" ? !k.archived : filter === keyState(k)),
  );
  const teams = (k: UserKeyRow) =>
    data?.team_members
      .filter((m) => m.user_id === k.user)
      .map(
        (m) =>
          data.teams.find((t) => t.id === m.team_id)?.name ||
          m.team_id.slice(0, 8),
      ) || [];
  async function readKey(k: UserKeyRow, copy = false) {
    setBusy(true);
    setError("");
    setNotice("");
    try {
      const result = await api<{ key: string }>(
        `/key-directory/${k.id}/reveal`,
        "POST",
      );
      if (copy) {
        try {
          await navigator.clipboard.writeText(result.key);
          setCopied(k.id);
          setNotice("已复制 Key，可直接用于登录或 API 请求。");
          return;
        } catch {
          setError("浏览器未允许自动复制，请显示 Key 后手动复制。");
        }
      }
      setDetail(undefined);
      setVisible(false);
      setSecret({ id: k.id, name: k.name, key: result.key });
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }
  async function copyShownKey() {
    if (!secret) return;
    try {
      await navigator.clipboard.writeText(secret.key);
      setCopied(secret.id);
      setNotice("已复制 Key。");
      setError("");
    } catch {
      setError("浏览器未允许自动复制，请显示 Key 后手动复制。");
    }
  }
  async function rotateKey() {
    if (!rotation || busy) return;
    setBusy(true);
    setError("");
    try {
      const result = await api<{ id: string; name: string; key: string }>(
        `/key-directory/${rotation.id}/rotate`,
        "POST",
        { confirm: true },
      );
      setRotation(undefined);
      setDetail(undefined);
      setCopied("");
      setVisible(false);
      setSecret(result);
      setNotice("轮转成功，旧 Key 已失效。请更新使用该 Key 的客户端。");
      await reload();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }
  async function confirmLifecycle(k: UserKeyRow, destroy: boolean) {
    setBusy(true);
    setError("");
    try {
      await api(
        `/key-directory/${k.id}/${destroy ? "delete" : "revoke"}`,
        "POST",
        { confirm: true },
      );
      setRevokeOwn(undefined);
      setDeleteOwn(undefined);
      setDetail(undefined);
      setNotice(destroy ? "Key 已删除，密文已擦除。" : "Key 已吊销，无法再登录。");
      await reload();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }
  async function setDisabled(k: UserKeyRow, disabled: boolean) {
    setBusy(true);
    setError("");
    try {
      await api(
        `/key-directory/${k.id}/${disabled ? "disable" : "enable"}`,
        "POST",
        disabled ? { confirm: true } : {},
      );
      setDisableOwn(undefined);
      setDetail(undefined);
      setNotice(disabled ? "Key 已禁用，该 Key 的登录会话已结束。" : "Key 已重新启用，可用它再次登录。");
      await reload();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }
  return (
    <>
      <div className="key-explainer">
        <ShieldCheck size={22} />
        <div>
          <strong>每个 Key 都有明确的主人</strong>
          <p>
            登录 Key
            绑定用户，通过团队成员关系获得网盘权限。本人可查看、反复复制及轮转
            Key。可禁用后再次启用；吊销和删除不可恢复。不能对最后一把有效登录
            Key 做禁用、吊销或删除。
          </p>
        </div>
      </div>
      {creating && <WorkspaceDialog title="新建 Key" close={()=>setCreating(false)}><form onSubmit={async e=>{e.preventDefault();setBusy(true);setError("");try{const out=await api<{id:string;name:string;key:string}>("/key-directory","POST",{name:createName.trim(),days:Number(days)});setSecret(out);setVisible(false);setCreating(false);await reload();}catch(e){setError(errorText(e));}finally{setBusy(false);}}}><label>Key 名称<Input autoFocus required maxLength={80} value={createName} onChange={e=>setCreateName(e.target.value)}/></label><label>有效期<SelectAdapter value={days} onChange={e=>setDays(e.target.value)}><option value="30">30 天</option><option value="90">90 天</option><option value="365">1 年</option><option value="0">长期有效</option></SelectAdapter></label><p>绑定当前账号并沿用其权限。现有 Key 继续有效。</p><div className="dialog-actions"><Button variant="outline" type="button" onClick={()=>setCreating(false)}>取消</Button><Button variant="default" type="submit"  disabled={busy||!createName.trim()}>{busy?"正在创建…":"创建 Key"}</Button></div>{error&&<p role="alert" className="banner error">{error}</p>}</form></WorkspaceDialog>}
      <div className="workspace-toolbar">
        <Button variant="default" type="submit"  onClick={()=>{setCreateName("");setError("");setCreating(true);}}>新建 Key</Button>
        <SearchField
            aria-label="搜索 Key"
            placeholder="搜索名称、Key 标识或用户…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        <SelectField aria-label="Key 状态" value={filter} onValueChange={setFilter} options={["all","有效","已禁用","已过期","已吊销","已归档"].map(value=>({value,label:value==="all"?"全部未归档":value}))} />
        <span className="text-muted-foreground">{rows.length} 个 Key</span>
      </div>
      {error && (
        <p className="banner error" role="alert">
          {error}
        </p>
      )}
      {notice && (
        <p className="banner success" role="status">
          {notice}
        </p>
      )}
      <div className="surface-panel table-scroll key-directory-table">
        <DataTable searchable={false} label="Key 目录" data={rows} getRowId={(k)=>String(k.id)} columns={[{id:"c0",header:<> 名称 / 标识 </>,cell:(k)=><>
                  <Button layout="surface" variant="ghost" type="button" className="key-name max-w-64 min-w-40 whitespace-normal" onClick={() => setDetail(k)}>
                    <KeyRound size={18} className="shrink-0" />
                    <span className="min-w-0">
                      <strong className="flex-wrap break-words">
                        {k.name || "未命名 Key"}
                        {k.current && <span className="pill">当前登录</span>}
                      </strong>
                      <small>key_{k.id.slice(0, 12)}</small>
                    </span>
                  </Button>
                </>},
{id:"c1",header:<> 绑定用户 </>,cell:(k)=><>
                  <strong title={k.user}>{k.user_name || k.name}</strong>
                  <small title={k.user}>
                    {k.admin ? "管理员" : "用户"}
                  </small>
                </>},
{id:"c2",header:<> 团队 </>,cell:(k)=><>
                  {teams(k).length ? (
                    teams(k).map((t) => (
                      <span className="pill max-w-36" key={t} title={t}>
                        <span className="block truncate">{t}</span>
                      </span>
                    ))
                  ) : (
                    <span className="text-muted-foreground">未加入团队</span>
                  )}
                </>},
{id:"c3",header:<> 状态 / 有效期 </>,cell:(k)=><>
                  <span
                    className={`key-status ${keyState(k) === "有效" ? "valid" : ""}`}
                  >
                    {keyState(k)}
                  </span>
                  <small>
                    {k.expires_at && !k.expires_at.startsWith("0001")
                      ? k.expires_at.slice(0, 10)
                      : "长期有效"}
                  </small>
                </>},
{id:"c4",header:<> 操作 </>,cell:(k)=><>
                  <div className="flex min-w-max items-center justify-end gap-2">
                    <Button variant="outline" type="button" title={k.can_copy ? "复制完整 Key" : unavailable(k)} aria-label={`${copied===k.id ? "再次复制" : "复制"} ${k.name} 的 Key`} disabled={busy || !k.can_copy} onClick={()=>void readKey(k,true)}>{copied===k.id ? <Check/> : <Copy/>}{copied===k.id ? "已复制" : "复制 Key"}</Button>
                    <Button variant="ghost" type="button" onClick={()=>setDetail(k)}>详情</Button>
                    <DropdownMenu>
                      <DropdownMenuTrigger render={<Button variant="ghost" type="button" size="icon" aria-label={`${k.name} 的更多操作`} disabled={busy}><MoreHorizontal/></Button>}/>
                      <DropdownMenuContent align="end" className="min-w-48" aria-label="Key 操作">
                        <DropdownMenuItem disabled={!k.can_copy} onClick={()=>void readKey(k)}><Eye/>查看 Key</DropdownMenuItem>
                        {!k.archived && <DropdownMenuItem onClick={()=>{setEdit(k);setName(k.name);setError("");}}><Pencil/>修改名称</DropdownMenuItem>}
                        {k.can_rotate && <DropdownMenuItem onClick={()=>{setRotation(k);setError("");setNotice("");}}><RefreshCw/>轮转 Key</DropdownMenuItem>}
                        {(k.can_disable || k.can_enable || k.can_revoke || k.can_delete) && <DropdownMenuSeparator/>}
                        {k.can_disable && <DropdownMenuItem onClick={()=>{setDisableOwn(k);setError("");}}><Pause/>禁用</DropdownMenuItem>}
                        {k.can_enable && <DropdownMenuItem onClick={()=>void setDisabled(k,false)}><Play/>启用</DropdownMenuItem>}
                        {k.can_revoke && <DropdownMenuItem variant="destructive" onClick={()=>{setRevokeOwn(k);setError("");}}>吊销</DropdownMenuItem>}
                        {k.can_delete && <DropdownMenuItem variant="destructive" onClick={()=>{setDeleteOwn(k);setError("");}}><Trash2/>删除</DropdownMenuItem>}
                      </DropdownMenuContent>
                    </DropdownMenu>
                  </div>
                </>}]} />
        {!rows.length && (
          <div className="workspace-empty">
            <KeyRound size={30} />
            <h3>
              {keys.length ? "没有匹配的 Key" : "还没有签发任何用户密钥。"}
            </h3>
            <p>仅显示当前身份可管理的凭据。</p>
          </div>
        )}
      </div>
      {detail && (
        <WorkspaceDialog
          title="Key 详情与绑定"
          close={() => setDetail(undefined)}
        >
          <div className="detail-heading">
            <span className="surface-icon">
              <KeyRound />
            </span>
            <div>
              <h3>{detail.name}</h3>
              <span className="pill">{keyState(detail)}</span>
            </div>
          </div>
          <dl className="metadata-list">
            <dt>Key 标识</dt>
            <dd>
              <code>{detail.id}</code>
            </dd>
            <dt>所属用户</dt>
            <dd>
              {detail.user_name || detail.name}
              <small>{detail.user}</small>
            </dd>
            <dt>所属团队</dt>
            <dd>{teams(detail).join("、") || "未加入团队"}</dd>
            <dt>身份权限</dt>
            <dd>{detail.admin ? "管理员身份" : "普通用户身份"}</dd>
            <dt>有效期</dt>
            <dd>
              {detail.expires_at && !detail.expires_at.startsWith("0001")
                ? detail.expires_at.slice(0, 10)
                : "长期有效"}
            </dd>
          </dl>
          <p className="info-note">
            Key
            的用户绑定不可转移。团队及网盘权限请在“用户与团队”管理，不通过修改
            Key 名称变更。
          </p>
          <div className="dialog-actions">
            <Button variant="outline" type="submit"

              disabled={busy || !detail.can_copy}
              onClick={() => void readKey(detail, true)}
            >
              <Copy size={15} />
              复制 Key
            </Button>
            {detail.can_rotate && (
              <Button variant="outline" type="submit"

                disabled={busy}
                onClick={() => {
                  setRotation(detail);
                  setDetail(undefined);
                  setError("");
                }}
              >
                <RefreshCw size={15} />
                轮转 Key
              </Button>
            )}
            {detail.can_disable && (
              <Button variant="outline" type="submit"

                disabled={busy}
                onClick={() => {
                  setDisableOwn(detail);
                  setDetail(undefined);
                }}
              >
                禁用
              </Button>
            )}
            {detail.can_enable && (
              <Button variant="outline" type="submit"

                disabled={busy}
                onClick={() => void setDisabled(detail, false)}
              >
                启用
              </Button>
            )}
            {detail.can_revoke && (
              <Button variant="outline" type="submit"

                disabled={busy}
                onClick={() => {
                  setRevokeOwn(detail);
                  setDetail(undefined);
                }}
              >
                吊销
              </Button>
            )}
            {detail.can_delete && (
              <Button variant="destructive" type="submit"

                disabled={busy}
                onClick={() => {
                  setDeleteOwn(detail);
                  setDetail(undefined);
                }}
              >
                删除 Key
              </Button>
            )}
          </div>
          {!detail.can_copy && (
            <p className="info-note">{unavailable(detail)}</p>
          )}
          {canManage && !detail.archived && !detail.can_delete && (
            <div className="dialog-actions">
              {!detail.revoked && !detail.can_revoke && (
                <Button variant="outline" type="submit"

                  onClick={() => {
                    setDetail(undefined);
                    revoke(detail);
                  }}
                >
                  吊销
                </Button>
              )}
              <Button variant="destructive" type="submit"

                onClick={() => {
                  setDetail(undefined);
                  archive(detail);
                }}
              >
                删除并归档
              </Button>
            </div>
          )}
          {notice && <p role="status">{notice}</p>}
          {error && (
            <p role="alert" className="error">
              {error}
            </p>
          )}
        </WorkspaceDialog>
      )}
      {disableOwn && (
        <WorkspaceDialog title="禁用 Key" close={() => !busy && setDisableOwn(undefined)}>
          <p>禁用「{disableOwn.name}」后不能用它登录，已换发会话立即失效。密文仍保留，之后可以重新启用。</p>
          {error && <p role="alert" className="banner error">{error}</p>}
          <div className="dialog-actions">
            <Button variant="outline" type="submit"  disabled={busy} onClick={() => setDisableOwn(undefined)}>取消</Button>
            <Button variant="outline" type="submit"  disabled={busy} onClick={() => void setDisabled(disableOwn, true)}>
              {busy ? "禁用中…" : "确认禁用"}
            </Button>
          </div>
        </WorkspaceDialog>
      )}
      {revokeOwn && (
        <WorkspaceDialog title="吊销 Key" close={() => !busy && setRevokeOwn(undefined)}>
          <p>吊销「{revokeOwn.name}」后，该 Key 不能再登录，已换发的浏览器会话立即失效。</p>
          {error && <p role="alert" className="banner error">{error}</p>}
          <div className="dialog-actions">
            <Button variant="outline" type="submit"  disabled={busy} onClick={() => setRevokeOwn(undefined)}>取消</Button>
            <Button variant="destructive" type="submit"  disabled={busy} onClick={() => void confirmLifecycle(revokeOwn, false)}>
              {busy ? "吊销中…" : "确认吊销"}
            </Button>
          </div>
        </WorkspaceDialog>
      )}
      {deleteOwn && (
        <WorkspaceDialog title="删除 Key" close={() => !busy && setDeleteOwn(undefined)}>
          <p>删除「{deleteOwn.name}」会吊销该 Key、擦除可取回的密文，且不能再复制。此操作不可恢复。</p>
          {error && <p role="alert" className="banner error">{error}</p>}
          <div className="dialog-actions">
            <Button variant="outline" type="submit"  disabled={busy} onClick={() => setDeleteOwn(undefined)}>取消</Button>
            <Button variant="destructive" type="submit"  disabled={busy} onClick={() => void confirmLifecycle(deleteOwn, true)}>
              {busy ? "删除中…" : "确认删除"}
            </Button>
          </div>
        </WorkspaceDialog>
      )}
      {rotation && (
        <WorkspaceDialog
          title="轮转 Key"
          close={() => {
            if (!busy) setRotation(undefined);
          }}
        >
          <div className="key-rotation-summary">
            <span className="surface-icon">
              <RefreshCw />
            </span>
            <h3>{rotation.name}</h3>
            <p>生成新的 Key，旧 Key 立即失效。</p>
          </div>
          <ul className="key-impact-list">
            <li>使用旧 Key 的脚本、应用和其他登录会话需要更新凭据。</li>
            <li>当前操作会话保持登录；用户、团队权限和有效期不变。</li>
            <li>新 Key 可在此页面再次查看和复制。</li>
          </ul>
          {error && (
            <p role="alert" className="banner error">
              {error}
            </p>
          )}
          <div className="dialog-actions">
            <Button variant="outline" type="submit"

              disabled={busy}
              onClick={() => setRotation(undefined)}
            >
              取消
            </Button>
            <Button variant="default" type="submit"

              disabled={busy}
              onClick={() => void rotateKey()}
            >
              {busy ? "轮转中…" : "确认轮转"}
            </Button>
          </div>
        </WorkspaceDialog>
      )}
      {secret && (
        <WorkspaceDialog
          title="查看与复制 Key"
          close={() => {
            setSecret(undefined);
            setVisible(false);
            setError("");
          }}
        >
          <p className="info-note">
            {secret.name} · 仅本人可见。关闭后清空当前面板中的密钥。
          </p>
          <label className="key-secret-field">
            完整 Key
            <div>
              <Input
                aria-label="完整 Key"
                type={visible ? "text" : "password"}
                value={secret.key}
                readOnly
                autoComplete="off"
                spellCheck={false}
                onFocus={(e) => e.target.select()}
              />
              <Button variant="ghost" size="icon" type="submit"
                className="icon"
                aria-label={visible ? "隐藏 Key" : "显示 Key"}
                onClick={() => setVisible(!visible)}
              >
                {visible ? <EyeOff size={18} /> : <Eye size={18} />}
              </Button>
            </div>
          </label>
          {notice && (
            <p role="status" className="banner success">
              {notice}
            </p>
          )}
          {error && (
            <p role="alert" className="banner error">
              {error}
            </p>
          )}
          <div className="dialog-actions">
            <Button variant="outline" type="submit"

              onClick={() => {
                setSecret(undefined);
                setVisible(false);
              }}
            >
              关闭
            </Button>
            <Button variant="default" type="submit"

              onClick={() => void copyShownKey()}
            >
              <Copy size={15} />
              {copied === secret.id ? "再次复制 Key" : "复制 Key"}
            </Button>
          </div>
        </WorkspaceDialog>
      )}
      {edit && (
        <WorkspaceDialog title="修改 Key 名称" close={() => setEdit(undefined)}>
          <form
            onSubmit={async (e) => {
              e.preventDefault();
              setBusy(true);
              setError("");
              try {
                await api(`/key-directory/${edit.id}/label`, "POST", {
                  name: name.trim(),
                });
                await reload();
                setEdit(undefined);
                setNotice("Key 名称已更新，所属用户和权限保持不变。");
              } catch (e) {
                setError(errorText(e));
              } finally {
                setBusy(false);
              }
            }}
          >
            <label>
              Key 名称
              <Input
                autoFocus
                required
                maxLength={80}
                value={name}
                onChange={(e) => setName(e.target.value)}
                placeholder="例如：个人电脑 · 素材整理"
              />
            </label>
            <p>建议用设备或用途命名，便于区分。</p>
            {error && (
              <p role="alert" className="error">
                {error}
              </p>
            )}
            <div className="dialog-actions">
              <Button variant="outline"
                type="button"

                onClick={() => setEdit(undefined)}
              >
                取消
              </Button>
              <Button variant="default" type="submit"

                disabled={busy || !name.trim()}
              >
                {busy ? "保存中…" : "保存名称"}
              </Button>
            </div>
          </form>
        </WorkspaceDialog>
      )}
    </>
  );
}
