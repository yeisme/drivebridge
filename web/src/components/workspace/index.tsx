import type { ReactNode } from "react";
import { X } from "lucide-react";
import { Button } from "../ui/button";

export function PageToolbar({
  eyebrow,
  title,
  subtitle,
  actions,
  children,
  className = "",
}: {
  eyebrow?: string;
  title: string;
  subtitle?: string;
  actions?: ReactNode;
  children?: ReactNode;
  className?: string;
}) {
  return <header className={`workspace-page-toolbar ${className}`}>
    <div className="workspace-page-toolbar-copy">
      {eyebrow && <p className="workspace-eyebrow">{eyebrow}</p>}
      <h1>{title}</h1>
      {subtitle && <p className="workspace-page-toolbar-subtitle">{subtitle}</p>}
    </div>
    {children}
    {actions && <div className="workspace-page-toolbar-actions">{actions}</div>}
  </header>;
}

export function WorkspaceNotice({
  title,
  detail,
  action,
  tone = "neutral",
}: {
  title: string;
  detail?: string;
  action?: ReactNode;
  tone?: "neutral" | "info" | "warning" | "danger" | "success";
}) {
  return <section role={tone === "danger" ? "alert" : "status"} className={`workspace-notice workspace-notice-${tone}`}>
    <div className="workspace-notice-copy"><strong>{title}</strong>{detail && <p>{detail}</p>}</div>
    {action && <div className="workspace-notice-action">{action}</div>}
  </section>;
}

export function WorkspaceEmptyState({
  icon,
  title,
  description,
  actions,
  className = "",
}: {
  icon?: ReactNode;
  title: string;
  description?: ReactNode;
  actions?: ReactNode;
  className?: string;
}) {
  return <section role="status" className={`workspace-empty-state ${className}`}>
    {icon && <div className="workspace-empty-icon">{icon}</div>}
    <div className="workspace-empty-copy"><h2>{title}</h2>{description && <p>{description}</p>}{actions && <div className="workspace-empty-actions">{actions}</div>}</div>
  </section>;
}

export function WorkspaceSkeleton({ label = "正在加载", rows = 4 }: { label?: string; rows?: number }) {
  return <section role="status" aria-label={label} className="workspace-skeleton"><span className="sr-only">{label}…</span>{Array.from({ length: rows }, (_, index) => <div className="workspace-skeleton-row" key={index}><i /><span /><b /></div>)}</section>;
}

export function InspectorHeader({ eyebrow, title, onClose, closeLabel = "关闭检查器" }: { eyebrow?: string; title: string; onClose?: () => void; closeLabel?: string }) {
  return <header className="workspace-inspector-header">
    <div className="min-w-0"><p className="workspace-eyebrow">{eyebrow || "检查器"}</p><h2 title={title}>{title}</h2></div>
    {onClose && <Button variant="ghost" size="icon" type="button" aria-label={closeLabel} onClick={onClose}><X /></Button>}
  </header>;
}

export function ViewModeSwitch<T extends string>({ items, value, onChange, label }: { items: Array<{ value: T; label: string; icon: ReactNode }>; value: T; onChange: (value: T) => void; label: string }) {
  return <div className="workspace-view-switch" role="group" aria-label={label}>{items.map(item => <Button key={item.value} type="button" variant="ghost" size="icon" aria-label={item.label} aria-pressed={value === item.value} onClick={() => onChange(item.value)}>{item.icon}</Button>)}</div>;
}

export function Breadcrumbs({ items, label = "路径" }: { items: Array<{ label: string; onClick?: () => void }>; label?: string }) {
  return <nav className="workspace-breadcrumbs" aria-label={label}>{items.map((item, index) => <span className="workspace-breadcrumb-item" key={`${item.label}-${index}`}>{index > 0 && <span className="workspace-breadcrumb-separator" aria-hidden="true">/</span>}{item.onClick ? <Button type="button" variant="ghost" onClick={item.onClick}>{item.label}</Button> : <strong title={item.label}>{item.label}</strong>}</span>)}</nav>;
}

export function SelectionToolbar({ count, children, onClear }: { count: number; children?: ReactNode; onClear?: () => void }) {
  return <div className="workspace-selection-toolbar" role="toolbar" aria-label="已选择项目操作"><span>{count} 项已选择</span>{children}{onClear && <Button type="button" variant="ghost" onClick={onClear}>取消选择</Button>}</div>;
}
