import { expect, it } from "vitest";
import { readFile, readdir } from "node:fs/promises";
import { join, relative } from "node:path";

it("keeps shell geometry tokens in one canonical stylesheet", async () => {
  const root = join(process.cwd(), "src");
  const names = ["navigation-expanded-width", "navigation-rail-width", "workspace-topbar-height", "radius-control", "radius-surface", "radius-finder", "icon-button"];
  const declarations = new Map(names.map(name => [name, [] as string[]]));
  async function walk(directory: string) {
    for (const entry of await readdir(directory, { withFileTypes: true })) {
      const file = join(directory, entry.name);
      if (entry.isDirectory()) await walk(file);
      else if (entry.name.endsWith(".css")) {
        const source = await readFile(file, "utf8");
        for (const name of names) {
          const matches = source.match(new RegExp(`--${name}\\s*:`, "g")) || [];
          for (const _ of matches) declarations.get(name)!.push(relative(root, file));
        }
      }
    }
  }
  await walk(root);
  for (const [name, files] of declarations) expect(files, name).toEqual(["app/theme/tokens.css"]);
  const source = await readFile(join(root, "app/theme/tokens.css"), "utf8");
  expect(source).toContain("--navigation-expanded-width: 184px;");
  expect(source).toContain("--navigation-rail-width: 56px;");
  expect(source).toContain("--workspace-topbar-height: 48px;");
  expect(source).toContain("--radius-control: 6px;");
  expect(source).toContain("--radius-surface: 8px;");
  expect(source).toContain("--radius-finder: 12px;");
  expect(source).toContain("--ui-radius: 8px;");
  expect(source).toContain("--ui-background: #f5f6f4;");
  expect(source.split(".dark {")[1]).toContain("--ui-border: #587563;");
});
