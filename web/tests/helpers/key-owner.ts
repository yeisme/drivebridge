import { expect, type APIRequestContext } from "@playwright/test";

// Each test owns its disposable identity; another spec may rotate the seeded key.
export async function freshOwnerKey(request: APIRequestContext, name: string) {
  const login = await request.post("/auth/key", { data: { key: "dbu_" + "2".repeat(96) } });
  expect(login.ok()).toBe(true);
  const me = (await (await request.get("/api/v1/me")).json()).data;
  const response = await request.post("/api/v1/users", {
    headers: { Origin: new URL(login.url()).origin, "X-CSRF-Token": me.csrf_token },
    data: { users: [{ name: `${name}-${Date.now()}` }] },
  });
  expect(response.ok()).toBe(true);
  return (await response.json()).data.issued[0].key as string;
}
