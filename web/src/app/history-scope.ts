// Encoded segments prevent ambiguous concatenation between identities.
export function historyScope(instance: string, user: string, tenant: string, space: string, page: "transfers" | "inbox") {
  if (!instance || !user) return "";
  return [instance, user, tenant, space, page].map(encodeURIComponent).join(":");
}
