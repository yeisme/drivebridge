import type { ResourceDescriptor } from './contract';

export interface ResourceListProjection {
  items: ResourceDescriptor[];
  filteredItems: ResourceDescriptor[];
  search: string;
}

export function projectResourceList(items: ResourceDescriptor[], search: string): ResourceListProjection {
  const normalized = search.trim().toLocaleLowerCase();
  return { items, search, filteredItems: normalized ? items.filter(item => item.display_name.toLocaleLowerCase().includes(normalized)) : items };
}
