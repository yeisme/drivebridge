import { defineConfig } from "@playwright/test";
import fs from "node:fs";
import os from "node:os";
import path from "node:path";
const browserRoots = [...new Set([
  process.env.PLAYWRIGHT_BROWSERS_PATH && process.env.PLAYWRIGHT_BROWSERS_PATH !== "0" ? process.env.PLAYWRIGHT_BROWSERS_PATH : "",
  path.join(os.homedir(), ".cache/ms-playwright"),
].filter(Boolean))];
const candidates = browserRoots.flatMap((root) => {
  if (!fs.existsSync(root)) return [];
  return fs.readdirSync(root).filter((x) => /^chromium-\d+$/.test(x)).map((x) => path.join(root, x));
}).sort().reverse();
const executable =
  process.env.PLAYWRIGHT_CHROMIUM ||
  candidates
    .map((x) => path.join(x, "chrome-linux64/chrome"))
    .find((x) => fs.existsSync(x));
const port = Number(process.env.DRIVEBRIDGE_TEST_PORT || 31878);
const webDir = process.env.DRIVEBRIDGE_WEB_DIR
  ? path.resolve(process.env.DRIVEBRIDGE_WEB_DIR)
  : path.resolve(import.meta.dirname, "dist");
export default defineConfig({
  testDir: "tests",
  timeout: 60000,
  workers: 1,
  reporter: "list",
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    launchOptions: executable ? { executablePath: executable } : {},
    trace: "off",
    screenshot: "off",
    video: "off",
  },
  webServer: {
    command: `go run ./cmd/drivebridge-fixture --listen 127.0.0.1:${port} ${process.env.DRIVEBRIDGE_TEST_LABEL_MANAGEMENT === "1" ? "--label-management" : ""} ${process.env.DRIVEBRIDGE_TEST_HOLD_MANAGEMENT === "1" ? "--hold-management" : ""} ${process.env.DRIVEBRIDGE_TEST_SHARE_RECEIVERS === "1" ? "--share-receivers" : ""} ${process.env.DRIVEBRIDGE_TEST_AUTOMATION_MANAGEMENT === "1" ? "--automation-management" : ""} ${process.env.DRIVEBRIDGE_TEST_PROPERTY_EDITING === "1" ? "--property-editing" : ""} ${process.env.DRIVEBRIDGE_TEST_TRANSFER_PLANS === "1" ? "--transfer-plans" : ""} ${process.env.DRIVEBRIDGE_TEST_ARCHIVE_FAILURE === "1" ? "--fail-archive-publish-once" : ""} ${process.env.DRIVEBRIDGE_TEST_TEAM_DEMO === "1" ? "--demo --governance" : process.env.DRIVEBRIDGE_TEST_DEMO === "1" ? "--demo" : ""} --web-dir ${JSON.stringify(webDir)}`,
    cwd: path.resolve(import.meta.dirname, "../server"),
    url: `http://127.0.0.1:${port}/healthz`,
    reuseExistingServer: false,
    timeout: 120000,
    gracefulShutdown: { signal: "SIGINT", timeout: 5000 },
  },
});
