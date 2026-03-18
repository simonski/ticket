const { test, expect } = require("@playwright/test");

const SAMPLE_TICKETS = [
  { ticket_id: 1, title: "Auth Epic", type: "epic", stage: "develop", state: "active", key: "DM-1", parent_id: null },
  { ticket_id: 2, title: "Login task", type: "task", stage: "develop", state: "idle", key: "DM-2", parent_id: 1 },
  { ticket_id: 3, title: "Login bug", type: "bug", stage: "develop", state: "active", key: "DM-3", parent_id: 1 },
  { ticket_id: 4, title: "Standalone task", type: "task", stage: "design", state: "idle", key: "DM-4", parent_id: null },
];

async function setupWithHierarchy(page, ticketData) {
  await page.route("**/api/board/ws", (route) => route.abort());
  await page.goto("/");
  await page.evaluate((data) => {
    showApp("admin", "admin");
    projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
    localStorage.setItem("task-project", "1");
    tickets = data;
    renderBoard();
    activatePerspective("hierarchy");
    renderHierarchy();
  }, ticketData);
}

test.describe("hierarchy view", () => {
  test("hierarchy nav button is present", async ({ page }) => {
    await page.route("**/api/board/ws", (route) => route.abort());
    await page.goto("/");
    await page.evaluate(() => {
      showApp("admin", "admin");
    });
    const btn = page.locator('.left-panel-item[data-left-panel-action="hierarchy"]');
    await expect(btn).toBeVisible();
    await expect(btn).toHaveText("hierarchy");
  });

  test("hierarchy overlay item is present", async ({ page }) => {
    await page.route("**/api/board/ws", (route) => route.abort());
    await page.goto("/");
    await page.evaluate(() => {
      showApp("admin", "admin");
    });
    const item = page.locator('.perspective-item[data-perspective="hierarchy"]');
    await expect(item).toBeAttached();
  });

  test("renders epic groups with children", async ({ page }) => {
    await setupWithHierarchy(page, SAMPLE_TICKETS);

    const groups = await page.evaluate(() =>
      Array.from(document.querySelectorAll("#hierarchy-list .hierarchy-group")).length
    );
    // One epic group + one no-parent group
    expect(groups).toBe(2);
  });

  test("epic title appears in group header", async ({ page }) => {
    await setupWithHierarchy(page, SAMPLE_TICKETS);

    const epicTitle = await page.evaluate(() => {
      const header = document.querySelector(".hierarchy-epic .hierarchy-epic-title");
      return header ? header.textContent : null;
    });
    expect(epicTitle).toContain("Auth Epic");
  });

  test("child tickets appear under their epic", async ({ page }) => {
    await setupWithHierarchy(page, SAMPLE_TICKETS);

    const rows = await page.evaluate(() =>
      Array.from(document.querySelectorAll(".hierarchy-row .hierarchy-row-title")).map((el) => el.textContent)
    );
    expect(rows).toContain("Login task");
    expect(rows).toContain("Login bug");
  });

  test("standalone task appears in no-parent group", async ({ page }) => {
    await setupWithHierarchy(page, SAMPLE_TICKETS);

    const noParentGroup = await page.evaluate(() => {
      const groups = Array.from(document.querySelectorAll(".hierarchy-group"));
      const noParent = groups.find((g) => g.classList.contains("hierarchy-noparent"));
      if (!noParent) return null;
      return Array.from(noParent.querySelectorAll(".hierarchy-row-title")).map((el) => el.textContent);
    });
    expect(noParentGroup).not.toBeNull();
    expect(noParentGroup).toContain("Standalone task");
  });

  test("shows empty state when no tickets", async ({ page }) => {
    await setupWithHierarchy(page, []);

    const empty = await page.evaluate(() => {
      const el = document.querySelector("#hierarchy-list .hierarchy-empty");
      return el ? el.textContent : null;
    });
    expect(empty).toContain("No tickets");
  });

  test("clicking a child row opens edit modal", async ({ page }) => {
    await setupWithHierarchy(page, SAMPLE_TICKETS);

    await page.evaluate(() => {
      const rows = document.querySelectorAll(".hierarchy-row");
      if (rows[0]) rows[0].click();
    });

    const modal = page.locator("#modal-overlay");
    await expect(modal).not.toHaveClass(/hidden/);
  });
});
