import path from "node:path";
import { defineConfig } from "vite";
import react from "@vitejs/plugin-react";
import tailwindcss from "@tailwindcss/vite";

export default defineConfig(({ mode }) => ({
  plugins: [react(), tailwindcss()],
  // Each mode keeps its own dependency cache, so the app and the demo can run
  // side by side without re-optimizing each other's dependencies.
  cacheDir: mode === "demo" ? "node_modules/.vite-demo" : "node_modules/.vite",
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src")
    }
  },
  server: {
    proxy: {
      "/api": "http://127.0.0.1:8080",
      "/healthz": "http://127.0.0.1:8080",
      "/readyz": "http://127.0.0.1:8080"
    }
  }
}));
