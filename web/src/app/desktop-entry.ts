import type { DesktopBridge } from '../generated/desktop-ipc';

export type DesktopEntryBridge = Pick<DesktopBridge, 'capabilities'> & Partial<Pick<DesktopBridge, 'chooseVault' | 'listNotes' | 'searchNotes' | 'readDocument' | 'readLinks'>>;
declare global { interface Window { drivebridgeDesktop?: DesktopEntryBridge } }

// Only the packaged origin selects local mode. A page-defined global on a
// remote website must not silently replace its established cloud entry.
export function isDesktopEntry(location: Pick<Location, 'protocol' | 'hostname'>): boolean {
  return location.protocol === 'drivebridge-app:' && location.hostname === 'app';
}

export function readDesktopCapabilities(value: unknown): string[] {
  if (!value || typeof value !== 'object' || Array.isArray(value) || Object.keys(value).join() !== 'data') throw new Error('desktop_contract_invalid');
  const data = (value as { data: unknown }).data;
  if (!data || typeof data !== 'object' || Array.isArray(data)) throw new Error('desktop_contract_invalid');
  const fields = Object.keys(data).sort().join();
  const candidate = data as { schema_version?: unknown; available?: unknown };
  if (fields !== 'available,schema_version' || candidate.schema_version !== 'drivebridge.desktop.v1alpha1' || !Array.isArray(candidate.available) || candidate.available.length > 64 || candidate.available.some(item => typeof item !== 'string' || !/^[a-z][A-Za-z0-9]{0,63}$/.test(item)) || new Set(candidate.available).size !== candidate.available.length) throw new Error('desktop_contract_invalid');
  return candidate.available as string[];
}
