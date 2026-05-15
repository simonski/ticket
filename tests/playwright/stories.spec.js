const { test, expect } = require("@playwright/test");
const { createMockAPI, gotoRoot, resetApp } = require("./helpers");

test.describe.configure({ mode: "serial" });

const SAMPLE_STORIES = [
  { story_id: 1, title: "User auth story", description: "As a user I want to log in", project_id: 1 },
  { story_id: 2, title: "Dashboard story", description: "Build main dashboard", project_id: 1 },
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
    tickets: [],
    stories: SAMPLE_STORIES,
    perspective: "stories",
  });
  await page.evaluate(() => renderStoriesPanel());
});

async function showStories(stories = SAMPLE_STORIES) {
  await resetApp(page, {
    username: "admin",
    role: "admin",
    tickets: [],
    stories,
    perspective: "stories",
  });
  await page.evaluate(() => renderStoriesPanel());
}

test.describe("stories", () => {
  test("stories panel renders story list", async () => {
    const result = await page.evaluate(() => {
      const rows = document.querySelectorAll("#stories-list .story-row");
      return Array.from(rows).map((r) => r.textContent);
    });

    expect(result.length).toBe(2);
    expect(result[0]).toContain("User auth story");
    expect(result[1]).toContain("Dashboard story");
  });

  test("clicking a story opens the story modal", async () => {
    await page.evaluate(() => {
      openStoryModal(stories[0]);
    });

    await expect(page.locator("#story-modal-overlay")).not.toHaveClass(/hidden/);
    await expect(page.locator("#story-title")).toHaveValue("User auth story");
    await expect(page.locator("#story-description")).toHaveValue("As a user I want to log in");
  });

  test("new story modal opens with empty fields", async () => {
    await page.evaluate(() => openStoryModal());

    await expect(page.locator("#story-modal-overlay")).not.toHaveClass(/hidden/);
    await expect(page.locator("#story-title")).toHaveValue("");
    await expect(page.locator("#story-description")).toHaveValue("");
  });

  test("close story modal hides overlay", async () => {
    await page.evaluate(() => openStoryModal(stories[0]));
    await expect(page.locator("#story-modal-overlay")).not.toHaveClass(/hidden/);

    await page.evaluate(() => closeStoryModal());
    await expect(page.locator("#story-modal-overlay")).toHaveClass(/hidden/);
  });

  test("story save button posts to API for new story", async () => {
    let capturedBody = null;
    api.setRoutes([
      ["**/api/stories", (route) => {
        if (route.request().method() === "POST") {
          capturedBody = route.request().postDataJSON();
          return route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify({ story_id: 99, title: capturedBody?.title, description: capturedBody?.description, project_id: 1 }),
          });
        }
        return route.fulfill({ status: 200, contentType: "application/json", body: "[]" });
      }],
    ]);
    await showStories([]);
    await page.evaluate(() => openStoryModal());
    await page.evaluate(() => {
      document.getElementById("story-title").value = "New story";
      document.getElementById("story-description").value = "Description here";
      document.getElementById("story-save").click();
    });
    await expect.poll(() => capturedBody).not.toBeNull();

    expect(capturedBody).not.toBeNull();
    expect(capturedBody.title).toBe("New story");
  });

  test("story update calls PUT for existing story", async () => {
    let putBody = null;
    api.setRoutes([
      ["**/api/stories/1", (route) => {
        if (route.request().method() === "PUT") {
          putBody = route.request().postDataJSON();
          return route.fulfill({
            status: 200,
            contentType: "application/json",
            body: JSON.stringify({ story_id: 1, ...putBody }),
          });
        }
        return route.fulfill({ status: 200, contentType: "application/json", body: "{}" });
      }],
    ]);
    await showStories(SAMPLE_STORIES);
    await page.evaluate(() => openStoryModal(stories[0]));
    await page.evaluate(() => {
      document.getElementById("story-title").value = "Updated auth story";
      document.getElementById("story-save").click();
    });
    await expect.poll(() => putBody).not.toBeNull();

    expect(putBody).not.toBeNull();
    expect(putBody.title).toBe("Updated auth story");
  });
});
