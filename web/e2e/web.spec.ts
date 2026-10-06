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

// Regression: the Schedule tab froze while it fetched Discord channels on
// open. It must render from the saved schedule and only ask Discord for
// channels when the leader chooses to change the destination.
test("schedule shows the saved destination and loads channels only on demand", async ({ page }) => {
  const recorder = await mockSignedInLeader(page);
  await page.goto("/app");
  await page.getByRole("button", { name: "Schedule" }).click();
  // Regression: saved names used to render as "Discord server" / "Saved channel".
  await expect(page.getByText("Posts go to Scions.")).toBeVisible();
  await expect(page.getByText("#raid-schedule")).toBeVisible();
  await expect(page.getByText("Saved channel")).toHaveCount(0);
  expect(recorder.requests.some((request) => request.includes("/channels"))).toBe(false);

  await page.getByRole("button", { name: "Change" }).click();
  await expect(page.getByRole("combobox", { name: "Discord channel" })).toBeVisible();
  expect(recorder.requests).toContain("GET /api/guilds/g1/channels");
});

// Regression: raid times rendered as "09:00 PM"; schedules use a 24-hour clock.
test("raid times use a 24-hour clock", async ({ page }) => {
  await mockSignedInLeader(page);
  await page.goto("/app");
  const header = page.getByRole("columnheader").nth(1);
  await expect(header).toContainText("21:00");
  await expect(header).not.toContainText(/AM|PM/);
});

test("a member edits and saves their availability on the web", async ({ page }) => {
  const recorder = await mockSignedInLeader(page);
  await page.goto("/app");
  const grid = page.getByRole("table", { name: /availability/i });
  // The signed-in member's row comes first and is marked as theirs.
  await expect(grid.getByRole("rowheader").first()).toContainText("Alisaie");
  await expect(grid.getByRole("rowheader").first()).toContainText("You");

  await page.getByRole("button", { name: "Edit my availability" }).click();
  const thursday = page.getByRole("button", { name: "Available on Thu 8" });
  await expect(thursday).toHaveAttribute("aria-pressed", "true");
  await thursday.click();
  await expect(thursday).toHaveAttribute("aria-pressed", "false");
  await page.getByRole("button", { name: "Save availability" }).click();

  await expect(page.getByText("Availability saved")).toBeVisible();
  expect(recorder.bodies["PUT /api/polls/10/availability"]).toEqual({ available: [101, 103] });
  await expect(page.getByRole("button", { name: "Edit my availability" })).toBeVisible();
});
