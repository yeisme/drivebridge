// Focused contract validation. This does not claim endpoint implementation.
import { execFileSync } from 'node:child_process';
import { fileURLToPath } from 'node:url';
import Ajv2020 from 'ajv/dist/2020.js';
import addFormats from 'ajv-formats';
import assert from 'node:assert/strict';

const root = fileURLToPath(new URL('../../', import.meta.url));
const document = JSON.parse(execFileSync('go', ['-C', 'server', 'run', './cmd/drivebridge-contracts', 'openapi'], { cwd: root, encoding: 'utf8' }));
const ajv = new Ajv2020({ strict: true, strictRequired: false, allErrors: true });
addFormats(ajv);
const schemas = document.components.schemas;
const automationChange = ajv.compile(schemas.AutomationChangeRequest);
const automationInput = { action: 'add_label', action_detail: 'rule:reviewed', prefix: 'references', budget_items: 10, enabled: false, clear_pause: false, expected_revision: 0 };
assert.equal(automationChange(automationInput), true);
for (const change of [{ action: 'delete' }, { budget_items: 0 }, { budget_items: 201 }, { enabled: null }, { clear_pause: undefined }, { expected_revision: -1 }, { owner: 'spoof' }]) assert.equal(automationChange({ ...automationInput, ...change }), false);
const automationExecute = ajv.compile(schemas.AutomationExecuteRequest);
assert.equal(automationExecute({ expected_revision: 1, preview_digest: 'a'.repeat(64) }), true);
for (const value of [{ expected_revision: 0, preview_digest: 'a'.repeat(64) }, { expected_revision: 1, preview_digest: 'bad' }, { expected_revision: 1, preview_digest: 'a'.repeat(64), owner: 'spoof' }]) assert.equal(automationExecute(value), false);
const create = ajv.compile(schemas.CreateGrantRequest);
const patch = ajv.compile(schemas.PatchGrantRequest);
const organization = ajv.compile(schemas.PatchOrganizationRequest);
const base = { name: 'Reader', protocol: 'mcp', space_id: 'space-fixture', connection_id: 'connection-fixture', prefix: '', actions: ['files.read'] };
assert.equal(create(base), true, JSON.stringify(create.errors));
for (const seconds of [3600, 86400, 604800, 2592000, 7776000, 31536000]) {
  assert.equal(create({ ...base, expiry_mode: 'duration', expires_in_seconds: seconds }), true);
}
for (const invalid of [
  { expiry_mode: null }, { expiry_mode: 'unknown' }, { expiry_mode: 'duration' },
  { expires_in_seconds: 3600 }, { expiry_mode: 'never', expires_in_seconds: null },
  { expiry_mode: 'duration', expires_in_seconds: 42 }, { actions: ['write'] },
  { actions: ['files.read', 'files.read'] }, { actions: [] }, { name: '' },
  { owner: 'spoofed' },
]) assert.equal(create({ ...base, ...invalid }), false, JSON.stringify(invalid));
assert.equal(patch({ expected_revision: 1, enabled: false }), true);
assert.equal(patch({ expected_revision: 1, enabled: null }), false);
assert.equal(patch({ expected_revision: 1, expires_in_seconds: 3600 }), false);
assert.equal(organization({ expected_revision: 0, group_id: null, pinned: false }), true);
assert.equal(organization({ expected_revision: 0, pinned: null }), false);
for (const schema of Object.values(schemas)) ajv.compile(schema);
const fixtures = JSON.parse(execFileSync('go', ['-C', 'server', 'run', './cmd/drivebridge-contracts', 'fixtures'], { cwd: root, encoding: 'utf8' }));
for (const [name, fixture] of Object.entries(fixtures)) {
  const validate = ajv.compile(schemas[name]);
  assert.equal(validate(fixture), true, `${name}: ${JSON.stringify(validate.errors)}`);
}
const resource = ajv.compile(schemas.RegisterResourceRequest);
const resourceBase = {space_id:"space",kind:"skill",display_name:"Synthetic",file_ref:"drivebridge://instance/file/f/version/v"};
assert.equal(resource(resourceBase),true);
assert.equal(resource({space_id:"space",kind:"reference_pack",display_name:"Pack",collection_snapshot_id:"snapshot"}),true);
for(const invalid of [{...resourceBase,collection_snapshot_id:"snapshot"},{...resourceBase,kind:"reference_pack"},{...resourceBase,file_ref:null},{...resourceBase,file_ref:""},{space_id:"space",kind:"skill",display_name:"Missing"},{...resourceBase,owner:"spoof"}]) assert.equal(resource(invalid),false);
const checkRequest = ajv.compile(schemas.ResourceCheckRequest);
assert.equal(checkRequest({expected_revision:1,checker_version:"utf8-integrity-v1"}),true);
for(const invalid of [{expected_revision:0,checker_version:"utf8-integrity-v1"},{expected_revision:1,checker_version:"execute"},{expected_revision:1},{expected_revision:1,checker_version:"utf8-integrity-v1",owner:"spoof"}]) assert.equal(checkRequest(invalid),false);
const checkReceipt=ajv.compile(schemas.ResourceIntegrityReceiptEnvelope);
assert.equal(checkReceipt({data:{...fixtures.ResourceIntegrityReceiptEnvelope.data,check_scope:"safe_to_execute"}}),false);
assert.equal(checkReceipt({data:{...fixtures.ResourceIntegrityReceiptEnvelope.data,state:"installed"}}),false);
const collectionCreate = ajv.compile(schemas.CreateFileCollectionRequest);
const collectionInput = {space_id:'space-fixture',name:'Rule',kind:'rule',scope:'private',rule:{mode:'exact'}};
assert.equal(collectionCreate(collectionInput),true);
assert.equal(collectionCreate({...collectionInput,kind:'manual',rule:undefined}),true);
assert.equal(collectionCreate({...collectionInput,kind:'manual',rule:{legacy:'ignored'}}),true);
assert.equal(collectionCreate({...collectionInput,kind:'manual',rule:null}),true);
const ruleSchema = ajv.compile(schemas.CollectionRule);
const conditions = [
 {field:'path_prefix',op:'contains',value:'report'},
 {field:'mime_type',op:'eq',value:'text/plain'},
 {field:'caller',op:'eq',value:''},
 {field:'labels.manual',op:'ne',value:'skip'},
 {field:'updated_at',op:'gte',value:'2026-01-01T00:00:00Z'},
 {field:'size_bytes',op:'lt',value:9007199254740991},
 {field:'trashed',op:'eq',value:false},
];
for(const condition of conditions) assert.equal(ruleSchema({mode:'exact',filters:[condition],sort:[{field:'size_bytes',direction:'desc'}]}),true,JSON.stringify(ruleSchema.errors));
for(const change of [{rule:undefined},{rule:null},{rule:{mode:'duplicates'}},{owner:'spoof'},{kind:'invalid'},{scope:'public'},{name:''}]) assert.equal(collectionCreate({...collectionInput,...change}),false);
for(const condition of [
 {field:'path_prefix',op:'eq',value:'x'},{field:'mime_type',op:'contains',value:'text'},
 {field:'size_bytes',op:'gt',value:'1'},{field:'size_bytes',op:'eq',value:1},{field:'size_bytes',op:'lt',value:1.5},
 {field:'trashed',op:'eq',value:true},{field:'updated_at',op:'gt',value:'yesterday'},
 {field:'labels.manual',op:'contains',value:''},{field:'caller',op:'eq',value:null},
 {field:'caller',op:'eq',value:'x',unknown:1},
]) assert.equal(collectionCreate({...collectionInput,rule:{mode:'exact',filters:[condition]}}),false,JSON.stringify(condition));
for(const change of [{space_ids:[]},{space_ids:['a','b']},{filters:null},{filters:Array(65).fill(conditions[0])},{sort:[{field:'name',direction:'asc'},{field:'name',direction:'desc'}]},{sort:[{field:'caller',direction:'asc'}]},{cursor:'old'},{limit:201},{unknown:1}]) assert.equal(ruleSchema({mode:'exact',...change}),false);
const collectionReceipt = ajv.compile(schemas.CollectionCreationReceiptEnvelope);
for (const change of [{result_revision:2},{request_digest:"invalid"},{members_total:0}]) assert.equal(collectionReceipt({data:{...fixtures.CollectionCreationReceiptEnvelope.data,...change}}),false);
const membersChange = ajv.compile(schemas.CollectionMembersChangeRequest);
assert.equal(membersChange({expected_revision:1,include:[],exclude:[]}),true);
for(const value of [{expected_revision:0,include:[],exclude:[]},{expected_revision:1,include:null,exclude:[]},{expected_revision:1,include:["a".repeat(48),"a".repeat(48)],exclude:[]},{expected_revision:1,include:["bad"],exclude:[]},{expected_revision:1,include:[],exclude:[],owner:"spoof"}]) assert.equal(membersChange(value),false);
const membersReceipt = ajv.compile(schemas.CollectionMembersReceiptEnvelope);
for(const change of [{result_revision:1},{request_digest:"bad"},{include:[]}]) assert.equal(membersReceipt({data:{...fixtures.CollectionMembersReceiptEnvelope.data,...change}}),false);
const memberDecisions = ajv.compile(schemas.CollectionMemberDecisionsEnvelope);
for(const change of [{revision:0},{items:null},{items:[{file_id:"a".repeat(48),space_id:"s",path:"a",decision:"unknown"}]}]) assert.equal(memberDecisions({data:{...fixtures.CollectionMemberDecisionsEnvelope.data,...change}}),false);
const snapshotCreate = ajv.compile(schemas.SnapshotCreationRequest);
assert.equal(snapshotCreate({expected_revision:1}),true);
for(const value of [{},{expected_revision:null},{expected_revision:0},{expected_revision:1,owner:"spoof"}]) assert.equal(snapshotCreate(value),false);
const snapshotReceipt = ajv.compile(schemas.SnapshotCreationReceiptEnvelope);
for(const change of [{collection_revision:0},{request_digest:"bad"},{items:[]}]) assert.equal(snapshotReceipt({data:{...fixtures.SnapshotCreationReceiptEnvelope.data,...change}}),false);
const definitionUpdate = ajv.compile(schemas.PropertyDefinitionUpdate);
const definition = {key:"status",display_name:"Status",group:"",type:"text"};
assert.equal(definitionUpdate({expected_revision:0,definitions:[definition]}),true);
assert.equal(definitionUpdate({expected_revision:0,definitions:[]}),true);
for(const row of [{...definition,type:"script"},{...definition,key:"Status"},{...definition,group:null},{key:"status",display_name:"Status",type:"text"},{...definition,owner:"spoof"}]) assert.equal(definitionUpdate({expected_revision:1,definitions:[row]}),false);
for(const input of [{expected_revision:-1,definitions:[]},{expected_revision:0,definitions:null},{expected_revision:0,definitions:Array(201).fill(definition)}]) assert.equal(definitionUpdate(input),false);
const definitionReceipt = ajv.compile(schemas.PropertyDefinitionReceiptEnvelope);
for(const change of [{result_revision:0},{request_digest:"bad"},{definitions:[]}]) assert.equal(definitionReceipt({data:{...fixtures.PropertyDefinitionReceiptEnvelope.data,...change}}),false);
const sidecarPatch = ajv.compile(schemas.PropertySidecarPatch);
const sidecarBase = {base_version_id:'a'.repeat(48),definition_revision:1,expected_revision:0,updates:{caption:'Synthetic',rank:1.25,active:true,tags:['a','b']},remove_keys:[]};
assert.equal(sidecarPatch(sidecarBase),true);
for(const change of [{updates:null},{updates:{caption:null}},{updates:{nested:{bad:true}}},{updates:{'bad-key':'x'}},{updates:{tags:[1]}},{remove_keys:['a','a']},{expected_revision:-1},{owner:'spoof'}]) assert.equal(sidecarPatch({...sidecarBase,...change}),false);
const sidecarView = ajv.compile(schemas.PropertySidecarViewEnvelope);
for(const change of [{storage_kind:'frontmatter'},{values:null},{values:{caption:{untyped:true}}}]) assert.equal(sidecarView({data:{...fixtures.PropertySidecarViewEnvelope.data,...change}}),false);
const snapshotIndex = ajv.compile(schemas.SnapshotIndexEnvelope);
const indexBase = fixtures.SnapshotIndexEnvelope.data;
assert.equal(snapshotIndex({data:{...indexBase,items:[]}}),true);
for (const change of [{items:null},{items:Array(1001).fill(indexBase.items[0])},{entries_total:1},{next_cursor:'invented'},{request_digest:'a'.repeat(64)}]) assert.equal(snapshotIndex({data:{...indexBase,...change}}),false);
for (const change of [{snapshot_id:'bad'},{collection_revision:0},{collection_revision:9007199254740992},{created_at:'bad'},{entries_visible:1},{file_ref:'forbidden'}]) assert.equal(snapshotIndex({data:{...indexBase,items:[{...indexBase.items[0],...change}]}}),false);
const snapshotIndexOperation = document.paths['/api/v1/file-collections/{id}/snapshots'].get;
assert.equal(snapshotIndexOperation['x-human-only'],true);
assert.equal(snapshotIndexOperation.parameters.some(item=>item.in==='query'),false);
assert.ok(snapshotIndexOperation.responses['429']);
const grant = ajv.compile(schemas.GrantView);
for (const field of ['expiry_mode', 'expires_at', 'expiry_changed_at']) {
  const invalid = { ...fixtures.GrantView };
  delete invalid[field];
  assert.equal(grant(invalid), false, `Missing ${field} must fail`);
}
assert.equal(grant({ ...fixtures.GrantView, expiry_mode: 'unknown' }), false);
assert.equal(grant({ ...fixtures.GrantView, expiry_mode: 'duration', expires_at: null }), false);
assert.equal(grant({ ...fixtures.GrantView, expires_at: '2026-01-02T00:00:00Z' }), false);
assert.equal(grant({ ...fixtures.GrantView, expiry_mode: 'duration', expires_at: '2026-01-02T00:00:00Z' }), true);
const submission = ajv.compile(schemas.CreateSubmissionRequest);
const upload = { name: 'fixture.txt', mime_type: 'text/plain', size_bytes: 0, sha256: 'a'.repeat(64) };
assert.equal(submission(upload), true);
for (const invalid of [{ path: 'formal/overwrite.txt' }, { file_id: 'existing' }, { grant_id: 'other' }, { purpose: 'ordinary_upload' }, { size_bytes: -1 }, { sha256: 'bad' }]) {
  assert.equal(submission({ ...upload, ...invalid }), false, JSON.stringify(invalid));
}
const suggestion = ajv.compile(schemas.LabelSuggestionRequest);
assert.equal(suggestion({ version_id: 'version-fixture', labels: ['reference'] }), true);
for (const labels of [[], [''], ['x'.repeat(65)], Array(101).fill('reference')]) {
  assert.equal(suggestion({ version_id: 'version-fixture', labels }), false);
}
const grantEnvelope = ajv.compile(schemas.GrantEnvelope);
assert.equal(grantEnvelope({ data: fixtures.GrantView }), true);
assert.equal(grantEnvelope(fixtures.GrantView), false);
assert.equal(grantEnvelope({ data: { ...fixtures.GrantView, expiry_mode: 'duration', expires_at: null } }), false);
const replay = ajv.compile(schemas.GrantReplayEnvelope);
const replayData = { grant: fixtures.GrantView, credential: { state: 'not_replayable' } };
assert.equal(replay({ data: replayData }), true);
assert.equal(replay({ data: { ...replayData, credential: { state: 'not_replayable', secret: 'synthetic-forbidden' } } }), false);
const issued = ajv.compile(schemas.GrantIssuedEnvelope);
assert.equal(issued({ data: { grant: fixtures.GrantView, credential: { state: 'issued', secret: 'synthetic-only' } } }), true);
assert.equal(issued({ data: replayData }), false);
const grantWrite = ajv.compile(schemas.GrantWriteEnvelope);
assert.equal(grantWrite({ data: replayData }), true);
assert.equal(grantWrite({ data: { grant: fixtures.GrantView, credential: { state: 'issued', secret: 'synthetic-only' } } }), true);
assert.equal(grantWrite({ data: { ...replayData, credential: { state: 'not_replayable', secret: 'synthetic-forbidden' } } }), false);
const grantPage = ajv.compile(schemas.GrantPageEnvelope);
const page = { items: [fixtures.GrantView], next_cursor: '', observed_at: '2026-01-01T00:00:00Z', total: null, total_kind: 'unknown' };
assert.equal(grantPage({ data: page }), true);
assert.equal(grantPage({ data: { ...page, total: 0 } }), false);
assert.equal(grantPage({ data: { ...page, total_kind: 'exact' } }), false);
assert.equal(grantPage({ data: { ...page, total_kind: 'exact', total: 1 } }), true);
assert.equal(grantPage({ data: { ...page, items: [{ ...fixtures.GrantView, expiry_mode: 'duration', expires_at: null }] } }), false);
const decideSuggestion = ajv.compile(schemas.SuggestionDecisionRequest);
for (const [decision, labels, valid] of [['accept', ['reference'], true], ['reject', undefined, true], ['accept', undefined, false], ['reject', ['reference'], false], ['accept', ['reference', 'reference'], false], ['accept', [' '], false]]) {
  const input = { expected_revision: 1, base_version_id: 'version-fixture', decision };
  if (labels !== undefined) input.labels = labels;
  assert.equal(decideSuggestion(input), valid, JSON.stringify(input));
}
console.log(`Contract schema positive and negative cases passed (${Object.keys(schemas).length} schemas; no endpoint calls).`);
