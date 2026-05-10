        const apiClient = window.TicketAPI.createClient();
        const api = apiClient.request;
        const apiWithFallback = apiClient.requestWithFallback;
        const state = {
            auth: null,
            currentView: "tickets",
            viewScrollByPanel: {},
            scrollPersistenceReady: false,
            status: null,
            projects: [],
            goals: [],
            documents: [],
            tickets: [],
            interventions: [],
            interventionReport: null,
            interventionTrends: [],
            interventionDrilldown: null,
            projectForecast: [],
            forecastCalibration: null,
            forecastBacktest: null,
            workflowValidation: {},
            workflows: [],
            roles: [],
            agents: [],
            teams: [],
            selectedProjectID: null,
            selectedProjectDraft: emptyProject(),
            selectedGoalID: null,
            selectedDocumentID: null,
            selectedGoalDraft: {
                id: null,
                project_id: 0,
                title: "",
                description: "",
                notes: "",
                eta: "",
                priority: 1,
                status: "draft",
                refined_goal: "",
                decomposition: "",
            },
            selectedDocumentDraft: {
                id: null,
                project_id: 0,
                title: "",
                description: "",
                notes: "",
                content: "",
            },
            selectedWorkflowID: null,
            selectedWorkflowDraft: emptyWorkflow(),
            selectedRoleID: null,
            selectedRoleDraft: emptyRole(),
            selectedAgentID: null,
            selectedTeamID: null,
            selectedTeamDraft: emptyTeam(),
            activeTicket: null,
            ticketHistory: [],
            ticketComments: [],
            ticketLabels: [],
            projectLabels: [],
            ticketDependencies: [],
            ticketTimeEntries: [],
            ticketTimeTotal: 0,
            interventionWorkItems: {},
            interventionHistory: {},
            interventionComments: {},
            interventionStates: {},
            dependencyIndex: {},
            drag: null,
            liveSocket: null,
            liveRefreshTimer: null,
            goalChatSocket: null,
            goalChatMessages: [],
            goalChatLoadedFor: null,
            goalDecompositionItems: [],
            documentFiles: [],
            goalInboxStatusFilter: "",
            goalInboxSort: "updated_desc",
        };

        const TICKET_TYPES = ["epic", "task", "bug", "spike", "chore", "story", "note", "question", "requirement", "decision"];
        const FALLBACK_STAGES = ["backlog", "todo", "doing", "done"];
        const AUTH_STORAGE_KEY = "site2.auth";
        const SELECTED_PROJECT_STORAGE_KEY = "site2.selectedProjectID";
        const SELECTED_VIEW_STORAGE_KEY = "site2.selectedView";
        const VIEW_SCROLL_STORAGE_KEY = "site2.viewScroll";

        const els = {
            loginScreen: document.getElementById("login-screen"),
            loginForm: document.getElementById("login-form"),
            loginError: document.getElementById("login-error"),
            appShell: document.getElementById("app-shell"),
            appNotice: document.getElementById("app-notice"),
            projectMenuButton: document.getElementById("project-menu-button"),
            projectMenuDropdown: document.getElementById("project-menu-dropdown"),
            projectMenuList: document.getElementById("project-menu-list"),
            projectCreateLink: document.getElementById("project-create-link"),
            mainNav: document.getElementById("main-nav"),
            accountMenuButton: document.getElementById("account-menu-button"),
            accountMenuDropdown: document.getElementById("account-menu-dropdown"),
            accountMenuName: document.getElementById("account-menu-name"),
            projectList: document.getElementById("project-list"),
            goalList: document.getElementById("goal-list"),
            documentList: document.getElementById("document-list"),
            workflowList: document.getElementById("workflow-list"),
            roleList: document.getElementById("role-list"),
            agentList: document.getElementById("agent-list"),
            teamList: document.getElementById("team-list"),
            ticketBoard: document.getElementById("ticket-board"),
            boardSearch: document.getElementById("board-search"),
            boardHideDone: document.getElementById("board-hide-done"),
            interventionList: document.getElementById("intervention-list"),
            interventionFilter: document.getElementById("intervention-filter"),
            interventionSort: document.getElementById("intervention-sort"),
            interventionTrendsSummary: document.getElementById("intervention-trends-summary"),
            interventionReportSummary: document.getElementById("intervention-report-summary"),
            predictedWorkList: document.getElementById("predicted-work-list"),
            forecastCalibrationSummary: document.getElementById("forecast-calibration-summary"),
            stageGrid: document.getElementById("stage-grid"),
            workflowRoleBank: document.getElementById("workflow-role-bank"),
            workflowValidationSummary: document.getElementById("workflow-validation-summary"),
            ticketModal: document.getElementById("ticket-modal"),
            ticketHistory: document.getElementById("ticket-history"),
            ticketComments: document.getElementById("ticket-comments"),
            ticketCommentInput: document.getElementById("ticket-comment-input"),
            addTicketCommentButton: document.getElementById("add-ticket-comment-button"),
            ticketLabels: document.getElementById("ticket-labels"),
            ticketLabelSelect: document.getElementById("ticket-label-select"),
            addTicketLabelButton: document.getElementById("add-ticket-label-button"),
            ticketDependencies: document.getElementById("ticket-dependencies"),
            ticketDependencyInput: document.getElementById("ticket-dependency-input"),
            addTicketDependencyButton: document.getElementById("add-ticket-dependency-button"),
            ticketTimeEntries: document.getElementById("ticket-time-entries"),
            ticketTimeTotal: document.getElementById("ticket-time-total"),
            ticketTimeMinutes: document.getElementById("ticket-time-minutes"),
            ticketTimeNote: document.getElementById("ticket-time-note"),
            addTicketTimeButton: document.getElementById("add-ticket-time-button"),
            goalChatLog: document.getElementById("goal-chat-log"),
            goalChatInput: document.getElementById("goal-chat-input"),
            goalRefinedGoal: document.getElementById("goal-refined-goal"),
            goalDecomposition: document.getElementById("goal-decomposition"),
            goalDecompositionList: document.getElementById("goal-decomposition-list"),
            goalDecompositionItemInput: document.getElementById("goal-decomposition-item-input"),
            goalInboxStatusFilter: document.getElementById("goal-inbox-status-filter"),
            goalInboxSort: document.getElementById("goal-inbox-sort"),
            documentFilesList: document.getElementById("document-files-list"),
            documentUploadFile: document.getElementById("document-upload-file"),
            documentUploadName: document.getElementById("document-upload-name"),
        };

        function emptyProject() {
            return {
                id: null,
                prefix: "",
                title: "",
                description: "",
                acceptance_criteria: "",
                git_repository: "",
                visibility: "public",
                workflow_id: null,
                default_draft: false,
            };
        }

        function emptyWorkflow() {
            return { id: null, name: "", description: "", approval_policy: "single_role", progression_mode: "linear" };
        }

        function emptyGoal(projectID) {
            return {
                id: null,
                project_id: Number(projectID || 0),
                title: "",
                description: "",
                notes: "",
                eta: "",
                priority: 1,
                status: "draft",
                refined_goal: "",
                decomposition: "",
            };
        }

        function emptyDocument(projectID) {
            return {
                id: null,
                project_id: Number(projectID || 0),
                title: "",
                description: "",
                notes: "",
                content: "",
            };
        }

        function emptyRole() {
            return { id: null, title: "", description: "", acceptance_criteria: "", workflow_id: null };
        }

        function emptyTeam() {
            return { id: null, name: "", parent_team_id: null };
        }

        function emptyTicket(projectID) {
            return {
                id: "",
                key: "",
                project_id: projectID || state.selectedProjectID || 0,
                type: "task",
                title: "",
                description: "",
                acceptance_criteria: "",
                parent_id: null,
                status: "open",
                stage: getStageOptions()[0],
                priority: 0,
                order: 0,
                estimate_effort: 0,
                health: 0,
                draft: false,
                archived: false,
                workflow_id: null,
            };
        }

        function setNotice(message, isError) {
            if (!isError) {
                els.appNotice.textContent = "";
                els.appNotice.classList.remove("visible", "error");
                return;
            }
            els.appNotice.textContent = message;
            els.appNotice.classList.add("visible");
            els.appNotice.classList.add("error");
        }

        function toNullableNumber(value) {
            return value === null || value === undefined || value === "" ? null : Number(value);
        }

        function normalizeRole(role) {
            return Object.assign({}, role, {
                id: role.id !== undefined ? role.id : role.role_id,
                workflow_id: toNullableNumber(role.workflow_id),
            });
        }

        function normalizeStage(stage) {
            return Object.assign({}, stage, {
                id: stage.id !== undefined ? stage.id : stage.workflow_stage_id,
                workflow_id: Number(stage.workflow_id),
                name: stage.name || stage.stage_name,
                wow: stage.wow !== undefined ? stage.wow : (stage.description || ""),
                dor: stage.dor !== undefined ? stage.dor : (stage.definition_of_ready || ""),
                dod: stage.dod !== undefined ? stage.dod : (stage.definition_of_done || ""),
                roles: Array.isArray(stage.roles) ? stage.roles.map(normalizeRole) : [],
                next_stage_ids: Array.isArray(stage.next_stage_ids) ? stage.next_stage_ids.map((id) => Number(id)).filter((id) => !Number.isNaN(id)) : [],
            });
        }

        function normalizeWorkflow(workflow) {
            return Object.assign({}, workflow, {
                id: workflow.id !== undefined ? workflow.id : workflow.workflow_id,
                approval_policy: workflow.approval_policy || "single_role",
                progression_mode: workflow.progression_mode || "linear",
                stages: Array.isArray(workflow.stages) ? workflow.stages.map(normalizeStage) : [],
            });
        }

        function normalizeProject(project) {
            return Object.assign({}, project, {
                id: project.id !== undefined ? project.id : project.project_id,
                workflow_id: toNullableNumber(project.workflow_id),
            });
        }

        function normalizeGoal(goal) {
            return Object.assign({}, goal, {
                id: goal.id !== undefined ? goal.id : goal.goal_id,
                project_id: Number(goal.project_id || 0),
                priority: Number(goal.priority || 1),
                status: goal.status || "draft",
                refined_goal: goal.refined_goal || "",
                decomposition: goal.decomposition || "",
            });
        }

        function normalizeDocument(documentItem) {
            return Object.assign({}, documentItem, {
                id: documentItem.id !== undefined ? documentItem.id : documentItem.document_id,
                project_id: Number(documentItem.project_id || 0),
                title: documentItem.title || "",
                description: documentItem.description || "",
                notes: documentItem.notes || "",
                content: documentItem.content || "",
            });
        }

        function normalizeDocumentFile(file) {
            return Object.assign({}, file, {
                id: file.id !== undefined ? file.id : file.file_id,
                document_id: Number(file.document_id || 0),
                file_name: file.file_name || "",
                content_type: file.content_type || "",
                size_bytes: Number(file.size_bytes || 0),
            });
        }

        function parseGoalDecompositionItems(text) {
            if (!text) {
                return [];
            }
            return String(text)
                .split(/\r?\n/)
                .map((line) => line.trim())
                .filter(Boolean)
                .map((line) => line.replace(/^[-*]\s+/, "").replace(/^\d+[\.\)\-\s]+/, "").trim())
                .filter(Boolean);
        }

        function formatGoalDecompositionItems(items) {
            if (!Array.isArray(items) || !items.length) {
                return "";
            }
            return items.map((item, index) => (String(index + 1) + ". " + String(item).trim())).join("\n");
        }

        function normalizeTicket(ticket) {
            return Object.assign({}, ticket, {
                id: ticket.id !== undefined ? ticket.id : ticket.ticket_id,
                project_id: Number(ticket.project_id),
                workflow_id: toNullableNumber(ticket.workflow_id),
                workflow_stage_id: toNullableNumber(ticket.workflow_stage_id),
                role_id: toNullableNumber(ticket.role_id),
                health: ticket.health !== undefined ? ticket.health : (ticket.health_score || 0),
            });
        }

        function normalizeTeam(team) {
            return Object.assign({}, team, {
                id: team.id !== undefined ? team.id : team.team_id,
                parent_team_id: toNullableNumber(team.parent_team_id),
            });
        }

        function normalizeAgent(agent) {
            return Object.assign({}, agent, {
                id: agent.id || agent.user_id,
            });
        }

        function storeAuth(auth) {
            sessionStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify(auth));
            apiClient.setToken(auth && auth.token ? auth.token : "");
        }

        function clearStoredAuth() {
            sessionStorage.removeItem(AUTH_STORAGE_KEY);
            apiClient.setToken("");
        }

        function storeSelectedProjectID(projectID) {
            const parsed = Number(projectID);
            if (!Number.isFinite(parsed) || parsed <= 0) {
                localStorage.removeItem(SELECTED_PROJECT_STORAGE_KEY);
                return;
            }
            localStorage.setItem(SELECTED_PROJECT_STORAGE_KEY, String(parsed));
        }

        function loadStoredSelectedProjectID() {
            const raw = localStorage.getItem(SELECTED_PROJECT_STORAGE_KEY);
            if (!raw) {
                return null;
            }
            const parsed = Number(raw);
            if (!Number.isFinite(parsed) || parsed <= 0) {
                localStorage.removeItem(SELECTED_PROJECT_STORAGE_KEY);
                return null;
            }
            return parsed;
        }

        function availableViewNames() {
            return Array.from(document.querySelectorAll(".nav button[data-view]"))
                .map((button) => String(button.dataset.view || "").trim())
                .filter(Boolean);
        }

        function isKnownView(viewName) {
            return availableViewNames().includes(String(viewName || "").trim());
        }

        function storeSelectedView(viewName) {
            if (!isKnownView(viewName)) {
                localStorage.removeItem(SELECTED_VIEW_STORAGE_KEY);
                return;
            }
            localStorage.setItem(SELECTED_VIEW_STORAGE_KEY, String(viewName));
        }

        function loadStoredSelectedView() {
            const raw = localStorage.getItem(SELECTED_VIEW_STORAGE_KEY);
            if (!raw) {
                return null;
            }
            if (!isKnownView(raw)) {
                localStorage.removeItem(SELECTED_VIEW_STORAGE_KEY);
                return null;
            }
            return raw;
        }

        function loadStoredViewScrollByPanel() {
            const raw = localStorage.getItem(VIEW_SCROLL_STORAGE_KEY);
            if (!raw) {
                return {};
            }
            try {
                const parsed = JSON.parse(raw);
                const cleaned = {};
                const known = new Set(availableViewNames());
                Object.keys(parsed || {}).forEach((key) => {
                    const value = Number(parsed[key]);
                    if (!known.has(key) || !Number.isFinite(value) || value < 0) {
                        return;
                    }
                    cleaned[key] = value;
                });
                return cleaned;
            } catch (error) {
                localStorage.removeItem(VIEW_SCROLL_STORAGE_KEY);
                return {};
            }
        }

        function storeViewScrollByPanel() {
            localStorage.setItem(VIEW_SCROLL_STORAGE_KEY, JSON.stringify(state.viewScrollByPanel || {}));
        }

        function storeCurrentViewScroll() {
            if (!state.scrollPersistenceReady) {
                return;
            }
            if (!state.currentView || !isKnownView(state.currentView)) {
                return;
            }
            state.viewScrollByPanel[state.currentView] = Math.max(0, Math.round(window.scrollY || 0));
            storeViewScrollByPanel();
        }

        function restoreCurrentViewScroll() {
            if (!state.currentView) {
                return;
            }
            const nextY = Number((state.viewScrollByPanel || {})[state.currentView] || 0);
            requestAnimationFrame(() => {
                window.scrollTo(0, Number.isFinite(nextY) && nextY >= 0 ? nextY : 0);
            });
        }

        function loadStoredAuth() {
            const raw = sessionStorage.getItem(AUTH_STORAGE_KEY);
            if (!raw) {
                return null;
            }
            try {
                const auth = JSON.parse(raw);
                if (!auth || !auth.username || !auth.token) {
                    return null;
                }
                return auth;
            } catch (error) {
                return null;
            }
        }

        function focusLoginUsername() {
            requestAnimationFrame(() => {
                const input = document.getElementById("login-username");
                if (input && !els.loginScreen.classList.contains("hidden")) {
                    input.focus();
                }
            });
        }

        function normalizeBool(value) {
            return value === true || value === "true";
        }

        function optionHTML(value, label, selected) {
            const isSelected = selected ? " selected" : "";
            return "<option value=\"" + String(value) + "\"" + isSelected + ">" + escapeHTML(label) + "</option>";
        }

        function setInnerHTMLIfChanged(element, html) {
            if (!element) return;
            if (element.innerHTML !== html) {
                element.innerHTML = html;
            }
        }

        function escapeHTML(value) {
            return String(value || "")
                .replace(/&/g, "&amp;")
                .replace(/</g, "&lt;")
                .replace(/>/g, "&gt;")
                .replace(/"/g, "&quot;")
                .replace(/'/g, "&#39;");
        }

        function arrayBufferToBase64(buffer) {
            const bytes = new Uint8Array(buffer || new ArrayBuffer(0));
            let binary = "";
            const chunkSize = 0x8000;
            for (let index = 0; index < bytes.length; index += chunkSize) {
                const chunk = bytes.subarray(index, index + chunkSize);
                binary += String.fromCharCode.apply(null, Array.from(chunk));
            }
            return btoa(binary);
        }

        function getCurrentProject() {
            return state.projects.find((project) => project.id === state.selectedProjectID) || null;
        }

        function getCurrentGoal() {
            return state.goals.find((goal) => (goal.id !== undefined ? goal.id : goal.goal_id) === state.selectedGoalID) || null;
        }

        function getCurrentDocument() {
            return state.documents.find((documentItem) => (documentItem.id !== undefined ? documentItem.id : documentItem.document_id) === state.selectedDocumentID) || null;
        }

        function getCurrentProjectWorkflow() {
            const project = getCurrentProject();
            if (!project || !project.workflow_id) {
                return null;
            }
            return state.workflows.find((item) => item.id === project.workflow_id) || null;
        }

        function getCurrentWorkflow() {
            return state.workflows.find((item) => item.id === state.selectedWorkflowID) || null;
        }

        function getCurrentRole() {
            return state.roles.find((item) => item.id === state.selectedRoleID) || null;
        }

        function getCurrentAgent() {
            return state.agents.find((item) => item.id === state.selectedAgentID) || null;
        }

        function getCurrentTeam() {
            return state.teams.find((item) => item.id === state.selectedTeamID) || null;
        }

        function getStageOptions() {
            const workflow = getCurrentProjectWorkflow();
            const fromWorkflow = workflow && Array.isArray(workflow.stages) && workflow.stages.length
                ? workflow.stages.map((stage) => stage.name)
                : [];
            const fromTickets = state.tickets.map((ticket) => ticket.stage).filter(Boolean);
            const stages = fromWorkflow.length
                ? fromWorkflow.concat(fromTickets)
                : (fromTickets.length ? fromTickets : FALLBACK_STAGES.slice());
            return Array.from(new Set(stages.filter(Boolean)));
        }

        function getBoardLaneDescriptors() {
            const workflow = getCurrentProjectWorkflow();
            const stageMap = new Map((workflow && workflow.stages ? workflow.stages : []).map((stage) => [stage.name, stage.id]));
            return getStageOptions().map((name) => ({
                name,
                workflowStageID: stageMap.get(name) || null,
            }));
        }

        function bindViewNavigation() {
            els.mainNav.addEventListener("click", (event) => {
                const button = event.target.closest("button[data-view]");
                if (!button) {
                    return;
                }
                switchView(button.dataset.view);
            });
        }

        function switchView(viewName, options) {
            if (!isKnownView(viewName)) {
                return;
            }
            const settings = options || {};
            if (state.currentView && state.currentView !== viewName) {
                storeCurrentViewScroll();
            }
            state.currentView = viewName;
            if (settings.persist !== false) {
                storeSelectedView(viewName);
            }
            document.querySelectorAll(".nav button[data-view]").forEach((button) => {
                button.classList.toggle("active", button.dataset.view === viewName);
            });
            document.querySelectorAll(".view").forEach((view) => {
                view.classList.toggle("active", view.id === "view-" + viewName);
            });
            if (settings.restoreScroll !== false) {
                restoreCurrentViewScroll();
            }
        }

        function renderProjectMenu() {
            const current = getCurrentProject();
            els.projectMenuButton.textContent = current ? (current.title + " (" + current.prefix + ")") : "Projects";
            const otherProjects = state.projects.filter((project) => project.id !== state.selectedProjectID);
            els.projectMenuList.innerHTML = otherProjects.length
                ? otherProjects.map((project) => "<button type=\"button\" class=\"menu-item\" data-project-switch=\"" + project.id + "\">" + escapeHTML(project.title + " (" + project.prefix + ")") + "</button>").join("")
                : "<div class=\"account-name\">No other projects</div>";
        }

        async function selectProject(projectID) {
            state.selectedProjectID = Number(projectID);
            storeSelectedProjectID(state.selectedProjectID);
            state.selectedProjectDraft = getCurrentProject() ? structuredClone(getCurrentProject()) : emptyProject();
            populateTicketTypeAndStageSelects();
            await Promise.all([loadTickets(), loadGoals(), loadDocuments()]);
            renderAll();
        }

        function populateWorkflowSelects() {
            const options = [optionHTML("", "None", false)].concat(
                state.workflows.map((workflow) => optionHTML(workflow.id, workflow.name, false))
            ).join("");
            document.getElementById("project-workflow").innerHTML = options;
            document.getElementById("role-workflow").innerHTML = options;
            document.getElementById("ticket-workflow").innerHTML = options;
        }

        function populateTeamParentSelect() {
            const current = getCurrentTeam();
            const options = [optionHTML("", "None", !current || !current.parent_team_id)]
                .concat(state.teams.filter((team) => !current || team.id !== current.id).map((team) => optionHTML(team.id, team.name, current && current.parent_team_id === team.id)))
                .join("");
            document.getElementById("team-parent").innerHTML = options;
        }

        function populateTicketTypeAndStageSelects() {
            document.getElementById("ticket-type").innerHTML = TICKET_TYPES.map((type) => optionHTML(type, type, false)).join("");
            document.getElementById("ticket-stage").innerHTML = getStageOptions().map((stage) => optionHTML(stage, stage, false)).join("");
        }

        async function loadStatus() {
            state.status = await api("/api/status");
            const username = state.status.username || "user";
            els.accountMenuButton.textContent = username.charAt(0).toUpperCase();
            els.accountMenuName.textContent = username;
        }

        async function loadProjects() {
            const projects = await api("/api/projects");
            state.projects = Array.isArray(projects) ? projects.map(normalizeProject) : [];
            if (!state.selectedProjectID) {
                state.selectedProjectID = loadStoredSelectedProjectID();
            }
            if (!state.selectedProjectID && state.projects.length) {
                state.selectedProjectID = state.projects[0].id;
            }
            if (state.selectedProjectID && !state.projects.some((project) => project.id === state.selectedProjectID)) {
                state.selectedProjectID = state.projects.length ? state.projects[0].id : null;
            }
            storeSelectedProjectID(state.selectedProjectID);
            const project = getCurrentProject();
            state.selectedProjectDraft = project ? structuredClone(project) : emptyProject();
        }

        async function loadTickets() {
            if (!state.selectedProjectID) {
                state.tickets = [];
                state.interventions = [];
                state.interventionReport = null;
                state.interventionTrends = [];
                state.interventionDrilldown = null;
                state.projectForecast = [];
                state.forecastCalibration = null;
                state.forecastBacktest = null;
                return;
            }
            const [tickets, interventions, interventionReport, interventionTrends, interventionDrilldown, projectForecast, forecastCalibration, forecastBacktest] = await Promise.all([
                api("/api/projects/" + state.selectedProjectID + "/tickets"),
                api("/api/projects/" + state.selectedProjectID + "/interventions"),
                apiWithFallback("/api/projects/" + state.selectedProjectID + "/interventions/report", null),
                apiWithFallback("/api/projects/" + state.selectedProjectID + "/interventions/trends?days=7", []),
                apiWithFallback("/api/projects/" + state.selectedProjectID + "/interventions/drilldown?escalation_hours=24", null),
                apiWithFallback("/api/projects/" + state.selectedProjectID + "/forecast?limit=100", []),
                apiWithFallback("/api/projects/" + state.selectedProjectID + "/forecast/calibration?lookback_hours=1", null),
                apiWithFallback("/api/projects/" + state.selectedProjectID + "/forecast/backtest?window_hours=24", null),
            ]);
            state.tickets = Array.isArray(tickets) ? tickets.map(normalizeTicket) : [];
            state.interventions = Array.isArray(interventions) ? interventions.map(normalizeTicket) : [];
            state.interventionReport = interventionReport;
            state.interventionTrends = Array.isArray(interventionTrends) ? interventionTrends : [];
            state.interventionDrilldown = interventionDrilldown;
            state.projectForecast = Array.isArray(projectForecast) ? projectForecast : [];
            state.forecastCalibration = forecastCalibration;
            state.forecastBacktest = forecastBacktest;
            const dependencyEntries = await Promise.all(state.tickets.map(async (ticket) => {
                try {
                    const dependencies = await api("/api/tickets/" + ticket.id + "/dependencies");
                    const ids = Array.isArray(dependencies) ? dependencies.map((entry) => String(entry.depends_on || "")).filter(Boolean) : [];
                    return [String(ticket.id), ids];
                } catch (error) {
                    return [String(ticket.id), []];
                }
            }));
            const workItemEntries = await Promise.all(state.interventions.map(async (ticket) => {
                try {
                    const workItems = await api("/api/tickets/" + ticket.id + "/work-items?limit=1");
                    return [String(ticket.id), Array.isArray(workItems) ? workItems : []];
                } catch (error) {
                    return [String(ticket.id), []];
                }
            }));
            const interventionStateEntries = await Promise.all(state.interventions.map(async (ticket) => {
                const nested = ticket.intervention_state;
                if (nested && typeof nested === "object" && nested.state) {
                    return [String(ticket.id), nested];
                }
                try {
                    const interventionState = await api("/api/tickets/" + ticket.id + "/intervention-state");
                    return [String(ticket.id), interventionState];
                } catch (error) {
                    return [String(ticket.id), { ticket_id: ticket.id, state: "open", owner_name: "" }];
                }
            }));
            const historyEntries = await Promise.all(state.interventions.map(async (ticket) => {
                try {
                    const history = await api("/api/tickets/" + ticket.id + "/history?limit=3");
                    return [String(ticket.id), Array.isArray(history) ? history : []];
                } catch (error) {
                    return [String(ticket.id), []];
                }
            }));
            const commentEntries = await Promise.all(state.interventions.map(async (ticket) => {
                try {
                    const comments = await api("/api/tickets/" + ticket.id + "/comments");
                    return [String(ticket.id), Array.isArray(comments) ? comments.slice(-3).reverse() : []];
                } catch (error) {
                    return [String(ticket.id), []];
                }
            }));
            state.dependencyIndex = Object.fromEntries(dependencyEntries);
            state.interventionWorkItems = Object.fromEntries(workItemEntries);
            state.interventionStates = Object.fromEntries(interventionStateEntries);
            state.interventionHistory = Object.fromEntries(historyEntries);
            state.interventionComments = Object.fromEntries(commentEntries);
        }

        async function loadGoals() {
            if (!state.selectedProjectID) {
                state.goals = [];
                state.selectedGoalID = null;
                state.selectedGoalDraft = emptyGoal(state.selectedProjectID);
                return;
            }
            try {
                const query = new URLSearchParams();
                if (state.goalInboxStatusFilter) {
                    query.set("status", state.goalInboxStatusFilter);
                }
                if (state.goalInboxSort) {
                    query.set("sort", state.goalInboxSort);
                }
                const inboxPath = "/api/projects/" + state.selectedProjectID + "/goal-inbox" + (query.toString() ? ("?" + query.toString()) : "");
                const inbox = await api(inboxPath);
                const goals = await api("/api/projects/" + state.selectedProjectID + "/goals");
                const goalsByID = new Map((Array.isArray(goals) ? goals : []).map((goal) => [goal.goal_id || goal.id, normalizeGoal(goal)]));
                state.goals = Array.isArray(inbox)
                    ? inbox.map((entry) => {
                        const id = entry.goal_id || entry.id;
                        const goal = goalsByID.get(id);
                        if (!goal) {
                            return null;
                        }
                        return Object.assign({}, goal, {
                            decomposition_depth: Number(entry.decomposition_depth || 0),
                            unresolved_clarifications: Number(entry.unresolved_clarifications || 0),
                            refinement_confirmed: Boolean(entry.refinement_confirmed),
                        });
                    }).filter(Boolean)
                    : [];
            } catch (error) {
                state.goals = [];
            }
            if (!state.selectedGoalID && state.goals.length) {
                state.selectedGoalID = state.goals[0].goal_id || state.goals[0].id;
            }
            if (state.selectedGoalID && !state.goals.some((goal) => (goal.goal_id || goal.id) === state.selectedGoalID)) {
                state.selectedGoalID = state.goals.length ? (state.goals[0].goal_id || state.goals[0].id) : null;
            }
            const current = getCurrentGoal();
            state.selectedGoalDraft = current ? structuredClone(normalizeGoal(current)) : emptyGoal(state.selectedProjectID);
            await loadGoalChatMessages();
        }

        async function loadDocuments() {
            if (!state.selectedProjectID) {
                state.documents = [];
                state.selectedDocumentID = null;
                state.selectedDocumentDraft = emptyDocument(state.selectedProjectID);
                state.documentFiles = [];
                return;
            }
            try {
                const response = await api("/api/projects/" + state.selectedProjectID + "/documents");
                state.documents = Array.isArray(response) ? response.map(normalizeDocument) : [];
            } catch (error) {
                state.documents = [];
            }
            if (!state.selectedDocumentID && state.documents.length) {
                state.selectedDocumentID = state.documents[0].id;
            }
            if (state.selectedDocumentID && !state.documents.some((documentItem) => documentItem.id === state.selectedDocumentID)) {
                state.selectedDocumentID = state.documents.length ? state.documents[0].id : null;
            }
            const current = getCurrentDocument();
            state.selectedDocumentDraft = current ? structuredClone(normalizeDocument(current)) : emptyDocument(state.selectedProjectID);
            if (!state.selectedDocumentID) {
                state.documentFiles = [];
                return;
            }
            await loadDocumentFiles(state.selectedDocumentID);
        }

        async function loadDocumentFiles(documentID) {
            try {
                const filesResponse = await api("/api/documents/" + documentID + "/files");
                state.documentFiles = Array.isArray(filesResponse) ? filesResponse.map(normalizeDocumentFile) : [];
            } catch (error) {
                state.documentFiles = [];
            }
        }

        async function loadGoalChatMessages() {
            if (!state.selectedGoalID) {
                state.goalChatMessages = [];
                state.goalChatLoadedFor = null;
                return;
            }
            if (state.goalChatLoadedFor === state.selectedGoalID) {
                return;
            }
            try {
                const messages = await api("/api/goals/" + state.selectedGoalID + "/chat/messages");
                state.goalChatMessages = Array.isArray(messages)
                    ? messages.map((message) => ({ author: message.author || "system", text: message.text || "" }))
                    : [];
            } catch (error) {
                state.goalChatMessages = [];
            }
            state.goalChatLoadedFor = state.selectedGoalID;
        }

        async function loadWorkflows() {
            const summariesResponse = await api("/api/workflows");
            const summaries = Array.isArray(summariesResponse) ? summariesResponse : [];
            state.workflows = await Promise.all(summaries.map(async (summary) => normalizeWorkflow(await api("/api/workflows/" + (summary.workflow_id || summary.id)))));
            if (!state.selectedWorkflowID && state.workflows.length) {
                state.selectedWorkflowID = state.workflows[0].id;
            }
            if (state.selectedWorkflowID && !state.workflows.some((item) => item.id === state.selectedWorkflowID)) {
                state.selectedWorkflowID = state.workflows.length ? state.workflows[0].id : null;
            }
            state.selectedWorkflowDraft = getCurrentWorkflow() ? structuredClone(getCurrentWorkflow()) : emptyWorkflow();
            if (state.selectedWorkflowID) {
                await loadWorkflowValidation(state.selectedWorkflowID).catch((error) => {
                    console.warn("failed to load workflow validation", error);
                });
            }
        }

        async function loadRoles() {
            const roles = await api("/api/roles");
            state.roles = Array.isArray(roles) ? roles.map(normalizeRole) : [];
            if (!state.selectedRoleID && state.roles.length) {
                state.selectedRoleID = state.roles[0].id;
            }
            if (state.selectedRoleID && !state.roles.some((item) => item.id === state.selectedRoleID)) {
                state.selectedRoleID = state.roles.length ? state.roles[0].id : null;
            }
            state.selectedRoleDraft = getCurrentRole() ? structuredClone(getCurrentRole()) : emptyRole();
        }

        async function loadAgents() {
            try {
                const agents = await api("/api/agents");
                state.agents = Array.isArray(agents) ? agents.map(normalizeAgent) : [];
                if (!state.selectedAgentID && state.agents.length) {
                    state.selectedAgentID = state.agents[0].id;
                }
            } catch (error) {
                state.agents = [];
            }
        }

        async function loadTeams() {
            try {
                const teams = await api("/api/teams");
                state.teams = Array.isArray(teams) ? teams.map(normalizeTeam) : [];
                if (!state.selectedTeamID && state.teams.length) {
                    state.selectedTeamID = state.teams[0].id;
                }
                if (state.selectedTeamID && !state.teams.some((team) => team.id === state.selectedTeamID)) {
                    state.selectedTeamID = state.teams.length ? state.teams[0].id : null;
                }
                state.selectedTeamDraft = getCurrentTeam() ? structuredClone(getCurrentTeam()) : emptyTeam();
            } catch (error) {
                state.teams = [];
            }
        }

        async function refreshAll() {
            await loadStatus();
            await Promise.all([loadWorkflows(), loadRoles(), loadProjects(), loadAgents(), loadTeams()]);
            renderProjectMenu();
            populateWorkflowSelects();
            populateTicketTypeAndStageSelects();
            populateTeamParentSelect();
            await Promise.all([loadTickets(), loadGoals(), loadDocuments()]);
            renderAll();
        }

        function showAuthenticatedShell() {
            els.loginScreen.classList.add("hidden");
            els.appShell.classList.remove("hidden");
            els.loginError.textContent = "";
            state.scrollPersistenceReady = true;
            restoreCurrentViewScroll();
        }

        function showLoginScreen() {
            state.scrollPersistenceReady = false;
            els.appShell.classList.add("hidden");
            els.loginScreen.classList.remove("hidden");
            focusLoginUsername();
        }

        function disconnectLiveUpdates() {
            if (state.liveRefreshTimer) {
                clearTimeout(state.liveRefreshTimer);
                state.liveRefreshTimer = null;
            }
            if (state.liveSocket) {
                state.liveSocket.close();
                state.liveSocket = null;
            }
        }

        function scheduleLiveRefresh() {
            if (state.liveRefreshTimer) {
                return;
            }
            state.liveRefreshTimer = setTimeout(() => {
                state.liveRefreshTimer = null;
                refreshAll().catch((error) => setNotice(error.message, true));
            }, 150);
        }

        function connectLiveUpdates() {
            if (window.__site2MockFetch || state.liveSocket) {
                return;
            }
            const scheme = window.location.protocol === "https:" ? "wss:" : "ws:";
            const socket = new WebSocket(scheme + "//" + window.location.host + "/api/ws");
            state.liveSocket = socket;
            socket.addEventListener("message", (event) => {
                try {
                    const payload = JSON.parse(event.data);
                    if (!payload || payload.type === "connected") {
                        return;
                    }
                    scheduleLiveRefresh();
                } catch (error) {
                    // Ignore malformed live payloads.
                }
            });
            socket.addEventListener("close", () => {
                if (state.liveSocket === socket) {
                    state.liveSocket = null;
                }
                if (state.auth) {
                    setTimeout(() => {
                        if (state.auth) {
                            connectLiveUpdates();
                        }
                    }, 1500);
                }
            });
        }

        function renderAll() {
            renderProjectMenu();
            populateWorkflowSelects();
            populateTicketTypeAndStageSelects();
            populateTeamParentSelect();
            renderProjects();
            renderGoals();
            renderDocuments();
            renderWorkflows();
            renderRoles();
            renderAgents();
            renderTeams();
            renderTicketBoard();
            renderPredictedNextWork();
            renderInterventions();
            renderEditors();
            restoreCurrentViewScroll();
        }

        function renderProjects() {
            if (!state.projects.length) {
                els.projectList.innerHTML = "<div class=\"empty\">No projects yet.</div>";
                return;
            }
            els.projectList.innerHTML = state.projects.map((project) => {
                const active = project.id === state.selectedProjectID ? " active" : "";
                return "<div class=\"entity-card" + active + "\" data-project-id=\"" + project.id + "\">" +
                    "<h4>" + escapeHTML(project.title) + " <small>(" + escapeHTML(project.prefix) + ")</small></h4>" +
                    "<p>" + escapeHTML(project.description || "No description") + "</p>" +
                    "<div class=\"tag-row tag-row-spaced\">" +
                    "<span class=\"chip\">" + escapeHTML(project.visibility || "public") + "</span>" +
                    "<span class=\"chip\">draft " + String(Boolean(project.default_draft)) + "</span>" +
                    "</div></div>";
            }).join("");
        }

        function renderGoals() {
            if (els.goalInboxStatusFilter) {
                els.goalInboxStatusFilter.value = state.goalInboxStatusFilter || "";
            }
            if (els.goalInboxSort) {
                els.goalInboxSort.value = state.goalInboxSort || "updated_desc";
            }
            if (!state.selectedProjectID) {
                els.goalList.innerHTML = "<div class=\"empty\">Select a project first.</div>";
                renderGoalChat();
                return;
            }
            if (!state.goals.length) {
                els.goalList.innerHTML = "<div class=\"empty\">No goals yet.</div>";
                renderGoalChat();
                return;
            }
            els.goalList.innerHTML = state.goals.map((rawGoal) => {
                const goal = normalizeGoal(rawGoal);
                const active = goal.id === state.selectedGoalID ? " active" : "";
                return "<div class=\"entity-card" + active + "\" data-goal-id=\"" + goal.id + "\">" +
                    "<h4>" + escapeHTML(goal.title || "Untitled goal") + "</h4>" +
                    "<p>" + escapeHTML(goal.description || "No description") + "</p>" +
                    "<div class=\"tag-row tag-row-spaced\">" +
                    "<span class=\"chip\">" + escapeHTML(goal.status || "draft") + "</span>" +
                    "<span class=\"chip\">p" + escapeHTML(String(goal.priority || 1)) + "</span>" +
                    "<span class=\"chip\">depth " + escapeHTML(String(goal.decomposition_depth || 0)) + "</span>" +
                    "<span class=\"chip\">open clarifications " + escapeHTML(String(goal.unresolved_clarifications || 0)) + "</span>" +
                    "</div></div>";
            }).join("");
            renderGoalChat();
        }

        function renderDocuments() {
            if (!state.selectedProjectID) {
                els.documentList.innerHTML = "<div class=\"empty\">Select a project first.</div>";
                return;
            }
            if (!state.documents.length) {
                els.documentList.innerHTML = "<div class=\"empty\">No documents yet.</div>";
                return;
            }
            els.documentList.innerHTML = state.documents.map((documentItem) => {
                const active = documentItem.id === state.selectedDocumentID ? " active" : "";
                return "<div class=\"entity-card" + active + "\" data-document-id=\"" + documentItem.id + "\">" +
                    "<h4>" + escapeHTML(documentItem.title || "Untitled document") + "</h4>" +
                    "<p>" + escapeHTML(documentItem.description || "No description") + "</p>" +
                    "<p class=\"meta meta-top\">Updated " + escapeHTML(documentItem.updated_at || documentItem.created_at || "") + "</p>" +
                    "</div>";
            }).join("");
        }

        function renderGoalEditor() {
            const goal = state.selectedGoalDraft || emptyGoal(state.selectedProjectID);
            document.getElementById("goal-editor-title").textContent = goal.id ? "Goal: " + goal.title : "Goal editor";
            document.getElementById("goal-title").value = goal.title || "";
            document.getElementById("goal-description").value = goal.description || "";
            document.getElementById("goal-notes").value = goal.notes || "";
            document.getElementById("goal-eta").value = goal.eta || "";
            document.getElementById("goal-priority").value = String(goal.priority || 1);
            document.getElementById("goal-status").value = goal.status || "draft";
            els.goalRefinedGoal.value = goal.refined_goal || "";
            els.goalDecomposition.value = goal.decomposition || "";
            state.goalDecompositionItems = parseGoalDecompositionItems(goal.decomposition || "");
            renderGoalDecompositionList();
            document.getElementById("delete-goal-button").disabled = !goal.id;
            document.getElementById("refine-goal-button").disabled = !goal.id;
            document.getElementById("ready-goal-button").disabled = !goal.id;
            document.getElementById("save-goal-refinement-button").disabled = !goal.id;
            document.getElementById("goal-use-last-agent-response").disabled = !goal.id;
        }

        function renderGoalDecompositionList() {
            if (!els.goalDecompositionList) {
                return;
            }
            if (!Array.isArray(state.goalDecompositionItems) || !state.goalDecompositionItems.length) {
                els.goalDecompositionList.innerHTML = "<div class=\"empty\">No decomposition items yet.</div>";
                return;
            }
            els.goalDecompositionList.innerHTML = state.goalDecompositionItems.map((item, index) => {
                return "<div class=\"goal-decomposition-item\" data-decomposition-index=\"" + String(index) + "\">" +
                    "<div class=\"meta\">" + escapeHTML(item) + "</div>" +
                    "<div class=\"goal-decomposition-controls\">" +
                    "<button type=\"button\" data-decomposition-up=\"" + String(index) + "\" " + (index === 0 ? "disabled" : "") + ">↑</button>" +
                    "<button type=\"button\" data-decomposition-down=\"" + String(index) + "\" " + (index === state.goalDecompositionItems.length - 1 ? "disabled" : "") + ">↓</button>" +
                    "<button type=\"button\" data-decomposition-delete=\"" + String(index) + "\" class=\"btn-danger\">Remove</button>" +
                    "</div></div>";
            }).join("");
        }

        function renderGoalChat() {
            if (!els.goalChatLog) {
                return;
            }
            if (!Array.isArray(state.goalChatMessages) || !state.goalChatMessages.length) {
                els.goalChatLog.innerHTML = "<div class=\"empty\">No messages yet.</div>";
                return;
            }
            els.goalChatLog.innerHTML = state.goalChatMessages.map((message) => {
                const author = String(message.author || "system").toLowerCase();
                let className = "history-item";
                if (author === "agent") {
                    className += " history-item-agent";
                } else if (author === "system") {
                    className += " history-item-system";
                }
                return "<div class=\"" + className + "\"><strong>" + escapeHTML(message.author || "system") + "</strong><div class=\"meta\">" +
                    escapeHTML(message.text || "") + "</div></div>";
            }).join("");
            els.goalChatLog.scrollTop = els.goalChatLog.scrollHeight;
        }

        function renderWorkflows() {
            const roleBankHTML = state.roles.length
                ? state.roles.map((role) => "<span class=\"role-chip\" draggable=\"true\" data-role-bank-id=\"" + role.id + "\">" + escapeHTML(role.title) + "</span>").join("")
                : "<span class=\"meta\">No roles yet.</span>";
            setInnerHTMLIfChanged(els.workflowRoleBank, roleBankHTML);

            if (!state.workflows.length) {
                setInnerHTMLIfChanged(els.workflowList, "<div class=\"empty\">No Workflows yet.</div>");
                setInnerHTMLIfChanged(els.stageGrid, "");
                return;
            }
            const workflowListHTML = state.workflows.map((workflow) => {
                const active = workflow.id === state.selectedWorkflowID ? " active" : "";
                return "<div class=\"entity-card" + active + "\" data-workflow-id=\"" + workflow.id + "\">" +
                    "<h4>" + escapeHTML(workflow.name) + "</h4>" +
                    "<p>" + escapeHTML(workflow.description || "No description") + "</p>" +
                    "<p class=\"meta\">policy: " + escapeHTML(workflow.approval_policy || "single_role") + " · mode: " + escapeHTML(workflow.progression_mode || "linear") + "</p>" +
                    "<p class=\"meta meta-top\">" + (workflow.stages ? workflow.stages.length : 0) + " stages</p>" +
                    "</div>";
            }).join("");
            setInnerHTMLIfChanged(els.workflowList, workflowListHTML);

            const workflow = getCurrentWorkflow();
            if (!workflow || !Array.isArray(workflow.stages) || !workflow.stages.length) {
                setInnerHTMLIfChanged(els.stageGrid, "<div class=\"empty\">No stages yet.</div>");
                return;
            }

            const stageGridHTML = workflow.stages.map((stage) => {
                const roleNames = (stage.roles || []).map((role) => {
                    const fullRole = state.roles.find((item) => item.id === role.id);
                    const label = fullRole ? fullRole.title : role.title || ("role " + role.id);
                    return "<span class=\"role-chip\" draggable=\"true\" data-stage-id=\"" + stage.id + "\" data-role-id=\"" + role.id + "\">" + escapeHTML(label) + "</span>";
                }).join("");
                const addRoleOptions = state.roles
                    .filter((role) => !(stage.roles || []).some((current) => current.id === role.id))
                    .map((role) => optionHTML(role.id, role.title, false))
                    .join("");
                return "<div class=\"stage-card\" draggable=\"true\" data-stage-id=\"" + stage.id + "\">" +
                    "<div class=\"panel-head\"><div><h4>" + escapeHTML(stage.name) + "</h4><small>Drag to reorder</small></div>" +
                    "<button type=\"button\" class=\"btn-danger\" data-delete-stage=\"" + stage.id + "\">Delete</button></div>" +
                    "<div class=\"row\">" +
                    "<div class=\"field\"><label>Stage name</label><input data-stage-name=\"" + stage.id + "\" value=\"" + escapeHTML(stage.name) + "\"></div>" +
                    "<div class=\"field\"><label>Ways of working</label><textarea data-stage-wow=\"" + stage.id + "\">" + escapeHTML(stage.wow || stage.description || "") + "</textarea></div>" +
                    "</div>" +
                    "<div class=\"row\">" +
                    "<div class=\"field\"><label>Definition of ready</label><textarea data-stage-dor=\"" + stage.id + "\">" + escapeHTML(stage.dor || "") + "</textarea></div>" +
                    "<div class=\"field\"><label>Definition of done</label><textarea data-stage-dod=\"" + stage.id + "\">" + escapeHTML(stage.dod || "") + "</textarea></div>" +
                    "</div>" +
                    "<div class=\"entity-actions\"><button type=\"button\" class=\"btn-primary\" data-save-stage=\"" + stage.id + "\">Save stage</button></div>" +
                    "<div class=\"field\"><label>Roles in stage</label><div class=\"role-chip-row\" data-stage-role-row=\"" + stage.id + "\">" + (roleNames || "<span class=\"meta\">No roles</span>") + "</div></div>" +
                    "<div class=\"row\">" +
                    "<div class=\"field\"><label>Add role</label><select data-add-role-select=\"" + stage.id + "\">" + optionHTML("", "Choose role", true) + addRoleOptions + "</select></div>" +
                    "<div class=\"field field-align-end\"><button type=\"button\" data-add-role=\"" + stage.id + "\">Add role</button></div>" +
                    "</div>" +
                    "<div class=\"field\"><label>Transitions (next stages)</label><select multiple data-stage-next=\"" + stage.id + "\">" +
                    workflow.stages
                        .filter((candidate) => Number(candidate.id) !== Number(stage.id))
                        .map((candidate) => optionHTML(candidate.id, candidate.name || candidate.stage_name || ("stage " + candidate.id), (stage.next_stage_ids || []).some((nextID) => Number(nextID) === Number(candidate.id))))
                        .join("") +
                    "</select></div>" +
                    "</div>";
            }).join("");
        }

        function renderRoles() {
            if (!state.roles.length) {
                els.roleList.innerHTML = "<div class=\"empty\">No roles yet.</div>";
                return;
            }
            els.roleList.innerHTML = state.roles.map((role) => {
                const active = role.id === state.selectedRoleID ? " active" : "";
                const workflow = state.workflows.find((item) => item.id === role.workflow_id);
                return "<div class=\"entity-card" + active + "\" data-role-id=\"" + role.id + "\">" +
                    "<h4>" + escapeHTML(role.title) + "</h4>" +
                    "<p>" + escapeHTML(role.description || "No description") + "</p>" +
                    "<p class=\"meta meta-top\">Workflow: " + escapeHTML(workflow ? workflow.name : "None") + "</p>" +
                    "</div>";
            }).join("");
        }

        function renderAgents() {
            if (!state.agents.length) {
                els.agentList.innerHTML = "<div class=\"empty\">No agents available.</div>";
                return;
            }
            els.agentList.innerHTML = state.agents.map((agent) => {
                const active = agent.id === state.selectedAgentID ? " active" : "";
                return "<div class=\"entity-card" + active + "\" data-agent-id=\"" + escapeHTML(agent.id) + "\">" +
                    "<h4>" + escapeHTML(agent.id) + "</h4>" +
                    "<p>" + escapeHTML(agent.enabled ? "enabled" : "disabled") + "</p>" +
                    "</div>";
            }).join("");
        }

        function renderTeams() {
            if (!state.teams.length) {
                els.teamList.innerHTML = "<div class=\"empty\">No teams yet.</div>";
                return;
            }
            els.teamList.innerHTML = state.teams.map((team) => {
                const active = team.id === state.selectedTeamID ? " active" : "";
                const parent = state.teams.find((item) => item.id === team.parent_team_id);
                return "<div class=\"entity-card" + active + "\" data-team-id=\"" + team.id + "\">" +
                    "<h4>" + escapeHTML(team.name) + "</h4>" +
                    "<p>" + escapeHTML(parent ? ("Parent: " + parent.name) : "Top-level team") + "</p>" +
                    "</div>";
            }).join("");
        }

        function renderTicketBoard() {
            const lanes = getBoardLaneDescriptors();
            const searchText = (els.boardSearch && els.boardSearch.value ? els.boardSearch.value : "").trim().toLowerCase();
            const hideDone = Boolean(els.boardHideDone && els.boardHideDone.checked);
            els.ticketBoard.innerHTML = lanes.map((lane) => {
                const cards = state.tickets
                    .filter((ticket) => (ticket.stage || lanes[0].name) === lane.name)
                    .filter((ticket) => !hideDone || String(ticket.stage || "").toLowerCase() !== "done")
                    .filter((ticket) => !searchText || String(ticket.title || "").toLowerCase().includes(searchText) || String(ticket.key || ticket.id || "").toLowerCase().includes(searchText))
                    .sort((a, b) => (a.order || 0) - (b.order || 0))
                    .map((ticket) => renderTicketCard(ticket))
                    .join("");
                const draggable = lane.workflowStageID ? "true" : "false";
                const stageAttr = lane.workflowStageID ? " data-workflow-stage-id=\"" + lane.workflowStageID + "\"" : "";
                return "<div class=\"lane\" draggable=\"" + draggable + "\" data-lane-stage=\"" + escapeHTML(lane.name) + "\"" + stageAttr + ">" +
                    "<div class=\"lane-head\"><h3>" + escapeHTML(lane.name) + "</h3><span class=\"chip\">" + (cards ? (cards.match(/ticket-card/g) || []).length : 0) + "</span></div>" +
                    (cards || "<div class=\"empty\">No tickets</div>") +
                    "</div>";
            }).join("");
        }

        function renderPredictedNextWork() {
            if (!els.predictedWorkList) {
                return;
            }
            if (els.forecastCalibrationSummary) {
                const calibration = state.forecastCalibration;
                const backtest = state.forecastBacktest;
                if (calibration && Number(calibration.sample_count || 0) > 0) {
                    els.forecastCalibrationSummary.textContent = "Calibration: " + String(Math.round((calibration.accuracy_rate || 0) * 100)) +
                        "% accuracy across " + String(calibration.sample_count || 0) + " samples" +
                        (backtest && Number(backtest.sample_count || 0) > 0
                            ? " · backtest " + String(Math.round((backtest.accuracy_rate || 0) * 100)) + "% / " + String(backtest.sample_count || 0) + " samples"
                            : "");
                } else {
                    els.forecastCalibrationSummary.textContent = backtest && Number(backtest.sample_count || 0) > 0
                        ? "Backtest: " + String(Math.round((backtest.accuracy_rate || 0) * 100)) + "% over last " + String(backtest.window_hours || 24) + "h"
                        : "Calibration: building sample history";
                }
            }
            const project = getCurrentProject();
            if (!project) {
                els.predictedWorkList.innerHTML = "<div class=\"empty\">Select a project to see predictions.</div>";
                return;
            }
            const predictions = state.projectForecast
                .filter((entry) => entry && entry.key && entry.detail);
            if (!predictions.length) {
                els.predictedWorkList.innerHTML = "<div class=\"empty\">No forecastable tickets.</div>";
                return;
            }
            els.predictedWorkList.innerHTML = predictions
                .map((entry) => "<div class=\"history-item\"><strong>" + escapeHTML(entry.key || entry.ticket_id || "") + "</strong> — " + escapeHTML(entry.detail || "") + " <span class=\"meta\">(" + escapeHTML(String(entry.confidence_percent || 0)) + "% confidence)</span></div>")
                .join("");
        }

        function renderInterventions() {
            if (els.interventionReportSummary) {
                if (state.interventionReport) {
                    const report = state.interventionReport;
                    const drilldown = state.interventionDrilldown;
                    const ownerTop = drilldown && Array.isArray(drilldown.by_owner) && drilldown.by_owner.length ? drilldown.by_owner[0] : null;
                    els.interventionReportSummary.textContent = "open " + String(report.open_count || 0) +
                        " · triaged " + String(report.triaged_count || 0) +
                        " · in progress " + String(report.in_progress_count || 0) +
                        " · resolved " + String(report.resolved_count || 0) +
                        " · oldest active age " + String(report.oldest_open_age_h || 0) + "h" +
                        (drilldown ? " · escalated " + String(drilldown.escalated_count || 0) : "") +
                        (ownerTop ? " · top owner " + String(ownerTop.key || "") + " (" + String(ownerTop.count || 0) + ")" : "");
                } else {
                    els.interventionReportSummary.textContent = "";
                }
            }
            if (els.interventionTrendsSummary) {
                const latest = Array.isArray(state.interventionTrends) && state.interventionTrends.length
                    ? state.interventionTrends[state.interventionTrends.length - 1]
                    : null;
                if (latest) {
                    els.interventionTrendsSummary.textContent = "Trend (" + String(latest.day || "today") + "): open " +
                        String(latest.open_count || 0) + " · triaged " + String(latest.triaged_count || 0) +
                        " · in progress " + String(latest.in_progress_count || 0) +
                        " · resolved " + String(latest.resolved_count || 0);
                } else {
                    els.interventionTrendsSummary.textContent = "";
                }
            }
            const failed = state.interventions
                .filter((ticket) => {
                    const mode = (els.interventionFilter && els.interventionFilter.value) || "all";
                    if (mode === "all") {
                        return true;
                    }
                    const latestWorkItem = (state.interventionWorkItems[String(ticket.id)] || [])[0] || null;
                    if (mode === "unassigned") {
                        return !latestWorkItem || !String(latestWorkItem.assignee_id || "").trim();
                    }
                    if (mode === "agent") {
                        return latestWorkItem && String(latestWorkItem.assignee_type || "").toLowerCase() === "agent";
                    }
                    if (mode === "human") {
                        return latestWorkItem && String(latestWorkItem.assignee_type || "").toLowerCase() === "human";
                    }
                    return true;
                })
                .sort((a, b) => {
                    const mode = (els.interventionSort && els.interventionSort.value) || "priority";
                    if (mode === "order") {
                        return (a.order || 0) - (b.order || 0);
                    }
                    if (mode === "recent") {
                        const aDate = Date.parse(a.updated_at || a.created_at || "") || 0;
                        const bDate = Date.parse(b.updated_at || b.created_at || "") || 0;
                        return bDate - aDate;
                    }
                    return (a.priority || 0) - (b.priority || 0);
                });
            if (!failed.length) {
                els.interventionList.innerHTML = "<div class=\"empty\">No intervention items.</div>";
                return;
            }
            els.interventionList.innerHTML = failed.map((ticket) => {
                const latestWorkItem = (state.interventionWorkItems[String(ticket.id)] || [])[0] || null;
                const latestHistory = (state.interventionHistory[String(ticket.id)] || [])[0] || null;
                const latestComments = state.interventionComments[String(ticket.id)] || [];
                const mailboxState = state.interventionStates[String(ticket.id)] || { state: "open", owner_name: "" };
                const commentsHTML = latestComments.length
                    ? latestComments.map((comment) => "<div class=\"history-item\">" + escapeHTML(comment.comment || "") + "</div>").join("")
                    : "<div class=\"history-item\">No conversation yet.</div>";
                const latestLabel = latestWorkItem
                    ? ("latest work item: " + String(latestWorkItem.status || "unknown") + " · assignee " + String(latestWorkItem.assignee_id || "n/a"))
                    : "latest work item: none";
                const historyLabel = latestHistory
                    ? ("latest event: " + String(latestHistory.event_type || "unknown"))
                    : "latest event: none";
                return "<div class=\"entity-card\" data-intervention-ticket-id=\"" + ticket.id + "\">" +
                    "<h4>" + escapeHTML(ticket.key || ticket.id || "ticket") + " · " + escapeHTML(ticket.title || "(untitled)") + "</h4>" +
                    "<p>" + escapeHTML("stage: " + (ticket.stage || "unknown") + " · state: " + (ticket.state || "fail")) + "</p>" +
                    "<p>" + escapeHTML("mailbox: " + String(mailboxState.state || "open") + " · owner " + String(mailboxState.owner_name || "unassigned")) + "</p>" +
                    "<p>" + escapeHTML(latestLabel) + "</p>" +
                    "<p>" + escapeHTML(historyLabel) + "</p>" +
                    "<div class=\"field\"><label>Mailbox state</label><select data-intervention-state=\"" + ticket.id + "\">" +
                    optionHTML("open", "Open", String(mailboxState.state || "") === "open") +
                    optionHTML("triaged", "Triaged", String(mailboxState.state || "") === "triaged") +
                    optionHTML("in_progress", "In progress", String(mailboxState.state || "") === "in_progress") +
                    optionHTML("resolved", "Resolved", String(mailboxState.state || "") === "resolved") +
                    optionHTML("wont_fix", "Won't fix", String(mailboxState.state || "") === "wont_fix") +
                    "</select></div>" +
                    "<div class=\"field\"><label>Decision</label><select data-intervention-outcome=\"" + ticket.id + "\">" +
                    optionHTML("retry-role", "Retry role", false) +
                    optionHTML("retry-stage", "Retry previous stage", false) +
                    optionHTML("split-work", "Split into follow-up", false) +
                    optionHTML("cancel", "Cancel/archive ticket", false) +
                    "</select></div>" +
                    "<div class=\"field\"><label>Message</label><textarea data-intervention-message=\"" + ticket.id + "\" placeholder=\"Why this decision?\"></textarea></div>" +
                    "<div class=\"field\"><label>Conversation</label><div class=\"history-list\">" + commentsHTML + "</div></div>" +
                    "<div class=\"field\"><label>Add intervention comment</label><textarea data-intervention-comment=\"" + ticket.id + "\" placeholder=\"Add context for the accountable human or agent.\"></textarea></div>" +
                    "<div class=\"entity-actions\">" +
                    "<button type=\"button\" class=\"btn-primary\" data-open-intervention-ticket=\"" + ticket.id + "\">Open ticket</button>" +
                    "<button type=\"button\" data-save-intervention-state=\"" + ticket.id + "\">Save mailbox state</button>" +
                    "<button type=\"button\" data-add-intervention-comment=\"" + ticket.id + "\">Add comment</button>" +
                    "<button type=\"button\" data-retry-intervention-ticket=\"" + ticket.id + "\">Quick retry</button>" +
                    "<button type=\"button\" data-cancel-intervention-ticket=\"" + ticket.id + "\">Quick cancel</button>" +
                    "<button type=\"button\" data-apply-intervention-ticket=\"" + ticket.id + "\">Apply decision</button>" +
                    "</div>" +
                    "</div>";
            }).join("");
            setInnerHTMLIfChanged(els.stageGrid, stageGridHTML);
        }

        function renderTicketCard(ticket) {
            return "<div class=\"ticket-card\" draggable=\"true\" data-ticket-id=\"" + ticket.id + "\">" +
                "<div class=\"panel-head panel-head-tight\"><h4>" + escapeHTML(ticket.key || ticket.id || "New") + "</h4><span class=\"chip\">" + escapeHTML(ticket.type || "task") + "</span></div>" +
                "<p>" + escapeHTML(ticket.title || "(untitled)") + "</p>" +
                "<div class=\"tag-row\">" +
                "<span class=\"chip\">p" + escapeHTML(ticket.priority || 0) + "</span>" +
                "<span class=\"chip\">draft " + String(Boolean(ticket.draft)) + "</span>" +
                "</div>" +
                "</div>";
        }

        function renderEditors() {
            renderProjectEditor();
            renderGoalEditor();
            renderDocumentEditor();
            renderWorkflowEditor();
            renderRoleEditor();
            renderAgentEditor();
            renderTeamEditor();
        }

        function renderProjectEditor() {
            const project = state.selectedProjectDraft || emptyProject();
            document.getElementById("project-editor-title").textContent = project.id ? "Project: " + project.title : "Project editor";
            document.getElementById("project-prefix").value = project.prefix || "";
            document.getElementById("project-title").value = project.title || "";
            document.getElementById("project-description").value = project.description || "";
            document.getElementById("project-ac").value = project.acceptance_criteria || "";
            document.getElementById("project-git-repository").value = project.git_repository || "";
            document.getElementById("project-visibility").value = project.visibility || "public";
            document.getElementById("project-default-draft").value = String(Boolean(project.default_draft));
            document.getElementById("project-workflow").value = project.workflow_id || "";
            document.getElementById("delete-project-button").disabled = !project.id;
        }

        function renderWorkflowEditor() {
            const workflow = state.selectedWorkflowDraft || emptyWorkflow();
            document.getElementById("workflow-editor-title").textContent = workflow.id ? "Workflow: " + workflow.name : "Workflow editor";
            document.getElementById("workflow-name").value = workflow.name || "";
            document.getElementById("workflow-description").value = workflow.description || "";
            document.getElementById("workflow-approval-policy").value = workflow.approval_policy || "single_role";
            document.getElementById("workflow-progression-mode").value = workflow.progression_mode || "linear";
            document.getElementById("delete-workflow-button").disabled = !workflow.id;
            const validation = workflow.id ? state.workflowValidation[String(workflow.id)] : null;
            if (els.workflowValidationSummary) {
                if (!workflow.id) {
                    els.workflowValidationSummary.textContent = "";
                } else if (!validation) {
                    els.workflowValidationSummary.textContent = "Validation: not run yet";
                } else {
                    const issues = Array.isArray(validation.issues) ? validation.issues.length : 0;
                    const warnings = Array.isArray(validation.warnings) ? validation.warnings.length : 0;
                    els.workflowValidationSummary.textContent = "Validation: " + (validation.valid ? "valid" : "invalid") +
                        " · issues " + String(issues) + " · warnings " + String(warnings);
                }
            }
        }

        function renderDocumentEditor() {
            const documentDraft = state.selectedDocumentDraft || emptyDocument(state.selectedProjectID);
            document.getElementById("document-editor-title").textContent = documentDraft.id ? "Document: " + (documentDraft.title || "Untitled") : "Document editor";
            document.getElementById("document-title").value = documentDraft.title || "";
            document.getElementById("document-description").value = documentDraft.description || "";
            document.getElementById("document-notes").value = documentDraft.notes || "";
            document.getElementById("document-content").value = documentDraft.content || "";
            document.getElementById("delete-document-button").disabled = !documentDraft.id;

            if (!documentDraft.id) {
                els.documentFilesList.innerHTML = "<div class=\"empty\">Save the document to add files.</div>";
                return;
            }
            if (!state.documentFiles.length) {
                els.documentFilesList.innerHTML = "<div class=\"empty\">No files attached.</div>";
                return;
            }
            els.documentFilesList.innerHTML = state.documentFiles.map((file) => (
                "<div class=\"history-item\">" +
                "<div><strong>" + escapeHTML(file.file_name || ("file " + file.id)) + "</strong></div>" +
                "<div class=\"meta\">" + escapeHTML(String(file.size_bytes || 0)) + " bytes · " + escapeHTML(file.content_type || "application/octet-stream") + "</div>" +
                "<div class=\"entity-actions\">" +
                "<button type=\"button\" data-document-file-download=\"" + file.id + "\">Download</button>" +
                "<button type=\"button\" class=\"btn-danger\" data-document-file-delete=\"" + file.id + "\">Delete</button>" +
                "</div>" +
                "</div>"
            )).join("");
        }

        function renderRoleEditor() {
            const role = state.selectedRoleDraft || emptyRole();
            document.getElementById("role-editor-title").textContent = role.id ? "Role: " + role.title : "Role editor";
            document.getElementById("role-title").value = role.title || "";
            document.getElementById("role-description").value = role.description || "";
            document.getElementById("role-ac").value = role.acceptance_criteria || "";
            document.getElementById("role-workflow").value = role.workflow_id || "";
            document.getElementById("delete-role-button").disabled = !role.id;
        }

        function renderAgentEditor() {
            const agent = getCurrentAgent();
            document.getElementById("agent-editor-title").textContent = agent ? "Agent: " + agent.id : "Agent editor";
            document.getElementById("agent-id").value = agent ? agent.id : "";
            document.getElementById("agent-enabled").value = agent ? String(Boolean(agent.enabled)) : "";
            document.getElementById("agent-new-password").value = "";
            document.getElementById("save-agent-button").disabled = !agent;
            document.getElementById("toggle-agent-button").disabled = !agent;
            document.getElementById("delete-agent-button").disabled = !agent;
        }

        function renderTeamEditor() {
            const team = state.selectedTeamDraft || emptyTeam();
            document.getElementById("team-editor-title").textContent = team.id ? "Team: " + team.name : "Team editor";
            document.getElementById("team-name").value = team.name || "";
            document.getElementById("team-parent").value = team.parent_team_id || "";
            document.getElementById("delete-team-button").disabled = !team.id;
        }

        async function handleLogin(event) {
            event.preventDefault();
            const formData = new FormData(els.loginForm);
            const username = String(formData.get("username") || "").trim();
            const password = String(formData.get("password") || "");
            try {
                const authBody = await apiClient.login(username, password);
                const auth = {
                    username: (authBody.user && authBody.user.username) || username,
                    token: authBody.token,
                };
                state.auth = auth;
                await refreshAll();
                storeAuth(auth);
                showAuthenticatedShell();
                connectLiveUpdates();
            } catch (error) {
                state.auth = null;
                clearStoredAuth();
                els.loginError.textContent = error.message;
            }
        }

        function bindProjectHandlers() {
            els.projectList.addEventListener("click", (event) => {
                const card = event.target.closest("[data-project-id]");
                if (!card) {
                    return;
                }
                state.selectedProjectID = Number(card.dataset.projectId);
                storeSelectedProjectID(state.selectedProjectID);
                const project = getCurrentProject();
                state.selectedProjectDraft = project ? structuredClone(project) : emptyProject();
                renderProjectMenu();
                populateTicketTypeAndStageSelects();
                Promise.all([loadTickets(), loadGoals(), loadDocuments()]).then(renderAll).catch((error) => setNotice(error.message, true));
            });

            document.getElementById("new-project-button").addEventListener("click", () => {
                state.selectedProjectID = null;
                storeSelectedProjectID(state.selectedProjectID);
                state.selectedProjectDraft = emptyProject();
                renderEditors();
            });

            document.getElementById("reset-project-button").addEventListener("click", () => {
                state.selectedProjectDraft = getCurrentProject() ? structuredClone(getCurrentProject()) : emptyProject();
                renderEditors();
            });

            document.getElementById("project-form").addEventListener("submit", async (event) => {
                event.preventDefault();
                const draft = state.selectedProjectDraft;
                const prefixInput = document.getElementById("project-prefix");
                prefixInput.value = prefixInput.value.trim().toUpperCase();
                prefixInput.setCustomValidity("");
                if (!prefixInput.checkValidity()) {
                    prefixInput.reportValidity();
                    return;
                }
                const payload = {
                    prefix: prefixInput.value,
                    title: document.getElementById("project-title").value.trim(),
                    description: document.getElementById("project-description").value.trim(),
                    acceptance_criteria: document.getElementById("project-ac").value.trim(),
                    git_repository: document.getElementById("project-git-repository").value.trim(),
                    visibility: document.getElementById("project-visibility").value,
                    workflow_id: document.getElementById("project-workflow").value ? Number(document.getElementById("project-workflow").value) : null,
                };
                try {
                    const project = normalizeProject(draft.id
                        ? await api("/api/projects/" + draft.id, { method: "PUT", body: JSON.stringify(payload) })
                        : await api("/api/projects", { method: "POST", body: JSON.stringify(payload) }));
                    await api("/api/projects/" + project.id + "/set-draft", {
                        method: "PUT",
                        body: JSON.stringify({ draft: normalizeBool(document.getElementById("project-default-draft").value) }),
                    });
                    state.selectedProjectID = project.id;
                    storeSelectedProjectID(state.selectedProjectID);
                    await Promise.all([loadProjects(), loadWorkflows()]);
                    await Promise.all([loadTickets(), loadGoals(), loadDocuments()]);
                    renderAll();
                    setNotice("Project saved.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            document.getElementById("delete-project-button").addEventListener("click", async () => {
                const draft = state.selectedProjectDraft;
                if (!draft.id) {
                    return;
                }
                try {
                    await api("/api/projects/" + draft.id, { method: "DELETE" });
                    state.selectedProjectID = null;
                    storeSelectedProjectID(state.selectedProjectID);
                    state.selectedProjectDraft = emptyProject();
                    await loadProjects();
                    await Promise.all([loadTickets(), loadGoals(), loadDocuments()]);
                    renderAll();
                    setNotice("Project deleted.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });
        }

        function ensureGoalChatConnected() {
            if (window.__site2MockFetch || state.goalChatSocket) {
                return;
            }
            const scheme = window.location.protocol === "https:" ? "wss:" : "ws:";
            const socket = new WebSocket(scheme + "//" + window.location.host + "/api/chat/ws");
            state.goalChatSocket = socket;
            socket.addEventListener("message", (event) => {
                try {
                    const payload = JSON.parse(event.data);
                    if (!payload || !payload.type) {
                        return;
                    }
                    if (payload.type === "chat_stream" || payload.type === "chat_output") {
                        const text = payload.text || "";
                        state.goalChatMessages.push({ author: "agent", text: text });
                        if (state.selectedGoalID && text) {
                            api("/api/goals/" + state.selectedGoalID + "/chat/messages", {
                                method: "POST",
                                body: JSON.stringify({ author: "agent", text: text }),
                            }).catch(() => {});
                        }
                        renderGoalChat();
                        return;
                    }
                    if (payload.type === "chat_error") {
                        const text = payload.error || "chat error";
                        state.goalChatMessages.push({ author: "system", text: text });
                        if (state.selectedGoalID && text) {
                            api("/api/goals/" + state.selectedGoalID + "/chat/messages", {
                                method: "POST",
                                body: JSON.stringify({ author: "system", text: text }),
                            }).catch(() => {});
                        }
                        renderGoalChat();
                    }
                } catch (error) {
                    // Ignore malformed chat payloads.
                }
            });
            socket.addEventListener("close", () => {
                if (state.goalChatSocket === socket) {
                    state.goalChatSocket = null;
                }
            });
        }

        function sendGoalChatMessage(text) {
            const messageText = (text || "").trim();
            if (!messageText) {
                return;
            }
            if (window.__site2MockFetch) {
                state.goalChatMessages.push({ author: "user", text: messageText });
                if (state.selectedGoalID) {
                    api("/api/goals/" + state.selectedGoalID + "/chat/messages", {
                        method: "POST",
                        body: JSON.stringify({ author: "user", text: messageText }),
                    }).catch(() => {});
                }
                renderGoalChat();
                return;
            }
            ensureGoalChatConnected();
            if (!state.goalChatSocket || state.goalChatSocket.readyState !== WebSocket.OPEN) {
                setTimeout(() => sendGoalChatMessage(messageText), 200);
                return;
            }
            state.goalChatMessages.push({ author: "user", text: messageText });
            if (state.selectedGoalID) {
                api("/api/goals/" + state.selectedGoalID + "/chat/messages", {
                    method: "POST",
                    body: JSON.stringify({ author: "user", text: messageText }),
                }).catch(() => {});
            }
            renderGoalChat();
            state.goalChatSocket.send(JSON.stringify({ type: "chat_input", text: messageText }));
        }

        function syncGoalDecompositionFromTextarea() {
            const text = els.goalDecomposition ? els.goalDecomposition.value : "";
            state.goalDecompositionItems = parseGoalDecompositionItems(text);
            const draft = state.selectedGoalDraft || emptyGoal(state.selectedProjectID);
            draft.decomposition = text;
            state.selectedGoalDraft = draft;
            renderGoalDecompositionList();
        }

        function syncGoalDecompositionToTextarea() {
            const text = formatGoalDecompositionItems(state.goalDecompositionItems || []);
            if (els.goalDecomposition) {
                els.goalDecomposition.value = text;
            }
            const draft = state.selectedGoalDraft || emptyGoal(state.selectedProjectID);
            draft.decomposition = text;
            state.selectedGoalDraft = draft;
            renderGoalDecompositionList();
        }

        function bindGoalsHandlers() {
            els.goalList.addEventListener("click", (event) => {
                const card = event.target.closest("[data-goal-id]");
                if (!card) {
                    return;
                }
                state.selectedGoalID = Number(card.dataset.goalId);
                const goal = getCurrentGoal();
                state.selectedGoalDraft = goal ? structuredClone(normalizeGoal(goal)) : emptyGoal(state.selectedProjectID);
                loadGoalChatMessages().then(renderAll).catch((error) => setNotice(error.message, true));
            });

            if (els.goalDecomposition) {
                els.goalDecomposition.addEventListener("input", () => {
                    syncGoalDecompositionFromTextarea();
                });
            }

            if (els.goalDecompositionList) {
                els.goalDecompositionList.addEventListener("click", (event) => {
                    const upButton = event.target.closest("[data-decomposition-up]");
                    if (upButton) {
                        const index = Number(upButton.dataset.decompositionUp);
                        if (index > 0) {
                            const next = state.goalDecompositionItems.slice();
                            [next[index - 1], next[index]] = [next[index], next[index - 1]];
                            state.goalDecompositionItems = next;
                            syncGoalDecompositionToTextarea();
                        }
                        return;
                    }
                    const downButton = event.target.closest("[data-decomposition-down]");
                    if (downButton) {
                        const index = Number(downButton.dataset.decompositionDown);
                        if (index >= 0 && index < state.goalDecompositionItems.length - 1) {
                            const next = state.goalDecompositionItems.slice();
                            [next[index], next[index + 1]] = [next[index + 1], next[index]];
                            state.goalDecompositionItems = next;
                            syncGoalDecompositionToTextarea();
                        }
                        return;
                    }
                    const deleteButton = event.target.closest("[data-decomposition-delete]");
                    if (deleteButton) {
                        const index = Number(deleteButton.dataset.decompositionDelete);
                        if (index >= 0 && index < state.goalDecompositionItems.length) {
                            state.goalDecompositionItems = state.goalDecompositionItems.filter((_, itemIndex) => itemIndex !== index);
                            syncGoalDecompositionToTextarea();
                        }
                    }
                });
            }

            document.getElementById("goal-decomposition-item-add").addEventListener("click", () => {
                const text = (els.goalDecompositionItemInput ? els.goalDecompositionItemInput.value : "").trim();
                if (!text) {
                    return;
                }
                state.goalDecompositionItems = (state.goalDecompositionItems || []).concat([text]);
                if (els.goalDecompositionItemInput) {
                    els.goalDecompositionItemInput.value = "";
                }
                syncGoalDecompositionToTextarea();
            });

            if (els.goalDecompositionItemInput) {
                els.goalDecompositionItemInput.addEventListener("keydown", (event) => {
                    if (event.key !== "Enter") {
                        return;
                    }
                    event.preventDefault();
                    document.getElementById("goal-decomposition-item-add").click();
                });
            }

            if (els.goalInboxStatusFilter) {
                els.goalInboxStatusFilter.addEventListener("change", () => {
                    state.goalInboxStatusFilter = els.goalInboxStatusFilter.value || "";
                    loadGoals().then(renderAll).catch((error) => setNotice(error.message, true));
                });
            }
            if (els.goalInboxSort) {
                els.goalInboxSort.addEventListener("change", () => {
                    state.goalInboxSort = els.goalInboxSort.value || "updated_desc";
                    loadGoals().then(renderAll).catch((error) => setNotice(error.message, true));
                });
            }

            document.getElementById("new-goal-button").addEventListener("click", () => {
                state.selectedGoalID = null;
                state.selectedGoalDraft = emptyGoal(state.selectedProjectID);
                renderEditors();
            });

            document.getElementById("reset-goal-button").addEventListener("click", () => {
                state.selectedGoalDraft = getCurrentGoal() ? structuredClone(normalizeGoal(getCurrentGoal())) : emptyGoal(state.selectedProjectID);
                renderEditors();
            });

            document.getElementById("goal-form").addEventListener("submit", async (event) => {
                event.preventDefault();
                if (!state.selectedProjectID) {
                    setNotice("Select a project first.", true);
                    return;
                }
                const draft = state.selectedGoalDraft || emptyGoal(state.selectedProjectID);
                const payload = {
                    title: document.getElementById("goal-title").value.trim(),
                    description: document.getElementById("goal-description").value.trim(),
                    notes: document.getElementById("goal-notes").value.trim(),
                    eta: document.getElementById("goal-eta").value.trim(),
                    priority: Number(document.getElementById("goal-priority").value || 1),
                };
                try {
                    const goal = normalizeGoal(draft.id
                        ? await api("/api/goals/" + draft.id, { method: "PUT", body: JSON.stringify(payload) })
                        : await api("/api/projects/" + state.selectedProjectID + "/goals", { method: "POST", body: JSON.stringify(payload) }));
                    state.selectedGoalID = goal.id;
                    await loadGoals();
                    renderAll();
                    setNotice("Goal saved.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            document.getElementById("delete-goal-button").addEventListener("click", async () => {
                const draft = state.selectedGoalDraft || emptyGoal(state.selectedProjectID);
                if (!draft.id) {
                    return;
                }
                try {
                    await api("/api/goals/" + draft.id, { method: "DELETE" });
                    state.selectedGoalID = null;
                    state.selectedGoalDraft = emptyGoal(state.selectedProjectID);
                    await loadGoals();
                    renderAll();
                    setNotice("Goal deleted.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            document.getElementById("refine-goal-button").addEventListener("click", async () => {
                const draft = state.selectedGoalDraft || emptyGoal(state.selectedProjectID);
                if (!draft.id) {
                    return;
                }
                try {
                    const goal = normalizeGoal(await api("/api/goals/" + draft.id + "/refine", { method: "POST", body: JSON.stringify({}) }));
                    state.selectedGoalID = goal.id;
                    await loadGoals();
                    renderAll();
                    const prompt = "Refine this dirty goal into a clean goal and a decomposition.\nReturn sections:\n1) CLEAN GOAL\n2) HIGH-LEVEL OBJECTIVES\n3) PROPOSED SEQUENCE OF WORK\n4) EPICS/STORIES BREAKDOWN\n5) QUESTIONS/ASSUMPTIONS\n\nGoal: " + goal.title + "\nDescription: " + (goal.description || "") + "\nNotes: " + (goal.notes || "");
                    sendGoalChatMessage(prompt);
                    setNotice("Goal moved to refining.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            document.getElementById("ready-goal-button").addEventListener("click", async () => {
                const draft = state.selectedGoalDraft || emptyGoal(state.selectedProjectID);
                if (!draft.id) {
                    return;
                }
                const refinedGoal = (els.goalRefinedGoal.value || "").trim();
                const decomposition = (els.goalDecomposition.value || "").trim();
                if (!refinedGoal || !decomposition) {
                    setNotice("Set a clean goal and decomposition before marking ready.", true);
                    return;
                }
                try {
                    await api("/api/goals/" + draft.id + "/refinement", {
                        method: "PUT",
                        body: JSON.stringify({
                            refined_goal: refinedGoal,
                            decomposition: decomposition,
                        }),
                    });
                    const goal = normalizeGoal(await api("/api/goals/" + draft.id + "/ready", {
                        method: "POST",
                        body: JSON.stringify({ confirm_refinement: true }),
                    }));
                    state.selectedGoalID = goal.id;
                    await loadGoals();
                    renderAll();
                    setNotice("Goal marked ready.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            document.getElementById("save-goal-refinement-button").addEventListener("click", async () => {
                const draft = state.selectedGoalDraft || emptyGoal(state.selectedProjectID);
                if (!draft.id) {
                    return;
                }
                try {
                    const goal = normalizeGoal(await api("/api/goals/" + draft.id + "/refinement", {
                        method: "PUT",
                        body: JSON.stringify({
                            refined_goal: els.goalRefinedGoal.value.trim(),
                            decomposition: els.goalDecomposition.value.trim(),
                        }),
                    }));
                    state.selectedGoalID = goal.id;
                    await loadGoals();
                    renderAll();
                    setNotice("Goal refinement output saved.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            document.getElementById("goal-use-last-agent-response").addEventListener("click", () => {
                const lastAgentMessage = [...state.goalChatMessages].reverse().find((message) => message.author === "agent" && message.text);
                if (!lastAgentMessage) {
                    setNotice("No agent response available yet.", true);
                    return;
                }
                els.goalDecomposition.value = lastAgentMessage.text;
                syncGoalDecompositionFromTextarea();
                if (!els.goalRefinedGoal.value.trim()) {
                    els.goalRefinedGoal.value = "Refined from latest agent response.";
                }
                setNotice("Applied latest agent response to decomposition.");
            });

            document.getElementById("goal-chat-send").addEventListener("click", () => {
                const text = (els.goalChatInput.value || "").trim();
                if (!text) {
                    return;
                }
                els.goalChatInput.value = "";
                sendGoalChatMessage(text);
            });
            els.goalChatInput.addEventListener("keydown", (event) => {
                if (event.key !== "Enter") {
                    return;
                }
                event.preventDefault();
                const text = (els.goalChatInput.value || "").trim();
                if (!text) {
                    return;
                }
                els.goalChatInput.value = "";
                sendGoalChatMessage(text);
            });
        }

        function bindDocumentsHandlers() {
            els.documentList.addEventListener("click", async (event) => {
                const card = event.target.closest("[data-document-id]");
                if (!card) {
                    return;
                }
                state.selectedDocumentID = Number(card.dataset.documentId);
                const documentItem = getCurrentDocument();
                state.selectedDocumentDraft = documentItem ? structuredClone(normalizeDocument(documentItem)) : emptyDocument(state.selectedProjectID);
                await loadDocumentFiles(state.selectedDocumentID);
                renderAll();
            });

            document.getElementById("new-document-button").addEventListener("click", () => {
                state.selectedDocumentID = null;
                state.selectedDocumentDraft = emptyDocument(state.selectedProjectID);
                state.documentFiles = [];
                renderEditors();
            });

            document.getElementById("reset-document-button").addEventListener("click", async () => {
                const documentItem = getCurrentDocument();
                state.selectedDocumentDraft = documentItem ? structuredClone(normalizeDocument(documentItem)) : emptyDocument(state.selectedProjectID);
                if (state.selectedDocumentID) {
                    await loadDocumentFiles(state.selectedDocumentID);
                } else {
                    state.documentFiles = [];
                }
                renderEditors();
            });

            document.getElementById("document-form").addEventListener("submit", async (event) => {
                event.preventDefault();
                if (!state.selectedProjectID) {
                    setNotice("Select a project first.", true);
                    return;
                }
                const draft = state.selectedDocumentDraft || emptyDocument(state.selectedProjectID);
                const payload = {
                    title: document.getElementById("document-title").value.trim(),
                    description: document.getElementById("document-description").value.trim(),
                    notes: document.getElementById("document-notes").value.trim(),
                    content: document.getElementById("document-content").value,
                };
                try {
                    const response = draft.id
                        ? await api("/api/documents/" + draft.id, { method: "PUT", body: JSON.stringify(payload) })
                        : await api("/api/projects/" + state.selectedProjectID + "/documents", { method: "POST", body: JSON.stringify(payload) });
                    const documentItem = normalizeDocument(response);
                    state.selectedDocumentID = documentItem.id;
                    await loadDocuments();
                    renderAll();
                    setNotice("Document saved.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            document.getElementById("delete-document-button").addEventListener("click", async () => {
                const draft = state.selectedDocumentDraft || emptyDocument(state.selectedProjectID);
                if (!draft.id) {
                    return;
                }
                try {
                    await api("/api/documents/" + draft.id, { method: "DELETE" });
                    state.selectedDocumentID = null;
                    state.selectedDocumentDraft = emptyDocument(state.selectedProjectID);
                    state.documentFiles = [];
                    await loadDocuments();
                    renderAll();
                    setNotice("Document deleted.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            document.getElementById("upload-document-file-button").addEventListener("click", async () => {
                const draft = state.selectedDocumentDraft || emptyDocument(state.selectedProjectID);
                if (!draft.id) {
                    setNotice("Save the document before uploading files.", true);
                    return;
                }
                const fileInput = els.documentUploadFile;
                const selectedFile = fileInput && fileInput.files && fileInput.files[0] ? fileInput.files[0] : null;
                if (!selectedFile) {
                    setNotice("Choose a file first.", true);
                    return;
                }
                try {
                    const buffer = await selectedFile.arrayBuffer();
                    const payload = {
                        file_name: (els.documentUploadName.value || "").trim() || selectedFile.name,
                        content_type: selectedFile.type || "application/octet-stream",
                        content: arrayBufferToBase64(buffer),
                    };
                    await api("/api/documents/" + draft.id + "/files", {
                        method: "POST",
                        body: JSON.stringify(payload),
                    });
                    fileInput.value = "";
                    els.documentUploadName.value = "";
                    await loadDocumentFiles(draft.id);
                    renderEditors();
                    setNotice("File uploaded.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            els.documentFilesList.addEventListener("click", async (event) => {
                const downloadButton = event.target.closest("[data-document-file-download]");
                if (downloadButton) {
                    const fileID = Number(downloadButton.dataset.documentFileDownload);
                    const draft = state.selectedDocumentDraft || emptyDocument(state.selectedProjectID);
                    if (!draft.id || !fileID) {
                        return;
                    }
                    try {
                        const downloaded = await apiClient.fetchDocumentFile(draft.id, fileID);
                        const objectURL = URL.createObjectURL(downloaded.blob);
                        const anchor = document.createElement("a");
                        anchor.href = objectURL;
                        anchor.download = downloaded.fileName;
                        document.body.appendChild(anchor);
                        anchor.click();
                        anchor.remove();
                        URL.revokeObjectURL(objectURL);
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                    return;
                }
                const deleteButton = event.target.closest("[data-document-file-delete]");
                if (!deleteButton) {
                    return;
                }
                const fileID = Number(deleteButton.dataset.documentFileDelete);
                const draft = state.selectedDocumentDraft || emptyDocument(state.selectedProjectID);
                if (!draft.id || !fileID) {
                    return;
                }
                try {
                    await api("/api/documents/" + draft.id + "/files/" + fileID, { method: "DELETE" });
                    await loadDocumentFiles(draft.id);
                    renderEditors();
                    setNotice("File removed.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });
        }

        function bindWorkflowHandlers() {
            els.workflowList.addEventListener("click", (event) => {
                const card = event.target.closest("[data-workflow-id]");
                if (!card) {
                    return;
                }
                state.selectedWorkflowID = Number(card.dataset.workflowId);
                state.selectedWorkflowDraft = getCurrentWorkflow() ? structuredClone(getCurrentWorkflow()) : emptyWorkflow();
                loadWorkflowValidation(state.selectedWorkflowID).then(renderAll).catch(() => renderAll());
            });

            document.getElementById("new-workflow-button").addEventListener("click", () => {
                state.selectedWorkflowID = null;
                state.selectedWorkflowDraft = emptyWorkflow();
                renderEditors();
            });

            document.getElementById("reset-workflow-button").addEventListener("click", () => {
                state.selectedWorkflowDraft = getCurrentWorkflow() ? structuredClone(getCurrentWorkflow()) : emptyWorkflow();
                renderEditors();
            });

            document.getElementById("workflow-form").addEventListener("submit", async (event) => {
                event.preventDefault();
                const draft = state.selectedWorkflowDraft;
                const payload = {
                    name: document.getElementById("workflow-name").value.trim(),
                    description: document.getElementById("workflow-description").value.trim(),
                    approval_policy: document.getElementById("workflow-approval-policy").value,
                    progression_mode: document.getElementById("workflow-progression-mode").value,
                };
                try {
                    const workflow = normalizeWorkflow(draft.id
                        ? await api("/api/workflows/" + draft.id, { method: "PUT", body: JSON.stringify(payload) })
                        : await api("/api/workflows", { method: "POST", body: JSON.stringify(payload) }));
                    state.selectedWorkflowID = workflow.id;
                    await Promise.all([loadWorkflows(), loadProjects(), loadRoles()]);
                    renderAll();
                    setNotice("Workflow saved.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            document.getElementById("delete-workflow-button").addEventListener("click", async () => {
                const draft = state.selectedWorkflowDraft;
                if (!draft.id) {
                    return;
                }
                try {
                    await api("/api/workflows/" + draft.id, { method: "DELETE" });
                    state.selectedWorkflowID = null;
                    state.selectedWorkflowDraft = emptyWorkflow();
                    await Promise.all([loadWorkflows(), loadProjects(), loadRoles()]);
                    renderAll();
                    setNotice("Workflow deleted.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            document.getElementById("validate-workflow-button").addEventListener("click", async () => {
                const workflow = getCurrentWorkflow();
                if (!workflow || !workflow.id) {
                    return;
                }
                try {
                    await loadWorkflowValidation(workflow.id, true);
                    renderAll();
                    setNotice("Workflow validation refreshed.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            document.getElementById("auto-chain-transitions-button").addEventListener("click", () => {
                const workflow = getCurrentWorkflow();
                if (!workflow || !Array.isArray(workflow.stages) || workflow.stages.length < 2) {
                    return;
                }
                workflow.stages.forEach((stage, index) => {
                    const select = document.querySelector("[data-stage-next=\"" + stage.id + "\"]");
                    if (!select) {
                        return;
                    }
                    Array.from(select.options).forEach((option) => {
                        option.selected = false;
                    });
                    if (index + 1 < workflow.stages.length) {
                        const nextID = Number(workflow.stages[index + 1].id);
                        const nextOption = Array.from(select.options).find((option) => Number(option.value) === nextID);
                        if (nextOption) {
                            nextOption.selected = true;
                        }
                    }
                });
                setNotice("Auto-chain transitions staged. Click Save all stages.");
            });

            document.getElementById("save-all-stages-button").addEventListener("click", async () => {
                const workflow = getCurrentWorkflow();
                if (!workflow || !Array.isArray(workflow.stages) || !workflow.stages.length) {
                    return;
                }
                try {
                    for (const stage of workflow.stages) {
                        await saveStage(stage.id);
                    }
                    await loadWorkflowValidation(workflow.id, true);
                    renderAll();
                    setNotice("All stages saved.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            document.getElementById("new-stage-form").addEventListener("submit", async (event) => {
                event.preventDefault();
                const workflow = getCurrentWorkflow();
                if (!workflow) {
                    setNotice("Select an Workflow first.", true);
                    return;
                }
                try {
                    await api("/api/workflows/" + workflow.id + "/stages", {
                        method: "POST",
                        body: JSON.stringify({
                            stage_name: document.getElementById("new-stage-name").value.trim(),
                            wow: document.getElementById("new-stage-wow").value.trim(),
                            dor: document.getElementById("new-stage-dor").value.trim(),
                            dod: document.getElementById("new-stage-dod").value.trim(),
                            sort_order: Number(document.getElementById("new-stage-sort-order").value || 0),
                        }),
                    });
                    document.getElementById("new-stage-form").reset();
                    await loadWorkflows();
                    renderAll();
                    setNotice("Stage added.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            els.stageGrid.addEventListener("click", async (event) => {
                const saveButton = event.target.closest("[data-save-stage]");
                if (saveButton) {
                    const stageID = Number(saveButton.dataset.saveStage);
                    await saveStage(stageID);
                    return;
                }
                const deleteButton = event.target.closest("[data-delete-stage]");
                if (deleteButton) {
                    const stageID = Number(deleteButton.dataset.deleteStage);
                    try {
                        await api("/api/workflows/stages/" + stageID, { method: "DELETE" });
                        await loadWorkflows();
                        renderAll();
                        setNotice("Stage deleted.");
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                    return;
                }
                const addRoleButton = event.target.closest("[data-add-role]");
                if (addRoleButton) {
                    const stageID = Number(addRoleButton.dataset.addRole);
                    const workflow = getCurrentWorkflow();
                    const select = document.querySelector("[data-add-role-select=\"" + stageID + "\"]");
                    if (!workflow || !select || !select.value) {
                        return;
                    }
                    try {
                        await api("/api/workflows/stages/roles/" + workflow.id + "/" + stageID, {
                            method: "POST",
                            body: JSON.stringify({ role_id: Number(select.value) }),
                        });
                        await loadWorkflows();
                        renderAll();
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                }
            });

            bindStageDragAndDrop();
        }

        async function loadWorkflowValidation(workflowID, force) {
            const key = String(workflowID || "");
            if (!key) {
                return null;
            }
            if (!force && state.workflowValidation[key]) {
                return state.workflowValidation[key];
            }
            const report = await api("/api/workflows/" + workflowID + "/validate");
            state.workflowValidation[key] = report;
            return report;
        }

        async function saveStage(stageID) {
            try {
                await api("/api/workflows/stages/" + stageID, {
                    method: "PUT",
                    body: JSON.stringify({
                        stage_name: document.querySelector("[data-stage-name=\"" + stageID + "\"]").value.trim(),
                        wow: document.querySelector("[data-stage-wow=\"" + stageID + "\"]").value.trim(),
                        dor: document.querySelector("[data-stage-dor=\"" + stageID + "\"]").value.trim(),
                        dod: document.querySelector("[data-stage-dod=\"" + stageID + "\"]").value.trim(),
                    }),
                });
                const selectedTransitions = Array.from(document.querySelectorAll("[data-stage-next=\"" + stageID + "\"] option:checked"))
                    .map((option) => Number(option.value))
                    .filter((id) => !Number.isNaN(id) && id > 0);
                await api("/api/workflows/stages/" + stageID + "/transitions", {
                    method: "PUT",
                    body: JSON.stringify({ to_stage_ids: selectedTransitions }),
                });
                await loadWorkflows();
                renderAll();
                setNotice("Stage saved.");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        function bindRolesHandlers() {
            els.roleList.addEventListener("click", (event) => {
                const card = event.target.closest("[data-role-id]");
                if (!card) {
                    return;
                }
                state.selectedRoleID = Number(card.dataset.roleId);
                state.selectedRoleDraft = getCurrentRole() ? structuredClone(getCurrentRole()) : emptyRole();
                renderAll();
            });

            document.getElementById("new-role-button").addEventListener("click", () => {
                state.selectedRoleID = null;
                state.selectedRoleDraft = emptyRole();
                renderEditors();
            });

            document.getElementById("reset-role-button").addEventListener("click", () => {
                state.selectedRoleDraft = getCurrentRole() ? structuredClone(getCurrentRole()) : emptyRole();
                renderEditors();
            });

            document.getElementById("role-form").addEventListener("submit", async (event) => {
                event.preventDefault();
                const draft = state.selectedRoleDraft;
                const payload = {
                    title: document.getElementById("role-title").value.trim(),
                    description: document.getElementById("role-description").value.trim(),
                    acceptance_criteria: document.getElementById("role-ac").value.trim(),
                    workflow_id: document.getElementById("role-workflow").value ? Number(document.getElementById("role-workflow").value) : null,
                };
                try {
                    const role = normalizeRole(draft.id
                        ? await api("/api/roles/" + draft.id, { method: "PUT", body: JSON.stringify(payload) })
                        : await api("/api/roles", { method: "POST", body: JSON.stringify(payload) }));
                    state.selectedRoleID = role.id;
                    await Promise.all([loadRoles(), loadWorkflows()]);
                    renderAll();
                    setNotice("Role saved.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            document.getElementById("delete-role-button").addEventListener("click", async () => {
                const draft = state.selectedRoleDraft;
                if (!draft.id) {
                    return;
                }
                try {
                    await api("/api/roles/" + draft.id, { method: "DELETE" });
                    state.selectedRoleID = null;
                    state.selectedRoleDraft = emptyRole();
                    await Promise.all([loadRoles(), loadWorkflows()]);
                    renderAll();
                    setNotice("Role deleted.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });
        }

        function bindAgentsHandlers() {
            els.agentList.addEventListener("click", (event) => {
                const card = event.target.closest("[data-agent-id]");
                if (!card) {
                    return;
                }
                state.selectedAgentID = card.dataset.agentId;
                renderEditors();
            });

            document.getElementById("new-agent-button").addEventListener("click", async () => {
                const password = prompt("Optional password for the new agent", "");
                if (password === null) {
                    return;
                }
                try {
                    const created = normalizeAgent(await api("/api/agents", { method: "POST", body: JSON.stringify({ password: password || "" }) }));
                    await loadAgents();
                    state.selectedAgentID = created.id;
                    renderAll();
                    setNotice("Agent created. Password: " + (created.password || "(generated server-side and not returned)"));
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            document.getElementById("agent-form").addEventListener("submit", async (event) => {
                event.preventDefault();
                const agent = getCurrentAgent();
                if (!agent) {
                    return;
                }
                try {
                    await api("/api/agents/" + agent.id, {
                        method: "PUT",
                        body: JSON.stringify({ password: document.getElementById("agent-new-password").value }),
                    });
                    await loadAgents();
                    renderAll();
                    setNotice("Agent password rotated.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            document.getElementById("toggle-agent-button").addEventListener("click", async () => {
                const agent = getCurrentAgent();
                if (!agent) {
                    return;
                }
                try {
                    await api("/api/agents/" + agent.id + "/" + (agent.enabled ? "disable" : "enable"), { method: "POST" });
                    await loadAgents();
                    renderAll();
                    setNotice("Agent updated.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            document.getElementById("delete-agent-button").addEventListener("click", async () => {
                const agent = getCurrentAgent();
                if (!agent) {
                    return;
                }
                try {
                    await api("/api/agents/" + agent.id, { method: "DELETE" });
                    state.selectedAgentID = null;
                    await loadAgents();
                    renderAll();
                    setNotice("Agent deleted.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });
        }

        function bindTeamsHandlers() {
            els.teamList.addEventListener("click", (event) => {
                const card = event.target.closest("[data-team-id]");
                if (!card) {
                    return;
                }
                state.selectedTeamID = Number(card.dataset.teamId);
                state.selectedTeamDraft = getCurrentTeam() ? structuredClone(getCurrentTeam()) : emptyTeam();
                renderAll();
            });

            document.getElementById("new-team-button").addEventListener("click", () => {
                state.selectedTeamID = null;
                state.selectedTeamDraft = emptyTeam();
                renderEditors();
            });

            document.getElementById("reset-team-button").addEventListener("click", () => {
                state.selectedTeamDraft = getCurrentTeam() ? structuredClone(getCurrentTeam()) : emptyTeam();
                renderAll();
            });

            document.getElementById("team-form").addEventListener("submit", async (event) => {
                event.preventDefault();
                const draft = state.selectedTeamDraft;
                const payload = {
                    name: document.getElementById("team-name").value.trim(),
                    parent_team_id: document.getElementById("team-parent").value ? Number(document.getElementById("team-parent").value) : null,
                };
                try {
                    const team = normalizeTeam(draft.id
                        ? await api("/api/teams/" + draft.id, { method: "PUT", body: JSON.stringify(payload) })
                        : await api("/api/teams", { method: "POST", body: JSON.stringify(payload) }));
                    state.selectedTeamID = team.id;
                    await loadTeams();
                    renderAll();
                    setNotice("Team saved.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            document.getElementById("delete-team-button").addEventListener("click", async () => {
                const draft = state.selectedTeamDraft;
                if (!draft.id) {
                    return;
                }
                try {
                    await api("/api/teams/" + draft.id, { method: "DELETE" });
                    state.selectedTeamID = null;
                    state.selectedTeamDraft = emptyTeam();
                    await loadTeams();
                    renderAll();
                    setNotice("Team deleted.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });
        }

        function bindTicketsHandlers() {
            document.getElementById("new-ticket-button").addEventListener("click", () => openTicketModal(emptyTicket()));
            if (els.boardSearch) {
                els.boardSearch.addEventListener("input", () => renderTicketBoard());
            }
            if (els.boardHideDone) {
                els.boardHideDone.addEventListener("change", () => renderTicketBoard());
            }
            document.getElementById("close-ticket-modal").addEventListener("click", closeTicketModal);
            els.ticketModal.addEventListener("click", (event) => {
                if (event.target === els.ticketModal) {
                    closeTicketModal();
                }
            });

            els.projectMenuButton.addEventListener("click", (event) => {
                event.stopPropagation();
                els.accountMenuDropdown.classList.remove("open");
                els.accountMenuButton.setAttribute("aria-expanded", "false");
                const isOpen = els.projectMenuDropdown.classList.toggle("open");
                els.projectMenuButton.setAttribute("aria-expanded", isOpen ? "true" : "false");
            });

            els.projectMenuDropdown.addEventListener("click", async (event) => {
                const projectButton = event.target.closest("[data-project-switch]");
                if (projectButton) {
                    try {
                        await selectProject(projectButton.dataset.projectSwitch);
                        els.projectMenuDropdown.classList.remove("open");
                        els.projectMenuButton.setAttribute("aria-expanded", "false");
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                    return;
                }

                if (event.target.closest("#project-create-link")) {
                    els.projectMenuDropdown.classList.remove("open");
                    els.projectMenuButton.setAttribute("aria-expanded", "false");
                    state.selectedProjectID = null;
                    storeSelectedProjectID(state.selectedProjectID);
                    state.selectedProjectDraft = emptyProject();
                    switchView("projects");
                    renderAll();
                }
            });

            els.ticketBoard.addEventListener("click", (event) => {
                const card = event.target.closest("[data-ticket-id]");
                if (!card) {
                    return;
                }
                const ticket = state.tickets.find((item) => String(item.id) === card.dataset.ticketId);
                if (ticket) {
                    openTicketModal(ticket);
                }
            });
            els.interventionList.addEventListener("click", (event) => {
                const button = event.target.closest("[data-open-intervention-ticket]");
                if (button) {
                    const ticket = state.interventions.find((item) => String(item.id) === button.dataset.openInterventionTicket) ||
                        state.tickets.find((item) => String(item.id) === button.dataset.openInterventionTicket);
                    if (ticket) {
                        openTicketModal(ticket);
                    }
                    return;
                }
                const stateButton = event.target.closest("[data-save-intervention-state]");
                if (stateButton) {
                    const ticketID = stateButton.dataset.saveInterventionState;
                    const stateInput = els.interventionList.querySelector("[data-intervention-state=\"" + ticketID + "\"]");
                    const nextState = stateInput ? String(stateInput.value || "").trim() : "";
                    if (!nextState) {
                        setNotice("Select an intervention mailbox state.", true);
                        return;
                    }
                    api("/api/tickets/" + ticketID + "/intervention-state", {
                        method: "POST",
                        body: JSON.stringify({ state: nextState }),
                    }).then(async (updatedState) => {
                        state.interventionStates[String(ticketID)] = updatedState;
                        setNotice("Intervention mailbox state updated.");
                        await loadTickets();
                        renderInterventions();
                    }).catch((error) => setNotice(error.message, true));
                    return;
                }
                const retryButton = event.target.closest("[data-retry-intervention-ticket]");
                if (retryButton) {
                    const ticketID = retryButton.dataset.retryInterventionTicket;
                    interveneTicket(ticketID, "retry-role", "Quick retry from interventions board.").catch((error) => setNotice(error.message, true));
                    return;
                }
                const cancelButton = event.target.closest("[data-cancel-intervention-ticket]");
                if (cancelButton) {
                    const ticketID = cancelButton.dataset.cancelInterventionTicket;
                    interveneTicket(ticketID, "cancel", "Quick cancel from interventions board.").catch((error) => setNotice(error.message, true));
                    return;
                }
                const commentButton = event.target.closest("[data-add-intervention-comment]");
                if (commentButton) {
                    const ticketID = commentButton.dataset.addInterventionComment;
                    const commentInput = els.interventionList.querySelector("[data-intervention-comment=\"" + ticketID + "\"]");
                    const comment = commentInput ? String(commentInput.value || "").trim() : "";
                    if (!comment) {
                        setNotice("Intervention comment is required.", true);
                        return;
                    }
                    api("/api/tickets/" + ticketID + "/comments", {
                        method: "POST",
                        body: JSON.stringify({ comment: comment }),
                    }).then(async () => {
                        if (commentInput) {
                            commentInput.value = "";
                        }
                        await loadTickets();
                        renderAll();
                        setNotice("Intervention comment added.");
                    }).catch((error) => setNotice(error.message, true));
                    return;
                }
                const applyButton = event.target.closest("[data-apply-intervention-ticket]");
                if (!applyButton) {
                    return;
                }
                const ticketID = applyButton.dataset.applyInterventionTicket;
                const outcomeInput = els.interventionList.querySelector("[data-intervention-outcome=\"" + ticketID + "\"]");
                const messageInput = els.interventionList.querySelector("[data-intervention-message=\"" + ticketID + "\"]");
                const outcome = outcomeInput ? outcomeInput.value : "";
                const message = messageInput ? messageInput.value : "";
                interveneTicket(ticketID, outcome, message).catch((error) => setNotice(error.message, true));
            });
            if (els.interventionFilter) {
                els.interventionFilter.addEventListener("change", () => renderInterventions());
            }
            if (els.interventionSort) {
                els.interventionSort.addEventListener("change", () => renderInterventions());
            }

            document.getElementById("ticket-form").addEventListener("submit", async (event) => {
                event.preventDefault();
                await saveActiveTicket();
            });

            document.getElementById("delete-ticket-button").addEventListener("click", async () => {
                if (!state.activeTicket || !state.activeTicket.id) {
                    closeTicketModal();
                    return;
                }
                try {
                    await api("/api/tickets/" + state.activeTicket.id, { method: "DELETE" });
                    closeTicketModal();
                    await loadTickets();
                    renderTicketBoard();
                    setNotice("Ticket deleted.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            document.getElementById("ticket-open-button").addEventListener("click", () => ticketAction("open"));
            document.getElementById("ticket-close-button").addEventListener("click", () => ticketAction("close"));
            document.getElementById("ticket-archive-button").addEventListener("click", () => ticketAction("archive"));
            document.getElementById("ticket-unarchive-button").addEventListener("click", () => ticketAction("unarchive"));
            els.addTicketCommentButton.addEventListener("click", () => {
                addTicketComment();
            });
            els.addTicketLabelButton.addEventListener("click", () => {
                addTicketLabel();
            });
            els.ticketLabels.addEventListener("click", (event) => {
                const button = event.target.closest("[data-remove-ticket-label]");
                if (!button) {
                    return;
                }
                removeTicketLabel(button.dataset.removeTicketLabel);
            });
            els.addTicketDependencyButton.addEventListener("click", () => {
                addTicketDependency();
            });
            els.ticketDependencies.addEventListener("click", (event) => {
                const button = event.target.closest("[data-remove-ticket-dependency]");
                if (!button) {
                    return;
                }
                removeTicketDependency(button.dataset.removeTicketDependency);
            });
            els.addTicketTimeButton.addEventListener("click", () => {
                addTicketTimeEntry();
            });

            bindTicketBoardDragAndDrop();
        }

        function openTicketModal(ticket) {
            state.activeTicket = structuredClone(ticket);
            document.getElementById("ticket-modal-title").textContent = ticket.id ? "Ticket " + (ticket.key || ticket.id) : "New ticket";
            populateTicketTypeAndStageSelects();
            document.getElementById("ticket-type").value = ticket.type || "task";
            document.getElementById("ticket-status").value = ticket.status || "open";
            document.getElementById("ticket-stage").value = ticket.stage || getStageOptions()[0];
            document.getElementById("ticket-title").value = ticket.title || "";
            document.getElementById("ticket-description").value = ticket.description || "";
            document.getElementById("ticket-ac").value = ticket.acceptance_criteria || "";
            document.getElementById("ticket-parent").value = ticket.parent_id || "";
            document.getElementById("ticket-workflow").value = ticket.workflow_id || "";
            document.getElementById("ticket-draft").value = String(Boolean(ticket.draft));
            document.getElementById("ticket-priority").value = ticket.priority || 0;
            document.getElementById("ticket-order").value = ticket.order || 0;
            document.getElementById("ticket-estimate-effort").value = ticket.estimate_effort || 0;
            document.getElementById("ticket-health").value = ticket.health || 0;
            document.getElementById("delete-ticket-button").disabled = !ticket.id;
            els.ticketCommentInput.value = "";
            els.ticketModal.classList.add("open");
            loadTicketHistory(ticket.id).catch((error) => {
                els.ticketHistory.innerHTML = "<div class=\"empty\">" + escapeHTML(error.message) + "</div>";
            });
            loadTicketComments(ticket.id).catch((error) => {
                els.ticketComments.innerHTML = "<div class=\"empty\">" + escapeHTML(error.message) + "</div>";
            });
            loadProjectLabels(ticket.project_id || state.selectedProjectID).catch((error) => {
                els.ticketLabels.innerHTML = "<div class=\"empty\">" + escapeHTML(error.message) + "</div>";
            });
            loadTicketLabels(ticket.id).catch((error) => {
                els.ticketLabels.innerHTML = "<div class=\"empty\">" + escapeHTML(error.message) + "</div>";
            });
            loadTicketDependencies(ticket.id).catch((error) => {
                els.ticketDependencies.innerHTML = "<div class=\"empty\">" + escapeHTML(error.message) + "</div>";
            });
            loadTicketTime(ticket.id).catch((error) => {
                els.ticketTimeEntries.innerHTML = "<div class=\"empty\">" + escapeHTML(error.message) + "</div>";
            });
        }

        function closeTicketModal() {
            state.activeTicket = null;
            state.ticketHistory = [];
            state.ticketComments = [];
            state.ticketLabels = [];
            state.projectLabels = [];
            state.ticketDependencies = [];
            state.ticketTimeEntries = [];
            state.ticketTimeTotal = 0;
            els.ticketCommentInput.value = "";
            els.ticketDependencyInput.value = "";
            els.ticketTimeMinutes.value = "30";
            els.ticketTimeNote.value = "";
            els.ticketModal.classList.remove("open");
        }

        async function loadTicketHistory(ticketID) {
            if (!ticketID) {
                els.ticketHistory.innerHTML = "<div class=\"empty\">History appears after the first save.</div>";
                return;
            }
            state.ticketHistory = await api("/api/tickets/" + ticketID + "/history");
            if (!Array.isArray(state.ticketHistory) || !state.ticketHistory.length) {
                els.ticketHistory.innerHTML = "<div class=\"empty\">No history yet.</div>";
                return;
            }
            els.ticketHistory.innerHTML = state.ticketHistory.map((item) => {
                return "<div class=\"history-item\"><strong>" + escapeHTML(item.action || item.type || "event") + "</strong><div class=\"meta\">" +
                    escapeHTML(item.created_at || item.timestamp || "") + "</div><div class=\"meta\">" +
                    escapeHTML(item.comment || item.message || "") + "</div></div>";
            }).join("");
        }

        function renderTicketComments() {
            if (!Array.isArray(state.ticketComments) || !state.ticketComments.length) {
                els.ticketComments.innerHTML = "<div class=\"empty\">No comments yet.</div>";
                return;
            }
            els.ticketComments.innerHTML = state.ticketComments.map((item) => {
                return "<div class=\"history-item\"><strong>" + escapeHTML(item.author || "user") + "</strong><div class=\"meta\">" +
                    escapeHTML(item.date || item.created_at || "") + "</div><div class=\"meta\">" +
                    escapeHTML(item.text || item.comment || "") + "</div></div>";
            }).join("");
        }

        async function loadTicketComments(ticketID) {
            if (!ticketID) {
                els.ticketComments.innerHTML = "<div class=\"empty\">Comments appear after the first save.</div>";
                return;
            }
            state.ticketComments = await api("/api/tickets/" + ticketID + "/comments");
            renderTicketComments();
        }

        function renderTicketLabels() {
            if (!Array.isArray(state.ticketLabels) || !state.ticketLabels.length) {
                els.ticketLabels.innerHTML = "<div class=\"empty\">No labels yet.</div>";
            } else {
                els.ticketLabels.innerHTML = state.ticketLabels.map((label) => {
                    return "<div class=\"history-item\"><strong>" + escapeHTML(label.name || "") + "</strong><div class=\"meta\">" +
                        escapeHTML(label.color || "") + "</div><button type=\"button\" data-remove-ticket-label=\"" + String(label.label_id || label.id) + "\">Remove</button></div>";
                }).join("");
            }
            const options = (state.projectLabels || []).map((label) => {
                const labelID = label.label_id || label.id;
                return "<option value=\"" + String(labelID) + "\">" + escapeHTML((label.name || "") + " " + (label.color || "")) + "</option>";
            }).join("");
            els.ticketLabelSelect.innerHTML = "<option value=\"\">Choose label</option>" + options;
        }

        async function loadProjectLabels(projectID) {
            if (!projectID) {
                state.projectLabels = [];
                renderTicketLabels();
                return;
            }
            state.projectLabels = await api("/api/projects/" + projectID + "/labels");
            renderTicketLabels();
        }

        async function loadTicketLabels(ticketID) {
            if (!ticketID) {
                state.ticketLabels = [];
                renderTicketLabels();
                return;
            }
            state.ticketLabels = await api("/api/tickets/" + ticketID + "/labels");
            renderTicketLabels();
        }

        async function addTicketLabel() {
            if (!state.activeTicket || !state.activeTicket.id) {
                return;
            }
            const labelID = Number(els.ticketLabelSelect.value || 0);
            if (!labelID) {
                setNotice("Choose a label first.", true);
                return;
            }
            try {
                await api("/api/tickets/" + state.activeTicket.id + "/labels", {
                    method: "POST",
                    body: JSON.stringify({ label_id: labelID }),
                });
                await loadTicketLabels(state.activeTicket.id);
                setNotice("Label added.");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        async function removeTicketLabel(labelID) {
            if (!state.activeTicket || !state.activeTicket.id) {
                return;
            }
            try {
                await api("/api/tickets/" + state.activeTicket.id + "/labels/" + labelID, { method: "DELETE" });
                await loadTicketLabels(state.activeTicket.id);
                setNotice("Label removed.");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        function renderTicketDependencies() {
            if (!Array.isArray(state.ticketDependencies) || !state.ticketDependencies.length) {
                els.ticketDependencies.innerHTML = "<div class=\"empty\">No dependencies yet.</div>";
                return;
            }
            els.ticketDependencies.innerHTML = state.ticketDependencies.map((dep) => {
                return "<div class=\"history-item\"><strong>" + escapeHTML(dep.depends_on || "") + "</strong><div class=\"meta\">" +
                    escapeHTML(dep.created_at || "") + "</div><button type=\"button\" data-remove-ticket-dependency=\"" + escapeHTML(dep.depends_on || "") + "\">Remove</button></div>";
            }).join("");
        }

        async function loadTicketDependencies(ticketID) {
            if (!ticketID) {
                state.ticketDependencies = [];
                renderTicketDependencies();
                return;
            }
            state.ticketDependencies = await api("/api/tickets/" + ticketID + "/dependencies");
            renderTicketDependencies();
        }

        async function addTicketDependency() {
            if (!state.activeTicket || !state.activeTicket.id) {
                return;
            }
            const dependsOn = els.ticketDependencyInput.value.trim();
            if (!dependsOn) {
                setNotice("Dependency ticket is required.", true);
                return;
            }
            try {
                await api("/api/dependencies", {
                    method: "POST",
                    body: JSON.stringify({
                        project_id: state.activeTicket.project_id || state.selectedProjectID,
                        ticket_id: state.activeTicket.id,
                        depends_on: dependsOn,
                    }),
                });
                els.ticketDependencyInput.value = "";
                await loadTicketDependencies(state.activeTicket.id);
                setNotice("Dependency added.");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        async function removeTicketDependency(dependsOn) {
            if (!state.activeTicket || !state.activeTicket.id) {
                return;
            }
            try {
                const query = new URLSearchParams({
                    project_id: String(state.activeTicket.project_id || state.selectedProjectID),
                    ticket_id: String(state.activeTicket.id),
                    depends_on: String(dependsOn),
                });
                await api("/api/dependencies?" + query.toString(), { method: "DELETE" });
                await loadTicketDependencies(state.activeTicket.id);
                setNotice("Dependency removed.");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        function renderTicketTimeEntries() {
            els.ticketTimeTotal.textContent = "Total minutes: " + String(state.ticketTimeTotal || 0);
            if (!Array.isArray(state.ticketTimeEntries) || !state.ticketTimeEntries.length) {
                els.ticketTimeEntries.innerHTML = "<div class=\"empty\">No time entries yet.</div>";
                return;
            }
            els.ticketTimeEntries.innerHTML = state.ticketTimeEntries.map((entry) => {
                return "<div class=\"history-item\"><strong>" + escapeHTML(String(entry.minutes || 0) + "m") + "</strong><div class=\"meta\">" +
                    escapeHTML(entry.note || "") + "</div><div class=\"meta\">" +
                    escapeHTML(entry.created_at || "") + "</div></div>";
            }).join("");
        }

        async function loadTicketTime(ticketID) {
            if (!ticketID) {
                state.ticketTimeEntries = [];
                state.ticketTimeTotal = 0;
                renderTicketTimeEntries();
                return;
            }
            const [entries, total] = await Promise.all([
                api("/api/tickets/" + ticketID + "/time"),
                api("/api/tickets/" + ticketID + "/time/total"),
            ]);
            state.ticketTimeEntries = entries;
            state.ticketTimeTotal = Number((total && total.total) || 0);
            renderTicketTimeEntries();
        }

        async function addTicketTimeEntry() {
            if (!state.activeTicket || !state.activeTicket.id) {
                return;
            }
            const minutes = Number(els.ticketTimeMinutes.value || 0);
            const note = els.ticketTimeNote.value.trim();
            if (!minutes || minutes < 1) {
                setNotice("Minutes must be positive.", true);
                return;
            }
            try {
                await api("/api/tickets/" + state.activeTicket.id + "/time", {
                    method: "POST",
                    body: JSON.stringify({ minutes: minutes, note: note }),
                });
                els.ticketTimeNote.value = "";
                await loadTicketTime(state.activeTicket.id);
                setNotice("Time logged.");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        async function addTicketComment() {
            if (!state.activeTicket || !state.activeTicket.id) {
                return;
            }
            const comment = els.ticketCommentInput.value.trim();
            if (!comment) {
                setNotice("Comment cannot be empty.", true);
                return;
            }
            try {
                await api("/api/tickets/" + state.activeTicket.id + "/comments", {
                    method: "POST",
                    body: JSON.stringify({ comment: comment }),
                });
                els.ticketCommentInput.value = "";
                await Promise.all([loadTicketComments(state.activeTicket.id), loadTicketHistory(state.activeTicket.id)]);
                setNotice("Comment added.");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        async function saveActiveTicket() {
            const payload = {
                project_id: state.selectedProjectID,
                type: document.getElementById("ticket-type").value,
                title: document.getElementById("ticket-title").value.trim(),
                description: document.getElementById("ticket-description").value.trim(),
                acceptance_criteria: document.getElementById("ticket-ac").value.trim(),
                parent_id: document.getElementById("ticket-parent").value.trim() || null,
                status: document.getElementById("ticket-status").value,
                stage: document.getElementById("ticket-stage").value,
                priority: Number(document.getElementById("ticket-priority").value || 0),
                order: Number(document.getElementById("ticket-order").value || 0),
                estimate_effort: Number(document.getElementById("ticket-estimate-effort").value || 0),
                health: Number(document.getElementById("ticket-health").value || 0),
            };
            try {
                const ticket = normalizeTicket(state.activeTicket && state.activeTicket.id
                    ? await api("/api/tickets/" + state.activeTicket.id, { method: "PUT", body: JSON.stringify(payload) })
                    : await api("/api/tickets", { method: "POST", body: JSON.stringify(payload) }));

                const wantsDraft = normalizeBool(document.getElementById("ticket-draft").value);
                if (wantsDraft !== Boolean(ticket.draft)) {
                    await api("/api/tickets/" + ticket.id + "/" + (wantsDraft ? "draft" : "undraft"), { method: "POST" });
                }

                const workflowValue = document.getElementById("ticket-workflow").value;
                if (workflowValue) {
                    await api("/api/tickets/" + ticket.id + "/workflow", {
                        method: "POST",
                        body: JSON.stringify({ workflow_id: Number(workflowValue) }),
                    });
                } else if (ticket.workflow_id) {
                    await api("/api/tickets/" + ticket.id + "/workflow", { method: "DELETE" });
                }

                closeTicketModal();
                await loadTickets();
                renderTicketBoard();
                setNotice("Ticket saved.");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        async function ticketAction(action) {
            if (!state.activeTicket || !state.activeTicket.id) {
                return;
            }
            try {
                await api("/api/tickets/" + state.activeTicket.id + "/" + action, { method: "POST", body: JSON.stringify({}) });
                await loadTickets();
                const updated = state.tickets.find((ticket) => ticket.id === state.activeTicket.id);
                renderTicketBoard();
                if (updated) {
                    openTicketModal(updated);
                }
                setNotice("Ticket updated.");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        async function interveneTicket(ticketID, outcome, message) {
            if (!ticketID || !outcome) {
                setNotice("Select an intervention decision.", true);
                return;
            }
            try {
                await api("/api/tickets/" + ticketID + "/intervene", {
                    method: "POST",
                    body: JSON.stringify({
                        outcome,
                        message,
                    }),
                });
                await loadTickets();
                renderTicketBoard();
                renderInterventions();
                setNotice("Intervention decision applied.");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        function bindTicketBoardDragAndDrop() {
            els.ticketBoard.addEventListener("dragstart", (event) => {
                const card = event.target.closest("[data-ticket-id]");
                if (!card) {
                    const lane = event.target.closest("[data-workflow-stage-id]");
                    if (!lane) {
                        return;
                    }
                    state.drag = {
                        type: "board-stage",
                        workflowStageID: Number(lane.dataset.workflowStageId),
                    };
                    event.dataTransfer.effectAllowed = "move";
                    return;
                }
                state.drag = { type: "ticket", ticketID: card.dataset.ticketId };
                event.dataTransfer.effectAllowed = "move";
            });

            els.ticketBoard.addEventListener("dragover", (event) => {
                const lane = event.target.closest("[data-lane-stage]");
                if (!lane || !state.drag) {
                    return;
                }
                if (state.drag.type === "ticket") {
                    event.preventDefault();
                    document.querySelectorAll(".lane").forEach((item) => item.classList.remove("drag-target"));
                    lane.classList.add("drag-target");
                    return;
                }
                if (state.drag.type === "board-stage" && lane.dataset.workflowStageId) {
                    event.preventDefault();
                    document.querySelectorAll(".lane").forEach((item) => item.classList.remove("drag-target"));
                    lane.classList.add("drag-target");
                }
            });

            els.ticketBoard.addEventListener("dragleave", (event) => {
                const lane = event.target.closest("[data-lane-stage]");
                if (lane) {
                    lane.classList.remove("drag-target");
                }
            });

            els.ticketBoard.addEventListener("drop", async (event) => {
                const lane = event.target.closest("[data-lane-stage]");
                if (!lane || !state.drag) {
                    return;
                }
                if (state.drag.type === "ticket") {
                    event.preventDefault();
                    lane.classList.remove("drag-target");
                    const ticket = state.tickets.find((item) => String(item.id) === state.drag.ticketID);
                    state.drag = null;
                    if (!ticket || ticket.stage === lane.dataset.laneStage) {
                        return;
                    }
                    try {
                        await api("/api/tickets/" + ticket.id, {
                            method: "PUT",
                            body: JSON.stringify(Object.assign({}, ticket, { stage: lane.dataset.laneStage })),
                        });
                        await loadTickets();
                        renderTicketBoard();
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                    return;
                }
                if (state.drag.type === "board-stage" && lane.dataset.workflowStageId) {
                    event.preventDefault();
                    lane.classList.remove("drag-target");
                    const workflow = getCurrentProjectWorkflow();
                    const targetStageID = Number(lane.dataset.workflowStageId);
                    if (!workflow || !targetStageID || targetStageID === state.drag.workflowStageID) {
                        state.drag = null;
                        return;
                    }
                    const ordered = Array.from(els.ticketBoard.querySelectorAll("[data-workflow-stage-id]"))
                        .map((item) => Number(item.dataset.workflowStageId))
                        .filter((stageID) => stageID !== state.drag.workflowStageID);
                    const targetIndex = ordered.indexOf(targetStageID);
                    ordered.splice(targetIndex >= 0 ? targetIndex : ordered.length, 0, state.drag.workflowStageID);
                    state.drag = null;
                    try {
                        await api("/api/workflows/" + workflow.id + "/reorder", {
                            method: "PUT",
                            body: JSON.stringify({ stage_ids: ordered }),
                        });
                        await loadWorkflows();
                        await loadTickets();
                        renderAll();
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                }
            });
        }

        function bindStageDragAndDrop() {
            els.workflowRoleBank.addEventListener("dragstart", (event) => {
                const role = event.target.closest("[data-role-bank-id]");
                if (!role) {
                    return;
                }
                state.drag = {
                    type: "stage-role",
                    stageID: null,
                    roleID: Number(role.dataset.roleBankId),
                };
                event.dataTransfer.effectAllowed = "move";
            });

            els.stageGrid.addEventListener("dragstart", (event) => {
                const role = event.target.closest("[data-role-id]");
                if (role) {
                    state.drag = {
                        type: "stage-role",
                        stageID: Number(role.dataset.stageId),
                        roleID: Number(role.dataset.roleId),
                    };
                    event.dataTransfer.effectAllowed = "move";
                    return;
                }
                const stage = event.target.closest("[data-stage-id]");
                if (!stage) {
                    return;
                }
                state.drag = { type: "stage", stageID: Number(stage.dataset.stageId) };
                event.dataTransfer.effectAllowed = "move";
            });

            els.stageGrid.addEventListener("dragover", (event) => {
                if (!state.drag) {
                    return;
                }
                const roleRow = event.target.closest("[data-stage-role-row]");
                const stageCard = event.target.closest(".stage-card");
                if (state.drag.type === "stage-role" && roleRow) {
                    event.preventDefault();
                    document.querySelectorAll("[data-stage-role-row]").forEach((row) => row.classList.remove("drag-target"));
                    roleRow.classList.add("drag-target");
                    return;
                }
                if (state.drag.type === "stage" && stageCard) {
                    event.preventDefault();
                    document.querySelectorAll(".stage-card").forEach((card) => card.classList.remove("drag-target"));
                    stageCard.classList.add("drag-target");
                }
            });

            els.stageGrid.addEventListener("drop", async (event) => {
                if (!state.drag) {
                    return;
                }
                const workflow = getCurrentWorkflow();
                if (!workflow) {
                    state.drag = null;
                    return;
                }
                if (state.drag.type === "stage-role") {
                    const roleRow = event.target.closest("[data-stage-role-row]");
                    if (!roleRow) {
                        state.drag = null;
                        return;
                    }
                    event.preventDefault();
                    roleRow.classList.remove("drag-target");
                    const targetStageID = Number(roleRow.dataset.stageRoleRow);
                    const stage = workflow.stages.find((item) => item.id === targetStageID);
                    if (!stage) {
                        state.drag = null;
                        return;
                    }
                    try {
                        if (state.drag.stageID && state.drag.stageID !== targetStageID) {
                            await api("/api/workflows/stages/roles/" + workflow.id + "/" + state.drag.stageID + "/" + state.drag.roleID, { method: "DELETE" });
                            await api("/api/workflows/stages/roles/" + workflow.id + "/" + targetStageID, { method: "POST", body: JSON.stringify({ role_id: state.drag.roleID }) });
                        } else if (!state.drag.stageID) {
                            await api("/api/workflows/stages/roles/" + workflow.id + "/" + targetStageID, { method: "POST", body: JSON.stringify({ role_id: state.drag.roleID }) });
                        } else {
                            const roleIDs = (stage.roles || []).map((role) => role.id).filter((roleID) => roleID !== state.drag.roleID);
                            roleIDs.push(state.drag.roleID);
                            await api("/api/workflows/stages/roles/" + workflow.id + "/" + targetStageID, {
                                method: "PUT",
                                body: JSON.stringify({ role_ids: roleIDs }),
                            });
                        }
                        await loadWorkflows();
                        renderAll();
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                    state.drag = null;
                    return;
                }

                const targetStage = event.target.closest(".stage-card");
                if (!targetStage) {
                    state.drag = null;
                    return;
                }
                event.preventDefault();
                targetStage.classList.remove("drag-target");
                const ordered = Array.from(els.stageGrid.querySelectorAll(".stage-card"))
                    .map((card) => Number(card.dataset.stageId))
                    .filter((stageID) => stageID !== state.drag.stageID);
                const targetIndex = ordered.indexOf(Number(targetStage.dataset.stageId));
                ordered.splice(targetIndex >= 0 ? targetIndex : ordered.length, 0, state.drag.stageID);
                try {
                    await api("/api/workflows/" + workflow.id + "/reorder", {
                        method: "PUT",
                        body: JSON.stringify({ stage_ids: ordered }),
                    });
                    await loadWorkflows();
                    renderAll();
                } catch (error) {
                    setNotice(error.message, true);
                }
                state.drag = null;
            });
        }

        function bindMiscHandlers() {
            function closeProjectMenu() {
                els.projectMenuDropdown.classList.remove("open");
                els.projectMenuButton.setAttribute("aria-expanded", "false");
            }

            function closeAccountMenu() {
                els.accountMenuDropdown.classList.remove("open");
                els.accountMenuButton.setAttribute("aria-expanded", "false");
            }

            els.accountMenuButton.addEventListener("click", (event) => {
                event.stopPropagation();
                closeProjectMenu();
                const isOpen = els.accountMenuDropdown.classList.toggle("open");
                els.accountMenuButton.setAttribute("aria-expanded", isOpen ? "true" : "false");
            });

            els.accountMenuDropdown.addEventListener("click", (event) => {
                const item = event.target.closest("[data-account-action]");
                if (item) {
                    closeAccountMenu();
                }
            });

            document.addEventListener("click", (event) => {
                if (!event.target.closest(".account-menu")) {
                    closeProjectMenu();
                    closeAccountMenu();
                }
            });

            document.getElementById("logout-button").addEventListener("click", async () => {
                try {
                    await api("/api/logout", { method: "POST", body: JSON.stringify({}) });
                } catch (error) {
                    // Ignore logout transport errors while still clearing local state.
                }
                closeAccountMenu();
                disconnectLiveUpdates();
                if (state.goalChatSocket) {
                    state.goalChatSocket.close();
                    state.goalChatSocket = null;
                }
                state.auth = null;
                clearStoredAuth();
                showLoginScreen();
                els.loginForm.reset();
            });
        }

        bindViewNavigation();
        bindProjectHandlers();
        bindGoalsHandlers();
        bindDocumentsHandlers();
        bindWorkflowHandlers();
        bindRolesHandlers();
        bindAgentsHandlers();
        bindTeamsHandlers();
        bindTicketsHandlers();
        bindMiscHandlers();
        els.loginForm.addEventListener("submit", handleLogin);
        state.viewScrollByPanel = loadStoredViewScrollByPanel();
        state.currentView = loadStoredSelectedView() || state.currentView;
        switchView(state.currentView, { restoreScroll: false });
        window.addEventListener("scroll", storeCurrentViewScroll, { passive: true });

        (async function restoreSession() {
            const auth = loadStoredAuth();
            if (!auth) {
                showLoginScreen();
                return;
            }
            state.auth = auth;
            apiClient.setToken(auth.token);
            document.getElementById("login-username").value = auth.username;
            showAuthenticatedShell();
            try {
                await refreshAll();
                connectLiveUpdates();
            } catch (error) {
                disconnectLiveUpdates();
                if (state.goalChatSocket) {
                    state.goalChatSocket.close();
                    state.goalChatSocket = null;
                }
                state.auth = null;
                clearStoredAuth();
                els.loginError.textContent = error.message;
                showLoginScreen();
            }
        }());

        window.site2 = {
            state,
            refreshAll,
            switchView,
            openTicketModal,
            closeTicketModal,
            emptyTicket,
        };
