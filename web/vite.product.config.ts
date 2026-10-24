import path from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// The product page at the site root; the demo build is added under demo/.
// Paths are relative so the site works on a custom domain or under
// joshchen-dev.github.io/raidy/ alike.
export default defineConfig({
  root: path.resolve(__dirname, "product"),
  base: "./",
  cacheDir: path.resolve(__dirname, "node_modules/.vite-product"),
  // No single-page fallback: an unknown path is a 404, not the product page again.
  appType: "mpa",
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src")
    }
  },
  server: {
    // `npm run dev:product` also starts the demo; serve it at /demo/ here.
    proxy: { "/demo": { target: "http://127.0.0.1:5174", ws: true } }
  },
  build: {
    outDir: path.resolve(__dirname, "dist-pages"),
    emptyOutDir: false
  }
});
