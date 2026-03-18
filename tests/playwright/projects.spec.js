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

const PROJECTS = [
  { project_id: 1, title: "Alpha", prefix: "AL", status: "open", visibility: "private" },
  { project_id: 2, title: "Beta", prefix: "BT", status: "open", visibility: "public" },
];

async function setupWithProjects(page) {
  await mockAPI(page, [
    ["**/api/board/ws", (route) => route.abort()],
  ]);
  await page.goto("/");
  await page.evaluate((projs) => {
    showApp("admin", "admin");
    projects = projs;
    localStorage.setItem("task-project", "1");
    tickets = [];
    renderProjMenu();
    renderBoard();
  }, PROJECTS);
}

test.describe("project management", () => {
  test("project dropdown shows available projects", async ({ page }) => {
    await setupWithProjects(page);

    await page.evaluate(() => openProjMenu());

    const result = await page.evaluate(() => {
      const menu = document.getElementById("proj-menu");
      return {
        visible: menu && !menu.classList.contains("hidden"),
        content: menu?.textContent || "",
      };
    });

    expect(result.visible).toBe(true);
    expect(result.content).toContain("Alpha");
    expect(result.content).toContain("Beta");
  });

  test("selecting a project updates localStorage", async ({ page }) => {
    await setupWithProjects(page);

    await page.evaluate(() => {
      setSelectedProjectID(2);
    });

    const selected = await page.evaluate(() => localStorage.getItem("task-project"));
    expect(selected).toBe("2");
  });

  test("create project modal opens with empty fields", async ({ page }) => {
    await setupWithProjects(page);

    await page.evaluate(() => openProjModal());

    await expect(page.locator("#proj-modal-overlay")).not.toHaveClass(/hidden/);
    await expect(page.locator("#proj-modal-title")).toHaveValue("");
  });

  test("create project posts to API", async ({ page }) => {
    let postBody = null;
    await mockAPI(page, [
      ["**/api/projects", (route) => {
        if (route.request().method() === "POST") {
          postBody = route.request().postDataJSON();
          return route.fulfill({
            status: 200, contentType: "application/json",
            body: JSON.stringify({ project_id: 99, title: postBody?.title, prefix: postBody?.prefix, status: "open" }),
          });
        }
        return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(PROJECTS) });
      }],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate((projs) => {
      showApp("admin", "admin");
      projects = projs;
      localStorage.setItem("task-project", "1");
      tickets = [];
      renderBoard();
      openProjModal();
    }, PROJECTS);

    await page.fill("#proj-modal-title", "Gamma");
    await page.fill("#proj-modal-prefix", "GM");

    await page.click("#proj-modal-create");
    await page.waitForTimeout(300);

    expect(postBody).not.toBeNull();
    expect(postBody.title).toBe("Gamma");
    expect(postBody.prefix).toBe("GM");
  });

  test("close project modal hides overlay", async ({ page }) => {
    await setupWithProjects(page);

    await page.evaluate(() => openProjModal());
    await expect(page.locator("#proj-modal-overlay")).not.toHaveClass(/hidden/);

    await page.evaluate(() => closeProjModal());
    await expect(page.locator("#proj-modal-overlay")).toHaveClass(/hidden/);
  });

  test("project members view loads user and team lists", async ({ page }) => {
    await mockAPI(page, [
      ["**/api/projects/1/users", [
        { user_id: 1, username: "alice", role: "owner" },
        { user_id: 2, username: "bob", role: "editor" },
      ]],
      ["**/api/projects/1/teams", [
        { team_id: 1, name: "Platform", role: "editor" },
      ]],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate((projs) => {
      showApp("admin", "admin");
      projects = projs;
      localStorage.setItem("task-project", "1");
      tickets = [];
      renderBoard();
    }, PROJECTS);

    await page.evaluate(() => {
      activatePerspective("members");
      loadProjectMembers();
    });
    await page.waitForTimeout(500);

    const result = await page.evaluate(() => {
      const membersEl = document.getElementById("project-members-list");
      const teamsEl = document.getElementById("project-teams-list");
      return {
        membersContent: membersEl?.textContent || "",
        teamsContent: teamsEl?.textContent || "",
      };
    });

    expect(result.membersContent).toContain("User #1");
    expect(result.membersContent).toContain("User #2");
    expect(result.teamsContent).toContain("Team #1");
  });

  test("add project member posts to API", async ({ page }) => {
    let postBody = null;
    await mockAPI(page, [
      ["**/api/projects/1/users", (route) => {
        if (route.request().method() === "POST") {
          postBody = route.request().postDataJSON();
          return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
        }
        return route.fulfill({ status: 200, contentType: "application/json", body: "[]" });
      }],
      ["**/api/projects/1/teams", []],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate((projs) => {
      showApp("admin", "admin");
      projects = projs;
      localStorage.setItem("task-project", "1");
      tickets = [];
      renderBoard();
      activatePerspective("members");
    }, PROJECTS);
    await page.evaluate(() => loadProjectMembers());
    await page.waitForTimeout(300);

    // Fill user ID and click add via evaluate (element may not be in viewport)
    await page.evaluate(() => {
      document.getElementById("project-member-user-id").value = "3";
    });
    await page.evaluate(() => {
      document.getElementById("project-member-add").click();
    });
    await page.waitForTimeout(300);
    expect(postBody).not.toBeNull();
  });
});
