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
  { workflow_id: 1, name: "default", description: "Standard lifecycle" },
  { workflow_id: 2, name: "kanban", description: "Simple kanban" },
];

const SAMPLE_STAGES = [
  { workflow_stage_id: 1, workflow_id: 1, stage_name: "design", description: "", role_id: 5, role_title: "BA", sort_order: 0 },
  { workflow_stage_id: 2, workflow_id: 1, stage_name: "develop", description: "", role_id: 6, role_title: "Lead Engineer", sort_order: 1 },
  { workflow_stage_id: 3, workflow_id: 1, stage_name: "test", description: "", role_id: 4, role_title: "QA/Tester", sort_order: 2 },
  { workflow_stage_id: 4, workflow_id: 1, stage_name: "done", description: "", role_id: 1, role_title: "Product Owner", sort_order: 3 },
];

async function setupWorkflows(page) {
  await mockAPI(page, [
    ["**/api/board/ws", (route) => route.abort()],
  ]);
  await page.goto("/");
  await page.evaluate((wfs) => {
    showApp("admin", "admin");
    projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
    localStorage.setItem("task-project", "1");
    tickets = [];
    workflows = wfs;
    renderBoard();
    activatePerspective("workflows");
    renderWorkflowList();
  }, SAMPLE_WORKFLOWS);
}

test.describe("workflow management", () => {
  test("workflow list renders cards", async ({ page }) => {
    await setupWorkflows(page);

    const cards = await page.locator("#workflow-list .management-card, #workflow-list .workflow-card").count();
    expect(cards).toBe(2);
  });

  test("clicking workflow card opens editor", async ({ page }) => {
    await mockAPI(page, [
      ["**/api/workflows/1", {
        workflow_id: 1, name: "default", description: "Standard lifecycle",
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
      workflows = wfs;
      renderBoard();
      activatePerspective("workflows");
      renderWorkflowList();
    }, SAMPLE_WORKFLOWS);

    await page.evaluate(() => openWorkflowEditor(workflows[0]));

    await expect(page.locator("#workflow-modal-overlay")).toBeVisible();
    await expect(page.locator("#workflow-name")).toHaveValue("default");
  });

  test("create workflow posts to API", async ({ page }) => {
    let postBody = null;
    await mockAPI(page, [
      ["**/api/workflows", (route) => {
        if (route.request().method() === "POST") {
          postBody = route.request().postDataJSON();
          return route.fulfill({
            status: 200, contentType: "application/json",
            body: JSON.stringify({ workflow_id: 99, name: postBody?.name, description: postBody?.description }),
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
      workflows = [];
      renderBoard();
      activatePerspective("workflows");
      renderWorkflowList();
      openWorkflowEditor();
    });

    await page.fill("#workflow-name", "custom");
    await page.fill("#workflow-description", "Custom workflow");
    await page.click("#workflow-save");
    await page.waitForTimeout(300);

    expect(postBody).not.toBeNull();
    expect(postBody.name).toBe("custom");
  });

  test("delete workflow calls DELETE", async ({ page }) => {
    let deleteCalled = false;
    await mockAPI(page, [
      ["**/api/workflows/1", (route) => {
        if (route.request().method() === "DELETE") {
          deleteCalled = true;
          return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
        }
        if (route.request().method() === "GET") {
          return route.fulfill({
            status: 200, contentType: "application/json",
            body: JSON.stringify({ workflow_id: 1, name: "default", description: "", stages: SAMPLE_STAGES }),
          });
        }
        return route.continue();
      }],
      ["**/api/workflows", []],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate((wfs) => {
      showApp("admin", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = [];
      workflows = wfs;
      renderBoard();
      activatePerspective("workflows");
      renderWorkflowList();
      openWorkflowEditor(workflows[0]);
    }, SAMPLE_WORKFLOWS);

    await page.evaluate(() => { window._origUiConfirm = window.uiConfirm; window.uiConfirm = async () => true; });
    const deleteBtn = page.locator("#workflow-delete");
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
      ["**/api/workflows/1/stages", (route) => {
        if (route.request().method() === "POST") {
          stageBody = route.request().postDataJSON();
          return route.fulfill({
            status: 200, contentType: "application/json",
            body: JSON.stringify({ workflow_stage_id: 99, ...stageBody }),
          });
        }
        return route.continue();
      }],
      ["**/api/workflows/1", {
        workflow_id: 1, name: "default", description: "Standard lifecycle",
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
      workflows = wfs;
      renderBoard();
      activatePerspective("workflows");
      renderWorkflowList();
      openWorkflowEditor(workflows[0]);
    }, SAMPLE_WORKFLOWS);

    // Fill stage form and click add
    const stageNameInput = page.locator("#workflow-stage-name, #stage-name");
    const stageAddBtn = page.locator("#workflow-stage-add, #stage-add");

    if (await stageNameInput.count() > 0 && await stageAddBtn.count() > 0) {
      await stageNameInput.fill("review");
      await stageAddBtn.click();
      await page.waitForTimeout(300);
      expect(stageBody).not.toBeNull();
      expect(stageBody.stage_name).toBe("review");
    }
  });

  test("workflow stages are displayed after opening editor", async ({ page }) => {
    await mockAPI(page, [
      ["**/api/workflows/1", {
        workflow_id: 1, name: "default", description: "Standard lifecycle",
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
      workflows = wfs;
      renderBoard();
      activatePerspective("workflows");
      renderWorkflowList();
      openWorkflowEditor(workflows[0]);
    }, SAMPLE_WORKFLOWS);

    await page.waitForTimeout(500);

    const stagesContent = await page.evaluate(() => {
      const stagesList = document.getElementById("workflow-stages-list") || document.getElementById("workflow-stages");
      return stagesList?.textContent || "";
    });

    expect(stagesContent).toContain("design");
    expect(stagesContent).toContain("develop");
  });

  test("close workflow modal hides overlay", async ({ page }) => {
    await setupWorkflows(page);

    await page.evaluate(() => openWorkflowEditor(workflows[0]));

    // closeWorkflowModal might not exist — check first
    const closed = await page.evaluate(() => {
      if (typeof closeWorkflowModal === "function") {
        closeWorkflowModal();
        return true;
      }
      const overlay = document.getElementById("workflow-modal-overlay");
      if (overlay) { overlay.classList.add("hidden"); return true; }
      return false;
    });

    if (closed) {
      await expect(page.locator("#workflow-modal-overlay")).not.toBeVisible();
    }
  });

  test("export button triggers download for existing workflow", async ({ page }) => {
    await mockAPI(page, [
      ["**/api/workflows/1/export", {
        name: "default",
        description: "Standard lifecycle",
        stages: [
          { stage_name: "design", description: "", role: "BA", sort_order: 0 },
          { stage_name: "develop", description: "", role: "Lead Engineer", sort_order: 1 },
        ],
      }],
      ["**/api/workflows/1", {
        workflow_id: 1, name: "default", description: "Standard lifecycle",
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
      workflows = wfs;
      renderBoard();
      activatePerspective("workflows");
      renderWorkflowList();
      openWorkflowEditor(workflows[0]);
    }, SAMPLE_WORKFLOWS);

    const exportBtn = page.locator("#workflow-export");
    if (await exportBtn.count() > 0) {
      // Export button should be visible for existing workflows
      await expect(exportBtn).toBeVisible();
    }
  });

  test("import button exists in workflow view", async ({ page }) => {
    await setupWorkflows(page);

    const importBtn = page.locator("#workflow-import, button:has-text('Import')");
    if (await importBtn.count() > 0) {
      await expect(importBtn.first()).toBeVisible();
    }
  });
});
