const { test, expect } = require("@playwright/test");

test("landing page exposes ticket-first UI controls", async ({ page }) => {
  const pageErrors = [];
  page.on("pageerror", (err) => {
    pageErrors.push(String(err && err.message ? err.message : err));
  });

  await page.goto("/");

  await expect(page.locator("#login-screen")).toBeVisible();
  await expect(page.locator("#login-form")).toBeVisible();
  await expect(page.locator("#login-pixel-logo")).toBeVisible();
  await expect(page.locator("#login-user")).toBeVisible();
  await expect(page.locator("#login-pass")).toBeVisible();
  await expect(page.getByRole("button", { name: "Login" })).toBeVisible();
  await expect(page.locator("#login-user")).toBeFocused();
  await expect(page.locator("#left-panel-handle")).toBeHidden();
  const ticketFormHeaders = await page.evaluate(() => {
    const ths = Array.from(document.querySelectorAll("#ticket-modal thead th"));
    return ths.map((th) => String(th.textContent || "").trim());
  });
  expect(ticketFormHeaders).toEqual(["Field", "Value"]);
  await expect(page.locator("#ticket-modal")).toContainText("Ticket Form");
  await expect(page.locator('[data-left-panel-action="swimlanes"]')).toHaveText("board");
  await expect(page.getByText("kanban")).toHaveCount(0);
  await expect(page.locator("#perspective-btn")).toHaveCount(0);
  await expect(page.getByText("ESC to close")).toHaveCount(0);
  await expect(page.locator("#settings-modal")).not.toContainText("ESC to close");
  const layout = await page.evaluate(() => {
    const mainContent = document.getElementById("main-content");
    const leftPanel = document.getElementById("left-panel");
    const appHead = document.querySelector(".app-head");
    if (!mainContent || !leftPanel || !appHead) return null;
    const mainStyle = window.getComputedStyle(mainContent);
    const panelStyle = window.getComputedStyle(leftPanel);
    const headStyle = window.getComputedStyle(appHead);
    return {
      mainOverflowY: mainStyle.overflowY,
      leftPanelOverflowY: panelStyle.overflowY,
      appHeadPosition: headStyle.position,
    };
  });
  expect(layout).not.toBeNull();
  expect(layout.mainOverflowY).toBe("auto");
  expect(layout.leftPanelOverflowY).toBe("auto");
  expect(layout.appHeadPosition).toBe("sticky");
  const selectorPersistence = await page.evaluate(() => {
    const leftPanel = document.getElementById("left-panel");
    const appShell = document.getElementById("app-shell");
    const backdrop = document.getElementById("left-panel-backdrop");
    if (!leftPanel || !appShell || !backdrop) return null;
    leftPanel.classList.add("open");
    appShell.classList.add("panel-open");
    backdrop.dispatchEvent(new MouseEvent("click", { bubbles: true }));
    const openAfterBackdropClick = leftPanel.classList.contains("open");
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
    const openAfterEscape = leftPanel.classList.contains("open");
    leftPanel.classList.remove("open");
    appShell.classList.remove("panel-open");
    return { openAfterBackdropClick, openAfterEscape };
  });
  expect(selectorPersistence).not.toBeNull();
  expect(selectorPersistence.openAfterBackdropClick).toBe(true);
  expect(selectorPersistence.openAfterEscape).toBe(true);
  const escapeDoesNotClosePanels = await page.evaluate(() => {
    const overlayIds = [
      "search-overlay",
      "perspective-overlay",
      "proj-modal-overlay",
      "story-modal-overlay",
      "agent-modal-overlay",
      "role-modal-overlay",
      "team-modal-overlay",
      "settings-modal-overlay",
      "modal-overlay",
      "dialog-overlay",
    ];
    const overlays = overlayIds
      .map((id) => document.getElementById(id))
      .filter(Boolean);
    overlays.forEach((node) => node.classList.remove("hidden"));
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
    const allStillVisible = overlays.every((node) => !node.classList.contains("hidden"));
    overlays.forEach((node) => node.classList.add("hidden"));
    return allStillVisible;
  });
  expect(escapeDoesNotClosePanels).toBe(true);
  const focusRetention = await page.evaluate(() => {
    if (typeof renderBoard !== "function" || typeof setFocusedCard !== "function") return null;
    tickets = [{
      ticket_id: 101,
      title: "Focus Me",
      key: "TK-101",
      type: "task",
      state: "idle",
      stage: "design",
      created_at: "2026-01-01T00:00:00Z",
      updated_at: "2026-01-01T00:00:00Z",
    }];
    renderBoard();
    const first = document.querySelector('article.ticket[data-ticket-id="101"]');
    if (!first) return null;
    setFocusedCard(first);
    const before = first.classList.contains("focused");
    renderBoard();
    const after = document.querySelector('article.ticket[data-ticket-id="101"]');
    return {
      before,
      afterFocused: Boolean(after && after.classList.contains("focused")),
    };
  });
  expect(focusRetention).not.toBeNull();
  expect(focusRetention.before).toBe(true);
  expect(focusRetention.afterFocused).toBe(true);
  expect(pageErrors, `unexpected page errors: ${pageErrors.join("\n")}`).toEqual([]);
});

test("authenticated app opens the channel selector by default", async ({ page }) => {
  await page.goto("/");

  const state = await page.evaluate(() => {
    localStorage.setItem("left-panel-open", "0");
    if (typeof showApp !== "function") return null;
    showApp("alice", "user");
    const leftPanel = document.getElementById("left-panel");
    const appShell = document.getElementById("app-shell");
    const handle = document.getElementById("left-panel-handle");
    const logo = document.getElementById("pixel-logo");
    if (!leftPanel || !appShell || !handle || !logo) return null;
    const rootStyle = window.getComputedStyle(document.documentElement);
    const selectorWidth = parseFloat(rootStyle.getPropertyValue("--left-panel-width")) || 0;
    const logoStyle = window.getComputedStyle(logo);
    return {
      leftPanelOpen: leftPanel.classList.contains("open"),
      shellOpen: appShell.classList.contains("panel-open"),
      handleLabel: String(handle.textContent || "").trim(),
      logoMaxWidth: parseFloat(logoStyle.maxWidth) || 0,
      selectorWidth,
    };
  });

  expect(state).not.toBeNull();
  expect(state.leftPanelOpen).toBe(true);
  expect(state.shellOpen).toBe(true);
  expect(state.handleLabel).toBe("minimise");
  expect(state.logoMaxWidth).toBe(state.selectorWidth);
});

test("project modal no longer exposes a git branch field", async ({ page }) => {
  await page.goto("/");

  const state = await page.evaluate(() => {
    if (typeof showApp !== "function" || typeof openProjModal !== "function") return null;
    showApp("alice", "user");
    openProjModal();
    return {
      hasProjectGitBranch: Boolean(document.getElementById("proj-modal-git-branch")),
      hasProjectGitRepository: Boolean(document.getElementById("proj-modal-git-repository")),
      hasProjectDefaultDraft: Boolean(document.getElementById("proj-modal-default-draft")),
    };
  });

  expect(state).not.toBeNull();
  expect(state.hasProjectGitBranch).toBe(false);
  expect(state.hasProjectGitRepository).toBe(true);
  expect(state.hasProjectDefaultDraft).toBe(true);
});

test("management panels support card mode with popup editing", async ({ page }) => {
  await page.goto("/");

  const cardCount = await page.evaluate(() => {
    if (typeof showApp !== "function") return -1;
    showApp("admin", "admin");
    agents = [{ agent_id: 7, name: "Atlas", description: "Build agent", enabled: true, status: "idle" }];
    roles = [{ role_id: 9, title: "Architect", motivation: "Shape systems", goals: "Reduce risk" }];
    teams = [{ team_id: 5, name: "Platform", parent_team_id: null }];
    setManagementMode("agents", "card");
    setManagementMode("roles", "card");
    setManagementMode("teams", "card");
    renderAgentSelector();
    renderRoleSelector();
    renderTeamSelector();
    activatePerspective("agents");
    renderAgentList();
    renderRoleList();
    renderTeamList();
    return document.querySelectorAll("#agent-list .management-card").length;
  });
  expect(cardCount).toBeGreaterThan(0);

  await page.locator("#agent-list .management-card").first().click({ force: true });
  await expect(page.locator("#agent-modal-overlay")).toBeVisible();
  await expect(page.locator("#agent-name")).toHaveValue("Atlas");

  await page.evaluate(() => { closeAgentModal(); activatePerspective("roles"); renderRoleList(); });
  await page.locator("#role-list .management-card").first().click({ force: true });
  await expect(page.locator("#role-modal-overlay")).toBeVisible();
  await expect(page.locator("#role-title")).toHaveValue("Architect");

  await page.evaluate(() => { closeRoleModal(); activatePerspective("teams"); renderTeamList(); });
  await page.locator("#team-list .management-card").first().click({ force: true });
  await expect(page.locator("#team-modal-overlay")).toBeVisible();
  await expect(page.locator("#team-name")).toHaveValue("Platform");
});

test("ticket modal scroll stays inside the popup", async ({ page }) => {
  await page.goto("/");

  const scrollState = await page.evaluate(() => {
    if (typeof showApp !== "function" || typeof openEdit !== "function") return null;
    showApp("alice", "user");
    const mainContent = document.getElementById("main-content");
    const modal = document.getElementById("ticket-modal");
    if (!mainContent || !modal) return null;
    const mainFiller = document.createElement("div");
    mainFiller.id = "main-scroll-filler";
    mainFiller.style.height = "2000px";
    mainContent.appendChild(mainFiller);
    mainContent.scrollTop = 180;
    openEdit({
      ticket_id: 101,
      key: "TK-101",
      type: "task",
      title: "Scroll Test",
      description: "",
      acceptance_criteria: "",
      git_repository: "",
      git_branch: "",
      stage: "design",
      state: "idle",
      open: true,
      archived: false,
    });
    const formTable = modal.querySelector(".form-table");
    if (!formTable) return null;
    const filler = document.createElement("div");
    filler.id = "modal-scroll-filler";
    filler.style.height = "1600px";
    filler.setAttribute("data-testid", "modal-scroll-filler");
    formTable.after(filler);
    const modalStyle = window.getComputedStyle(modal);
    const mainStyle = window.getComputedStyle(mainContent);
    modal.scrollTop = 320;
    const result = {
      mainOverflowY: mainStyle.overflowY,
      mainScrollTop: mainContent.scrollTop,
      modalOverflowY: modalStyle.overflowY,
      modalScrollTop: modal.scrollTop,
      modalScrollHeight: modal.scrollHeight,
      modalClientHeight: modal.clientHeight,
    };
    const fillerNode = document.getElementById("modal-scroll-filler");
    if (fillerNode) fillerNode.remove();
    const mainFillerNode = document.getElementById("main-scroll-filler");
    if (mainFillerNode) mainFillerNode.remove();
    return result;
  });

  expect(scrollState).not.toBeNull();
  expect(scrollState.mainOverflowY).toBe("hidden");
  expect(scrollState.mainScrollTop).toBe(180);
  expect(scrollState.modalOverflowY).toBe("auto");
  expect(scrollState.modalScrollHeight).toBeGreaterThan(scrollState.modalClientHeight);
  expect(scrollState.modalScrollTop).toBeGreaterThan(0);
});

test("ticket modal shows labels and time tracking sections for existing tickets", async ({ page }) => {
  await page.goto("/");

  const result = await page.evaluate(() => {
    if (typeof showApp !== "function" || typeof openEdit !== "function") return null;
    showApp("alice", "user");
    openEdit({
      ticket_id: 42,
      key: "TK-42",
      type: "task",
      title: "Labels and Time Test",
      description: "",
      acceptance_criteria: "",
      git_repository: "",
      git_branch: "",
      stage: "design",
      state: "idle",
      open: true,
      archived: false,
    });
    const labelsSection = document.getElementById("ticket-labels-section");
    const timeSection = document.getElementById("ticket-time-section");
    const labelSelect = document.getElementById("ticket-label-select");
    const labelAddBtn = document.getElementById("ticket-label-add");
    const timeMinutes = document.getElementById("ticket-time-minutes");
    const timeNote = document.getElementById("ticket-time-note");
    const timeLogBtn = document.getElementById("ticket-time-log");
    if (!labelsSection || !timeSection) return null;
    return {
      labelsVisible: labelsSection.style.display !== "none",
      timeVisible: timeSection.style.display !== "none",
      hasLabelSelect: !!labelSelect,
      hasLabelAddBtn: !!labelAddBtn,
      labelAddText: labelAddBtn ? labelAddBtn.textContent : "",
      hasTimeMinutes: !!timeMinutes,
      hasTimeNote: !!timeNote,
      hasTimeLogBtn: !!timeLogBtn,
      timeLogText: timeLogBtn ? timeLogBtn.textContent : "",
    };
  });

  expect(result).not.toBeNull();
  expect(result.labelsVisible).toBe(true);
  expect(result.timeVisible).toBe(true);
  expect(result.hasLabelSelect).toBe(true);
  expect(result.hasLabelAddBtn).toBe(true);
  expect(result.labelAddText).toBe("+ Label");
  expect(result.hasTimeMinutes).toBe(true);
  expect(result.hasTimeNote).toBe(true);
  expect(result.hasTimeLogBtn).toBe(true);
  expect(result.timeLogText).toBe("+ Time");
});

test("new ticket modal hides labels and time sections", async ({ page }) => {
  await page.goto("/");

  const result = await page.evaluate(() => {
    if (typeof showApp !== "function" || typeof openNew !== "function") return null;
    showApp("alice", "user");
    // Need a project selected
    projects = [{ project_id: 1, title: "Test", prefix: "TK", status: "open", default_draft: true }];
    window.call = async (url) => {
      if (url === "/api/sdlcs") {
        return [{ sdlc_id: 8, name: "Delivery Flow" }];
      }
      return [];
    };
    localStorage.setItem("task-project", "1");
    openNew();
    const labelsSection = document.getElementById("ticket-labels-section");
    const timeSection = document.getElementById("ticket-time-section");
    const draftField = document.getElementById("ticket-draft");
    const sdlcField = document.getElementById("ticket-sdlc");
    return {
      labelsHidden: labelsSection ? labelsSection.style.display === "none" : null,
      timeHidden: timeSection ? timeSection.style.display === "none" : null,
      hasDraftField: Boolean(draftField),
      draftValue: draftField ? draftField.value : null,
      hasSdlcField: Boolean(sdlcField),
    };
  });

  expect(result).not.toBeNull();
  expect(result.labelsHidden).toBe(true);
  expect(result.timeHidden).toBe(true);
  expect(result.hasDraftField).toBe(true);
  expect(result.draftValue).toBe("true");
  expect(result.hasSdlcField).toBe(true);
});

test("board lanes expose quick new-ticket actions", async ({ page }) => {
  await page.goto("/");

  const result = await page.evaluate(() => {
    if (typeof showApp !== "function" || typeof renderBoard !== "function") return null;
    showApp("alice", "user");
    projects = [{ project_id: 1, title: "Test", prefix: "TK", status: "open", default_draft: false }];
    tickets = [];
    localStorage.setItem("task-project", "1");
    window.call = async (url) => {
      if (url === "/api/sdlcs") return [];
      return [];
    };
    renderBoard();
    const laneButtons = Array.from(document.querySelectorAll("[data-lane-new]"));
    laneButtons[1]?.click();
    const overlay = document.getElementById("modal-overlay");
    return {
      laneCount: laneButtons.length,
      modalVisible: overlay ? !overlay.classList.contains("hidden") : false,
      selectedStage: document.getElementById("ticket-stage").value,
    };
  });

  expect(result).not.toBeNull();
  expect(result.laneCount).toBe(4);
  expect(result.modalVisible).toBe(true);
  expect(result.selectedStage).toBe("develop");
});

test("sdlc editor renders draggable stage cards with inline role controls", async ({ page }) => {
  await page.goto("/");

  const result = await page.evaluate(async () => {
    if (typeof showApp !== "function" || typeof openSdlcEditor !== "function") return null;
    showApp("admin", "admin");
    window.call = async (url) => {
      if (url === "/api/roles") {
        return [{ role_id: 2, title: "Engineer" }, { role_id: 3, title: "QA" }];
      }
      if (url === "/api/sdlcs/9") {
        return {
          sdlc_id: 9,
          name: "Delivery",
          stages: [{
            sdlc_stage_id: 41,
            sdlc_id: 9,
            stage_name: "develop",
            description: "Build the thing",
            definition_of_ready: "Specs ready",
            definition_of_done: "Tests green",
            sort_order: 1,
            roles: [{ role_id: 2, title: "Engineer" }, { role_id: 3, title: "QA" }],
          }],
        };
      }
      return [];
    };
    await openSdlcEditor({ sdlc_id: 9, name: "Delivery", description: "Ship changes" });
    const card = document.querySelector(".sdlc-stage-card");
    const roleChip = card ? card.querySelector(".sdlc-role-chip") : null;
    return {
      hasCard: Boolean(card),
      draggable: card ? card.draggable : false,
      hasSaveButton: Boolean(card && card.querySelector('[data-stage-action="save"]')),
      hasRoleChip: Boolean(card && card.querySelector(".sdlc-role-chip")),
      roleChipDraggable: roleChip ? roleChip.draggable : false,
      hasRoleSelect: Boolean(card && card.querySelector("[data-stage-role-select]")),
      hasDorField: Boolean(card && card.querySelector('[data-stage-field="dor"]')),
    };
  });

  expect(result).not.toBeNull();
  expect(result.hasCard).toBe(true);
  expect(result.draggable).toBe(true);
  expect(result.hasSaveButton).toBe(true);
  expect(result.hasRoleChip).toBe(true);
  expect(result.roleChipDraggable).toBe(true);
  expect(result.hasRoleSelect).toBe(true);
  expect(result.hasDorField).toBe(true);
});

test("sdlc role reordering sends the updated role order", async ({ page }) => {
  await page.goto("/");

  const result = await page.evaluate(async () => {
    if (typeof showApp !== "function" || typeof openSdlcEditor !== "function" || typeof reorderStageRoles !== "function") return null;
    showApp("admin", "admin");
    const requests = [];
    const detail = {
      sdlc_id: 9,
      name: "Delivery",
      stages: [{
        sdlc_stage_id: 41,
        sdlc_id: 9,
        stage_name: "develop",
        description: "Build the thing",
        definition_of_ready: "Specs ready",
        definition_of_done: "Tests green",
        sort_order: 1,
        roles: [{ role_id: 2, title: "Engineer" }, { role_id: 3, title: "QA" }],
      }],
    };
    window.call = async (url, options = {}) => {
      requests.push({ url, method: options.method || "GET", body: options.body || null });
      if (url === "/api/roles") {
        return [{ role_id: 2, title: "Engineer" }, { role_id: 3, title: "QA" }];
      }
      if (url === "/api/sdlcs/9") {
        return detail;
      }
      return { status: "ok" };
    };
    await openSdlcEditor({ sdlc_id: 9, name: "Delivery", description: "Ship changes" });
    await reorderStageRoles(detail.stages[0], 3, 2);
    const reorderRequest = requests.find((req) => req.url === "/api/sdlcs/stages/roles/9/41" && req.method === "PUT");
    return reorderRequest ? JSON.parse(reorderRequest.body) : null;
  });

  expect(result).not.toBeNull();
  expect(result.role_ids).toEqual([3, 2]);
});

test("sdlc editor keyboard shortcuts focus the new stage input and selected stage field", async ({ page }) => {
  await page.goto("/");

  const result = await page.evaluate(async () => {
    if (typeof showApp !== "function" || typeof openSdlcEditor !== "function") return null;
    showApp("admin", "admin");
    window.call = async (url) => {
      if (url === "/api/roles") {
        return [{ role_id: 2, title: "Engineer" }];
      }
      if (url === "/api/sdlcs/9") {
        return {
          sdlc_id: 9,
          name: "Delivery",
          stages: [{
            sdlc_stage_id: 41,
            sdlc_id: 9,
            stage_name: "develop",
            description: "Build the thing",
            definition_of_ready: "Specs ready",
            definition_of_done: "Tests green",
            sort_order: 1,
            roles: [{ role_id: 2, title: "Engineer" }],
          }],
        };
      }
      return [];
    };
    await openSdlcEditor({ sdlc_id: 9, name: "Delivery", description: "Ship changes" });
    document.body.focus();
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "n", bubbles: true }));
    const newStageFocused = document.activeElement && document.activeElement.id === "sdlc-stage-name";
    const selectedCardEl = document.querySelector(".sdlc-stage-card");
    if (selectedCardEl) selectedCardEl.focus();
    window.dispatchEvent(new KeyboardEvent("keydown", { key: "e", bubbles: true }));
    const selectedField = document.activeElement;
    const selectedCard = document.querySelector(".sdlc-stage-card.selected");
    return {
      newStageFocused,
      selectedFieldName: selectedField ? selectedField.getAttribute("data-stage-field") : null,
      selectedStageId: selectedCard ? selectedCard.dataset.stageId : null,
    };
  });

  expect(result).not.toBeNull();
  expect(result.newStageFocused).toBe(true);
  expect(result.selectedFieldName).toBe("name");
  expect(result.selectedStageId).toBe("41");
});

test("websocket event compatibility keeps board refresh for legacy and normalized payloads", async ({ page }) => {
  await page.goto("/");

  const result = await page.evaluate(() => {
    const cases = [
      { msg: { type: "ticket_created" }, wantType: "ticket_created", wantRefresh: true },
      { msg: { type: "ticket_updated" }, wantType: "ticket_updated", wantRefresh: true },
      { msg: { type: "ticket_deleted" }, wantType: "ticket_deleted", wantRefresh: true },
      { msg: { entity_type: "ticket", change_type: "updated" }, wantType: "ticket_updated", wantRefresh: true },
      { msg: { entity_type: "ticket", change_type: "changed" }, wantType: "ticket_changed", wantRefresh: true },
      { msg: { entity_type: "project", change_type: "users_updated" }, wantType: "project_users_updated", wantRefresh: true },
      { msg: { entity_type: "project", change_type: "changed" }, wantType: "project_changed", wantRefresh: true },
      { msg: { entity_type: "project", change_type: "created" }, wantType: "project_created", wantRefresh: false },
      { msg: { type: "connected" }, wantType: "connected", wantRefresh: false },
      { msg: { type: "unknown_event" }, wantType: "unknown_event", wantRefresh: false },
    ];
      return cases.map((testCase) => {
        const eventType = window.__wsEventType(testCase.msg);
        const refresh = window.__wsShouldRefreshBoard(testCase.msg, eventType);
      return {
        eventType,
        refresh,
        wantType: testCase.wantType,
        wantRefresh: testCase.wantRefresh,
      };
    });
  });

  for (const item of result) {
    expect(item.eventType).toBe(item.wantType);
    expect(item.refresh).toBe(item.wantRefresh);
  }
});
