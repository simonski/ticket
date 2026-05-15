const { test, expect } = require("@playwright/test");
const { createMockAPI, gotoRoot, resetApp, resetLogin } = require("./helpers");

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
  await resetLogin(page);
});

test.describe("authentication", () => {
  test("login form submits credentials and transitions to app", async () => {
    let statusCallCount = 0;
    api.setRoutes([
      ["**/api/login", { token: "test-token-123" }],
      ["**/api/status", (route) => {
        statusCallCount++;
        return route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({
            authenticated: true,
            user: { username: "alice", role: "admin" },
            server_version: "1.0.0",
            registration_enabled: false,
            chat_enabled: false,
          }),
        });
      }],
      ["**/api/projects", [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }]],
      ["**/api/projects/*/tickets", []],
      ["**/api/projects/*/stories", []],
      ["**/api/projects/*/labels", []],
      ["**/api/agents", []],
      ["**/api/board/ws", (route) => route.abort()],
    ]);

    await page.evaluate(() => {
      document.getElementById("login-user").value = "alice";
      document.getElementById("login-pass").value = "secret";
      document.querySelector('#login-form button[type="submit"]').click();
    });

    await expect(page.locator("#login-screen")).toHaveClass(/hidden/);
    await expect(page.locator("#app")).not.toHaveClass(/hidden/);
    await expect(page.locator("#profile-avatar")).toHaveText("AL");
    expect(statusCallCount).toBe(1);
  });

  test("login failure shows error message", async () => {
    api.setRoutes([
      [
        "**/api/login",
        (route) =>
          route.fulfill({
            status: 401,
            contentType: "application/json",
            body: JSON.stringify({ error: "invalid credentials" }),
          }),
      ],
    ]);

    await page.evaluate(() => {
      document.getElementById("login-user").value = "bad";
      document.getElementById("login-pass").value = "wrong";
      document.querySelector('#login-form button[type="submit"]').click();
    });

    await expect(page.locator("#login-status")).toContainText("Login failed");
    await expect(page.locator("#login-screen")).toBeVisible();
  });

  test("register form toggles and submits", async () => {
    api.setRoutes([
      ["**/api/register", { status: "ok" }],
    ]);
    await resetLogin(page, { status: { registration_enabled: true, chat_enabled: false } });

    await page.click("#show-register");
    await expect(page.locator("#register-form")).toBeVisible();
    await expect(page.locator("#login-form")).toHaveClass(/hidden/);

    await page.evaluate(() => {
      document.getElementById("register-user").value = "newuser";
      document.getElementById("register-pass").value = "newpass";
      document.querySelector('#register-form button[type="submit"]').click();
    });

    await expect(page.locator("#login-status")).toContainText("Registered");
    await expect(page.locator("#register-form")).toHaveClass(/hidden/);
    await expect(page.locator("#login-form")).not.toHaveClass(/hidden/);
  });

  test("register shows pending approval message when auto-approve is disabled", async () => {
    api.setRoutes([
      ["**/api/register", { approved: false }],
    ]);
    await resetLogin(page, {
      status: { registration_enabled: true, registration_auto_approve: false, chat_enabled: false },
    });

    await page.click("#show-register");
    await page.evaluate(() => {
      document.getElementById("register-user").value = "pending-user";
      document.getElementById("register-pass").value = "newpass";
      document.querySelector('#register-form button[type="submit"]').click();
    });

    await expect(page.locator("#login-status")).toContainText("Wait for an admin to approve");
    await expect(page.locator("#register-form")).toHaveClass(/hidden/);
  });

  test("hide register button returns to login form", async () => {
    await resetLogin(page, { status: { registration_enabled: true, chat_enabled: false } });

    await page.click("#show-register");
    await expect(page.locator("#register-form")).toBeVisible();

    await page.click("#hide-register");
    await expect(page.locator("#register-form")).toHaveClass(/hidden/);
    await expect(page.locator("#login-form")).not.toHaveClass(/hidden/);
  });

  test("logout returns to login screen", async () => {
    api.setRoutes([["**/api/logout", { status: "ok" }]]);
    await resetApp(page, { username: "alice", role: "admin", tickets: [] });
    await page.evaluate(() => {
      token = "fake-token";
      localStorage.setItem("task-token", token);
    });

    await expect(page.locator("#app")).not.toHaveClass(/hidden/);
    await page.click("#profile-avatar");
    await expect(page.locator("#profile-menu")).not.toHaveClass(/hidden/);
    await page.click("#menu-logout");

    await expect(page.locator("#login-screen")).not.toHaveClass(/hidden/);
  });

  test("admin-only menu items are gated for non-admin users", async () => {
    await resetApp(page, { username: "viewer", role: "user", tickets: [] });

    const result = await page.evaluate(() => {
      switchPerspective("agents");
      const perspective = localStorage.getItem("task-perspective");

      const agentNav = document.querySelector('[data-left-panel-action="agents"]');
      const roleNav = document.querySelector('[data-left-panel-action="roles"]');
      const teamNav = document.querySelector('[data-left-panel-action="teams"]');
      const workflowNav = document.querySelector('[data-left-panel-action="workflows"]');
      const settingsNav = document.querySelector('[data-left-panel-action="settings"]');

      return {
        perspective,
        agentHidden: agentNav ? window.getComputedStyle(agentNav).display === "none" : true,
        roleHidden: roleNav ? window.getComputedStyle(roleNav).display === "none" : true,
        teamHidden: teamNav ? window.getComputedStyle(teamNav).display === "none" : true,
        workflowHidden: workflowNav ? window.getComputedStyle(workflowNav).display === "none" : true,
        settingsHidden: settingsNav ? window.getComputedStyle(settingsNav).display === "none" : true,
      };
    });

    expect(result.perspective).toBe("swimlanes");
    expect(result.agentHidden).toBe(true);
    expect(result.roleHidden).toBe(true);
    expect(result.teamHidden).toBe(true);
    expect(result.workflowHidden).toBe(true);
    expect(result.settingsHidden).toBe(true);
  });

  test("admin user sees admin-only nav items", async () => {
    await resetApp(page, { username: "admin", role: "admin", tickets: [] });

    const result = await page.evaluate(() => {
      const agentNav = document.querySelector('[data-left-panel-action="agents"]');
      const roleNav = document.querySelector('[data-left-panel-action="roles"]');
      const teamNav = document.querySelector('[data-left-panel-action="teams"]');
      const workflowNav = document.querySelector('[data-left-panel-action="workflows"]');
      const settingsNav = document.querySelector('[data-left-panel-action="settings"]');

      return {
        agentVisible: agentNav ? window.getComputedStyle(agentNav).display !== "none" : false,
        roleVisible: roleNav ? window.getComputedStyle(roleNav).display !== "none" : false,
        teamVisible: teamNav ? window.getComputedStyle(teamNav).display !== "none" : false,
        workflowVisible: workflowNav ? window.getComputedStyle(workflowNav).display !== "none" : false,
        settingsVisible: settingsNav ? window.getComputedStyle(settingsNav).display !== "none" : false,
      };
    });

    expect(result.agentVisible).toBe(true);
    expect(result.roleVisible).toBe(true);
    expect(result.teamVisible).toBe(true);
    expect(result.workflowVisible).toBe(true);
    expect(result.settingsVisible).toBe(true);
  });

  test("admin settings save registration auto-approve", async () => {
    let registrationPayload = null;
    api.setRoutes([
      ["**/api/config/registration", async (route) => {
        registrationPayload = JSON.parse(route.request().postData() || "{}");
        await route.fulfill({
          status: 200,
          contentType: "application/json",
          body: JSON.stringify({ registration_enabled: true, registration_auto_approve: false }),
        });
      }],
      ["**/api/config/chat_enabled", { chat_enabled: false }],
      ["**/api/config/chat_limits", { chat_max_connections: 2, chat_max_duration_minutes: 3 }],
    ]);
    await resetApp(page, { username: "admin", role: "admin", tickets: [] });
    await page.evaluate(() => {
      registrationEnabled = true;
      registrationAutoApprove = true;
      chatEnabled = false;
      chatMaxConnections = 2;
      chatMaxDurationMinutes = 3;
      switchPerspective("settings");
      populateSettingsPanel();
    });

    await page.uncheck("#settings-registration-auto-approve");
    await page.click("#settings-save");

    expect(registrationPayload).toEqual({ enabled: true, auto_approve: false });
    await expect(page.locator("#settings-status")).toContainText("Settings saved");
  });

});
