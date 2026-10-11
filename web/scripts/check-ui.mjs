import { readdir, readFile } from "node:fs/promises";
import ts from "typescript";
import { pathToFileURL } from "node:url";
import { prototypeImports } from "./production-imports.mjs";
export async function checkUI(root = new URL("../src/", import.meta.url)) {
const failures = [];
async function walk(url, relative = "") {
  for (const entry of await readdir(url, { withFileTypes: true })) {
    const name = relative ? `${relative}/${entry.name}` : entry.name;
    if (entry.isDirectory()) {
      await walk(new URL(`${entry.name}/`, url), name);
      continue;
    }
    if (!/\.(ts|tsx|css)$/.test(entry.name) || entry.name.includes(".test."))
      continue;
    const text = await readFile(new URL(entry.name, url), "utf8");
    if (entry.name.endsWith(".css")) {
      if (/\.(?:button|ui-button|directory-table|transfer-card)\b/.test(text))
        failures.push(`${name}: retired CSS selector`);
      continue;
    }
    failures.push(...prototypeImports(text, name));
    // Primitives may render native controls, but may not import prototype state.
    if (name.startsWith("components/ui/")) continue;
    const file = ts.createSourceFile(
      name,
      text,
      ts.ScriptTarget.Latest,
      true,
      entry.name.endsWith(".tsx") ? ts.ScriptKind.TSX : ts.ScriptKind.TS,
    );
    function visit(node) {
      if (ts.isJsxOpeningElement(node) || ts.isJsxSelfClosingElement(node)) {
        const tag = node.tagName.getText(file);
        if (["select", "table", "dialog", "textarea", "button"].includes(tag))
          failures.push(
            `${name}:${file.getLineAndCharacterOfPosition(node.pos).line + 1}: native ${tag}`,
          );
        if (tag === "input") {
          const type = node.attributes.properties.find(
            (a) => a.name?.getText(file) === "type",
          )?.initializer;
          if (
            !type ||
            !ts.isStringLiteral(type) ||
            !["file", "hidden", "color", "range"].includes(type.text)
          )
            failures.push(`${name}: use Input/Checkbox/RadioGroup`);
        }
      }
      ts.forEachChild(node, visit);
    }
    visit(file);
  }
}
await walk(root);
return failures;
}

if (process.argv[1] && import.meta.url === pathToFileURL(process.argv[1]).href) {
  const failures = await checkUI();
  if (failures.length) {
    console.error(failures.join("\n"));
    process.exitCode = 1;
  } else {
    console.log("UI component and stylesheet checks passed.");
  }
}
