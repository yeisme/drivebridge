import { test, expect } from "@playwright/test";

// Smoke: Aigora unified-login governance codes surface actionable Chinese
// guidance on the login page instead of a generic failure.
test("unified_login_required shows actionable guidance on the login page", async ({
  page,
}) => {
  await page.route("**/auth/key", (route) =>
    route.fulfill({
      status: 403,
      contentType: "application/json",
      body: JSON.stringify({
        error: { code: "unified_login_required", message: "fixture" },
      }),
    }),
  );
  await page.goto("/");
  await page.locator("#login-key").fill("dbu_" + "1".repeat(96));
  await page.getByRole("button", { name: "登录", exact: true }).click();
  const alert = page.getByRole("alert");
  await expect(alert).toBeVisible();
  await expect(alert).toContainText("统一登录");
  await expect(alert).toContainText("Aigora");
});
