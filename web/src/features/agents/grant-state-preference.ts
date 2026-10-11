import { useState } from 'react';
import type { Grant } from './grant-client';
export type GrantStateFilter = 'all' | Grant['state'];
export const grantStateKey = (scope?: string) => scope ? 'drivebridge:grant-state:' + scope : '';
export function readGrantState(key: string): GrantStateFilter {
  try {
    const value = key ? localStorage.getItem(key) : null;
    if (value === 'all' || value === 'active' || value === 'disabled' || value === 'expired' || value === 'revoked') return value;
  } catch { /* Display preferences must not prevent authorized reads. */ }
  return 'all';
}
export function useGrantStatePreference(scope?: string) {
  const key = grantStateKey(scope);
  const [saved, setSaved] = useState(() => ({ key, value: readGrantState(key) }));
  const current = saved.key === key ? saved : { key, value: readGrantState(key) };
  if (saved.key !== key) setSaved(current);
  function change(value: GrantStateFilter) {
    setSaved({ key, value });
    if (key) {
      try { localStorage.setItem(key, value); }
      catch { /* Keep this choice in memory when browser storage is denied. */ }
    }
  }
  return [current.value, change] as const;
}
