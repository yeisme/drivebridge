import { fileRef } from "./preview-model";
import { FileButton } from "./HoverPreview";
import {
  Dialog,
  DialogContent,
  DialogTitle,
  DialogDescription,
} from "./Dialog";
import type { State } from "./model";
import { useState } from "react";
import {
  ArrowRight,
  Check,
  FileSearch,
  FolderSearch,
  Pause,
  Play,
  Plus,
  Search,
  X,
} from "lucide-react";
import { Input } from "@/components/ui/input";
import { Textarea } from "@/components/ui/textarea";
import { Checkbox } from "@/components/ui/checkbox";
import { searchResults, emptySearch, scopeFiles } from "./cloud-model";
import { AssetCard } from "./CreativeWorkspace";
import { FileToolbar } from "./CloudToolbar";
import { TagPicker } from "./TagWorkspace";
import { settingCategories, matchingSettings } from "./settings-catalog";
import {
  Btn,
  Choice,
  Empty,
  Heading,
  IconBtn,
  panel,
  Pill,
  useDemo,
} from "./ui";

export function IndexSettings() {
  const { state: s, dispatch, open } = useDemo();
  const node=s.storage.nodes.find(n=>n.id===s.storage.inspect && n.space===s.space)!;
  const folder=node?.path || "";
  const [excluded, setExcluded] = useState("私密,凭据");
  const [kinds, setKinds] = useState(["document", "image"]);
  const [policy, setPolicy] = useState<"manual" | "changes">("manual");
  const labels: Record<string, string> = {
    queued: "排队中",
    processing: "处理中",
    ready: "可用",
    partial: "部分失败",
    paused: "已暂停",
    stale: "已过期",
  };
  return (
    <div className="space-y-5">
      <h3 className="font-semibold">内容索引</h3>
      <p className="text-sm leading-7 text-muted-foreground">
        普通文件名和标签搜索不需要内容索引。连接存储不会自动开启内容分析。
      </p>
      <p className="text-sm">索引目录：{s.connections.find(c=>c.id===node?.connectionId)?.name} / {folder}</p>
      <Input
        aria-label="索引排除项"
        className="h-11"
        value={excluded}
        onChange={(e) => setExcluded(e.target.value)}
        placeholder="按路径或文件名包含词排除，以逗号分隔"
      />
      <div className="flex flex-wrap gap-2">
        {[
          { id: "document", name: "文档" },
          { id: "image", name: "图片" },
          { id: "video", name: "视频" },
          { id: "audio", name: "音频" },
          { id: "sheet", name: "表格" },
        ].map((k) => (
          <Btn
            key={k.id}
            variant={kinds.includes(k.id) ? "secondary" : "outline"}
            aria-pressed={kinds.includes(k.id)}
            onClick={() =>
              setKinds(
                kinds.includes(k.id)
                  ? kinds.filter((x) => x !== k.id)
                  : [...kinds, k.id],
              )
            }
          >
            {k.name}
          </Btn>
        ))}
      </div>
      <Choice
        label="索引更新策略"
        value={policy}
        onChange={(v) => setPolicy(v as typeof policy)}
        options={[
          { value: "manual", label: "手动更新" },
          { value: "changes", label: "授权目录变化后更新（演示）" },
        ]}
      />
      <div className="rounded-lg bg-accent p-4 text-sm leading-7">
        范围：{folder} 及子目录；排除：{excluded || "无"}
        <br />
        处理位置：浏览器内样本演示；未来服务端内容处理位置待接入。
        <br />
        本次无模型调用，费用为 0；真实服务费用未知，需接入后明确确认。
      </div>
      <Btn
        disabled={!kinds.length || !node || !s.online || s.connections.find(c=>c.id===node.connectionId)?.read!=="passed"}
        onClick={() =>
          open({
            type: "danger",
            title: "授权这个目录建立内容索引？",
            text: `仅分析「${folder}」及子目录，排除 ${excluded || "无"}。本次为无费用样本演示，不扩大 Agent 文件权限。`,
            action: {
              type: "index-create",
              folder,
              directoryId: node?.id,
              connectionId: node?.connectionId,
              exclusions: excluded
                .split(/[,，]/)
                .map((x) => x.trim())
                .filter(Boolean),
              kinds,
              policy,
            },
          })
        }
      >
        确认范围并建立索引
      </Btn>
      <div className="space-y-3">
        {!s.indexes.some((i) => i.space === s.space) && (
          <p className="text-sm text-muted-foreground">
            尚未启用任何内容索引。
          </p>
        )}
        {s.indexes
          .filter((i) => i.space === s.space && i.directoryId === node?.id)
          .map((i) => (
            <article key={i.id} className={`${panel} p-4`} data-index-id={i.id}>
              <div className="flex flex-wrap items-center justify-between gap-2">
                <strong className="text-sm">{i.folder}</strong>
                <Pill tone={i.status === "ready" ? "green" : "amber"}>
                  {labels[i.status]}
                </Pill>
              </div>
              <p className="my-2 text-xs leading-6 text-muted-foreground">
                {i.id} · {i.refs.length} 项可检索 · {i.failed.length} 项失败 ·
                更新 {i.observedAt || "尚未观测"}
              </p>
              {i.failed.map((id) => (
                <p key={id} className="text-xs text-destructive">
                  {s.files.find((f) => f.id === id)?.name}
                  ：样本解析中断，可恢复原任务
                </p>
              ))}
              <div className="flex flex-wrap gap-2">
                <Btn
                  variant="outline"
                  onClick={() =>
                    dispatch({
                      type: "index-status",
                      id: i.id,
                      status: "paused",
                    })
                  }
                >
                  暂停后续分析
                </Btn>
                <Btn
                  variant="outline"
                  onClick={() => dispatch({ type: "index-recover", id: i.id })}
                >
                  恢复原索引任务
                </Btn>
                <Btn
                  variant="ghost"
                  onClick={() =>
                    dispatch({
                      type: "index-status",
                      id: i.id,
                      status: "stale",
                    })
                  }
                >
                  演示索引过期
                </Btn>
                <Btn
                  variant="ghost"
                  onClick={() =>
                    open({
                      type: "danger",
                      title: "清除这份分析数据？",
                      text: "仅清除模拟索引与排队分析，不删除文件和标签。",
                      action: { type: "index-clear", id: i.id },
                    })
                  }
                >
                  清除分析数据
                </Btn>
              </div>
            </article>
          ))}
      </div>
    </div>
  );
}

