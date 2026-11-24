import path from 'node:path';
import { defineConfig } from "@playwright/test";

// Electron end-to-end project. It launches the real desktop main process with the
// built Web artifact and the shipped host bundle; it never starts the fixture web
// server used by the browser project.
export default defineConfig({
  testDir: "tests-electron",
  outputDir: process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS ? path.join(process.env.DRIVEBRIDGE_EVIDENCE_ARTIFACTS, "electron-test-results") : "test-results",
  timeout: 240000,
  workers: 1,
  reporter: "list",
  use: { trace: "off", screenshot: "off", video: "off" },
});
