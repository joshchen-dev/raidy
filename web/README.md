# Raidy web console

React, TypeScript, Vite, and Tailwind CSS provide Raidy's configuration console.

```sh
npm install
npm run dev
```

The development server runs at `http://localhost:5173` and proxies `/api`, `/healthz`, and `/readyz` to the Go service at `http://localhost:8080`.

`npm run build` writes production assets to `dist/`. Set `WEB_DIST_DIR` to that absolute directory when the Go service should serve the UI directly.
