import { expect, test } from "@playwright/test";
import { mockSignedInLeader } from "./fixtures";

test("landing explains the product and offers Discord sign-in", async ({ page }) => {
  await page.route("**/api/**", (route) =>
    route.fulfill({ status: 401, contentType: "application/json", body: '{"error":"sign in with Discord"}' })
  );
  await page.goto("/");
  await expect(page.getByRole("heading", { level: 1 })).toContainText("whole static");
  await expect(page.getByRole("link", { name: "Sign in with Discord" })).toHaveAttribute("href", "/api/auth/login");
  await expect(page.getByRole("table", { name: /availability/i })).toBeVisible();
});

test("availability grid shows the earliest open period and highlights full attendance", async ({ page }) => {
  const recorder = await mockSignedInLeader(page);
  await page.goto("/app");
  const grid = page.getByRole("table", { name: /availability/i });
  await expect(grid).toBeVisible();
  await expect(grid.getByRole("rowheader", { name: /Alisaie/ })).toBeVisible();
  const counts = grid.getByTestId("available-count");
  await expect(counts).toHaveText(["8/8 — everyone is available", "7/8", "5/8"]);
  await expect(counts.first()).toHaveClass(/text-available/);
  await expect(counts.nth(1)).not.toHaveClass(/text-available/);
  await expect(grid.getByRole("img", { name: "Estinien: Available" }).first()).toBeVisible();

  await page.getByRole("combobox", { name: "Voting period" }).click();
  await page.getByRole("option").nth(1).click();
  await expect(counts).toHaveText(["2/8"]);

  expect(recorder.requests.some((request) => request.includes("/channels"))).toBe(false);
});

test("leader confirms a date from the grid", async ({ page }) => {
  const recorder = await mockSignedInLeader(page);
  await page.goto("/app");
  const actions = page.getByRole("button", { name: /^Actions for/ });
  await actions.nth(1).click();
  await page.getByRole("button", { name: "Confirm date" }).click();
  await expect(page.getByText(/confirmed$/)).toBeVisible();
  expect(recorder.bodies["POST /api/occurrences/102/status"]).toEqual({ action: "confirm" });
});

test("schedule edits show an unsaved-changes bar that can be discarded", async ({ page }) => {
  await mockSignedInLeader(page);
  await page.goto("/app");
  await page.getByRole("button", { name: "Schedule" }).click();
  await expect(page.getByText("You have unsaved changes")).toHaveCount(0);
  await page
    .getByRole("radio", { name: "Mon" })
    .or(page.getByRole("button", { name: "Mon" }))
    .click();
  await expect(page.getByText("You have unsaved changes")).toBeVisible();
  await expect(page.getByRole("complementary", { name: "Preview" }).getByRole("listitem")).toHaveCount(2);
  await page.getByRole("button", { name: "Discard" }).click();
  await expect(page.getByText("You have unsaved changes")).toHaveCount(0);
});

test("the page never scrolls sideways on a phone", async ({ page }, testInfo) => {
  test.skip(testInfo.project.name !== "mobile");
  await mockSignedInLeader(page);
  await page.goto("/app");
  await expect(page.getByRole("table", { name: /availability/i })).toBeVisible();
  const overflow = await page.evaluate(() => document.documentElement.scrollWidth - window.innerWidth);
  expect(overflow).toBeLessThanOrEqual(0);
});
