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
  await expect(page.locator("#login-user")).toBeFocused();
  expect(pageErrors, `unexpected page errors: ${pageErrors.join("\n")}`).toEqual([]);
});

test("websocket event compatibility keeps board refresh for legacy and normalized payloads", async ({ page }) => {
  await page.goto("/");

  const result = await page.evaluate(() => {
    const cases = [
      { msg: { type: "ticket_created" }, wantType: "ticket_created", wantRefresh: true },
      { msg: { type: "ticket_updated" }, wantType: "ticket_updated", wantRefresh: true },
      { msg: { type: "ticket_deleted" }, wantType: "ticket_deleted", wantRefresh: true },
      { msg: { entity_type: "ticket", change_type: "updated" }, wantType: "ticket_updated", wantRefresh: true },
      { msg: { entity_type: "ticket", change_type: "changed" }, wantType: "ticket_changed", wantRefresh: true },
      { msg: { entity_type: "project", change_type: "users_updated" }, wantType: "project_users_updated", wantRefresh: true },
      { msg: { entity_type: "project", change_type: "changed" }, wantType: "project_changed", wantRefresh: true },
      { msg: { entity_type: "project", change_type: "created" }, wantType: "project_created", wantRefresh: false },
      { msg: { type: "connected" }, wantType: "connected", wantRefresh: false },
      { msg: { type: "unknown_event" }, wantType: "unknown_event", wantRefresh: false },
    ];
      return cases.map((testCase) => {
        const eventType = window.__wsEventType(testCase.msg);
        const refresh = window.__wsShouldRefreshBoard(testCase.msg, eventType);
      return {
        eventType,
        refresh,
        wantType: testCase.wantType,
        wantRefresh: testCase.wantRefresh,
      };
    });
  });

  for (const item of result) {
    expect(item.eventType).toBe(item.wantType);
    expect(item.refresh).toBe(item.wantRefresh);
  }
});
