import { useState } from "react";
import { navGroupsKey, readNavGroups, writeNavGroups } from "../nav-groups";

export function useNavGroups(instance: string, user: string) {
  const key = navGroupsKey(instance, user);
  const [state, setState] = useState(() => ({ key, expanded: readNavGroups(key) }));
  const current = state.key === key ? state : { key, expanded: readNavGroups(key) };
  if (state.key !== key) setState(current);
  function change(expanded: Record<string, boolean>) {
    setState({ key, expanded });
    writeNavGroups(key, expanded);
  }
  return [current.expanded, change] as const;
}
