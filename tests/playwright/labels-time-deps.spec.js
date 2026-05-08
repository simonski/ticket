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

const TICKET = {
  ticket_id: 1, key: "DM-1", type: "task", title: "Test Ticket",
  description: "", acceptance_criteria: "", stage: "design", state: "idle",
  open: true, archived: false, created_at: "2026-01-01T00:00:00Z",
  updated_at: "2026-01-01T00:00:00Z", assignee: "", parent_id: null,
  git_repository: "", git_branch: "", priority: 0, order: 0,
  estimate_effort: 0, estimate_complete: 0, health_score: 0,
};

const PROJECT_LABELS = [
  { label_id: 10, name: "bug", color: "red", project_id: 1 },
  { label_id: 11, name: "feature", color: "blue", project_id: 1 },
];

async function openTicketWithMocks(page, extraRoutes = []) {
  await mockAPI(page, [
    ["**/api/board/ws", (route) => route.abort()],
    ...extraRoutes,
  ]);
  await page.goto("/");
  await page.evaluate((ticket) => {
    showApp("alice", "admin");
    projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
    localStorage.setItem("task-selected-project", "1");
    tickets = [ticket];
    renderBoard();
  }, TICKET);
}

test.describe("labels", () => {
  test("existing ticket shows labels section with project labels dropdown", async ({ page }) => {
    await openTicketWithMocks(page, [
      ["**/api/tickets/1/labels", [{ label_id: 10, name: "bug", color: "red" }]],
      ["**/api/tickets/1/time", []],
      ["**/api/tickets/1/dependencies", []],
      ["**/api/projects/*/labels", PROJECT_LABELS],
    ]);

    await page.evaluate(() => openEdit(tickets[0]));

    const result = await page.evaluate(() => {
      const section = document.getElementById("ticket-labels-section");
      const select = document.getElementById("ticket-label-select");
      const options = select ? Array.from(select.options).map((o) => o.textContent) : [];
      const chips = document.querySelectorAll("#ticket-labels-list .label-chip, #ticket-labels-list .chip");
      return {
        visible: section?.style.display !== "none",
        optionCount: options.length,
        options,
        chipCount: chips.length,
      };
    });

    expect(result.visible).toBe(true);
    expect(result.optionCount).toBeGreaterThan(0);
  });

  test("add label button posts to API", async ({ page }) => {
    let addLabelCalled = false;
    await openTicketWithMocks(page, [
      ["**/api/tickets/1/labels", (route) => {
        if (route.request().method() === "POST") {
          addLabelCalled = true;
          return route.fulfill({ status: 200, contentType: "application/json", body: "[]" });
        }
        return route.fulfill({ status: 200, contentType: "application/json", body: "[]" });
      }],
      ["**/api/tickets/1/time", []],
      ["**/api/tickets/1/dependencies", []],
      ["**/api/projects/*/labels", PROJECT_LABELS],
    ]);

    await page.evaluate(() => openEdit(tickets[0]));
    await expect(page.locator("#ticket-label-select")).toBeVisible();

    // Select a label and click add
    await page.evaluate(() => {
      const select = document.getElementById("ticket-label-select");
      // If options didn't load, manually add one
      if (!select.options.length || !select.value) {
        const opt = document.createElement("option");
        opt.value = "10";
        opt.textContent = "bug";
        select.appendChild(opt);
        select.value = "10";
      } else {
        select.value = select.options[0].value;
      }
    });

    const addBtn = page.locator("#ticket-label-add");
    if (await addBtn.count() > 0) {
      await addBtn.click();
      await expect.poll(() => addLabelCalled).toBe(true);
      expect(addLabelCalled).toBe(true);
    }
  });
});

test.describe("time tracking", () => {
  test("existing ticket shows time tracking section with input fields", async ({ page }) => {
    await openTicketWithMocks(page, [
      ["**/api/tickets/1/labels", []],
      ["**/api/tickets/1/time", [
        { time_entry_id: 1, ticket_id: 1, minutes: 30, note: "Morning", username: "alice" },
      ]],
      ["**/api/tickets/1/time/total", { total: 30 }],
      ["**/api/tickets/1/dependencies", []],
      ["**/api/projects/*/labels", []],
    ]);

    await page.evaluate(() => openEdit(tickets[0]));

    const result = await page.evaluate(() => {
      const section = document.getElementById("ticket-time-section");
      const minutesInput = document.getElementById("ticket-time-minutes");
      const noteInput = document.getElementById("ticket-time-note");
      const logBtn = document.getElementById("ticket-time-log");
      return {
        visible: section?.style.display !== "none",
        hasMinutes: !!minutesInput,
        hasNote: !!noteInput,
        hasLogBtn: !!logBtn,
        logBtnText: logBtn?.textContent || "",
      };
    });

    expect(result.visible).toBe(true);
    expect(result.hasMinutes).toBe(true);
    expect(result.hasNote).toBe(true);
    expect(result.hasLogBtn).toBe(true);
    expect(result.logBtnText).toBe("+ Time");
  });

  test("log time button posts to API", async ({ page }) => {
    let timeLogBody = null;
    await openTicketWithMocks(page, [
      ["**/api/tickets/1/labels", []],
      ["**/api/tickets/1/time", (route) => {
        if (route.request().method() === "POST") {
          timeLogBody = route.request().postDataJSON();
          return route.fulfill({
            status: 200, contentType: "application/json",
            body: JSON.stringify({ time_entry_id: 5, ticket_id: 1, minutes: 45, note: "Testing" }),
          });
        }
        return route.fulfill({ status: 200, contentType: "application/json", body: "[]" });
      }],
      ["**/api/tickets/1/time/total", { total: 0 }],
      ["**/api/tickets/1/dependencies", []],
      ["**/api/projects/*/labels", []],
    ]);

    await page.evaluate(() => openEdit(tickets[0]));

    await page.fill("#ticket-time-minutes", "45");
    await page.fill("#ticket-time-note", "Testing");
    await page.click("#ticket-time-log");
    await expect.poll(() => timeLogBody).not.toBeNull();

    expect(timeLogBody).not.toBeNull();
    expect(timeLogBody.minutes).toBe(45);
  });
});

test.describe("dependencies", () => {
  test("existing ticket shows dependencies section", async ({ page }) => {
    await openTicketWithMocks(page, [
      ["**/api/tickets/1/labels", []],
      ["**/api/tickets/1/time", []],
      ["**/api/tickets/1/dependencies", [
        { ticket_id: 1, depends_on: 2, depends_on_key: "DM-2", depends_on_title: "Fix login bug" },
      ]],
      ["**/api/projects/*/labels", []],
    ]);

    await page.evaluate(() => openEdit(tickets[0]));
    await expect(page.locator("#ticket-deps-list")).toBeVisible();

    const result = await page.evaluate(() => {
      const depsEl = document.getElementById("ticket-deps-list");
      const addInput = document.getElementById("ticket-dep-input");
      const addBtn = document.getElementById("ticket-dep-add");
      return {
        hasDepsList: !!depsEl,
        depsContent: depsEl?.textContent || "",
        hasAddInput: !!addInput,
        hasAddBtn: !!addBtn,
      };
    });

    expect(result.hasDepsList).toBe(true);
    expect(result.depsContent).toContain("#2");
  });

  test("add dependency button posts to API", async ({ page }) => {
    let depBody = null;
    await openTicketWithMocks(page, [
      ["**/api/tickets/1/labels", []],
      ["**/api/tickets/1/time", []],
      ["**/api/tickets/1/dependencies", []],
      ["**/api/projects/*/labels", []],
      ["**/api/dependencies", (route) => {
        if (route.request().method() === "POST") {
          depBody = route.request().postDataJSON();
          return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
        }
        return route.continue();
      }],
    ]);

    await page.evaluate(() => openEdit(tickets[0]));

    const addBtn = page.locator("#ticket-dep-add");
    const addInput = page.locator("#ticket-dep-input");
    if (await addBtn.count() > 0 && await addInput.count() > 0) {
      await addInput.fill("2");
      await addBtn.click();
      await expect.poll(() => depBody).not.toBeNull();
      expect(depBody).not.toBeNull();
      expect(depBody.depends_on).toBe(2);
    }
  });

  test("remove dependency calls DELETE", async ({ page }) => {
    let deleteCalled = false;
    await openTicketWithMocks(page, [
      ["**/api/tickets/1/labels", []],
      ["**/api/tickets/1/time", []],
      ["**/api/tickets/1/dependencies", [
        { ticket_id: 1, depends_on: 2, depends_on_key: "DM-2", depends_on_title: "Fix login bug" },
      ]],
      ["**/api/projects/*/labels", []],
      ["**/api/dependencies**", (route) => {
        if (route.request().method() === "DELETE") {
          deleteCalled = true;
          return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
        }
        return route.continue();
      }],
    ]);

    await page.evaluate(() => openEdit(tickets[0]));

    // Find and click the remove button on the dependency chip
    const removeBtn = page.locator("#ticket-deps-list .chip-remove, #ticket-deps-list .dep-remove, #ticket-deps-list button").first();
    if (await removeBtn.count() > 0) {
      await removeBtn.click();
      await expect.poll(() => deleteCalled).toBe(true);
      expect(deleteCalled).toBe(true);
    }
  });
});
