import { api } from '../../../api';
import type { Snapshot } from '../snapshot-contract';

export function getResourceSnapshot(snapshot: string, signal: AbortSignal) {
  return api<Snapshot>('/collection-snapshots/' + encodeURIComponent(snapshot), 'GET', undefined, {}, signal);
}
