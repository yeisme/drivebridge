import { expect, it } from "vitest";
import { mkdtemp, mkdir, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, sep } from "node:path";
import { pathToFileURL } from "node:url";
// @ts-expect-error Existing quality checker is a JavaScript build tool.
import { checkUI } from "../scripts/check-ui.mjs";

it("checks primitive imports without prohibiting their native controls", async () => {
  const directory = await mkdtemp(join(tmpdir(), "drivebridge-ui-check-"));
  try {
    await mkdir(join(directory, "components/ui"), { recursive: true });
    const primitive = join(directory, "components/ui/button.tsx");
    await writeFile(primitive, 'export const Button = () => <button />;');
    const root = pathToFileURL(directory + sep);
    expect(await checkUI(root)).toEqual([]);
    await writeFile(primitive, 'import { state } from "../../../prototypes/demo"; export const Button = () => <button />;');
    expect(await checkUI(root)).toEqual(["components/ui/button.tsx:1: production prototype import"]);
    await writeFile(primitive, 'export const Button = () => <button />;');
    await writeFile(join(directory, "page.tsx"), 'export const Page = () => <button />;');
    expect(await checkUI(root)).toEqual(["page.tsx:1: native button"]);
    await writeFile(join(directory, "page.tsx"), 'export const Page = () => <div />;');
    await writeFile(join(directory, "client.ts"), 'export * from "../prototypes/client";');
    expect(await checkUI(root)).toEqual(["client.ts:1: production prototype import"]);
  } finally { await rm(directory, { recursive: true, force: true }); }
});
