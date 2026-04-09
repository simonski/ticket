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
    agents = [];
    roles = [];
    teams = [];
    sdlcs = [];
    stories = [];
    renderBoard();
  });
}

test.describe("perspective switching", () => {
  test("switching perspectives activates the correct view section", async ({ page }) => {
    await setupAdmin(page);

    const perspectives = ["swimlanes", "stories", "agents", "roles", "teams", "settings"];

    for (const p of perspectives) {
      await page.evaluate((name) => activatePerspective(name), p);

      const result = await page.evaluate((name) => {
        const view = document.getElementById(`view-${name}`);
        return view ? view.classList.contains("active") : false;
      }, p);

      expect(result).toBe(true);
    }
  });

  test("perspective picker opens and closes", async ({ page }) => {
    await setupAdmin(page);

    await page.evaluate(() => openPerspectivePicker());
    await expect(page.locator("#perspective-overlay")).not.toHaveClass(/hidden/);

    await page.evaluate(() => closePerspectivePicker());
    await expect(page.locator("#perspective-overlay")).toHaveClass(/hidden/);
  });

  test("perspective picker shows available items for admin", async ({ page }) => {
    await setupAdmin(page);

    const items = await page.evaluate(() => {
      openPerspectivePicker();
      return visiblePerspectiveItems().map((el) => el.dataset.perspective || el.dataset.action || el.textContent.trim());
    });

    expect(items.length).toBeGreaterThan(3);
  });

  test("non-admin perspective picker hides admin items", async ({ page }) => {
    await mockAPI(page, [["**/api/board/ws", (route) => route.abort()]]);
    await page.goto("/");
    await page.evaluate(() => {
      showApp("viewer", "user");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      tickets = [];
      renderBoard();
    });

    const items = await page.evaluate(() => {
      openPerspectivePicker();
      const visible = visiblePerspectiveItems();
      return visible.map((el) => el.dataset.perspective || el.dataset.action || "");
    });

    expect(items).not.toContain("agents");
    expect(items).not.toContain("roles");
    expect(items).not.toContain("teams");
    expect(items).not.toContain("settings");
  });

  test("switchPerspective saves to localStorage", async ({ page }) => {
    await setupAdmin(page);

    await page.evaluate(() => activatePerspective("stories"));

    const saved = await page.evaluate(() => localStorage.getItem("task-perspective"));
    expect(saved).toBe("stories");
  });
});

test.describe("left panel", () => {
  test("left panel toggles open and closed", async ({ page }) => {
    await setupAdmin(page);

    // Panel should start open after showApp
    const initiallyOpen = await page.evaluate(() =>
      document.getElementById("left-panel").classList.contains("open")
    );
    expect(initiallyOpen).toBe(true);

    // Close it
    await page.evaluate(() => setLeftPanelOpen(false));
    const closed = await page.evaluate(() =>
      document.getElementById("left-panel").classList.contains("open")
    );
    expect(closed).toBe(false);

    // Toggle back open
    await page.evaluate(() => toggleLeftPanel());
    const reopened = await page.evaluate(() =>
      document.getElementById("left-panel").classList.contains("open")
    );
    expect(reopened).toBe(true);
  });

  test("left panel items trigger correct perspective", async ({ page }) => {
    await setupAdmin(page);

    // Click swimlanes
    await page.evaluate(() => setLeftPanelActive("swimlanes"));
    const active = await page.evaluate(() => {
      const btn = document.querySelector('[data-left-panel-action="swimlanes"]');
      return btn?.classList.contains("active") || false;
    });
    expect(active).toBe(true);
  });

  test("left panel active state highlights correct item", async ({ page }) => {
    await setupAdmin(page);

    const actions = ["swimlanes", "stories", "agents", "roles", "teams"];

    for (const action of actions) {
      await page.evaluate((a) => setLeftPanelActive(a), action);

      const result = await page.evaluate((a) => {
        const items = document.querySelectorAll("[data-left-panel-action]");
        const states = {};
        items.forEach((item) => {
          states[item.dataset.leftPanelAction] = item.classList.contains("active");
        });
        return states[a] || false;
      }, action);

      expect(result).toBe(true);
    }
  });
});

test.describe("profile menu", () => {
  test("profile avatar click opens menu", async ({ page }) => {
    await setupAdmin(page);

    await page.click("#profile-avatar");
    await expect(page.locator("#profile-menu")).not.toHaveClass(/hidden/);
  });

  test("profile menu shows user initials", async ({ page }) => {
    await setupAdmin(page);

    const text = await page.locator("#profile-avatar").textContent();
    expect(text).toBe("AD");
  });

  test("profile menu contains settings, agents, roles, teams, logout", async ({ page }) => {
    await setupAdmin(page);

    await page.click("#profile-avatar");

    const items = await page.evaluate(() => {
      const menu = document.getElementById("profile-menu");
      return menu ? menu.textContent : "";
    });

    expect(items).toContain("Settings");
    expect(items).toContain("Agents");
    expect(items).toContain("Roles");
    expect(items).toContain("Teams");
    expect(items).toContain("Logout");
  });

  test("clicking outside profile menu closes it", async ({ page }) => {
    await setupAdmin(page);

    await page.click("#profile-avatar");
    await expect(page.locator("#profile-menu")).not.toHaveClass(/hidden/);

    await page.evaluate(() => closeProfileMenu());
    await expect(page.locator("#profile-menu")).toHaveClass(/hidden/);
  });
});

test.describe("settings", () => {
  test("settings panel populates admin fields", async ({ page }) => {
    await setupAdmin(page);

    await page.evaluate(() => {
      registrationEnabled = true;
      chatEnabled = true;
      chatMaxConnections = 10;
      chatMaxDurationMinutes = 60;
      populateSettingsPanel();
      activatePerspective("settings");
    });

    const result = await page.evaluate(() => {
      const regCheckbox = document.getElementById("settings-registration-enabled");
      const chatCheckbox = document.getElementById("settings-chat-enabled");
      const maxConn = document.getElementById("settings-chat-max-connections");
      const maxDur = document.getElementById("settings-chat-max-duration");
      return {
        regChecked: regCheckbox?.checked,
        chatChecked: chatCheckbox?.checked,
        maxConn: maxConn?.value,
        maxDur: maxDur?.value,
      };
    });

    expect(result.regChecked).toBe(true);
    expect(result.chatChecked).toBe(true);
    expect(result.maxConn).toBe("10");
    expect(result.maxDur).toBe("60");
  });

  test("save settings posts config changes", async ({ page }) => {
    const configCalls = [];
    await mockAPI(page, [
      ["**/api/config/**", (route) => {
        if (route.request().method() === "PUT" || route.request().method() === "POST") {
          configCalls.push({ url: route.request().url(), body: route.request().postDataJSON() });
        }
        return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
      }],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate(() => {
      showApp("admin", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = [];
      registrationEnabled = false;
      chatEnabled = false;
      renderBoard();
      populateSettingsPanel();
      activatePerspective("settings");
    });

    await page.click("#settings-save");
    await page.waitForTimeout(300);

    expect(configCalls.length).toBeGreaterThan(0);
  });
});

test.describe("dialog system", () => {
  test("uiAlert shows dialog and resolves on OK", async ({ page }) => {
    await setupAdmin(page);

    const result = await page.evaluate(() => {
      let resolved = false;
      uiAlert("Test message").then(() => { resolved = true; });
      const box = document.getElementById("dialog-box");
      const message = box?.querySelector(".dialog-message, .dialog-body, p")?.textContent || box?.textContent || "";
      const overlay = document.getElementById("dialog-overlay");
      return {
        visible: overlay && !overlay.classList.contains("hidden"),
        content: message,
      };
    });

    expect(result.visible).toBe(true);
    expect(result.content).toContain("Test message");

    // Close dialog
    await page.evaluate(() => closeDialog(true));
    await expect(page.locator("#dialog-overlay")).toHaveClass(/hidden/);
  });

  test("uiConfirm shows dialog with custom OK text", async ({ page }) => {
    await setupAdmin(page);

    const result = await page.evaluate(() => {
      uiConfirm("Are you sure?", "Yes, delete");
      const box = document.getElementById("dialog-box");
      const okBtn = document.getElementById("dialog-ok");
      return {
        message: box?.textContent || "",
        okText: okBtn?.textContent || "",
      };
    });

    expect(result.message).toContain("Are you sure?");
    expect(result.okText).toContain("Yes, delete");

    await page.evaluate(() => closeDialog(false));
  });
});

test.describe("new ticket FAB", () => {
  test("new ticket button is visible when logged in", async ({ page }) => {
    await setupAdmin(page);

    await expect(page.locator("#new-ticket")).toBeVisible();
  });

  test("new ticket button is hidden on login screen", async ({ page }) => {
    await page.goto("/");

    const result = await page.evaluate(() => {
      const btn = document.getElementById("new-ticket");
      return btn ? btn.classList.contains("hidden") : true;
    });

    expect(result).toBe(true);
  });

  test("clicking new ticket button opens new ticket modal", async ({ page }) => {
    await setupAdmin(page);

    // Ensure project is selected and close panel to avoid click interception
    await page.evaluate(() => {
      localStorage.setItem("task-project", "1");
      setLeftPanelOpen(false);
    });

    await page.click("#new-ticket");

    await expect(page.locator("#modal-overlay")).not.toHaveClass(/hidden/);
    await expect(page.locator("#ticket-title")).toHaveValue("");
  });
});
