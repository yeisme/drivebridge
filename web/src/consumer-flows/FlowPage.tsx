import type { Space, UserIdentity } from "../api";
import { ConsumerFlows, CENTERS, type CenterId } from "./ConsumerFlows";
import { SearchWorkspace } from "./SearchWorkspace";
import { WorkspacePageHeader } from "../components/WorkspacePageHeader";
import { Button } from "../components/ui/button";

export function flowPageRoute(page: "flows" | "search", center: CenterId, tenant: string) {
  const target = page === "search" && center === "search" ? "search" : "flows";
  const query = new URLSearchParams();
  if (tenant) query.set("tenant", tenant);
  if (target === "flows") query.set("center", center);
  return { page: target, tab: center, hash: `#/${target}${query.size ? "?" + query : ""}` } as const;
}

export function FlowPage({ page, tab, tenant, user, spaces, onNavigate }: {
  page: "flows" | "search";
  tab: string;
  tenant: string;
  user: UserIdentity;
  spaces: Space[];
  onNavigate: (page: "flows" | "search", tab: CenterId) => void;
}) {
  const navigate = (next: CenterId) => {
    const route = flowPageRoute(page, next, tenant);
    history.replaceState(null, "", location.pathname + location.search + route.hash);
    onNavigate(route.page, next);
  };
  if (page === "search") return <div className="min-w-0 space-y-6">
    <WorkspacePageHeader title="搜索" description="查找授权范围内的固定版本文件。元数据与内容发现分别展示。" actions={<Button type="button" variant="ghost" className="min-h-11 md:min-h-8" onClick={() => navigate("upload")}>打开文件流程</Button>}/>
    <SearchWorkspace spaces={spaces}/>
  </div>;
  const center = CENTERS.find(center => center.id === tab)?.id || "upload";
  return <ConsumerFlows user={user} spaces={spaces} initialCenter={center} onCenterChange={navigate}/>;
}
