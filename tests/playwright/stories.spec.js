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

const SAMPLE_STORIES = [
  { story_id: 1, title: "User auth story", description: "As a user I want to log in", project_id: 1 },
  { story_id: 2, title: "Dashboard story", description: "Build main dashboard", project_id: 1 },
];

async function setupWithStories(page) {
  await mockAPI(page, [
    ["**/api/board/ws", (route) => route.abort()],
  ]);
  await page.goto("/");
  await page.evaluate((storiesData) => {
    showApp("admin", "admin");
    projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
    localStorage.setItem("task-project", "1");
    tickets = [];
    stories = storiesData;
    renderBoard();
    activatePerspective("stories");
    renderStoriesPanel();
  }, SAMPLE_STORIES);
}

test.describe("stories", () => {
  test("stories panel renders story list", async ({ page }) => {
    await setupWithStories(page);

    const result = await page.evaluate(() => {
      const rows = document.querySelectorAll("#stories-list .story-row");
      return Array.from(rows).map((r) => r.textContent);
    });

    expect(result.length).toBe(2);
    expect(result[0]).toContain("User auth story");
    expect(result[1]).toContain("Dashboard story");
  });

  test("clicking a story opens the story modal", async ({ page }) => {
    await setupWithStories(page);

    await page.evaluate(() => {
      openStoryModal(stories[0]);
    });

    await expect(page.locator("#story-modal-overlay")).not.toHaveClass(/hidden/);
    await expect(page.locator("#story-title")).toHaveValue("User auth story");
    await expect(page.locator("#story-description")).toHaveValue("As a user I want to log in");
  });

  test("new story modal opens with empty fields", async ({ page }) => {
    await setupWithStories(page);

    await page.evaluate(() => openStoryModal());

    await expect(page.locator("#story-modal-overlay")).not.toHaveClass(/hidden/);
    await expect(page.locator("#story-title")).toHaveValue("");
    await expect(page.locator("#story-description")).toHaveValue("");
  });

  test("close story modal hides overlay", async ({ page }) => {
    await setupWithStories(page);

    await page.evaluate(() => openStoryModal(stories[0]));
    await expect(page.locator("#story-modal-overlay")).not.toHaveClass(/hidden/);

    await page.evaluate(() => closeStoryModal());
    await expect(page.locator("#story-modal-overlay")).toHaveClass(/hidden/);
  });

  test("story save button posts to API for new story", async ({ page }) => {
    let capturedBody = null;
    await mockAPI(page, [
      ["**/api/stories", (route) => {
        if (route.request().method() === "POST") {
          capturedBody = route.request().postDataJSON();
          return route.fulfill({
            status: 200, contentType: "application/json",
            body: JSON.stringify({ story_id: 99, title: capturedBody?.title, description: capturedBody?.description, project_id: 1 }),
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
      stories = [];
      renderBoard();
      activatePerspective("stories");
    });

    await page.evaluate(() => openStoryModal());
    await page.fill("#story-title", "New story");
    await page.fill("#story-description", "Description here");

    await page.click("#story-save");
    await expect.poll(() => capturedBody).not.toBeNull();

    expect(capturedBody).not.toBeNull();
    expect(capturedBody.title).toBe("New story");
  });

  test("story update calls PUT for existing story", async ({ page }) => {
    let putBody = null;
    await mockAPI(page, [
      ["**/api/stories/1", (route) => {
        if (route.request().method() === "PUT") {
          putBody = route.request().postDataJSON();
          return route.fulfill({
            status: 200, contentType: "application/json",
            body: JSON.stringify({ story_id: 1, ...putBody }),
          });
        }
        return route.continue();
      }],
      ["**/api/board/ws", (route) => route.abort()],
    ]);
    await page.goto("/");
    await page.evaluate((storiesData) => {
      showApp("admin", "admin");
      projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
      localStorage.setItem("task-project", "1");
      tickets = [];
      stories = storiesData;
      renderBoard();
      activatePerspective("stories");
    }, SAMPLE_STORIES);

    await page.evaluate(() => openStoryModal(stories[0]));
    await page.fill("#story-title", "Updated auth story");
    await page.click("#story-save");
    await expect.poll(() => putBody).not.toBeNull();

    expect(putBody).not.toBeNull();
    expect(putBody.title).toBe("Updated auth story");
  });
});
