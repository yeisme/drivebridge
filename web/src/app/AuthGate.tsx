import { ArrowUpRight, Check, LoaderCircle, ShieldCheck } from "lucide-react";
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
  onKeyLogin: () => void;
  onDemoLogin: () => void;
}

// Render only before the authenticated workspace mounts. Permissions and
// session acquisition remain in the caller and the server, never this view.
export function AuthGate({ loading, mode, busy, value, error, onChange, onKeyLogin, onDemoLogin }: AuthGateProps) {
  if (loading) return <main className="loading"><LoaderCircle className="spin" /><p>正在打开你的空间…</p></main>;
  return <main className="login">
    <div className="login-card">
      <div className="brand"><span className="brandmark"><BrandMark size={32} /></span>DriveBridge</div>
      <span className="eyebrow">YOUR FILES. YOUR AGENTS.</span>
      <h1>文件就位，<br />工作开始。</h1>
      <p>一个属于你的文件空间。连接存储，整理素材，<br />让每个 Agent 在你允许的范围内工作。</p>
      {mode?.demo && <>
        <Button variant="default" type="submit" disabled={busy} onClick={onDemoLogin}>{busy ? "正在打开体验空间…" : "无需账号，进入本地体验"}</Button>
        <p>使用独立的临时体验空间，可上传和预览文件。服务关闭后数据会清空。</p>
      </>}
      {mode?.oidc === true && <a className={buttonVariants({ variant: "default", size: "default", className: "w-full" })} href="/auth/login">使用账号登录 <ArrowUpRight size={18} /></a>}
      {mode && <KeyLoginForm value={value} busy={busy} secondary={!!mode.oidc} onChange={onChange} onSubmit={onKeyLogin} />}
      {error && <p role="alert" className="error">{error}</p>}
      {mode?.governance && <Suspense fallback={<p role="status">正在加载账号恢复入口…</p>}><AccountRecovery /></Suspense>}
      <footer><ShieldCheck size={16} /> 个人空间隔离 · 授权随时可撤销</footer>
    </div>
    <div className="login-art" aria-hidden="true">
      <div className="art-orbit" />
      <div className="art-file"><BrandMark size={72} /><strong>为下一步，准备就绪</strong><span>文件 · 存储 · Agent</span></div>
      <div className="art-tag"><Check size={16} /> 按你的方式连接</div>
    </div>
  </main>;
}
