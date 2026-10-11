import { defineConfig } from "vitest/config";
export default defineConfig({
  test: {
    environment: "node",
    include: ["prototypes/personal-agent-workspace/*.test.ts"],
  },
});
