import type { components } from '../../generated/personal-workspace';

export type ResourceDescriptor = components['schemas']['ResourceEnvelope']['data'];
export type ResourceDescriptorPage = components['schemas']['ResourcePageEnvelope']['data'];
export type ResourceKind = ResourceDescriptor['kind'];
export type ResourceLifecycle = 'loading' | 'empty' | 'ready' | 'stale' | 'error' | 'denied' | 'offline' | 'unsupported' | 'unknown';

export const resourceKinds: Record<ResourceKind, string> = { skill: 'Skill', template: '模板', reference_pack: '资料包' };

const text = (value: unknown): value is string => typeof value === 'string' && value.length > 0 && value.length <= 2048;

export function isResourceDescriptor(row: ResourceDescriptor): boolean {
  return Boolean(row && text(row.descriptor_id) && text(row.space_id) && text(row.display_name) && Object.hasOwn(resourceKinds, row.kind) && Number.isSafeInteger(row.descriptor_revision) && row.descriptor_revision > 0 && row.check_state === 'unknown' && typeof row.created_at === 'string' && Number.isFinite(Date.parse(row.created_at)) && ((text(row.file_ref) && row.collection_snapshot_id === undefined && row.kind !== 'reference_pack') || (text(row.collection_snapshot_id) && row.file_ref === undefined)));
}
