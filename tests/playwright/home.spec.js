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
  await expect(page.locator('[data-left-panel-action="swimlanes"]')).toHaveText("board");
  await expect(page.getByText("kanban")).toHaveCount(0);
  await expect(page.locator("#perspective-btn")).toHaveCount(0);
  await expect(page.getByText("ESC to close")).toHaveCount(0);
  await expect(page.locator("#settings-modal")).not.toContainText("ESC to close");
  const layout = await page.evaluate(() => {
    const mainContent = document.getElementById("main-content");
    const leftPanel = document.getElementById("left-panel");
    const appHead = document.querySelector(".app-head");
    if (!mainContent || !leftPanel || !appHead) return null;
    const mainStyle = window.getComputedStyle(mainContent);
    const panelStyle = window.getComputedStyle(leftPanel);
    const headStyle = window.getComputedStyle(appHead);
    return {
      mainOverflowY: mainStyle.overflowY,
      leftPanelOverflowY: panelStyle.overflowY,
      appHeadPosition: headStyle.position,
    };
  });
  expect(layout).not.toBeNull();
  expect(layout.mainOverflowY).toBe("auto");
  expect(layout.leftPanelOverflowY).toBe("auto");
  expect(layout.appHeadPosition).toBe("sticky");
  const selectorPersistence = await page.evaluate(() => {
    const leftPanel = document.getElementById("left-panel");
    const appShell = document.getElementById("app-shell");
    const backdrop = document.getElementById("left-panel-backdrop");
    if (!leftPanel || !appShell || !backdrop) return null;
    leftPanel.classList.add("open");
    appShell.classList.add("panel-open");
    backdrop.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    const openAfterBackdropClick = leftPanel.classList.contains("open");
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
    const openAfterEscape = leftPanel.classList.contains("open");
    leftPanel.classList.remove("open");
    appShell.classList.remove("panel-open");
    return { openAfterBackdropClick, openAfterEscape };
  });
  expect(selectorPersistence).not.toBeNull();
  expect(selectorPersistence.openAfterBackdropClick).toBe(true);
  expect(selectorPersistence.openAfterEscape).toBe(true);
  const escapeDoesNotClosePanels = await page.evaluate(() => {
    const overlayIds = [
      "search-overlay",
      "perspective-overlay",
      "proj-modal-overlay",
      "story-modal-overlay",
      "agent-modal-overlay",
      "role-modal-overlay",
      "team-modal-overlay",
      "settings-modal-overlay",
      "modal-overlay",
      "dialog-overlay",
    ];
    const overlays = overlayIds
      .map((id) => document.getElementById(id))
      .filter(Boolean);
    overlays.forEach((node) => node.classList.remove("hidden"));
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
    const allStillVisible = overlays.every((node) => !node.classList.contains("hidden"));
    overlays.forEach((node) => node.classList.add("hidden"));
    return allStillVisible;
  });
  expect(escapeDoesNotClosePanels).toBe(true);
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
