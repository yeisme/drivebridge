import { loadSession, verifySessionIdentity } from "./app/session";
import { AuthGate } from "./app/AuthGate";
import { WorkspaceShell, WorkspaceMain } from "./app/WorkspaceShell";
import { WorkspaceViews } from "./app/WorkspaceViews";
import { MobileNavigation } from "./app/MobileNavigation";
import { DesktopNavigation } from "./app/DesktopNavigation";
import { SidebarFooter } from "./app/SidebarFooter";
import { RouteBoundary } from "./app/RouteBoundary";
import { CommandSearch } from "./app/CommandSearch";
import { MobileNavigationDrawer } from "./app/MobileNavigationDrawer";
import { useMobileViewport } from "./app/useMobileViewport";
import { useSidebarPreference } from "./app/useSidebarPreference";
import { useNavGroups } from "./app/useNavGroups";
import { historyScope } from "./app/history-scope";
import { latestRequest } from "./app/latest-request";
import { poll } from "./app/poll";
import { taskPollDelay } from "./app/task-poll-delay";
import { useRouteLeaveGuard } from "./app/route-leave";
import { readOperationProjection } from "./app/operation-projection";
import { isTypingTarget } from "./finder-keys";
import "./app/mobile-shell.css";
import { PreviewSection } from "./components/PreviewSection";
import { Textarea } from "./components/ui/textarea";
import { Checkbox } from "./components/ui/checkbox";
import { Input } from "./components/ui/input";
import { Button } from "./components/ui/button";
import { SelectAdapter } from "./components/SelectAdapter";
import { AppDialog as Modal } from "./components/AppDialog";
import { ThemeToggle } from "./components/ThemeToggle";
import { PersonalSettings } from "./PersonalSettings";
import { teamURL } from "./app/team-route";
import { workspaceRoute, workspacePageURL, governanceURL } from "./workspace-route";
import { DashboardOverview, IdentityDirectory } from "./Workspace";

import { GovernanceBar, useGovernance, governanceMode } from "./governance-context";
import { getDataContext } from "./api";
import { FilesPage } from "./features/files/FilesPage";
import { FilePreviewDialog } from "./features/files/FilePreviewDialog";
import { TrashPage } from "./features/trash/TrashPage";
import { restorableSelection } from "./features/trash/selection";
import { MediaPreview } from "./Media";
import { canOpenMarkdownNotebook } from "./features/notebook/notebook-capability";
import { PreviewFiles } from "./PreviewFiles";
import { RobotSetup } from "./RobotSetup";
import { CapacityPanel } from "./CapacityView";
import { ShareDialog, SharesPage } from "./Sharing";
import { InsightsPanel } from "./insights/Insights";
import { InfoWorkspace } from "./info-workspace/InfoWorkspace";
import { FlowPage } from "./consumer-flows/FlowPage";
import { copyReference, pinnedFile, authMode, keyLogin, demoLogin } from "./api";
import { LegacyAgentPage } from "./features/agents/LegacyAgentPage";
import { GrantOverview } from "./features/agents/GrantOverview";
import { ConnectionsPage } from "./features/storage/ConnectionsPage";
import { OperationTasks } from "./features/tasks/OperationTasks";
import { WorkspaceHome } from "./features/home/WorkspaceHome";
import { LegacyTransfersPage } from "./features/tasks/LegacyTransfersPage";
import { LegacyInboxPage } from "./features/tasks/LegacyInboxPage";
import { SubmissionInbox } from "./features/inbox/SubmissionInbox";
import { LabelSuggestionEntry } from "./features/files/LabelSuggestions";
import { FilePropertiesEntry } from "./features/properties/FileProperties";
import type { AuthMode, IssuedUserKey, UserKeyRow, UserIdentity } from "./api";
import React, { lazy, Suspense, useCallback, useEffect, useReducer, useRef, useState } from "react";

const CapabilitiesPage = lazy(() => import("./features/capabilities/CapabilitiesPage").then(module => ({ default: module.CapabilitiesPage })));
const DevicesPage = lazy(() => import("./features/devices/DevicesPage").then(module => ({ default: module.DevicesPage })));
const AutomationPage = lazy(() => import("./features/automation/AutomationPage").then(module => ({ default: module.AutomationPage })));
const PropertiesPage = lazy(() => import("./features/properties/PropertiesPage").then(module => ({ default: module.PropertiesPage })));
const CollectionsPage = lazy(() => import("./features/collections/CollectionsPage").then(module => ({ default: module.CollectionsPage })));
const ResourcesPage = lazy(() => import("./features/resources/page").then(module => ({ default: module.ResourcesPage })));
const KnowledgePage = lazy(() => import("./features/notebook/page").then(module => ({ default: module.KnowledgePage })));
const MarkdownNotebookPreview = lazy(() => import("./features/notebook/MarkdownNotebookPreview").then(module => ({ default: module.MarkdownNotebookPreview })));
const GovernancePanel = lazy(() => import("./Governance").then(module => ({ default: module.GovernancePanel })));
const TeamWorkspace = lazy(() => import("./Teams").then(module => ({ default: module.TeamWorkspace })));
const KeyDirectory = lazy(() => import("./KeyDirectory").then(module => ({ default: module.KeyDirectory })));
import { navigationReducer, type Page } from "./app/navigation-state";
import {
  PanelLeft,
  Folder,
  Cloud,
  ArrowUpRight,
  ArrowDownToLine,
  Search,
  Plus,
  Trash2,
  Check,
  LoaderCircle,
  X,
  ChevronRight,
  ShieldCheck,
  KeyRound,
  ArrowLeft,
  ArrowUpDown,
  Plug,
  Copy,
  Inbox,
  UsersRound,
  Users,
  Share2,
  Gauge,
  LayoutDashboard,
  HardDrive,
  Bot,
  FileText,
  Settings,
  Workflow,
  Monitor,
  CircleHelp,
  Library,
  Tags,
  Folders,
} from "lucide-react";
import { BrandMark } from "./BrandMark";
import {
  api,
  upload,
  download,
  logout,
  bytes,
  errorText,
} from "./api";
import { backendOf, capabilityText } from "./storage";
import {
  pendingNavigationTasks,
  visibleNavGroups,
} from "./nav-groups";
import type {
  Space,
  DriveFile,
  Operation,
  Connection,
  Agent,
  Version,
  Audit,
} from "./api";
// Keep compatibility CSS before the shell overrides when page JS is lazy.
import "./governance.css";
import "./teams.css";
import "./key-directory.css";
import "./app/workspace-shell.css";
const navIcon = { size: 20, strokeWidth: 1.75 } as const;
const navigation: [Page, string, React.ReactNode][] = [
  ["overview", "我的工作台", <LayoutDashboard {...navIcon} />],
  ["insights", "资源态势", <Gauge {...navIcon} />],
  ["files", "我的网盘", <HardDrive {...navIcon} />],
  ["knowledge", "知识库", <FileText {...navIcon} />],
  ["resources", "资源库", <Library {...navIcon} />],
  ["collections", "项目集合", <Folders {...navIcon} />],
  ["properties", "标签与属性", <Tags {...navIcon} />],
  ["search", "搜索", <Search {...navIcon} />],
  ["flows", "文件流程", <Workflow {...navIcon} />],
  ["transfers", "任务中心", <ArrowUpDown {...navIcon} />],
  ["shares", "分享交付", <Share2 {...navIcon} />],
  ["teams", "团队", <Users {...navIcon} />],
  ["trash", "回收站", <Trash2 {...navIcon} />],
  ["connections", "存储管理", <Plug {...navIcon} />],
  ["devices", "设备与离线", <Monitor {...navIcon} />],
  ["automation", "备份与整理", <Workflow {...navIcon} />],
  ["capabilities", "能力与连接说明", <CircleHelp {...navIcon} />],
  ["inbox", "产物与收件", <Inbox {...navIcon} />],
  ["agents", "Agent 管理", <Bot {...navIcon} />],
  ["members", "访问凭据", <KeyRound {...navIcon} />],
  ["governance", "管理与授权", <ShieldCheck {...navIcon} />],
  ["settings", "个人设置", <Settings {...navIcon} />],
];
function App() {
  const mobile = useMobileViewport();
  const [mobileNavOpen, setMobileNavOpen] = useState(false);
  const [commandSearchOpen, setCommandSearchOpen] = useState(false);
  const searchReturnFocus = useRef<HTMLElement | null>(null);
  const openCommandSearch = () => { searchReturnFocus.current = document.activeElement as HTMLElement; setCommandSearchOpen(true); };
  const closeCommandSearch = () => {
    setCommandSearchOpen(false);
    requestAnimationFrame(() => {
      // Base UI restores focus on unmount. Only fill a missing restoration; never
      // steal focus from a control the user selected after the dialog closed.
      if (document.activeElement === document.body && searchReturnFocus.current?.isConnected) searchReturnFocus.current.focus();
    });
  };
  useEffect(() => { if (!mobile) setMobileNavOpen(false); }, [mobile]);
  const [navigationState, dispatchNavigation] = useReducer(navigationReducer, undefined, () => {
    const route = workspaceRoute(location.hash);
    return { page: navigation.find(n => n[0] === route.page)?.[0] || (new URLSearchParams(location.search).has("insight_from") ? "insights" : "files"), tab: route.tab };
  });
  const page = navigationState.page;
  const shellTab = navigationState.tab;
  const setPage = (page: Page) => dispatchNavigation({ type: "page", page });
  const [user, setUser] = useState<UserIdentity | null>(null),
    [instance, setInstance] = useState(""),
    [admin, setAdmin] = useState(false),
    [governanceEnabled, setGovernanceEnabled] = useState(false),
    [loginMode, setLoginMode] = useState<AuthMode | null>(null),
    [keyInput, setKeyInput] = useState(""),
    [keyBusy, setKeyBusy] = useState(false),
    [userKeys, setUserKeys] = useState<UserKeyRow[]>([]),
    [issuedKeys, setIssuedKeys] = useState<IssuedUserKey[]>([]),
    [issueNames, setIssueNames] = useState(""),
    [issueDays, setIssueDays] = useState(""),
    [issueAdmin, setIssueAdmin] = useState(false),
    [revokeTarget, setRevokeTarget] = useState<UserKeyRow | null>(null),
    [loading, setLoading] = useState(true),
    [spaces, setSpaces] = useState<Space[]>([]),
    [space, setSpace] = useState(workspaceRoute(location.hash).space || getDataContext().space),
    [files, setFiles] = useState<DriveFile[]>([]),
    [fileRead, setFileRead] = useState<{ space: string; page: string; state: 'loading' | 'ready' | 'error' | 'unavailable' }>({ space: '', page: '', state: 'loading' }),
    [ops, setOps] = useState<Operation[]>([]),
    [inboxOps, setInboxOps] = useState<Operation[]>([]),
    [taskAccessDenied, setTaskAccessDenied] = useState(false),
    [connections, setConnections] = useState<Connection[]>([]),
    [agents, setAgents] = useState<Agent[]>([]),
    [audits, setAudits] = useState<Audit[]>([]),
    [search, setSearch] = useState(""),
    [picked, setPicked] = useState<Record<string, boolean>>({}),
    [prefix, setPrefix] = useState(""),
    [grid, setGrid] = useState(false),
    [error, setError] = useState(""),
    [notice, setNotice] = useState(""),
    [busy, setBusy] = useState(false),
    [modal, setModal] = useState(""),
    [selected, setSelected] = useState<DriveFile | null>(null),
    [versions, setVersions] = useState<Version[]>([]),
    [token, setToken] = useState(""),
    [resume, setResume] = useState<Operation | undefined>(),
    [newVersion, setNewVersion] = useState<DriveFile | undefined>(),
    [driveItems, setDriveItems] = useState<any[]>([]),
    [driveConnection, setDriveConnection] = useState("");
  useEffect(() => {if(!notice)return;const timer=setTimeout(()=>setNotice(""),5000);return()=>clearTimeout(timer);},[notice]);
  useEffect(() => {if(!error)return;const timer=setTimeout(()=>setError(""),12000);return()=>clearTimeout(timer);},[error]);
  useEffect(() => {setError("");setNotice("");},[page,space]);
  const [demoPersona,setDemoPersona] = useState("admin");
  const [routeHash, setRouteHash] = useState(() => location.hash);
  const [knowledgeDirty, setKnowledgeDirty] = useState(false);
  const [propertyDirty, setPropertyDirty] = useState(false);
  const onPropertyDirty = useCallback((dirty: boolean) => { setPropertyDirty(dirty); }, []);
  useRouteLeaveGuard(knowledgeDirty, "放弃未保存的笔记草稿并离开知识库？");
  useRouteLeaveGuard(propertyDirty, "放弃未保存的文件属性草稿并离开？");
  const driveFocused = page === "files" && new URLSearchParams(routeHash.split("?")[1] || "").get("view") === "drive";
  const teamDetail = page === "teams" && /^#\/teams\/[^/?]+/.test(routeHash);
  useEffect(() => {const change=()=>{setRouteHash(location.hash);const route=workspaceRoute(location.hash);const next=navigation.find(n=>n[0]===route.page);if(next){dispatchNavigation({type:"route",page:next[0],tab:route.tab});setDriveOpen(!!route.space);if(route.space)setSpace(route.space);setPrefix("");setSelected(null);setShareSelection(undefined);}};window.addEventListener("hashchange",change);return()=>window.removeEventListener("hashchange",change)},[]);
  const [driveOpen, setDriveOpen] = useState(!!workspaceRoute(location.hash).space || !!getDataContext().space);
  useEffect(() => {
    if (!user) return;
    const keydown = (event: KeyboardEvent) => {
      if (event.defaultPrevented || event.isComposing || event.altKey || isTypingTarget(event.target) || window.getSelection()?.toString()) return;
      if (!(event.ctrlKey || event.metaKey) || event.key.toLowerCase() !== "k") return;
      if (document.querySelector('[role="dialog"]')) return;
      event.preventDefault(); openCommandSearch();
    };
    window.addEventListener("keydown", keydown);
    return () => window.removeEventListener("keydown", keydown);
  }, [user]);
  useEffect(() => { setCommandSearchOpen(false); }, [page, space, user?.id, instance]);
  const [sidebarCollapsed, toggleSidebar, resetSidebar, sidebarExplicit] = useSidebarPreference(instance, user?.id || "");
  const [navOpen, setNavOpen] = useNavGroups(instance, user?.id || "");
  useEffect(() => {
    const listener = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.isComposing || e.altKey || isTypingTarget(e.target) || window.getSelection()?.toString()) return;
      if (!(e.metaKey || e.ctrlKey) || e.key.toLowerCase() !== "b") return;
      e.preventDefault();
      if (mobile) setMobileNavOpen(value => !value);
      else toggleSidebar();
    };
    window.addEventListener("keydown", listener);
    return () => window.removeEventListener("keydown", listener);
  }, [mobile, toggleSidebar]);
  const [shareSelection, setShareSelection] = useState<{files: DriveFile[]; path: string}>();
  const [previewSiblings, setPreviewSiblings] = useState<DriveFile[]>([]);
  const [notebookPreview, setNotebookPreview] = useState(false);
  useEffect(() => { setNotebookPreview(false); }, [selected?.id, selected?.version_id]);
  const [usageOpen, setUsageOpen] = useState(false);
  const [usageSpace, setUsageSpace] = useState("");
  const [setupOpen, setSetupOpen] = useState(false);
  const [archived, setArchived] = useState(false);
  const [cancelOp, setCancelOp] = useState("");
  const [agentQuery, setAgentQuery] = useState("");
  const [agentFilter, setAgentFilter] = useState("all");
  const [archiveTarget, setArchiveTarget] = useState<{kind: "users" | "agents"; id: string; name: string}>();
  const input = useRef<HTMLInputElement>(null),
    controllers = useRef(new Map<string, AbortController>());
  const fileRequests = useRef(latestRequest());
  const view = useRef({ space, page });
  view.current = { space, page };
  const governance = useGovernance(user?.id);
  const [inviteCount, setInviteCount] = useState(0);
  useEffect(() => {
    if (!user || !governanceEnabled) return;
    const load = () => {
      void api<{ items: { status?: string }[] }>("/governance/team-invitations")
        .then((data) =>
          setInviteCount(
            (data.items || []).filter((i) => i.status && i.status !== "expired")
              .length,
          ),
        )
        .catch(() => setInviteCount(0));
    };
    load();
    window.addEventListener("drivebridge-governance-changed", load);
    return () =>
      window.removeEventListener("drivebridge-governance-changed", load);
  }, [user, governanceEnabled, governance?.tenant_id]);
  const accessMode = governanceMode(governance);
  const readOnly = governanceEnabled && (!governance || ["admin", "viewer"].includes(accessMode));
  const managementRestricted = readOnly || accessMode === "granted";
  useEffect(() => () => { for (const c of controllers.current.values()) c.abort(); }, []);
  const currentSpace = spaces.find((s) => s.id === space);
  const meterSpace = currentSpace || spaces[0];
  const requestedInsight = new URLSearchParams(routeHash.split("?")[1] || "").get("view");
  const insightView = requestedInsight && ["overview", "capacity", "transfers", "robots", "audit"].includes(requestedInsight) ? requestedInsight : (shellTab === "usage" ? "capacity" : "overview");
  const insightSpace = new URLSearchParams(routeHash.split("?")[1] || "").get("space") || "";
  useEffect(() => { setUsageOpen(false);  setSetupOpen(false); }, [user?.id, governance?.tenant_id, space]);
  const openUsage = (id?: string) => { setUsageSpace(id || meterSpace?.id || ""); setUsageOpen(true); };
  const openInsight = (view: string, id = new URLSearchParams(location.hash.split("?")[1] || "").get("space") || "") => { const q = new URLSearchParams(location.hash.split("?")[1] || ""); q.set("view", view); if (id) q.set("space", id); else q.delete("space"); location.hash = "/insights?" + q; };
  const canManageDirectory = admin || !!governance?.tenant_admin;

  const availableSpaces = [...new Map([...(governance?.spaces || []), ...spaces].map(s => [s.id, s])).values()];
  useEffect(() => {
    if (!loading && governance && driveOpen && !currentSpace && !getDataContext().space) {
      const accessible = governance.spaces.find(s => s.id === space);
      if (accessible && governance.space_roles[space]) openDrive(accessible);
    }
  }, [loading, governance, driveOpen, currentSpace, space]);
  function openDrive(next: Space) {
    const tenant = next.tenant_id || governance?.tenant_id || getDataContext().tenant;
    const query = new URLSearchParams();if(tenant)query.set("tenant",tenant);if(driveFocused)query.set("view","drive");if(next.team_id)query.set("team",next.team_id);if(governance?.space_roles[next.id] && governance.space_roles[next.id] !== "owner")query.set("scope","managed");
    location.hash = "#/drive/" + encodeURIComponent(next.id) + "?" + query;
    // Route admission owns page/scope changes; do not unmount drafts here.
  }
  function driveHome() {
    location.hash = workspacePageURL("files", getDataContext().tenant);
  }
  function navigatePage(id:Page) {
    if(id==="governance"){const c=getDataContext();location.hash=governanceURL("requests",c.tenant,c.space);return;}
    if(id==="files"){driveHome();return;}
    const tenant = getDataContext().tenant?"?tenant="+encodeURIComponent(getDataContext().tenant):"";
    if(id==="analytics"){location.hash="#/analytics"+tenant;return;}
    if(id==="users"){location.hash="#/users"+tenant;return;}
    if(id==="insights"){location.hash="#/insights"+tenant;return;}
    if(id==="members"){
      if(!canManageDirectory){location.hash="#/users"+tenant;return;}
      location.hash="#/members"+tenant;return;
    }
    location.hash=workspacePageURL(id, getDataContext().tenant);
  }
  function navigateLegacyPage(id: "transfers" | "inbox") {
    const url = workspacePageURL(id, getDataContext().tenant);
    location.hash = url + (url.includes("?") ? "&" : "?") + "view=legacy";
  }
  async function reloadKeys() {const out = await api<{keys:UserKeyRow[]}>("/key-directory"); setUserKeys(out.keys || []);}
  // 统一存储标识：空间绑定的连接类型决定徽标；未绑定/未识别时诚实显示。
  const currentBackend = backendOf(
    connections.find((c) => c.id === currentSpace?.connection_id)?.kind,
  );
  async function act(fn: () => Promise<void>) {
    setError("");
    setBusy(true);
    try {
      await fn();
    } catch (e) {
      setError(errorText(e));
    } finally {
      setBusy(false);
    }
  }
  async function reload() {
    const [sp, op, inbox, co, ag, au] = await Promise.all([
      api<Space[]>("/spaces"),
      api<Operation[]>("/operations"),
      api<Operation[]>("/inbox"),
      api<Connection[]>("/connections"),
      api<Agent[]>("/agents"),
      api<Audit[]>("/audit"),
    ]);
    setSpaces(sp);
    if (!space && sp[0]) setSpace(sp[0].id);
    setOps(op);
    setInboxOps(inbox);
    setTaskAccessDenied(false);
    setConnections(co);
    setAgents(ag);
    setAudits(au);
  }
  useEffect(()=>{if(!user)return;const changed=()=>{void reload().then(()=>setError("")).catch(e=>{if(e.name!=="AbortError")setError(errorText(e))})};window.addEventListener("drivebridge-governance-changed",changed);return()=>window.removeEventListener("drivebridge-governance-changed",changed)},[user,space]);
  async function refreshFiles() {
    const request = fileRequests.current.start();
    if (!["files", "trash", "insights"].includes(page) || !space || !spaces.some(s => s.id === space)) { setFiles([]); setFileRead({ space, page, state: 'unavailable' }); return; }
    const key = { space, page };
    setFileRead({ ...key, state: 'loading' });
    if (page === 'trash') { setFiles([]); setPicked({}); setSelected(null); setVersions([]); }
    const current = () => request.isCurrent() && view.current.space === key.space && view.current.page === key.page;
    const result = await api<DriveFile[]>(
      "/files?space_id=" +
        encodeURIComponent(space) +
        "&trash=" +
        (page === "trash"),
      "GET", undefined, {}, request.signal,
    ).catch(e => {
      if (current()) {
        setFileRead({ ...key, state: 'error' });
        if (page === 'trash') { setFiles([]); setPicked({}); setSelected(null); setVersions([]); }
        throw e;
      }
      return [] as DriveFile[];
    });
    if (current()) { setFiles(result); setFileRead({ ...key, state: 'ready' }); }
  }
  useEffect(() => {
    const controller = new AbortController();
    loadSession(controller.signal)
      .then(async (me) => {
        if (controller.signal.aborted) return;
        setUser(me.user);
        setInstance(me.instance_id || "");
        setAdmin(!!me.admin);
        setGovernanceEnabled(!!me.governance_enabled);
        await reload();
      })
      .catch((e) => {
        if (!controller.signal.aborted && e.name !== "AbortError" && e.message !== "unauthorized") setError(errorText(e));
      })
      .finally(() => { if (!controller.signal.aborted) setLoading(false); });
    authMode()
      .then(mode=>{if (controller.signal.aborted) return; setLoginMode(mode);setDemoPersona(mode.current_persona||"admin")})
      .catch(() => {});
    return () => controller.abort();
  }, []);
  useEffect(() => {
    if (user) refreshFiles().catch((e) => { if (e.name !== "AbortError") setError(errorText(e)); });
    return () => fileRequests.current.cancel();
  }, [space, page, user, spaces]);
  useEffect(() => {
    if (!user || !["users", "members"].includes(page)) return;
    reloadKeys().catch(e => setError(errorText(e)));
  }, [user, page]);
  const taskDelay = useRef(30000);
  const observedTaskDelay = taskPollDelay([...ops, ...inboxOps]);
  taskDelay.current = observedTaskDelay;
  useEffect(() => {
    if (!user || taskAccessDenied || page === "insights") return;
    return poll(
      readOperationProjection,
      ([nextOps, nextInbox]) => {
        taskDelay.current = taskPollDelay([...nextOps, ...nextInbox]);
        setOps(nextOps); setInboxOps(nextInbox); setTaskAccessDenied(false);
      },
      () => taskDelay.current,
      () => { setOps([]); setInboxOps([]); setTaskAccessDenied(true); },
      signal => verifySessionIdentity(user.id, instance, signal),
    );
  }, [user, instance, page, taskAccessDenied, observedTaskDelay]);
  useEffect(() => {
    if (page !== "trash") return;
    const onKey = (e: KeyboardEvent) => {
      if (!selected || modal || /INPUT|TEXTAREA|SELECT/.test((e.target as HTMLElement).tagName))
        return;
      if (e.shiftKey && (e.key === "Delete" || e.key === "Backspace")) {
        e.preventDefault();
        setNotice("当前工作区未启用永久删除，请使用恢复操作。");
      }
    };
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [page, selected, modal]);
  useEffect(() => {
    if (!selected) return;
    const controller = new AbortController();
    setVersions([]);
    api<Version[]>("/files/" + selected.id + "/versions", "GET", undefined, {}, controller.signal).then(next => { if (!controller.signal.aborted) setVersions(next); }).catch(e => { if (!controller.signal.aborted) setError(errorText(e)); });
    const keyboard = (e: KeyboardEvent) => {
      const target = e.target as HTMLElement;
      const dialog = target.closest('[role="dialog"]');
      if (modal || e.defaultPrevented || e.isComposing || (dialog && !dialog.classList.contains("quicklook-dialog")) || target.closest('.spreadsheet-reader, [role=listbox], [role=option], [role=menu], [role=tablist], [aria-label="笔记工作台"], [contenteditable="true"]') || /INPUT|TEXTAREA|SELECT|VIDEO|AUDIO/.test(target.tagName)) return;
      if (["ArrowLeft", "ArrowRight", "ArrowUp", "ArrowDown"].includes(e.key)) {
        const current = previewSiblings.findIndex(f => f.id === selected.id);
        const next = previewSiblings[current + (["ArrowRight", "ArrowDown"].includes(e.key) ? 1 : -1)];
        if (current >= 0 && next) { e.preventDefault(); setSelected(next); }
      }
    };
    // Capture phase: the Base UI dialog popup stops propagation of arrow keys
    // (COMPOSITE_KEYS), which would otherwise starve this window-level listener.
    window.addEventListener("keydown", keyboard, true);
    return () => { controller.abort(); window.removeEventListener("keydown", keyboard, true); };
  }, [selected, modal, previewSiblings]);
  async function openOperationResult(op: Operation) {
    const revision=getDataContext().revision;
    const headers: Record<string,string>=governanceEnabled && op.space_id?{"X-DriveBridge-Space":op.space_id}:{};
    const controller=new AbortController();const timer=setTimeout(()=>controller.abort(),15000);
    try {
      const result=await api<{file:DriveFile}>("/files/"+op.file_id,"GET",undefined,headers,controller.signal);
      const pinned=await pinnedFile(result.file,op.version_id,headers,controller.signal);
      if(revision!==getDataContext().revision)return;
      const file={...pinned.file,tenant_context:getDataContext().tenant||governance?.tenant_id};
      setModal("");setPreviewSiblings([file]);setSelected(file);
    } catch(e) {if(controller.signal.aborted)throw new Error("preview_timeout");throw e;} finally {clearTimeout(timer);}
  }
  async function uploadFiles(list: File[], target: { resume?: Operation; version?: DriveFile } = {}) {
    if (readOnly) { setError("当前为只读访问，请在管理与授权中心申请修改。"); return; }
    if (currentSpace && !currentSpace.connection_id) {
      setError("请先在存储连接页为个人空间绑定存储");
      return;
    }
    const uploadContextRevision = getDataContext().revision;
    for (const file of list) {
      if (getDataContext().revision !== uploadContextRevision) return;
      const controller = new AbortController();
      // Local queue identity also needs to work on explicitly supported LAN HTTP.
      const pendingID = `pending:${Array.from(crypto.getRandomValues(new Uint8Array(24)), byte => byte.toString(16).padStart(2, "0")).join("")}`;
      controllers.current.set(pendingID, controller);
      let id = "";
      try {
        await upload(
          file,
          space,
          target.resume?.path ||
            target.version?.path ||
            (prefix ? prefix + "/" : "") + file.name,
          (op) => {
            id = op.id;
            controllers.current.set(id, controller);
            setOps((old) => [op, ...old.filter((x) => x.id !== op.id)]);
          },
          controller.signal,
          target.resume,
          target.version,
        );
        setNotice("文件已保存并完成校验");
      } catch (e) {
        if ((e as Error).name === "AbortError") return;
        setError(errorText(e));
      } finally {
        controllers.current.delete(pendingID);
        if (id) controllers.current.delete(id);
      }
    }
    if (getDataContext().revision !== uploadContextRevision) return;
    await refreshFiles();
    await reload();
  }
  useEffect(() => { setPicked({}); }, [space, page, user?.id, instance]);
  const filtered = files.filter(
    (f) =>
      f.space_id === space &&
      f.path.startsWith(prefix ? prefix + "/" : "") &&
      f.path.toLowerCase().includes(search.toLowerCase()),
  );
  const folders = [
    ...new Set(
      filtered
        .map((f) => f.path.slice(prefix ? prefix.length + 1 : 0))
        .filter((p) => p.includes("/"))
        .map((p) => p.split("/")[0]),
    ),
  ];
  const visible = search
    ? filtered
    : filtered.filter(
        (f) => !f.path.slice(prefix ? prefix.length + 1 : 0).includes("/"),
      );
  const title = navigation.find((n) => n[0] === page)?.[1] || "";
  const allowedNav = navigation
    .filter(([id]) => (id !== "governance" || governanceEnabled) && (!getDataContext().space || !["connections","agents"].includes(id)) && (canManageDirectory || id !== "connections"))
    .map(([id]) => id);
  const navGroups = visibleNavGroups(allowedNav, navOpen);
  const pendingTaskCount = pendingNavigationTasks(ops);
  if (loading || !user) return <AuthGate
    loading={loading} mode={loginMode} busy={keyBusy} value={keyInput} error={error}
    onChange={setKeyInput}
    onDemoLogin={async () => {
      setKeyBusy(true); setError("");
      try { await demoLogin(); window.location.reload(); }
      catch (e) { setError(errorText(e)); setKeyBusy(false); }
    }}
    onKeyLogin={() => {
      setKeyBusy(true); setError("");
      keyLogin(keyInput.trim()).then(async () => {
        setKeyInput("");
        const me = await loadSession();
        setUser(me.user); setInstance(me.instance_id || ""); setAdmin(!!me.admin); setGovernanceEnabled(!!me.governance_enabled);
        await reload();
      }).catch(e => setError(errorText(e))).finally(() => setKeyBusy(false));
    }}
  />;
  return (
    <WorkspaceShell mobile={mobile} collapsed={sidebarCollapsed} focused={driveFocused}>
      {!mobile && <aside className="sidebar db-pine-nav db-docked-navigation shrink-0 overflow-hidden border-0 p-0 group-data-[collapsed=true]/shell:p-0">
        <a href="#" className="brand db-nav-brand flex h-16 shrink-0 items-center gap-2 px-3 text-[15px] tracking-normal group-data-[collapsed=true]/shell:justify-center group-data-[collapsed=true]/shell:px-0" onClick={driveHome} aria-label="DriveBridge">
          <span className="brandmark">
            <BrandMark size={32} />
          </span>
          <span className="db-nav-identity"><strong className="brand-name">DriveBridge</strong><span className="brand-subtitle">个人空间</span></span>
        </a>
        <DesktopNavigation groups={navGroups} items={navigation} page={page} title={title}
          pendingCount={pendingTaskCount} inviteCount={inviteCount}
          onToggle={(id, open) => setNavOpen({ ...navOpen, [id]: open })}
          onNavigate={target => { navigatePage(target); setPrefix(""); setSearch(""); }}
        />
        {sidebarExplicit && <Button layout="surface" type="button" className="mx-2 mb-2 rounded-md p-2 text-xs text-sidebar-foreground hover:bg-[var(--navigation-hover)]" aria-label="恢复自动导航布局" title="恢复自动导航布局" onClick={resetSidebar}><span className="group-data-[collapsed=true]/shell:hidden">恢复自动布局</span><PanelLeft className="hidden size-4 group-data-[collapsed=true]/shell:block"/></Button>}
        <SidebarFooter space={meterSpace} user={user} tenant={!!governance?.tenant_id}
          onUsage={() => openUsage()} onSettings={() => navigatePage("settings")}
          onLogout={() => { void act(logout); }} />
      </aside>}
      {mobile && <MobileNavigationDrawer open={mobileNavOpen} onOpenChange={setMobileNavOpen}
        groups={navGroups} items={navigation} page={page}
        onNavigate={target => { navigatePage(target); setPrefix(""); setSearch(""); }}
        onLogout={() => { void act(logout); }}
      />}
      {mobile && <MobileNavigation page={page} allowed={allowedNav} onNavigate={target => { navigatePage(target); setPrefix(""); setSearch(""); }} />}
      <WorkspaceMain>
        <header className="topbar max-md:py-0 max-md:[&_button]:min-h-11 max-md:[&_button]:min-w-11">
          {!mobile && <ThemeToggle />}
          <Button variant="ghost" size="icon" type="button" aria-label="搜索工作区" aria-haspopup="dialog" onClick={openCommandSearch}><Search size={19} /></Button>
          <Button variant="ghost" size="icon" type="button" className="icon sidebar-toggle" aria-label={mobile ? "打开全部导航" : sidebarCollapsed ? "展开侧边栏" : "折叠侧边栏"} aria-expanded={mobile ? mobileNavOpen : !sidebarCollapsed} onClick={() => { if (mobile) setMobileNavOpen(true); else toggleSidebar(); }}><PanelLeft size={19}/></Button>
          {governance?.enabled && <GovernanceBar data={governance} user={user}>{loginMode?.team_demo && <div className="team-demo-switch grid gap-2 rounded-lg border border-border bg-muted/40 p-3"><span className="text-xs font-medium">切换体验身份</span><div className="flex min-w-0 flex-wrap items-center gap-2 [&_[data-slot=select-trigger]]:flex-1"><SelectAdapter aria-label="体验身份" value={demoPersona} onChange={e=>setDemoPersona(e.target.value)}>{loginMode.personas?.map(p=><option key={p.id} value={p.id}>{p.name}</option>)}</SelectAdapter><Button variant="outline" size="sm" type="submit"  disabled={busy} onClick={()=>void act(async()=>{await demoLogin(demoPersona);location.hash="#/teams";location.reload();})}>切换体验身份</Button></div></div>}</GovernanceBar>}
          {mobile && <Button variant="ghost" size="icon" type="button" className="icon" aria-label="空间用量" onClick={() => openUsage()}><Cloud size={18}/></Button>}
          <div className="breadcrumb min-w-0 flex-1">
            {!mobile && !governanceEnabled && <><Button layout="surface" variant="ghost" type="button" className="breadcrumb-link" onClick={driveHome} aria-label="返回我的工作空间">我的工作空间</Button><ChevronRight size={14} /></>}
            {page === "files" && driveOpen ? <Button layout="surface" variant="ghost" type="button" className="breadcrumb-link" aria-label="返回网盘列表" onClick={driveHome}>{title}</Button> : <span className="min-w-0 truncate" title={title}>{title}</span>}
            {page === "files" && driveOpen && <><ChevronRight size={14}/><strong>{currentSpace?.name || "网盘工作区"}</strong></>}
          </div>
          <Button variant="ghost" type="submit" className="transfer-dock" onClick={() => navigateLegacyPage("transfers")} aria-label="打开交付中心"><ArrowUpDown size={17} /><span>{ops.filter(o => !["completed", "cancelled"].includes(o.status)).length} 项传输</span></Button>
        </header>

        <section className={`content ${page !== "knowledge" && !(page === "files" && driveOpen) && !teamDetail ? "workspace-page-content" : ""} ${teamDetail ? "team-detail-content" : ""} ${page === "knowledge" ? "p-0 overflow-hidden" : ""} ${page === "files" && driveOpen && currentSpace ? "drive-content max-md:p-3" : ""}`}>
          <h1 className="sr-only">{title}</h1>
          {error && (
            <div className="banner error app-toast" role="alert">
              {error}
              <Button variant="ghost" size="icon" type="submit"
                className="icon"
                aria-label="关闭错误"
                onClick={() => setError("")}
              >
                <X size={16} />
              </Button>
            </div>
          )}
          {notice && (
            <div className="banner success app-toast" role="status">
              <Check size={16} />
              {notice}
              <Button variant="ghost" size="icon" type="submit"
                className="icon"
                aria-label="关闭通知"
                onClick={() => setNotice("")}
              >
                <X size={16} />
              </Button>
            </div>
          )}
          <input
            ref={input}
            type="file"
            hidden
            multiple={!resume && !newVersion}
            onChange={(e) => {
              const list = Array.from(e.target.files || []);
              e.target.value = "";
              // Consume this picker intent before async work; ordinary drops never
              // inherit it and completion cannot clear a later picker selection.
              const target = { resume, version: newVersion };
              setResume(undefined); setNewVersion(undefined);
              void uploadFiles(list, target);
            }}
          />


          <RouteBoundary scope={JSON.stringify([instance, user.id, getDataContext().tenant, getDataContext().space, page, routeHash])}>
          <Suspense fallback={<p role="status">正在加载页面…</p>}>
          {page === "insights" && <div className="shell-tabs" role="tablist" aria-label="资源态势">{[["overview","概览"],["capacity","容量"],["transfers","传输"],["robots","机器人"],["audit","安全审计"]].map(([id,label]) => <Button variant="ghost" type="submit" key={id} role="tab" tabIndex={insightView === id ? 0 : -1} aria-selected={insightView === id} onKeyDown={e => { const tabs = Array.from(e.currentTarget.parentElement!.querySelectorAll<HTMLButtonElement>('[role="tab"]')); const index = tabs.indexOf(e.currentTarget); const next = e.key === "ArrowRight" ? (index + 1) % tabs.length : e.key === "ArrowLeft" ? (index + tabs.length - 1) % tabs.length : e.key === "Home" ? 0 : e.key === "End" ? tabs.length - 1 : -1; if (next >= 0) { e.preventDefault(); tabs[next].focus(); tabs[next].click(); } }} onClick={() => openInsight(id)}>{label}</Button>)}</div>}
          {page === "members" && (
            <div className="shell-tabs" role="tablist" aria-label="访问凭据">
              {canManageDirectory && <Button variant="ghost" type="button" role="tab" aria-selected={shellTab !== "keys"} onClick={() => navigatePage("members")}>用户</Button>}
              <Button variant="ghost" type="button" role="tab" aria-selected={shellTab === "keys" || !canManageDirectory} onClick={() => navigatePage("users")}>Key</Button>
            </div>
          )}
          {page === "members" && shellTab !== "keys" && !canManageDirectory && <section className="unavailable-drive"><h2>用户目录仅供管理员使用</h2><Button variant="outline" type="submit"  onClick={driveHome}>返回我的网盘</Button></section>}
          {page === "members" && shellTab !== "keys" && canManageDirectory && <IdentityDirectory data={governance} user={user} keys={userKeys} openKeys={() => navigatePage("users")} openTeam={id=>{location.hash=teamURL(id,"overview",governance?.tenant_id);}}/>}
          {page === "governance" && (governance?.enabled ? <GovernancePanel key={`${user.id}:${governance.tenant_id}:${getDataContext().space}`} initial={governance} user={user} initialTab={workspaceRoute(routeHash).tab || "requests"} close={() => navigatePage("files")} /> : <section role="status"><h2>管理与授权</h2><p>{governanceEnabled ? "正在加载管理数据…" : "当前实例未启用团队治理。"}</p><Button variant="outline" onClick={() => window.dispatchEvent(new Event("drivebridge-governance-changed"))}>重新加载</Button></section>)}
          {page === "teams" && <TeamWorkspace enabled={governanceEnabled} governance={governance} user={user} openDrive={openDrive} openKeys={()=>navigatePage("users")}/>}
          {page === "files" && <FilesPage open={driveOpen} currentSpace={currentSpace} focused={driveFocused} routeHash={routeHash}
            spaces={availableSpaces} connections={connections} governance={governance} openDrive={openDrive} home={driveHome}
            connect={!managementRestricted ? () => navigatePage("connections") : undefined}
            finder={{ readOnly, managementRestricted, space, prefix, setPrefix, revision: files, reload: refreshFiles, backendLabel: currentBackend.label,
              upload: list => { setResume(undefined); setNewVersion(undefined); if (list) void uploadFiles(list); else input.current?.click(); },
              onOperation: op => setOps(old => [op, ...old.filter(x => x.id !== op.id)]),
              onPreview: (file, siblings) => { setSelected(file); setPreviewSiblings(siblings); },
              onShare: (files, path) => setShareSelection({ files, path }),
            }} />}
          {page === "knowledge" && <KnowledgePage key={`${instance}:${user.id}:${getDataContext().tenant}:${getDataContext().space}`} actor={`${instance}:${user.id}`} spaces={availableSpaces} onDirtyChange={setKnowledgeDirty}/> }
          {page === "shares" && <SharesPage />}
          {page === "settings" && <PersonalSettings user={user} teamEnabled={governanceEnabled} onUserChange={setUser} openKeys={() => navigatePage("users")} />}
          {page === "trash" && <TrashPage spaces={spaces} space={space}
            readState={fileRead.space === space && fileRead.page === page ? fileRead.state : 'loading'}
            onReload={() => { void act(refreshFiles); }}
            retention={currentSpace?.trash_retention_days || 30}
            backend={connections.some(c => c.id === currentSpace?.connection_id) ? { label: currentBackend.label, description: capabilityText(currentBackend), unrecognized: currentBackend.unrecognized } : undefined}
            search={search} prefix={prefix} grid={grid} busy={busy} restricted={managementRestricted}
            count={filtered.length} folders={folders} visible={visible} picked={picked} setPicked={setPicked}
            restorableCount={restorableSelection(files, picked, space).length}
            onSpace={id => { setPicked({}); setSpace(id); setPrefix(""); }} onSearch={setSearch} onPrefix={setPrefix} onGrid={setGrid}
            onRetention={days => { void act(async () => {
              const updated = await api<Space>("/spaces/" + space + "/trash-policy", "POST", { days });
              setSpaces(old => old.map(s => s.id === updated.id ? { ...s, ...updated } : s)); await refreshFiles();
            }); }}
            onRestoreSelected={() => { void act(async () => {
              const ids = restorableSelection(files, picked, space);
              if (!ids.length) { setPicked({}); return; }
              await api("/files/batch/restore", "POST", { ids }); setPicked({}); await refreshFiles();
            }); }}
            onRestore={file => { void act(async () => { await api("/files/" + file.id + "/restore", "POST", {}); await refreshFiles(); }); }}
            onPreview={file => { setSelected(file); setVersions([]); }}
          />}
          {page === "devices" && <DevicesPage onNavigate={navigatePage}/>}
          {page === "automation" && <AutomationPage scope={JSON.stringify([instance,user.id,getDataContext().tenant,getDataContext().space])} spaces={availableSpaces.filter(s=>!getDataContext().space||s.id===getDataContext().space)} onNavigate={navigatePage}/>}
          {page === "capabilities" && <CapabilitiesPage scope={JSON.stringify([instance,user.id,getDataContext().tenant,getDataContext().space])} spaces={availableSpaces.filter(s=>!getDataContext().space||s.id===getDataContext().space)}/>}
          {page === "properties" && <PropertiesPage scope={JSON.stringify([instance,user.id,getDataContext().tenant,getDataContext().space])} spaces={availableSpaces.filter(s=>!getDataContext().space||s.id===getDataContext().space)}/>}
          {page === "collections" && <CollectionsPage scope={JSON.stringify([instance,user.id,getDataContext().tenant,getDataContext().space])} spaces={availableSpaces.filter(s=>!getDataContext().space||s.id===getDataContext().space)}/>}
          {page === "resources" && <ResourcesPage scope={JSON.stringify([instance,user.id,getDataContext().tenant,getDataContext().space])} spaces={availableSpaces.filter(s=>!getDataContext().space||s.id===getDataContext().space)}/>}
          {page === "overview" && <WorkspaceViews key={`home:${instance}:${user.id}:${getDataContext().tenant}:${getDataContext().space}:${routeHash}`}
            initialView={/^#\/overview(?:[/?]|$)/.test(routeHash) ? "legacy" : "primary"}
            label="工作台视图" primaryLabel="我的工作台" legacyLabel="容量与活动观察"
            primary={<WorkspaceHome scope={JSON.stringify([instance,user.id,getDataContext().tenant,getDataContext().space])} spaces={availableSpaces.filter(s=>!getDataContext().space||s.id===getDataContext().space)}/>}
            legacy={<InfoWorkspace key={`${user.id}:${governance?.tenant_id || ""}:${getDataContext().space}`} spaces={availableSpaces} preferenceIdentity={{ instance, user: user.id }} />}
          />} 

          {(page === "flows" || page === "search") && (
            <FlowPage
              key={`${page}:${user.id}:${governance?.tenant_id || ""}:${getDataContext().space}`}
              page={page} tab={shellTab} tenant={getDataContext().tenant}
              user={user} spaces={availableSpaces}
              onNavigate={(page, tab) => dispatchNavigation({ type: "route", page, tab })}
            />
          )}

          {page === "insights" && insightView === "overview" && (
            <>
              <DashboardOverview onUsage={openUsage} spaces={availableSpaces} operations={ops} agents={agents} files={files} spaceName={currentSpace?.name} openFiles={driveHome} openTransfers={() => setPage("transfers")} preview={file => {setSelected(file);setPreviewSiblings(files);}}/>

            </>
          )}
          {page === "insights" && insightView !== "overview" && (
            <>
            <InsightsPanel key={`${user.id}:${governance?.tenant_id || ""}:${getDataContext().space}:${insightView}:${insightSpace}`} view={insightView} initialSpace={insightSpace}
              actorNames={{users:Object.fromEntries([...(governance?.users || []), {...user,name:user.display_name || user.name}].map(u=>[u.id,u.name])),agents:Object.fromEntries(agents.map(a=>[a.id,a.name]))}} connectionNames={Object.fromEntries(connections.map(c=>[c.id,c.name]))}
              onNavigate={(kind, id) => {
                void act(async () => {
                  if (kind === "file") {
                    const result = await api<{ file: DriveFile }>(
                      "/files/" + encodeURIComponent(id),
                    );
                    setSelected(result.file);
                  } else {
                    const result = await api<{ operation: Operation }>(
                      "/operations/" + encodeURIComponent(id),
                    );
                    setPage("transfers");
                    setOps([result.operation]);
                  }
                });
              }}
            />
            </>
          )}
          {taskAccessDenied && ["transfers", "inbox"].includes(page) && <section role="alert" className="unavailable-drive"><h2>任务记录暂不可访问</h2><p>任务读取权限已失效，请重新登录或检查当前访问范围。</p><Button variant="outline" type="button" disabled={busy} onClick={() => void act(async () => {
            const signal = new AbortController().signal;
            await verifySessionIdentity(user.id, instance, signal);
            const [operations, inbox] = await readOperationProjection(signal);
            setOps(operations); setInboxOps(inbox); setTaskAccessDenied(false);
          })}>重新检查访问权限</Button></section>}
          {page === "transfers" && !taskAccessDenied && <WorkspaceViews key={`tasks:${instance}:${user.id}:${getDataContext().tenant}:${getDataContext().space}:${routeHash}`}
            initialView={/^#\/transfers(?:[/?]|$)/.test(routeHash) || new URLSearchParams(routeHash.split('?')[1]).get('view') === 'legacy' ? 'legacy' : 'primary'}
            label="任务视图" primaryLabel="任务与计划" legacyLabel={`传输记录与恢复${pendingTaskCount ? ` · ${pendingTaskCount} 项未结束` : ''}`}
            primary={<OperationTasks scope={JSON.stringify([instance,user.id,getDataContext().tenant,getDataContext().space])}/>}
            legacy={<LegacyTransfersPage
            key={"transfers:"+user.id+":"+getDataContext().tenant}
            operations={ops} scope={historyScope(instance, user.id, getDataContext().tenant, getDataContext().space, "transfers")}
            busy={busy} cancelId={cancelOp} onNavigate={target => target === 'transfers' || target === 'inbox' ? navigateLegacyPage(target) : navigatePage(target)}
            onOpen={op => { void act(() => openOperationResult(op)); }}
            onCopy={op => { void act(async () => {
              await copyReference({id:op.file_id!,version_id:op.version_id!,path:op.path,mime_type:op.mime_type||"",size_bytes:op.size_bytes,space_id:op.space_id,trashed:false,updated_at:op.updated_at||""});
              setNotice("已复制文件引用");
            }); }}
            onResume={op => { void act(async () => {
              if (op.kind === "upload" && op.bytes_transferred < op.size_bytes) { setResume(op); input.current?.click(); }
              else { await api("/operations/" + op.id + "/resume", "POST", {}); await reload(); await refreshFiles(); }
            }); }}
            onCancelRequest={setCancelOp}
            onCancelConfirm={op => { void act(async () => {
              controllers.current.get(op.id)?.abort(); await api("/operations/" + op.id + "/cancel", "POST", {});
              setCancelOp(""); await reload();
            }); }}
          />}/>} 
          {page === "inbox" && !taskAccessDenied && <WorkspaceViews key={`inbox:${instance}:${user.id}:${getDataContext().tenant}:${getDataContext().space}:${routeHash}`}
            initialView={new URLSearchParams(routeHash.split('?')[1]).get('view') === 'legacy' ? 'legacy' : 'primary'}
            label="收件视图" primaryLabel="待验收与归档" legacyLabel="历史传输收件"
            primary={<SubmissionInbox routeHash={routeHash} scope={JSON.stringify([instance,user.id,getDataContext().tenant,getDataContext().space])} spaces={availableSpaces.filter(s=>!getDataContext().space||s.id===getDataContext().space)}/>}
            legacy={<LegacyInboxPage
            key={"inbox:"+user.id+":"+getDataContext().tenant}
            operations={inboxOps} scope={historyScope(instance, user.id, getDataContext().tenant, getDataContext().space, "inbox")} busy={busy}
            onTransfers={() => navigateLegacyPage("transfers")}
            onOpen={op => { void act(() => openOperationResult(op)); }}
            onCopy={op => { void act(async () => {
              await copyReference({id:op.file_id!,version_id:op.version_id!,path:op.path,mime_type:op.mime_type||"",size_bytes:op.size_bytes,space_id:op.space_id,trashed:false,updated_at:op.updated_at||""});
              setNotice("已复制文件引用");
            }); }}
          />}/>} 
          {page === "connections" && <ConnectionsPage connections={connections}
            onCheck={c => { void act(async () => { await api("/connections/" + c.id + "/check", "POST", {}); setNotice("连接验证通过"); }); }}
            onBrowse={c => { void act(async () => {
              setDriveConnection(c.id);
              const out = await api<{ items: any[] }>("/connections/" + c.id + "/items");
              setDriveItems(out.items); setModal("onedrive");
            }); }}
            onUse={c => { setDriveConnection(c.id); setModal("space"); }}
            onDisconnect={c => { setDriveConnection(c.id); setModal("disconnect"); }}
            onAdd={() => setModal("storage")}
          />}
          {page === "members" && shellTab === "keys" && admin && !governanceEnabled && <div className="mb-3 flex justify-end"><Button type="button" className="min-h-11 md:min-h-8" onClick={() => { setIssueNames(""); setIssueDays(""); setIssueAdmin(false); setModal("issue"); }}><Plus size={16}/>批量签发</Button></div>}
          {page === "members" && shellTab === "keys" && <KeyDirectory key={`${user.id}:${governance?.tenant_id || ""}:${getDataContext().space || ""}`} keys={userKeys} data={governance} canManage={admin && !governanceEnabled} reload={reloadKeys} revoke={k => {setRevokeTarget(k);setModal("revokeuser");}} archive={k => setArchiveTarget({kind:"users",id:k.id,name:k.name})}/>}
          {page === "agents" && <WorkspaceViews key={`agents:${instance}:${user.id}:${getDataContext().tenant}:${getDataContext().space}`}
            label="Agent 授权视图" primaryLabel="Agent 管理" legacyLabel="旧版机器人授权"
            actions={<Button variant="outline" type="button" className="min-h-11 text-[13px] md:min-h-8" onClick={() => setSetupOpen(true)}>连接指南</Button>}
            primary={<GrantOverview spaces={availableSpaces.filter(s => !getDataContext().space || s.id === getDataContext().space)} scope={JSON.stringify([instance, user.id, getDataContext().tenant, getDataContext().space])} />}
            legacy={<><div className="mb-4 flex flex-wrap items-center justify-between gap-3"><p className="text-sm text-muted-foreground">旧协议授权保留原范围与期限规则，不自动转换为新授权。</p><Button onClick={() => setModal("agent")}><Plus size={16}/>授权机器人</Button></div><LegacyAgentPage
            agents={agents} audits={audits} spaces={availableSpaces}
            ownerLabel={getDataContext().space ? "当前空间" : user.name}
            query={agentQuery} filter={agentFilter} archived={archived}
            onQuery={setAgentQuery} onFilter={setAgentFilter} onArchived={setArchived}
            onRevoke={a => { setDriveConnection(a.id); setModal("revoke"); }}
            onArchive={a => setArchiveTarget({kind:"agents",id:a.id,name:a.name})}
          /></>}/>} 
          </Suspense>
          </RouteBoundary>
        </section>

      </WorkspaceMain>
      {commandSearchOpen && <CommandSearch spaces={availableSpaces} close={closeCommandSearch} />}
      {usageOpen && <CapacityPanel key={user.id+":"+(governance?.tenant_id || "")+":"+space} initialSpace={usageSpace} onClose={() => setUsageOpen(false)} onRefresh={setSpaces} onDetails={id => {setUsageOpen(false);openInsight("capacity",id);}}/>}
      {setupOpen && <Modal className="setup-drawer" title="连接机器人" close={() => setSetupOpen(false)}><RobotSetup expanded /></Modal>}
      {shareSelection && <ShareDialog {...shareSelection} space={space} close={() => setShareSelection(undefined)} />}
      {archiveTarget && <Modal title="删除并归档" close={() => setArchiveTarget(undefined)}><p>删除「{archiveTarget.name}」将立即吊销凭据{archiveTarget.kind === "users" ? "及其关联浏览器会话" : "的后续访问"}，记录保留在已归档中。文件不会删除。</p><div className="dialog-actions"><Button variant="outline" type="submit"  onClick={() => setArchiveTarget(undefined)}>取消</Button><Button variant="destructive" type="submit"  onClick={() => act(async () => { await api(`/${archiveTarget.kind}/${archiveTarget.id}/archive`, "POST", {}); setArchiveTarget(undefined); await reload(); if (admin) { const out = await api<{keys:UserKeyRow[]}>("/key-directory"); setUserKeys(out.keys); } })}>确认删除并归档</Button></div>{error && <p className="error" role="alert">{error}</p>}</Modal>}
      {selected && !modal && (
        <FilePreviewDialog
          title={selected.path.split("/").pop() || "文件详情"}
          close={() => setSelected(null)}
          notebookAction={canOpenMarkdownNotebook(selected, getDataContext().space || space) && <Button type="button" variant="outline" aria-pressed={notebookPreview} onClick={() => setNotebookPreview(value => !value)}>{notebookPreview ? "原文件预览" : "笔记工作台"}</Button>}
          information={<> 
          <PreviewFiles files={previewSiblings.length ? previewSiblings : [selected]} selected={selected} onSelect={setSelected} />
          <section className="space-y-3 border-t border-border pt-5"><h3 className="text-sm font-semibold">文件信息</h3><dl className="quicklook-metadata"><dt>路径</dt><dd title={selected.path}>{selected.path}</dd><dt>大小</dt><dd>{bytes(selected.size_bytes)}</dd><dt>格式</dt><dd>{selected.mime_type}</dd></dl></section>
          {!selected.trashed && <LabelSuggestionEntry key={JSON.stringify([instance,user.id,getDataContext().tenant,getDataContext().space,selected.id,selected.version_id])} file={selected.id} version={selected.version_id} scope={JSON.stringify([instance,user.id,getDataContext().tenant,getDataContext().space])} readOnly={managementRestricted}/>}
          {!selected.trashed && <FilePropertiesEntry key={JSON.stringify([instance,user.id,getDataContext().tenant,getDataContext().space,selected.id,selected.version_id])} file={selected.id} version={selected.version_id} space={selected.space_id} path={selected.path} scope={JSON.stringify([instance,user.id,getDataContext().tenant,getDataContext().space])} readOnly={managementRestricted} onDirty={onPropertyDirty}/>}
          <div className="detail-meta">

            <span>
              所属存储：
              {(() => {
                const backend = backendOf(
                  connections.find(
                    (c) =>
                      c.id ===
                      spaces.find((s) => s.id === selected.space_id)
                        ?.connection_id,
                  )?.kind,
                );
                return backend.label + " · " + capabilityText(backend);
              })()}
            </span>
          </div>
          <div className="modal-actions">
            <Button variant="default" type="submit"

              disabled={selected.trashed || managementRestricted}
              onClick={() => act(() => download(selected))}
            >
              <ArrowDownToLine size={16} />
              下载
            </Button>
            <Button variant="outline" type="submit"

              disabled={selected.trashed || managementRestricted}
              onClick={() => {
                setNewVersion(selected);
                input.current?.click();
                setSelected(null);
              }}
            >
              上传新版本
            </Button>
            <Button variant="outline" type="submit"

              disabled={managementRestricted}
              onClick={() =>
                act(async () => {
                  await api(
                    "/files/" +
                      selected.id +
                      (selected.trashed ? "/restore" : "/trash"),
                    "POST",
                    {},
                  );
                  setSelected(null);
                  await refreshFiles();
                })
              }
            >
              {selected.trashed ? "恢复" : "移入回收站"}
            </Button>
          </div>
          <PreviewSection title="复制到其他网盘"><form className="flex flex-col gap-4"
            onSubmit={(e) => {
              e.preventDefault();
              const form = new FormData(e.currentTarget);
              void act(async () => {
                await api(
                  "/archive",
                  "POST",
                  {
                    source_file_id: selected.id,
                    source_version_id: selected.version_id,
                    target_space_id: String(form.get("target_space_id")),
                    target_path: String(form.get("target_path")),
                  },
                  { "Idempotency-Key": crypto.randomUUID() },
                );
                setNotice("已复制固定版本到目标空间");
                setSelected(null);
                await reload();
              });
            }}
          >
            <p className="text-muted-foreground leading-relaxed">将当前版本复制到目标网盘，原文件保持不变。</p>
            <label className="flex flex-col gap-2 text-sm font-medium">目标空间<SelectAdapter name="target_space_id" defaultValue={selected.space_id}>{spaces.map((s) => <option key={s.id} value={s.id}>{s.name}</option>)}</SelectAdapter></label>
            <label className="flex flex-col gap-2 text-sm font-medium">目标路径<Input name="target_path" defaultValue={selected.path} required /></label>
            <Button variant="default" className="w-full" type="submit" disabled={managementRestricted || selected.trashed}>复制为受管副本</Button>
          </form></PreviewSection>
          <PreviewSection title="文件标识与引用">
          <dl className="quicklook-metadata"><dt>文件标识</dt><dd className="font-mono">{selected.id}</dd><dt>版本</dt><dd className="font-mono">{selected.version_id}</dd></dl>
          <p>
            来源：{selected.caller || "未记录"}
            {selected.task_ref ? " · 任务 " + selected.task_ref : ""}
          </p>
          <Button variant="outline" type="submit"

            onClick={() =>
              act(async () => {
                await copyReference(selected);
                setNotice(
                  "已复制固定版本引用；在已连接 DriveBridge 的 Agent 中使用",
                );
              })
            }
          >
            复制文件引用
          </Button>
          <p className="muted">
            引用固定当前所选版本。Agent 需要连接 DriveBridge 才能取用文件。
          </p>
          </PreviewSection>
          <PreviewSection title={`版本记录（${versions.length}）`}>
          {versions.map((v) => (
            <div className="version-row" key={v.id}>
              <span>{new Date(v.created_at).toLocaleString("zh-CN")}</span>
              <span>
                {bytes(v.size_bytes)}
                {v.caller ? " · " + v.caller : ""}
              </span>
              <Button variant="ghost" type="submit"
                className="text-button"
                onClick={() =>
                  act(async () => {
                    await copyReference(selected, v.id);
                    setNotice("已复制此版本引用");
                  })
                }
              >
                复制此版本引用
              </Button>
              <Button variant="ghost" type="submit"
                className="text-button"
                onClick={() => act(() => download(selected, v.id))}
              >
                下载此版本
              </Button>
              {!selected.trashed && v.id !== selected.version_id && <FilePropertiesEntry history file={selected.id} version={v.id} space={selected.space_id} path={selected.path} scope={JSON.stringify([instance,user.id,getDataContext().tenant,getDataContext().space])}/>}
            </div>
          ))}
          </PreviewSection>
          {connections.some((c) => c.kind === "onedrive" && !c.disabled) && (
            <form
              onSubmit={(e) => {
                e.preventDefault();
                const f = new FormData(e.currentTarget);
                void act(async () => {
                  await api(
                    "/connections/" + f.get("connection") + "/export",
                    "POST",
                    { file_id: selected.id },
                    { "Idempotency-Key": crypto.randomUUID() },
                  );
                  setNotice("文件已回存 OneDrive");
                  setSelected(null);
                  await reload();
                });
              }}
            >
              <label>
                回存至 OneDrive
                <SelectAdapter name="connection">
                  {connections
                    .filter((c) => c.kind === "onedrive" && !c.disabled)
                    .map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.name}
                      </option>
                    ))}
                </SelectAdapter>
              </label>
              <Button variant="outline" type="submit"  disabled={busy}>
                保存副本到 OneDrive
              </Button>
            </form>
          )}
          </>}
        >
          {notebookPreview && canOpenMarkdownNotebook(selected, getDataContext().space || space)
            ? <RouteBoundary scope={`${user.id}:${selected.space_id}:${selected.id}:${selected.version_id}`}><Suspense fallback={<p role="status">正在加载笔记工作台…</p>}><MarkdownNotebookPreview key={`${selected.space_id}:${selected.id}:${selected.version_id}`} actor={user.id} file={selected} siblings={previewSiblings}/></Suspense></RouteBoundary>
            : <MediaPreview key={selected.id + selected.version_id} file={selected} />}
          <nav className="quicklook-controls flex flex-row items-center justify-center gap-3 max-md:[&_button]:min-h-11" aria-label="切换预览文件">{[-1,1].map(step => {const index=previewSiblings.findIndex(f=>f.id===selected.id);const next=previewSiblings[index+step];return <Button variant="outline" size="sm" type="button" key={step} disabled={index<0 || !next} onClick={()=>setSelected(next)}>{step<0?"上一个":"下一个"}</Button>})}<span>{Math.max(1,previewSiblings.findIndex(f=>f.id===selected.id)+1)} / {Math.max(1,previewSiblings.length)} · 方向键切换</span></nav>
        </FilePreviewDialog>
      )}
      {modal && (
        <Modal
          className={modal === "agent" || modal === "issue" ? "sm:max-w-[480px]" : modal === "storage" ? "max-md:[&_button]:min-h-11 max-md:[&_input]:min-h-11" : undefined}
          title={
            (
              {
                storage: "连接存储",
                agent: "授权机器人",
                token: "保存你的机器人 token",
                folder: "选择上传目录",
                space: "设置存储空间",
                disconnect: "断开存储连接",
                revoke: "撤销 机器人授权",
                onedrive: "从 OneDrive 选择文件",
                issue: "批量签发用户密钥",
                issued: "保存新签发的密钥",
                revokeuser: "吊销用户密钥",
              } as Record<string, string>
            )[modal] || modal
          }
          close={() => {
            setModal("");
            setToken("");
            setIssuedKeys([]);
          }}
        >
          {error && (
            <div className="banner error" role="alert">
              {error}
            </div>
          )}
          {modal === "storage" && (
            <>
              <Button variant="ghost" type="submit"
                className="onedrive-connect button"
                disabled={busy}
                onClick={() =>
                  act(async () => {
                    const out = await api<{ authorization_url: string }>(
                      "/connections/onedrive",
                      "POST",
                      {},
                    );
                    location.assign(out.authorization_url);
                  })
                }
              >
                <Cloud />
                授权连接 OneDrive <ArrowUpRight size={16} />
              </Button>
              <div className="divider">或连接 S3 兼容存储</div>
              <form
                onSubmit={(e) => {
                  e.preventDefault();
                  const f = new FormData(e.currentTarget);
                  void act(async () => {
                    await api("/connections/s3", "POST", {
                      name: f.get("name"),
                      config: {
                        endpoint: f.get("endpoint"),
                        region: f.get("region"),
                        bucket: f.get("bucket"),
                        prefix: f.get("prefix"),
                        access_key: f.get("access_key"),
                        secret_key: f.get("secret_key"),
                      },
                    });
                    setModal("");
                    await reload();
                    setNotice("存储已连接，可绑定到新空间");
                  });
                }}
              >
                <label>
                  连接名称
                  <Input name="name" required placeholder="我的文件存储" />
                </label>
                <label>
                  服务地址
                  <Input
                    name="endpoint"
                    type="url"
                    required
                    placeholder="https://s3.example.com"
                  />
                </label>
                <div className="form-columns">
                  <label>
                    存储桶
                    <Input name="bucket" required />
                  </label>
                  <label>
                    区域
                    <Input name="region" defaultValue="us-east-1" required />
                  </label>
                </div>
                <label>
                  专用前缀
                  <Input name="prefix" placeholder="drivebridge" />
                </label>
                <label>
                  Access key
                  <Input name="access_key" autoComplete="off" required />
                </label>
                <label>
                  Secret key
                  <Input
                    name="secret_key"
                    type="password"
                    autoComplete="new-password"
                    required
                  />
                </label>
                <div className="dialog-actions">
                <Button variant="outline" type="button" onClick={() => setModal("")}>取消</Button>
                <Button variant="default" type="submit"  disabled={busy}>
                  {busy ? (
                    <LoaderCircle className="spin" size={17} />
                  ) : (
                    <Plug size={17} />
                  )}
                  验证并连接
                </Button>
                </div>
              </form>
            </>
          )}
          {modal === "agent" && (
            <form
              onSubmit={(e) => {
                e.preventDefault();
                const f = new FormData(e.currentTarget);
                void act(async () => {
                  const out = await api<{ token: string }>("/agents", "POST", {
                    name: f.get("name"),
                    space_id: space,
                    prefix: f.get("prefix"),
                    scopes: f.getAll("scope").join(","),
                    days: Number(f.get("days")),
                    connection_id: f.get("connection"),
                  });
                  setToken(out.token);
                  setModal("token");
                  await reload();
                });
              }}
            >
              <label>
                机器人名称
                <Input name="name" required placeholder="例如：文件整理助手" />
              </label>
              <label>
                允许访问的目录
                <Input
                  name="prefix"
                  placeholder="留空表示当前空间"
                  defaultValue={prefix}
                />
              </label>
              <fieldset className="form-options">
                <legend>允许的操作</legend>
                {[
                  ["read", "读取文件"],
                  ["write", "上传与更新"],
                  ["delete", "移入回收站"],
                  ["import", "导入外部文件"],
                  ["export", "回存外部网盘"],
                ].map(([v, l]) => (
                  <label className="checkbox" key={v}>
                    <Checkbox

                      name="scope"
                      value={v}
                      defaultChecked={v === "read"}
                    />
                    {l}
                  </label>
                ))}
              </fieldset>
              <label>
                允许的外部连接
                <SelectAdapter name="connection">
                  <option value="">不授权外部连接</option>
                  {connections
                    .filter((c) => c.kind === "onedrive" && !c.disabled)
                    .map((c) => (
                      <option key={c.id} value={c.id}>
                        {c.name}
                      </option>
                    ))}
                </SelectAdapter>
              </label>
              <label>
                有效天数
                <Input
                  type="number"
                  name="days"
                  min="1"
                  max="30"
                  defaultValue="7"
                  required
                />
              </label>
              <div className="dialog-actions">
                <Button variant="outline" type="button" onClick={() => setModal("")}>取消</Button>
                <Button variant="default" type="submit" disabled={busy}>
                  <KeyRound size={16} />
                  {busy ? "正在创建…" : "创建限定授权"}
                </Button>
              </div>
            </form>
          )}
          {modal === "issue" && admin && (
            <form
              onSubmit={(e) => {
                e.preventDefault();
                void act(async () => {
                  const users = issueNames
                    .split(/[\n,]+/)
                    .map((n) => n.trim())
                    .filter(Boolean)
                    .map((name) => ({
                      name,
                      ...(issueDays ? { days: Number(issueDays) } : {}),
                      ...(issueAdmin ? { admin: true } : {}),
                    }));
                  const out = await api<{ issued: IssuedUserKey[] }>(
                    "/users",
                    "POST",
                    { users },
                  );
                  setIssuedKeys(out.issued || []);
                  setModal("issued");
                  setNotice("已签发 " + (out.issued?.length || 0) + " 个用户");
                });
              }}
            >
              <label>
                用户名称（每行一个，或逗号分隔）
                <Textarea
                  name="names"
                  rows={4}
                  required
                  value={issueNames}
                  placeholder={"alice\nbob"}
                  onChange={(e) => setIssueNames(e.target.value)}
                />
              </label>
              <label>
                有效天数（留空 = 长期有效）
                <Input
                  name="days"
                  type="number"
                  min="1"
                  value={issueDays}
                  onChange={(e) => setIssueDays(e.target.value)}
                />
              </label>
              <label className="checkbox">
                <Checkbox

                  checked={issueAdmin}
                  onCheckedChange={(e) => setIssueAdmin(e)}
                />
                管理员（可签发与吊销用户密钥）
              </label>
              <div className="dialog-actions">
                <Button variant="outline" type="button" onClick={() => setModal("")}>取消</Button>
                <Button variant="default" type="submit" disabled={busy}><UsersRound size={16} />{busy ? "正在签发…" : "签发"}</Button>
              </div>
            </form>
          )}
          {modal === "issued" && (
            <>
              <div className="info-note">
                <ShieldCheck />
                <p>
                  这些密钥只显示这一次。请立即分发给对应用户；关闭后无法再次查看。
                </p>
              </div>
              {issuedKeys.map((k) => (
                <div className="issued-key" key={k.user_id}>
                  <label>
                    {k.name}
                    {k.admin ? "（管理员）" : ""}
                    <Textarea
                      aria-label={k.name + " 密钥"}
                      readOnly
                      value={k.key}
                      className="token-value"
                    />
                  </label>
                  <Button variant="outline" size="sm" type="submit"

                    onClick={() =>
                      act(async () => {
                        await navigator.clipboard.writeText(k.key);
                        setNotice("已复制，请安全保存");
                      })
                    }
                  >
                    <Copy size={16} />
                    复制
                  </Button>
                </div>
              ))}
              <div className="modal-actions">
                <Button variant="outline" type="submit"

                  onClick={() => {
                    setIssuedKeys([]);
                    setModal("");
                    api<{ keys: UserKeyRow[] }>("/key-directory")
                      .then((out) => setUserKeys(out.keys || []))
                      .catch(() => {});
                  }}
                >
                  我已保存，关闭
                </Button>
              </div>
            </>
          )}
          {modal === "revokeuser" && revokeTarget && (
            <>
              <p>
                吊销「{revokeTarget.name}」的密钥？该密钥与其换发的浏览器会话
                将在下一次请求时立即失效。
              </p>
              <div className="modal-actions">
                <Button variant="outline" type="submit"

                  onClick={() => {
                    setRevokeTarget(null);
                    setModal("");
                  }}
                >
                  取消
                </Button>
                <Button variant="destructive" type="submit"

                  disabled={busy}
                  onClick={() =>
                    act(async () => {
                      await api(
                        "/users/" + revokeTarget.id + "/revoke",
                        "POST",
                        {},
                      );
                      setRevokeTarget(null);
                      setModal("");
                      const out = await api<{ keys: UserKeyRow[] }>("/key-directory");
                      setUserKeys(out.keys || []);
                    })
                  }
                >
                  确认吊销
                </Button>
              </div>
            </>
          )}
          {modal === "token" && (
            <>
              {notice.startsWith("已复制") && <p role="status">{notice}</p>}
              <div className="info-note">
                <ShieldCheck />
                <p>
                  此 token 只显示一次。请保存到机器人
                  的私有配置，不要放进聊天、日志或代码仓库。
                </p>
              </div>
              <Textarea
                aria-label="新机器人 token"
                readOnly
                value={token}
                className="token-value"
              />
              <Button variant="default" type="submit"

                onClick={() =>
                  act(async () => {
                    await navigator.clipboard.writeText(token);
                    setNotice("已复制，请安全保存");
                  })
                }
              >
                <Copy size={16} />
                复制 token
              </Button>
              <Button variant="outline" type="submit"

                onClick={() => {
                  setToken("");
                  setModal("");
                }}
              >
                已安全保存
              </Button>
              <RobotSetup />
            </>
          )}
          {modal === "space" && (
            <form
              onSubmit={(e) => {
                e.preventDefault();
                const f = new FormData(e.currentTarget);
                void act(async () => {
                  if (currentSpace && !currentSpace.connection_id) {
                    await api("/spaces/" + space + "/bind", "POST", {
                      connection_id: driveConnection,
                    });
                  } else {
                    const out = await api<Space>("/spaces", "POST", {
                      name: f.get("name"),
                      connection_id: driveConnection,
                    });
                    setSpace(out.id);
                  }
                  setModal("");
                  await reload();
                });
              }}
            >
              <p>
                {!currentSpace ? "为此存储创建一个文件空间。" : currentSpace.connection_id
                  ? "当前空间的存储绑定不可变，可创建一个新空间。"
                  : "将此存储绑定到你的个人空间。"}
              </p>
              {(!currentSpace || currentSpace.connection_id) && (
                <label>
                  新空间名称
                  <Input name="name" required />
                </label>
              )}
              <Button variant="default" type="submit"  disabled={busy}>
                确定使用
              </Button>
            </form>
          )}
          {["disconnect", "revoke"].includes(modal) && (
            <>
              <p>
                {modal === "disconnect"
                  ? "断开后将无法读取该存储中的文件，已有文件引用仍会保留。"
                  : "此 Agent 的后续请求将被拒绝，已经下载的文件副本无法收回。"}
              </p>
              <div className="modal-actions">
                <Button variant="outline" type="submit"  onClick={() => setModal("")}>
                  取消
                </Button>
                <Button variant="destructive" type="submit"

                  disabled={busy}
                  onClick={() =>
                    act(async () => {
                      if (modal === "disconnect")
                        await api(
                          "/connections/" + driveConnection + "/disconnect",
                          "POST",
                          {},
                        );
                      else if (modal === "revoke")
                        await api(
                          "/agents/" + driveConnection + "/revoke",
                          "POST",
                          {},
                        );

                      setModal("");
                      setSelected(null);
                      await reload();
                      await refreshFiles();
                    })
                  }
                >
                  确认
                  {modal === "disconnect" ? "断开" : "撤销"}
                </Button>
              </div>
            </>
          )}
          {modal === "onedrive" && (
            <>
              <Button variant="ghost" type="submit"
                className="text-button"
                onClick={() =>
                  act(async () => {
                    const out = await api<{ items: any[] }>(
                      "/connections/" + driveConnection + "/items",
                    );
                    setDriveItems(out.items);
                  })
                }
              >
                <ArrowLeft size={16} />
                根目录
              </Button>
              <div className="drive-items">
                {driveItems.map((i) => (
                  <div key={i.id}>
                    <span className="file-type">
                      {i.folder ? <Folder /> : <FileText />}
                    </span>
                    <span>{i.name}</span>
                    <Button variant="outline" size="sm" type="submit"

                      disabled={busy}
                      onClick={() =>
                        act(async () => {
                          if (i.folder) {
                            const out = await api<{ items: any[] }>(
                              "/connections/" +
                                driveConnection +
                                "/items?parent=" +
                                encodeURIComponent(i.id),
                            );
                            setDriveItems(out.items);
                          } else {
                            await api(
                              "/connections/" + driveConnection + "/import",
                              "POST",
                              {
                                item_id: i.id,
                                version: i.eTag,
                                space_id: space,
                                path: (prefix ? prefix + "/" : "") + i.name,
                              },
                              { "Idempotency-Key": crypto.randomUUID() },
                            );
                            setNotice("文件已导入当前空间");
                            setModal("");
                            await reload();
                            await refreshFiles();
                          }
                        })
                      }
                    >
                      {i.folder ? "打开" : "导入"}
                    </Button>
                  </div>
                ))}
              </div>
              <small className="subtle">
                仅复制你选择的文件，不同步整个网盘。
              </small>
            </>
          )}
        </Modal>
      )}
    </WorkspaceShell>
  );
}
export default App;
