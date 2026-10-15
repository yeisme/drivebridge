import { DirectoryLayout, useDirectoryQuery } from "../../components/DirectoryLayout";
import { SearchField } from "../../components/SearchField";
import { SelectField } from "../../components/SelectField";
import { Button } from "../../components/ui/button";
import { ArrowRight, Cloud, Folder, HardDrive, UsersRound, Plus } from "lucide-react";
import { bytes, type Space, type Connection } from "../../api";
import { backendOf } from "../../storage";
import { CapacityBar } from "../../CapacityView";
import { capacityOf } from "../../capacity";
import type { GovernanceState } from "../../governance-context";

export function connectionObservation(connection?: Connection): string {
  if (!connection) return "连接状态未知";
  if (connection.disabled) return "已断开";
  return connection.status === "verified" ? "已验证" : "已配置，尚未验证";
}

export function filterSpaces(
  spaces: Space[],
  {
    group,
    storage,
    query,
    teams,
  }: {
    group: string;
    storage: string;
    query: string;
    teams?: GovernanceState["teams"];
  },
): Space[] {
  const needle = query.toLowerCase();
  return spaces.filter(
    (s) =>
      (group === "all" || (group === "personal" ? !s.team_id : !!s.team_id)) &&
      (!storage || s.connection_id === storage) &&
      `${s.name} ${teams?.find((t) => t.id === s.team_id)?.name || ""}`
        .toLowerCase()
        .includes(needle),
  );
}

export function DriveHome({
  spaces,
  connections,
  governance,
  open,
  connect,
}: {
  spaces: Space[];
  connections: Connection[];
  governance?: GovernanceState;
  open: (s: Space) => void;
  connect?: () => void;
}) {
  const [query, setQuery] = useDirectoryQuery("q");
  const [group, setGroup] = useDirectoryQuery("group", "all");
  const [storage, setStorage] = useDirectoryQuery("storage");
  const entries = filterSpaces(spaces, {
    group,
    storage,
    query,
    teams: governance?.teams,
  });
  return (
    <div className="drive-home">
      <div className="workspace-toolbar">
        <div className="segmented" aria-label="网盘类型">
          {[
            ["all", "全部网盘"],
            ["personal", "个人网盘"],
            ["team", "团队网盘"],
          ].map(([id, label]) => (
            <Button layout="surface" variant="ghost" type="button"
              key={id}
              aria-pressed={group === id}
              onClick={() => setGroup(id)}
            >
              {label}
            </Button>
          ))}
        </div>
        <SearchField
            aria-label="搜索网盘"
            placeholder="搜索网盘或团队…"
            value={query}
            onChange={(e) => setQuery(e.target.value)}
          />
        <SelectField aria-label="筛选存储" value={storage} onValueChange={setStorage} options={[{value:"",label:"所有存储"}, ...connections.map(c => ({value:c.id,label:c.name}))]} />
      </div>
      <div className="section-caption">
        <span>{entries.length} 个可访问网盘</span>
        <span>选择网盘，开始整理文件</span>
      </div>
      <DirectoryLayout label="网盘" items={entries.map(s => {
        const connection=connections.find(c=>c.id===s.connection_id), capacity=capacityOf(s);
        const owner=governance?.teams.find(t=>t.id===s.team_id)?.name || (s.team_id ? "团队空间" : "我的个人空间");
        const role=governance?.space_roles[s.id];
        return {id:s.id,title:s.name,subtitle:owner,icon:s.team_id ? <UsersRound size={24}/> : <HardDrive size={24}/>,openLabel:`打开网盘 ${s.name}`,open:()=>open(s),
          meta:<>{capacity.occupied === null ? "占用未知" : bytes(capacity.occupied)} / {capacity.quota === null ? "配额未提供" : bytes(capacity.quota)}</>,
          detail:<><CapacityBar space={s}/><dl className="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-3 text-sm"><dt>存储</dt><dd>{connection?.name || backendOf(connection?.kind || s.backend).label}</dd><dt>连接</dt><dd>{connectionObservation(connection)}</dd><dt>已占用</dt><dd>{capacity.occupied === null ? "未知" : bytes(capacity.occupied)}</dd><dt>预留</dt><dd>{capacity.reserved === null ? "未知" : bytes(capacity.reserved)}</dd><dt>配额</dt><dd>{capacity.quota === null ? "未提供" : bytes(capacity.quota)}</dd><dt>访问</dt><dd>{role ? ({admin:governance?.admin_direct_write ? "管理员 · 可读写" : "管理只读",custodian:"数据负责人",editor:"可编辑",viewer:"只读",owner:"本人空间"}[role] || role) : s.team_id ? "按已获授权访问" : "本人空间"}</dd></dl></>};
      })} cards={
<div className="drive-cards">
        {entries.map((s) => {
          const c = connections.find((c) => c.id === s.connection_id),
            team = governance?.teams.find((t) => t.id === s.team_id),
            capacity = capacityOf(s),
            reserved = capacity.reserved;
          return (
            <Button layout="surface" variant="ghost" type="button"
              key={s.id}
              className="drive-card"
              onClick={() => open(s)}
              aria-label={`打开网盘 ${s.name}`}
            >
              <div className="drive-card-top">
                <span className="surface-icon">
                  {s.team_id ? (
                    <UsersRound size={23} />
                  ) : (
                    <HardDrive size={23} />
                  )}
                </span>
                <span className="pill">{s.team_id ? "团队" : "个人"}</span>
              </div>
              <h2>{s.name}</h2>
              <p>{team?.name || (s.team_id ? "团队空间" : "我的个人空间")}</p>
              <div className="drive-storage">
                <Cloud size={14} />
                <span>{c?.name || backendOf(c?.kind || s.backend).label}</span>
                <span className="text-xs text-muted-foreground">{connectionObservation(c)}</span>
              </div>
              <CapacityBar space={s} />
              <div className="drive-capacity">
                <span>{capacity.occupied === null ? "占用未知" : bytes(capacity.occupied) + " 已占用"}</span>
                <span>
                  {capacity.quota ? `${bytes(capacity.quota)} 空间配额` : "配额未提供"}
                </span>
              </div>
              <div className="drive-card-footer">
                <span>
                  {c?.disabled
                    ? "存储已断开"
                    : reserved
                      ? `${bytes(reserved)} 传输预留`
                      : "进入文件工作区"}
                </span>
                <ArrowRight size={16} />
              </div>
            </Button>
          );
        })}
      </div>      }/>
      {!entries.length && (
        <div className="workspace-empty">
          <Folder size={36} />
          <h3>
            {query || storage
              ? "没有匹配的网盘"
              : group === "team"
                ? "还没有可访问的团队网盘"
                : "还没有网盘"}
          </h3>
          <p>
            {group === "team"
              ? "团队成员加入后，由负责人分配空间权限。"
              : "连接存储并设置空间后，即可在这里进入。"}
          </p>
          {connect && group !== "team" && (
            <Button variant="outline" type="submit"  onClick={connect}>
              <Plus size={16} />
              连接存储
            </Button>
          )}
        </div>
      )}
    </div>
  );
}

