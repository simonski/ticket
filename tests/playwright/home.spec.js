const { test, expect } = require("@playwright/test");

test("landing page exposes ticket-first UI controls", async ({ page }) => {
  await page.goto("/");

  await expect(page.getByRole("heading", { name: "ticket" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Create Project" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Create Ticket" })).toBeVisible();
  await expect(page.locator("#project-prefix")).toBeVisible();
  await expect(page.locator("#filter-status")).toBeVisible();

  const statusOptions = await page.locator("#filter-status option").allTextContents();
  expect(statusOptions).toContain("Develop / Active");
  expect(statusOptions).toContain("Done / Complete");
});
