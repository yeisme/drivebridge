import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig({
  root: import.meta.dirname,
  base: "./",
  resolve: { alias: { "@": new URL("../../src", import.meta.url).pathname } },
  plugins: [react(), tailwindcss()],
  build: { outDir: "../../temp/personal-agent-prototype", emptyOutDir: true },
  server: { host: "127.0.0.1", port: 31927, strictPort: true },
});
