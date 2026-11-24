import { ArrowUpRight, Check, FileText, Folder, Lock, LoaderCircle, ShieldCheck } from "lucide-react";
import { BrandMark } from "../BrandMark";
import { lazy, Suspense } from "react";
const AccountRecovery = lazy(() => import("../Governance").then(module => ({ default: module.AccountRecovery })));
import { Button, buttonVariants } from "../components/ui/button";
import type { AuthMode } from "../api";
import { KeyLoginForm } from "./KeyLoginForm";

interface AuthGateProps {
  loading: boolean;
  mode: AuthMode | null;
  busy: boolean;
  value: string;
  error: string;
  onChange: (value: string) => void;
  onKeyLogin: (overrideKey?: string) => void;
  onDemoLogin: () => void;
}

// Render only before the authenticated workspace mounts. Permissions and
// session acquisition remain in the caller and the server, never this view.
export function AuthGate({ loading, mode, busy, value, error, onChange, onKeyLogin, onDemoLogin }: AuthGateProps) {
  if (loading) return <main className="loading"><LoaderCircle className="spin" /><p>正在打开你的空间…</p></main>;
  return (
    <main className="login">
      <div className="login-card">
        <div className="brand text-foreground">
          <span className="brandmark text-primary"><BrandMark size={30} /></span>
          DriveBridge
        </div>
        <span className="eyebrow">PERSONAL CLOUD · AI READY</span>
        <h1>个人与 AI 协作的<br />文件工作台。</h1>
        <p>统一管理多存储文件、固定版本预览与 Markdown 知识库，<br />让每个 AI 助手仅在你授权的目录范围内安全交付。</p>
        {mode?.demo && (
          <>
            <Button variant="default" type="submit" className="tactile-cta h-10 rounded-lg text-sm font-medium" disabled={busy} onClick={onDemoLogin}>
              {busy ? "正在打开体验空间…" : "无需账号，进入本地体验"}
            </Button>
            <p className="mt-2 text-xs text-muted-foreground">使用独立的临时体验空间，可上传和预览文件。服务关闭后数据会清空。</p>
          </>
        )}
        {mode?.oidc === true && (
          <a className={buttonVariants({ variant: "default", size: "default", className: "tactile-cta h-10 w-full rounded-lg font-medium" })} href="/auth/login">
            使用账号登录 <ArrowUpRight size={16} />
          </a>
        )}
        {mode && <KeyLoginForm value={value} busy={busy} secondary={!!mode.oidc} onChange={onChange} onSubmit={onKeyLogin} />}
        {error && <p role="alert" className="error">{error}</p>}
        {mode?.governance && <Suspense fallback={<p role="status">正在加载账号恢复入口…</p>}><AccountRecovery /></Suspense>}
        <footer><ShieldCheck size={15} className="text-primary" /> 个人空间隔离 · 固定版本校验 · 授权随时可撤销</footer>
      </div>

      <div className="login-art" aria-hidden="true">
        <div className="w-full max-w-md px-8">
          <div className="overflow-hidden rounded-xl border border-border bg-card shadow-[var(--shadow-md)]">
            <div className="flex items-center justify-between border-b border-border bg-muted/40 px-3.5 py-2.5 text-xs">
              <span className="font-medium text-foreground">个人空间 / 项目研究</span>
              <span className="inline-flex items-center gap-1 rounded-md border border-border bg-background px-2 py-0.5 text-[11px] text-muted-foreground">
                <Lock size={11} className="text-primary" /> 版本已锁定
              </span>
            </div>
            <div className="divide-y divide-border/70 text-xs">
              <div className="flex items-center justify-between px-3.5 py-2.5">
                <span className="flex items-center gap-2 font-medium text-foreground">
                  <Folder size={14} className="text-muted-foreground" /> 参考素材归档
                </span>
                <span className="text-muted-foreground">24 项</span>
              </div>
              <div className="flex items-center justify-between bg-accent/50 px-3.5 py-2.5">
                <span className="flex items-center gap-2 font-medium text-foreground">
                  <FileText size={14} className="text-primary" /> 调研纪要与提纲.md
                </span>
                <span className="tabular-nums text-muted-foreground">18.4 KB</span>
              </div>
              <div className="flex items-center justify-between px-3.5 py-2.5">
                <span className="flex items-center gap-2 text-foreground">
                  <Check size={14} className="text-[var(--success)]" /> AI 投递箱 · 待人工查收
                </span>
                <span className="rounded-md bg-secondary px-1.5 py-0.5 text-[11px] font-medium text-foreground">隔离审阅</span>
              </div>
            </div>
          </div>
          <p className="mt-4 text-center text-xs text-muted-foreground">
            多存储统一目录 · 校验通过后完成入库 · 最小目录级 AI 授权
          </p>
        </div>
      </div>
    </main>
  );
}
