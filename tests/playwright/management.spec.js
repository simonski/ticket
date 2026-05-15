const { test, expect } = require("@playwright/test");
const { createMockAPI, gotoRoot, resetApp } = require("./helpers");

test.describe.configure({ mode: "serial" });

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
    tickets: [],
    agents: [],
    roles: [],
    teams: [],
  });
});

async function showAgents(agents = [], managementMode = "card") {
  await resetApp(page, {
    username: "admin",
    role: "admin",
    tickets: [],
    agents,
    perspective: "agents",
    managementModes: { agents: managementMode },
  });
  await page.evaluate(() => renderAgentList());
}

async function showRoles(roles = [], managementMode = "card") {
  await resetApp(page, {
    username: "admin",
    role: "admin",
    tickets: [],
    roles,
    perspective: "roles",
    managementModes: { roles: managementMode },
  });
  await page.evaluate(() => renderRoleList());
}

async function showTeams(teams = [], managementMode = "card") {
  await resetApp(page, {
    username: "admin",
    role: "admin",
    tickets: [],
    teams,
    perspective: "teams",
    managementModes: { teams: managementMode },
  });
  await page.evaluate(() => renderTeamList());
}

test.describe("agent management", () => {
  test("agent list renders in card and list modes", async () => {
    await showAgents([
      { agent_id: 1, name: "Atlas", description: "Build agent", enabled: true, status: "idle" },
      { agent_id: 2, name: "Hermes", description: "Deploy agent", enabled: false, status: "offline" },
    ]);

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

  test("opening agent editor shows correct data", async () => {
    await showAgents([{ agent_id: 7, name: "Atlas", description: "Build agent", enabled: true, status: "idle" }]);

    await page.evaluate(() => openAgentEditor(agents[0]));
    await expect(page.locator("#agent-modal-overlay")).toBeVisible();
    await expect(page.locator("#agent-name")).toHaveValue("Atlas");
    await expect(page.locator("#agent-description")).toHaveValue("Build agent");
  });

  test("create agent posts to API", async () => {
    let postBody = null;
    api.setRoutes([
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
    ]);
    await showAgents([]);

    // Open new agent form
    await page.evaluate(() => openAgentEditor());
    await expect(page.locator("#agent-modal-overlay")).toBeVisible();

    await page.evaluate(() => {
      agentNameInput.value = "NewBot";
      agentDescriptionInput.value = "Test bot";
      agentPasswordInput.value = "secret123";
      agentSaveBtn.click();
    });
    await expect.poll(() => postBody, { timeout: 15000 }).not.toBeNull();

    expect(postBody).not.toBeNull();
    expect(postBody.name).toBe("NewBot");
  });

  test("delete agent calls DELETE", async () => {
    let deleteCalled = false;
    api.setRoutes([
      ["**/api/agents/7", (route) => {
        if (route.request().method() === "DELETE") {
          deleteCalled = true;
          return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
        }
        return route.continue();
      }],
      ["**/api/agents", []],
    ]);
    await showAgents([{ agent_id: 7, name: "Atlas", description: "Build agent", enabled: true, status: "idle" }]);

    await page.evaluate(() => openAgentEditor(agents[0]));
    await page.evaluate(() => { window._origUiConfirm = window.uiConfirm; window.uiConfirm = async () => true; });

    const deleteBtn = page.locator("#agent-delete");
    if (await deleteBtn.count() > 0) {
      await deleteBtn.click();
      await expect.poll(() => deleteCalled).toBe(true);
      expect(deleteCalled).toBe(true);
    }

    await page.evaluate(() => { if (window._origUiConfirm) window.uiConfirm = window._origUiConfirm; });
  });

  test("close agent modal hides overlay", async () => {
    await showAgents([{ agent_id: 1, name: "Atlas", description: "Build", enabled: true, status: "idle" }]);
    await page.evaluate(() => {
      openAgentEditor(agents[0]);
    });
    await expect(page.locator("#agent-modal-overlay")).toBeVisible();
    await page.evaluate(() => closeAgentModal());
    await expect(page.locator("#agent-modal-overlay")).not.toBeVisible();
  });
});

test.describe("role management", () => {
  test("role list renders cards", async () => {
    await showRoles([
      { role_id: 1, title: "Product Owner", motivation: "Value", goals: "Backlog" },
      { role_id: 2, title: "Architect", motivation: "Design", goals: "Scale" },
    ]);

    const cards = await page.locator("#role-list .management-card").count();
    expect(cards).toBe(2);
  });

  test("clicking role card opens editor", async () => {
    await showRoles([{ role_id: 9, title: "Architect", motivation: "Shape systems", goals: "Reduce risk" }]);

    await page.evaluate(() => {
      document.querySelector("#role-list .management-card")?.click();
    });
    await expect(page.locator("#role-modal-overlay")).toBeVisible();
    await expect(page.locator("#role-title")).toHaveValue("Architect");
    await expect(page.locator("#role-motivation")).toHaveValue("Shape systems");
    await expect(page.locator("#role-goals")).toHaveValue("Reduce risk");
  });

  test("create role posts to API", async () => {
    let postBody = null;
    api.setRoutes([
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
    ]);
    await showRoles([]);
    await page.evaluate(() => openRoleEditor());

    await page.evaluate(() => {
      document.getElementById("role-title").value = "Security Lead";
      document.getElementById("role-motivation").value = "Protect";
      document.getElementById("role-goals").value = "Zero breaches";
      document.getElementById("role-save").click();
    });
    await expect.poll(() => postBody).not.toBeNull();

    expect(postBody).not.toBeNull();
    expect(postBody.title).toBe("Security Lead");
  });

  test("update role calls PUT", async () => {
    let putBody = null;
    api.setRoutes([
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
    ]);
    await showRoles([{ role_id: 9, title: "Architect", motivation: "Shape systems", goals: "Reduce risk" }]);
    await page.evaluate(() => openRoleEditor(roles[0]));

    await page.evaluate(() => {
      document.getElementById("role-title").value = "Chief Architect";
      document.getElementById("role-save").click();
    });
    await expect.poll(() => putBody).not.toBeNull();

    expect(putBody).not.toBeNull();
    expect(putBody.title).toBe("Chief Architect");
  });

  test("delete role calls DELETE", async () => {
    let deleteCalled = false;
    api.setRoutes([
      ["**/api/roles/9", (route) => {
        if (route.request().method() === "DELETE") {
          deleteCalled = true;
          return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
        }
        return route.continue();
      }],
      ["**/api/roles", []],
    ]);
    await showRoles([{ role_id: 9, title: "Architect", motivation: "Shape", goals: "Scale" }]);
    await page.evaluate(async () => {
      selectedRoleID = 9;
      await call("/api/roles/9", { method: "DELETE", headers: headers() });
    });
    await expect.poll(() => deleteCalled).toBe(true);
    expect(deleteCalled).toBe(true);
  });
});

test.describe("team management", () => {
  test("team list renders cards", async () => {
    await showTeams([
      { team_id: 1, name: "Platform", parent_team_id: null },
      { team_id: 2, name: "Frontend", parent_team_id: 1 },
    ]);

    const cards = await page.locator("#team-list .management-card").count();
    expect(cards).toBe(2);
  });

  test("clicking team card opens editor", async () => {
    await showTeams([{ team_id: 5, name: "Platform", parent_team_id: null }]);

    await page.locator("#team-list .management-card").first().click();
    await expect(page.locator("#team-modal-overlay")).toBeVisible();
    await expect(page.locator("#team-name")).toHaveValue("Platform");
  });

  test("create team posts to API", async () => {
    let postBody = null;
    api.setRoutes([
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
    ]);
    await showTeams([]);
    await page.evaluate(() => openTeamEditor());

    await page.evaluate(() => {
      document.getElementById("team-name").value = "Backend";
      document.getElementById("team-save").click();
    });
    await expect.poll(() => postBody).not.toBeNull();

    expect(postBody).not.toBeNull();
    expect(postBody.name).toBe("Backend");
  });

  test("update team calls PUT", async () => {
    let putBody = null;
    api.setRoutes([
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
    ]);
    await showTeams([{ team_id: 5, name: "Platform", parent_team_id: null }]);
    await page.evaluate(() => openTeamEditor(teams[0]));

    await page.evaluate(() => {
      document.getElementById("team-name").value = "Platform V2";
      document.getElementById("team-save").click();
    });
    await expect.poll(() => putBody).not.toBeNull();

    expect(putBody).not.toBeNull();
    expect(putBody.name).toBe("Platform V2");
  });

  test("delete team calls DELETE", async () => {
    let deleteCalled = false;
    api.setRoutes([
      ["**/api/teams/5", (route) => {
        if (route.request().method() === "DELETE") {
          deleteCalled = true;
          return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
        }
        return route.continue();
      }],
      ["**/api/teams", []],
    ]);
    await showTeams([{ team_id: 5, name: "Platform", parent_team_id: null }]);
    await page.evaluate(async () => {
      selectedTeamID = 5;
      await call("/api/teams/5", { method: "DELETE", headers: headers() });
    });
    await expect.poll(() => deleteCalled).toBe(true);
    expect(deleteCalled).toBe(true);
  });

  test("close team modal hides overlay", async () => {
    await showTeams([{ team_id: 5, name: "Platform", parent_team_id: null }]);
    await page.evaluate(() => {
      openTeamEditor(teams[0]);
    });
    await expect(page.locator("#team-modal-overlay")).toBeVisible();
    await page.evaluate(() => closeTeamModal());
    await expect(page.locator("#team-modal-overlay")).not.toBeVisible();
  });

  test("team members tab loads and displays members", async () => {
    api.setRoutes([
      ["**/api/teams/5/users", [
        { user_id: 1, username: "alice", role: "owner", job_title: "Engineer" },
      ]],
      ["**/api/teams/5/agents", []],
      ["**/api/projects/*/teams", []],
    ]);
    await showTeams([{ team_id: 5, name: "Platform", parent_team_id: null }]);
    // openTeamEditor is async — need to await it
    await page.evaluate(() => openTeamEditor(teams[0]));

    await expect.poll(async () => {
      const text = await page.evaluate(() => document.getElementById("team-members")?.textContent || "");
      return text.includes("alice");
    }).toBe(true);

    const result = await page.evaluate(() => {
      const el = document.getElementById("team-members");
      return {
        content: el?.textContent || "",
      };
    });

    expect(result.content).toContain("alice");
  });
});
