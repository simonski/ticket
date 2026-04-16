const { test, expect } = require("@playwright/test");

function installSite2Mock(page) {
  return page.addInitScript(() => {
    const db = {
      status: { username: "admin", mode: "local", version: "dev" },
      nextProjectID: 2,
      projects: [
        {
          project_id: 1,
          prefix: "OPS",
          title: "Operations",
          description: "Keep things running",
          acceptance_criteria: "",
          git_repository: "acme/ops",
          visibility: "public",
          sdlc_id: 1,
          default_draft: false,
        },
      ],
      tickets: [
        {
          ticket_id: "OPS-101",
          project_id: 1,
          type: "task",
          title: "Move me",
          description: "",
          acceptance_criteria: "",
          status: "open",
          stage: "design",
          priority: 1,
          order: 1,
          estimate_effort: 3,
          health_score: 5,
          draft: false,
          archived: false,
          sdlc_id: null,
        },
      ],
      sdlcs: [
        {
          sdlc_id: 1,
          name: "Delivery",
          description: "Default flow",
          stages: [
            { sdlc_stage_id: 11, sdlc_id: 1, stage_name: "backlog", description: "", definition_of_ready: "", definition_of_done: "", roles: [{ role_id: 5, title: "Engineer" }] },
            { sdlc_stage_id: 12, sdlc_id: 1, stage_name: "todo", description: "", definition_of_ready: "", definition_of_done: "", roles: [] },
            { sdlc_stage_id: 13, sdlc_id: 1, stage_name: "doing", description: "", definition_of_ready: "", definition_of_done: "", roles: [] },
            { sdlc_stage_id: 14, sdlc_id: 1, stage_name: "done", description: "", definition_of_ready: "", definition_of_done: "", roles: [] },
          ],
        },
      ],
      roles: [
        { role_id: 5, title: "Engineer", description: "Build", acceptance_criteria: "", sdlc_id: 1 },
        { role_id: 6, title: "QA", description: "Verify", acceptance_criteria: "", sdlc_id: 1 },
      ],
      agents: [{ user_id: "agent-1", enabled: true }],
      teams: [{ team_id: 21, name: "Platform", parent_team_id: null }],
    };

    window.__site2Requests = [];

    function json(body, status = 200) {
      return new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      });
    }

    function parseBody(body) {
      if (!body) {
        return {};
      }
      return JSON.parse(body);
    }

    function last(pathParts) {
      return pathParts[pathParts.length - 1];
    }

    window.__site2MockFetch = async (input, init = {}) => {
      const method = (init.method || "GET").toUpperCase();
      const url = new URL(typeof input === "string" ? input : input.url, window.location.origin);
      const path = url.pathname;
      const body = parseBody(init.body);

      window.__site2Requests.push({ method, path, body });

      if (path === "/api/status") {
        return json(db.status);
      }
      if (path === "/api/login" && method === "POST") {
        return json({ token: "test-token", user: { username: body.username || "admin" } });
      }
      if (path === "/api/logout" && method === "POST") {
        return json({ status: "ok" });
      }
      if (path === "/api/projects" && method === "GET") {
        return json(db.projects);
      }
      if (path === "/api/projects" && method === "POST") {
        const project = {
          project_id: db.nextProjectID++,
          prefix: body.prefix,
          title: body.title,
          description: body.description || "",
          acceptance_criteria: body.acceptance_criteria || "",
          git_repository: body.git_repository || "",
          visibility: body.visibility || "public",
          sdlc_id: body.sdlc_id || null,
          default_draft: false,
        };
        db.projects.push(project);
        return json(project, 201);
      }
      if (path.match(/^\/api\/projects\/\d+$/) && method === "PUT") {
        const id = Number(last(path.split("/")));
        const project = db.projects.find((item) => item.project_id === id);
        Object.assign(project, body);
        return json(project);
      }
      if (path.match(/^\/api\/projects\/\d+$/) && method === "DELETE") {
        const id = Number(last(path.split("/")));
        db.projects = db.projects.filter((item) => item.project_id !== id);
        return json({ status: "deleted" });
      }
      if (path.match(/^\/api\/projects\/\d+\/set-draft$/) && method === "PUT") {
        const id = Number(path.split("/")[3]);
        const project = db.projects.find((item) => item.project_id === id);
        project.default_draft = Boolean(body.draft);
        return json(project);
      }
      if (path.match(/^\/api\/projects\/\d+\/tickets$/) && method === "GET") {
        const id = Number(path.split("/")[3]);
        return json(db.tickets.filter((ticket) => ticket.project_id === id));
      }
      if (path === "/api/sdlcs" && method === "GET") {
        return json(db.sdlcs.map(({ stages, ...sdlc }) => sdlc));
      }
      if (path.match(/^\/api\/sdlcs\/\d+$/) && method === "GET") {
        const id = Number(last(path.split("/")));
        return json(db.sdlcs.find((item) => item.sdlc_id === id));
      }
      if (path === "/api/roles" && method === "GET") {
        return json(db.roles);
      }
      if (path === "/api/agents" && method === "GET") {
        return json(db.agents);
      }
      if (path === "/api/teams" && method === "GET") {
        return json(db.teams);
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/history$/) && method === "GET") {
        return json([{ action: "created", created_at: "now", comment: "" }]);
      }
      if (path.match(/^\/api\/tickets\/[^/]+$/) && method === "PUT") {
        const id = last(path.split("/"));
        const ticket = db.tickets.find((item) => item.ticket_id === id);
        if (!ticket) {
          return json({ error: "not found" }, 404);
        }
        Object.assign(ticket, body);
        return json(ticket);
      }
      if (path === "/api/tickets" && method === "POST") {
        const ticket = Object.assign(
          {
            ticket_id: "OPS-999",
            archived: false,
            draft: false,
            sdlc_id: null,
          },
          body,
        );
        db.tickets.push(ticket);
        return json(ticket, 201);
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/(draft|undraft|open|close|archive|unarchive)$/) && method === "POST") {
        return json({ status: "ok" });
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/sdlc$/) && (method === "POST" || method === "DELETE")) {
        return json({ status: "ok" });
      }
      if (path.match(/^\/api\/sdlcs\/\d+\/reorder$/) && method === "PUT") {
        const sdlc = db.sdlcs.find((item) => item.sdlc_id === Number(path.split("/")[3]));
        sdlc.stages = body.stage_ids.map((id) => sdlc.stages.find((stage) => stage.sdlc_stage_id === id));
        return json({ status: "reordered" });
      }
      if (path.match(/^\/api\/sdlcs\/stages\/roles\/\d+\/\d+$/) && method === "POST") {
        const parts = path.split("/");
        const sdlc = db.sdlcs.find((item) => item.sdlc_id === Number(parts[5]));
        const stage = sdlc.stages.find((item) => item.sdlc_stage_id === Number(parts[6]));
        const role = db.roles.find((item) => item.role_id === Number(body.role_id));
        stage.roles.push({ role_id: role.role_id, title: role.title });
        return json({ status: "created" }, 201);
      }
      if (path.match(/^\/api\/sdlcs\/stages\/roles\/\d+\/\d+$/) && method === "PUT") {
        const parts = path.split("/");
        const sdlc = db.sdlcs.find((item) => item.sdlc_id === Number(parts[5]));
        const stage = sdlc.stages.find((item) => item.sdlc_stage_id === Number(parts[6]));
        stage.roles = body.role_ids.map((id) => {
          const role = db.roles.find((item) => item.role_id === id);
          return { role_id: role.role_id, title: role.title };
        });
        return json({ status: "reordered" });
      }

      return json({ error: `Unhandled ${method} ${path}` }, 500);
    };
  });
}

test("focuses the username field on first load", async ({ page }) => {
  await installSite2Mock(page);
  await page.goto("/site2/");
  await page.evaluate(() => {
    sessionStorage.clear();
    window.location.reload();
  });
  await expect(page.locator("#login-username")).toBeVisible();
  await expect.poll(() => page.evaluate(() => document.activeElement && document.activeElement.id)).toBe("login-username");
});

test("does not emit CSP inline-style violations after login", async ({ page }) => {
  const messages = [];
  page.on("console", (message) => messages.push(message.text()));

  await installSite2Mock(page);
  await page.goto("/site2/");
  await page.evaluate(() => {
    sessionStorage.clear();
    window.location.reload();
  });
  await page.locator("#login-username").fill("admin");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByText("Ticket board")).toBeVisible();

  const cspMessages = messages.filter((text) => text.includes("Applying inline style violates the following Content Security Policy directive"));
  expect(cspMessages).toEqual([]);
});

test("keeps the session and visible tickets across refresh", async ({ page }) => {
  await installSite2Mock(page);
  await page.goto("/site2/");
  await page.evaluate(() => {
    sessionStorage.clear();
    window.location.reload();
  });
  await page.locator("#login-username").fill("admin");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.locator("#ticket-board")).toContainText("Move me");

  await page.reload();

  await expect(page.getByText("Ticket board")).toBeVisible();
  await expect(page.locator("#ticket-board")).toContainText("Move me");
  await expect(page.locator("#login-screen")).toHaveClass(/hidden/);
});

test.beforeEach(async ({ page }) => {
  await installSite2Mock(page);
  await page.goto("/site2/");
  await page.locator("#login-username").fill("admin");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByText("Ticket board")).toBeVisible();
});

test("creates a project and persists default draft settings through the existing API", async ({ page }) => {
  await page.getByRole("button", { name: "Projects" }).click();
  await page.getByRole("button", { name: "New project" }).click();
  await page.locator("#project-prefix").fill("WEB");
  await page.locator("#project-title").fill("Website");
  await page.locator("#project-default-draft").selectOption("true");
  await page.getByRole("button", { name: "Save project" }).click();

  await expect(page.locator("#project-list")).toContainText("Website");

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ method: "POST", path: "/api/projects", body: expect.objectContaining({ prefix: "WEB", title: "Website" }) }),
      expect.objectContaining({ method: "PUT", path: "/api/projects/2/set-draft", body: { draft: true } }),
    ]),
  );
});

test("moves a ticket across the board with drag and drop", async ({ page }) => {
  await expect(page.locator('[data-lane-stage="design"]')).toContainText("Move me");
  await page.dragAndDrop('[data-ticket-id="OPS-101"]', '[data-lane-stage="done"]');
  await expect(page.locator('[data-lane-stage="done"]')).toContainText("Move me");

  const requests = await page.evaluate(() => window.__site2Requests.filter((request) => request.path === "/api/tickets/OPS-101"));
  expect(requests.some((request) => request.body.stage === "done")).toBeTruthy();
});

test("adds a role inside the SDLC editor using the existing stage-role API", async ({ page }) => {
  await page.getByRole("button", { name: "SDLCs" }).click();
  await expect(page.locator("#stage-grid")).toContainText("backlog");
  await expect(page.locator("#sdlc-role-bank")).toContainText("Engineer");
  await page.locator('[data-add-role-select="12"]').selectOption("6");
  await page.locator('[data-add-role="12"]').click();

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ method: "POST", path: "/api/sdlcs/stages/roles/1/12", body: { role_id: 6 } }),
    ]),
  );
});
