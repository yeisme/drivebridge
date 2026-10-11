import { RecordRow } from "./CompactUI";
import { FileButton } from "./HoverPreview";
import { fileRef } from "./preview-model";
import React, { useState } from "react";
import { ArrowUpRight, Check, Plus } from "lucide-react";
import { cn } from "@/lib/utils";
import {
  Btn,
  Choice,
  Empty,
  FileIcon,
  Heading,
  panel,
  Pill,
  useDemo,
} from "./ui";

export function InboxPage() {
  const { state: s, dispatch, open, go } = useDemo();
  const [folder, setFolder] = useState(
    s.space === "personal"
      ? "山野计划"
      : s.folders.find((f) => f.space === s.space)?.path || "待整理",
  );
  const files = s.files.filter(
    (f) => f.space === s.space && f.inbox && !f.trashed,
  );
  return (
    <>
      <Heading title="产物与收件" note="先检查内容，再决定放在哪里。">
        <Btn onClick={() => open({ type: "upload" })}>
          <Plus size={15} />
          手机导入
        </Btn>
      </Heading>
      <div className="mb-5 flex flex-wrap items-center gap-3">
        <Pill tone="amber">{files.length} 项待整理</Pill>
        <span className="text-xs text-muted-foreground">
          归档到 DriveBridge 云盘
        </span>
        <Choice
          label="归档目录"
          value={folder}
          options={s.folders
            .filter((f) => f.space === s.space && f.connectionId === "drive")
            .map((f) => ({ value: f.path, label: f.path }))}
          onChange={setFolder}
        />
      </div>
      <div className="">
        {files.map((f) => {
          const task = s.tasks.find(
            (t) =>
              f.id.startsWith(t.id) || t.items.some((i) => i.outputId === f.id),
          );
          const call = s.calls.find(
            (c) => c.space === s.space && c.outputId === f.id,
          );
          const app = s.apps.find((a) => a.id === call?.appId);
          const upload = s.tasks.find(
            (t) =>
              t.kind === "upload" && t.items.some((i) => i.fileId === f.id),
          );
          const ready = !upload || upload.status === "done";
          return (
            <RecordRow
              key={f.id}
              name={f.name}
              summary={`${f.origin === "agent" ? app?.name || "外部应用" : "手机导入"} · v${f.version} · ${task?.id || call?.operationId || "导入记录"}`}
              status={<Pill tone="amber">{ready ? "待验收" : "验证中"}</Pill>}
              onOpen={() =>
                open({
                  type: "preview",
                  id: f.id,
                  archiveFolder: folder,
                  scope: {
                    label: "收件箱",
                    refs: files.map((f) => fileRef(f)),
                  },
                })
              }
              actions={
                <FileButton
                  reference={fileRef(f)}
                  variant="ghost"
                  onOpen={() =>
                    open({
                      type: "preview",
                      id: f.id,
                      archiveFolder: folder,
                      scope: {
                        label: "收件箱",
                        refs: files.map((f) => fileRef(f)),
                      },
                    })
                  }
                >
                  预览
                </FileButton>
              }
            >
              <article
                className={cn(
                  panel,
                  "@container grid gap-3 p-4 min-[1280px]:grid-cols-[minmax(0,1fr)_500px]",
                )}
              >
                <div className="flex items-start gap-3">
                  <FileIcon file={f} />
                  <div className="min-w-0 flex-1">
                    <h2>
                      <FileButton
                        reference={fileRef(f)}
                        layout="surface"
                        className="!h-auto !justify-start !whitespace-normal !p-0 text-left text-sm font-semibold"
                        onOpen={() =>
                          open({
                            type: "preview",
                            id: f.id,
                            archiveFolder: folder,
                            scope: {
                              label: "收件箱",
                              refs: files.map((f) => fileRef(f)),
                            },
                          })
                        }
                      >
                        {f.name}
                      </FileButton>
                    </h2>
                    <p className="mt-1.5 text-xs leading-5 text-muted-foreground">
                      {f.origin === "agent"
                        ? `${app?.name || "资料助手"} · 指定目录授权`
                        : "你的手机 · 主动导入"}{" "}
                      · v{f.version} · {f.size}
                    </p>
                    <p className="mt-1 text-xs leading-6 text-muted-foreground">
                      {task && s.knowledge.taskPlans[task.id] && (
                        <span>
                          Skill 固定 v
                          {s.knowledge.taskPlans[task.id].skillVersion} ·{" "}
                        </span>
                      )}
                      来源任务：
                      {call?.operationId ||
                        task?.id ||
                        upload?.id ||
                        "演示导入记录"}{" "}
                      · 建议位置：{folder}
                    </p>
                    {f.inputRefs?.map((ref) => {
                      const input = s.files.find(
                        (x) =>
                          x.id === ref.id && x.space === s.space && !x.trashed,
                      );
                      return input ? (
                        <FileButton
                          reference={{
                            ...ref,
                            connectionId: input.connectionId,
                            externalRevision: input.externalRevision,
                          }}
                          key={ref.id}
                          variant="link"
                          className="mt-1 !h-auto !whitespace-normal !px-0 text-left text-xs"
                          onOpen={() =>
                            open({
                              type: "preview",
                              id: ref.id,
                              version: ref.version,
                              single: true,
                            })
                          }
                        >
                          输入：{input.name} · v{ref.version}
                          <ArrowUpRight size={12} />
                        </FileButton>
                      ) : (
                        <p
                          key={ref.id}
                          className="mt-1 text-xs text-muted-foreground"
                        >
                          输入引用当前不可访问
                        </p>
                      );
                    })}
                  </div>
                  <Pill tone={ready ? "amber" : "neutral"}>
                    {ready ? "待验收" : "验证中"}
                  </Pill>
                </div>
                <div className="flex items-center gap-2 overflow-x-auto min-[1280px]:justify-end">
                  <Btn
                    variant="ghost"
                    onClick={() =>
                      open({
                        type: "danger",
                        title: "退回这份产物？",
                        text: "示例文件会移入回收站，仍可恢复。",
                        action: { type: "reject", id: f.id },
                      })
                    }
                  >
                    退回
                  </Btn>
                  <FileButton
                    reference={fileRef(f)}
                    variant="outline"
                    onOpen={() =>
                      open({
                        type: "preview",
                        id: f.id,
                        archiveFolder: folder,
                        scope: {
                          label: "收件箱",
                          refs: files.map((f) => fileRef(f)),
                        },
                      })
                    }
                  >
                    预览
                  </FileButton>
                  {f.inputRefs?.[0] && (
                    <Btn
                      variant="outline"
                      onClick={() =>
                        open({
                          type: "compare",
                          ids: [f.inputRefs![0].id, f.id],
                          version: f.inputRefs![0].version,
                        })
                      }
                    >
                      对照输入与产物
                    </Btn>
                  )}
                  <Btn
                    variant="ghost"
                    onClick={() =>
                      dispatch({ type: "plan", ids: [f.id], target: folder })
                    }
                  >
                    预览归档计划
                  </Btn>
                  <Btn
                    disabled={!ready}
                    onClick={() =>
                      dispatch({ type: "accept", id: f.id, folder })
                    }
                  >
                    <Check size={14} />
                    接收并归档
                  </Btn>
                </div>
              </article>
            </RecordRow>
          );
        })}
      </div>
      {!files.length && (
        <Empty
          title="收件箱已整理完毕"
          text="以后手机导入和 Agent 产物会在这里等你验收。"
          action="查看我的文件"
          onAction={() => go("files")}
        />
      )}
    </>
  );
}
