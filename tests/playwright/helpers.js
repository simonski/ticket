const DEMO_PROJECT = { project_id: 1, title: "Demo", prefix: "DM", status: "open" };

const DEFAULT_STATUS = {
  authenticated: false,
  server_version: "1.0.0",
  registration_enabled: false,
  registration_auto_approve: false,
  chat_enabled: false,
};

const OVERLAY_IDS = [
  "search-overlay",
  "perspective-overlay",
  "proj-modal-overlay",
  "story-modal-overlay",
  "agent-modal-overlay",
  "role-modal-overlay",
  "team-modal-overlay",
  "workflow-modal-overlay",
  "settings-modal-overlay",
  "modal-overlay",
  "dialog-overlay",
];

const patternCache = new Map();

function globToRegExp(pattern) {
  if (patternCache.has(pattern)) {
    return patternCache.get(pattern);
  }
  const escaped = pattern
    .replace(/[.+?^${}()|[\]\\]/g, "\\$&")
    .replace(/\*\*/g, "___DOUBLE_WILDCARD___")
    .replace(/\*/g, "[^/]*")
    .replace(/___DOUBLE_WILDCARD___/g, ".*");
  const regex = new RegExp(`^${escaped}$`);
  patternCache.set(pattern, regex);
  return regex;
}

function matches(url, pattern) {
  return globToRegExp(pattern).test(url);
}

function jsonResponse(body, status = 200) {
  return {
    status,
    contentType: "application/json",
    body: JSON.stringify(body),
  };
}

async function createMockAPI(page, initialRoutes = []) {
  let indexedSuffixRoutes = new Map();
  let exactRoutes = new Map();
  let wildcardRoutes = [];
  const indexRoutes = (nextRoutes = []) => {
    indexedSuffixRoutes = new Map();
    exactRoutes = new Map();
    wildcardRoutes = [];
    nextRoutes.forEach(([pattern, handler]) => {
      if (pattern.startsWith("**") && !pattern.slice(2).includes("*")) {
        indexedSuffixRoutes.set(pattern.slice(2), handler);
        return;
      }
      if (!pattern.includes("*")) {
        exactRoutes.set(pattern, handler);
        return;
      }
      wildcardRoutes.push([globToRegExp(pattern), handler]);
    });
  };
  indexRoutes(initialRoutes);
  await page.route("**/api/**", async (route) => {
    const url = route.request().url();
    if (url.includes("/api/board/ws")) {
      await route.abort();
      return;
    }
    let handler = exactRoutes.get(url);
    if (!handler) {
      try {
        const pathname = new URL(url).pathname;
        handler = indexedSuffixRoutes.get(pathname);
      } catch {
        handler = null;
      }
    }
    if (!handler) {
      const match = wildcardRoutes.find(([regex]) => regex.test(url));
      if (match) {
        handler = match[1];
      }
    }
    if (!handler) {
      await route.fulfill(jsonResponse({ error: "not mocked" }, 404));
      return;
    }
    if (typeof handler === "function") {
      await handler(route);
      return;
    }
    await route.fulfill(jsonResponse(handler));
  });
  return {
    setRoutes(nextRoutes = []) {
      indexRoutes(nextRoutes);
    },
  };
}

function withDefaultStatus(routes = [], status = DEFAULT_STATUS) {
  return [["**/api/status", status], ...routes];
}

async function gotoRoot(page, api, routes = []) {
  api.setRoutes(withDefaultStatus(routes));
  await page.goto("/");
  await page.waitForFunction(() => typeof showApp === "function" && typeof renderBoard === "function");
}

async function resetApp(page, state = {}) {
  const project = state.project || DEMO_PROJECT;
  const projectsState = state.projects || (project ? [project] : []);
  await page.waitForFunction(() => typeof showApp === "function" && typeof renderBoard === "function");
  await page.evaluate(
    ({ project, projectsState, state, overlayIds }) => {
      localStorage.clear();
      sessionStorage.clear();
      if (typeof window.__testOriginalCall !== "function" && typeof call === "function") {
        window.__testOriginalCall = call;
      }
      if (typeof window.__testOriginalUIConfirm !== "function" && typeof uiConfirm === "function") {
        window.__testOriginalUIConfirm = uiConfirm;
      }
      if (typeof window.__testOriginalCall === "function") {
        window.call = window.__testOriginalCall;
      }
      if (typeof window.__testOriginalUIConfirm === "function") {
        window.uiConfirm = window.__testOriginalUIConfirm;
      }

      showApp(state.username || "admin", state.role || "admin");
      token = state.token || "test-token";
      registrationEnabled = state.registrationEnabled ?? false;
      registrationAutoApprove = state.registrationAutoApprove ?? false;
      chatEnabled = state.chatEnabled ?? false;
      chatMaxConnections = state.chatMaxConnections ?? 0;
      chatMaxDurationMinutes = state.chatMaxDurationMinutes ?? 0;

      projects = projectsState;
      tickets = state.tickets || [];
      agents = state.agents || [];
      roles = state.roles || [];
      teams = state.teams || [];
      workflows = state.workflows || [];
      stories = state.stories || [];
      labels = state.labels || [];
      if (typeof selectedAgentID !== "undefined") selectedAgentID = 0;
      if (typeof selectedRoleID !== "undefined") selectedRoleID = 0;
      if (typeof selectedTeamID !== "undefined") selectedTeamID = 0;
      if (typeof selectedWorkflowID !== "undefined") selectedWorkflowID = 0;
      if (typeof selectedWorkflowDetail !== "undefined") selectedWorkflowDetail = null;
      if (typeof selectedWorkflowStageID !== "undefined") selectedWorkflowStageID = 0;
      if (typeof selectedWorkflowRoleID !== "undefined") selectedWorkflowRoleID = 0;

      if (typeof renderProjMenu === "function") renderProjMenu();
      const projectSelect = document.getElementById("project-select");
      if (projectSelect && project) {
        projectSelect.value = String(project.project_id);
      }
      if (project) {
        localStorage.setItem("task-project", String(project.project_id));
      }

      overlayIds.forEach((id) => {
        const node = document.getElementById(id);
        if (node) node.classList.add("hidden");
      });

      if (typeof closeProfileMenu === "function") closeProfileMenu();
      if (typeof renderBoard === "function") renderBoard();
      if (state.managementModes && typeof setManagementMode === "function") {
        Object.entries(state.managementModes).forEach(([name, mode]) => setManagementMode(name, mode));
      }
      if (state.perspective && typeof activatePerspective === "function") {
        activatePerspective(state.perspective);
      }
    },
    { project, projectsState, state, overlayIds: OVERLAY_IDS }
  );
}

async function resetLogin(page, state = {}) {
  const status = state.status || DEFAULT_STATUS;
  await page.waitForFunction(() => typeof showLogin === "function" && typeof canRegister === "function");
  await page.evaluate(
    ({ status, overlayIds, state }) => {
      localStorage.clear();
      sessionStorage.clear();
      if (typeof window.__testOriginalCall !== "function" && typeof call === "function") {
        window.__testOriginalCall = call;
      }
      if (typeof window.__testOriginalUIConfirm !== "function" && typeof uiConfirm === "function") {
        window.__testOriginalUIConfirm = uiConfirm;
      }
      if (typeof window.__testOriginalCall === "function") {
        window.call = window.__testOriginalCall;
      }
      if (typeof window.__testOriginalUIConfirm === "function") {
        window.uiConfirm = window.__testOriginalUIConfirm;
      }
      token = "";
      currentUsername = "";
      currentUserRole = "";
      registrationEnabled = status.registrationEnabled ?? status.registration_enabled ?? false;
      registrationAutoApprove = status.registrationAutoApprove ?? status.registration_auto_approve ?? false;
      chatEnabled = status.chatEnabled ?? status.chat_enabled ?? false;
      chatMaxConnections = status.chatMaxConnections ?? status.chat_max_connections ?? 0;
      chatMaxDurationMinutes = status.chatMaxDurationMinutes ?? status.chat_max_duration_minutes ?? 0;
      chatRunningProcesses = 0;
      projects = [];
      tickets = [];
      agents = [];
      roles = [];
      teams = [];
      workflows = [];
      stories = [];
      ticketLabelsMap = {};
      projectLabels = [];
      if (typeof selectedAgentID !== "undefined") selectedAgentID = 0;
      if (typeof selectedRoleID !== "undefined") selectedRoleID = 0;
      if (typeof selectedTeamID !== "undefined") selectedTeamID = 0;
      if (typeof selectedWorkflowID !== "undefined") selectedWorkflowID = 0;
      if (typeof selectedWorkflowDetail !== "undefined") selectedWorkflowDetail = null;
      if (typeof selectedWorkflowStageID !== "undefined") selectedWorkflowStageID = 0;
      if (typeof selectedWorkflowRoleID !== "undefined") selectedWorkflowRoleID = 0;
      backlogWorkflowCatalog = [];
      if (typeof backlogWorkflowDetails?.clear === "function") backlogWorkflowDetails.clear();
      backlogWorkflowLoadPromise = null;
      if (typeof canRegister === "function") canRegister(Boolean(registrationEnabled));
      if (typeof applyChatFeatureAvailability === "function") applyChatFeatureAvailability();
      overlayIds.forEach((id) => {
        const node = document.getElementById(id);
        if (node) node.classList.add("hidden");
      });
      const loginForm = document.getElementById("login-form");
      if (loginForm) {
        loginForm.classList.remove("hidden");
        loginForm.reset();
      }
      const registerForm = document.getElementById("register-form");
      if (registerForm) {
        registerForm.classList.add("hidden");
        registerForm.reset();
      }
      const loginStatus = document.getElementById("login-status");
      if (loginStatus) loginStatus.textContent = "";
      showLogin(state.message || "");
    },
    { status, overlayIds: OVERLAY_IDS, state }
  );
}

module.exports = {
  DEMO_PROJECT,
  DEFAULT_STATUS,
  createMockAPI,
  gotoRoot,
  jsonResponse,
  resetLogin,
  resetApp,
  withDefaultStatus,
};
