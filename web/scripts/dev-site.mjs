// Runs the product page and the demo together, so links to /demo/ work:
// the product dev server forwards /demo/ to the demo dev server.
import { spawn } from "node:child_process";

const demo = ["--mode", "demo", "--base", "/demo/", "--host", "127.0.0.1", "--port", "5174", "--strictPort"];
const product = ["--config", "vite.product.config.ts", ...process.argv.slice(2)];

// Run vite directly rather than through npx, so stopping this script stops both servers.
const servers = [demo, product].map((args) =>
  spawn(process.execPath, ["node_modules/vite/bin/vite.js", ...args], { stdio: "inherit" })
);

const stop = () => servers.forEach((server) => server.kill());
process.on("SIGINT", stop);
process.on("SIGTERM", stop);
servers.forEach((server) => server.on("exit", (code) => code && (stop(), process.exit(code))));
