export interface DetailPreferenceIdentity { instance: string; user: string }

export function detailPreferenceKey(identity?: DetailPreferenceIdentity): string | undefined {
  if (!identity?.instance || !identity.user) return undefined;
  return `drivebridge:detail-width:${encodeURIComponent(identity.instance)}:${encodeURIComponent(identity.user)}:file`;
}

export function readDetailWidth(key?: string): number {
  if (!key) return 360;
  try {
    const value = Number(localStorage.getItem(key));
    return Number.isFinite(value) && value >= 320 && value <= 640 ? value : 360;
  } catch { return 360; }
}

export function saveDetailWidth(key: string | undefined, width: number): void {
  if (!key || !Number.isFinite(width) || width < 320 || width > 640) return;
  try { localStorage.setItem(key, String(width)); }
  catch { /* Optional preference: retain in-memory resizing if storage is denied. */ }
}
