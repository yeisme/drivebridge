import { test, expect } from "@playwright/test";
import { execFileSync, spawn, type ChildProcess } from "node:child_process";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
import { once } from "node:events";
const owner = path.resolve(import.meta.dirname, "../..");
const artifacts =
  process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS ||
  path.resolve("temp/browser-artifacts");
test("local viewer bootstraps once, shows actual local usage and keeps file routes read-only", async ({
  page,
  request,
}) => {
  test.setTimeout(120000);
  const temp = fs.mkdtempSync(
    path.join(os.tmpdir(), "drivebridge-local-view-"),
  );
  let child: ChildProcess | undefined;
  let ownerProcess: ChildProcess | undefined;
  try {
    const binary = path.join(temp, "drivebridge"),
      state = path.join(temp, "state"),
      local = path.join(temp, "local"),
      remote = path.join(temp, "remote"),
      config = path.join(temp, "rclone.conf"),
      launchFile = path.join(temp, "launch.url");
    fs.mkdirSync(local);
    fs.mkdirSync(remote);
    fs.writeFileSync(config, `[fixture]\ntype = alias\nremote = ${remote}\n`, {
      mode: 0o600,
    });
    fs.writeFileSync(path.join(local, "report.md"), "# Local report\n");
    const env = Object.fromEntries(
      Object.entries(process.env).filter(
        ([k]) => !k.startsWith("DRIVEBRIDGE_"),
      ),
    );
    env.XDG_CONFIG_HOME = path.join(temp, "config");
    // Resolve the live sibling module in a disposable graph, never rewriting its lockfiles.
    const modfile = path.join(temp, "viewer.mod");
    fs.copyFileSync(path.join(owner, "go.mod"), modfile);
    fs.copyFileSync(path.join(owner, "go.sum"), path.join(temp, "viewer.sum"));
    execFileSync("go", ["build", "-mod=mod", "-modfile", modfile, "-o", binary, "./cmd/drivebridge"], {
      cwd: owner,
      env,
      timeout: 90000,
      stdio: "pipe",
    });
    fs.mkdirSync(artifacts, {recursive:true});
    fs.copyFileSync(modfile, path.join(artifacts, "local-viewer.mod"));
    fs.copyFileSync(path.join(temp, "viewer.sum"), path.join(artifacts, "local-viewer.sum"));
    const common = ["--state", state, "--rclone-config", config];
    execFileSync(
      binary,
      [
        ...common,
        "setup",
        "--remote",
        "fixture",
        "--remote-path",
        "",
        "--local-root",
        local,
      ],
      { cwd: temp, env, stdio: "pipe" },
    );
    execFileSync(
      binary,
      [
        ...common,
        "upload",
        "--space",
        "personal",
        "--file",
        "report.md",
        "--caller",
        "report-agent",
      ],
      { cwd: temp, env, stdio: "pipe" },
    );
    child = spawn(
      binary,
      [
        ...common,
        "view",
        "--no-open",
        "--launch-file",
        launchFile,
        "--web-dir",
        path.resolve("dist"),
      ],
      { cwd: temp, env, stdio: ["ignore", "pipe", "pipe"] },
    );
    await expect
      .poll(() => fs.existsSync(launchFile), { timeout: 10000 })
      .toBe(true);
    const launch = fs.readFileSync(launchFile, "utf8").trim(),
      base = new URL(launch).origin;
    expect((await request.get(base + "/api/v1/usage/summary")).status()).toBe(
      401,
    );
    await page.goto(launch);
    await expect(page.getByRole("button", { name: /^打开网盘 / }).first()).toBeVisible();
    // Bootstrap credentials must leave the URL before presentation state is added.
    expect(new URL(page.url()).hash).toBe("");
    await page.getByRole("button", { name: /^打开网盘 / }).first().click();
    await page.getByRole("button", { name: "网格视图" }).click();
    await expect(page.locator(".finder-file").filter({ hasText: "report.md" })).toBeVisible();
    await expect(page.getByRole("button", { name: "上传文件", exact: true })).toBeDisabled();
    await page.screenshot({ path: path.join(artifacts, "local-real-owner-offline.png"), fullPage: true });
    await page.getByRole("button",{name:"总览",exact:true}).click();
    await expect(
      page.getByText("本机数据 · 不自动上传", { exact: false }),
    ).toBeVisible();
    expect(new URL(page.url()).hash).toBe("#/drives?selected=personal");
    await expect(page.getByText("受管逻辑用量", { exact: true })).toBeVisible();
    const me = (await (await page.request.get(base + "/api/v1/me")).json())
      .data;
    const summary = (
      await (await page.request.get(base + "/api/v1/usage/summary")).json()
    ).data;
    expect(summary.totals.effective_bytes).toBe(15);
    expect(summary.capacities[0].quota_bytes).toBeNull();
    const denied = await page.request.post(
      base + "/api/v1/files/example/trash",
      { headers: { Origin: base, "X-CSRF-Token": me.csrf_token }, data: {} },
    );
    expect(denied.status()).toBe(403);
    await page.getByRole("tab", { name: "传输用量", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: /有效文件量与应用观测量/ }),
    ).toBeVisible();
    fs.mkdirSync(artifacts, { recursive: true });
    await page.screenshot({
      path: path.join(artifacts, "local-insights.png"),
      fullPage: true,
    });
    await page.setViewportSize({ width: 390, height: 844 });
    await page.getByRole("tab", { name: "安全审计", exact: true }).click();
    await expect(
      page.getByRole("heading", { name: "事件时间线", exact: true }),
    ).toBeVisible();
    expect(await page.evaluate(() => document.documentElement.scrollWidth <= innerWidth)).toBe(true);
    await page.screenshot({
      path: path.join(artifacts, "local-audit-mobile.png"),
      fullPage: true,
    });
    // Storage browser, tier 1: without a running owner the view honestly
    // falls back to the observed index and says why.
    await page.setViewportSize({ width: 1280, height: 800 });
    await page.getByRole("tab", { name: "存储浏览", exact: true }).click();
    await expect(page.getByText(/未检测到运行中的 owner/)).toBeVisible();
    await expect(page.getByText(/report\.md/).first()).toBeVisible();
    // Storage browser, tier 2: start the owner with the insights socket; the
    // same directory becomes a live listing.
    ownerProcess = spawn(
      binary,
      [...common, "mcp", "--enabled", "--insights"],
      { cwd: temp, env, stdio: ["pipe", "ignore", "pipe"] },
    );
    await expect
      .poll(() => fs.existsSync(path.join(state, "insights.sock")), {
        timeout: 15000,
      })
      .toBe(true);
    await page.reload();
    await page.getByRole("button", { name: /^打开网盘 / }).first().click();
    await page.getByRole("button", { name: "网格视图" }).click();
    await expect(page.locator(".finder-file").filter({ hasText: "report.md" })).toBeVisible();
    await expect(page.getByRole("button", { name: "上传文件", exact: true })).toBeEnabled();
    await page.screenshot({ path: path.join(artifacts, "local-real-owner-online.png"), fullPage: true });
    await page.getByRole("button",{name:"总览",exact:true}).click();
    await page.getByRole("tab", { name: "存储浏览", exact: true }).click();
    await expect(
      page.getByText(/实时目录 · 列表会更新已观察索引/),
    ).toBeVisible();
    await expect(page.getByText(/report\.md/).first()).toBeVisible();
    await page.getByRole("button", { name: "复制引用" }).first().click();
    await expect(page.getByText("已复制固定引用，可在已连接的 Agent 中使用。")).toBeVisible();
    await page.screenshot({
      path: path.join(artifacts, "local-storage-browser.png"),
      fullPage: true,
    });
  } finally {
    if (ownerProcess && ownerProcess.exitCode === null) {
      ownerProcess.kill("SIGINT");
      await new Promise<void>((resolve) =>
        setTimeout(() => {
          if (ownerProcess?.exitCode === null) ownerProcess.kill("SIGKILL");
          resolve();
        }, 3000),
      );
    }
    if (child && child.exitCode === null) {
      const closed = once(child, "close");
      child.kill("SIGINT");
      await Promise.race([
        closed,
        new Promise<void>((resolve) =>
          setTimeout(() => {
            if (child?.exitCode === null) child.kill("SIGKILL");
            resolve();
          }, 5000),
        ),
      ]);
    }
    fs.rmSync(temp, { recursive: true, force: true });
  }
});
