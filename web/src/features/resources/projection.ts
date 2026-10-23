import type { ResourceDescriptor } from './contract';

export interface ResourceListProjection {
  items: ResourceDescriptor[];
  filteredItems: ResourceDescriptor[];
  search: string;
}

export type ResourceSort = 'name' | 'created';

export function projectResourceList(items: ResourceDescriptor[], search: string, sort: ResourceSort = 'name'): ResourceListProjection {
  const normalized = search.trim().toLocaleLowerCase();
  const filtered = normalized ? items.filter(item => item.display_name.toLocaleLowerCase().includes(normalized)) : items;
  const filteredItems = [...filtered].sort((left, right) => sort === 'created'
    ? right.created_at.localeCompare(left.created_at)
    : left.display_name.localeCompare(right.display_name, 'zh-CN', { sensitivity: 'base' }));
  return { items, search, filteredItems };
}
