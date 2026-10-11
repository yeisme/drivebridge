// Tab-local scope only: no credentials, file cache, or server session mutation.
export interface DataContext {
  readonly tenant: string;
  readonly space: string;
  readonly revision: number;
}
let dataContext: DataContext = Object.freeze({ tenant: "", space: "", revision: 0 });

export function getDataContext() { return dataContext; }

export function setDataContext(tenant: string, space = "") {
  dataContext = Object.freeze({ tenant, space, revision: dataContext.revision + 1 });
  window.dispatchEvent(new Event("drivebridge-context"));
}

export function contextHeaders(includeSpace = true): Record<string, string> {
  return {
    ...(dataContext.tenant ? { "X-DriveBridge-Tenant": dataContext.tenant } : {}),
    ...(includeSpace && dataContext.space ? { "X-DriveBridge-Space": dataContext.space } : {}),
  };
}

export function contextURL(path: string) {
  const url = new URL(path, location.origin);
  if (dataContext.tenant) url.searchParams.set("tenant_context", dataContext.tenant);
  if (dataContext.space) url.searchParams.set("space_context", dataContext.space);
  return url.pathname + url.search + url.hash;
}
