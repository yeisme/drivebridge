import fs from "node:fs";
import path from "node:path";
import { createRequire } from "node:module";
import type { Plugin } from "vite";
const require = createRequire(import.meta.url);
const root = path.dirname(require.resolve("pdfjs-dist/package.json"));
const folders = ["cmaps", "standard_fonts", "wasm"];
export function pdfAssets(): Plugin {
  return {
    name: "local-pdf-resources",
    generateBundle() {
      for (const folder of folders)
        for (const name of fs.readdirSync(path.join(root, folder))) {
          const file = path.join(root, folder, name);
          if (fs.statSync(file).isFile())
            this.emitFile({
              type: "asset",
              fileName: `pdfjs/${folder}/${name}`,
              source: fs.readFileSync(file),
            });
        }
    },
    configureServer(server) {
      server.middlewares.use((req, res, next) => {
        const match = req.url
          ?.split("?")[0]
          .match(/^\/pdfjs\/(cmaps|standard_fonts|wasm)\/([\w.-]+)$/);
        if (!match || match[2].startsWith(".")) return next();
        const file = path.join(root, match[1], match[2]);
        if (!fs.existsSync(file) || !fs.statSync(file).isFile()) return next();
        res.setHeader(
          "Content-Type",
          file.endsWith(".wasm")
            ? "application/wasm"
            : "application/octet-stream",
        );
        fs.createReadStream(file).pipe(res);
      });
    },
  };
}
