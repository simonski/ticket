const { test, expect } = require("@playwright/test");

// Helper: intercept API calls with mock responses
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

test.describe("authentication", () => {
  test("login form submits credentials and transitions to app", async ({ page }) => {
    let statusCallCount = 0;
    await mockAPI(page, [
      ["**/api/login", { token: "test-token-123" }],
      ["**/api/status", (route) => {
        statusCallCount++;
        // First call (page load): not authenticated
        // Second call (after login): authenticated
        if (statusCallCount <= 1) {
          return route.fulfill({
            status: 200, contentType: "application/json",
            body: JSON.stringify({ authenticated: false, server_version: "1.0.0", registration_enabled: false, chat_enabled: false }),
          });
        }
        return route.fulfill({
          status: 200, contentType: "application/json",
          body: JSON.stringify({ authenticated: true, user: { username: "alice", role: "admin" }, server_version: "1.0.0", registration_enabled: false, chat_enabled: false }),
        });
      }],
      ["**/api/projects", [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }]],
      ["**/api/projects/*/tickets", []],
      ["**/api/projects/*/stories", []],
      ["**/api/projects/*/labels", []],
      ["**/api/agents", []],
      ["**/api/board/ws", (route) => route.abort()],
    ]);

    await page.goto("/");
    await expect(page.locator("#login-screen")).toBeVisible();

    await page.fill("#login-user", "alice");
    await page.fill("#login-pass", "secret");
    await page.click('#login-form button[type="submit"]');

    await expect(page.locator("#login-screen")).toHaveClass(/hidden/);
    await expect(page.locator("#app")).not.toHaveClass(/hidden/);
    await expect(page.locator("#profile-avatar")).toHaveText("AL");
  });

  test("login failure shows error message", async ({ page }) => {
    await mockAPI(page, [
      [
        "**/api/login",
        (route) =>
          route.fulfill({
            status: 401,
            contentType: "application/json",
            body: JSON.stringify({ error: "invalid credentials" }),
          }),
      ],
      ["**/api/config/registration", { enabled: false }],
    ]);

    await page.goto("/");
    await page.fill("#login-user", "bad");
    await page.fill("#login-pass", "wrong");
    await page.click('#login-form button[type="submit"]');

    await expect(page.locator("#login-status")).toContainText("Login failed");
    await expect(page.locator("#login-screen")).toBeVisible();
  });

  test("register form toggles and submits", async ({ page }) => {
    await mockAPI(page, [
      ["**/api/register", { status: "ok" }],
      ["**/api/status", { authenticated: false, server_version: "1.0.0", registration_enabled: true, chat_enabled: false }],
    ]);

    await page.goto("/");

    // Enable registration button visibility
    await page.evaluate(() => canRegister(true));

    // Show register form
    await page.click("#show-register");
    await expect(page.locator("#register-form")).toBeVisible();
    await expect(page.locator("#login-form")).toHaveClass(/hidden/);

    await page.fill("#register-user", "newuser");
    await page.fill("#register-pass", "newpass");
    await page.click('#register-form button[type="submit"]');

    await expect(page.locator("#login-status")).toContainText("Registered");
    await expect(page.locator("#register-form")).toHaveClass(/hidden/);
    await expect(page.locator("#login-form")).not.toHaveClass(/hidden/);
  });

  test("register shows pending approval message when auto-approve is disabled", async ({ page }) => {
    await mockAPI(page, [
      ["**/api/register", { approved: false }],
      ["**/api/status", { authenticated: false, server_version: "1.0.0", registration_enabled: true, registration_auto_approve: false, chat_enabled: false }],
    ]);

    await page.goto("/");
    await page.evaluate(() => canRegister(true));
    await page.click("#show-register");

    await page.fill("#register-user", "pending-user");
    await page.fill("#register-pass", "newpass");
    await page.click('#register-form button[type="submit"]');

    await expect(page.locator("#login-status")).toContainText("Wait for an admin to approve");
    await expect(page.locator("#register-form")).toHaveClass(/hidden/);
  });

  test("hide register button returns to login form", async ({ page }) => {
    await page.goto("/");

    await page.evaluate(() => canRegister(true));
    await page.click("#show-register");
    await expect(page.locator("#register-form")).toBeVisible();

    await page.click("#hide-register");
    await expect(page.locator("#register-form")).toHaveClass(/hidden/);
    await expect(page.locator("#login-form")).not.toHaveClass(/hidden/);
  });

  test("logout returns to login screen", async ({ page }) => {
    await mockAPI(page, [
      ["**/api/logout", { status: "ok" }],
    ]);

    await page.goto("/");

    // Simulate logged-in state
    await page.evaluate(() => {
      showApp("alice", "admin");
      token = "fake-token";
    });

    await expect(page.locator("#app")).not.toHaveClass(/hidden/);

    // Click profile avatar then logout
    await page.click("#profile-avatar");
    await expect(page.locator("#profile-menu")).not.toHaveClass(/hidden/);
    await page.click("#menu-logout");

    await expect(page.locator("#login-screen")).not.toHaveClass(/hidden/);
  });

  test("admin-only menu items are gated for non-admin users", async ({ page }) => {
    await page.goto("/");

    const result = await page.evaluate(() => {
      showApp("viewer", "user");

      // Admin perspectives should redirect to swimlanes
      switchPerspective("agents");
      const perspective = localStorage.getItem("task-perspective");

      // Admin visibility: left panel items for admin-only views should be hidden
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

  test("admin user sees admin-only nav items", async ({ page }) => {
    await page.goto("/");

    const result = await page.evaluate(() => {
      showApp("admin", "admin");

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

  test("admin settings save registration auto-approve", async ({ page }) => {
    let registrationPayload = null;
    await mockAPI(page, [
      ["**/api/status", { authenticated: false, server_version: "1.0.0", registration_enabled: true, registration_auto_approve: true, chat_enabled: false }],
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

    await page.goto("/");
    await page.evaluate(() => {
      showApp("admin", "admin");
      token = "fake-token";
      switchPerspective("settings");
      populateSettingsPanel();
    });

    await page.uncheck("#settings-registration-auto-approve");
    await page.click("#settings-save");

    expect(registrationPayload).toEqual({ enabled: true, auto_approve: false });
    await expect(page.locator("#settings-status")).toContainText("Settings saved");
  });
});
