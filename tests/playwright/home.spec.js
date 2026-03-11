const { test, expect } = require("@playwright/test");

test("landing page exposes ticket-first UI controls", async ({ page }) => {
  const pageErrors = [];
  page.on("pageerror", (err) => {
    pageErrors.push(String(err && err.message ? err.message : err));
  });

  await page.goto("/");

  await expect(page.locator("#login-screen")).toBeVisible();
  await expect(page.locator("#login-form")).toBeVisible();
  await expect(page.locator("#login-pixel-logo")).toBeVisible();
  await expect(page.locator("#login-user")).toBeVisible();
  await expect(page.locator("#login-pass")).toBeVisible();
  await expect(page.getByRole("button", { name: "Login" })).toBeVisible();
  expect(pageErrors, `unexpected page errors: ${pageErrors.join("\n")}`).toEqual([]);
});
