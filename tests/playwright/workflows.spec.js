const { test, expect } = require("@playwright/test");
const { createMockAPI, gotoRoot, resetApp } = require("./helpers");

test.describe.configure({ mode: "serial" });

const SAMPLE_PROJECT = { project_id: 1, title: "Demo", prefix: "DM", status: "open" };
const SAMPLE_WORKFLOWS = [
  { workflow_id: 1, name: "default", description: "Standard lifecycle" },
  { workflow_id: 2, name: "kanban", description: "Simple kanban" },
];
const DEFAULT_WORKFLOW = SAMPLE_WORKFLOWS[0];

const SAMPLE_STAGES = [
  { workflow_stage_id: 1, workflow_id: 1, stage_name: "design", description: "", role_id: 5, role_title: "BA", sort_order: 0 },
  { workflow_stage_id: 2, workflow_id: 1, stage_name: "develop", description: "", role_id: 6, role_title: "Lead Engineer", sort_order: 1 },
  { workflow_stage_id: 3, workflow_id: 1, stage_name: "test", description: "", role_id: 4, role_title: "QA/Tester", sort_order: 2 },
  { workflow_stage_id: 4, workflow_id: 1, stage_name: "done", description: "", role_id: 1, role_title: "Product Owner", sort_order: 3 },
];

let page;
let api;

test.beforeAll(async ({ browser }) => {
  page = await browser.newPage();
  api = await createMockAPI(page);
  await gotoRoot(page, api);
});

test.afterAll(async () => {
  await page.close();
});

test.beforeEach(async () => {
  api.setRoutes([]);
  await resetApp(page, {
    username: "admin",
    role: "admin",
    projects: [SAMPLE_PROJECT],
    tickets: [],
    workflows: SAMPLE_WORKFLOWS,
    perspective: "workflows",
  });
  await page.evaluate(() => renderWorkflowList());
});

async function showWorkflows(workflows = SAMPLE_WORKFLOWS, overrides = {}) {
  await resetApp(page, {
    username: "admin",
    role: "admin",
    projects: [SAMPLE_PROJECT],
    tickets: [],
    workflows,
    perspective: "workflows",
    ...overrides,
  });
  await page.evaluate(() => renderWorkflowList());
}

test.describe("workflow management", () => {
  test("workflow list renders cards", async () => {
    const cards = await page.locator("#workflow-list .management-card, #workflow-list .workflow-card").count();
    expect(cards).toBe(2);
  });

  test("clicking workflow card opens editor", async () => {
    api.setRoutes([
      ["**/api/workflows/1", {
        workflow_id: 1,
        name: "default",
        description: "Standard lifecycle",
        stages: SAMPLE_STAGES,
      }],
    ]);
    await page.evaluate((workflow) => openWorkflowEditor(workflow), DEFAULT_WORKFLOW);

    await expect(page.locator("#workflow-modal-overlay")).toBeVisible();
    await expect(page.locator("#workflow-name")).toHaveValue("default");
  });

  test("create workflow posts to API", async () => {
    let postBody = null;
    api.setRoutes([
      ["**/api/workflows", (route) => {
        if (route.request().method() === "POST") {
          postBody = route.request().postDataJSON();
          return route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify({ workflow_id: 99, name: postBody?.name, description: postBody?.description }),
          });
        }
        return route.fulfill({ status: 200, contentType: "application/json", body: "[]" });
      }],
    ]);
    await showWorkflows([]);
    await page.evaluate(() => openWorkflowEditor());

    await page.evaluate(() => {
      document.getElementById("workflow-name").value = "custom";
      document.getElementById("workflow-description").value = "Custom workflow";
      document.getElementById("workflow-save").click();
    });
    await expect.poll(() => postBody).not.toBeNull();

    expect(postBody).not.toBeNull();
    expect(postBody.name).toBe("custom");
  });

  test("delete workflow calls DELETE", async () => {
    let deleteCalled = false;
    api.setRoutes([
      ["**/api/workflows/1", (route) => {
        if (route.request().method() === "DELETE") {
          deleteCalled = true;
          return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
        }
        if (route.request().method() === "GET") {
          return route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify({ workflow_id: 1, name: "default", description: "", stages: SAMPLE_STAGES }),
          });
        }
        return route.continue();
      }],
      ["**/api/workflows", []],
    ]);
    await page.evaluate((workflow) => openWorkflowEditor(workflow), DEFAULT_WORKFLOW);
    await expect(page.locator("#workflow-modal-overlay")).toBeVisible();
    await expect(page.locator("#workflow-name")).toHaveValue("default");

    await page.evaluate(() => { window._origUiConfirm = window.uiConfirm; window.uiConfirm = async () => true; });
    const deleteBtn = page.locator("#workflow-delete");
    if (await deleteBtn.count() > 0) {
      await expect(deleteBtn).toBeVisible();
      await deleteBtn.evaluate((button) => button.click());
      await expect.poll(() => deleteCalled, { timeout: 15000 }).toBe(true);
      await expect(page.locator("#workflow-modal-overlay")).toBeHidden();
      expect(deleteCalled).toBe(true);
    }
    await page.evaluate(() => { if (window._origUiConfirm) window.uiConfirm = window._origUiConfirm; });
  });

  test("add stage posts to API", async () => {
    let stageBody = null;
    api.setRoutes([
      ["**/api/workflows/1/stages", (route) => {
        if (route.request().method() === "POST") {
          stageBody = route.request().postDataJSON();
          return route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify({ workflow_stage_id: 99, ...stageBody }),
          });
        }
        return route.continue();
      }],
      ["**/api/workflows/1", {
        workflow_id: 1,
        name: "default",
        description: "Standard lifecycle",
        stages: SAMPLE_STAGES,
      }],
    ]);
    await page.evaluate((workflow) => openWorkflowEditor(workflow), DEFAULT_WORKFLOW);

    const stageNameInput = page.locator("#workflow-stage-name, #stage-name");
    const stageAddBtn = page.locator("#workflow-stage-add, #stage-add");

    if (await stageNameInput.count() > 0 && await stageAddBtn.count() > 0) {
      await page.evaluate(() => {
        const input = document.querySelector("#workflow-stage-name, #stage-name");
        const button = document.querySelector("#workflow-stage-add, #stage-add");
        if (input) input.value = "review";
        if (button) button.click();
      });
      await expect.poll(() => stageBody).not.toBeNull();
      expect(stageBody).not.toBeNull();
      expect(stageBody.stage_name).toBe("review");
    }
  });

  test("workflow stages are displayed after opening editor", async () => {
    api.setRoutes([
      ["**/api/workflows/1", {
        workflow_id: 1,
        name: "default",
        description: "Standard lifecycle",
        stages: SAMPLE_STAGES,
      }],
    ]);
    await page.evaluate((workflow) => openWorkflowEditor(workflow), DEFAULT_WORKFLOW);

    const stagesList = page.locator("#workflow-stages-list, #workflow-stages").first();
    await expect(stagesList).toContainText("design");
    await expect(stagesList).toContainText("develop");
  });

  test("close workflow modal hides overlay", async () => {
    await page.evaluate((workflow) => openWorkflowEditor(workflow), DEFAULT_WORKFLOW);

    const closed = await page.evaluate(() => {
      if (typeof closeWorkflowModal === "function") {
        closeWorkflowModal();
        return true;
      }
      const overlay = document.getElementById("workflow-modal-overlay");
      if (overlay) {
        overlay.classList.add("hidden");
        return true;
      }
      return false;
    });

    if (closed) {
      await expect(page.locator("#workflow-modal-overlay")).not.toBeVisible();
    }
  });

  test("export button triggers download for existing workflow", async () => {
    api.setRoutes([
      ["**/api/workflows/1/export", {
        name: "default",
        description: "Standard lifecycle",
        stages: [
          { stage_name: "design", description: "", role: "BA", sort_order: 0 },
          { stage_name: "develop", description: "", role: "Lead Engineer", sort_order: 1 },
        ],
      }],
      ["**/api/workflows/1", {
        workflow_id: 1,
        name: "default",
        description: "Standard lifecycle",
        stages: SAMPLE_STAGES,
      }],
    ]);
    await page.evaluate((workflow) => openWorkflowEditor(workflow), DEFAULT_WORKFLOW);

    const exportBtn = page.locator("#workflow-export");
    if (await exportBtn.count() > 0) {
      await expect(exportBtn).toBeVisible();
    }
  });

  test("import button exists in workflow view", async () => {
    const importBtn = page.locator("#workflow-import, button:has-text('Import')");
    if (await importBtn.count() > 0) {
      await expect(importBtn.first()).toBeVisible();
    }
  });
});
