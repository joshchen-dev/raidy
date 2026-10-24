import { defineConfig } from "@playwright/test";

// Captures the product page's screenshots from the running demo:
// npm run screenshots
const demoPort = 5181;
const productPort = 5182;

export default defineConfig({
  testDir: "scripts",
  testMatch: "screenshots.spec.ts",
  workers: 1,
  reporter: "list",
  use: {
    timezoneId: "Asia/Tokyo",
    launchOptions: { executablePath: process.env.PLAYWRIGHT_CHROMIUM_PATH || undefined }
  },
  webServer: [
    {
      command: `npx vite --mode demo --base /demo/ --host 127.0.0.1 --port ${demoPort} --strictPort`,
      url: `http://127.0.0.1:${demoPort}/demo/`,
      reuseExistingServer: !process.env.CI
    },
    {
      command: `npx vite --config vite.product.config.ts --host 127.0.0.1 --port ${productPort} --strictPort`,
      url: `http://127.0.0.1:${productPort}/`,
      reuseExistingServer: !process.env.CI
    }
  ]
});
