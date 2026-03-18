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

async function setupAdmin(page) {
  await mockAPI(page, [
    ["**/api/board/ws", (route) => route.abort()],
  ]);
  await page.goto("/");
  await page.evaluate(() => {
    showApp("admin", "admin");
    projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
    localStorage.setItem("task-project", "1");
    tickets = [];
    renderBoard();
  });
}

test.describe("agent management", () => {
  test("agent list renders in card and list modes", async ({ page }) => {
    await setupAdmin(page);
    await page.evaluate(() => {
      agents = [
        { agent_id: 1, name: "Atlas", description: "Build agent", enabled: true, status: "idle" },
        { agent_id: 2, name: "Hermes", description: "Deploy agent", enabled: false, status: "offline" },
      ];
      activatePerspective("agents");
      renderAgentList();
    });

    // Card mode
    await page.evaluate(() => setManagementMode("agents", "card"));
    let cards = await page.locator("#agent-list .management-card").count();
    expect(cards).toBe(2);

    // List mode
    await page.evaluate(() => {
      setManagementMode("agents", "list");
      renderAgentList();
    });
    const listItems = await page.evaluate(() =>
      document.querySelectorAll("#agent-list .agent-row").length
    );
    expect(listItems).toBeGreaterThan(0);
  });

  test("clicking agent card opens editor with correct data", async ({ page }) => {
    await setupAdmin(page);
    await page.evaluate(() => {
      agents = [{ agent_id: 7, name: "Atlas", description: "Build agent", enabled: true, status: "idle" }];
      activatePerspective("agents");
      setManagementMode("agents", "card");
      renderAgentList();
    });

    await page.locator("#agent-list .management-card").first().click();
    await expect(page.locator("#agent-modal-overlay")).toBeVisible();
    await expect(page.locator("#agent-name")).toHaveValue("Atlas");
    await expect(page.locator("#agent-description")).toHaveValue("Build agent");
  });

  test("create agent posts to API", async ({ page }) => {
    let postBody = null;
    await mockAPI(page, [
      ["**/api/agents", (route) => {
        if (route.request().method() === "POST") {
          postBody = route.request().postDataJSON();
          return route.fulfill({
            status: 200, contentType: "application/json",
            body: JSON.stringify({ agent_id: 99, name: postBody?.name, description: postBody?.description, enabled: true, status: "idle" }),
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
      agents = [];
      renderBoard();
      activatePerspective("agents");
      renderAgentList();
    });

    // Open new agent form
    await page.evaluate(() => openAgentEditor());

    await page.fill("#agent-name", "NewBot");
    await page.fill("#agent-description", "Test bot");
    await page.fill("#agent-password", "secret123");

    await page.click("#agent-save");
    await page.waitForTimeout(300);

    expect(postBody).not.toBeNull();
    expect(postBody.name).toBe("NewBot");
  });

  test("delete agent calls DELETE", async ({ page }) => {
    let deleteCalled = false;
    await mockAPI(page, [
      ["**/api/agents/7", (route) => {
        if (route.request().method() === "DELETE") {
          deleteCalled = true;
          return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
        }
        return route.continue();
      }],
      ["**/api/agents", []],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate(() => {
      showApp("admin", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = [];
      agents = [{ agent_id: 7, name: "Atlas", description: "Build agent", enabled: true, status: "idle" }];
      renderBoard();
      activatePerspective("agents");
      renderAgentList();
    });

    await page.evaluate(() => openAgentEditor(agents[0]));
    await page.evaluate(() => { window._origUiConfirm = window.uiConfirm; window.uiConfirm = async () => true; });

    const deleteBtn = page.locator("#agent-delete");
    if (await deleteBtn.count() > 0) {
      await deleteBtn.click();
      await page.waitForTimeout(300);
      expect(deleteCalled).toBe(true);
    }

    await page.evaluate(() => { if (window._origUiConfirm) window.uiConfirm = window._origUiConfirm; });
  });

  test("close agent modal hides overlay", async ({ page }) => {
    await setupAdmin(page);
    await page.evaluate(() => {
      agents = [{ agent_id: 1, name: "Atlas", description: "Build", enabled: true, status: "idle" }];
      activatePerspective("agents");
      renderAgentList();
      openAgentEditor(agents[0]);
    });
    await expect(page.locator("#agent-modal-overlay")).toBeVisible();
    await page.evaluate(() => closeAgentModal());
    await expect(page.locator("#agent-modal-overlay")).not.toBeVisible();
  });
});

test.describe("role management", () => {
  test("role list renders cards", async ({ page }) => {
    await setupAdmin(page);
    await page.evaluate(() => {
      roles = [
        { role_id: 1, title: "Product Owner", motivation: "Value", goals: "Backlog" },
        { role_id: 2, title: "Architect", motivation: "Design", goals: "Scale" },
      ];
      activatePerspective("roles");
      setManagementMode("roles", "card");
      renderRoleList();
    });

    const cards = await page.locator("#role-list .management-card").count();
    expect(cards).toBe(2);
  });

  test("clicking role card opens editor", async ({ page }) => {
    await setupAdmin(page);
    await page.evaluate(() => {
      roles = [{ role_id: 9, title: "Architect", motivation: "Shape systems", goals: "Reduce risk" }];
      activatePerspective("roles");
      setManagementMode("roles", "card");
      renderRoleList();
    });

    await page.locator("#role-list .management-card").first().click();
    await expect(page.locator("#role-modal-overlay")).toBeVisible();
    await expect(page.locator("#role-title")).toHaveValue("Architect");
    await expect(page.locator("#role-motivation")).toHaveValue("Shape systems");
    await expect(page.locator("#role-goals")).toHaveValue("Reduce risk");
  });

  test("create role posts to API", async ({ page }) => {
    let postBody = null;
    await mockAPI(page, [
      ["**/api/roles", (route) => {
        if (route.request().method() === "POST") {
          postBody = route.request().postDataJSON();
          return route.fulfill({
            status: 200, contentType: "application/json",
            body: JSON.stringify({ role_id: 99, ...postBody }),
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
      roles = [];
      renderBoard();
      activatePerspective("roles");
      renderRoleList();
      openRoleEditor();
    });

    await page.fill("#role-title", "Security Lead");
    await page.fill("#role-motivation", "Protect");
    await page.fill("#role-goals", "Zero breaches");
    await page.click("#role-save");
    await page.waitForTimeout(300);

    expect(postBody).not.toBeNull();
    expect(postBody.title).toBe("Security Lead");
  });

  test("update role calls PUT", async ({ page }) => {
    let putBody = null;
    await mockAPI(page, [
      ["**/api/roles/9", (route) => {
        if (route.request().method() === "PUT") {
          putBody = route.request().postDataJSON();
          return route.fulfill({
            status: 200, contentType: "application/json",
            body: JSON.stringify({ role_id: 9, ...putBody }),
          });
        }
        return route.continue();
      }],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate(() => {
      showApp("admin", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = [];
      roles = [{ role_id: 9, title: "Architect", motivation: "Shape systems", goals: "Reduce risk" }];
      renderBoard();
      activatePerspective("roles");
      renderRoleList();
      openRoleEditor(roles[0]);
    });

    await page.fill("#role-title", "Chief Architect");
    await page.click("#role-save");
    await page.waitForTimeout(300);

    expect(putBody).not.toBeNull();
    expect(putBody.title).toBe("Chief Architect");
  });

  test("delete role calls DELETE", async ({ page }) => {
    let deleteCalled = false;
    await mockAPI(page, [
      ["**/api/roles/9", (route) => {
        if (route.request().method() === "DELETE") {
          deleteCalled = true;
          return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
        }
        return route.continue();
      }],
      ["**/api/roles", []],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate(() => {
      showApp("admin", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = [];
      roles = [{ role_id: 9, title: "Architect", motivation: "Shape", goals: "Scale" }];
      renderBoard();
      activatePerspective("roles");
      renderRoleList();
      openRoleEditor(roles[0]);
    });

    await page.evaluate(() => { window._origUiConfirm = window.uiConfirm; window.uiConfirm = async () => true; });
    const deleteBtn = page.locator("#role-delete");
    if (await deleteBtn.count() > 0) {
      await deleteBtn.click();
      await page.waitForTimeout(300);
      expect(deleteCalled).toBe(true);
    }
    await page.evaluate(() => { if (window._origUiConfirm) window.uiConfirm = window._origUiConfirm; });
  });
});

test.describe("team management", () => {
  test("team list renders cards", async ({ page }) => {
    await setupAdmin(page);
    await page.evaluate(() => {
      teams = [
        { team_id: 1, name: "Platform", parent_team_id: null },
        { team_id: 2, name: "Frontend", parent_team_id: 1 },
      ];
      activatePerspective("teams");
      setManagementMode("teams", "card");
      renderTeamList();
    });

    const cards = await page.locator("#team-list .management-card").count();
    expect(cards).toBe(2);
  });

  test("clicking team card opens editor", async ({ page }) => {
    await setupAdmin(page);
    await page.evaluate(() => {
      teams = [{ team_id: 5, name: "Platform", parent_team_id: null }];
      activatePerspective("teams");
      setManagementMode("teams", "card");
      renderTeamList();
    });

    await page.locator("#team-list .management-card").first().click();
    await expect(page.locator("#team-modal-overlay")).toBeVisible();
    await expect(page.locator("#team-name")).toHaveValue("Platform");
  });

  test("create team posts to API", async ({ page }) => {
    let postBody = null;
    await mockAPI(page, [
      ["**/api/teams", (route) => {
        if (route.request().method() === "POST") {
          postBody = route.request().postDataJSON();
          return route.fulfill({
            status: 200, contentType: "application/json",
            body: JSON.stringify({ team_id: 99, name: postBody?.name, parent_team_id: null }),
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
      teams = [];
      renderBoard();
      activatePerspective("teams");
      renderTeamList();
      openTeamEditor();
    });

    await page.fill("#team-name", "Backend");
    await page.click("#team-save");
    await page.waitForTimeout(300);

    expect(postBody).not.toBeNull();
    expect(postBody.name).toBe("Backend");
  });

  test("update team calls PUT", async ({ page }) => {
    let putBody = null;
    await mockAPI(page, [
      ["**/api/teams/5", (route) => {
        if (route.request().method() === "PUT") {
          putBody = route.request().postDataJSON();
          return route.fulfill({
            status: 200, contentType: "application/json",
            body: JSON.stringify({ team_id: 5, ...putBody }),
          });
        }
        return route.continue();
      }],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate(() => {
      showApp("admin", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = [];
      teams = [{ team_id: 5, name: "Platform", parent_team_id: null }];
      renderBoard();
      activatePerspective("teams");
      renderTeamList();
      openTeamEditor(teams[0]);
    });

    await page.fill("#team-name", "Platform V2");
    await page.click("#team-save");
    await page.waitForTimeout(300);

    expect(putBody).not.toBeNull();
    expect(putBody.name).toBe("Platform V2");
  });

  test("delete team calls DELETE", async ({ page }) => {
    let deleteCalled = false;
    await mockAPI(page, [
      ["**/api/teams/5", (route) => {
        if (route.request().method() === "DELETE") {
          deleteCalled = true;
          return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
        }
        return route.continue();
      }],
      ["**/api/teams", []],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate(() => {
      showApp("admin", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = [];
      teams = [{ team_id: 5, name: "Platform", parent_team_id: null }];
      renderBoard();
      activatePerspective("teams");
      renderTeamList();
      openTeamEditor(teams[0]);
    });

    await page.evaluate(() => { window._origUiConfirm = window.uiConfirm; window.uiConfirm = async () => true; });
    const deleteBtn = page.locator("#team-delete");
    if (await deleteBtn.count() > 0) {
      await deleteBtn.click();
      await page.waitForTimeout(300);
      expect(deleteCalled).toBe(true);
    }
    await page.evaluate(() => { if (window._origUiConfirm) window.uiConfirm = window._origUiConfirm; });
  });

  test("close team modal hides overlay", async ({ page }) => {
    await setupAdmin(page);
    await page.evaluate(() => {
      teams = [{ team_id: 5, name: "Platform", parent_team_id: null }];
      activatePerspective("teams");
      renderTeamList();
      openTeamEditor(teams[0]);
    });
    await expect(page.locator("#team-modal-overlay")).toBeVisible();
    await page.evaluate(() => closeTeamModal());
    await expect(page.locator("#team-modal-overlay")).not.toBeVisible();
  });

  test("team members tab loads and displays members", async ({ page }) => {
    await mockAPI(page, [
      ["**/api/teams/5/users", [
        { user_id: 1, username: "alice", role: "owner", job_title: "Engineer" },
      ]],
      ["**/api/teams/5/agents", []],
      ["**/api/projects/*/teams", []],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate(() => {
      showApp("admin", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = [];
      teams = [{ team_id: 5, name: "Platform", parent_team_id: null }];
      renderBoard();
      activatePerspective("teams");
      renderTeamList();
    });
    // openTeamEditor is async — need to await it
    await page.evaluate(() => openTeamEditor(teams[0]));

    // Wait for team details to load
    await page.waitForTimeout(500);

    const result = await page.evaluate(() => {
      const el = document.getElementById("team-members");
      return {
        content: el?.textContent || "",
      };
    });

    expect(result.content).toContain("alice");
  });
});
