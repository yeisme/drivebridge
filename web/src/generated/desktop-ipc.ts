// Generated from Go IPC catalog and owner schemas. No runtime implementation.
import type { components as Desktop } from './desktop';
import type { components as Pinax } from './pinax-reader';
import type { components as PinaxWriterSync } from './pinax-writer-sync';
export type DesktopResult<T> = { data: T } | { error: Desktop['schemas']['DesktopError'] };
export interface DesktopBridge {
  capabilities(request: { input: Desktop['schemas']["EmptyRequest"] }): Promise<DesktopResult<Desktop['schemas']["DesktopCapabilities"]>>;
  listSources(request: { input: Desktop['schemas']["PageQuery"] }): Promise<DesktopResult<Desktop['schemas']["SourcePage"]>>;
  chooseVault(request: { input: Desktop['schemas']["EmptyRequest"] }): Promise<DesktopResult<Desktop['schemas']["BindingPreview"]>>;
  confirmBinding(request: { input: Desktop['schemas']["ConfirmBinding"]; idempotency_key: string }): Promise<DesktopResult<Desktop['schemas']["BindingMutationReceipt"]>>;
  lookupBindingMutation(request: { input: Desktop['schemas']["BindingMutationLookup"] }): Promise<DesktopResult<Desktop['schemas']["BindingLookupResult"]>>;
  setSourceVisibility(request: { input: Desktop['schemas']["SetSourceVisibility"]; binding_id: string; idempotency_key: string }): Promise<DesktopResult<Desktop['schemas']["BindingView"]>>;
  releaseProtection(request: { input: Desktop['schemas']["ReleaseProtection"]; binding_id: string; idempotency_key: string }): Promise<DesktopResult<Desktop['schemas']["BindingMutationReceipt"]>>;
  listNotes(request: { input: Pinax['schemas']["pinax.reader.notes.request.v1alpha1"]; source_id: string }): Promise<DesktopResult<Pinax['schemas']["pinax.reader.note_list.v1alpha1"]>>;
  searchNotes(request: { input: Pinax['schemas']["pinax.reader.search.request.v1alpha1"]; source_id: string }): Promise<DesktopResult<Pinax['schemas']["pinax.reader.search_results.v1alpha1"]>>;
  readDocument(request: { input: Pinax['schemas']["pinax.reader.document.request.v1alpha1"] & { revision: string }; source_id: string }): Promise<DesktopResult<Pinax['schemas']["pinax.reader.document.v1alpha1"]>>;
  readLinks(request: { input: Pinax['schemas']["pinax.reader.links.request.v1alpha1"]; source_id: string }): Promise<DesktopResult<Pinax['schemas']["pinax.reader.link_page.v1alpha1"]>>;
  listTags(request: { input: Pinax['schemas']["pinax.reader.tags.request.v1alpha1"]; source_id: string }): Promise<DesktopResult<Pinax['schemas']["pinax.reader.tags.v1alpha1"]>>;
  readGraph(request: { input: Pinax['schemas']["pinax.reader.graph.request.v1alpha1"]; source_id: string }): Promise<DesktopResult<Pinax['schemas']["pinax.reader.graph.v1alpha1"]>>;
  findDaily(request: { input: Pinax['schemas']["pinax.reader.daily.request.v1alpha1"]; source_id: string }): Promise<DesktopResult<Pinax['schemas']["pinax.reader.daily.v1alpha1"]>>;
  readEditorSource(request: { input: PinaxWriterSync['schemas']["pinax.writer.editor_source.request.v1alpha1"]; source_id: string }): Promise<DesktopResult<PinaxWriterSync['schemas']["pinax.writer.editor_source.v1alpha1"]>>;
  proposeEditorDraft(request: { input: Desktop['schemas']["DesktopEditorDraftInput"]; source_id: string }): Promise<DesktopResult<PinaxWriterSync['schemas']["pinax.projection.v1"]>>;
  syncStatus(request: { input: PinaxWriterSync['schemas']["pinax.sync.status.request.v1"]; source_id: string }): Promise<DesktopResult<PinaxWriterSync['schemas']["pinax.projection.v1"]>>;
  syncDiff(request: { input: PinaxWriterSync['schemas']["pinax.sync.diff.request.v1"]; source_id: string }): Promise<DesktopResult<PinaxWriterSync['schemas']["pinax.projection.v1"]>>;
  syncPush(request: { input: Omit<PinaxWriterSync['schemas']["pinax.sync.push.request.v1"], "X-Pinax-Operation-ID" | "Idempotency-Key">; source_id: string; idempotency_key: string }): Promise<DesktopResult<PinaxWriterSync['schemas']["pinax.projection.v1"]>>;
  syncPull(request: { input: Omit<PinaxWriterSync['schemas']["pinax.sync.pull.request.v1"], "X-Pinax-Operation-ID" | "Idempotency-Key">; source_id: string; idempotency_key: string }): Promise<DesktopResult<PinaxWriterSync['schemas']["pinax.projection.v1"]>>;
  listSyncConflicts(request: { input: PinaxWriterSync['schemas']["pinax.sync.conflicts.list.request.v1"]; source_id: string }): Promise<DesktopResult<PinaxWriterSync['schemas']["pinax.projection.v1"]>>;
  diffSyncConflict(request: { input: PinaxWriterSync['schemas']["pinax.sync.conflicts.diff.request.v1"]; source_id: string }): Promise<DesktopResult<PinaxWriterSync['schemas']["pinax.projection.v1"]>>;
  resolveSyncConflict(request: { input: Desktop['schemas']["DesktopSyncConflictResolveInput"]; source_id: string; idempotency_key: string }): Promise<DesktopResult<PinaxWriterSync['schemas']["pinax.projection.v1"]>>;
  listReadingState(request: { input: Desktop['schemas']["PageQuery"] }): Promise<DesktopResult<Desktop['schemas']["ReadingStatePage"]>>;
  updateReadingState(request: { input: Desktop['schemas']["UpdateReadingState"]; idempotency_key: string }): Promise<DesktopResult<Desktop['schemas']["ReadingState"]>>;
  lookupReadingMutation(request: { input: Desktop['schemas']["LookupReadingMutation"] }): Promise<DesktopResult<Desktop['schemas']["ReadingMutationResult"]>>;
  listTopicPins(request: { input: Desktop['schemas']["PageQuery"] }): Promise<DesktopResult<Desktop['schemas']["TopicPinPage"]>>;
  updateTopicPin(request: { input: Desktop['schemas']["UpdateTopicPin"]; idempotency_key: string }): Promise<DesktopResult<Desktop['schemas']["TopicPin"]>>;
  lookupTopicPinMutation(request: { input: Desktop['schemas']["TopicPinLookup"] }): Promise<DesktopResult<Desktop['schemas']["TopicPinLookupResult"]>>;
  exportSource(request: { input: Desktop['schemas']["ExportSourceInput"]; idempotency_key: string }): Promise<DesktopResult<Desktop['schemas']["ExportOperation"]>>;
  getExport(request: { input: Desktop['schemas']["EmptyRequest"]; operation_id: string }): Promise<DesktopResult<Desktop['schemas']["ExportOperation"]>>;
  lookupExport(request: { input: Desktop['schemas']["ExportLookup"] }): Promise<DesktopResult<Desktop['schemas']["ExportLookupResult"]>>;
}
