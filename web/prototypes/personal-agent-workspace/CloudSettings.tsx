import { useEffect, useRef, useState } from "react";
import { matchingSettings, settingItems } from "./settings-catalog";
import { ArrowLeft, ArrowRight, Settings2 } from "lucide-react";
import { Input } from "@/components/ui/input";
import { Checkbox } from "@/components/ui/checkbox";
import { IndexSettings } from "./SearchWorkspace";
import { settingCategories } from "./settings-catalog";
import { Btn, Choice, Heading, panel, useDemo } from "./ui";
function Toggle({
  label,
  note,
  checked,
  onChange,
}: {
  label: string;
  note: string;
  checked: boolean;
  onChange: (v: boolean) => void;
}) {
  return (
    <div
      data-setting={label}
      tabIndex={-1}
      className="flex min-h-12 scroll-mt-24 items-center gap-3 border-b border-border py-2"
    >
      <div className="min-w-0 flex-1">
        <p className="text-sm font-medium">
          {label}
          <span className="ml-2 text-[11px] font-normal text-muted-foreground">
            {settingItems.find((x) => x.target === label)?.scope}
          </span>
        </p>
        <p className="mt-1 text-xs leading-6 text-muted-foreground">{note}</p>
      </div>
      <Checkbox
        aria-label={label}
        checked={checked}
        onCheckedChange={(v) => onChange(!!v)}
      />
    </div>
  );
}
export function SettingsWorkspace() {
  const { state: s, dispatch, go, open, compact } = useDemo();
  const root = useRef<HTMLDivElement>(null);
  const [narrow, setNarrow] = useState(compact);
  const [highlight, setHighlight] = useState("");
  useEffect(() => {
    const el = root.current;
    if (!el) return;
    const observer = new ResizeObserver((entries) =>
      setNarrow(entries[0].contentRect.width < 720),
    );
    observer.observe(el);
    return () => observer.disconnect();
  }, []);
  useEffect(() => {
    if (!highlight) return;
    const target = Array.from(
      root.current?.querySelectorAll<HTMLElement>("[data-setting]") || [],
    ).find((el) => el.dataset.setting === highlight);
    if (target) {
      target.scrollIntoView({ block: "nearest" });
      target.focus({ preventScroll: true });
    }
  }, [highlight, s.settingSection]);
  const section = s.settingSection || "general";
  const results = matchingSettings(s.settingsQuery);

  const p = s.preferences;
  const opts = s.options;
  const pref = (patch: Partial<typeof p>) =>
    dispatch({ type: "preferences", patch });
  const option = (patch: Partial<typeof opts>) =>
    dispatch({ type: "options", patch });
  const categories = settingCategories;
  const links = [
    { id: "collections", name: "项目集合" },
    { id: "tags", name: "标签" },
    { id: "inbox", name: "产物与收件" },
    { id: "collaboration", name: "共享与协作" },
    { id: "shares", name: "分享与交付" },
    { id: "agents", name: "外部调用" },
    { id: "connections", name: "存储管理" },
    { id: "devices", name: "设备与离线" },
    { id: "automation", name: "备份与整理" },
    { id: "trash", name: "回收站" },
    { id: "capabilities", name: "能力与连接说明" },
  ] as const;
  return (
    <div ref={root}>
      <Heading
        title={compact ? "我的" : "设置"}
        note="本设备偏好、空间设置与外部授权分别管理。"
      />
      <div className="sticky top-0 z-10 bg-background py-2">
        <Input
          aria-label="搜索设置"
          className="mb-5 h-11"
          value={s.settingsQuery}
          onChange={(e) =>
            dispatch({ type: "settings-query", query: e.target.value })
          }
          placeholder="搜索设置，例如：索引、缓存、主题"
        />
      </div>
      <div
        className={
          narrow ? "" : "grid grid-cols-[200px_minmax(0,1fr)] items-start gap-7"
        }
      >
        {(!narrow || !s.settingSection || s.settingsQuery) && (
          <nav aria-label="设置分类" className="grid gap-1 self-start">
            {categories.map((c) => (
              <Btn
                key={c.id}
                aria-label={c.name}
                variant={section === c.id ? "secondary" : "ghost"}
                className="!h-11 !justify-between !px-3"
                onClick={() => {
                  dispatch({ type: "settings-section", section: c.id });
                  dispatch({ type: "settings-query", query: "" });
                  setHighlight("");
                }}
              >
                {c.name}
                {s.settingsQuery ? (
                  <small>
                    {results.filter((r) => r.section === c.id).length}
                  </small>
                ) : narrow ? (
                  <ArrowRight size={14} />
                ) : null}
              </Btn>
            ))}
          </nav>
        )}
        <div className="min-w-0">
          {s.settingsQuery && (
            <section aria-label="设置搜索结果" className="space-y-2">
              <p className="mb-3 text-xs text-muted-foreground">
                {results.length} 个设置匹配
              </p>
              {results.map((r) => (
                <Btn
                  key={r.title}
                  variant="outline"
                  className="!h-auto !min-h-20 !w-full !justify-between !whitespace-normal text-left"
                  onClick={() => {
                    dispatch({ type: "settings-section", section: r.section });
                    dispatch({ type: "settings-query", query: "" });
                    setHighlight(r.target);
                  }}
                >
                  <span>
                    <strong className="block text-sm">{r.title}</strong>
                    <span className="mt-1 block text-xs text-muted-foreground">
                      {r.scope} · {r.keywords}
                    </span>
                  </span>
                  <ArrowRight size={15} />
                </Btn>
              ))}
              {!results.length && (
                <p className="py-5 text-sm">没有匹配的设置，请更换关键词。</p>
              )}
            </section>
          )}
          {narrow && !s.settingSection && !s.settingsQuery && (
            <section className={`${panel} divide-y divide-border`}>
              <p className="p-4 text-sm font-semibold">
                Yeisme · {s.space === "personal" ? "个人空间" : "团队空间"}
              </p>
              <div className="p-4">
                <Choice
                  label="切换空间"
                  value={s.space}
                  onChange={(v) =>
                    dispatch({ type: "space", space: v as typeof s.space })
                  }
                  options={[
                    { value: "personal", label: "个人空间" },
                    { value: "team", label: "团队空间" },
                  ]}
                  className="w-full"
                />
              </div>
              {links.map((l) => (
                <Btn
                  key={l.id}
                  variant="ghost"
                  className="!h-14 !w-full !justify-between !rounded-none px-4"
                  onClick={() => go(l.id)}
                >
                  {l.name}
                  <ArrowRight size={15} />
                </Btn>
              ))}
            </section>
          )}
          {(!narrow || s.settingSection) && !s.settingsQuery && (
            <section className="max-w-4xl space-y-3 [&_[data-setting]:focus]:bg-accent [&_[data-setting]:focus]:outline-2">
              {narrow && (
                <Btn
                  variant="ghost"
                  onClick={() =>
                    dispatch({ type: "settings-section", section: "" })
                  }
                >
                  <ArrowLeft size={16} />
                  返回设置分类
                </Btn>
              )}
              <div className="flex items-center gap-3">
                <Settings2 size={19} className="text-primary" />
                <h2 className="text-sm font-semibold">
                  {settingCategories.find((c) => c.id === section)?.name}
                </h2>
                <span className="ml-auto text-xs text-muted-foreground">
                  {section === "general" ? "本设备偏好" : "演示空间设置"}
                </span>
              </div>
              {s.settingsNotice && (
                <p role="status" className="rounded-lg bg-accent p-3 text-sm">
                  {s.settingsNotice}
                </p>
              )}
              {section === "editor" && (
                <>
                  <Toggle
                    label="Markdown 自动保存"
                    note="输入停止 800ms 后保存。只保存在本轮演示，中文输入法组合期间暂停。"
                    checked={s.knowledge.options.autosave}
                    onChange={(autosave) =>
                      dispatch({ type: "k-options", patch: { autosave } })
                    }
                  />
                  <Setting label="默认编辑模式">
                    <Choice
                      label="默认 Markdown 模式"
                      value={s.knowledge.options.defaultMode}
                      onChange={(defaultMode) =>
                        dispatch({
                          type: "k-options",
                          patch: {
                            defaultMode: defaultMode as
                              | "live"
                              | "source"
                              | "read",
                          },
                        })
                      }
                      options={[
                        { value: "live", label: "实时编辑" },
                        { value: "source", label: "源码" },
                        { value: "read", label: "阅读" },
                      ]}
                    />
                  </Setting>
                  <p className="text-xs leading-6 text-muted-foreground">
                    Ctrl/Cmd+S 保存，Ctrl/Cmd+F 查找，编辑区支持撤销与重做。
                  </p>
                </>
              )}
              {section === "links" && (
                <>
                  <Setting label="附件默认位置">
                    <Choice
                      label="附件目录"
                      value={s.knowledge.options.attachmentFolder}
                      onChange={(attachmentFolder) =>
                        dispatch({
                          type: "k-options",
                          patch: { attachmentFolder },
                        })
                      }
                      options={[
                        { value: "知识库 / 附件", label: "知识库 / 附件" },
                        { value: "待整理", label: "待整理" },
                      ]}
                    />
                  </Setting>
                  <Setting label="链接格式">
                    <Choice
                      label="默认链接格式"
                      value={s.knowledge.options.linkFormat}
                      onChange={(linkFormat) =>
                        dispatch({
                          type: "k-options",
                          patch: {
                            linkFormat: linkFormat as "wiki" | "markdown",
                          },
                        })
                      }
                      options={[
                        { value: "wiki", label: "双链 [[文档]]" },
                        { value: "markdown", label: "标准 Markdown 链接" },
                      ]}
                    />
                  </Setting>
                  <p className="text-xs leading-6 text-muted-foreground">
                    链接移动与重命名先预览影响。删除引用不会删除附件。
                  </p>
                </>
              )}
              {section === "general" && (
                <>
                  <div className="grid gap-4 min-[900px]:grid-cols-2">
                    <Setting label="外观">
                      <Choice
                        label="外观主题"
                        value={p.theme}
                        onChange={(v) => pref({ theme: v as typeof p.theme })}
                        options={[
                          { value: "system", label: "跟随系统" },
                          { value: "light", label: "浅色" },
                          { value: "dark", label: "深色" },
                        ]}
                        className="w-full"
                      />
                    </Setting>
                    <Setting label="界面密度">
                      <Choice
                        label="界面密度"
                        value={p.density}
                        onChange={(v) =>
                          pref({ density: v as typeof p.density })
                        }
                        options={[
                          { value: "comfortable", label: "舒适" },
                          { value: "compact", label: "紧凑" },
                        ]}
                        className="w-full"
                      />
                    </Setting>
                    <Setting label="默认文件视图">
                      <Choice
                        label="默认文件视图"
                        value={p.layout}
                        onChange={(v) => pref({ layout: v as typeof p.layout })}
                        options={[
                          { value: "list", label: "列表" },
                          { value: "grid", label: "网格" },
                          { value: "gallery", label: "大图" },
                        ]}
                        className="w-full"
                      />
                    </Setting>
                    <Setting label="启动页">
                      <Choice
                        label="启动页"
                        value={p.startup}
                        onChange={(v) =>
                          pref({ startup: v as typeof p.startup })
                        }
                        options={[
                          { value: "files", label: "我的云盘" },
                          { value: "home", label: "首页" },
                        ]}
                        className="w-full"
                      />
                    </Setting>
                  </div>
                  <Toggle
                    label="减少动效"
                    note="始终尊重系统减少动效；开启后进一步关闭本原型过渡。"
                    checked={p.reduceMotion}
                    onChange={(v) => pref({ reduceMotion: v })}
                  />
                  <p
                    data-setting="快捷键"
                    tabIndex={-1}
                    className="text-sm leading-7"
                  >
                    快捷键：Ctrl / ⌘ K 搜索；文件区 Ctrl / ⌘ A
                    选择当前页；Escape 先关闭菜单，再退出多选。中文界面。
                  </p>
                  <Btn
                    variant="outline"
                    onClick={() => dispatch({ type: "settings-reset" })}
                  >
                    恢复默认偏好
                  </Btn>
                </>
              )}
              {section === "files" && (
                <>
                  <Setting label="默认上传目录">
                    <Choice
                      label="默认上传目录"
                      value={opts.uploadTarget}
                      onChange={(uploadTarget) => option({ uploadTarget })}
                      options={s.folders
                        .filter((f) => f.space === s.space)
                        .map((f) => ({ value: f.path, label: f.path }))}
                      className="w-full"
                    />
                  </Setting>
                  <p
                    data-setting="同名冲突"
                    tabIndex={-1}
                    className="rounded-lg bg-muted p-4 text-sm"
                  >
                    同名冲突：每次询问。覆盖内容始终单独确认。
                  </p>
                  <Toggle
                    label="仅在 Wi-Fi 下传输"
                    note="原生网络判断待接入，本轮只影响演示规则说明。"
                    checked={opts.wifiOnly}
                    onChange={(wifiOnly) => option({ wifiOnly })}
                  />
                  <Toggle
                    label="传输完成与异常提醒"
                    note="开启后在应用内任务入口显示演示提醒。"
                    checked={opts.notifyTransfers}
                    onChange={(notifyTransfers) => option({ notifyTransfers })}
                  />
                  <Btn variant="outline" onClick={() => go("connections")}>
                    查看传输与连接
                  </Btn>
                </>
              )}
              {section === "tags" && (
                <>
                  <Btn
                    data-setting="个人收藏"
                    onClick={() =>
                      dispatch({
                        type: "favorites-open",
                        connectionId: s.activeConnection,
                      })
                    }
                  >
                    管理当前连接的个人收藏
                  </Btn>
                  <Btn data-setting="属性定义" onClick={() => go("tags")}>
                    管理 Markdown 属性定义与类型
                  </Btn>
                  <Btn
                    data-setting="标签分组、颜色与合并"
                    onClick={() => go("tags")}
                  >
                    管理标签分组、颜色与合并
                  </Btn>
                  <Toggle
                    label="显示 Agent 标签建议"
                    note="建议不会自动成为人工标签；关闭仅隐藏建议显示。"
                    checked={opts.showSuggestions}
                    onChange={(showSuggestions) => option({ showSuggestions })}
                  />
                  <h3
                    data-setting="已保存搜索视图"
                    tabIndex={-1}
                    className="font-semibold"
                  >
                    已保存搜索视图
                  </h3>
                  {s.savedSearches
                    .filter((v) => v.space === s.space)
                    .map((v) => (
                      <div
                        key={v.id}
                        className="flex min-h-12 items-center gap-2 border-b border-border"
                      >
                        <Btn
                          variant="link"
                          onClick={() =>
                            dispatch({ type: "search-open", id: v.id })
                          }
                        >
                          {v.name}
                        </Btn>
                        <Btn
                          variant="ghost"
                          className="ml-auto"
                          onClick={() =>
                            dispatch({ type: "search-delete", id: v.id })
                          }
                        >
                          移除视图
                        </Btn>
                      </div>
                    ))}
                  <Btn variant="outline" onClick={() => go("collections")}>
                    查看项目集合
                  </Btn>
                </>
              )}
              {section === "search" && (
                <>
                  <Btn
                    data-setting="Markdown 属性提取"
                    onClick={() =>
                      dispatch({
                        type: "storage-page",
                        tab: "indexes",
                        directoryId: s.storage.activeId,
                      })
                    }
                  >
                    管理文首属性与正文标签提取
                  </Btn>
                  <Setting label="默认搜索目录">
                    <Choice
                      label="默认搜索目录"
                      value={opts.searchFolder}
                      onChange={(searchFolder) => option({ searchFolder })}
                      options={[
                        { value: "全部文件", label: "当前连接根目录" },
                        ...s.storage.nodes
                          .filter(
                            (n) =>
                              n.space === s.space &&
                              n.connectionId === s.activeConnection,
                          )
                          .map((n) => ({ value: n.id, label: n.path })),
                      ]}
                      className="w-full"
                    />
                  </Setting>
                  <div data-setting="内容索引" tabIndex={-1}>
                    <p className="text-xs leading-6">
                      目录元数据与内容分析独立授权，默认手动更新。配置范围与运行记录在存储管理中统一维护。
                    </p>
                    <Btn
                      onClick={() =>
                        dispatch({ type: "storage-page", tab: "indexes" })
                      }
                    >
                      管理目录与内容索引
                    </Btn>
                  </div>
                </>
              )}
              {section === "connections" && (
                <>
                  <div data-setting="关联资源确认" tabIndex={-1}>
                    <p className="text-sm leading-6">
                      复制外部引用不会扩大授权。Skill
                      的结构检查与运行能力分别核对；关联文档及附件需明确加入范围。
                    </p>
                    <Btn variant="outline" onClick={() => go("resources")}>
                      管理资源与 Skills
                    </Btn>
                  </div>
                  <Toggle
                    label="任务完成与待确认提醒"
                    note="通知返回原操作。"
                    checked={opts.notifyTasks}
                    onChange={(notifyTasks) => option({ notifyTasks })}
                  />

                  <p className="text-sm leading-7">
                    存储连接与调用应用分别授权。已连接不等于已授权，读取验证不证明写入可用。
                  </p>
                  <div className="flex flex-wrap gap-2">
                    <Btn
                      data-setting="MCP / API 外部调用"
                      onClick={() => go("agents")}
                    >
                      管理 MCP / API 外部调用
                    </Btn>
                    <Btn
                      data-setting="存储连接"
                      variant="outline"
                      onClick={() => go("connections")}
                    >
                      管理存储连接
                    </Btn>
                    <Btn variant="outline" onClick={() => go("capabilities")}>
                      能力与连接说明
                    </Btn>
                  </div>
                </>
              )}
              {section === "devices" && (
                <>
                  <p data-setting="离线缓存" tabIndex={-1} className="text-sm">
                    当前保存{" "}
                    {
                      s.files.filter(
                        (f) => f.space === s.space && f.offlineVersion,
                      ).length
                    }{" "}
                    个模拟离线副本。
                  </p>
                  <div className="flex flex-wrap gap-2">
                    <Btn
                      variant="outline"
                      onClick={() =>
                        open({
                          type: "danger",
                          title: "清除模拟离线缓存？",
                          text: "云端文件、标签和历史版本保留。本操作不清理浏览器以外的真实设备数据。",
                          action: { type: "cache-clear" },
                        })
                      }
                    >
                      清理离线缓存
                    </Btn>
                    <Btn variant="outline" onClick={() => go("devices")}>
                      查看设备与离线版本
                    </Btn>
                    <Btn
                      data-setting="后台限制"
                      variant="ghost"
                      onClick={() => go("capabilities")}
                    >
                      后台传输与通知限制
                    </Btn>
                  </div>
                  <p className="text-sm leading-7 text-muted-foreground">
                    设备是否关联：
                    {s.deviceLinked ? "演示设备已关联" : "尚未关联"}
                    。系统相册、后台传输与原生通知需客户端能力；Web
                    不假装具备这些能力。
                  </p>
                  {s.deviceLinked && (
                    <Btn
                      variant="outline"
                      onClick={() =>
                        open({
                          type: "danger",
                          title: "解除演示设备关联？",
                          text: "不会删除云端文件，也不代替撤销外部应用授权。",
                          action: { type: "unlink-device" },
                        })
                      }
                    >
                      解除设备关联
                    </Btn>
                  )}
                </>
              )}
              {section === "collaboration" && (
                <>
                  <Setting label="分享有效期">
                    <Choice
                      label="分享有效期"
                      value={String(s.collaborationDefaults.days)}
                      onChange={(v) =>
                        dispatch({
                          type: "collaboration-defaults",
                          patch: { days: Number(v) },
                        })
                      }
                      options={[1, 7, 30].map((v) => ({
                        value: String(v),
                        label: `${v} 天`,
                      }))}
                    />
                  </Setting>
                  <Toggle
                    label="默认允许下载"
                    note="当前空间：仅影响新建分享，旧分享保持原设置。"
                    checked={s.collaborationDefaults.download}
                    onChange={(download) =>
                      dispatch({
                        type: "collaboration-defaults",
                        patch: { download },
                      })
                    }
                  />
                  <Toggle
                    label="成员提及提醒"
                    note="本设备：仅站内演示，不发送外部消息，不增加权限。"
                    checked={s.collaborationDefaults.mentions}
                    onChange={(mentions) =>
                      dispatch({
                        type: "collaboration-defaults",
                        patch: { mentions },
                      })
                    }
                  />
                  <Btn onClick={() => go("collaboration")}>
                    管理共享、成员与邀请
                  </Btn>
                </>
              )}
              {section === "account" && (
                <>
                  <p
                    data-setting="身份与空间"
                    tabIndex={-1}
                    className="text-sm"
                  >
                    Yeisme · {s.space === "personal" ? "个人空间" : "团队空间"}{" "}
                    · 演示身份
                  </p>
                  <Choice
                    label="切换空间"
                    value={s.space}
                    onChange={(v) =>
                      dispatch({ type: "space", space: v as typeof s.space })
                    }
                    options={[
                      { value: "personal", label: "个人空间" },
                      { value: "team", label: "团队空间" },
                    ]}
                  />
                  <p className="text-sm leading-7 text-muted-foreground">
                    当前会话为本页内存演示。真实登录、登录会话撤销和账户退出尚未接入，重置仅清除本页样本状态。
                  </p>
                  <div className="flex flex-wrap gap-2">
                    <Btn
                      variant="outline"
                      onClick={() => open({ type: "capacity" })}
                    >
                      容量分析
                    </Btn>
                    <Btn variant="outline" onClick={() => go("devices")}>
                      设备与会话说明
                    </Btn>
                    <Btn
                      variant="ghost"
                      onClick={() =>
                        open({
                          type: "danger",
                          title: "重置本页演示？",
                          text: "清除模拟文件修改、会话与授权，恢复样本。不是退出真实账户。",
                          action: { type: "reset" },
                        })
                      }
                    >
                      重置演示会话
                    </Btn>
                  </div>
                </>
              )}
            </section>
          )}
        </div>
      </div>
    </div>
  );
}
function Setting({
  label,
  children,
}: {
  label: string;
  children: React.ReactNode;
}) {
  return (
    <div
      data-setting={label}
      tabIndex={-1}
      className="scroll-mt-24 space-y-2 rounded-lg p-1"
    >
      <p className="text-sm font-medium">
        {label}
        <span className="ml-2 text-[11px] font-normal text-muted-foreground">
          {settingItems.find((x) => x.target === label)?.scope}
        </span>
      </p>
      {children}
    </div>
  );
}
