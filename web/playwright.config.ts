import { defineConfig, devices } from "@playwright/test";

const port = 5179;
const demoPort = 5180;

export default defineConfig({
  testDir: "e2e",
  fullyParallel: true,
  reporter: process.env.CI ? "github" : "list",
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    trace: "retain-on-failure",
    // The fixtures' team is in Tokyo; individual tests move the viewer elsewhere.
    timezoneId: "Asia/Tokyo",
    launchOptions: { executablePath: process.env.PLAYWRIGHT_CHROMIUM_PATH || undefined }
  },
  projects: [
    { name: "desktop", testIgnore: "demo.spec.ts", use: { ...devices["Desktop Chrome"] } },
    { name: "mobile", testIgnore: "demo.spec.ts", use: { ...devices["Pixel 7"] } },
    // The demo build runs the real app against in-browser sample data.
    {
      name: "demo",
      testMatch: "demo.spec.ts",
      use: { ...devices["Desktop Chrome"], baseURL: `http://127.0.0.1:${demoPort}` }
    }
  ],
  webServer: [
    {
      command: `npx vite --host 127.0.0.1 --port ${port} --strictPort`,
      url: `http://127.0.0.1:${port}`,
      reuseExistingServer: !process.env.CI
    },
    {
      command: `npx vite --mode demo --base /demo/ --host 127.0.0.1 --port ${demoPort} --strictPort`,
      url: `http://127.0.0.1:${demoPort}/demo/`,
      reuseExistingServer: !process.env.CI
    }
  ]
});
