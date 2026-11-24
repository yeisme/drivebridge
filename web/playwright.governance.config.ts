import { defineConfig } from "@playwright/test";
import base from "./playwright.config";
import path from "node:path";
export default defineConfig({
  ...base,
  testMatch: ["governance.spec.ts"],
  webServer: {
    ...(base.webServer as object),
    command: `go run ./cmd/drivebridge-fixture --listen 127.0.0.1:${process.env.DRIVEBRIDGE_TEST_PORT || 31878} --web-dir ${JSON.stringify(process.env.DRIVEBRIDGE_WEB_DIR ? path.resolve(process.env.DRIVEBRIDGE_WEB_DIR) : path.resolve(import.meta.dirname, "dist"))} --governance`,
  },
});
