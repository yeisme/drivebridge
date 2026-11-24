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
  const classes = [
    "app group/shell bg-background db-pine-workspace",
    mobile && "mobile-shell",
    collapsed && "sidebar-collapsed",
    focused && "drive-focused",
  ]
    .filter(Boolean)
    .join(" ");
  return (
    <div
      className={classes}
      data-pine-workspace="true"
      data-collapsed={collapsed}
      data-mobile={mobile}
      data-focused={focused}
    >
      {children}
    </div>
  );
}

export function WorkspaceMain({ children }: { children: ReactNode }) {
  return <main className="main db-workspace-content bg-background">{children}</main>;
}
