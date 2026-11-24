import { expect, test } from "@playwright/test";
import path from "node:path";

const adminKey = `dbu_${"2".repeat(96)}`;

test("?token= auto-injects login, scrubs URL immediately, enforces one-time invite consumption, and supports smart paste", async ({
  page,
  browser,
}) => {
  const artifacts = process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS;
  await page.setViewportSize({ width: 1440, height: 920 });

  // 1. Admin logs in directly via ?token= auto-injection and verifies URL scrubbing
  await page.goto(`/?token=${encodeURIComponent(adminKey)}#/users`);
  await expect(page.getByRole("heading", { name: "访问凭据", exact: true })).toBeVisible();
  expect(page.url()).not.toContain("token=");
  expect(page.url()).not.toContain("dbu_");

  // 2. Issue a one-time invitation user via the dedicated invite button
  await page.getByRole("button", { name: "签发新用户 / 一次性邀请链接", exact: true }).click();
  await page.locator("textarea[name=names]").fill("受邀设计师小王");
  await page.getByRole("button", { name: "签发", exact: true }).click();

  const issuedDialog = page.getByRole("dialog", { name: "保存新签发的密钥" });
  await expect(issuedDialog).toContainText("一次性免密邀请");
  const oneTimeToken = await issuedDialog.locator("textarea.token-value").first().inputValue();
  expect(oneTimeToken).toMatch(/^dbu_[0-9a-f]{96}$/);
  if (artifacts) {
    await page.screenshot({ path: path.join(artifacts, "one-time-token-issued-1440.png"), animations: "disabled" });
  }
  await page.getByRole("button", { name: "我已保存，关闭", exact: true }).click();

  // 3. First guest opens ?token=<oneTimeToken>: lands straight in personal cloud drive!
  const guest1Context = await browser.newContext({ viewport: { width: 1440, height: 920 } });
  const guest1 = await guest1Context.newPage();
  const baseUrl = new URL(page.url()).origin;
  await guest1.goto(`${baseUrl}/?token=${encodeURIComponent(oneTimeToken)}`);
  await expect(guest1.getByRole("region", { name: "文件浏览", exact: true })).toBeVisible();
  expect(guest1.url()).not.toContain("token=");
  await expect(guest1.getByText("已通过专属直连链接自动登录")).toBeVisible();
  // Refreshing guest1 keeps their exchanged cookie session alive
  await guest1.reload();
  await expect(guest1.getByRole("region", { name: "文件浏览", exact: true })).toBeVisible();

  // 4. Second guest (or forwarded link) tries the exact same ?token=<oneTimeToken>: rejected!
  const guest2Context = await browser.newContext({ viewport: { width: 1440, height: 920 } });
  const guest2 = await guest2Context.newPage();
  await guest2.goto(`${baseUrl}/?token=${encodeURIComponent(oneTimeToken)}`);
  await expect(guest2.locator("#login-key")).toBeVisible();
  await expect(guest2.getByRole("alert")).toContainText("已失效");
  expect(guest2.url()).not.toContain("token=");

  // 5. Smart paste in KeyLoginForm: pasting Baidu-Netdisk-style multi-line invite block extracts key
  const multiLineSnippet = `邀请你使用 DriveBridge 云盘\n链接：${baseUrl}/?token=${adminKey}\n说明：点击或复制整段粘贴即可登录`;
  await guest2.locator("#login-key").fill(multiLineSnippet);
  await guest2.getByRole("button", { name: "登录", exact: true }).click();
  await expect(guest2.getByRole("button", { name: "账户菜单" })).toBeVisible();

  await guest1Context.close();
  await guest2Context.close();

  // 6. Admin KeyDirectory shows the one-time token as "已使用"
  await page.reload();
  await expect(
    page.getByRole("row").filter({ hasText: "受邀设计师小王" }).getByText("已使用", { exact: true }),
  ).toBeVisible();
});
