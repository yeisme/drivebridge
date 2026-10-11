import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";

const dir = path.resolve(
  import.meta.dirname,
  "../../temp/personal-agent-prototype",
);
let html = fs.readFileSync(path.join(dir, "index.html"), "utf8");
html = html.replace(
  /<script\b[^>]*src="([^"]+)"[^>]*><\/script>/g,
  (_, url) => {
    const source = fs
      .readFileSync(path.join(dir, url), "utf8")
      .replace(/<\/script/gi, "<\\/script");
    return `<script type="module">${source}</script>`;
  },
);
html = html.replace(
  /<link\b[^>]*href="([^"]+\.css)"[^>]*>/g,
  (_, url) => `<style>${fs.readFileSync(path.join(dir, url), "utf8")}</style>`,
);
html = html.replace(
  "</head>",
  "<meta http-equiv=\"Content-Security-Policy\" content=\"default-src 'none'; script-src 'unsafe-inline'; style-src 'unsafe-inline'; img-src data: blob:; font-src data:; connect-src 'none'; media-src blob:; base-uri 'none'; form-action 'none'\"></head>",
);
const artifact = path.join(dir, "opendesign.html");
fs.writeFileSync(artifact, html);
console.log(`Artifact: ${path.relative(process.cwd(), artifact)}`);
console.log(
  `Bytes: ${Buffer.byteLength(html)}; sha256: ${crypto.createHash("sha256").update(html).digest("hex")}`,
);
