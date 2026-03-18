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

async function setupWithChat(page, chatEnabledValue = true) {
  await mockAPI(page, [
    ["**/api/board/ws", (route) => route.abort()],
  ]);
  await page.goto("/");
  await page.evaluate((enabled) => {
    showApp("admin", "admin");
    projects = [{ project_id: 1, title: "Demo", prefix: "DM", status: "open" }];
    localStorage.setItem("task-project", "1");
    tickets = [];
    chatEnabled = enabled;
    chatMaxConnections = 2;
    chatMaxDurationMinutes = 3;
    renderBoard();
    applyChatFeatureAvailability();
  }, chatEnabledValue);
}

test.describe("chat perspective", () => {
  test("chat view section exists and activates", async ({ page }) => {
    await setupWithChat(page, true);

    await page.evaluate(() => activatePerspective("chat"));

    const result = await page.evaluate(() => {
      const view = document.getElementById("view-chat");
      return view ? view.classList.contains("active") : false;
    });

    expect(result).toBe(true);
  });

  test("chat thread and composer elements present", async ({ page }) => {
    await setupWithChat(page, true);

    await page.evaluate(() => activatePerspective("chat"));

    const result = await page.evaluate(() => {
      const thread = document.getElementById("chat-thread");
      const input = document.getElementById("chat-input");
      const sendBtn = document.getElementById("chat-send");
      return {
        hasThread: !!thread,
        hasInput: !!input,
        hasSend: !!sendBtn,
        placeholder: input?.placeholder || "",
      };
    });

    expect(result.hasThread).toBe(true);
    expect(result.hasInput).toBe(true);
    expect(result.hasSend).toBe(true);
    expect(result.placeholder).toContain("Codex");
  });

  test("appendChatBubble adds bubble to thread", async ({ page }) => {
    await setupWithChat(page, true);

    await page.evaluate(() => {
      activatePerspective("chat");
      // Clear the initial system bubble added by showApp
      document.getElementById("chat-thread").innerHTML = "";
      appendChatBubble("user", "Hello world");
    });

    const result = await page.evaluate(() => {
      const thread = document.getElementById("chat-thread");
      const bubbles = thread.querySelectorAll(".chat-bubble");
      return {
        count: bubbles.length,
        text: bubbles[0]?.textContent || "",
        hasUserClass: bubbles[0]?.classList.contains("user") || false,
      };
    });

    expect(result.count).toBe(1);
    expect(result.text).toBe("Hello world");
    expect(result.hasUserClass).toBe(true);
  });

  test("multiple bubble roles render correctly", async ({ page }) => {
    await setupWithChat(page, true);

    await page.evaluate(() => {
      activatePerspective("chat");
      document.getElementById("chat-thread").innerHTML = "";
      appendChatBubble("user", "Question");
      appendChatBubble("assistant", "Answer");
      appendChatBubble("system", "Notice");
    });

    const result = await page.evaluate(() => {
      const thread = document.getElementById("chat-thread");
      const bubbles = thread.querySelectorAll(".chat-bubble");
      return Array.from(bubbles).map((b) => ({
        role: b.className.replace("chat-bubble ", ""),
        text: b.textContent,
      }));
    });

    expect(result).toHaveLength(3);
    expect(result[0].role).toContain("user");
    expect(result[1].role).toContain("assistant");
    expect(result[2].role).toContain("system");
  });

  test("send button adds user bubble when text entered", async ({ page }) => {
    await setupWithChat(page, true);

    // Mock chat WS to avoid real connection
    await page.evaluate(() => {
      activatePerspective("chat");
      // Override connectChatWS to prevent real websocket
      connectChatWS = () => {};
      chatSocket = null;
    });

    await page.fill("#chat-input", "Test message");

    await page.evaluate(() => sendChatInput());

    const result = await page.evaluate(() => {
      const thread = document.getElementById("chat-thread");
      const userBubbles = thread.querySelectorAll(".chat-bubble.user");
      return {
        count: userBubbles.length,
        text: userBubbles[0]?.textContent || "",
        inputEmpty: document.getElementById("chat-input").value === "",
      };
    });

    expect(result.count).toBe(1);
    expect(result.text).toBe("Test message");
    expect(result.inputEmpty).toBe(true);
  });

  test("send with empty input does nothing", async ({ page }) => {
    await setupWithChat(page, true);

    await page.evaluate(() => {
      activatePerspective("chat");
      connectChatWS = () => {};
      document.getElementById("chat-thread").innerHTML = "";
    });

    await page.evaluate(() => sendChatInput());

    const count = await page.evaluate(() => {
      return document.getElementById("chat-thread").querySelectorAll(".chat-bubble").length;
    });

    expect(count).toBe(0);
  });
});

test.describe("chat disabled", () => {
  test("chat input disabled when chatEnabled is false", async ({ page }) => {
    await setupWithChat(page, false);

    await page.evaluate(() => {
      activatePerspective("chat");
      updateChatInputAvailability();
    });

    const result = await page.evaluate(() => {
      const input = document.getElementById("chat-input");
      const btn = document.getElementById("chat-send");
      return {
        inputDisabled: input?.disabled || false,
        btnDisabled: btn?.disabled || false,
        placeholder: input?.placeholder || "",
      };
    });

    expect(result.inputDisabled).toBe(true);
    expect(result.btnDisabled).toBe(true);
    expect(result.placeholder).toContain("disabled");
  });

  test("sendChatInput when disabled shows system message", async ({ page }) => {
    await setupWithChat(page, false);

    await page.evaluate(() => {
      // Clear any existing bubbles, then send
      document.getElementById("chat-thread").innerHTML = "";
      sendChatInput();
    });

    const result = await page.evaluate(() => {
      const thread = document.getElementById("chat-thread");
      const bubbles = thread.querySelectorAll(".chat-bubble.system");
      return {
        count: bubbles.length,
        text: bubbles[0]?.textContent || "",
      };
    });

    expect(result.count).toBe(1);
    expect(result.text).toContain("disabled");
  });

  test("chat left panel item hidden when disabled", async ({ page }) => {
    await setupWithChat(page, false);

    const hidden = await page.evaluate(() => {
      const item = document.querySelector('[data-left-panel-action="chat"]');
      return item ? item.classList.contains("hidden") : true;
    });

    expect(hidden).toBe(true);
  });

  test("chat perspective item hidden when disabled", async ({ page }) => {
    await setupWithChat(page, false);

    const hidden = await page.evaluate(() => {
      const item = document.querySelector('[data-perspective="chat"]');
      return item ? item.classList.contains("hidden") : true;
    });

    expect(hidden).toBe(true);
  });
});

test.describe("chat settings", () => {
  test("settings panel shows chat controls", async ({ page }) => {
    await setupWithChat(page, true);

    await page.evaluate(() => {
      populateSettingsPanel();
      activatePerspective("settings");
    });

    const result = await page.evaluate(() => {
      const chatCheckbox = document.getElementById("settings-chat-enabled");
      const maxConn = document.getElementById("settings-chat-max-connections");
      const maxDur = document.getElementById("settings-chat-max-duration");
      return {
        hasChatCheckbox: !!chatCheckbox,
        chatChecked: chatCheckbox?.checked || false,
        hasMaxConn: !!maxConn,
        maxConnValue: maxConn?.value || "",
        hasMaxDur: !!maxDur,
        maxDurValue: maxDur?.value || "",
      };
    });

    expect(result.hasChatCheckbox).toBe(true);
    expect(result.chatChecked).toBe(true);
    expect(result.hasMaxConn).toBe(true);
    expect(result.maxConnValue).toBe("2");
    expect(result.hasMaxDur).toBe(true);
    expect(result.maxDurValue).toBe("3");
  });
});
