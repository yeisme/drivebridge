import { pdfAssets } from "./scripts/pdf-assets";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";
const apiOrigin = process.env.DRIVEBRIDGE_DEV_API_ORIGIN || "http://127.0.0.1:8787";
export default defineConfig({
  resolve: { alias: { "@": new URL("./src", import.meta.url).pathname } },
  plugins: [react(), tailwindcss(), pdfAssets()],
  build: {
    rollupOptions: { input: { main: "index.html", insights: "insights.html" } },
  },
  server: {
    proxy: {
      "/api": apiOrigin,
      "/auth": apiOrigin,
    },
  },
});
