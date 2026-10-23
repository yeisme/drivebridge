import { expect, it } from 'vitest';
import { readFile } from 'node:fs/promises';
import { resolve } from 'node:path';
// @ts-expect-error Build-only JavaScript generator.
import { httpContract } from '../scripts/desktop-http-contract.mjs';

async function load() {
 const read = async (name: string) => JSON.parse(await readFile(resolve(process.cwd(), '../contracts/desktop', name), 'utf8'));
 const [ipc, desktop, owner, writerSync] = await Promise.all([read('ipc.json'), read('openapi.json'), read('pinax-reader.openapi.json'), read('pinax-writer-sync.openapi.json')]);
 return { ipc, desktop, owner, writerSync, http: httpContract(ipc['x-methods'], desktop, owner, writerSync) };
}
it('maps every planned method to envelope, original key and exact path parameters', async () => {
 const { ipc, http } = await load();
 for (const m of ipc['x-methods']) {
  const op = http.paths[m.path][m.http_method.toLowerCase()];
  expect(op['x-implementation-status']).toBe('planned');
  expect(op.security).toEqual([{ desktopSession: [] }]);
  expect(op['x-cache-control']).toBe('private, no-store');
  const params = op.parameters;
  expect(params.filter((p: {in:string}) => p.in === 'path').map((p: {name:string})=>p.name)).toEqual([...m.path.matchAll(/\{([^}]+)\}/g)].map((match: RegExpMatchArray)=>match[1]));
  expect(params.some((p: {name:string;required:boolean})=>p.name==='Idempotency-Key'&&p.required)).toBe(m.idempotent);
  if (m.http_method==='GET') expect(op.requestBody).toBeUndefined();
  if (m.name==='chooseVault') expect(op.requestBody.content['application/json'].schema.$ref).toContain('NativeSelection');
  if (m.name==='exportSource') expect(op.requestBody.content['application/json'].schema.$ref).toContain('ExportRequest');
  if (m.name==='readDocument') expect(params.find((p: {name:string})=>p.name==='revision').required).toBe(true);
  if (['syncPush','syncPull'].includes(m.name)) {
   const body=op.requestBody.content['application/json'].schema;
   expect(body.properties['X-Pinax-Operation-ID']).toBeUndefined();
   expect(body.properties['Idempotency-Key']).toBeUndefined();
   expect(body.additionalProperties).toBe(false);
   expect(body.$id).toBeUndefined();
  }
  for (const response of Object.values(op.responses) as {content:Record<string,{schema:{required:string[]}}> }[]) expect(response.content['application/json'].schema.required).toHaveLength(1);
 }
 const assets=http.paths['/v1/desktop/assets/{handle}'].get;
 expect(assets['x-max-range-bytes']).toBe(8388608);
 expect(assets['x-main-asset-channel-only']).toBe(true);
 expect(assets.responses['206'].headers['Content-Range'].required).toBe(true);
 expect(assets.responses['200'].content['application/octet-stream']).toBeDefined();
 expect(assets.responses['416']).toBeDefined();
});
it('rejects duplicate routes and schema drift',async()=>{
 const {ipc,desktop,owner,writerSync}=await load();
 expect(()=>httpContract([...ipc['x-methods'],ipc['x-methods'][0]],desktop,owner,writerSync)).toThrow();
 expect(()=>httpContract([{...ipc['x-methods'][0],response_schema:'missing'}],desktop,owner,writerSync)).toThrow();
});
