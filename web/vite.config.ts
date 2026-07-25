import { fileURLToPath, URL } from "node:url";

import tailwindcss from "@tailwindcss/vite";
import react from "@vitejs/plugin-react";
import { defineConfig } from "vite";

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": fileURLToPath(new URL("./src", import.meta.url)),
    },
  },
  server: {
    // Mirrors the Caddy reverse proxy in production: API + Go-served feeds.
    proxy: {
      "/api": "http://127.0.0.1:9000",
      "/robots.txt": "http://127.0.0.1:9000",
      "/sitemap.xml": "http://127.0.0.1:9000",
      "/rss.xml": "http://127.0.0.1:9000",
    },
  },
});
