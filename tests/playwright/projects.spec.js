const { test, expect } = require("@playwright/test");
const { createMockAPI, gotoRoot, resetApp } = require("./helpers");

test.describe.configure({ mode: "serial" });

const PROJECTS = [
  { project_id: 1, title: "Alpha", prefix: "AL", status: "open", visibility: "private" },
  { project_id: 2, title: "Beta", prefix: "BT", status: "open", visibility: "public" },
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
    projects: PROJECTS,
    tickets: [],
  });
});

async function showProjects(overrides = {}) {
  await resetApp(page, {
    username: "admin",
    role: "admin",
    projects: PROJECTS,
    tickets: [],
    ...overrides,
  });
}

test.describe("project management", () => {
  test("project dropdown shows available projects", async () => {
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

  test("selecting a project updates localStorage", async () => {
    await page.evaluate(() => {
      setSelectedProjectID(2);
    });

    const selected = await page.evaluate(() => localStorage.getItem("task-project"));
    expect(selected).toBe("2");
  });

  test("create project modal opens with empty fields", async () => {
    await page.evaluate(() => openProjModal());

    await expect(page.locator("#proj-modal-overlay")).not.toHaveClass(/hidden/);
    await expect(page.locator("#proj-modal-title")).toHaveValue("");
  });

  test("create project posts to API", async () => {
    let postBody = null;
    api.setRoutes([
      ["**/api/projects", (route) => {
        if (route.request().method() === "POST") {
          postBody = route.request().postDataJSON();
          return route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify({ project_id: 99, title: postBody?.title, prefix: postBody?.prefix, status: "open" }),
          });
        }
        return route.fulfill({ status: 200, contentType: "application/json", body: JSON.stringify(PROJECTS) });
      }],
    ]);
    await showProjects();
    await page.evaluate(() => openProjModal());

    await page.evaluate(() => {
      document.getElementById("proj-modal-title").value = "Gamma";
      document.getElementById("proj-modal-prefix").value = "GM";
      document.getElementById("proj-modal-create").click();
    });
    await expect.poll(() => postBody).not.toBeNull();

    expect(postBody).not.toBeNull();
    expect(postBody.title).toBe("Gamma");
    expect(postBody.prefix).toBe("GM");
  });

  test("close project modal hides overlay", async () => {
    await page.evaluate(() => openProjModal());
    await expect(page.locator("#proj-modal-overlay")).not.toHaveClass(/hidden/);

    await page.evaluate(() => closeProjModal());
    await expect(page.locator("#proj-modal-overlay")).toHaveClass(/hidden/);
  });

  test("project members view loads user and team lists", async () => {
    api.setRoutes([
      ["**/api/projects/1/users", [
        { user_id: 1, username: "alice", role: "owner" },
        { user_id: 2, username: "bob", role: "editor" },
      ]],
      ["**/api/projects/1/teams", [
        { team_id: 1, team_name: "Platform", role: "editor" },
      ]],
    ]);
    await showProjects({ perspective: "members" });

    await page.evaluate(() => loadProjectMembers());
    await expect.poll(async () => {
      const members = await page.evaluate(() => document.getElementById("project-members-list")?.textContent || "");
      return members.includes("alice") && members.includes("bob");
    }).toBe(true);

    const result = await page.evaluate(() => {
      const membersEl = document.getElementById("project-members-list");
      const teamsEl = document.getElementById("project-teams-list");
      return {
        membersContent: membersEl?.textContent || "",
        teamsContent: teamsEl?.textContent || "",
      };
    });

    expect(result.membersContent).toContain("alice");
    expect(result.membersContent).toContain("bob");
    expect(result.teamsContent).toContain("Platform");
  });

  test("add project member posts to API", async () => {
    let postBody = null;
    api.setRoutes([
      ["**/api/projects/1/users", (route) => {
        if (route.request().method() === "POST") {
          postBody = route.request().postDataJSON();
          return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
        }
        return route.fulfill({ status: 200, contentType: "application/json", body: "[]" });
      }],
      ["**/api/projects/1/teams", []],
    ]);
    await showProjects({ perspective: "members" });
    await page.evaluate(() => loadProjectMembers());
    await expect.poll(async () => {
      const input = await page.locator("#project-member-user-id").count();
      return input > 0;
    }).toBe(true);

    await page.evaluate(() => {
      document.getElementById("project-member-user-id").value = "3";
      document.getElementById("project-member-add").click();
    });
    await expect.poll(() => postBody).not.toBeNull();
    expect(postBody).not.toBeNull();
  });
});
