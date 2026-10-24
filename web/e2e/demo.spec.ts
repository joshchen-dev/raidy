import { expect, test, type Page } from "@playwright/test";

// Monday 09:00 in Tokyo: this week's raids are Wed 4, Thu 5 and Fri 6.
const monday = new Date("2026-11-02T00:00:00Z");

async function openDemo(page: Page) {
  const requests: string[] = [];
  // The demo must never reach a server.
  await page.route("**/api/**", (route) => {
    requests.push(route.request().url());
    return route.abort();
  });
  await page.clock.setFixedTime(monday);
  await page.goto("/demo/");
  await expect(page.getByRole("table", { name: /availability/i })).toBeVisible();
  return requests;
}

const counts = (page: Page) => page.getByTestId("available-count");

test("opens signed in on sample data without any server", async ({ page }) => {
  const requests = await openDemo(page);
  await expect(page.getByRole("note")).toContainText("Sample data");
  await expect(page.getByRole("link", { name: "Sign in with Discord" })).toHaveCount(0);
  const grid = page.getByRole("table", { name: /availability/i });
  await expect(grid.getByRole("rowheader").first()).toContainText("You");
  await expect(counts(page)).toHaveText(["8/8 — everyone is available", "7/8", "5/8"]);
  await expect(page).toHaveURL(/\/demo\/\?team=1$/);
  expect(requests).toEqual([]);
});

test("a vote updates the counts and flags the confirmed date", async ({ page }) => {
  await openDemo(page);
  await page.getByRole("button", { name: "Edit my availability" }).click();
  await page.getByRole("button", { name: "Available on Wed 4" }).click();
  await page.getByRole("button", { name: "Save availability" }).click();
  await expect(page.getByText("Availability saved")).toBeVisible();
  await expect(counts(page).first()).toHaveText("7/8");
  await expect(page.getByText("Needs attention")).toBeVisible();
});

test("the leader cancels a date", async ({ page }) => {
  await openDemo(page);
  await page.getByRole("button", { name: "Actions for Fri 6" }).click();
  await page.getByRole("button", { name: "Cancel date" }).click();
  await expect(page.getByText("Cancelled", { exact: true })).toBeVisible();
});

test("a new raid time moves the open week's dates", async ({ page }) => {
  await openDemo(page);
  await page.getByRole("button", { name: "Schedule" }).click();
  await page.getByRole("combobox", { name: "Start time" }).click();
  await page.getByRole("option", { name: "20:00" }).click();
  await page.getByRole("button", { name: "Save schedule" }).click();
  await expect(page.getByText("Schedule saved")).toBeVisible();
  await page.getByRole("button", { name: "Availability" }).click();
  await expect(page.getByRole("columnheader").nth(1)).toContainText("20:00");
});

test("reloading starts over from the sample data", async ({ page }) => {
  await openDemo(page);
  await page.getByRole("button", { name: "Actions for Fri 6" }).click();
  await page.getByRole("button", { name: "Cancel date" }).click();
  await expect(page.getByText("Cancelled", { exact: true })).toBeVisible();
  await page.reload();
  await expect(counts(page)).toHaveText(["8/8 — everyone is available", "7/8", "5/8"]);
  await expect(page.getByText("Cancelled", { exact: true })).toHaveCount(0);
});
