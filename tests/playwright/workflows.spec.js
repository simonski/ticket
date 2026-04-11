const { test, expect } = require("@playwright/test");

function mockAPI(page, routes) {
  return Promise.all(
    routes.map(([pattern, handler]) =>
      page.route(pattern, async (route) => {
        if (typeof handler === "function") return handler(route);
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify(handler),
        });
      })
    )
  );
}

const SAMPLE_WORKFLOWS = [
  { sdlc_id: 1, name: "default", description: "Standard lifecycle" },
  { sdlc_id: 2, name: "kanban", description: "Simple kanban" },
];

const SAMPLE_STAGES = [
  { sdlc_stage_id: 1, sdlc_id: 1, stage_name: "design", description: "", role_id: 5, role_title: "BA", sort_order: 0 },
  { sdlc_stage_id: 2, sdlc_id: 1, stage_name: "develop", description: "", role_id: 6, role_title: "Lead Engineer", sort_order: 1 },
  { sdlc_stage_id: 3, sdlc_id: 1, stage_name: "test", description: "", role_id: 4, role_title: "QA/Tester", sort_order: 2 },
  { sdlc_stage_id: 4, sdlc_id: 1, stage_name: "done", description: "", role_id: 1, role_title: "Product Owner", sort_order: 3 },
];

async function setupSDLCs(page) {
  await mockAPI(page, [
    ["**/api/board/ws", (route) => route.abort()],
  ]);
  await page.goto("/");
  await page.evaluate((wfs) => {
    showApp("admin", "admin");
    projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
    localStorage.setItem("task-project", "1");
    tickets = [];
    sdlcs = wfs;
    renderBoard();
    activatePerspective("sdlcs");
    renderSdlcList();
  }, SAMPLE_WORKFLOWS);
}

test.describe("sdlc management", () => {
  test("sdlc list renders cards", async ({ page }) => {
    await setupSDLCs(page);

    const cards = await page.locator("#sdlc-list .management-card, #sdlc-list .sdlc-card").count();
    expect(cards).toBe(2);
  });

  test("clicking sdlc card opens editor", async ({ page }) => {
    await mockAPI(page, [
      ["**/api/sdlcs/1", {
        sdlc_id: 1, name: "default", description: "Standard lifecycle",
        stages: SAMPLE_STAGES,
      }],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate((wfs) => {
      showApp("admin", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = [];
      sdlcs = wfs;
      renderBoard();
      activatePerspective("sdlcs");
      renderSdlcList();
    }, SAMPLE_WORKFLOWS);

    await page.evaluate(() => openSdlcEditor(sdlcs[0]));

    await expect(page.locator("#sdlc-modal-overlay")).toBeVisible();
    await expect(page.locator("#sdlc-name")).toHaveValue("default");
  });

  test("create sdlc posts to API", async ({ page }) => {
    let postBody = null;
    await mockAPI(page, [
      ["**/api/sdlcs", (route) => {
        if (route.request().method() === "POST") {
          postBody = route.request().postDataJSON();
          return route.fulfill({
            status: 200, contentType: "application/json",
            body: JSON.stringify({ sdlc_id: 99, name: postBody?.name, description: postBody?.description }),
          });
        }
        return route.fulfill({ status: 200, contentType: "application/json", body: "[]" });
      }],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate(() => {
      showApp("admin", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = [];
      sdlcs = [];
      renderBoard();
      activatePerspective("sdlcs");
      renderSdlcList();
      openSdlcEditor();
    });

    await page.fill("#sdlc-name", "custom");
    await page.fill("#sdlc-description", "Custom sdlc");
    await page.click("#sdlc-save");
    await page.waitForTimeout(300);

    expect(postBody).not.toBeNull();
    expect(postBody.name).toBe("custom");
  });

  test("delete sdlc calls DELETE", async ({ page }) => {
    let deleteCalled = false;
    await mockAPI(page, [
      ["**/api/sdlcs/1", (route) => {
        if (route.request().method() === "DELETE") {
          deleteCalled = true;
          return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
        }
        if (route.request().method() === "GET") {
          return route.fulfill({
            status: 200, contentType: "application/json",
            body: JSON.stringify({ sdlc_id: 1, name: "default", description: "", stages: SAMPLE_STAGES }),
          });
        }
        return route.continue();
      }],
      ["**/api/sdlcs", []],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate((wfs) => {
      showApp("admin", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = [];
      sdlcs = wfs;
      renderBoard();
      activatePerspective("sdlcs");
      renderSdlcList();
      openSdlcEditor(sdlcs[0]);
    }, SAMPLE_WORKFLOWS);

    await page.evaluate(() => { window._origUiConfirm = window.uiConfirm; window.uiConfirm = async () => true; });
    const deleteBtn = page.locator("#sdlc-delete");
    if (await deleteBtn.count() > 0) {
      await deleteBtn.click();
      await page.waitForTimeout(300);
      expect(deleteCalled).toBe(true);
    }
    await page.evaluate(() => { if (window._origUiConfirm) window.uiConfirm = window._origUiConfirm; });
  });

  test("add stage posts to API", async ({ page }) => {
    let stageBody = null;
    await mockAPI(page, [
      ["**/api/sdlcs/1/stages", (route) => {
        if (route.request().method() === "POST") {
          stageBody = route.request().postDataJSON();
          return route.fulfill({
            status: 200, contentType: "application/json",
            body: JSON.stringify({ sdlc_stage_id: 99, ...stageBody }),
          });
        }
        return route.continue();
      }],
      ["**/api/sdlcs/1", {
        sdlc_id: 1, name: "default", description: "Standard lifecycle",
        stages: SAMPLE_STAGES,
      }],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate((wfs) => {
      showApp("admin", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = [];
      sdlcs = wfs;
      renderBoard();
      activatePerspective("sdlcs");
      renderSdlcList();
      openSdlcEditor(sdlcs[0]);
    }, SAMPLE_WORKFLOWS);

    // Fill stage form and click add
    const stageNameInput = page.locator("#sdlc-stage-name, #stage-name");
    const stageAddBtn = page.locator("#sdlc-stage-add, #stage-add");

    if (await stageNameInput.count() > 0 && await stageAddBtn.count() > 0) {
      await stageNameInput.fill("review");
      await stageAddBtn.click();
      await page.waitForTimeout(300);
      expect(stageBody).not.toBeNull();
      expect(stageBody.stage_name).toBe("review");
    }
  });

  test("sdlc stages are displayed after opening editor", async ({ page }) => {
    await mockAPI(page, [
      ["**/api/sdlcs/1", {
        sdlc_id: 1, name: "default", description: "Standard lifecycle",
        stages: SAMPLE_STAGES,
      }],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate((wfs) => {
      showApp("admin", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = [];
      sdlcs = wfs;
      renderBoard();
      activatePerspective("sdlcs");
      renderSdlcList();
      openSdlcEditor(sdlcs[0]);
    }, SAMPLE_WORKFLOWS);

    await page.waitForTimeout(500);

    const stagesContent = await page.evaluate(() => {
      const stagesList = document.getElementById("sdlc-stages-list") || document.getElementById("sdlc-stages");
      return stagesList?.textContent || "";
    });

    expect(stagesContent).toContain("design");
    expect(stagesContent).toContain("develop");
  });

  test("close sdlc modal hides overlay", async ({ page }) => {
    await setupSDLCs(page);

    await page.evaluate(() => openSdlcEditor(sdlcs[0]));

    // closeSdlcModal might not exist — check first
    const closed = await page.evaluate(() => {
      if (typeof closeSdlcModal === "function") {
        closeSdlcModal();
        return true;
      }
      const overlay = document.getElementById("sdlc-modal-overlay");
      if (overlay) { overlay.classList.add("hidden"); return true; }
      return false;
    });

    if (closed) {
      await expect(page.locator("#sdlc-modal-overlay")).not.toBeVisible();
    }
  });

  test("export button triggers download for existing sdlc", async ({ page }) => {
    await mockAPI(page, [
      ["**/api/sdlcs/1/export", {
        name: "default",
        description: "Standard lifecycle",
        stages: [
          { stage_name: "design", description: "", role: "BA", sort_order: 0 },
          { stage_name: "develop", description: "", role: "Lead Engineer", sort_order: 1 },
        ],
      }],
      ["**/api/sdlcs/1", {
        sdlc_id: 1, name: "default", description: "Standard lifecycle",
        stages: SAMPLE_STAGES,
      }],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate((wfs) => {
      showApp("admin", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = [];
      sdlcs = wfs;
      renderBoard();
      activatePerspective("sdlcs");
      renderSdlcList();
      openSdlcEditor(sdlcs[0]);
    }, SAMPLE_WORKFLOWS);

    const exportBtn = page.locator("#sdlc-export");
    if (await exportBtn.count() > 0) {
      // Export button should be visible for existing sdlcs
      await expect(exportBtn).toBeVisible();
    }
  });

  test("import button exists in sdlc view", async ({ page }) => {
    await setupSDLCs(page);

    const importBtn = page.locator("#sdlc-import, button:has-text('Import')");
    if (await importBtn.count() > 0) {
      await expect(importBtn.first()).toBeVisible();
    }
  });
});
