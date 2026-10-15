import fs from "node:fs";
const p = JSON.parse(fs.readFileSync("package.json", "utf8"));
p.name = "drivebridge-client";
p.version = "0.1.2";
p.private = true;
p.type = "module";
p.scripts = {
  dev: "vite --host 127.0.0.1",
  build: "tsc --noEmit && vite build",
  test: "python3 scripts/check.py -- pnpm exec vitest run",
  "test:e2e": "python3 scripts/check.py -- pnpm exec playwright test",
  typecheck: "tsc --noEmit",
};
fs.writeFileSync("package.json", JSON.stringify(p, null, 2) + "\n");
// Update the CLI-created TypeScript skeleton through the project configurator.
const ts = await import("typescript");
const raw = fs.readFileSync("tsconfig.json", "utf8");
const parsed = ts.parseConfigFileTextToJson("tsconfig.json", raw).config;
parsed.compilerOptions.types = ["vite/client", "node"];
parsed.compilerOptions.paths = { ...parsed.compilerOptions.paths, "@/*": ["./src/*"] };
fs.writeFileSync("tsconfig.json", JSON.stringify(parsed, null, 2) + "\n");
