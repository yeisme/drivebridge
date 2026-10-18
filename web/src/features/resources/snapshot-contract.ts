export type SnapshotMember = { file_id: string; version_id: string; path: string };
export type Snapshot = { snapshot_id: string; collection_id: string; collection_revision: number; entries_total: number; entries_visible: number; created_at: string; items: SnapshotMember[] };
