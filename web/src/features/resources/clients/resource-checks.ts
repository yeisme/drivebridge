import { api } from '../../../api';
import type { components } from '../../../generated/personal-workspace';

export type ResourceIntegrityReceipt = components['schemas']['ResourceIntegrityReceiptEnvelope']['data'];
export type ResourceCheckRequest = components['schemas']['ResourceCheckRequest'];

export function requestResourceCheck(descriptor: string, input: ResourceCheckRequest, lookup: boolean, signal: AbortSignal) {
  return api<ResourceIntegrityReceipt>('/resource-descriptors/' + encodeURIComponent(descriptor) + '/checks' + (lookup ? '/lookup' : ''), 'POST', input, {}, signal);
}
