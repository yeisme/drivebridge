import { defineConfig } from "@playwright/test";
import base from "../../playwright.config";
export default defineConfig({
  testDir: import.meta.dirname,
  testMatch: [
    "v92.spec.ts",
    "v91.spec.ts",
    "v9.spec.ts",
    "navigation-hover.spec.ts",
    "v8.spec.ts",
    "v7.spec.ts",
    "v7-regression.spec.ts",
    "v6.spec.ts",
  ],
  timeout: 30000,
  workers: 1,
  retries: 0,
  reporter: "list",
  outputDir: "../../temp/prototype-browser-output",
  use: {
    ...base.use,
    baseURL: "http://127.0.0.1:31928",
    trace: "off",
    screenshot: "off",
    video: "off",
  },
  projects: [
    { name: "desktop", use: { viewport: { width: 1440, height: 1000 } } },
    {
      name: "mobile",
      use: {
        viewport: { width: 390, height: 844 },
        isMobile: true,
        hasTouch: true,
      },
    },
  ],
  webServer: {
    command:
      "python3 -m http.server 31928 --bind 127.0.0.1 --directory temp/personal-agent-prototype",
    cwd: new URL("../../", import.meta.url).pathname,
    url: "http://127.0.0.1:31928/opendesign.html",
    reuseExistingServer: false,
    timeout: 15000,
  },
});
