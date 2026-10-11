import type { Space } from "../api";
import { AppDialog } from "../components/AppDialog";
import { SearchWorkspace } from "../consumer-flows/SearchWorkspace";

export function CommandSearch({ spaces, close }: { spaces: Space[]; close: () => void }) {
  return <AppDialog title="搜索工作区" close={close}>
    <SearchWorkspace spaces={spaces}/>
  </AppDialog>;
}
