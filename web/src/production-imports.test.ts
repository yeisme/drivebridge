import { expect, it } from "vitest";
import { readdir, readFile } from "node:fs/promises";
import { join } from "node:path";
// @ts-expect-error Node quality checker is a JavaScript build tool.
import { prototypeImports } from "../scripts/production-imports.mjs";

it.each([
  'import { state } from "../../prototypes/demo/model";',
  'export * from "../prototype/state";',
  'export { state } from "@/prototypes/demo";',
  'const page = import("../prototypes/demo");',
  'const data = require("../prototypes/demo");',
  'type Model = import("../prototypes/demo").Model;',
])("rejects a prototype dependency: %s", source => {
  expect(prototypeImports(source, "app.tsx")).toHaveLength(1);
});

it("does not flag comments, data strings or normal production imports", () => {
  expect(prototypeImports('// prototype sample\nconst label="prototypes/demo"; import { api } from "./api";', "app.ts")).toEqual([]);
});

it("keeps current production TypeScript modules free of prototype dependencies", async () => {
  const findings: string[] = [];
  async function walk(directory: string) {
    for (const entry of await readdir(directory, { withFileTypes: true })) {
      const path = join(directory, entry.name);
      if (entry.isDirectory()) await walk(path);
      else if (/\.tsx?$/.test(entry.name) && !entry.name.includes(".test.")) {
        findings.push(...prototypeImports(await readFile(path, "utf8"), path));
      }
    }
  }
  await walk(join(process.cwd(), "src"));
  expect(findings).toEqual([]);
});
