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
          workflow_id: 1,
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
          workflow_id: null,
        },
      ],
      commentsByTicket: {
        "OPS-101": [{ author: "admin", text: "Initial note", date: "now" }],
      },
      labels: [{ label_id: 51, project_id: 1, name: "backend", color: "#ff6600", created_at: "now" }],
      ticketLabelIDs: { "OPS-101": [51] },
      dependenciesByTicket: {
        "OPS-101": [{ id: 71, project_id: 1, ticket_id: "OPS-101", depends_on: "OPS-100", created_by: "admin", created_at: "now" }],
      },
      nextDependencyID: 72,
      timeEntriesByTicket: {
        "OPS-101": [{ time_entry_id: 81, ticket_id: "OPS-101", user_id: "admin", minutes: 30, note: "Initial effort", created_at: "now" }],
      },
      nextTimeEntryID: 82,
      workflows: [
        {
          workflow_id: 1,
          name: "Delivery",
          description: "Default flow",
          stages: [
            { workflow_stage_id: 11, workflow_id: 1, stage_name: "backlog", description: "", definition_of_ready: "", definition_of_done: "", roles: [{ role_id: 5, title: "Engineer" }] },
            { workflow_stage_id: 12, workflow_id: 1, stage_name: "todo", description: "", definition_of_ready: "", definition_of_done: "", roles: [] },
            { workflow_stage_id: 13, workflow_id: 1, stage_name: "doing", description: "", definition_of_ready: "", definition_of_done: "", roles: [] },
            { workflow_stage_id: 14, workflow_id: 1, stage_name: "done", description: "", definition_of_ready: "", definition_of_done: "", roles: [] },
          ],
        },
      ],
      roles: [
        { role_id: 5, title: "Engineer", description: "Build", acceptance_criteria: "", workflow_id: 1 },
        { role_id: 6, title: "QA", description: "Verify", acceptance_criteria: "", workflow_id: 1 },
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
          workflow_id: body.workflow_id || null,
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
      if (path.match(/^\/api\/projects\/\d+\/interventions$/) && method === "GET") {
        const id = Number(path.split("/")[3]);
        return json(db.tickets.filter((ticket) => ticket.project_id === id && String(ticket.state || "").toLowerCase() === "fail"));
      }
      if (path.match(/^\/api\/projects\/\d+\/labels$/) && method === "GET") {
        const id = Number(path.split("/")[3]);
        return json(db.labels.filter((label) => label.project_id === id));
      }
      if (path === "/api/workflows" && method === "GET") {
        return json(db.workflows.map(({ stages, ...workflow }) => workflow));
      }
      if (path.match(/^\/api\/workflows\/\d+$/) && method === "GET") {
        const id = Number(last(path.split("/")));
        return json(db.workflows.find((item) => item.workflow_id === id));
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
      if (path.match(/^\/api\/tickets\/[^/]+\/comments$/) && method === "GET") {
        const id = path.split("/")[3];
        return json(db.commentsByTicket[id] || []);
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/comments$/) && method === "POST") {
        const id = path.split("/")[3];
        if (!db.commentsByTicket[id]) {
          db.commentsByTicket[id] = [];
        }
        const comment = {
          author: db.status.username,
          text: body.comment || "",
          date: "now",
        };
        db.commentsByTicket[id].unshift(comment);
        return json(comment, 201);
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/labels$/) && method === "GET") {
        const id = path.split("/")[3];
        const ids = db.ticketLabelIDs[id] || [];
        return json(ids.map((labelID) => db.labels.find((label) => label.label_id === labelID)).filter(Boolean));
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/labels$/) && method === "POST") {
        const id = path.split("/")[3];
        const labelID = Number(body.label_id);
        if (!db.ticketLabelIDs[id]) {
          db.ticketLabelIDs[id] = [];
        }
        if (!db.ticketLabelIDs[id].includes(labelID)) {
          db.ticketLabelIDs[id].push(labelID);
        }
        return json({ status: "added" });
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/labels\/\d+$/) && method === "DELETE") {
        const parts = path.split("/");
        const id = parts[3];
        const labelID = Number(parts[5]);
        db.ticketLabelIDs[id] = (db.ticketLabelIDs[id] || []).filter((item) => item !== labelID);
        return json({ status: "removed" });
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/dependencies$/) && method === "GET") {
        const id = path.split("/")[3];
        return json(db.dependenciesByTicket[id] || []);
      }
      if (path === "/api/dependencies" && method === "POST") {
        if (!db.dependenciesByTicket[body.ticket_id]) {
          db.dependenciesByTicket[body.ticket_id] = [];
        }
        const dependency = {
          id: db.nextDependencyID++,
          project_id: Number(body.project_id),
          ticket_id: body.ticket_id,
          depends_on: body.depends_on,
          created_by: db.status.username,
          created_at: "now",
        };
        db.dependenciesByTicket[body.ticket_id].push(dependency);
        return json(dependency, 201);
      }
      if (path === "/api/dependencies" && method === "DELETE") {
        const ticketID = url.searchParams.get("ticket_id");
        const dependsOn = url.searchParams.get("depends_on");
        db.dependenciesByTicket[ticketID] = (db.dependenciesByTicket[ticketID] || []).filter((item) => item.depends_on !== dependsOn);
        return json({ status: "deleted" });
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/time$/) && method === "GET") {
        const id = path.split("/")[3];
        return json(db.timeEntriesByTicket[id] || []);
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/time\/total$/) && method === "GET") {
        const id = path.split("/")[3];
        const total = (db.timeEntriesByTicket[id] || []).reduce((sum, entry) => sum + Number(entry.minutes || 0), 0);
        return json({ total });
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/time$/) && method === "POST") {
        const id = path.split("/")[3];
        if (!db.timeEntriesByTicket[id]) {
          db.timeEntriesByTicket[id] = [];
        }
        const entry = {
          time_entry_id: db.nextTimeEntryID++,
          ticket_id: id,
          user_id: db.status.username,
          minutes: Number(body.minutes || 0),
          note: body.note || "",
          created_at: "now",
        };
        db.timeEntriesByTicket[id].push(entry);
        return json(entry, 201);
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
            workflow_id: null,
          },
          body,
        );
        db.tickets.push(ticket);
        return json(ticket, 201);
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/(draft|undraft|open|close|archive|unarchive)$/) && method === "POST") {
        return json({ status: "ok" });
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/intervene$/) && method === "POST") {
        const id = path.split("/")[3];
        const ticket = db.tickets.find((item) => item.ticket_id === id);
        if (!ticket) {
          return json({ error: "not found" }, 404);
        }
        const outcome = String(body.outcome || "").toLowerCase();
        if (outcome === "cancel") {
          ticket.archived = true;
        } else {
          ticket.state = "idle";
        }
        let followUp = null;
        if (outcome === "split-work") {
          followUp = {
            ticket_id: "OPS-" + String(1000 + db.tickets.length),
            project_id: ticket.project_id,
            type: "task",
            title: "Follow-up: " + (ticket.title || "work"),
            description: body.message || "",
            acceptance_criteria: "",
            status: "open",
            stage: ticket.stage || "design",
            state: "idle",
            priority: ticket.priority || 1,
            order: 0,
            estimate_effort: 0,
            health_score: 0,
            draft: false,
            archived: false,
            workflow_id: ticket.workflow_id || null,
          };
          db.tickets.push(followUp);
        }
        return json({ ticket, follow_up: followUp, decision: outcome, intervention: true });
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/workflow$/) && (method === "POST" || method === "DELETE")) {
        return json({ status: "ok" });
      }
      if (path.match(/^\/api\/workflows\/\d+\/reorder$/) && method === "PUT") {
        const workflow = db.workflows.find((item) => item.workflow_id === Number(path.split("/")[3]));
        workflow.stages = body.stage_ids.map((id) => workflow.stages.find((stage) => stage.workflow_stage_id === id));
        return json({ status: "reordered" });
      }
      if (path.match(/^\/api\/workflows\/stages\/roles\/\d+\/\d+$/) && method === "POST") {
        const parts = path.split("/");
        const workflow = db.workflows.find((item) => item.workflow_id === Number(parts[5]));
        const stage = workflow.stages.find((item) => item.workflow_stage_id === Number(parts[6]));
        const role = db.roles.find((item) => item.role_id === Number(body.role_id));
        stage.roles.push({ role_id: role.role_id, title: role.title });
        return json({ status: "created" }, 201);
      }
      if (path.match(/^\/api\/workflows\/stages\/roles\/\d+\/\d+$/) && method === "PUT") {
        const parts = path.split("/");
        const workflow = db.workflows.find((item) => item.workflow_id === Number(parts[5]));
        const stage = workflow.stages.find((item) => item.workflow_stage_id === Number(parts[6]));
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
  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();

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

  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();
  await expect(page.locator("#ticket-board")).toContainText("Move me");
  await expect(page.locator("#login-screen")).toHaveClass(/hidden/);
});

test.beforeEach(async ({ page }) => {
  await installSite2Mock(page);
  await page.goto("/site2/");
  await page.locator("#login-username").fill("admin");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();
});

test("creates a project and persists default draft settings through the existing API", async ({ page }) => {
  await page.getByRole("button", { name: "Projects" }).click();
  await page.getByRole("button", { name: "New project" }).click();
  await expect(page.locator("#project-prefix")).toHaveAttribute("maxlength", "5");
  await expect(page.locator("#project-prefix")).toHaveAttribute("pattern", "[A-Z]{1,5}");
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

test("shows predicted next work entries on the board", async ({ page }) => {
  await expect(page.locator("#predicted-work-list")).toContainText("No forecastable tickets.");
});

test("reorders board stages through the Workflow reorder endpoint", async ({ page }) => {
  await page.dragAndDrop('[data-workflow-stage-id="11"]', '[data-workflow-stage-id="14"]');

  const requests = await page.evaluate(() => window.__site2Requests.filter((request) => request.path === "/api/workflows/1/reorder"));
  expect(requests.length).toBeGreaterThan(0);
});

test("adds a role inside the Workflow editor using the existing stage-role API", async ({ page }) => {
  await page.getByRole("button", { name: "Workflows" }).click();
  await expect(page.locator("#stage-grid")).toContainText("backlog");
  await expect(page.locator("#workflow-role-bank")).toContainText("Engineer");
  await page.locator('[data-add-role-select="12"]').selectOption("6");
  await page.locator('[data-add-role="12"]').click();

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ method: "POST", path: "/api/workflows/stages/roles/1/12", body: { role_id: 6 } }),
    ]),
  );
});

test("adds a comment from the ticket modal using the ticket comments endpoint", async ({ page }) => {
  await page.getByText("Move me").click();
  await expect(page.getByRole("heading", { name: "Comments" })).toBeVisible();
  await expect(page.locator("#ticket-comments")).toContainText("Initial note");
  await page.locator("#ticket-comment-input").fill("Looks good to me");
  await page.getByRole("button", { name: "Add comment" }).click();
  await expect(page.locator("#ticket-comments")).toContainText("Looks good to me");

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ method: "POST", path: "/api/tickets/OPS-101/comments", body: { comment: "Looks good to me" } }),
    ]),
  );
});

test("manages labels, dependencies, and time from the ticket modal", async ({ page }) => {
  await page.getByText("Move me").click();
  await expect(page.getByRole("heading", { name: "Labels" })).toBeVisible();
  await expect(page.locator("#ticket-labels")).toContainText("backend");
  await page.locator("#ticket-label-select").selectOption("51");
  await page.getByRole("button", { name: "Add label" }).click();

  await expect(page.getByRole("heading", { name: "Dependencies" })).toBeVisible();
  await expect(page.locator("#ticket-dependencies")).toContainText("OPS-100");
  await page.locator("#ticket-dependency-input").fill("OPS-102");
  await page.getByRole("button", { name: "Add dependency" }).click();
  await expect(page.locator("#ticket-dependencies")).toContainText("OPS-102");

  await expect(page.getByRole("heading", { name: "Time tracking" })).toBeVisible();
  await expect(page.locator("#ticket-time-total")).toContainText("30");
  await page.locator("#ticket-time-minutes").fill("15");
  await page.locator("#ticket-time-note").fill("Refactor");
  await page.getByRole("button", { name: "Log time" }).click();
  await expect(page.locator("#ticket-time-total")).toContainText("45");

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ method: "POST", path: "/api/tickets/OPS-101/labels", body: { label_id: 51 } }),
      expect.objectContaining({ method: "POST", path: "/api/dependencies", body: expect.objectContaining({ ticket_id: "OPS-101", depends_on: "OPS-102" }) }),
      expect.objectContaining({ method: "POST", path: "/api/tickets/OPS-101/time", body: { minutes: 15, note: "Refactor" } }),
    ]),
  );
});
