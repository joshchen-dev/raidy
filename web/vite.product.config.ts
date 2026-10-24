import path from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

// The product page at the site root; the demo build is added under /demo/.
export default defineConfig({
  root: path.resolve(__dirname, "product"),
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src")
    }
  },
  build: {
    outDir: path.resolve(__dirname, "dist-pages"),
    emptyOutDir: false
  }
});
