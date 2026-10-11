import type { ReactNode } from "react";

interface WorkspaceShellProps {
  mobile: boolean;
  collapsed: boolean;
  focused: boolean;
  children: ReactNode;
}

// Layout state only. Page data, permissions and portal ownership stay with
// their existing application components during incremental page migration.
export function WorkspaceShell({ mobile, collapsed, focused, children }: WorkspaceShellProps) {
  const classes = ["app group/shell bg-sidebar", mobile && "mobile-shell", collapsed && "sidebar-collapsed", focused && "drive-focused"].filter(Boolean).join(" ");
  return <div className={classes} data-collapsed={collapsed} data-mobile={mobile} data-focused={focused}>{children}</div>;
}

export function WorkspaceMain({ children }: { children: ReactNode }) {
  return <main className="main my-2 mr-2 rounded-xl bg-background group-data-[mobile=true]/shell:m-0 group-data-[mobile=true]/shell:rounded-none group-data-[focused=true]/shell:m-0 group-data-[focused=true]/shell:rounded-none">{children}</main>;
}
