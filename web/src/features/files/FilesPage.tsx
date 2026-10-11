import type { ComponentProps } from "react";
import { Maximize2, Minimize2 } from "lucide-react";
import { Finder } from "../../Finder";
import { DriveHome } from "./DriveHome";
import type { Space } from "../../api";
import { driveFocusURL } from "../../workspace-route";
import { Button, buttonVariants } from "../../components/ui/button";

type HomeProps = ComponentProps<typeof DriveHome>;
type FinderProps = ComponentProps<typeof Finder>;
interface Props {
  open: boolean;
  currentSpace?: Space;
  focused: boolean;
  routeHash: string;
  spaces: HomeProps["spaces"];
  connections: HomeProps["connections"];
  governance: HomeProps["governance"];
  openDrive: HomeProps["open"];
  connect?: HomeProps["connect"];
  home: () => void;
  finder: Pick<FinderProps, "readOnly" | "managementRestricted" | "space" | "prefix" | "setPrefix" | "revision" | "reload" | "backendLabel" | "upload" | "onOperation" | "onPreview" | "onShare">;
}

// Compose the existing file surfaces without introducing another file store.
export function FilesPage(p: Props) {
  if (!p.open) return <DriveHome spaces={p.spaces} connections={p.connections} governance={p.governance} open={p.openDrive} connect={p.connect} />;
  if (!p.currentSpace) return <section className="unavailable-drive" role="status"><h2>此网盘不在当前身份的可访问范围内</h2><p>可能已切换账号、租户或访问范围。请重新选择网盘。</p><Button type="button" onClick={p.home}>返回我的网盘</Button></section>;
  const focusLabel = p.focused ? "返回完整工作区" : "在新标签页专注使用网盘";
  return <Finder {...p.finder}
    focusAction={<a className={buttonVariants({ variant: "ghost", size: "icon" })} href={driveFocusURL(p.routeHash, !p.focused)} target={p.focused ? undefined : "_blank"} rel={p.focused ? undefined : "noopener"} title={focusLabel} aria-label={focusLabel}>{p.focused ? <Minimize2 /> : <Maximize2 />}</a>}
    showSpaceSwitcher={false}
    onedriveConnections={p.connections.filter(c => c.kind === "onedrive" && !c.disabled).map(c => ({ id: c.id, name: c.name }))}
    spaces={p.focused ? [p.currentSpace] : p.spaces}
    setSpace={id => { const next = p.spaces.find(s => s.id === id); if (next) p.openDrive(next); }}
  />;
}
