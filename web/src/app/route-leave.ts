import { useLayoutEffect } from 'react';

// Only booleans/messages live here; never retain drafts, identities or tokens.
// A forced identity/context invalidation bypasses this navigation-only guard.
const blockers = new Map<symbol, string>();

export function useRouteLeaveGuard(dirty: boolean, message: string) {
  useLayoutEffect(() => {
    if (!dirty) return;
    const key = Symbol('unsaved-route');
    blockers.set(key, message);
    return () => { blockers.delete(key); };
  }, [dirty, message]);
}

export function confirmRouteLeave(): boolean {
  if (!blockers.size) return true;
  const messages = [...new Set(blockers.values())];
  return window.confirm(messages.join('\n'));
}
