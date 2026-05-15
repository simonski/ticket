const { test, expect } = require("@playwright/test");
const { createMockAPI, gotoRoot, resetApp } = require("./helpers");

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

async function setupWithChat(chatEnabledValue = true) {
  api.setRoutes([
    ["**/api/board/ws", (route) => route.abort()],
    ["**/api/chat/ws", (route) => route.abort()],
  ]);
  await resetApp(page, {
    username: "admin",
    role: "admin",
    tickets: [],
    chatEnabled: chatEnabledValue,
    chatMaxConnections: 2,
    chatMaxDurationMinutes: 3,
  });
  await page.evaluate(() => {
    closeChatWS();
    chatRunningProcesses = 0;
    chatCapacityBlocked = false;
    activeAssistantBubble = null;
    const thread = document.getElementById("chat-thread");
    if (thread) thread.innerHTML = "";
    const input = document.getElementById("chat-input");
    if (input) input.value = "";
    applyChatFeatureAvailability();
  });
}

test.describe("chat perspective", () => {
  test("chat view section exists and activates", async () => {
    await setupWithChat(true);

    await page.evaluate(() => activatePerspective("chat"));

    const result = await page.evaluate(() => {
      const view = document.getElementById("view-chat");
      return view ? view.classList.contains("active") : false;
    });

    expect(result).toBe(true);
  });

  test("chat thread and composer elements present", async () => {
    await setupWithChat(true);

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

  test("appendChatBubble adds bubble to thread", async () => {
    await setupWithChat(true);

    await page.evaluate(() => {
      activatePerspective("chat");
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

  test("multiple bubble roles render correctly", async () => {
    await setupWithChat(true);

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

  test("send button adds user bubble when text entered", async () => {
    await setupWithChat(true);

    await page.evaluate(() => {
      activatePerspective("chat");
      connectChatWS = () => {};
      chatSocket = null;
      document.getElementById("chat-input").value = "Test message";
      sendChatInput();
    });

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

  test("send with empty input does nothing", async () => {
    await setupWithChat(true);

    await page.evaluate(() => {
      activatePerspective("chat");
      connectChatWS = () => {};
      document.getElementById("chat-thread").innerHTML = "";
    });

    await page.evaluate(() => sendChatInput());

    const count = await page.evaluate(() => document.getElementById("chat-thread").querySelectorAll(".chat-bubble").length);
    expect(count).toBe(0);
  });
});

test.describe("chat disabled", () => {
  test("chat input disabled when chatEnabled is false", async () => {
    await setupWithChat(false);

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

  test("sendChatInput when disabled shows system message", async () => {
    await setupWithChat(false);

    await page.evaluate(() => {
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

  test("chat left panel item hidden when disabled", async () => {
    await setupWithChat(false);

    const hidden = await page.evaluate(() => {
      const item = document.querySelector('[data-left-panel-action="chat"]');
      return item ? item.classList.contains("hidden") : true;
    });

    expect(hidden).toBe(true);
  });

  test("chat perspective item hidden when disabled", async () => {
    await setupWithChat(false);

    const hidden = await page.evaluate(() => {
      const item = document.querySelector('[data-perspective="chat"]');
      return item ? item.classList.contains("hidden") : true;
    });

    expect(hidden).toBe(true);
  });
});

test.describe("chat settings", () => {
  test("settings panel shows chat controls", async () => {
    await setupWithChat(true);

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
