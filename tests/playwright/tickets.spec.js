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

const SAMPLE_TICKETS = [
  {
    ticket_id: 1, key: "DM-1", type: "task", title: "Setup CI",
    description: "Configure CI pipeline", acceptance_criteria: "Green builds",
    stage: "design", state: "idle", open: true, archived: false,
    created_at: "2026-01-01T00:00:00Z", updated_at: "2026-01-01T00:00:00Z",
    assignee: "", parent_id: null, git_repository: "", git_branch: "",
    priority: 0, order: 0, estimate_effort: 0, estimate_complete: 0,
    health_score: 0,
  },
  {
    ticket_id: 2, key: "DM-2", type: "bug", title: "Fix login bug",
    description: "Login fails on Safari", acceptance_criteria: "Works on all browsers",
    stage: "develop", state: "active", open: true, archived: false,
    created_at: "2026-01-02T00:00:00Z", updated_at: "2026-01-02T00:00:00Z",
    assignee: "alice", parent_id: null, git_repository: "", git_branch: "",
    priority: 1, order: 0, estimate_effort: 3, estimate_complete: 0,
    health_score: 0,
  },
  {
    ticket_id: 3, key: "DM-3", type: "epic", title: "Auth overhaul",
    description: "Revamp auth system", acceptance_criteria: "",
    stage: "test", state: "idle", open: true, archived: false,
    created_at: "2026-01-03T00:00:00Z", updated_at: "2026-01-03T00:00:00Z",
    assignee: "", parent_id: null, git_repository: "", git_branch: "",
    priority: 0, order: 0, estimate_effort: 0, estimate_complete: 0,
    health_score: 0,
  },
];

async function setupApp(page, extraRoutes = []) {
  await mockAPI(page, [
    ["**/api/board/ws", (route) => route.abort()],
    ...extraRoutes,
  ]);
  await page.goto("/");
  await page.evaluate((tix) => {
    showApp("alice", "admin");
    projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
    if (typeof renderProjMenu === "function") renderProjMenu();
    const ps = document.getElementById("project-select");
    if (ps) ps.value = "1";
    localStorage.setItem("task-project", "1");
    tickets = tix;
    renderBoard();
  }, SAMPLE_TICKETS);
}

test.describe("ticket lifecycle", () => {
  test("board renders tickets in correct lanes", async ({ page }) => {
    await setupApp(page);

    const result = await page.evaluate(() => {
      const lanes = {};
      document.querySelectorAll(".lane").forEach((lane) => {
        const stage = lane.dataset.stage;
        const cards = Array.from(lane.querySelectorAll(".ticket"));
        lanes[stage] = cards.map((c) => c.dataset.ticketId);
      });
      return lanes;
    });

    expect(result.design).toContain("1");
    expect(result.develop).toContain("2");
    expect(result.test).toContain("3");
  });

  test("clicking a ticket card opens the edit modal", async ({ page }) => {
    await mockAPI(page, [
      ["**/api/tickets/1/labels", []],
      ["**/api/tickets/1/time", []],
      ["**/api/tickets/1/dependencies", []],
      ["**/api/projects/*/labels", []],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate((tix) => {
      showApp("alice", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = tix;
      renderBoard();
    }, SAMPLE_TICKETS);

    await page.click('article.ticket[data-ticket-id="1"]');

    await expect(page.locator("#modal-overlay")).not.toHaveClass(/hidden/);
    await expect(page.locator("#ticket-title")).toHaveValue("Setup CI");
    await expect(page.locator("#ticket-type")).toHaveValue("task");
    await expect(page.locator("#ticket-stage")).toHaveValue("design");
    await expect(page.locator("#ticket-state")).toHaveValue("idle");
    await expect(page.locator("#ticket-description")).toHaveValue("Configure CI pipeline");
    await expect(page.locator("#ticket-ac")).toHaveValue("Green builds");
  });

  test("opening a new ticket modal sets defaults", async ({ page }) => {
    await setupApp(page);

    await page.evaluate(() => openNew());

    await expect(page.locator("#modal-overlay")).not.toHaveClass(/hidden/);
    await expect(page.locator("#ticket-title")).toHaveValue("");
    await expect(page.locator("#ticket-type")).toHaveValue("task");
    await expect(page.locator("#ticket-stage")).toHaveValue("design");
    await expect(page.locator("#ticket-state")).toHaveValue("idle");
  });

  test("new ticket creation calls POST /api/tickets", async ({ page }) => {
    let capturedBody = null;
    await mockAPI(page, [
      [
        "**/api/tickets",
        (route) => {
          if (route.request().method() === "POST") {
            capturedBody = route.request().postDataJSON();
            return route.fulfill({
              status: 200,
              contentType: "application/json",
              body: JSON.stringify({
                ticket_id: 99, key: "DM-99", type: "task", title: capturedBody?.title || "",
                description: "", acceptance_criteria: "", stage: "design", state: "idle",
                open: true, archived: false, created_at: "2026-01-01T00:00:00Z",
                updated_at: "2026-01-01T00:00:00Z",
              }),
            });
          }
          return route.continue();
        },
      ],
      ["**/api/board/ws", (route) => route.abort()],
    ]);

    await page.goto("/");
    await page.evaluate((tix) => {
      showApp("alice", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = tix;
      renderBoard();
    }, SAMPLE_TICKETS);

    await page.evaluate(() => openNew());
    await page.fill("#ticket-title", "Brand new ticket");

    // Trigger persistModal
    await page.evaluate(() => persistModal());

    expect(capturedBody).not.toBeNull();
    expect(capturedBody.title).toBe("Brand new ticket");
    expect(capturedBody.project_id).toBe(1);
  });

  test("ticket update calls PUT /api/tickets/:id", async ({ page }) => {
    let capturedPut = null;
    await mockAPI(page, [
      ["**/api/tickets/1/labels", []],
      ["**/api/tickets/1/time", []],
      ["**/api/tickets/1/dependencies", []],
      ["**/api/projects/*/labels", []],
      [
        "**/api/tickets/1",
        (route) => {
          if (route.request().method() === "PUT") {
            capturedPut = route.request().postDataJSON();
            return route.fulfill({
              status: 200,
              contentType: "application/json",
              body: JSON.stringify({ ...SAMPLE_TICKETS[0], ...capturedPut }),
            });
          }
          return route.continue();
        },
      ],
      ["**/api/board/ws", (route) => route.abort()],
    ]);

    await page.goto("/");
    await page.evaluate((tix) => {
      showApp("alice", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = JSON.parse(JSON.stringify(tix));
      renderBoard();
    }, SAMPLE_TICKETS);

    // Open edit for ticket 1
    await page.evaluate(() => {
      const t = tickets.find((x) => x.ticket_id === 1);
      openEdit(t);
    });

    await page.fill("#ticket-title", "Updated CI Title");
    await page.evaluate(() => persistModal());

    expect(capturedPut).not.toBeNull();
    expect(capturedPut.title).toBe("Updated CI Title");
  });

  test("ticket delete calls DELETE /api/tickets/:id", async ({ page }) => {
    let deleteCalled = false;
    await mockAPI(page, [
      ["**/api/tickets/1/labels", []],
      ["**/api/tickets/1/time", []],
      ["**/api/tickets/1/dependencies", []],
      ["**/api/projects/*/labels", []],
      [
        "**/api/tickets/1",
        (route) => {
          if (route.request().method() === "DELETE") {
            deleteCalled = true;
            return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
          }
          if (route.request().method() === "PUT") {
            return route.fulfill({
              status: 200, contentType: "application/json",
              body: JSON.stringify(SAMPLE_TICKETS[0]),
            });
          }
          return route.continue();
        },
      ],
      ["**/api/board/ws", (route) => route.abort()],
    ]);

    await page.goto("/");
    await page.evaluate((tix) => {
      showApp("alice", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = JSON.parse(JSON.stringify(tix));
      renderBoard();
    }, SAMPLE_TICKETS);

    // Open ticket and trigger delete via the delete button
    await page.evaluate(() => {
      const t = tickets.find((x) => x.ticket_id === 1);
      openEdit(t);
    });

    // Intercept the confirmation dialog and auto-accept
    page.once("dialog", (d) => d.accept());
    const deleteBtn = page.locator("#ticket-delete-btn");
    if (await deleteBtn.count() > 0) {
      // Mock uiConfirm to return true
      await page.evaluate(() => {
        window._origUiConfirm = window.uiConfirm;
        window.uiConfirm = async () => true;
      });
      await deleteBtn.click();
      await page.waitForTimeout(200);
      expect(deleteCalled).toBe(true);
      await page.evaluate(() => {
        if (window._origUiConfirm) window.uiConfirm = window._origUiConfirm;
      });
    }
  });

  test("closing the modal hides the overlay", async ({ page }) => {
    await setupApp(page);

    await page.evaluate(() => {
      openEdit(tickets[0]);
    });
    await expect(page.locator("#modal-overlay")).not.toHaveClass(/hidden/);

    await page.evaluate(() => closeModal());
    await expect(page.locator("#modal-overlay")).toHaveClass(/hidden/);
  });

  test("drag-drop data attributes are set on ticket cards", async ({ page }) => {
    await setupApp(page);

    const result = await page.evaluate(() => {
      const card = document.querySelector('article.ticket[data-ticket-id="1"]');
      return {
        draggable: card?.getAttribute("draggable"),
        hasTicketId: !!card?.dataset.ticketId,
      };
    });

    expect(result.draggable).toBe("true");
    expect(result.hasTicketId).toBe(true);
  });

  test("moveTicketToStage updates local ticket and re-renders", async ({ page }) => {
    await mockAPI(page, [
      [
        "**/api/tickets/1",
        (route) => {
          if (route.request().method() === "PUT") {
            const body = route.request().postDataJSON();
            return route.fulfill({
              status: 200,
              contentType: "application/json",
              body: JSON.stringify({ ...SAMPLE_TICKETS[0], stage: body.stage, state: body.state }),
            });
          }
          return route.continue();
        },
      ],
      ["**/api/projects/*/tickets", SAMPLE_TICKETS],
      ["**/api/board/ws", (route) => route.abort()],
    ]);

    await page.goto("/");
    await page.evaluate((tix) => {
      showApp("alice", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = JSON.parse(JSON.stringify(tix));
      renderBoard();
    }, SAMPLE_TICKETS);

    await page.evaluate(() => moveTicketToStage(1, "develop"));

    const result = await page.evaluate(() => {
      const t = tickets.find((x) => x.ticket_id === 1);
      return { stage: t.stage };
    });

    expect(result.stage).toBe("develop");
  });

  test("ticket state and stage selects contain expected options", async ({ page }) => {
    await setupApp(page);

    await page.evaluate(() => openEdit(tickets[0]));

    const stages = await page.evaluate(() =>
      Array.from(document.getElementById("ticket-stage").options).map((o) => o.value)
    );
    const states = await page.evaluate(() =>
      Array.from(document.getElementById("ticket-state").options).map((o) => o.value)
    );

    expect(stages).toEqual(expect.arrayContaining(["design", "develop", "test", "done"]));
    expect(states).toEqual(expect.arrayContaining(["idle", "active", "success", "fail"]));
  });
});

test.describe("search", () => {
  test("search overlay opens, filters tickets, and clicking result opens edit", async ({ page }) => {
    await mockAPI(page, [
      ["**/api/tickets/2/labels", []],
      ["**/api/tickets/2/time", []],
      ["**/api/tickets/2/dependencies", []],
      ["**/api/projects/*/labels", []],
      ["**/api/board/ws", (route) => route.abort()],
    ]);

    await page.goto("/");
    await page.evaluate((tix) => {
      showApp("alice", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = tix;
      renderBoard();
    }, SAMPLE_TICKETS);

    await page.evaluate(() => openSearch());
    await expect(page.locator("#search-overlay")).not.toHaveClass(/hidden/);

    // Type search query
    await page.fill("#search-input", "login");

    await page.evaluate(() => renderSearchResults("login"));

    const results = await page.evaluate(() => {
      const items = document.querySelectorAll(".search-result");
      return Array.from(items).map((el) => el.textContent);
    });

    expect(results.length).toBe(1);
    expect(results[0]).toContain("Fix login bug");

    // Click the result
    await page.click(".search-result");
    await expect(page.locator("#search-overlay")).toHaveClass(/hidden/);
    await expect(page.locator("#modal-overlay")).not.toHaveClass(/hidden/);
    await expect(page.locator("#ticket-title")).toHaveValue("Fix login bug");
  });

  test("search shows all tickets when query is empty", async ({ page }) => {
    await setupApp(page);

    await page.evaluate(() => openSearch());
    await page.evaluate(() => renderSearchResults(""));

    const count = await page.evaluate(() =>
      document.querySelectorAll(".search-result").length
    );

    expect(count).toBe(3);
  });

  test("closing search hides the overlay", async ({ page }) => {
    await setupApp(page);

    await page.evaluate(() => openSearch());
    await expect(page.locator("#search-overlay")).not.toHaveClass(/hidden/);

    await page.evaluate(() => closeSearch());
    await expect(page.locator("#search-overlay")).toHaveClass(/hidden/);
  });
});
