import { devices, expect, test, type Page } from "@playwright/test";

const demo = "http://127.0.0.1:5181/demo/";
const product = "http://127.0.0.1:5182/";
const out = "product/public";

async function openDemo(page: Page, theme: "light" | "dark") {
  await page.emulateMedia({ colorScheme: theme });
  await page.goto(demo);
  // The first load can wait on Vite optimizing dependencies.
  await expect(page.getByRole("table", { name: /availability/i })).toBeVisible({ timeout: 30_000 });
  // The demo banner is noise on the product page.
  await page.getByRole("note").evaluate((banner) => banner.remove());
}

for (const theme of ["light", "dark"] as const) {
  test(`schedule editor, ${theme}`, async ({ browser }) => {
    const page = await browser.newPage({ viewport: { width: 1280, height: 800 }, deviceScaleFactor: 2 });
    await openDemo(page, theme);
    await page.getByRole("button", { name: "Schedule" }).click();
    await expect(page.getByRole("complementary", { name: "Preview" }).getByRole("listitem").first()).toBeVisible();
    await page.screenshot({ path: `${out}/screens/schedule-${theme}.png` });
  });

  test(`availability on a phone, ${theme}`, async ({ browser }) => {
    const page = await browser.newPage({ ...devices["Pixel 7"] });
    await openDemo(page, theme);
    await page.screenshot({ path: `${out}/screens/mobile-${theme}.png` });
  });
}

test("link preview image", async ({ browser }) => {
  const page = await browser.newPage({ viewport: { width: 1200, height: 630 } });
  await page.emulateMedia({ colorScheme: "light" });
  await page.goto(product);
  await expect(page.getByRole("figure", { name: "The Raidy post in Discord" })).toBeVisible();
  await page.screenshot({ path: `${out}/og.png` });
});
