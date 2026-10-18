import { api } from '../../../api';
import type { ResourceDescriptor, ResourceDescriptorPage } from '../contract';

export function listResourceDescriptors(query: URLSearchParams, signal: AbortSignal) {
  return api<ResourceDescriptorPage>('/resource-descriptors?' + query, 'GET', undefined, {}, signal);
}

export function getResourceDescriptor(descriptorId: string, signal: AbortSignal) {
  return api<ResourceDescriptor>('/resource-descriptors/' + encodeURIComponent(descriptorId), 'GET', undefined, {}, signal);
}
