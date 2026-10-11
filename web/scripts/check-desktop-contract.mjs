// DTO contract checks only: no desktop process, vault or credentials.
import { execFileSync } from 'node:child_process';
import { createHash } from 'node:crypto';
import { fileURLToPath } from 'node:url';
import assert from 'node:assert/strict';
import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';

const root = fileURLToPath(new URL('../../', import.meta.url));
const emit = kind => JSON.parse(execFileSync('go', ['-C', 'server', 'run', './cmd/drivebridge-contracts', kind, '--profile', 'desktop'], { cwd: root, encoding: 'utf8' }));
const document = emit('openapi');
const fixtures = emit('fixtures');
// Freeze D01.1: new binding/topic DTOs must not silently change earlier schemas.
const originalNames = ['KnowledgeSource', 'ReadingRef', 'FileRef', 'PinaxRef', 'ReadingState', 'UpdateReadingState', 'LookupReadingMutation', 'PageQuery', 'SourcePage', 'ReadingStatePage'];
const canonical = value => Array.isArray(value) ? value.map(canonical) : value && typeof value === 'object' ? Object.fromEntries(Object.keys(value).sort().map(key => [key, canonical(value[key])])) : value;
const original = Object.fromEntries(originalNames.map(name => [name, document.components.schemas[name]]));
assert.equal(createHash('sha256').update(JSON.stringify(canonical(original))).digest('hex'), '8e00a3f7d3133ab0d833da90174c650de3508642fa2bbd191e4b0a5e669c8f26');
const bindingNames = ['BindingLookupResult', 'BindingMutationLookup', 'BindingMutationReceipt', 'BindingPreview', 'BindingView', 'ConfirmBinding', 'NativeSelection', 'ReleaseProtection', 'SetSourceVisibility', 'TopicPin', 'TopicPinLookup', 'TopicPinLookupResult', 'TopicPinPage', 'TopicRef', 'UpdateTopicPin'];
const previous = Object.fromEntries([...originalNames, ...bindingNames].map(name => [name, document.components.schemas[name]]));
assert.equal(createHash('sha256').update(JSON.stringify(canonical(previous))).digest('hex'), '8483df38081404d7856d552b44a8f032e6bad3ecb2ecdd23dfc163ba88b67a90');
assert.deepEqual(document.paths, {});
assert.equal(document['x-implementation-status'], 'planned');
const ajv = new Ajv2020({ strict: true, strictRequired: false, allErrors: true });
addFormats(ajv);
const validators = Object.fromEntries(Object.entries(document.components.schemas).map(([name, schema]) => [name, ajv.compile(schema)]));
for (const [name, fixture] of Object.entries(fixtures)) assert.equal(validators[name](fixture), true, `${name}: ${JSON.stringify(validators[name].errors)}`);
const ref = validators.ReadingRef;
for (const fixture of [fixtures.FileRef, fixtures.PinaxRef]) {
  assert.equal(ref(fixture), true);
  for (const field of Object.keys(fixture)) {
    const invalid = { ...fixture }; delete invalid[field];
    assert.equal(ref(invalid), false, `Missing ${field}`);
  }
  for (const extra of [{ vault_root: '/private' }, { bearer: 'fixture-only' }, { kind: 'future' }]) assert.equal(ref({ ...fixture, ...extra }), false);
}
assert.equal(ref({ ...fixtures.FileRef, source_id: 'mixed' }), false);
assert.equal(ref({ ...fixtures.PinaxRef, file_id: 'mixed' }), false);
assert.equal(ref({ ...fixtures.PinaxRef, revision: 'latest' }), false);
const update = validators.UpdateReadingState;
const base = { target_ref: fixtures.FileRef, expected_revision: 0 };
for (const fields of [{ favorite: false }, { read_later: true }, { clear_position: true }, { position: { revision: 'version-fixture', anchor: 'h1', offset: 0 } }]) assert.equal(update({ ...base, ...fields }), true, JSON.stringify(update.errors));
for (const fields of [{}, { favorite: null }, { read_later: null }, { position: null }, { clear_position: false }, { clear_position: true, position: { revision: 'v', anchor: 'h1', offset: 0 } }, { expected_revision: -1 }, { body: 'draft' }, { token: 'fixture' }, { subject: 'spoofed' }]) assert.equal(update({ ...base, ...fields }), false, JSON.stringify(fields));
for (const input of [{ limit: 0 }, { limit: 201 }, { limit: null }, { cursor: 'a'.repeat(4097) }]) assert.equal(validators.PageQuery(input), false);
for (const state of ['future', null]) assert.equal(validators.KnowledgeSource({ ...fixtures.KnowledgeSource, state }), false);
assert.equal(validators.ReadingState({ ...fixtures.ReadingState, position: null }), true);
assert.equal(validators.ReadingState({ ...fixtures.ReadingState, revision: 0 }), false);
for (const name of ['NativeSelection', 'BindingPreview', 'ConfirmBinding', 'SetSourceVisibility', 'ReleaseProtection', 'BindingView', 'BindingMutationLookup', 'BindingMutationReceipt', 'BindingLookupResult', 'TopicRef', 'UpdateTopicPin', 'TopicPin', 'TopicPinPage', 'TopicPinLookup', 'TopicPinLookupResult']) {
  assert.equal(validators[name]({ ...fixtures[name], body: 'forbidden' }), false, name);
  for (const field of document.components.schemas[name].required || []) {
    const missing = { ...fixtures[name] }; delete missing[field];
    assert.equal(validators[name](missing), false, `${name} requires ${field}`);
  }
}
const preview = fixtures.BindingPreview;
for (const fields of [{ scopes: ['writer.editor'] }, { scopes: ['reader.body'] }, { scopes: ['reader.metadata', 'reader.metadata'] }, { decision: 'future' }, { pending_mutations: 1 }, { reason: 'managed_scope_busy' }, { vault_root: '/private' }, { expected_guard_revision: 9007199254740992 }]) assert.equal(validators.BindingPreview({ ...preview, ...fields }), false, JSON.stringify(fields));
assert.equal(validators.BindingPreview({ ...preview, decision: 'blocked', reason: 'managed_scope_unverifiable' }), true);
for (const name of ['ConfirmBinding', 'ReleaseProtection']) {
  assert.equal(validators[name]({ ...fixtures[name], confirmed: false }), false);
  assert.equal(validators[name]({ ...fixtures[name], scopes: ['writer.editor'] }), false);
  assert.equal(validators[name]({ ...fixtures[name], principal: 'spoofed' }), false);
}
assert.equal(validators.SetSourceVisibility({ expected_revision: 1, hidden: false }), true);
assert.equal(validators.SetSourceVisibility({ expected_revision: 1, hidden: true, protection_active: false }), false);
assert.equal(validators.ReleaseProtection({ ...fixtures.ReleaseProtection, expected_revision: 0 }), false);
const binding = fixtures.BindingView;
for (const state of ['active', 'hidden', 'blocked', 'released']) {
  const active = state !== 'released';
  assert.equal(validators.BindingView({ ...binding, state, protection_active: active }), true);
  assert.equal(validators.BindingView({ ...binding, state, protection_active: !active }), false);
}
const receipt = fixtures.BindingMutationReceipt;
assert.equal(validators.BindingLookupResult({ found: true, receipt }), true);
assert.equal(validators.BindingLookupResult({ found: false, receipt }), false);
assert.equal(validators.BindingLookupResult({ found: true, receipt: null }), false);
assert.equal(validators.BindingMutationReceipt({ ...receipt, action: 'release' }), false);
assert.equal(validators.BindingMutationReceipt({ ...receipt, action: 'release', binding: { ...binding, state: 'released', protection_active: false } }), true);
assert.equal(validators.BindingMutationLookup({ action: 'release', idempotency_key: 'original-key' }), true);
assert.equal(validators.BindingMutationLookup({ action: 'rotate', idempotency_key: 'original-key' }), false);
assert.equal(validators.BindingMutationLookup({ action: 'confirm', idempotency_key: '../bad' }), false);
assert.equal(validators.NativeSelection({ native_selection_id: 'opaque', path: '/private' }), false);
assert.equal(validators.UpdateTopicPin({ ...fixtures.UpdateTopicPin, pinned: false }), true);
assert.equal(validators.UpdateTopicPin({ ...fixtures.UpdateTopicPin, target_ref: { ...fixtures.TopicRef, kind: 'folder' } }), false);
assert.equal(validators.TopicPinLookupResult({ found: false, pin: null }), true);
assert.equal(validators.TopicPinLookupResult({ found: true, pin: null }), false);
assert.equal(validators.TopicPinLookupResult({ found: false, pin: fixtures.TopicPin }), false);
for (const name of ['ExportRequest', 'ExportLookup', 'ExportOperation', 'ExportLookupResult', 'ByteRange', 'SourceRangeRequest', 'AssetRangeRequest']) {
  assert.equal(validators[name]({ ...fixtures[name], path: '/private' }), false, name);
  for (const field of document.components.schemas[name].required || []) {
    const missing = { ...fixtures[name] }; delete missing[field];
    assert.equal(validators[name](missing), false, `${name} requires ${field}`);
  }
}
for (const fields of [{ body: 'forbidden' }, { overwrite: true }, { native_selection_id: '' }, { expected_destination_revision: '' }, { principal: 'spoofed' }]) assert.equal(validators.ExportRequest({ ...fixtures.ExportRequest, ...fields }), false);
assert.equal(validators.ExportLookup({ idempotency_key: '../new' }), false);
assert.equal(validators.ExportLookupResult({ found: true, operation: fixtures.ExportOperation }), true);
assert.equal(validators.ExportLookupResult({ found: false, operation: fixtures.ExportOperation }), false);
assert.equal(validators.ExportLookupResult({ found: true, operation: null }), false);
assert.equal(validators.ExportOperation({ ...fixtures.ExportOperation, total_bytes: null }), false);
assert.equal(validators.ExportOperation({ ...fixtures.ExportOperation, ui_state: 'unknown', total_bytes: null }), true);
assert.equal(validators.ExportOperation({ ...fixtures.ExportOperation, ui_state: 'future' }), false);
assert.equal(validators.ByteRange({ start: -1, end: 0 }), false);
assert.equal(validators.ByteRange({ start: 0, end: 9007199254740992 }), false);
assert.equal(validators.AssetRangeRequest({ ...fixtures.AssetRangeRequest, handle: '../file' }), false);
console.log(`Desktop DTO contract checks passed (${Object.keys(validators).length} schemas; no runtime capability claim).`);
