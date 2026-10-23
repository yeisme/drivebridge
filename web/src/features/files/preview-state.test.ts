import { describe, expect, it } from 'vitest';
import { createPreviewWorkspaceState, previewKey, previewReducer } from './preview-state';
import type { DriveFile } from '../../api';

const file = (id: string, version = 'v1', path = `${id}.txt`): DriveFile => ({
  id, version_id: version, space_id: 'space', path, size_bytes: 1, mime_type: 'text/plain', trashed: false, updated_at: '2026-01-01T00:00:00Z',
});

describe('preview reducer', () => {
  it('isolates tabs by space, file and revision and reuses only an unpinned preview tab', () => {
    let state = createPreviewWorkspaceState();
    state = previewReducer(state, { type: 'open', file: file('one') });
    state = previewReducer(state, { type: 'pin', key: previewKey(file('one')), pinned: true });
    state = previewReducer(state, { type: 'open', file: file('two') });
    expect(state.tabs).toHaveLength(2);
    state = previewReducer(state, { type: 'pin', key: previewKey(file('two')), pinned: true });
    state = previewReducer(state, { type: 'open', file: file('three') });
    expect(state.tabs.map(tab => tab.fileId)).toEqual(['one', 'two', 'three']);
    state = previewReducer(state, { type: 'open', file: file('two', 'v2') });
    expect(state.tabs.map(tab => tab.versionId)).toEqual(['v1', 'v1', 'v1', 'v2']);
  });

  it('supports close others, close right, reader state and panel/tree state', () => {
    let state = createPreviewWorkspaceState();
    for (const id of ['one', 'two', 'three']) state = previewReducer(state, { type: 'open', file: file(id), pinned: true });
    state = previewReducer(state, { type: 'reader', key: previewKey(file('two')), state: { scrollTop: 120, zoom: 1.2 } });
    state = previewReducer(state, { type: 'panel', open: false });
    state = previewReducer(state, { type: 'tree', path: 'docs', expanded: true });
    expect(state.tabs[1].readerState).toEqual({ scrollTop: 120, zoom: 1.2 });
    expect(state.rightPanelOpen).toBe(false);
    expect(state.treeExpanded).toContain('docs');
    state = previewReducer(state, { type: 'closeOthers', key: previewKey(file('two')) });
    expect(state.tabs.map(tab => tab.fileId)).toEqual(['two']);
  });

  it('closes all unpinned tabs while retaining pinned tabs', () => {
    let state = createPreviewWorkspaceState();
    state = previewReducer(state, { type: 'open', file: file('one'), pinned: true });
    state = previewReducer(state, { type: 'open', file: file('two') });
    state = previewReducer(state, { type: 'closeAll' });
    expect(state.tabs.map(tab => tab.key)).toEqual([previewKey(file('one'))]);
    expect(state.activeTab).toBe(previewKey(file('one')));
  });
});
