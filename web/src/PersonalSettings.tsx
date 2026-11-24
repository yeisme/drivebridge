import { UserAvatar } from "./components/UserAvatar";
import { ComboboxField, type ComboboxOption } from "./components/ComboboxField";
import { useState, useEffect, useRef } from "react";
import {
  ArrowUpRight,
  Bot,
  Check,
  CircleHelp,
  Copy,
  Gauge,
  HardDrive,
  KeyRound,
  Library,
  ListTodo,
  Monitor,
  Moon,
  Palette,
  Plug,
  ShieldCheck,
  SlidersHorizontal,
  Sun,
  Tags,
  UserRound,
  Users,
  Workflow,
} from "lucide-react";
import { useAppearance, defaultAppearance, type Theme } from "./appearance";
import { Button } from "./components/ui/button";
import { WorkspacePageHeader } from "./components/WorkspacePageHeader";
import "./personal-settings.css";
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "./components/ui/card";
import { RadioGroup, RadioGroupItem } from "./components/ui/radio-group";
import { Switch } from "./components/ui/switch";
import { Input } from "./components/ui/input";
import { api, errorText, type UserIdentity } from "./api";
import type { Page } from "./app/navigation-state";

const themes = [
  { value: "light", title: "浅色", hint: "明亮清透背景，舒适浏览文件", icon: Sun },
  { value: "dark", title: "深色", hint: "柔和深色画布，降低夜间眩光", icon: Moon },
  {
    value: "system",
    title: "跟随系统",
    hint: "随设备外观自动切换",
    icon: Monitor,
  },
] as const;

interface HubEntry {
  id: Page | "drives";
  title: string;
  description: string;
  icon: typeof Plug;
}

export function PersonalSettings({
  user,
  teamEnabled,
  governanceEnabled = false,
  allowedPages,
  onNavigate,
  onOpenAllDrives,
  onUserChange,
  openKeys,
}: {
  user: UserIdentity;
  teamEnabled: boolean;
  governanceEnabled?: boolean;
  allowedPages?: readonly Page[];
  onNavigate?: (page: Page) => void;
  onOpenAllDrives?: () => void;
  onUserChange: (user: UserIdentity) => void;
  openKeys: () => void;
}) {
  const root = useRef<HTMLDivElement>(null);
  const [activeSection, setActiveSection] = useState("appearance");
  const canVisit = (target: Page) => !allowedPages || allowedPages.includes(target);

  const storageEntries: HubEntry[] = [
    ...(onOpenAllDrives
      ? [{ id: "drives" as const, title: "全部网盘目录", description: "查看与切换全部个人网盘和团队空间", icon: HardDrive }]
      : []),
    ...(canVisit("connections")
      ? [{ id: "connections" as const, title: "存储连接挂载", description: "绑定 OneDrive、S3 兼容存储并检查状态", icon: Plug }]
      : []),
    ...(canVisit("devices")
      ? [{ id: "devices" as const, title: "设备与离线状态", description: "查看当前浏览器与桌面宿主的离线能力", icon: Monitor }]
      : []),
    ...(canVisit("insights")
      ? [{ id: "insights" as const, title: "空间容量与动态", description: "查看各网盘容量分布与传输趋势", icon: Gauge }]
      : []),
  ];

  const aiAndToolsEntries: HubEntry[] = [
    ...(canVisit("agents")
      ? [{ id: "agents" as const, title: "AI 助手授权管理", description: "按文件夹分配最小读写权限与有效期", icon: Bot }]
      : []),
    ...(canVisit("automation")
      ? [{ id: "automation" as const, title: "自动备份与整理", description: "查看已配置的自动整理与归档规则", icon: Workflow }]
      : []),
    ...(canVisit("properties")
      ? [{ id: "properties" as const, title: "标签与属性字典", description: "管理自定义文件标签与元数据字段", icon: Tags }]
      : []),
    ...(canVisit("resources")
      ? [{ id: "resources" as const, title: "模板与资源包", description: "管理可复用的参考模板与资料包文件", icon: Library }]
      : []),
    ...(canVisit("flows")
      ? [{ id: "flows" as const, title: "快捷文件流程", description: "批量上传、高级检索与固定清单交付", icon: SlidersHorizontal }]
      : []),
    ...(canVisit("transfers")
      ? [{ id: "transfers" as const, title: "传输与任务历史", description: "查看历史上传校验、整理计划与回执", icon: ListTodo }]
      : []),
  ];

  const securityEntries: HubEntry[] = [
    ...(canVisit("teams") && (teamEnabled || governanceEnabled)
      ? [{ id: "teams" as const, title: "团队协作空间", description: "管理共享团队网盘、成员邀请与配额", icon: Users }]
      : []),
    ...(canVisit("governance") && governanceEnabled
      ? [{ id: "governance" as const, title: "空间审批与治理", description: "处理访问申请、审计事件与租户策略", icon: ShieldCheck }]
      : []),
    ...(canVisit("capabilities")
      ? [{ id: "capabilities" as const, title: "服务连接自检", description: "检查当前服务端已启用的存储与能力", icon: CircleHelp }]
      : []),
  ];

  const hasHub = Boolean(onNavigate) && (storageEntries.length > 0 || aiAndToolsEntries.length > 0 || securityEntries.length > 0);
  const outline = [
    ["appearance", "外观与偏好"],
    ["profile", "个人资料"],
    ...(hasHub ? ([["hub", "空间与高级管理"]] as const) : []),
    ["account", "账号与凭据"],
  ] as const;

  useEffect(() => {
    const scroll = root.current?.closest(".content");
    if (!scroll) return;
    const track = () => {
      if (scroll.scrollTop + scroll.clientHeight >= scroll.scrollHeight - 2) {
        setActiveSection("account");
        return;
      }
      const top = scroll.getBoundingClientRect().top + 80;
      const visible = Array.from(
        root.current!.querySelectorAll<HTMLElement>("[data-settings-section]"),
      ).filter((section) => section.getBoundingClientRect().top <= top);
      setActiveSection(visible.at(-1)?.dataset.settingsSection || "appearance");
    };
    scroll.addEventListener("scroll", track, { passive: true });
    return () => scroll.removeEventListener("scroll", track);
  }, []);

  const jump = (section: string) => {
    const target = root.current?.querySelector<HTMLElement>(
      `[data-settings-section="${section}"]`,
    );
    target?.scrollIntoView({ block: "start", behavior: "auto" });
    target?.focus({ preventScroll: true });
    setActiveSection(section);
  };

  const { preferences, resolvedTheme, setAppearance, storageError } =
    useAppearance();
  const [copyState, setCopyState] = useState<"idle" | "copied" | "failed">("idle");
  const [displayName, setDisplayName] = useState(user.display_name ?? user.name);
  const [avatarRef, setAvatarRef] = useState(user.avatar_ref ?? "");
  const [profileState, setProfileState] = useState<"idle" | "saving" | "saved" | "error">("idle");
  const [profileError, setProfileError] = useState("");
  const [avatarOptions, setAvatarOptions] = useState<ComboboxOption[]>([]);
  const [avatarQuery, setAvatarQuery] = useState("");
  const [avatarLoading, setAvatarLoading] = useState(false);
  const [avatarError, setAvatarError] = useState("");
  const [avatarRetry, setAvatarRetry] = useState(0);

  useEffect(() => {
    const controller = new AbortController();
    const timer = setTimeout(() => {
      setAvatarLoading(true);
      setAvatarError("");
      api<{ items: ComboboxOption[] }>(
        `/me/avatar-options?q=${encodeURIComponent(avatarQuery)}`,
        "GET",
        undefined,
        {},
        controller.signal,
      )
        .then((out) => {
          if (!controller.signal.aborted) setAvatarOptions(out.items);
        })
        .catch((error) => {
          if (!controller.signal.aborted) setAvatarError(errorText(error));
        })
        .finally(() => {
          if (!controller.signal.aborted) setAvatarLoading(false);
        });
    }, 250);
    return () => {
      clearTimeout(timer);
      controller.abort();
    };
  }, [avatarQuery, avatarRetry]);

  const nameInvalid =
    [...displayName.trim()].length < 1 ||
    [...displayName.trim()].length > 80 ||
    /[\u0000-\u001f\u007f]/.test(displayName);
  const profileDirty =
    displayName.trim() !== (user.display_name ?? user.name) ||
    avatarRef.trim() !== (user.avatar_ref ?? "");

  async function saveProfile(event: React.FormEvent) {
    event.preventDefault();
    const nextName = displayName.trim();
    const nextAvatar = avatarRef.trim();
    if (nameInvalid) {
      setProfileError("显示名称需要 1–80 个字符。");
      setProfileState("error");
      return;
    }

    setProfileState("saving");
    setProfileError("");
    try {
      const out = await api<{ user: UserIdentity }>("/me", "PATCH", {
        display_name: nextName,
        avatar_ref: nextAvatar,
      });
      onUserChange(out.user);
      setDisplayName(out.user.display_name ?? out.user.name);
      setAvatarRef(out.user.avatar_ref ?? "");
      setProfileState("saved");
    } catch (error) {
      setProfileError(errorText(error));
      setProfileState("error");
    }
  }

  function renderHubGrid(items: HubEntry[]) {
    return (
      <div className="grid gap-2.5 sm:grid-cols-2">
        {items.map(({ id, title, description, icon: Icon }) => (
          <Button
            key={id}
            layout="surface"
            variant="ghost"
            type="button"
            onClick={() => {
              if (id === "drives") onOpenAllDrives?.();
              else onNavigate?.(id);
            }}
            className="flex h-auto min-h-14 w-full items-start justify-between gap-3 rounded-lg border border-border bg-background p-3 text-left transition-colors hover:border-input hover:bg-muted/40"
          >
            <div className="flex min-w-0 items-start gap-2.5">
              <span className="mt-0.5 flex size-8 shrink-0 items-center justify-center rounded-md bg-secondary text-foreground">
                <Icon className="size-4" />
              </span>
              <span className="grid min-w-0 gap-0.5 whitespace-normal">
                <strong className="text-[13px] font-semibold text-foreground">{title}</strong>
                <span className="text-xs leading-relaxed text-muted-foreground">{description}</span>
              </span>
            </div>
            <ArrowUpRight className="mt-1 size-4 shrink-0 text-muted-foreground" aria-hidden="true" />
          </Button>
        ))}
      </div>
    );
  }

  return (
    <div
      ref={root}
      className="settings-shell grid min-w-0 w-full gap-4 text-foreground max-md:[&_button:not([role=radio]):not([role=switch])]:min-h-11 max-md:[&_input:not([type=hidden])]:min-h-11"
      data-testid="personal-settings"
    >
      <WorkspacePageHeader title="个人设置" />
      <div className="settings-layout">
        <nav className="settings-outline" aria-label="设置大纲">
          <p className="mb-2 text-xs font-medium text-muted-foreground">设置与管理</p>
          <div className="settings-outline-items">
            {outline.map(([id, title]) => (
              <Button
                key={id}
                type="button"
                variant={activeSection === id ? "secondary" : "ghost"}
                className="min-h-11 justify-start rounded-lg text-[13px] md:min-h-9"
                aria-current={activeSection === id ? "location" : undefined}
                onClick={() => jump(id)}
              >
                {title}
              </Button>
            ))}
          </div>
        </nav>

        <div className="settings-sections grid min-w-0 divide-y divide-border">
          <Card
            data-settings-section="appearance"
            role="region"
            aria-label="外观设置"
            tabIndex={-1}
            className="scroll-mt-4 rounded-none border-0 bg-transparent px-0 py-6 shadow-none first:pt-0 focus-visible:outline-2 focus-visible:outline-ring"
          >
            <CardHeader className="px-0">
              <CardTitle className="flex items-center gap-2 text-base">
                <Palette className="size-4 text-muted-foreground" />
                外观与浏览偏好
              </CardTitle>
              <CardDescription>
                定制浅色或深色界面外观，修改立即在当前设备生效。
              </CardDescription>
            </CardHeader>
            <CardContent className="grid gap-5 px-0">
              <RadioGroup
                aria-label="主题模式"
                value={preferences.theme}
                onValueChange={(value) => setAppearance({ theme: value as Theme })}
                className="grid gap-2.5 sm:grid-cols-3"
              >
                {themes.map(({ value, title, hint, icon: Icon }) => (
                  <label
                    key={value}
                    className="flex min-h-14 cursor-pointer items-center gap-3 rounded-lg border border-border bg-card px-3.5 py-2.5 transition-colors has-data-checked:border-primary has-data-checked:bg-accent hover:bg-muted/50"
                  >
                    <div className="flex shrink-0 items-center gap-2">
                      <Icon className="hidden size-4 text-muted-foreground lg:block" />
                      <RadioGroupItem value={value} />
                    </div>
                    <span className="grid gap-0.5">
                      <strong className="text-sm font-medium">{title}</strong>
                      <span className="text-xs text-muted-foreground">{hint}</span>
                    </span>
                  </label>
                ))}
              </RadioGroup>
              <Button
                type="button"
                variant="outline"
                className="justify-self-start"
                onClick={() => setAppearance(defaultAppearance)}
              >
                恢复默认外观
              </Button>
              {storageError && (
                <p role="alert" className="text-sm text-destructive">
                  {storageError}
                </p>
              )}
              <p className="text-xs text-muted-foreground" role="status">
                当前生效：{resolvedTheme === "dark" ? "深色" : "浅色"} · 修改立即生效
              </p>
              <div className="grid gap-3 border-t border-border pt-5">
                <h3 className="text-sm font-medium">列表密度</h3>
                <RadioGroup
                  aria-label="列表密度"
                  value={preferences.density}
                  onValueChange={(value) =>
                    setAppearance({
                      density: value as "comfortable" | "compact",
                    })
                  }
                  className="flex flex-wrap gap-6"
                >
                  <label className="flex min-h-11 cursor-pointer items-center gap-2 text-sm md:min-h-8">
                    <RadioGroupItem value="comfortable" />
                    舒适
                  </label>
                  <label className="flex min-h-11 cursor-pointer items-center gap-2 text-sm md:min-h-8">
                    <RadioGroupItem value="compact" />
                    紧凑
                  </label>
                </RadioGroup>
                <p className="text-xs text-muted-foreground">
                  调整文件列表与表格的行间距，不改变文件内容。
                </p>
              </div>
              <div className="flex items-center justify-between gap-4 border-t border-border pt-5">
                <label htmlFor="reduce-motion" className="grid cursor-pointer gap-1">
                  <strong className="text-sm font-medium">减少动效</strong>
                  <span className="text-xs text-muted-foreground">
                    关闭非必要界面过渡；未开启时自动遵循系统动效设置。
                  </span>
                </label>
                <Switch
                  id="reduce-motion"
                  checked={preferences.motion === "reduced"}
                  onCheckedChange={(checked) =>
                    setAppearance({ motion: checked ? "reduced" : "system" })
                  }
                />
              </div>
            </CardContent>
          </Card>

          <Card
            data-settings-section="profile"
            role="region"
            aria-label="个人资料设置"
            tabIndex={-1}
            className="scroll-mt-4 rounded-none border-0 bg-transparent px-0 py-6 shadow-none focus-visible:outline-2 focus-visible:outline-ring"
          >
            <CardHeader className="px-0">
              <CardTitle className="flex items-center gap-2 text-base">
                <UserRound className="size-4 text-muted-foreground" />
                个人资料
              </CardTitle>
              <CardDescription>
                资料保存到当前账号，在其他设备登录自动同步。
              </CardDescription>
            </CardHeader>
            <CardContent className="px-0">
              <form className="grid gap-4" onSubmit={saveProfile}>
                <UserAvatar user={user} />
                <label className="grid gap-2 text-sm font-medium">
                  显示名称
                  <Input
                    value={displayName}
                    aria-invalid={nameInvalid}
                    disabled={profileState === "saving"}
                    onChange={(event) => {
                      setDisplayName(event.target.value);
                      setProfileError("");
                      setProfileState("idle");
                    }}
                    aria-describedby="display-name-help"
                  />
                </label>
                <p id="display-name-help" className="text-xs text-muted-foreground">
                  1–80 个字符，不会改变登录 ID。
                  {nameInvalid && "请输入有效的显示名称。"}
                </p>
                <div className="grid gap-2">
                  <span className="text-sm font-medium">头像</span>
                  <ComboboxField
                    aria-label="选择头像"
                    disabled={profileState === "saving"}
                    value={avatarRef}
                    onValueChange={(value) => {
                      setAvatarRef(value);
                      setProfileState("idle");
                    }}
                    options={avatarOptions}
                    onSearchChange={setAvatarQuery}
                    loading={avatarLoading}
                    error={avatarError}
                    onRetry={() => setAvatarRetry((n) => n + 1)}
                    placeholder={
                      avatarRef ? "已选头像（可更换）" : "从个人网盘选择图片"
                    }
                  />
                  <p className="text-xs text-muted-foreground">
                    选择不超过 2 MB 的 PNG、JPEG、WebP 或 GIF；可清空以恢复默认头像。
                  </p>
                </div>
                {profileError && (
                  <p role="alert" className="text-sm text-destructive">
                    {profileError}
                  </p>
                )}
                {profileState === "saved" && (
                  <p role="status" className="text-sm text-[var(--success)]">
                    个人资料已保存。
                  </p>
                )}
                <div className="flex flex-wrap items-center gap-3">
                  <Button
                    type="submit"
                    disabled={!profileDirty || nameInvalid || profileState === "saving"}
                  >
                    {profileState === "saving" ? "正在保存…" : "保存资料"}
                  </Button>
                  {profileDirty && (
                    <span className="text-xs text-muted-foreground">有未保存修改</span>
                  )}
                </div>
              </form>
            </CardContent>
          </Card>

          {hasHub && (
            <Card
              data-settings-section="hub"
              role="region"
              aria-label="空间与高级管理"
              tabIndex={-1}
              className="scroll-mt-4 rounded-none border-0 bg-transparent px-0 py-6 shadow-none focus-visible:outline-2 focus-visible:outline-ring"
            >
              <CardHeader className="px-0">
                <CardTitle className="flex items-center gap-2 text-base">
                  <SlidersHorizontal className="size-4 text-muted-foreground" />
                  空间、AI 助手与高级工具
                </CardTitle>
                <CardDescription>
                  集中管理网盘存储挂载、AI 助手文件夹授权、自动整理规则与团队权限。
                </CardDescription>
              </CardHeader>
              <CardContent className="grid gap-6 px-0">
                {storageEntries.length > 0 && (
                  <div className="grid gap-2.5">
                    <h3 className="text-xs font-semibold tracking-wide text-muted-foreground">
                      云盘与存储挂载
                    </h3>
                    {renderHubGrid(storageEntries)}
                  </div>
                )}
                {aiAndToolsEntries.length > 0 && (
                  <div className="grid gap-2.5 border-t border-border pt-5">
                    <h3 className="text-xs font-semibold tracking-wide text-muted-foreground">
                      AI 协作与整理工具
                    </h3>
                    {renderHubGrid(aiAndToolsEntries)}
                  </div>
                )}
                {securityEntries.length > 0 && (
                  <div className="grid gap-2.5 border-t border-border pt-5">
                    <h3 className="text-xs font-semibold tracking-wide text-muted-foreground">
                      团队与服务诊断
                    </h3>
                    {renderHubGrid(securityEntries)}
                  </div>
                )}
              </CardContent>
            </Card>
          )}

          <Card
            data-settings-section="account"
            role="region"
            aria-label="账号信息设置"
            tabIndex={-1}
            className="scroll-mt-4 rounded-none border-0 bg-transparent px-0 py-6 shadow-none focus-visible:outline-2 focus-visible:outline-ring"
          >
            <CardHeader className="px-0">
              <CardTitle className="flex items-center gap-2 text-base">
                <KeyRound className="size-4 text-muted-foreground" />
                账号与访问凭据
              </CardTitle>
              <CardDescription>
                当前登录身份与个人登录密钥管理，访问范围由服务端裁决。
              </CardDescription>
            </CardHeader>
            <CardContent className="grid gap-4 px-0">
              <dl className="grid gap-4 text-sm sm:grid-cols-[5rem_minmax(0,1fr)]">
                <dt className="text-muted-foreground">显示名称</dt>
                <dd className="m-0 font-medium">
                  {user.display_name || user.name || "未设置名称"}
                </dd>
                <dt className="text-muted-foreground">用户 ID</dt>
                <dd className="m-0 min-w-0">
                  <code className="break-all text-xs">{user.id}</code>
                  <Button
                    variant="outline"
                    size="sm"
                    className="mt-2 flex"
                    onClick={async () => {
                      try {
                        await navigator.clipboard.writeText(user.id);
                        setCopyState("copied");
                      } catch {
                        setCopyState("failed");
                      }
                    }}
                  >
                    {copyState === "copied" ? <Check /> : <Copy />}
                    {copyState === "copied" ? "已复制用户 ID" : "复制用户 ID"}
                  </Button>
                </dd>
              </dl>
              {copyState === "failed" && (
                <p role="alert" className="text-sm text-destructive">
                  无法使用剪贴板，请选择上方用户 ID 手动复制。
                </p>
              )}
              <div className="flex flex-wrap items-center justify-between gap-3 border-t border-border pt-4">
                <span className="text-sm text-muted-foreground">
                  查看或轮换个人登录密钥
                </span>
                <Button variant="outline" onClick={openKeys}>
                  <KeyRound />
                  访问凭据
                </Button>
              </div>
            </CardContent>
          </Card>
        </div>
      </div>
    </div>
  );
}
