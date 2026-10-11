import { useMemo, useState } from "react";
import { AppDialog } from "../../components/AppDialog";
import { Button } from "../../components/ui/button";
import { Checkbox } from "../../components/ui/checkbox";
import { Textarea } from "../../components/ui/textarea";

interface Props {
  baseVersion: string;
  base: string;
  remoteVersion: string;
  remote: string;
  originalDraft: string;
  draft: string;
  maxBytes: number;
  onDraft: (body: string) => void;
  onConfirm: () => void;
  onClose: () => void;
}

// Show one exact changed span, retaining shared prefix/suffix lines. No
// automatic merge or quadratic line matching over the document byte budget.
function changedLines(before: string, after: string) {
  const oldLines = before.split("\n"), newLines = after.split("\n");
  let start = 0, end = 0;
  while (start < oldLines.length && start < newLines.length && oldLines[start] === newLines[start]) start++;
  while (end < oldLines.length - start && end < newLines.length - start && oldLines[oldLines.length - 1 - end] === newLines[newLines.length - 1 - end]) end++;
  return { removed: oldLines.slice(start, oldLines.length - end).map(line => line || "（空行）").join("\n"), added: newLines.slice(start, newLines.length - end).map(line => line || "（空行）").join("\n"), line: start + 1 };
}

export function DocumentConflictMerge({ baseVersion, base, remoteVersion, remote, originalDraft, draft, maxBytes, onDraft, onConfirm, onClose }: Props) {
  const [reviewedDraft, setReviewedDraft] = useState<string>();
  const [confirmed, setConfirmed] = useState(false);
  const diff = useMemo(() => changedLines(remote, draft), [remote, draft]);
  const reviewing = reviewedDraft === draft;
  const tooLarge = new TextEncoder().encode(draft).byteLength > maxBytes;
  return <AppDialog title="人工合并冲突" close={onClose} className="sm:max-w-5xl">
    <p className="break-all text-sm">原基线版本：<code>{baseVersion}</code>；新基线版本：<code>{remoteVersion}</code>。远端正文已固定到此版本，不会自动跟随最新内容。</p>
    <p className="text-sm text-muted-foreground">原操作已回查。请人工整合三份正文；原版本与远端版本保持不变，只有确认后才以新恢复身份创建版本。关闭此窗口仍保留内存草稿。</p>
    <div className="grid min-w-0 gap-3 md:grid-cols-3">
      {([["原基线正文", base], ["远端固定版本正文", remote], ["合并前本地草稿", originalDraft]] as const).map(([title, body]) => <section key={title} className="min-w-0"><h3 className="text-sm font-medium">{title}</h3><pre className="max-h-48 overflow-auto whitespace-pre-wrap break-words rounded border p-2 text-xs">{body}</pre></section>)}
    </div>
    <label className="block text-sm font-medium">人工合并正文<Textarea aria-label="人工合并正文" className="mt-1 min-h-48 w-full min-w-0 font-mono text-sm" value={draft} onChange={event => { setReviewedDraft(undefined); setConfirmed(false); onDraft(event.target.value); }}/></label>
    {tooLarge && <p role="alert">合并正文超过 {maxBytes} 字节，缩减后才能保存。</p>}
    {reviewing && <section aria-label="相对新基线的差异" className="space-y-2">
      <h3 className="text-sm font-medium">相对新基线的差异（从第 {diff.line} 行起的变更范围）</h3>
      {remote === draft ? <p>正文与新基线相同，没有内容差异。</p> : <div className="grid min-w-0 gap-3 md:grid-cols-2"><section className="min-w-0"><h4>删除 / 替换前</h4><pre className="max-h-48 overflow-auto whitespace-pre-wrap break-words rounded border p-2 text-xs">{diff.removed || "（空）"}</pre></section><section className="min-w-0"><h4>新增 / 替换后</h4><pre className="max-h-48 overflow-auto whitespace-pre-wrap break-words rounded border p-2 text-xs">{diff.added || "（空）"}</pre></section></div>}
      <label className="flex items-start gap-2 break-all text-sm"><Checkbox checked={confirmed} onCheckedChange={setConfirmed}/>我已审阅差异，确认以版本 {remoteVersion} 为新基线保存</label>
    </section>}
    <div className="flex flex-wrap justify-end gap-2 pt-3"><Button type="button" variant="outline" onClick={onClose}>返回并保留草稿</Button><Button type="button" variant="outline" disabled={tooLarge} onClick={() => { setReviewedDraft(draft); setConfirmed(false); }}>预览合并差异</Button>{reviewing && <Button type="button" disabled={!confirmed || tooLarge} onClick={onConfirm}>确认新基线并保存新版本</Button>}</div>
  </AppDialog>;
}
