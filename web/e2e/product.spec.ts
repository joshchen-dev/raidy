import { devices, expect, test } from "@playwright/test";

const monday = new Date("2026-11-02T00:00:00Z");

test("the product page leads to the live demo and the code", async ({ page }) => {
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 })).toHaveText("See which nights your whole static can raid.");
  await expect(page.getByRole("link", { name: "Try the live demo" })).toHaveAttribute("href", "demo/");
  await expect(page.getByRole("link", { name: "View on GitHub" }).first()).toHaveAttribute(
    "href",
    "https://github.com/joshchen-dev/raidy"
  );
});

test("tapping a night in the hero updates the Discord post", async ({ page }) => {
  await page.clock.setFixedTime(monday);
  await page.goto("/");
  const post = page.getByRole("figure", { name: "The Raidy post in Discord" });
  await expect(post).toContainText("8/8 available · Confirmed");
  await page.getByRole("button", { name: "Available on Wed 4" }).click();
  await expect(post).toContainText("7/8 available · Attention required");
  await expect(page.getByTestId("available-count").first()).toHaveText("7/8");
});

test("the page never scrolls sideways on a phone", async ({ browser }) => {
  const page = await browser.newPage({ ...devices["Pixel 7"] });
  await page.goto("http://127.0.0.1:5184/");
  await expect(page.getByRole("figure", { name: "The Raidy post in Discord" })).toBeVisible();
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow).toBeLessThanOrEqual(0);
});
