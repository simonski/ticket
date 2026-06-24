        // Browser tests install window.__site2MockFetch before the app boots so the
        // whole API surface can be mocked without a server (tests/playwright/site2.spec.js).
        const apiClient = window.TicketAPI.createClient(
            window.__site2MockFetch ? { fetch: window.__site2MockFetch } : undefined,
        );
        const api = apiClient.request;
        const apiWithFallback = apiClient.requestWithFallback;
        const state = {
            auth: null,
            passkeys: [],
            passkeyError: "",
            passkeyBusy: false,
            passkeyStatus: "",
            passkeyStatusError: false,
            accountModalOpen: false,
            panels: null,
            accessRoles: [],
            currentView: "tickets",
            viewScrollByPanel: {},
            scrollPersistenceReady: false,
            status: null,
            plans: [],
            defaultPlan: null,
            selectedPlanSlug: "",
            selectedPlanDraft: null,
            projects: [],
            projectAccessRequests: [],
            projectAccessReviewEnabled: false,
            projectHistory: [],
            projectHistoryError: "",
            myProjectAccessRequests: [],
            myNotifications: [],
            documents: [],
            contextGraph: null,
            contextFocusKey: null,
            contextMatches: null,
            tickets: [],
            interventions: [],
            interventionReport: null,
            interventionTrends: [],
            interventionDrilldown: null,
            workflowValidation: {},
            workflows: [],
            roles: [],
            agents: [],
            teams: [],
            selectedProjectID: null,
            selectedProjectDraft: emptyProject(),
            selectedDocumentID: null,
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
            workflowStageViewMode: "board",
            workflowGraphNeedsReset: false,
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
            activeRoomID: 0,
            lastRoomID: 0,
            rooms: [],
            chatSort: "recency",
            chatCollapsed: {},
            liveRefreshTimer: null,
            refinementSocket: null,
            refinementTicketId: null,
            refinementPendingSend: null,
            refinementLastHumanText: null,
            agentBarPollTimer: null,
            documentFiles: [],
            systemAgentModelConfig: { provider: "", model: "", url: "", api_key: "", providers: [] },
            projectAgentModelConfig: { provider: "", model: "", url: "", api_key: "" },
            configSettings: [],
            selectedConfigSettingKey: "",
            selectedProviderConfigID: "",
            navOrder: [],
            users: [],
            teamMembers: [],
            releases: [],
            selectedReleaseID: "backlog",
            boardPerspective: localStorage.getItem("site2.board-view") || "board",
            org: null,
            programmes: [],
            selectedProgrammeID: null,
            projectAgents: [],
        };

        const TICKET_TYPES = ["epic", "task", "bug", "spike", "chore", "story", "note", "question", "requirement", "decision"];
        const FALLBACK_STAGES = ["backlog", "todo", "doing", "done"];
        const BACKLOG_BOARD_STAGES = ["idea", "refine", "ready"];
        const RELEASE_BOARD_STAGES = ["ready", "develop", "complete", "reject"];
        const AUTH_STORAGE_KEY = "site2.auth";
        const SELECTED_PROJECT_STORAGE_KEY = "site2.selectedProjectID";
        const SELECTED_VIEW_STORAGE_KEY = "site2.selectedView";
        const VIEW_SCROLL_STORAGE_KEY = "site2.viewScroll";
        const NAV_ORDER_STORAGE_KEY = "site2.navOrder";
        const NAV_ITEMS = [
            { view: "tickets", label: "Board", section: "general", icon: "<svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><path d=\"M4 7h16\"></path><path d=\"M4 12h16\"></path><path d=\"M4 17h10\"></path></svg>" },
            { view: "projects", label: "Projects", section: "general", icon: "<svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><path d=\"M3 7h18\"></path><path d=\"M6 7v10\"></path><path d=\"M12 7v10\"></path><path d=\"M18 7v10\"></path><path d=\"M3 17h18\"></path></svg>" },
            { view: "chat", label: "Chat", section: "general", icon: "<svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><path d=\"M4 5h16v11H7l-3 3z\"></path><path d=\"M8 9h8\"></path><path d=\"M8 12h5\"></path></svg>" },
            { view: "interventions", label: "Mailbox", section: "general", icon: "<svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><path d=\"M12 4v8\"></path><path d=\"M12 16h.01\"></path><circle cx=\"12\" cy=\"12\" r=\"9\"></circle></svg>" },
            { view: "workflows", label: "Workflows", section: "process", icon: "<svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><path d=\"M5 6h14\"></path><path d=\"M5 12h9\"></path><path d=\"M5 18h14\"></path><path d=\"M17 10l2 2-2 2\"></path></svg>" },
            { view: "roles", label: "Roles", section: "process", icon: "<svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><path d=\"M7 8a3 3 0 1 0 0.001 0\"></path><path d=\"M17 16a3 3 0 1 0 0.001 0\"></path><path d=\"M9.5 10.5l5 3\"></path></svg>" },
            { view: "documents", label: "Documents", section: "process", icon: "<svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><path d=\"M7 3h7l5 5v13H7z\"></path><path d=\"M14 3v5h5\"></path><path d=\"M9 13h8\"></path><path d=\"M9 17h8\"></path></svg>" },
            { view: "context", label: "Context", section: "process", icon: "<svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><circle cx=\"6\" cy=\"6\" r=\"2.5\"></circle><circle cx=\"18\" cy=\"8\" r=\"2.5\"></circle><circle cx=\"12\" cy=\"18\" r=\"2.5\"></circle><path d=\"M8 7l7.5 1\"></path><path d=\"M7 8l4 8\"></path><path d=\"M17 10l-4 6\"></path></svg>" },
            { view: "teams", label: "Teams", section: "process", icon: "<svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><path d=\"M8 11a2.5 2.5 0 1 0 0.001 0\"></path><path d=\"M16 9a2 2 0 1 0 0.001 0\"></path><path d=\"M4 19a4 4 0 0 1 8 0\"></path><path d=\"M14 19a3 3 0 0 1 6 0\"></path></svg>" },
            { view: "programmes", label: "Programmes", section: "admin", adminOnly: true, icon: "<svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><path d=\"M3 3h7v7H3z\"/><path d=\"M14 3h7v7h-7z\"/><path d=\"M3 14h7v7H3z\"/><path d=\"M14 14h7v7h-7z\"/></svg>" },
            { view: "settings", label: "Settings", section: "admin", adminOnly: true, icon: "<svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><path d=\"M12 3v4\"></path><path d=\"M12 17v4\"></path><path d=\"M4.9 6.3l2.8 2\"></path><path d=\"M16.3 15.7l2.8 2\"></path><path d=\"M3 12h4\"></path><path d=\"M17 12h4\"></path><path d=\"M4.9 17.7l2.8-2\"></path><path d=\"M16.3 8.3l2.8-2\"></path><circle cx=\"12\" cy=\"12\" r=\"3.5\"></circle></svg>" },
            { view: "agents", label: "Agents", section: "admin", adminOnly: true, icon: "<svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><path d=\"M12 3v4\"></path><path d=\"M8 8a4 4 0 1 1 8 0\"></path><path d=\"M7 13h10v7H7z\"></path></svg>" },
            { view: "users", label: "Users", section: "admin", adminOnly: true, icon: "<svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><path d=\"M16 21v-2a4 4 0 0 0-4-4H6a4 4 0 0 0-4 4v2\"></path><circle cx=\"9\" cy=\"7\" r=\"4\"></circle><path d=\"M22 21v-2a4 4 0 0 0-3-3.87\"></path><path d=\"M16 3.13a4 4 0 0 1 0 7.75\"></path></svg>" },
            { view: "access", label: "Access", section: "admin", adminOnly: true, icon: "<svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><rect x=\"5\" y=\"11\" width=\"14\" height=\"9\" rx=\"2\"></rect><path d=\"M8 11V8a4 4 0 0 1 8 0v3\"></path></svg>" },
            { view: "admin-summary", label: "Summary", section: "admin", adminOnly: true, icon: "<svg viewBox=\"0 0 24 24\" aria-hidden=\"true\"><rect x=\"3\" y=\"3\" width=\"7\" height=\"7\"/><rect x=\"14\" y=\"3\" width=\"7\" height=\"7\"/><rect x=\"3\" y=\"14\" width=\"7\" height=\"7\"/><rect x=\"14\" y=\"14\" width=\"7\" height=\"7\"/></svg>" },
        ];
        let navDragView = "";
        let documentDropDepth = 0;
        let documentDropSuccessTimer = null;

        const els = {
            loginScreen: document.getElementById("login-screen"),
            loginForm: document.getElementById("login-form"),
            loginPasskeyButton: document.getElementById("login-passkey-button"),
            registerForm: document.getElementById("register-form"),
            registerHelp: document.getElementById("register-help"),
            loginError: document.getElementById("login-error"),
            accountModal: document.getElementById("account-modal"),
            accountModalTitle: document.getElementById("account-modal-title"),
            accountModalSummary: document.getElementById("account-modal-summary"),
            accountProfileDetails: document.getElementById("account-profile-details"),
            accountPasskeyList: document.getElementById("account-passkey-list"),
            accountPasskeyStatus: document.getElementById("account-passkey-status"),
            accountPasskeyName: document.getElementById("account-passkey-name"),
            accountPasskeyEnrollButton: document.getElementById("account-passkey-enroll-button"),
            accountOpenConfigButton: document.getElementById("account-open-config-button"),
            closeAccountModal: document.getElementById("close-account-modal"),
            versionOverlay: document.getElementById("version-overlay"),
            showRegisterButton: document.getElementById("show-register-button"),
            hideRegisterButton: document.getElementById("hide-register-button"),
            appShell: document.getElementById("app-shell"),
            appNotice: document.getElementById("app-notice"),
            projectMenuButton: document.getElementById("project-menu-button"),
            projectMenuDropdown: document.getElementById("project-menu-dropdown"),
            projectMenuList: document.getElementById("project-menu-list"),
            projectCreateLink: document.getElementById("project-create-link"),
            mainNav: document.getElementById("main-nav"),
            adminNav: document.getElementById("admin-nav"),
            userList: document.getElementById("user-list"),
            newUserButton: document.getElementById("new-user-button"),
            userModal: document.getElementById("user-modal"),
            userModalTitle: document.getElementById("user-modal-title"),
            userForm: document.getElementById("user-form"),
            userModalUsername: document.getElementById("user-modal-username"),
            userModalEmail: document.getElementById("user-modal-email"),
            userModalPassword: document.getElementById("user-modal-password"),
            userModalRole: document.getElementById("user-modal-role"),
            userModalStatusRow: document.getElementById("user-modal-status-row"),
            userModalEnabled: document.getElementById("user-modal-enabled"),
            userModalError: document.getElementById("user-modal-error"),
            userModalSave: document.getElementById("user-modal-save"),
            userModalResetPw: document.getElementById("user-modal-reset-pw"),
            userModalDelete: document.getElementById("user-modal-delete"),
            userModalGeneratedPw: document.getElementById("user-modal-generated-pw"),
            teamMemberList: document.getElementById("team-member-list"),
            teamInviteUserSelect: document.getElementById("team-invite-user-select"),
            teamInviteRole: document.getElementById("team-invite-role"),
            teamInviteButton: document.getElementById("team-invite-button"),
            planAdminPanel: document.getElementById("plan-admin-panel"),
            defaultPlanSelect: document.getElementById("default-plan-select"),
            registrationEnabledSelect: document.getElementById("registration-enabled-select"),
            registrationAutoApproveSelect: document.getElementById("registration-auto-approve-select"),
            savePlanAdminButton: document.getElementById("save-plan-admin-button"),
            planList: document.getElementById("plan-list"),
            planEditorTitle: document.getElementById("plan-editor-title"),
            planSlug: document.getElementById("plan-slug"),
            planName: document.getElementById("plan-name"),
            planDescription: document.getElementById("plan-description"),
            planMaxProjects: document.getElementById("plan-max-projects"),
            planMaxPrivateProjects: document.getElementById("plan-max-private-projects"),
            planMaxTickets: document.getElementById("plan-max-tickets"),
            planMaxTicketsPerProject: document.getElementById("plan-max-tickets-per-project"),
            planMaxTeamMemberships: document.getElementById("plan-max-team-memberships"),
            planMaxAPICallsPerDay: document.getElementById("plan-max-api-calls-per-day"),
            planDefaultProjectAlias: document.getElementById("plan-default-project-alias"),
            planAutoAssignPublicTeam: document.getElementById("plan-auto-assign-public-team"),
            planAutoCreatePrivateProject: document.getElementById("plan-auto-create-private-project"),
            planAutoCreatePrivateTeam: document.getElementById("plan-auto-create-private-team"),
            projectAccessRequestsPanel: document.getElementById("project-access-requests-panel"),
            projectAccessRequestsSummary: document.getElementById("project-access-requests-summary"),
            projectAccessRequestsList: document.getElementById("project-access-requests-list"),
            projectHistoryPanel: document.getElementById("project-history-panel"),
            projectHistorySummary: document.getElementById("project-history-summary"),
            projectHistoryList: document.getElementById("project-history-list"),
            projectRequestAccessPanel: document.getElementById("project-request-access-panel"),
            projectRequestAccessForm: document.getElementById("project-request-access-form"),
            projectRequestAccessRef: document.getElementById("project-request-access-ref"),
            projectRequestAccessMessage: document.getElementById("project-request-access-message"),
            projectMyAccessRequestsPanel: document.getElementById("project-my-access-requests-panel"),
            projectMyAccessRequestsSummary: document.getElementById("project-my-access-requests-summary"),
            projectMyAccessRequestsList: document.getElementById("project-my-access-requests-list"),
            projectNotificationsPanel: document.getElementById("project-notifications-panel"),
            projectNotificationsSummary: document.getElementById("project-notifications-summary"),
            projectNotificationsList: document.getElementById("project-notifications-list"),
            accountMenuButton: document.getElementById("account-menu-button"),
            accountMenuDropdown: document.getElementById("account-menu-dropdown"),
            accountMenuName: document.getElementById("account-menu-name"),
            projectList: document.getElementById("project-list"),
            documentList: document.getElementById("document-list"),
            workflowList: document.getElementById("workflow-list"),
            workflowSelect: document.getElementById("workflow-select"),
            workflowSettings: document.getElementById("workflow-settings"),
            workflowStageBoard: document.getElementById("workflow-stage-board"),
            workflowViewBoardButton: document.getElementById("workflow-view-board"),
            workflowViewGraphButton: document.getElementById("workflow-view-graph"),
            roleList: document.getElementById("role-list"),
            agentList: document.getElementById("agent-list"),
            teamList: document.getElementById("team-list"),
            ticketBoard: document.getElementById("ticket-board"),
            ticketListView: document.getElementById("ticket-list-view"),
            ticketPlanView: document.getElementById("ticket-plan-view"),
            projectAgentBar: document.getElementById("project-agent-bar"),
            adminSummaryContent: document.getElementById("admin-summary-content"),
            boardSearch: document.getElementById("board-search"),
            boardHideDone: document.getElementById("board-hide-done"),
            boardReleaseSelect: document.getElementById("board-release-select"),
            interventionList: document.getElementById("intervention-list"),
            interventionFilter: document.getElementById("intervention-filter"),
            interventionSort: document.getElementById("intervention-sort"),
            interventionTrendsSummary: document.getElementById("intervention-trends-summary"),
            interventionReportSummary: document.getElementById("intervention-report-summary"),
            stageGrid: document.getElementById("stage-grid"),
            workflowRoleBank: document.getElementById("workflow-role-bank"),
            workflowValidationSummary: document.getElementById("workflow-validation-summary"),
            ticketModal: document.getElementById("ticket-modal"),
            ticketPullRequests: document.getElementById("ticket-pull-requests"),
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
            ticketContext: document.getElementById("ticket-context"),
            ticketContextType: document.getElementById("ticket-context-type"),
            ticketContextTarget: document.getElementById("ticket-context-target"),
            addTicketContextButton: document.getElementById("add-ticket-context-button"),
            ticketTimeEntries: document.getElementById("ticket-time-entries"),
            ticketTimeTotal: document.getElementById("ticket-time-total"),
            ticketTimeMinutes: document.getElementById("ticket-time-minutes"),
            ticketTimeNote: document.getElementById("ticket-time-note"),
            addTicketTimeButton: document.getElementById("add-ticket-time-button"),
            agentHarnessSummary: document.getElementById("agent-harness-summary"),
            systemAgentProvider: document.getElementById("system-agent-provider"),
            projectAgentProvider: document.getElementById("project-agent-provider"),
            resolvedAgentProvider: document.getElementById("resolved-agent-provider"),
            resolvedAgentModel: document.getElementById("resolved-agent-model"),
            resolvedAgentURL: document.getElementById("resolved-agent-url"),
            resolvedAgentAPIKey: document.getElementById("resolved-agent-api-key"),
            providerConfigSelect: document.getElementById("provider-config-select"),
            providerConfigForm: document.getElementById("provider-config-form"),
            providerConfigID: document.getElementById("provider-config-id"),
            providerConfigLabel: document.getElementById("provider-config-label"),
            providerConfigModel: document.getElementById("provider-config-model"),
            providerConfigURL: document.getElementById("provider-config-url"),
            providerConfigAuthType: document.getElementById("provider-config-auth-type"),
            providerConfigRequiresURL: document.getElementById("provider-config-requires-url"),
            providerConfigAPIKey: document.getElementById("provider-config-api-key"),
            providerConfigModels: document.getElementById("provider-config-models"),
            configSettingsSummary: document.getElementById("config-settings-summary"),
            configSettingsList: document.getElementById("config-settings-list"),
            configSettingForm: document.getElementById("config-setting-form"),
            configSettingEditorTitle: document.getElementById("config-setting-editor-title"),
            configSettingKey: document.getElementById("config-setting-key"),
            configSettingValue: document.getElementById("config-setting-value"),
            documentFilesList: document.getElementById("document-files-list"),
            documentUploadFile: document.getElementById("document-upload-file"),
            documentUploadName: document.getElementById("document-upload-name"),
            documentsView: document.getElementById("view-documents"),
            documentDropOverlay: document.getElementById("document-drop-overlay"),
            dialogOverlay: document.getElementById("dialog-overlay"),
            dialogBox: document.getElementById("dialog-box"),
            dialogMessage: document.getElementById("dialog-message"),
            dialogInputWrap: document.getElementById("dialog-input-wrap"),
            dialogInput: document.getElementById("dialog-input"),
            dialogOK: document.getElementById("dialog-ok"),
            dialogCancel: document.getElementById("dialog-cancel"),
            orgForm: document.getElementById("org-form"),
            orgName: document.getElementById("org-name"),
            orgDomain: document.getElementById("org-domain"),
            orgDescription: document.getElementById("org-description"),
            orgLogo: document.getElementById("org-logo"),
            saveOrgButton: document.getElementById("save-org-button"),
            programmeList: document.getElementById("programme-list"),
            programmeEditorTitle: document.getElementById("programme-editor-title"),
            programmeForm: document.getElementById("programme-form"),
            programmeName: document.getElementById("programme-name"),
            programmeDescription: document.getElementById("programme-description"),
            programmeProjectsList: document.getElementById("programme-projects-list"),
            saveProgrammeButton: document.getElementById("save-programme-button"),
            deleteProgrammeButton: document.getElementById("delete-programme-button"),
            resetProgrammeButton: document.getElementById("reset-programme-button"),
        };
        const TRASH_ICON_SVG = "<svg class=\"icon-trash\" viewBox=\"0 0 24 24\" aria-hidden=\"true\"><path d=\"M4 7h16\"></path><path d=\"M9 7V5h6v2\"></path><path d=\"M10 11v6\"></path><path d=\"M14 11v6\"></path><path d=\"M7 7l1 12h8l1-12\"></path></svg>";
        let dialogResolve = null;
        let dialogState = null;
        let workflowAutosaveTimer = null;
        let workflowAutosaveInFlight = false;
        let workflowAutosaveQueued = false;
        const WORKFLOW_GRAPH_NODE_WIDTH = 184;
        const WORKFLOW_GRAPH_NODE_HEIGHT = 60;
        const WORKFLOW_GRAPH_COLUMN_GAP = 248;
        const WORKFLOW_GRAPH_ROW_GAP = 132;

        function emptyProject() {
            return {
                id: null,
                prefix: "",
                title: "",
                description: "",
                acceptance_criteria: "",
                git_repository: "",
                visibility: "public",
                accepts_new_members: false,
                workflow_id: null,
                default_draft: false,
            };
        }

        function emptyWorkflow() {
            return { id: null, name: "", description: "", approval_policy: "single_role", progression_mode: "linear" };
        }

        function emptyPlan() {
            return {
                plan_id: null,
                slug: "",
                name: "",
                description: "",
                max_projects: 1,
                max_private_projects: 1,
                max_tickets: 100,
                max_tickets_per_project: 100,
                max_team_memberships: 10,
                max_api_calls_per_day: 1000,
                default_project_alias: "public",
                registration_actions: {
                    auto_assign_public_team: false,
                    auto_create_private_project: false,
                    auto_create_private_team: false,
                    teams: [],
                    projects: [],
                },
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

        function serverVersionFromStatus(status) {
            if (!status || typeof status !== "object") {
                return "";
            }
            return String(status.server_version || status.version || "").trim();
        }

        function setServerVersion(version, unavailable = false) {
            if (!els.versionOverlay) {
                return;
            }
            if (unavailable) {
                els.versionOverlay.textContent = "server: offline";
                return;
            }
            els.versionOverlay.textContent = version ? `server: ${version}` : "server: unknown";
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

        function normalizePlan(plan) {
            const item = plan || {};
            const actions = item.registration_actions || {};
            return Object.assign({}, emptyPlan(), item, {
                plan_id: item.plan_id !== undefined ? item.plan_id : item.id,
                slug: String(item.slug || "").trim(),
                name: String(item.name || "").trim(),
                description: String(item.description || ""),
                max_projects: Number(item.max_projects || 0),
                max_private_projects: Number(item.max_private_projects || 0),
                max_tickets: Number(item.max_tickets || 0),
                max_tickets_per_project: Number(item.max_tickets_per_project || 0),
                max_team_memberships: Number(item.max_team_memberships || 0),
                max_api_calls_per_day: Number(item.max_api_calls_per_day || 0),
                default_project_alias: String(item.default_project_alias || "public"),
                registration_actions: {
                    auto_assign_public_team: Boolean(actions.auto_assign_public_team),
                    auto_create_private_project: Boolean(actions.auto_create_private_project),
                    auto_create_private_team: Boolean(actions.auto_create_private_team),
                    teams: Array.isArray(actions.teams) ? actions.teams : [],
                    projects: Array.isArray(actions.projects) ? actions.projects : [],
                },
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
                accepts_new_members: Boolean(project.accepts_new_members),
            });
        }

        function normalizeProjectAccessRequest(request) {
            return Object.assign({}, request, {
                id: request.id !== undefined ? request.id : request.request_id,
                project_id: Number(request.project_id || 0),
                project_prefix: String(request.project_prefix || ""),
                project_title: String(request.project_title || ""),
                decision_message: String(request.decision_message || ""),
                decided_by: String(request.decided_by || ""),
                decided_at: String(request.decided_at || ""),
            });
        }

        function parseHistoryPayload(payload) {
            if (!payload) {
                return {};
            }
            if (typeof payload === "object") {
                return payload;
            }
            if (typeof payload !== "string") {
                return {};
            }
            try {
                const parsed = JSON.parse(payload);
                return parsed && typeof parsed === "object" ? parsed : {};
            } catch (_error) {
                return {};
            }
        }

        function humanizeHistoryEventType(eventType) {
            return String(eventType || "event")
                .replace(/^project_/, "")
                .replace(/^ticket_/, "")
                .replace(/_/g, " ");
        }

        function formatHistoryPayloadSummary(payload) {
            const entries = Object.entries(payload || {}).filter((entry) => {
                const value = entry[1];
                return value !== undefined && value !== null && value !== "" && typeof value !== "object";
            });
            if (!entries.length) {
                return "";
            }
            return entries.slice(0, 3).map((entry) => entry[0] + ": " + String(entry[1])).join(" · ");
        }

        function formatProjectHistorySummary(event) {
            const payload = parseHistoryPayload(event && event.payload);
            const eventType = String(event && event.event_type || "");
            if (eventType === "project_access_request_created") {
                const username = String(payload.username || payload.user_id || "user");
                const projectPrefix = String(payload.project_prefix || "");
                const message = String(payload.message || "").trim();
                let summary = username + " requested access";
                if (projectPrefix) {
                    summary += " to " + projectPrefix;
                }
                if (message) {
                    summary += ": " + message;
                }
                return summary;
            }
            if (eventType === "project_access_request_approved" || eventType === "project_access_request_rejected") {
                const username = String(payload.username || payload.user_id || "user");
                const projectPrefix = String(payload.project_prefix || "");
                const requestID = Number(payload.request_id || 0);
                const verb = eventType === "project_access_request_rejected" ? "rejected" : "approved";
                const decisionMessage = String(payload.decision_message || "");
                let summary = verb + " access request";
                if (requestID) {
                    summary += " #" + String(requestID);
                }
                summary += " for " + username;
                if (projectPrefix) {
                    summary += " on " + projectPrefix;
                }
                if (decisionMessage) {
                    summary += ": " + decisionMessage;
                }
                return summary;
            }
            const payloadSummary = formatHistoryPayloadSummary(payload);
            return payloadSummary ? (humanizeHistoryEventType(eventType) + " · " + payloadSummary) : humanizeHistoryEventType(eventType);
        }

        function isAdmin() {
            return Boolean(state.status && state.status.user && state.status.user.role === "admin");
        }
        function currentUserID() {
            return (state.status && state.status.user && (state.status.user.user_id || state.status.user.id)) || "";
        }

        // canAccessPanel gates a nav view by the user's effective panel set
        // (TK-135). Admins see everything. Before the panel set loads
        // (state.panels === null) we stay optimistic so the nav doesn't flash
        // empty. Admin-only items are gated by isAdmin() separately.
        function canAccessPanel(view) {
            if (isAdmin()) {
                return true;
            }
            if (!Array.isArray(state.panels)) {
                return true;
            }
            return state.panels.includes(view);
        }
        function navItemVisible(item) {
            return item.adminOnly ? isAdmin() : canAccessPanel(item.view);
        }

        function visibleNavItems() {
            return NAV_ITEMS.filter(navItemVisible);
        }
        function visibleGeneralNavItems() {
            return NAV_ITEMS.filter((item) => item.section === "general" && navItemVisible(item));
        }
        function visibleProcessNavItems() {
            return NAV_ITEMS.filter((item) => item.section === "process" && navItemVisible(item));
        }
        function visibleAdminNavItems() {
            return NAV_ITEMS.filter((item) => item.section === "admin" && isAdmin());
        }

        function isPermissionErrorMessage(message) {
            return /forbidden|unauthorized|access denied/i.test(String(message || ""));
        }

        function emptyAgentModelConfig() {
            return {
                provider: "",
                model: "",
                url: "",
                api_key: "",
                providers: [],
            };
        }

        function normalizeAgentModelConfig(config) {
            const cfg = config || {};
            return {
                provider: String(cfg.provider || cfg.agent_model_provider || "").trim(),
                model: String(cfg.model || cfg.agent_model_name || "").trim(),
                url: String(cfg.url || cfg.agent_model_url || "").trim(),
                api_key: String(cfg.api_key || cfg.agent_model_api_key || "").trim(),
                providers: Array.isArray(cfg.providers) ? cfg.providers : [],
            };
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

        // splitAgentRoles parses an agent's role field (comma-separated role names)
        // into a trimmed list, dropping empties. Mirrors store.SplitAgentRoles.
        function splitAgentRoles(agentRole) {
            return String(agentRole || "")
                .split(",")
                .map((r) => r.trim())
                .filter((r) => r.length > 0);
        }

        function normalizePasskeyCredential(credential) {
            return Object.assign({}, credential, {
                id: credential.id || credential.credential_id,
                name: String(credential.name || "").trim(),
                created_at: String(credential.created_at || ""),
                updated_at: String(credential.updated_at || ""),
                last_used_at: String(credential.last_used_at || ""),
            });
        }

        function storeAuth(auth) {
            sessionStorage.setItem(AUTH_STORAGE_KEY, JSON.stringify(auth));
            apiClient.setToken(auth && auth.token ? auth.token : "");
            localStorage.setItem("tk-authed", "1");
        }

        function clearStoredAuth() {
            sessionStorage.removeItem(AUTH_STORAGE_KEY);
            localStorage.removeItem("tk-authed");
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

        function currentDefaultProjectID() {
            const value = state.status && state.status.user ? Number(state.status.user.default_project_id) : 0;
            return Number.isFinite(value) && value > 0 ? value : null;
        }

        function availableViewNames() {
            return visibleNavItems().map((item) => item.view);
        }

        function sanitizeNavOrder(order) {
            const knownViews = availableViewNames();
            const knownSet = new Set(knownViews);
            const next = [];
            (Array.isArray(order) ? order : []).forEach((value) => {
                const view = String(value || "").trim();
                if (!view || !knownSet.has(view) || next.includes(view)) {
                    return;
                }
                next.push(view);
            });
            knownViews.forEach((view) => {
                if (!next.includes(view)) {
                    next.push(view);
                }
            });
            return next;
        }

        function isKnownView(viewName) {
            return availableViewNames().includes(String(viewName || "").trim());
        }

        function storeNavOrder(order) {
            localStorage.setItem(NAV_ORDER_STORAGE_KEY, JSON.stringify(sanitizeNavOrder(order)));
        }

        function loadStoredNavOrder() {
            const raw = localStorage.getItem(NAV_ORDER_STORAGE_KEY);
            if (!raw) {
                return sanitizeNavOrder([]);
            }
            try {
                return sanitizeNavOrder(JSON.parse(raw));
            } catch (error) {
                localStorage.removeItem(NAV_ORDER_STORAGE_KEY);
                return sanitizeNavOrder([]);
            }
        }

        function renderMainNav() {
            const generalItems = visibleGeneralNavItems();
            const adminItems = visibleAdminNavItems();

            // General nav
            const generalByView = new Map(generalItems.map((item) => [item.view, item]));
            const storedOrder = state.navOrder && state.navOrder.length ? state.navOrder : loadStoredNavOrder();
            const generalOrder = sanitizeNavOrder(storedOrder).filter((view) => generalByView.has(view));
            // Ensure all general items are present in order
            generalItems.forEach((item) => {
                if (!generalOrder.includes(item.view)) generalOrder.push(item.view);
            });
            state.navOrder = sanitizeNavOrder(storedOrder);
            storeNavOrder(state.navOrder);
            const generalHtml = generalOrder.map((view) => {
                const item = generalByView.get(view);
                if (!item) return "";
                const active = view === state.currentView ? " active" : "";
                return "<button type=\"button\" data-view=\"" + item.view + "\" class=\"" + active.trim() + "\" draggable=\"true\">" +
                    "<span class=\"nav-icon\">" + item.icon + "</span><span>" + escapeHTML(item.label) + "</span></button>";
            }).join("");
            setInnerHTMLIfChanged(els.mainNav, generalHtml);

            // Process nav (static — workflows, roles, documents, teams)
            const processNavEl = document.getElementById("process-nav");
            if (processNavEl) {
                const processHtml = visibleProcessNavItems().map((item) => {
                    const active = item.view === state.currentView ? " active" : "";
                    return "<button type=\"button\" data-view=\"" + item.view + "\" class=\"" + active.trim() + "\">" +
                        "<span class=\"nav-icon\">" + item.icon + "</span><span>" + escapeHTML(item.label) + "</span></button>";
                }).join("");
                setInnerHTMLIfChanged(processNavEl, processHtml);
            }

            // Admin nav
            const adminNavEl = document.getElementById("admin-nav");
            const adminSection = document.querySelector(".nav-admin-section");
            if (adminSection) {
                adminSection.style.display = adminItems.length ? "" : "none";
            }
            if (adminNavEl) {
                const adminHtml = adminItems.map((item) => {
                    const active = item.view === state.currentView ? " active" : "";
                    return "<button type=\"button\" data-view=\"" + item.view + "\" class=\"" + active.trim() + "\" draggable=\"true\">" +
                        "<span class=\"nav-icon\">" + item.icon + "</span><span>" + escapeHTML(item.label) + "</span></button>";
                }).join("");
                setInnerHTMLIfChanged(adminNavEl, adminHtml);
            }
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

        function focusRegisterUsername() {
            requestAnimationFrame(() => {
                const input = document.getElementById("register-username");
                if (input && !els.loginScreen.classList.contains("hidden") && !els.registerForm.classList.contains("hidden")) {
                    input.focus();
                }
            });
        }

        function isAuthError(error) {
            return Boolean(error && (Number(error.status) === 401 || Number(error.status) === 403 || error.isAuthError));
        }

        function authError(message) {
            const error = new Error(message || "unauthorized");
            error.status = 401;
            error.isAuthError = true;
            return error;
        }

        function browserSupportsPasskeys() {
            return typeof window.PublicKeyCredential !== "undefined"
                && Boolean(navigator.credentials)
                && typeof navigator.credentials.get === "function";
        }

        function browserSupportsPasskeyEnrollment() {
            return typeof window.PublicKeyCredential !== "undefined"
                && Boolean(navigator.credentials)
                && typeof navigator.credentials.create === "function";
        }

        function decodeBase64URL(value) {
            const base64 = String(value || "").replace(/-/g, "+").replace(/_/g, "/");
            const padded = base64 + "=".repeat((4 - base64.length % 4) % 4);
            const raw = window.atob(padded);
            const bytes = new Uint8Array(raw.length);
            for (let index = 0; index < raw.length; index += 1) {
                bytes[index] = raw.charCodeAt(index);
            }
            return bytes;
        }

        function encodeBase64URL(buffer) {
            const bytes = buffer instanceof Uint8Array ? buffer : new Uint8Array(buffer || []);
            let raw = "";
            for (let index = 0; index < bytes.length; index += 1) {
                raw += String.fromCharCode(bytes[index]);
            }
            return window.btoa(raw).replace(/\+/g, "-").replace(/\//g, "_").replace(/=+$/g, "");
        }

        function normalizePasskeyRequestOptions(options) {
            if (window.PublicKeyCredential && typeof window.PublicKeyCredential.parseRequestOptionsFromJSON === "function") {
                return window.PublicKeyCredential.parseRequestOptionsFromJSON(options);
            }
            const normalized = typeof structuredClone === "function"
                ? structuredClone(options)
                : JSON.parse(JSON.stringify(options || {}));
            normalized.challenge = decodeBase64URL(normalized.challenge);
            if (Array.isArray(normalized.allowCredentials)) {
                normalized.allowCredentials = normalized.allowCredentials.map((item) => ({
                    ...item,
                    id: decodeBase64URL(item.id),
                }));
            }
            return normalized;
        }

        function normalizePasskeyCreationOptions(options) {
            if (window.PublicKeyCredential && typeof window.PublicKeyCredential.parseCreationOptionsFromJSON === "function") {
                return window.PublicKeyCredential.parseCreationOptionsFromJSON(options);
            }
            const normalized = typeof structuredClone === "function"
                ? structuredClone(options)
                : JSON.parse(JSON.stringify(options || {}));
            normalized.challenge = decodeBase64URL(normalized.challenge);
            if (normalized.user && normalized.user.id) {
                normalized.user.id = decodeBase64URL(normalized.user.id);
            }
            if (Array.isArray(normalized.excludeCredentials)) {
                normalized.excludeCredentials = normalized.excludeCredentials.map((item) => ({
                    ...item,
                    id: decodeBase64URL(item.id),
                }));
            }
            return normalized;
        }

        function serializePasskeyCredential(assertion) {
            if (!assertion || !assertion.response) {
                throw new Error("passkey assertion did not return a credential");
            }
            const response = assertion.response;
            const payload = {
                id: assertion.id,
                rawId: encodeBase64URL(assertion.rawId),
                type: assertion.type,
                response: {
                    clientDataJSON: encodeBase64URL(response.clientDataJSON),
                },
            };
            if (response.authenticatorData) {
                payload.response.authenticatorData = encodeBase64URL(response.authenticatorData);
            }
            if (response.signature) {
                payload.response.signature = encodeBase64URL(response.signature);
            }
            if (response.attestationObject) {
                payload.response.attestationObject = encodeBase64URL(response.attestationObject);
            }
            if (response.userHandle) {
                payload.response.userHandle = encodeBase64URL(response.userHandle);
            }
            if (typeof response.getTransports === "function") {
                payload.response.transports = response.getTransports();
            }
            if (assertion.authenticatorAttachment) {
                payload.authenticatorAttachment = assertion.authenticatorAttachment;
            }
            if (typeof assertion.getClientExtensionResults === "function") {
                payload.clientExtensionResults = assertion.getClientExtensionResults();
            }
            return payload;
        }

        function delay(ms) {
            return new Promise((resolve) => window.setTimeout(resolve, ms));
        }

        function setPasskeyStatus(message, isError) {
            state.passkeyStatus = String(message || "");
            state.passkeyStatusError = Boolean(isError);
        }

        async function finalizeAuthenticatedSession(auth) {
            state.auth = auth;
            await refreshAll();
            storeAuth(auth);
            showAuthenticatedShell();
            connectLiveUpdates();
            startAgentBarPoller();
        }

        function resetAuthFailure(message) {
            state.auth = null;
            clearStoredAuth();
            els.loginError.textContent = message;
        }

        async function completePasskeyLogin(username) {
            const start = await apiClient.startPasskeyLogin(username);
            const challenge = await apiClient.getPasskeyChallenge(start.code);
            if (!challenge || challenge.kind !== "login") {
                throw new Error("passkey login challenge was not available");
            }
            const assertion = await navigator.credentials.get({
                publicKey: normalizePasskeyRequestOptions(challenge.public_key),
            });
            await apiClient.finishPasskeyFlow(start.code, serializePasskeyCredential(assertion));
            let result = null;
            for (let attempt = 0; attempt < 5; attempt += 1) {
                result = await apiClient.pollPasskey(start.code);
                if (result && result.status === "complete") {
                    break;
                }
                await delay(150);
            }
            if (!result || result.status !== "complete" || !result.token) {
                throw new Error("passkey login did not complete");
            }
            return {
                username: (result.user && result.user.username) || username,
                token: result.token,
            };
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

        // humanizeSince renders a coarse elapsed time since a stored UTC timestamp
        // ("2026-06-22 18:08:52"), mirroring the CLI/TUI helper (TK-90).
        function humanizeSince(ts) {
            if (!ts) return "";
            const then = new Date(String(ts).trim().replace(" ", "T") + "Z");
            if (isNaN(then.getTime())) return "";
            const ms = Date.now() - then.getTime();
            if (ms < 60000) return "just now";
            if (ms < 3600000) return Math.floor(ms / 60000) + "m";
            if (ms < 86400000) return Math.floor(ms / 3600000) + "h";
            return Math.floor(ms / 86400000) + "d";
        }

        function closeDialog(result) {
            if (els.dialogOverlay) {
                els.dialogOverlay.classList.add("hidden");
            }
            const resolver = dialogResolve;
            const settings = dialogState;
            dialogResolve = null;
            dialogState = null;
            // Capture the prompt input value BEFORE clearing the field, otherwise
            // the resolver always receives an empty string.
            const inputValue = els.dialogInput ? els.dialogInput.value : "";
            if (els.dialogInputWrap) {
                els.dialogInputWrap.classList.add("hidden");
            }
            if (els.dialogInput) {
                els.dialogInput.value = "";
            }
            if (resolver) {
                if (settings && settings.input) {
                    resolver(result === true ? inputValue : null);
                    return;
                }
                resolver(Boolean(result));
            }
        }

        function openDialog(message, options) {
            if (!els.dialogOverlay || !els.dialogMessage || !els.dialogOK || !els.dialogCancel) {
                if (options && options.input) {
                    const nextValue = window.prompt(String(message || ""), String(options.inputValue || ""));
                    return Promise.resolve(nextValue === null ? null : String(nextValue));
                }
                return Promise.resolve(options && options.confirm === false ? true : window.confirm(String(message || "")));
            }
            if (dialogResolve) {
                closeDialog(false);
            }
            const settings = Object.assign({ confirm: true, okText: "OK", cancelText: "Cancel", input: false, inputValue: "" }, options || {});
            dialogState = settings;
            els.dialogMessage.textContent = String(message || "");
            els.dialogOK.textContent = settings.okText;
            els.dialogCancel.textContent = settings.cancelText;
            els.dialogCancel.classList.toggle("hidden", settings.confirm === false);
            if (els.dialogInputWrap && els.dialogInput) {
                els.dialogInputWrap.classList.toggle("hidden", !settings.input);
                els.dialogInput.value = settings.input ? String(settings.inputValue || "") : "";
            }
            els.dialogOverlay.classList.remove("hidden");
            setTimeout(() => {
                if (settings.input && els.dialogInput) {
                    els.dialogInput.focus();
                    els.dialogInput.select();
                    return;
                }
                els.dialogOK.focus();
            }, 0);
            return new Promise((resolve) => {
                dialogResolve = resolve;
            });
        }

        function uiAlert(message) {
            return openDialog(message, { confirm: false, okText: "OK" });
        }

        function uiConfirm(message, okText) {
            return openDialog(message, { confirm: true, okText: okText || "OK", cancelText: "Cancel" });
        }

        function uiPrompt(message, inputValue, okText) {
            return openDialog(message, {
                confirm: true,
                okText: okText || "Save",
                cancelText: "Cancel",
                input: true,
                inputValue: inputValue || "",
            });
        }

        function decorateDeleteButtons(root) {
            const scope = root || document;
            scope.querySelectorAll("button.btn-danger").forEach((button) => {
                if (button.dataset.deleteIconApplied === "true") {
                    return;
                }
                const label = String(button.getAttribute("aria-label") || button.textContent || "").trim().replace(/\s+/g, " ");
                if (!label || !/delete/i.test(label)) {
                    return;
                }
                button.dataset.deleteIconApplied = "true";
                button.setAttribute("aria-label", label);
                button.setAttribute("title", label);
                button.classList.add("icon-button-danger");
                button.innerHTML = TRASH_ICON_SVG + "<span class=\"sr-only\">" + escapeHTML(label) + "</span>";
            });
        }

        window.closeDialog = closeDialog;
        window.uiAlert = uiAlert;
        window.uiConfirm = uiConfirm;
        window.uiPrompt = uiPrompt;

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

        // ticketStageIsBacklog reports whether a ticket's current stage is a backlog
        // (preparation/refinement) stage in its project's workflow — design in Agile,
        // idea/refine/ready in the bootstrap workflow, etc.
        function ticketStageIsBacklog(ticket) {
            if (!ticket) return false;
            const stage = String(ticket.stage || "").toLowerCase();
            const workflow = getCurrentProjectWorkflow();
            const st = (workflow && Array.isArray(workflow.stages) ? workflow.stages : [])
                .find((s) => String(s.name || "").toLowerCase() === stage);
            if (st) return Boolean(st.is_backlog_stage);
            // Fall back to the lifecycle backlog stage names when no workflow stage matches.
            return stage === "idea" || stage === "refine" || stage === "ready";
        }

        // ticketInRefinement reports whether a story is in the preparation phase: a
        // draft sitting in a backlog stage. Refinement happens in place on such a
        // ticket — there is no literal "refine" stage.
        function ticketInRefinement(ticket) {
            return Boolean(ticket && ticket.draft && ticketStageIsBacklog(ticket));
        }

        // workflowRequiresReady reports whether a workflow gates work behind a
        // readiness pipeline: it has backlog stages and/or a "ready" stage. Such a
        // workflow indicates a story must be readied (refined) before it is assigned.
        function workflowRequiresReady(workflow) {
            if (!workflow || !Array.isArray(workflow.stages)) return false;
            return workflow.stages.some((s) =>
                s.is_backlog_stage || String(s.name || "").toLowerCase() === "ready");
        }

        // ticketIsReadyForAssignment reports whether a story has been readied for
        // work. Readiness is the draft flag clearing (via refinement approval /
        // MarkTicketReady), not a particular stage.
        function ticketIsReadyForAssignment(ticket) {
            return Boolean(ticket && !ticket.draft);
        }

        // ticketIsRefined reports whether a backlog story's refinement has produced a
        // ready recommendation — it's "refined" and can be promoted to development.
        function ticketIsRefined(ticket) {
            return Boolean(ticket && ticket.recommended_ready && ticketStageIsBacklog(ticket));
        }

        // refinementSortRank floats stories that are refining or refined to the top of
        // a lane/list (rank 0) ahead of everything else (rank 1).
        function refinementSortRank(ticket) {
            return (ticketInRefinement(ticket) || ticketIsRefined(ticket)) ? 0 : 1;
        }

        // refinementBadgeHTML returns the "refining…/✦ refining/✓ refined" chip for a
        // story, or "" when it is neither. Shared by board cards and list rows.
        function refinementBadgeHTML(ticket) {
            if (ticketIsRefined(ticket)) {
                return "<span class=\"chip chip-success\">✓ refined</span>";
            }
            if (ticketInRefinement(ticket)) {
                const working = ticket.state === "active" && String(ticket.assignee || "").trim() !== "";
                return working
                    ? "<span class=\"chip chip-refining\"><span class=\"refining-pulse\"></span> refining…</span>"
                    : "<span class=\"chip chip-refining\">✦ refining</span>";
            }
            return "";
        }

        // findDevelopStageName resolves the workflow stage a refined story should move
        // into: a stage literally named "develop", else the first non-backlog stage.
        function findDevelopStageName() {
            const workflow = getCurrentProjectWorkflow();
            const stages = workflow && Array.isArray(workflow.stages) ? workflow.stages : [];
            const explicit = stages.find((s) => String(s.name || "").toLowerCase() === "develop");
            if (explicit) return explicit.name;
            const firstWork = stages.find((s) => !s.is_backlog_stage);
            return firstWork ? firstWork.name : "";
        }

        function getCurrentWorkflow() {
            return state.workflows.find((item) => item.id === state.selectedWorkflowID) || null;
        }

        function normalizedStageName(name) {
            return String(name || "").trim().toLowerCase();
        }

        function readWorkflowSettingRadio(name) {
            const input = document.querySelector("input[name=\"" + name + "\"]:checked");
            return input ? String(input.value || "") : "";
        }

        function buildWorkflowPayloadFromForm() {
            return {
                name: document.getElementById("workflow-name").value.trim(),
                description: document.getElementById("workflow-description").value.trim(),
                approval_policy: readWorkflowSettingRadio("workflow-approval-policy") || "single_role",
                progression_mode: readWorkflowSettingRadio("workflow-progression-mode") || "linear",
            };
        }

        function syncWorkflowDraftFromForm() {
            const current = state.selectedWorkflowDraft || emptyWorkflow();
            state.selectedWorkflowDraft = Object.assign({}, current, buildWorkflowPayloadFromForm());
        }

        async function persistWorkflowSettings() {
            syncWorkflowDraftFromForm();
            const draft = state.selectedWorkflowDraft || emptyWorkflow();
            if (!draft.name) {
                return;
            }
            if (workflowAutosaveInFlight) {
                workflowAutosaveQueued = true;
                return;
            }
            workflowAutosaveInFlight = true;
            try {
                const workflow = normalizeWorkflow(draft.id
                    ? await api("/api/workflows/" + draft.id, { method: "PUT", body: JSON.stringify(buildWorkflowPayloadFromForm()) })
                    : await api("/api/workflows", { method: "POST", body: JSON.stringify(buildWorkflowPayloadFromForm()) }));
                state.selectedWorkflowID = workflow.id;
                await Promise.all([loadWorkflows(), loadProjects(), loadRoles()]);
                state.selectedWorkflowDraft = Object.assign({}, getCurrentWorkflow() ? structuredClone(getCurrentWorkflow()) : workflow, state.selectedWorkflowDraft, { id: workflow.id });
                renderAll();
            } catch (error) {
                setNotice(error.message, true);
            } finally {
                workflowAutosaveInFlight = false;
                if (workflowAutosaveQueued) {
                    workflowAutosaveQueued = false;
                    persistWorkflowSettings();
                }
            }
        }

        function scheduleWorkflowAutosave() {
            syncWorkflowDraftFromForm();
            if (workflowAutosaveTimer) {
                clearTimeout(workflowAutosaveTimer);
            }
            workflowAutosaveTimer = setTimeout(() => {
                workflowAutosaveTimer = null;
                persistWorkflowSettings();
            }, 350);
        }

        function cancelWorkflowAutosave() {
            if (workflowAutosaveTimer) {
                clearTimeout(workflowAutosaveTimer);
                workflowAutosaveTimer = null;
            }
            workflowAutosaveQueued = false;
        }

        function workflowHasDuplicateStageName(workflow, stageName, excludeStageID) {
            if (!workflow || !Array.isArray(workflow.stages)) {
                return false;
            }
            const target = normalizedStageName(stageName);
            if (!target) {
                return false;
            }
            return workflow.stages.some((stage) => Number(stage.id) !== Number(excludeStageID) && normalizedStageName(stage.name) === target);
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
            const allStages = workflow && workflow.stages ? workflow.stages : [];
            const stageMap = new Map(allStages.map((stage) => [stage.name, stage]));
            const sel = state.selectedReleaseID;

            let primaryNames;
            if (sel === "backlog") {
                // Show stages marked as backlog stages; fall back to hardcoded list if workflow has none flagged.
                const backlogStages = allStages.filter((s) => s.is_backlog_stage);
                primaryNames = backlogStages.length > 0
                    ? backlogStages.map((s) => s.name)
                    : BACKLOG_BOARD_STAGES;
            } else if (sel && sel !== "") {
                // Show stages NOT marked as backlog stages; fall back to hardcoded list if workflow has none flagged.
                const releaseStages = allStages.filter((s) => !s.is_backlog_stage);
                primaryNames = releaseStages.length > 0
                    ? releaseStages.map((s) => s.name)
                    : RELEASE_BOARD_STAGES;
            } else {
                primaryNames = getStageOptions();
            }

            // Safety net: any stage present in currently-visible tickets gets a column.
            // Merge everything and sort by workflow sort_order so the sequence always
            // matches the workflow definition regardless of load timing.
            const visibleTickets = sel === "backlog" || !sel || sel === ""
                ? state.tickets
                : releaseFilterTickets(state.tickets);
            const presentStages = new Set(visibleTickets.map((t) => t.stage).filter(Boolean));

            // Use sort_order from the stage object itself (not array index) for stability.
            const workflowOrderMap = new Map(allStages.map((s) => [
                s.name,
                typeof s.sort_order === "number" ? s.sort_order : 9999,
            ]));

            const allNeeded = new Set([...primaryNames, ...presentStages]);
            const orderedNames = [...allNeeded].sort((a, b) => {
                const ia = workflowOrderMap.has(a) ? workflowOrderMap.get(a) : 9999;
                const ib = workflowOrderMap.has(b) ? workflowOrderMap.get(b) : 9999;
                return ia - ib;
            });

            return orderedNames.map((name) => ({
                name,
                workflowStageID: (stageMap.get(name) || {}).id || null,
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
            const processNavEl = document.getElementById("process-nav");
            if (processNavEl) {
                processNavEl.addEventListener("click", (event) => {
                    const button = event.target.closest("button[data-view]");
                    if (!button) {
                        return;
                    }
                    switchView(button.dataset.view);
                });
            }
            const adminNavEl = document.getElementById("admin-nav");
            if (adminNavEl) {
                adminNavEl.addEventListener("click", (event) => {
                    const button = event.target.closest("button[data-view]");
                    if (!button) {
                        return;
                    }
                    switchView(button.dataset.view);
                });
            }
            const settingsSubnav = document.getElementById("settings-subnav");
            if (settingsSubnav) {
                settingsSubnav.addEventListener("click", (event) => {
                    const btn = event.target.closest("[data-settings-tab]");
                    if (btn) switchSettingsTab(btn.dataset.settingsTab);
                });
            }
            const ticketSubnav = document.getElementById("ticket-subnav");
            if (ticketSubnav) {
                ticketSubnav.addEventListener("click", (event) => {
                    const btn = event.target.closest("[data-ticket-tab]");
                    if (btn) switchTicketTab(btn.dataset.ticketTab);
                });
            }
            els.mainNav.addEventListener("dragstart", (event) => {
                const button = event.target.closest("button[data-view]");
                if (!button) {
                    return;
                }
                navDragView = String(button.dataset.view || "").trim();
                if (!navDragView) {
                    return;
                }
                button.classList.add("dragging");
                if (event.dataTransfer) {
                    event.dataTransfer.effectAllowed = "move";
                    event.dataTransfer.setData("text/plain", navDragView);
                }
            });
            els.mainNav.addEventListener("dragend", () => {
                navDragView = "";
                els.mainNav.querySelectorAll("button.dragging").forEach((button) => button.classList.remove("dragging"));
            });
            els.mainNav.addEventListener("dragover", (event) => {
                const targetButton = event.target.closest("button[data-view]");
                if (!targetButton || !navDragView || targetButton.dataset.view === navDragView) {
                    return;
                }
                event.preventDefault();
                if (event.dataTransfer) {
                    event.dataTransfer.dropEffect = "move";
                }
            });
            els.mainNav.addEventListener("drop", (event) => {
                const targetButton = event.target.closest("button[data-view]");
                if (!targetButton || !navDragView) {
                    return;
                }
                event.preventDefault();
                const targetView = String(targetButton.dataset.view || "").trim();
                if (!targetView || targetView === navDragView) {
                    return;
                }
                const nextOrder = sanitizeNavOrder(state.navOrder);
                const fromIndex = nextOrder.indexOf(navDragView);
                const toIndex = nextOrder.indexOf(targetView);
                if (fromIndex < 0 || toIndex < 0 || fromIndex === toIndex) {
                    return;
                }
                nextOrder.splice(fromIndex, 1);
                nextOrder.splice(toIndex, 0, navDragView);
                state.navOrder = nextOrder;
                storeNavOrder(nextOrder);
                renderMainNav();
                switchView(state.currentView, { persist: false, restoreScroll: false });
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
            if (viewName !== "documents") {
                clearDocumentDropState();
            }
            if (settings.persist !== false) {
                storeSelectedView(viewName);
            }
            document.querySelectorAll("#main-nav button[data-view], #admin-nav button[data-view]").forEach((button) => {
                button.classList.toggle("active", button.dataset.view === viewName);
            });
            document.querySelectorAll(".view").forEach((view) => {
                view.classList.toggle("active", view.id === "view-" + viewName);
            });
            if (settings.restoreScroll !== false) {
                restoreCurrentViewScroll();
            }
            if (viewName === "admin-summary") {
                renderAdminSummary();
            }
            if (viewName === "context") {
                refreshContextView();
            }
            if (viewName === "chat") {
                refreshChatView();
            }
            if (viewName === "access") {
                refreshAccessView();
            }
            if (viewName === "settings") {
                let tab = "organisation";
                try { tab = localStorage.getItem("site2.settingsTab") || "organisation"; } catch (e) { /* ignore */ }
                switchSettingsTab(tab);
            }
        }

        function switchSettingsTab(tabName) {
            const name = String(tabName || "organisation");
            document.querySelectorAll("#settings-subnav .seg-btn").forEach((btn) => {
                btn.classList.toggle("active", btn.dataset.settingsTab === name);
            });
            document.querySelectorAll("#view-settings .settings-panel").forEach((panel) => {
                panel.classList.toggle("active", panel.dataset.settingsPanel === name);
            });
            if (name === "email") { loadEmailSettings(); }
            try { localStorage.setItem("site2.settingsTab", name); } catch (e) { /* ignore */ }
        }

        function switchTicketTab(tabName) {
            const name = String(tabName || "details");
            document.querySelectorAll("#ticket-subnav .seg-btn").forEach((btn) => {
                btn.classList.toggle("active", btn.dataset.ticketTab === name);
            });
            document.querySelectorAll("#ticket-modal .ticket-panel").forEach((panel) => {
                panel.classList.toggle("active", panel.dataset.ticketPanel === name);
            });
        }

        function programmeLabelForProject(project) {
            if (!project || !project.programme_id) return "";
            const prog = (state.programmes || []).find((p) => p.id === project.programme_id);
            return prog ? " · <span class=\"chip\" style=\"font-size:0.7rem;padding:1px 5px\">" + escapeHTML(prog.name) + "</span>" : "";
        }

        function renderProjectMenu() {
            const current = getCurrentProject();
            const defaultProjectID = currentDefaultProjectID();
            const currentIsDefault = current && defaultProjectID === current.id;
            els.projectMenuButton.textContent = current ? (current.title + " (" + current.prefix + ")" + (currentIsDefault ? " · default" : "")) : "Projects";
            const otherProjects = state.projects.filter((project) => project.id !== state.selectedProjectID);
            els.projectMenuList.innerHTML = otherProjects.length
                ? otherProjects.map((project) => {
                    const label = project.title + " (" + project.prefix + ")" + (defaultProjectID === project.id ? " · default" : "");
                    const programmeBadge = programmeLabelForProject(project);
                    return "<button type=\"button\" class=\"dropdown-item\" data-project-switch=\"" + project.id + "\">" + escapeHTML(label) + programmeBadge + "</button>";
                }).join("")
                : "<div class=\"dropdown-label\">No other projects</div>";
        }

        async function selectProject(projectID) {
            state.selectedProjectID = Number(projectID);
            storeSelectedProjectID(state.selectedProjectID);
            state.selectedProjectDraft = getCurrentProject() ? structuredClone(getCurrentProject()) : emptyProject();
            populateTicketTypeAndStageSelects();
            await loadProjectAgentModelConfig();
            await Promise.all([loadTickets(), loadReleases(), loadDocuments(), loadProjectAccessRequests(), loadProjectHistory(), loadMyProjectAccessRequests(), loadMyNotifications(), loadProjectAgents()]);
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
            if (state.auth && state.status && state.status.authenticated === false) {
                throw authError("session expired");
            }
            setServerVersion(serverVersionFromStatus(state.status));
            const username = (state.status.user && state.status.user.username) || "user";
            els.accountMenuButton.textContent = username.charAt(0).toUpperCase();
            els.accountMenuName.textContent = username;
        }

        async function loadPlans() {
            if (!isAdmin()) {
                state.plans = [];
                state.defaultPlan = null;
                state.selectedPlanSlug = "";
                state.selectedPlanDraft = emptyPlan();
                return;
            }
            const [plans, defaultPlan] = await Promise.all([
                apiClient.listPlans(),
                apiClient.getDefaultPlan(),
            ]);
            state.plans = Array.isArray(plans) ? plans.map(normalizePlan) : [];
            state.defaultPlan = defaultPlan ? normalizePlan(defaultPlan) : null;
            const selectedSlug = state.selectedPlanSlug;
            const fallbackSlug = (state.defaultPlan && state.defaultPlan.slug) || (state.plans[0] && state.plans[0].slug) || "";
            state.selectedPlanSlug = state.plans.some((plan) => plan.slug === selectedSlug) ? selectedSlug : fallbackSlug;
            state.selectedPlanDraft = getCurrentPlan() ? structuredClone(getCurrentPlan()) : emptyPlan();
        }

        async function loadPublicStatus() {
            try {
                state.status = await api("/api/status", { method: "GET", auth: false });
                setServerVersion(serverVersionFromStatus(state.status));
                if (state.status && state.status.user) {
                    const username = state.status.user.username || "user";
                    els.accountMenuButton.textContent = username.charAt(0).toUpperCase();
                    els.accountMenuName.textContent = username;
                }
            } catch (error) {
                state.status = null;
                setServerVersion("", true);
            }
            syncRegistrationUI();
        }

        async function loadSystemAgentModelConfig() {
            try {
                const response = await api("/api/config/agent-model");
                state.systemAgentModelConfig = normalizeAgentModelConfig(response);
            } catch (error) {
                state.systemAgentModelConfig = emptyAgentModelConfig();
            }
        }

        async function loadProjectAgentModelConfig() {
            if (!state.selectedProjectID) {
                state.projectAgentModelConfig = emptyAgentModelConfig();
                return;
            }
            try {
                const response = await api("/api/projects/" + state.selectedProjectID + "/agent-model");
                state.projectAgentModelConfig = normalizeAgentModelConfig(response);
            } catch (error) {
                state.projectAgentModelConfig = emptyAgentModelConfig();
            }
        }

        async function loadProjects() {
            const projects = await api("/api/projects");
            state.projects = Array.isArray(projects) ? projects.map(normalizeProject) : [];
            if (!state.selectedProjectID) {
                state.selectedProjectID = loadStoredSelectedProjectID();
            }
            if (!state.selectedProjectID) {
                state.selectedProjectID = currentDefaultProjectID();
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
            await loadProjectAgentModelConfig();
        }

        async function loadProjectAccessRequests() {
            const project = getCurrentProject();
            state.projectAccessRequests = [];
            state.projectAccessReviewEnabled = false;
            if (!project || !project.id) {
                return;
            }
            try {
                const requests = await apiClient.listProjectAccessRequests(project.prefix || project.id, "");
                state.projectAccessRequests = Array.isArray(requests) ? requests.map(normalizeProjectAccessRequest) : [];
                state.projectAccessReviewEnabled = true;
            } catch (error) {
                if (isPermissionErrorMessage(error && error.message)) {
                    return;
                }
                console.warn("failed to load project access requests", error);
            }
        }

        async function loadProjectHistory() {
            const project = getCurrentProject();
            state.projectHistory = [];
            state.projectHistoryError = "";
            if (!project || !project.id) {
                return;
            }
            try {
                const events = await apiClient.listProjectHistory(project.prefix || project.id, { limit: 10 });
                state.projectHistory = Array.isArray(events) ? events : [];
            } catch (error) {
                state.projectHistoryError = String(error && error.message || "Failed to load project history.");
                console.warn("failed to load project history", error);
            }
        }

        async function loadMyProjectAccessRequests() {
            state.myProjectAccessRequests = [];
            try {
                const requests = await apiClient.listMyProjectAccessRequests("");
                state.myProjectAccessRequests = Array.isArray(requests) ? requests.map(normalizeProjectAccessRequest) : [];
            } catch (error) {
                console.warn("failed to load my project access requests", error);
            }
        }

        async function loadMyNotifications() {
            state.myNotifications = [];
            try {
                const notifications = await apiClient.listMyNotifications("", 20);
                state.myNotifications = Array.isArray(notifications) ? notifications : [];
            } catch (error) {
                console.warn("failed to load notifications", error);
            }
        }

        async function loadTickets() {
            if (!state.selectedProjectID) {
                state.tickets = [];
                state.interventions = [];
                state.interventionReport = null;
                state.interventionTrends = [];
                state.interventionDrilldown = null;
                return;
            }
            const [tickets, interventions, interventionReport, interventionTrends, interventionDrilldown] = await Promise.all([
                api("/api/projects/" + state.selectedProjectID + "/tickets"),
                api("/api/projects/" + state.selectedProjectID + "/interventions"),
                apiWithFallback("/api/projects/" + state.selectedProjectID + "/interventions/report", null),
                apiWithFallback("/api/projects/" + state.selectedProjectID + "/interventions/trends?days=7", []),
                apiWithFallback("/api/projects/" + state.selectedProjectID + "/interventions/drilldown?escalation_hours=24", null),
            ]);
            state.tickets = Array.isArray(tickets) ? tickets.map(normalizeTicket) : [];
            state.interventions = Array.isArray(interventions) ? interventions.map(normalizeTicket) : [];
            state.interventionReport = interventionReport;
            state.interventionTrends = Array.isArray(interventionTrends) ? interventionTrends : [];
            state.interventionDrilldown = interventionDrilldown;
            const [dependencyEntries, interventionDetailEntries] = await Promise.all([
                Promise.all(state.tickets.map(async (ticket) => {
                    try {
                        const dependencies = await api("/api/tickets/" + ticket.id + "/dependencies");
                        const ids = Array.isArray(dependencies) ? dependencies.map((entry) => String(entry.depends_on || "")).filter(Boolean) : [];
                        return [String(ticket.id), ids];
                    } catch (error) {
                        return [String(ticket.id), []];
                    }
                })),
                Promise.all(state.interventions.map(async (ticket) => {
                    const nested = ticket.intervention_state;
                    const [workItems, interventionState, history, comments] = await Promise.all([
                        api("/api/tickets/" + ticket.id + "/work-items?limit=1").catch(() => []),
                        nested && typeof nested === "object" && nested.state
                            ? Promise.resolve(nested)
                            : api("/api/tickets/" + ticket.id + "/intervention-state").catch(() => ({ ticket_id: ticket.id, state: "open", owner_name: "" })),
                        api("/api/tickets/" + ticket.id + "/history?limit=3").catch(() => []),
                        api("/api/tickets/" + ticket.id + "/comments").catch(() => []),
                    ]);
                    return [String(ticket.id), {
                        workItems: Array.isArray(workItems) ? workItems : [],
                        interventionState,
                        history: Array.isArray(history) ? history : [],
                        comments: Array.isArray(comments) ? comments.slice(-3).reverse() : [],
                    }];
                })),
            ]);
            state.dependencyIndex = Object.fromEntries(dependencyEntries);
            state.interventionWorkItems = Object.fromEntries(interventionDetailEntries.map(([ticketID, detail]) => [ticketID, detail.workItems]));
            state.interventionStates = Object.fromEntries(interventionDetailEntries.map(([ticketID, detail]) => [ticketID, detail.interventionState]));
            state.interventionHistory = Object.fromEntries(interventionDetailEntries.map(([ticketID, detail]) => [ticketID, detail.history]));
            state.interventionComments = Object.fromEntries(interventionDetailEntries.map(([ticketID, detail]) => [ticketID, detail.comments]));
        }

        async function loadReleases() {
            if (!state.selectedProjectID) {
                state.releases = [];
                return;
            }
            try {
                state.releases = await apiClient.listReleases(state.selectedProjectID);
            } catch (e) {
                state.releases = [];
            }
            // Restore selected release from localStorage
            const saved = localStorage.getItem("site2.release." + state.selectedProjectID);
            if (saved !== null) {
                state.selectedReleaseID = saved;
            } else {
                state.selectedReleaseID = "backlog";
            }
        }

        async function loadProjectAgents() {
            if (!state.selectedProjectID) { state.projectAgents = []; return; }
            try {
                state.projectAgents = await apiClient.get("/api/projects/" + state.selectedProjectID + "/agents");
            } catch (e) {
                state.projectAgents = [];
            }
        }

        async function loadDocuments() {
            if (!canAccessPanel("documents")) {
                state.documents = [];
                state.selectedDocumentID = null;
                return;
            }
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
            // /api/roles is admin-only; non-admins must still be able to boot, so
            // degrade to an empty list rather than aborting the whole load.
            try {
                const roles = await api("/api/roles");
                state.roles = Array.isArray(roles) ? roles.map(normalizeRole) : [];
            } catch (error) {
                state.roles = [];
            }
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
                if (state.selectedTeamID) {
                    await fetchTeamMembers(state.selectedTeamID);
                }
            } catch (error) {
                state.teams = [];
            }
        }

        async function loadConfigSettings() {
            if (!isAdmin()) {
                state.configSettings = [];
                state.selectedConfigSettingKey = "";
                return;
            }
            try {
                const settings = await api("/api/config/settings");
                state.configSettings = Array.isArray(settings) ? settings.map((item) => ({
                    key: String(item.key || "").trim(),
                    value: String(item.value || ""),
                })).filter((item) => item.key) : [];
                if (!state.selectedConfigSettingKey || !state.configSettings.some((item) => item.key === state.selectedConfigSettingKey)) {
                    state.selectedConfigSettingKey = state.configSettings.length ? state.configSettings[0].key : "";
                }
            } catch (error) {
                state.configSettings = [];
                state.selectedConfigSettingKey = "";
                throw error;
            }
        }

        async function loadOrchestratorConfig() {
            const intervalInput = document.getElementById("orchestrator-interval");
            const heartbeatInput = document.getElementById("orchestrator-heartbeat-timeout");
            if (!intervalInput || !heartbeatInput) {
                return;
            }
            const config = await api("/api/config/orchestrator");
            intervalInput.value = config && config.interval_seconds != null ? config.interval_seconds : "";
            heartbeatInput.value = config && config.heartbeat_timeout_seconds != null ? config.heartbeat_timeout_seconds : "";
            const idleInput = document.getElementById("orchestrator-refinement-idle");
            if (idleInput) {
                idleInput.value = config && config.refinement_idle_minutes != null ? config.refinement_idle_minutes : "";
            }
        }

        async function loadPasskeys() {
            if (!state.auth) {
                state.passkeys = [];
                state.passkeyError = "";
                return;
            }
            try {
                const passkeys = await apiClient.listMyPasskeys();
                state.passkeys = Array.isArray(passkeys) ? passkeys.map(normalizePasskeyCredential) : [];
                state.passkeyError = "";
            } catch (error) {
                state.passkeys = [];
                state.passkeyError = error.message;
            }
        }

        async function loadMyPanels() {
            try {
                const res = await api("/api/me/panels");
                state.panels = Array.isArray(res && res.panels) ? res.panels : [];
            } catch (error) {
                // If the panel set can't be loaded, stay optimistic (null) so the
                // nav isn't hidden; server-side enforcement still applies.
                state.panels = null;
            }
        }

        async function refreshAll() {
            await loadStatus();
            await Promise.all([loadMyPanels(), loadSystemAgentModelConfig(), loadWorkflows(), loadRoles(), loadProjects(), loadAgents(), loadTeams(), loadPlans(), loadPasskeys(), fetchUsers(), loadOrg(), loadProgrammes()]);
            await loadConfigSettings();
            try {
                await loadOrchestratorConfig();
            } catch (error) {
                /* non-admins cannot read orchestrator config; ignore */
            }
            renderProjectMenu();
            populateWorkflowSelects();
            populateTicketTypeAndStageSelects();
            populateTeamParentSelect();
            await Promise.all([loadTickets(), loadReleases(), loadDocuments(), loadProjectAccessRequests(), loadProjectHistory(), loadMyProjectAccessRequests(), loadMyNotifications(), loadProjectAgents()]);
            renderAll();
        }

        function showAuthenticatedShell() {
            els.loginScreen.classList.add("hidden");
            els.appShell.classList.remove("hidden");
            els.loginError.textContent = "";
            state.scrollPersistenceReady = true;
            restoreCurrentViewScroll();
        }

        function showRegisterForm() {
            if (!state.status || !state.status.registration_enabled) {
                return;
            }
            els.loginForm.classList.add("hidden");
            els.registerForm.classList.remove("hidden");
            els.loginError.textContent = "";
            focusRegisterUsername();
        }

        function showLoginForm() {
            els.registerForm.classList.add("hidden");
            els.loginForm.classList.remove("hidden");
            focusLoginUsername();
        }

        function syncRegistrationUI() {
            const enabled = Boolean(state.status && state.status.registration_enabled);
            els.showRegisterButton.classList.toggle("hidden", !enabled);
            if (els.loginPasskeyButton) {
                els.loginPasskeyButton.classList.toggle("hidden", !browserSupportsPasskeys());
            }
            if (els.registerHelp) {
                els.registerHelp.textContent = state.status && state.status.registration_auto_approve === false
                    ? "Leave password blank to let the server generate one. New accounts require admin approval before sign-in."
                    : "Leave password blank to let the server generate one.";
            }
            if (!enabled) {
                els.registerForm.classList.add("hidden");
                els.loginForm.classList.remove("hidden");
            }
        }

        function showLoginScreen() {
            state.scrollPersistenceReady = false;
            els.appShell.classList.add("hidden");
            els.loginScreen.classList.remove("hidden");
            if (els.accountModal) {
                els.accountModal.classList.remove("open");
            }
            state.accountModalOpen = false;
            syncRegistrationUI();
            showLoginForm();
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

        function startAgentBarPoller() {
            if (state.agentBarPollTimer) return;
            state.agentBarPollTimer = setInterval(async () => {
                if (!state.auth || !state.selectedProjectID) return;
                try {
                    await loadProjectAgents();
                    renderProjectAgentBar();
                } catch (_) { /* ignore */ }
            }, 15000);
        }

        // ── Chat / multiplayer rooms (TK-118 / S5) ───────────────────────
        function roomScope(room) {
            if (room.slug && room.slug.indexOf("dm-") === 0) { return "people"; }
            if (!room.project_id) { return "global"; }
            return room.ticket_id ? "breakout" : "project";
        }
        // Display label for a room. DMs show the other participant(s), not "DM: me, them".
        function roomLabel(room) {
            const name = String(room.name || "room");
            if (room.slug && room.slug.indexOf("dm-") === 0 && name.indexOf("DM: ") === 0) {
                const me = (state.status && state.status.user && state.status.user.username) || "";
                const others = name.slice(4).split(", ").filter((n) => n && n.toLowerCase() !== me.toLowerCase());
                if (others.length) { return others.join(", "); }
            }
            return name;
        }
        // @name and #label become highlighted entities in the message feed (S6).
        function highlightRoomText(body) {
            return escapeHTML(String(body || ""))
                .replace(/@([A-Za-z0-9][A-Za-z0-9_-]*)/g, "<span class=\"chat-mention\">@$1</span>")
                .replace(/#([A-Za-z0-9][A-Za-z0-9_-]*)/g, "<span class=\"chat-label\">#$1</span>");
        }
        function refreshChatView() {
            loadRooms().then(() => ensureDefaultRooms()).then(() => {
                renderRoomsList();
                // Always land the user in a room so they can start chatting. Keep
                // the current room if it's still present; otherwise pick a sensible
                // default (the public global room, else the first available).
                const roomID = (state.activeRoomID && state.rooms.some((r) => r.room_id === state.activeRoomID))
                    ? state.activeRoomID
                    : pickDefaultRoomID();
                if (roomID) {
                    selectRoom(roomID);
                }
            });
        }
        function pickDefaultRoomID() {
            if (!state.rooms.length) { return 0; }
            const general = state.rooms.find((r) => roomScope(r) === "global" && r.visibility === "public");
            return (general || state.rooms[0]).room_id;
        }
        // Automatically create a public global room and a project room the first
        // time chat is opened without them.
        function ensureDefaultRooms() {
            const tasks = [];
            if (!state.rooms.some((r) => roomScope(r) === "global" && r.visibility === "public")) {
                tasks.push(apiClient.post("/api/rooms", { name: "general", visibility: "public" }).catch(() => {}));
            }
            if (state.selectedProjectID && !state.rooms.some((r) => roomScope(r) === "project" && r.project_id === state.selectedProjectID)) {
                const proj = (state.projects || []).find((p) => p.id === state.selectedProjectID);
                const pname = proj ? String(proj.prefix || proj.title || "project").toLowerCase() : "project";
                tasks.push(apiClient.post("/api/rooms", { name: pname, visibility: "public", project_id: state.selectedProjectID }).catch(() => {}));
            }
            // Provision the user's personal agent + its DM (appears under People & Agents).
            if (!state.rooms.some((r) => roomScope(r) === "people")) {
                tasks.push(api("/api/me/agent").catch(() => {}));
            }
            if (!tasks.length) { return Promise.resolve(); }
            return Promise.all(tasks).then(() => loadRooms());
        }
        function loadRooms() {
            return api("/api/rooms").then((rooms) => {
                state.rooms = Array.isArray(rooms) ? rooms : [];
            }).catch(() => { state.rooms = []; });
        }
        function renderRoomsList() {
            const list = document.getElementById("chat-rooms-list");
            if (!list) { return; }
            if (!state.rooms.length) {
                list.innerHTML = "<div class=\"meta\" style=\"padding:10px\">No rooms yet — create one.</div>";
                return;
            }
            const groups = [["global", "Global"], ["project", "Project"], ["people", "People & Agents"], ["breakout", "Breakouts"]];
            const sortRooms = (rooms) => rooms.slice().sort((a, b) => {
                if (state.chatSort === "alpha") {
                    return roomLabel(a).toLowerCase().localeCompare(roomLabel(b).toLowerCase());
                }
                return String(b.updated_at || "").localeCompare(String(a.updated_at || "")); // recency
            });
            let html = "";
            groups.forEach((g) => {
                const scoped = sortRooms(state.rooms.filter((r) => roomScope(r) === g[0]));
                if (!scoped.length) { return; }
                const collapsed = Boolean(state.chatCollapsed[g[0]]);
                // Sum unread across the group's non-active rooms (shown on the header when collapsed).
                const groupUnread = scoped.reduce((sum, r) => sum + (r.room_id === state.activeRoomID ? 0 : (r.unread_count || 0)), 0);
                const headerBadge = (collapsed && groupUnread > 0) ? "<span class=\"chat-unread-badge\">" + (groupUnread > 99 ? "99+" : groupUnread) + "</span>" : "";
                html += "<button type=\"button\" class=\"chat-room-group\" data-chat-group=\"" + g[0] + "\">" +
                    "<span class=\"chat-group-caret\">" + (collapsed ? "▸" : "▾") + "</span> " + escapeHTML(g[1]) + headerBadge + "</button>";
                if (collapsed) { return; }
                html += scoped.map((r) => {
                    const active = r.room_id === state.activeRoomID;
                    const n = active ? 0 : (r.unread_count || 0);
                    const badge = n > 0 ? "<span class=\"chat-unread-badge\">" + (n > 99 ? "99+" : n) + "</span>" : "";
                    const icon = g[0] === "people" ? "@ " : (r.visibility === "private" ? "🔒 " : "# ");
                    return "<button type=\"button\" class=\"chat-room-item" + (active ? " active" : "") + (n > 0 ? " unread" : "") + "\" data-room-id=\"" + r.room_id + "\">" +
                        icon + escapeHTML(roomLabel(r)) + badge +
                        "<span class=\"chat-room-count\">" + (r.member_count || 0) + "</span></button>";
                }).join("");
            });
            list.innerHTML = html;
            // Wire group collapse toggles.
            list.querySelectorAll("[data-chat-group]").forEach((el) => {
                el.addEventListener("click", () => {
                    const key = el.dataset.chatGroup;
                    state.chatCollapsed[key] = !state.chatCollapsed[key];
                    try { localStorage.setItem("site2.chatCollapsed", JSON.stringify(state.chatCollapsed)); } catch (e) { /* ignore */ }
                    renderRoomsList();
                });
            });
        }
        function selectRoom(roomID) {
            // Remember where we came from so /leave can return there.
            if (state.activeRoomID && state.activeRoomID !== roomID) {
                state.lastRoomID = state.activeRoomID;
            }
            state.activeRoomID = roomID;
            const room = state.rooms.find((r) => r.room_id === roomID);
            const titleEl = document.getElementById("chat-room-title");
            const topicEl = document.getElementById("chat-room-topic");
            if (titleEl) { titleEl.textContent = room ? room.name : "Room"; }
            if (topicEl) { topicEl.textContent = room ? (room.topic || "") : ""; }
            renderRoomsList();
            const input = document.getElementById("chat-composer-input");
            const sendBtn = document.getElementById("chat-send-button");
            if (input) {
                input.disabled = false;
                // Focus the composer so the user can type immediately on entering a room.
                input.focus();
            }
            if (sendBtn) { sendBtn.disabled = false; }
            const joinBtn = document.getElementById("chat-join-button");
            const leaveBtn = document.getElementById("chat-leave-button");
            if (joinBtn) { joinBtn.hidden = false; }
            // The public global room and a project's room cannot be left.
            if (leaveBtn) { leaveBtn.hidden = Boolean(room && room.permanent); }
            const renameBtn = document.getElementById("chat-rename-button");
            // You can rename a room you own (or as admin).
            if (renameBtn) { renameBtn.hidden = !(room && (room.created_by === currentUserID() || isAdmin())); }
            subscribeRoom(roomID);
            // Loading messages marks the room read server-side; refresh the list
            // afterwards so the unread marker clears.
            loadRoomMessages(roomID).then(() => loadRooms()).then(renderRoomsList);
        }
        function loadRoomMessages(roomID) {
            return api("/api/rooms/" + roomID + "/messages").then((msgs) => {
                if (state.activeRoomID !== roomID) { return; }
                const box = document.getElementById("chat-messages");
                if (!box) { return; }
                box.innerHTML = (Array.isArray(msgs) ? msgs : []).map((m) =>
                    "<div class=\"chat-msg chat-msg-" + escapeHTML(m.kind || "text") + "\" data-message-id=\"" + m.message_id + "\">" +
                    "<span class=\"chat-msg-sender\">" + escapeHTML(m.sender_name || m.sender_id) + "</span> " +
                    "<span class=\"chat-msg-body\">" + highlightRoomText(m.body) + "</span></div>").join("");
                box.scrollTop = box.scrollHeight;
            }).catch(() => {});
        }
        function sendRoomMessage() {
            const input = document.getElementById("chat-composer-input");
            if (!input || !state.activeRoomID) { return; }
            const body = String(input.value || "").trim();
            if (!body) { return; }
            const roomID = state.activeRoomID;
            const isLeave = body.toLowerCase() === "/leave";
            // /msg sends to a private chat WITHOUT moving the sender to it.
            const isMsg = body.toLowerCase().indexOf("/msg ") === 0;
            input.value = "";
            apiClient.post("/api/rooms/" + roomID + "/messages", { body: body })
                .then((msg) => loadRooms().then(() => {
                    renderRoomsList();
                    if (isLeave) {
                        // After leaving, return to the previous room (or a default).
                        selectRoom(roomReturnTarget(roomID));
                        return;
                    }
                    if (isMsg) {
                        // Stay put; the DM was delivered to the other room.
                        loadRoomMessages(roomID);
                        setNotice("Message sent");
                        return;
                    }
                    // /msg routes to a DM room and /new + /join switch rooms — follow
                    // the message if the server placed it somewhere other than here.
                    const target = (msg && msg.room_id && msg.room_id !== roomID) ? msg.room_id : roomID;
                    if (target !== roomID) { selectRoom(target); } else { loadRoomMessages(roomID); }
                }))
                .catch((err) => setNotice("Send failed: " + err.message, true));
        }
        // roomReturnTarget picks where to land after leaving `leftRoomID`: the last
        // room if it still exists, else a sensible default.
        function roomReturnTarget(leftRoomID) {
            if (state.lastRoomID && state.lastRoomID !== leftRoomID && state.rooms.some((r) => r.room_id === state.lastRoomID)) {
                return state.lastRoomID;
            }
            return pickDefaultRoomID();
        }
        function createRoomFromPrompt() {
            Promise.resolve(uiPrompt("New room name", "", "Create")).then((name) => {
                const trimmed = String(name || "").trim();
                if (!trimmed) { return; }
                return apiClient.post("/api/rooms", { name: trimmed }).then((room) => {
                    return loadRooms().then(() => {
                        renderRoomsList();
                        if (room && room.room_id) { selectRoom(room.room_id); }
                    });
                });
            }).catch((err) => setNotice("Create room failed: " + err.message, true));
        }
        // Open (or create) a breakout room scoped to a ticket (S8).
        function openBreakoutRoom(ticket) {
            if (!ticket || !ticket.id) { return; }
            const ticketKey = ticket.id;
            const projectID = ticket.project_id || state.selectedProjectID || null;
            const go = (roomID) => {
                if (typeof closeTicketModal === "function") { closeTicketModal(); }
                switchView("chat");
                loadRooms().then(() => { renderRoomsList(); selectRoom(roomID); });
            };
            loadRooms().then(() => {
                const existing = state.rooms.find((r) => r.ticket_id === ticketKey);
                if (existing) { go(existing.room_id); return; }
                apiClient.post("/api/rooms", { name: "Breakout " + ticketKey, topic: ticket.title || "", project_id: projectID, ticket_id: ticketKey })
                    .then((room) => { if (room && room.room_id) { go(room.room_id); } })
                    .catch((err) => setNotice("Breakout failed: " + err.message, true));
            });
        }
        function subscribeRoom(roomID) {
            try {
                if (state.liveSocket && state.liveSocket.readyState === 1) {
                    state.liveSocket.send(JSON.stringify({ type: "subscribe", room_id: roomID }));
                }
            } catch (e) { /* ignore */ }
        }
        function setupChat() {
            const list = document.getElementById("chat-rooms-list");
            if (list) {
                list.addEventListener("click", (event) => {
                    const item = event.target.closest("[data-room-id]");
                    if (item) { selectRoom(Number(item.dataset.roomId)); }
                });
            }
            const newBtn = document.getElementById("new-room-button");
            if (newBtn) { newBtn.addEventListener("click", createRoomFromPrompt); }
            const breakoutBtn = document.getElementById("ticket-breakout-button");
            if (breakoutBtn) { breakoutBtn.addEventListener("click", () => openBreakoutRoom(state.activeTicket)); }
            // Restore persisted sort + group-collapse preferences.
            try {
                const s = localStorage.getItem("site2.chatSort");
                if (s === "alpha" || s === "recency") { state.chatSort = s; }
                const c = localStorage.getItem("site2.chatCollapsed");
                if (c) { state.chatCollapsed = JSON.parse(c) || {}; }
            } catch (e) { /* ignore */ }
            const sortBtn = document.getElementById("chat-sort-button");
            if (sortBtn) {
                const syncSortLabel = () => { sortBtn.textContent = state.chatSort === "alpha" ? "↕ A–Z" : "↕ Recent"; };
                syncSortLabel();
                sortBtn.addEventListener("click", () => {
                    state.chatSort = state.chatSort === "alpha" ? "recency" : "alpha";
                    try { localStorage.setItem("site2.chatSort", state.chatSort); } catch (e) { /* ignore */ }
                    syncSortLabel();
                    renderRoomsList();
                });
            }
            const renameBtn = document.getElementById("chat-rename-button");
            if (renameBtn) {
                renameBtn.addEventListener("click", () => {
                    if (!state.activeRoomID) { return; }
                    const room = state.rooms.find((r) => r.room_id === state.activeRoomID);
                    Promise.resolve(uiPrompt("Rename room", room ? room.name : "", "Rename")).then((name) => {
                        const trimmed = String(name || "").trim();
                        if (!trimmed) { return null; }
                        return api("/api/rooms/" + state.activeRoomID, { method: "PATCH", body: JSON.stringify({ name: trimmed }) })
                            .then(() => loadRooms()).then(() => { renderRoomsList(); selectRoom(state.activeRoomID); });
                    }).catch((err) => setNotice(err.message || "Rename failed", true));
                });
            }
            const composer = document.getElementById("chat-composer");
            if (composer) { composer.addEventListener("submit", (event) => { event.preventDefault(); sendRoomMessage(); }); }
            const joinBtn = document.getElementById("chat-join-button");
            if (joinBtn) {
                joinBtn.addEventListener("click", () => {
                    if (!state.activeRoomID) { return; }
                    apiClient.post("/api/rooms/" + state.activeRoomID + "/join").then(() => loadRooms()).then(renderRoomsList).catch(() => {});
                });
            }
            const leaveBtn = document.getElementById("chat-leave-button");
            if (leaveBtn) {
                leaveBtn.addEventListener("click", () => {
                    if (!state.activeRoomID) { return; }
                    const left = state.activeRoomID;
                    apiClient.post("/api/rooms/" + left + "/leave")
                        .then(() => loadRooms())
                        .then(() => { renderRoomsList(); selectRoom(roomReturnTarget(left)); })
                        .catch((err) => setNotice(err.message || "Cannot leave this room", true));
                });
            }
        }

        // ── Board keyboard navigation (TK-128) ───────────────────────────
        // Arrow keys and w/a/s/d move focus between ticket cards on the board
        // (and Enter opens the focused ticket) instead of scrolling the page.
        let focusedBoardTicketID = "";
        function boardLanesWithCards() {
            if (!els.ticketBoard) { return []; }
            return Array.from(els.ticketBoard.querySelectorAll("[data-lane-stage]"))
                .map((lane) => Array.from(lane.querySelectorAll("[data-ticket-id]")))
                .filter((cards) => cards.length > 0);
        }
        function focusBoardCard(el) {
            if (!els.ticketBoard) { return; }
            els.ticketBoard.querySelectorAll(".ticket-card.kbd-focus").forEach((c) => c.classList.remove("kbd-focus"));
            if (!el) { return; }
            el.classList.add("kbd-focus");
            el.scrollIntoView({ block: "nearest", inline: "nearest" });
            focusedBoardTicketID = el.dataset.ticketId;
        }
        function moveBoardFocus(dir) {
            const lanes = boardLanesWithCards();
            if (!lanes.length) { return; }
            let li = -1;
            let ci = -1;
            for (let i = 0; i < lanes.length && li < 0; i += 1) {
                for (let j = 0; j < lanes[i].length; j += 1) {
                    if (lanes[i][j].dataset.ticketId === focusedBoardTicketID) {
                        li = i;
                        ci = j;
                        break;
                    }
                }
            }
            if (li < 0) {
                focusBoardCard(lanes[0][0]);
                return;
            }
            if (dir === "up") {
                ci = Math.max(0, ci - 1);
            } else if (dir === "down") {
                ci = Math.min(lanes[li].length - 1, ci + 1);
            } else if (dir === "left") {
                li = Math.max(0, li - 1);
                ci = Math.min(ci, lanes[li].length - 1);
            } else if (dir === "right") {
                li = Math.min(lanes.length - 1, li + 1);
                ci = Math.min(ci, lanes[li].length - 1);
            }
            focusBoardCard(lanes[li][ci]);
        }
        function openFocusedBoardCard() {
            if (!focusedBoardTicketID) { return false; }
            const ticket = (state.tickets || []).find((t) => String(t.id) === focusedBoardTicketID);
            if (ticket) {
                openTicketModal(ticket);
                return true;
            }
            return false;
        }
        function boardKeyboardNavActive() {
            if (state.currentView !== "tickets") { return false; }
            if (paletteEls.overlay && !paletteEls.overlay.classList.contains("hidden")) { return false; }
            if (els.dialogOverlay && !els.dialogOverlay.classList.contains("hidden")) { return false; }
            if (els.ticketModal && els.ticketModal.classList.contains("open")) { return false; }
            if (state.accountModalOpen) { return false; }
            const a = document.activeElement;
            if (a && (a.tagName === "INPUT" || a.tagName === "TEXTAREA" || a.tagName === "SELECT" || a.isContentEditable)) { return false; }
            return true;
        }
        function setupBoardKeyboardNav() {
            document.addEventListener("keydown", (event) => {
                if (event.ctrlKey || event.metaKey || event.altKey || event.shiftKey) { return; }
                if (!boardKeyboardNavActive()) { return; }
                let dir = null;
                switch (event.key) {
                    case "ArrowUp": case "w": case "W": dir = "up"; break;
                    case "ArrowDown": case "s": case "S": dir = "down"; break;
                    case "ArrowLeft": case "a": case "A": dir = "left"; break;
                    case "ArrowRight": case "d": case "D": dir = "right"; break;
                    default: break;
                }
                if (dir) {
                    event.preventDefault();
                    moveBoardFocus(dir);
                    return;
                }
                if (event.key === "Enter" && openFocusedBoardCard()) {
                    event.preventDefault();
                }
            });
        }

        // ── Access roles admin panel (TK-135) ────────────────────────────
        let selectedAccessRoleID = 0;
        let accessGrantablePanels = [];

        function panelLabel(view) {
            const item = NAV_ITEMS.find((n) => n.view === view);
            return item ? item.label : view;
        }

        async function refreshAccessView() {
            if (!isAdmin()) { return; }
            try {
                const [rolesRes, panelsRes] = await Promise.all([api("/api/access-roles"), api("/api/panels")]);
                state.accessRoles = Array.isArray(rolesRes) ? rolesRes : [];
                accessGrantablePanels = (panelsRes && Array.isArray(panelsRes.grantable)) ? panelsRes.grantable : [];
            } catch (error) {
                state.accessRoles = [];
                accessGrantablePanels = [];
            }
            if (!state.users || !state.users.length) {
                try { await fetchUsers(); } catch (e) { /* ignore */ }
            }
            renderAccessView();
        }

        function renderAccessView() {
            renderAccessRoleList();
            renderAccessPanelCheckboxes();
            renderAccessAssignUserOptions();
            renderAccessAssignRoles();
        }

        function renderAccessRoleList() {
            const list = document.getElementById("access-role-list");
            if (!list) { return; }
            if (!state.accessRoles.length) {
                list.innerHTML = "<div class=\"meta\">No access roles yet.</div>";
                return;
            }
            list.innerHTML = state.accessRoles.map((role) => {
                const panels = (role.panels || []).map(panelLabel).join(", ") || "no panels";
                const builtin = role.builtin ? " <span class=\"chip\">builtin</span>" : "";
                const active = role.id === selectedAccessRoleID ? " active" : "";
                return "<button type=\"button\" class=\"access-role-item" + active + "\" data-access-role-id=\"" + role.id + "\">"
                    + "<strong>" + escapeHTML(role.name) + "</strong>" + builtin
                    + "<div class=\"meta\">" + escapeHTML(panels) + "</div></button>";
            }).join("");
            list.querySelectorAll("[data-access-role-id]").forEach((el) => {
                el.addEventListener("click", () => selectAccessRole(Number(el.dataset.accessRoleId)));
            });
        }

        function renderAccessPanelCheckboxes() {
            const box = document.getElementById("access-role-panels");
            if (!box) { return; }
            const role = state.accessRoles.find((r) => r.id === selectedAccessRoleID);
            // A brand-new role defaults to all panels checked.
            const granted = new Set(role ? (role.panels || []) : accessGrantablePanels);
            box.innerHTML = accessGrantablePanels.map((p) =>
                "<label class=\"access-check-row\"><input type=\"checkbox\" data-access-panel=\"" + p + "\""
                + (granted.has(p) ? " checked" : "") + "> " + escapeHTML(panelLabel(p)) + "</label>"
            ).join("");
        }

        function selectAccessRole(id) {
            selectedAccessRoleID = id;
            const role = state.accessRoles.find((r) => r.id === id);
            document.getElementById("access-role-name").value = role ? role.name : "";
            document.getElementById("access-role-description").value = role ? (role.description || "") : "";
            const title = document.getElementById("access-role-editor-title");
            if (title) { title.textContent = role ? ("Edit: " + role.name) : "Role editor"; }
            const del = document.getElementById("delete-access-role-button");
            if (del) { del.style.display = (role && !role.builtin) ? "" : "none"; }
            renderAccessRoleList();
            renderAccessPanelCheckboxes();
        }

        function clearAccessRoleForm() {
            selectedAccessRoleID = 0;
            const nameEl = document.getElementById("access-role-name");
            const descEl = document.getElementById("access-role-description");
            if (nameEl) { nameEl.value = ""; }
            if (descEl) { descEl.value = ""; }
            const title = document.getElementById("access-role-editor-title");
            if (title) { title.textContent = "New role"; }
            renderAccessRoleList();
            renderAccessPanelCheckboxes();
        }

        function selectedAccessPanelKeys() {
            return Array.from(document.querySelectorAll("#access-role-panels [data-access-panel]:checked"))
                .map((el) => el.dataset.accessPanel);
        }

        async function saveAccessRole(event) {
            event.preventDefault();
            const body = {
                name: document.getElementById("access-role-name").value.trim(),
                description: document.getElementById("access-role-description").value.trim(),
                panels: selectedAccessPanelKeys(),
            };
            if (!body.name) { setNotice("Role name is required", true); return; }
            try {
                if (selectedAccessRoleID) {
                    await api("/api/access-roles/" + selectedAccessRoleID, { method: "PUT", body: JSON.stringify(body) });
                } else {
                    await api("/api/access-roles", { method: "POST", body: JSON.stringify(body) });
                }
                setNotice("Access role saved");
                clearAccessRoleForm();
                await refreshAccessView();
            } catch (error) {
                setNotice(error.message || "Failed to save role", true);
            }
        }

        async function deleteAccessRoleAction() {
            if (!selectedAccessRoleID) { return; }
            try {
                await api("/api/access-roles/" + selectedAccessRoleID, { method: "DELETE" });
                setNotice("Access role deleted");
                clearAccessRoleForm();
                await refreshAccessView();
            } catch (error) {
                setNotice(error.message || "Failed to delete role", true);
            }
        }

        function renderAccessAssignUserOptions() {
            const sel = document.getElementById("access-assign-user");
            if (!sel) { return; }
            const users = (state.users || []).filter((u) => (u.user_type || "user") !== "agent");
            const current = sel.value;
            sel.innerHTML = "<option value=\"\">Select a user…</option>" + users.map((u) =>
                "<option value=\"" + escapeHTML(u.user_id || u.id) + "\">" + escapeHTML(u.username || u.user_id) + "</option>"
            ).join("");
            if (current) { sel.value = current; }
        }

        async function renderAccessAssignRoles() {
            const box = document.getElementById("access-assign-roles");
            const sel = document.getElementById("access-assign-user");
            if (!box || !sel) { return; }
            const userID = sel.value;
            const assigned = new Set();
            if (userID) {
                try {
                    const res = await api("/api/access-roles/memberships?user_id=" + encodeURIComponent(userID));
                    (res && res.role_ids ? res.role_ids : []).forEach((id) => assigned.add(id));
                } catch (e) { /* ignore */ }
            }
            box.innerHTML = state.accessRoles.map((role) =>
                "<label class=\"access-check-row\"><input type=\"checkbox\" data-assign-role=\"" + role.id + "\""
                + (assigned.has(role.id) ? " checked" : "") + "> " + escapeHTML(role.name) + "</label>"
            ).join("") || "<div class=\"meta\">No roles to assign.</div>";
        }

        async function saveAccessAssignment(event) {
            event.preventDefault();
            const sel = document.getElementById("access-assign-user");
            const userID = sel ? sel.value : "";
            if (!userID) { setNotice("Select a user first", true); return; }
            const roleIDs = Array.from(document.querySelectorAll("#access-assign-roles [data-assign-role]:checked"))
                .map((el) => Number(el.dataset.assignRole));
            try {
                await api("/api/access-roles/memberships", { method: "PUT", body: JSON.stringify({ user_id: userID, role_ids: roleIDs }) });
                setNotice("Roles assigned");
            } catch (error) {
                setNotice(error.message || "Failed to assign roles", true);
            }
        }

        // ── Email (SMTP) settings (TK-132) ───────────────────────────────
        async function loadEmailSettings() {
            try {
                const cfg = await api("/api/email/settings");
                renderEmailForm(cfg || {});
            } catch (error) {
                /* non-admins never see this tab; ignore */
            }
        }
        function renderEmailForm(cfg) {
            const set = (id, val) => { const el = document.getElementById(id); if (el) { el.value = val == null ? "" : val; } };
            const enabled = document.getElementById("email-enabled");
            if (enabled) { enabled.checked = Boolean(cfg.enabled); }
            set("email-host", cfg.host);
            set("email-port", cfg.port || 587);
            set("email-security", cfg.security || "starttls");
            set("email-username", cfg.username);
            set("email-from-address", cfg.from_address);
            set("email-from-name", cfg.from_name);
            const pw = document.getElementById("email-password");
            if (pw) { pw.value = ""; }
            const state = document.getElementById("email-password-state");
            if (state) { state.textContent = cfg.has_password ? "A password is set — leave blank to keep it." : "No password set."; }
        }
        async function saveEmailSettings(event) {
            event.preventDefault();
            const val = (id) => { const el = document.getElementById(id); return el ? el.value.trim() : ""; };
            const body = {
                enabled: Boolean((document.getElementById("email-enabled") || {}).checked),
                host: val("email-host"),
                port: parseInt(val("email-port"), 10) || 587,
                security: val("email-security") || "starttls",
                username: val("email-username"),
                from_address: val("email-from-address"),
                from_name: val("email-from-name"),
                password: val("email-password"),
            };
            try {
                await api("/api/email/settings", { method: "PUT", body: JSON.stringify(body) });
                setNotice("Email settings saved");
                await loadEmailSettings();
            } catch (error) {
                setNotice(error.message || "Failed to save email settings", true);
            }
        }
        function setupEmailSettings() {
            const form = document.getElementById("email-form");
            if (form) { form.addEventListener("submit", saveEmailSettings); }
        }

        function setupAccessView() {
            const form = document.getElementById("access-role-form");
            if (form) { form.addEventListener("submit", saveAccessRole); }
            const newBtn = document.getElementById("new-access-role-button");
            if (newBtn) { newBtn.addEventListener("click", clearAccessRoleForm); }
            const resetBtn = document.getElementById("reset-access-role-button");
            if (resetBtn) { resetBtn.addEventListener("click", clearAccessRoleForm); }
            const delBtn = document.getElementById("delete-access-role-button");
            if (delBtn) { delBtn.addEventListener("click", deleteAccessRoleAction); }
            const assignForm = document.getElementById("access-assign-form");
            if (assignForm) { assignForm.addEventListener("submit", saveAccessAssignment); }
            const userSel = document.getElementById("access-assign-user");
            if (userSel) { userSel.addEventListener("change", renderAccessAssignRoles); }
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
                    if (payload.type === "room_message") {
                        if (state.activeRoomID && Number(payload.room_id) === state.activeRoomID) {
                            loadRoomMessages(state.activeRoomID);
                        }
                        return;
                    }
                    scheduleLiveRefresh();
                    // Near-real-time refinement: when the open ticket changes, refresh
                    // its refinement transcript + thinking indicator immediately.
                    if (payload.ticket_id) {
                        refreshOpenRefinement(payload.ticket_id);
                    }
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
            renderMainNav();
            renderProjectMenu();
            renderAccountModal();
            populateWorkflowSelects();
            populateTicketTypeAndStageSelects();
            populateTeamParentSelect();
            renderProjects();
            renderDocuments();
            renderWorkflows();
            renderRoles();
            renderAgents();
            renderTeams();
            renderUsers();
            renderTeamMembers();
            renderReleaseSelect();
            renderProjectAgentBar();
            renderTicketBoard();
            renderTicketListView();
            renderTicketPlanView();
            if (isAdmin()) { renderAdminSummary(); }
            renderInterventions();
            renderEditors();
            renderPlans();
            renderConfigSettingsPanel();
            renderOrg();
            renderProgrammeList();
            renderProgrammeEditor();
            decorateDeleteButtons(document);
            restoreCurrentViewScroll();
        }

        function renderAccountModal() {
            if (!els.accountModal) {
                return;
            }
            els.accountModal.classList.toggle("open", Boolean(state.accountModalOpen));
            if (els.accountModalTitle) {
                els.accountModalTitle.textContent = "Account settings";
            }
            if (els.accountModalSummary) {
                els.accountModalSummary.textContent = "Manage your passkeys for website and CLI sign-in.";
            }
            if (els.accountOpenConfigButton) {
                els.accountOpenConfigButton.classList.toggle("hidden", !isAdmin());
            }
            if (els.accountProfileDetails) {
                const user = (state.status && state.status.user) || {};
                const rows = [
                    { label: "Username", value: user.username || state.auth && state.auth.username || "user" },
                    { label: "Role", value: user.role || "user" },
                ];
                if (user.email) {
                    rows.push({ label: "Email", value: user.email });
                }
                if (user.display_name) {
                    rows.push({ label: "Display name", value: user.display_name });
                }
                els.accountProfileDetails.innerHTML = rows.map((row) => (
                    "<div class=\"history-item\"><strong>" + escapeHTML(row.label) + "</strong><div class=\"meta\">" + escapeHTML(row.value) + "</div></div>"
                )).join("");
            }
            if (els.accountPasskeyEnrollButton) {
                const disabled = state.passkeyBusy || !browserSupportsPasskeyEnrollment();
                els.accountPasskeyEnrollButton.disabled = disabled;
                els.accountPasskeyEnrollButton.textContent = state.passkeyBusy ? "Working…" : "Enroll passkey";
            }
            if (els.accountPasskeyStatus) {
                const message = state.passkeyError || state.passkeyStatus || (browserSupportsPasskeyEnrollment() ? "" : "This browser does not support passkey enrollment.");
                els.accountPasskeyStatus.textContent = message;
                els.accountPasskeyStatus.classList.toggle("error", Boolean(message) && (state.passkeyError || state.passkeyStatusError || !browserSupportsPasskeyEnrollment()));
            }
            if (els.accountPasskeyList) {
                if (state.passkeyError) {
                    els.accountPasskeyList.innerHTML = "<div class=\"empty\">" + escapeHTML(state.passkeyError) + "</div>";
                    return;
                }
                if (!state.passkeys.length) {
                    els.accountPasskeyList.innerHTML = "<div class=\"empty\">No passkeys enrolled yet.</div>";
                    return;
                }
                els.accountPasskeyList.innerHTML = state.passkeys.map((credential) => (
                    "<div class=\"history-item\" data-passkey-id=\"" + escapeHTML(credential.id) + "\">" +
                        "<div><strong>" + escapeHTML(credential.name || "Unnamed passkey") + "</strong></div>" +
                        "<div class=\"meta\">created " + escapeHTML(credential.created_at || "unknown") +
                            (credential.last_used_at ? " · last used " + escapeHTML(credential.last_used_at) : "") +
                        "</div>" +
                        "<div class=\"entity-actions\">" +
                            "<button type=\"button\" class=\"btn-ghost\" data-passkey-action=\"rename\" data-passkey-id=\"" + escapeHTML(credential.id) + "\"" + (state.passkeyBusy ? " disabled" : "") + ">Rename</button>" +
                            "<button type=\"button\" class=\"btn-danger\" data-passkey-action=\"delete\" data-passkey-id=\"" + escapeHTML(credential.id) + "\"" + (state.passkeyBusy ? " disabled" : "") + ">Delete</button>" +
                        "</div>" +
                    "</div>"
                )).join("");
            }
        }

        function renderProjects() {
            if (!state.projects.length) {
                els.projectList.innerHTML = "<div class=\"empty\">No projects yet.</div>";
                return;
            }
            const defaultProjectID = currentDefaultProjectID();
            els.projectList.innerHTML = state.projects.map((project) => {
                const active = project.id === state.selectedProjectID ? " active" : "";
                const isDefault = project.id === defaultProjectID;
                const defaultChip = isDefault ? "<span class=\"chip\">default</span>" : "";
                const defaultButton = "<button type=\"button\" class=\"btn-ghost\" data-project-default-id=\"" + project.id + "\">" + (isDefault ? "Clear default" : "Set default") + "</button>";
                return "<div class=\"entity-card" + active + "\" data-project-id=\"" + project.id + "\">" +
                    "<h4>" + escapeHTML(project.title) + " <small>(" + escapeHTML(project.prefix) + ")</small></h4>" +
                    "<p>" + escapeHTML(project.description || "No description") + "</p>" +
                    "<div class=\"tag-row tag-row-spaced\">" +
                    "<span class=\"chip\">" + escapeHTML(project.visibility || "public") + "</span>" +
                    "<span class=\"chip\">requests " + (project.accepts_new_members ? "open" : "closed") + "</span>" +
                    "<span class=\"chip\">draft " + String(Boolean(project.default_draft)) + "</span>" +
                    defaultChip +
                    defaultButton +
                    "</div></div>";
            }).join("");
        }

        function renderPlans() {
            if (!els.planAdminPanel) {
                return;
            }
            const admin = isAdmin();
            els.planAdminPanel.classList.toggle("hidden", !admin);
            if (!admin) {
                if (els.planList) {
                    els.planList.innerHTML = "<div class=\"empty\">Plans are only visible to admins.</div>";
                }
                return;
            }
            const plans = Array.isArray(state.plans) ? state.plans : [];
            const defaultSlug = state.defaultPlan && state.defaultPlan.slug ? state.defaultPlan.slug : "";
            const selectedSlug = plans.some((plan) => plan.slug === state.selectedPlanSlug)
                ? state.selectedPlanSlug
                : (defaultSlug || (plans[0] && plans[0].slug) || "");
            state.selectedPlanSlug = selectedSlug;
            if (els.defaultPlanSelect) {
                els.defaultPlanSelect.innerHTML = plans.map((plan) => {
                    const selected = plan.slug === defaultSlug ? " selected" : "";
                    return "<option value=\"" + escapeHTML(plan.slug) + "\"" + selected + ">" + escapeHTML(plan.name || plan.slug) + "</option>";
                }).join("");
            }
            if (els.registrationEnabledSelect) {
                els.registrationEnabledSelect.value = String(!(state.status && state.status.registration_enabled === false));
            }
            if (els.registrationAutoApproveSelect) {
                els.registrationAutoApproveSelect.value = String(!(state.status && state.status.registration_auto_approve === false));
            }
            if (!plans.length) {
                if (els.planList) {
                    els.planList.innerHTML = "<div class=\"empty\">No plans available.</div>";
                }
                state.selectedPlanDraft = state.selectedPlanDraft && !state.selectedPlanDraft.plan_id ? state.selectedPlanDraft : emptyPlan();
                renderPlanEditor();
                return;
            }
            if (!state.selectedPlanDraft || (state.selectedPlanDraft.plan_id && state.selectedPlanDraft.slug !== selectedSlug)) {
                state.selectedPlanDraft = getCurrentPlan() ? structuredClone(getCurrentPlan()) : emptyPlan();
            }
            if (els.planList) {
                els.planList.innerHTML = plans.map((plan) => {
                    const actions = plan.registration_actions || {};
                    const badges = [
                        plan.slug === defaultSlug ? "default" : "",
                        "projects " + String(plan.max_projects),
                        "private " + String(plan.max_private_projects),
                        "tickets/project " + String(plan.max_tickets_per_project),
                        actions.auto_assign_public_team ? "public team" : "",
                        actions.auto_create_private_project ? "private project" : "",
                        actions.auto_create_private_team ? "private team" : "",
                    ].filter(Boolean).map((label) => "<span class=\"chip\">" + escapeHTML(label) + "</span>").join("");
                    const active = plan.slug === selectedSlug ? " active" : "";
                    return "<div class=\"entity-card" + active + "\" data-plan-slug=\"" + escapeHTML(plan.slug) + "\">" +
                        "<h4>" + escapeHTML(plan.name || plan.slug) + " <small>(" + escapeHTML(plan.slug) + ")</small></h4>" +
                        "<p>" + escapeHTML(plan.description || "No description") + "</p>" +
                        "<div class=\"tag-row tag-row-spaced\">" + badges + "</div>" +
                        "</div>";
                }).join("");
            }
            renderPlanEditor();
        }

        function renderPlanEditor() {
            if (!els.planSlug) {
                return;
            }
            const plan = state.selectedPlanDraft || emptyPlan();
            const actions = plan.registration_actions || {};
            if (els.planEditorTitle) {
                els.planEditorTitle.textContent = plan.plan_id ? ("Plan: " + (plan.name || plan.slug)) : "Plan editor";
            }
            els.planSlug.value = plan.slug || "";
            els.planName.value = plan.name || "";
            els.planDescription.value = plan.description || "";
            els.planMaxProjects.value = String(plan.max_projects || 0);
            els.planMaxPrivateProjects.value = String(plan.max_private_projects || 0);
            els.planMaxTickets.value = String(plan.max_tickets || 0);
            els.planMaxTicketsPerProject.value = String(plan.max_tickets_per_project || 0);
            els.planMaxTeamMemberships.value = String(plan.max_team_memberships || 0);
            els.planMaxAPICallsPerDay.value = String(plan.max_api_calls_per_day || 0);
            els.planDefaultProjectAlias.value = plan.default_project_alias || "public";
            els.planAutoAssignPublicTeam.value = String(Boolean(actions.auto_assign_public_team));
            els.planAutoCreatePrivateProject.value = String(Boolean(actions.auto_create_private_project));
            els.planAutoCreatePrivateTeam.value = String(Boolean(actions.auto_create_private_team));
            const deleteButton = document.getElementById("delete-plan-button");
            if (deleteButton) {
                deleteButton.disabled = !plan.plan_id;
            }
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

        function maskSecret(value) {
            const raw = String(value || "").trim();
            if (!raw) {
                return "(inherited/empty)";
            }
            if (raw.length <= 4) {
                return "****";
            }
            return "****" + raw.slice(-4);
        }

        function renderProviderSelect(selectElement, selectedProvider, includeInherit) {
            if (!selectElement) {
                return;
            }
            const providers = Array.isArray(state.systemAgentModelConfig.providers) ? state.systemAgentModelConfig.providers : [];
            const options = [];
            const providerIDs = new Set();
            if (includeInherit) {
                options.push(optionHTML("", "(inherit)", !selectedProvider));
            } else if (!providers.length) {
                options.push(optionHTML("", "(none)", true));
            }
            providers.forEach((provider) => {
                const id = String(provider.id || "").trim();
                if (!id) {
                    return;
                }
                providerIDs.add(id);
                const label = String(provider.label || id);
                options.push(optionHTML(id, label, id === selectedProvider));
            });
            if (selectedProvider && !providerIDs.has(selectedProvider)) {
                options.push(optionHTML(selectedProvider, selectedProvider + " (custom)", true));
            }
            setInnerHTMLIfChanged(selectElement, options.join(""));
            selectElement.value = selectedProvider || "";
        }

        function providerByID(providerID) {
            const providers = Array.isArray(state.systemAgentModelConfig.providers) ? state.systemAgentModelConfig.providers : [];
            const id = String(providerID || "").trim();
            return providers.find((provider) => String(provider.id || "").trim() === id) || null;
        }

        function renderModelSelect(selectElement, providerID, selectedModel, includeInherit) {
            if (!selectElement) {
                return;
            }
            const provider = providerByID(providerID);
            const models = provider && Array.isArray(provider.models) ? provider.models : [];
            const options = [];
            if (includeInherit) {
                options.push(optionHTML("", "(inherit)", !selectedModel));
            }
            models.forEach((model) => {
                const value = String(model || "").trim();
                if (!value) {
                    return;
                }
                options.push(optionHTML(value, value, value === selectedModel));
            });
            if (selectedModel && !models.includes(selectedModel)) {
                options.push(optionHTML(selectedModel, selectedModel + " (custom)", true));
            } else if (!includeInherit && !selectedModel && provider && provider.default_model) {
                options.push(optionHTML(provider.default_model, provider.default_model, true));
            }
            if (!options.length) {
                options.push(optionHTML("", "(none)", true));
            }
            setInnerHTMLIfChanged(selectElement, options.join(""));
            selectElement.value = selectedModel || "";
        }

        function harnessFields(scope) {
            if (scope === "system") {
                return {
                    provider: els.systemAgentProvider,
                    model: els.systemAgentModel,
                    url: els.systemAgentURL,
                    apiKey: els.systemAgentAPIKey,
                    includeInherit: false,
                };
            }
            if (scope === "project") {
                return {
                    provider: els.projectAgentProvider,
                    model: els.projectAgentModel,
                    url: els.projectAgentURL,
                    apiKey: els.projectAgentAPIKey,
                    includeInherit: true,
                };
            }
            return {
                provider: null,
                model: null,
                url: null,
                apiKey: null,
                includeInherit: true,
            };
        }

        function applyProviderSelectionDefaults(scope) {
            const fields = harnessFields(scope);
            if (!fields.provider) {
                return;
            }
            const providerID = String(fields.provider.value || "").trim();
            const provider = providerByID(providerID);
            if (!provider) {
                return;
            }
            if (fields.model && !String(fields.model.value || "").trim() && provider.default_model) {
                fields.model.value = provider.default_model;
            }
            if (fields.url && !String(fields.url.value || "").trim() && provider.base_url) {
                fields.url.value = provider.base_url;
            }
        }

        function applyHarnessRequirements(scope) {
            const fields = harnessFields(scope);
            const providerID = fields.provider ? String(fields.provider.value || "").trim() : "";
            const provider = providerByID(providerID);
            const inherited = fields.includeInherit && !providerID;

            if (fields.model) {
                fields.model.disabled = inherited;
                fields.model.required = !inherited;
            }
            if (fields.url) {
                fields.url.disabled = inherited;
                fields.url.required = Boolean(provider && provider.requires_url);
                fields.url.placeholder = inherited
                    ? "inherits from parent"
                    : (provider && provider.requires_url ? "required for this provider" : (provider && provider.base_url ? provider.base_url : "optional"));
            }
            if (fields.apiKey) {
                const authType = provider ? String(provider.auth_type || "api_key").toLowerCase() : "api_key";
                const apiKeyRequired = !inherited && authType === "api_key";
                fields.apiKey.disabled = inherited || authType === "none";
                fields.apiKey.required = apiKeyRequired;
                fields.apiKey.placeholder = inherited
                    ? "inherits from parent"
                    : (authType === "none" ? "not required for this provider" : "required for this provider");
            }
        }

        function normalizedProviderConfig(provider) {
            const item = provider || {};
            const models = Array.isArray(item.models) ? item.models.map((model) => String(model || "").trim()).filter(Boolean) : [];
            const defaultModel = String(item.default_model || "").trim();
            if (defaultModel && !models.includes(defaultModel)) {
                models.unshift(defaultModel);
            }
            return {
                id: String(item.id || "").trim(),
                label: String(item.label || "").trim(),
                base_url: String(item.base_url || "").trim(),
                default_model: defaultModel,
                auth_type: String(item.auth_type || "api_key").trim() || "api_key",
                requires_url: Boolean(item.requires_url),
                api_key: String(item.api_key || "").trim(),
                models: models,
            };
        }

        function providerConfigs() {
            return (Array.isArray(state.systemAgentModelConfig.providers) ? state.systemAgentModelConfig.providers : [])
                .map(normalizedProviderConfig)
                .filter((provider) => provider.id);
        }

        function currentConfigSetting() {
            return state.configSettings.find((item) => item.key === state.selectedConfigSettingKey) || null;
        }

        function getCurrentPlan() {
            return state.plans.find((plan) => plan.slug === state.selectedPlanSlug) || null;
        }

        function setConfigSettingEditor(setting) {
            const current = setting || { key: "", value: "" };
            if (els.configSettingEditorTitle) {
                els.configSettingEditorTitle.textContent = current.key ? ("Config editor - " + current.key) : "Config editor";
            }
            if (els.configSettingKey) {
                els.configSettingKey.value = current.key || "";
            }
            if (els.configSettingValue) {
                els.configSettingValue.value = current.value || "";
            }
            const deleteButton = document.getElementById("delete-config-setting-button");
            if (deleteButton) {
                deleteButton.disabled = !current.key;
            }
        }

        function renderConfigSettingsPanel() {
            if (!els.configSettingsList || !els.configSettingsSummary) {
                return;
            }
            const admin = isAdmin();
            const configView = document.getElementById("view-settings");
            if (configView) {
                configView.classList.toggle("hidden", !admin);
            }
            if (!admin) {
                return;
            }
            const settings = Array.isArray(state.configSettings) ? state.configSettings : [];
            els.configSettingsSummary.textContent = settings.length
                ? (String(settings.length) + " live settings available.")
                : "No custom settings yet. Create the first key to seed app_settings.";
            if (!state.selectedConfigSettingKey || !settings.some((item) => item.key === state.selectedConfigSettingKey)) {
                state.selectedConfigSettingKey = settings.length ? settings[0].key : "";
            }
            if (!settings.length) {
                els.configSettingsList.innerHTML = "<div class=\"empty\">No settings yet.</div>";
                setConfigSettingEditor(null);
                return;
            }
            els.configSettingsList.innerHTML = settings.map((item) => {
                const active = item.key === state.selectedConfigSettingKey ? " active" : "";
                const preview = item.value.length > 160 ? item.value.slice(0, 160) + "..." : item.value;
                return "<div class=\"entity-card" + active + "\" data-config-setting-key=\"" + escapeHTML(item.key) + "\">" +
                    "<h4 class=\"config-key\">" + escapeHTML(item.key) + "</h4>" +
                    "<p class=\"config-value-preview\">" + escapeHTML(preview || "(empty)") + "</p></div>";
            }).join("");
            setConfigSettingEditor(currentConfigSetting());
        }

        function renderProviderConfigPanel() {
            if (!els.providerConfigSelect || !els.providerConfigID) {
                return;
            }
            const providers = providerConfigs();
            if (!state.selectedProviderConfigID || !providers.some((provider) => provider.id === state.selectedProviderConfigID)) {
                state.selectedProviderConfigID = providers.length ? providers[0].id : "";
            }
            if (!providers.length) {
                setInnerHTMLIfChanged(els.providerConfigSelect, optionHTML("", "No configurations defined", true));
                els.providerConfigID.value = "";
                els.providerConfigLabel.value = "";
                els.providerConfigModel.value = "";
                els.providerConfigURL.value = "";
                els.providerConfigAuthType.value = "api_key";
                els.providerConfigRequiresURL.value = "false";
                els.providerConfigAPIKey.value = "";
                els.providerConfigModels.value = "";
                document.getElementById("delete-provider-config-button").disabled = true;
                return;
            }
            const selectOptions = providers.map((provider) => {
                const label = (provider.label || provider.id) + " (" + provider.id + ")";
                return optionHTML(provider.id, label, provider.id === state.selectedProviderConfigID);
            }).join("");
            setInnerHTMLIfChanged(els.providerConfigSelect, selectOptions);
            els.providerConfigSelect.value = state.selectedProviderConfigID;
            const selected = providers.find((provider) => provider.id === state.selectedProviderConfigID) || providers[0];
            els.providerConfigID.value = selected.id;
            els.providerConfigLabel.value = selected.label;
            els.providerConfigModel.value = selected.default_model;
            els.providerConfigURL.value = selected.base_url;
            els.providerConfigAuthType.value = selected.auth_type || "api_key";
            els.providerConfigRequiresURL.value = selected.requires_url ? "true" : "false";
            els.providerConfigAPIKey.value = selected.api_key || "";
            els.providerConfigModels.value = (selected.models || []).join("\n");
            document.getElementById("delete-provider-config-button").disabled = !selected.id;
        }

        function renderAgentHarnessEditor() {
            const hasProject = Boolean(state.selectedProjectID);

            const system = normalizeAgentModelConfig(state.systemAgentModelConfig);
            const project = normalizeAgentModelConfig(state.projectAgentModelConfig);

            renderProviderSelect(els.systemAgentProvider, system.provider, false);
            renderProviderSelect(els.projectAgentProvider, project.provider, true);
            renderModelSelect(els.systemAgentModel, system.provider, system.model, false);
            renderModelSelect(els.projectAgentModel, project.provider, project.model, true);

            if (els.systemAgentModel) {
                els.systemAgentModel.value = system.model || "";
            }
            if (els.systemAgentURL) {
                els.systemAgentURL.value = system.url || "";
            }
            if (els.systemAgentAPIKey) {
                els.systemAgentAPIKey.value = system.api_key || "";
            }

            if (els.projectAgentModel) {
                els.projectAgentModel.disabled = !hasProject;
            }
            if (els.projectAgentURL) {
                els.projectAgentURL.value = project.url || "";
                els.projectAgentURL.disabled = !hasProject;
            }
            if (els.projectAgentAPIKey) {
                els.projectAgentAPIKey.value = project.api_key || "";
                els.projectAgentAPIKey.disabled = !hasProject;
            }
            if (els.projectAgentProvider) {
                els.projectAgentProvider.disabled = !hasProject;
            }

            applyHarnessRequirements("system");
            applyHarnessRequirements("project");

            if (!hasProject) {
                if (els.projectAgentProvider) els.projectAgentProvider.disabled = true;
                if (els.projectAgentModel) els.projectAgentModel.disabled = true;
                if (els.projectAgentURL) els.projectAgentURL.disabled = true;
                if (els.projectAgentAPIKey) els.projectAgentAPIKey.disabled = true;
            }

            const saveProjectButton = document.getElementById("save-project-agent-model");
            const clearProjectButton = document.getElementById("clear-project-agent-model");
            if (saveProjectButton) saveProjectButton.disabled = !hasProject;
            if (clearProjectButton) clearProjectButton.disabled = !hasProject;

            if (els.agentHarnessSummary) {
                if (!hasProject) {
                    els.agentHarnessSummary.textContent = "Select a project to configure project overrides.";
                } else {
                    els.agentHarnessSummary.textContent = "Hierarchy: project → system. Effective model shown below.";
                }
            }
            renderProviderConfigPanel();
        }

        function summarizeStageCopy(value, fallback) {
            const text = String(value || "").trim();
            if (!text) {
                return fallback;
            }
            return text.length > 120 ? text.slice(0, 117) + "..." : text;
        }

        async function persistWorkflowStageOrder(workflowID, orderedStageIDs) {
            await api("/api/workflows/" + workflowID + "/reorder", {
                method: "PUT",
                body: JSON.stringify({ stage_ids: orderedStageIDs }),
            });
            await loadWorkflows();
            renderAll();
        }

        function renderWorkflowViewToggle() {
            if (els.workflowViewBoardButton) {
                const active = state.workflowStageViewMode === "board";
                els.workflowViewBoardButton.setAttribute("aria-pressed", active ? "true" : "false");
                els.workflowViewBoardButton.classList.toggle("active", active);
            }
            if (els.workflowViewGraphButton) {
                const active = state.workflowStageViewMode === "graph";
                els.workflowViewGraphButton.setAttribute("aria-pressed", active ? "true" : "false");
                els.workflowViewGraphButton.classList.toggle("active", active);
            }
            if (els.workflowStageBoard) {
                els.workflowStageBoard.classList.toggle("graph-mode", state.workflowStageViewMode === "graph");
            }
            if (els.stageGrid) {
                els.stageGrid.classList.toggle("stage-grid-graph", state.workflowStageViewMode === "graph");
            }
        }

        function renderWorkflowStageSections(stage, workflow) {
            const roleCardsHTML = (stage.roles || []).map((role) => {
                const fullRole = state.roles.find((item) => item.id === role.id);
                const label = fullRole ? fullRole.title : role.title || ("role " + role.id);
                const description = (fullRole && fullRole.description) || role.description || "";
                return "<article class=\"ticket-card workflow-role-card\" draggable=\"true\" data-stage-id=\"" + stage.id + "\" data-role-id=\"" + role.id + "\" data-role-description=\"" + escapeHTML(description) + "\">" +
                    "<div class=\"panel-head panel-head-tight\">" +
                    "<strong>" + escapeHTML(label) + "</strong>" +
                    "</div>" +
                    "</article>";
            }).join("");
            const addRoleOptions = state.roles
                .filter((role) => !(stage.roles || []).some((current) => current.id === role.id))
                .map((role) => optionHTML(role.id, role.title, false))
                .join("");
            return "<details class=\"stage-card-section\">" +
                "<summary><span class=\"stage-card-section-title\">Config</span></summary>" +
                "<div class=\"stage-card-section-body stack\">" +
                "<div class=\"field\"><label>Ways of working</label><textarea data-stage-wow=\"" + stage.id + "\">" + escapeHTML(stage.wow || stage.description || "") + "</textarea></div>" +
                "<div class=\"field\"><label>Definition of ready</label><textarea data-stage-dor=\"" + stage.id + "\">" + escapeHTML(stage.dor || "") + "</textarea></div>" +
                "<div class=\"field\"><label>Definition of done</label><textarea data-stage-dod=\"" + stage.id + "\">" + escapeHTML(stage.dod || "") + "</textarea></div>" +
                "<div class=\"field\"><label>Transitions (next stages)</label><select multiple data-stage-next=\"" + stage.id + "\">" +
                workflow.stages
                    .filter((candidate) => Number(candidate.id) !== Number(stage.id))
                    .map((candidate) => optionHTML(candidate.id, candidate.name || candidate.stage_name || ("stage " + candidate.id), (stage.next_stage_ids || []).some((nextID) => Number(nextID) === Number(candidate.id))))
                    .join("") +
                "</select></div>" +
                "<div class=\"stage-card-actions\"><button type=\"button\" class=\"btn btn-primary btn-sm\" data-save-stage=\"" + stage.id + "\">Save</button></div>" +
                "</div>" +
                "</details>" +
                "<details class=\"stage-card-section\" open>" +
                "<summary><span class=\"stage-card-section-title\">Roles</span></summary>" +
                "<div class=\"stage-card-section-body\">" +
                "<div class=\"field\"><div class=\"workflow-role-list stage-card-dropzone\" data-stage-role-row=\"" + stage.id + "\">" + (roleCardsHTML || "<div class=\"empty workflow-role-empty\">Drop roles here</div>") + "</div></div>" +
                "<div class=\"stage-card-add-role\">" +
                "<div class=\"field\"><label>Add role</label><div class=\"select-shell\"><select data-add-role-select=\"" + stage.id + "\">" + optionHTML("", "Choose role", true) + addRoleOptions + "</select></div></div>" +
                "<button type=\"button\" class=\"btn btn-sm\" data-add-role=\"" + stage.id + "\">Add</button>" +
                "</div>" +
                "</div>" +
                "</details>";
        }

        function renderWorkflowStageCard(stage, workflow, index) {
            const moveLeftDisabled = index === 0 ? " disabled" : "";
            const moveRightDisabled = index === workflow.stages.length - 1 ? " disabled" : "";
            return "<article class=\"lane workflow-stage-lane stage-card\" draggable=\"true\" data-stage-id=\"" + stage.id + "\">" +
                "<div class=\"lane-head stage-card-top\">" +
                "<div class=\"stage-card-heading\">" +
                "<div class=\"stage-card-order\">" + String(index + 1) + "</div>" +
                "<div class=\"workflow-stage-title-wrap\">" +
                "<button type=\"button\" class=\"stage-card-title-button\" data-rename-stage=\"" + stage.id + "\" data-stage-title=\"" + stage.id + "\" aria-label=\"Rename stage " + escapeHTML(stage.name) + "\">" +
                "<h4 class=\"stage-card-title\">" + escapeHTML(stage.name) + "</h4>" +
                "</button>" +
                "</div>" +
                "</div>" +
                "<div class=\"stage-card-controls\">" +
                "<button type=\"button\" class=\"stage-card-move-button\" data-move-stage=\"" + stage.id + "\" data-move-direction=\"left\" aria-label=\"Move stage left\"" + moveLeftDisabled + ">&lt;</button>" +
                "<button type=\"button\" class=\"stage-card-move-button\" data-move-stage=\"" + stage.id + "\" data-move-direction=\"right\" aria-label=\"Move stage right\"" + moveRightDisabled + ">&gt;</button>" +
                "<button type=\"button\" class=\"btn btn-sm btn-danger\" data-delete-stage=\"" + stage.id + "\">Del</button>" +
                "</div>" +
                "</div>" +
                renderWorkflowStageSections(stage, workflow) +
                "</article>";
        }

        function buildWorkflowGraphLayout(workflow) {
            const layoutByID = {};
            const orderIndexByID = new Map(workflow.stages.map((stage, index) => [Number(stage.id), index]));
            const depthByID = new Map(workflow.stages.map((stage) => [Number(stage.id), 0]));

            workflow.stages.forEach((stage) => {
                const stageID = Number(stage.id);
                const stageDepth = depthByID.get(stageID) || 0;
                (stage.next_stage_ids || []).forEach((nextID) => {
                    const targetID = Number(nextID);
                    if (!orderIndexByID.has(targetID)) {
                        return;
                    }
                    if ((orderIndexByID.get(targetID) || 0) <= (orderIndexByID.get(stageID) || 0)) {
                        return;
                    }
                    depthByID.set(targetID, Math.max(depthByID.get(targetID) || 0, stageDepth + 1));
                });
            });

            const orderedNodes = [];
            workflow.stages.forEach((stage) => {
                const stageID = Number(stage.id);
                const index = orderIndexByID.get(stageID) || 0;
                const depth = depthByID.get(stageID) || 0;
                const rowIndex = index;
                const x = depth * WORKFLOW_GRAPH_COLUMN_GAP;
                const y = rowIndex * WORKFLOW_GRAPH_ROW_GAP;
                const layout = { id: stageID, index, depth, rowIndex, x, y };
                layoutByID[String(stage.id)] = layout;
                orderedNodes.push(layout);
            });

            const maxDepth = orderedNodes.reduce((max, node) => Math.max(max, node.depth), 0);
            const maxRow = orderedNodes.reduce((max, node) => Math.max(max, node.rowIndex), 0);
            const paddingX = 92;
            const paddingY = 80;
            const width = Math.max(720, paddingX * 2 + maxDepth * WORKFLOW_GRAPH_COLUMN_GAP + WORKFLOW_GRAPH_NODE_WIDTH);
            const height = Math.max(420, paddingY * 2 + maxRow * WORKFLOW_GRAPH_ROW_GAP + WORKFLOW_GRAPH_NODE_HEIGHT);
            orderedNodes.forEach((layout) => {
                layout.x += paddingX;
                layout.y += paddingY;
            });
            return { layoutByID, orderedNodes, width, height };
        }

        function buildWorkflowGraphEdges(workflow, layoutByID) {
            const directed = [];
            workflow.stages.forEach((stage) => {
                (stage.next_stage_ids || []).forEach((nextID) => {
                    if (layoutByID[String(nextID)]) {
                        directed.push({ from: Number(stage.id), to: Number(nextID) });
                    }
                });
            });
            const edgeSet = new Set(directed.map((edge) => edge.from + ":" + edge.to));
            const handled = new Set();
            const edges = [];
            directed.forEach((edge) => {
                const key = edge.from + ":" + edge.to;
                if (handled.has(key)) {
                    return;
                }
                const reverseKey = edge.to + ":" + edge.from;
                if (edgeSet.has(reverseKey)) {
                    const fromLayout = layoutByID[String(edge.from)];
                    const toLayout = layoutByID[String(edge.to)];
                    const ordered = fromLayout.x <= toLayout.x
                        ? { from: edge.from, to: edge.to }
                        : { from: edge.to, to: edge.from };
                    edges.push({ from: ordered.from, to: ordered.to, direction: "both" });
                    handled.add(key);
                    handled.add(reverseKey);
                    return;
                }
                edges.push({ from: edge.from, to: edge.to, direction: "forward" });
                handled.add(key);
            });
            return edges;
        }

        function describeWorkflowGraphEdge(edge, layoutByID) {
            const from = layoutByID[String(edge.from)];
            const to = layoutByID[String(edge.to)];
            if (!from || !to) {
                return "";
            }
            const startX = from.x + WORKFLOW_GRAPH_NODE_WIDTH / 2;
            const startY = from.y + WORKFLOW_GRAPH_NODE_HEIGHT / 2;
            const endX = to.x + WORKFLOW_GRAPH_NODE_WIDTH / 2;
            const endY = to.y + WORKFLOW_GRAPH_NODE_HEIGHT / 2;
            const dx = endX - startX;
            const dy = endY - startY;
            const hopCount = Math.max(1, Math.abs(from.index - to.index));
            const liftBase = Math.min(200, 30 + hopCount * 18);
            const sameBand = Math.abs(dy) < 50;
            const lift = sameBand
                ? ((from.rowIndex % 2 === 0 ? -1 : 1) * liftBase)
                : (dy > 0 ? -1 : 1) * Math.min(160, 22 + Math.abs(dy) * 0.22);
            const controlX1 = startX + dx * 0.32;
            const controlY1 = startY + lift;
            const controlX2 = endX - dx * 0.32;
            const controlY2 = endY + lift;
            return "M " + startX + " " + startY + " C " + controlX1 + " " + controlY1 + ", " + controlX2 + " " + controlY2 + ", " + endX + " " + endY;
        }

        function renderWorkflowGraphNode(stage, layout) {
            return "<div class=\"workflow-graph-node\" data-graph-node-id=\"" + stage.id + "\" style=\"left:" + layout.x + "px; top:" + layout.y + "px;\">" +
                "<button type=\"button\" class=\"workflow-graph-node-button\" data-rename-stage=\"" + stage.id + "\" data-stage-title=\"" + stage.id + "\" aria-label=\"Rename stage " + escapeHTML(stage.name) + "\">" +
                "<span class=\"workflow-graph-node-title\">" + escapeHTML(stage.name) + "</span>" +
                "</button>" +
                "</div>";
        }

        function renderWorkflowGraph(workflow) {
            const layout = buildWorkflowGraphLayout(workflow);
            const edges = buildWorkflowGraphEdges(workflow, layout.layoutByID);
            const edgeMarkup = edges.map((edge) => {
                const path = describeWorkflowGraphEdge(edge, layout.layoutByID);
                const markers = edge.direction === "both"
                    ? " marker-start=\"url(#workflow-graph-arrow)\" marker-end=\"url(#workflow-graph-arrow)\""
                    : " marker-end=\"url(#workflow-graph-arrow)\"";
                return "<path class=\"workflow-graph-edge workflow-graph-edge-" + edge.direction + "\" data-workflow-graph-edge data-edge-direction=\"" + edge.direction + "\" d=\"" + path + "\"" + markers + "></path>";
            }).join("");
            const nodeMarkup = workflow.stages.map((stage) => renderWorkflowGraphNode(stage, layout.layoutByID[String(stage.id)])).join("");
            return "<div class=\"workflow-graph-viewport\" data-workflow-graph-viewport=\"true\">" +
                "<div class=\"workflow-graph-frame\">" +
                "<div class=\"workflow-graph-surface\" style=\"width:" + layout.width + "px; height:" + layout.height + "px;\">" +
                "<svg class=\"workflow-graph-links\" viewBox=\"0 0 " + layout.width + " " + layout.height + "\" preserveAspectRatio=\"xMinYMin meet\" aria-label=\"Workflow graph\">" +
                "<defs>" +
                "<marker id=\"workflow-graph-arrow\" viewBox=\"0 0 10 10\" refX=\"8\" refY=\"5\" markerWidth=\"7\" markerHeight=\"7\" orient=\"auto-start-reverse\">" +
                "<path d=\"M 0 0 L 10 5 L 0 10 z\" fill=\"rgba(255, 122, 26, 0.92)\"></path>" +
                "</marker>" +
                "</defs>" +
                edgeMarkup +
                "</svg>" +
                nodeMarkup +
                "</div>" +
                "</div>" +
                "</div>";
        }

        function resetWorkflowGraphViewportIfNeeded() {
            if (state.workflowStageViewMode !== "graph" || !state.workflowGraphNeedsReset) {
                return;
            }
            if (els.workflowStageBoard) {
                els.workflowStageBoard.scrollLeft = 0;
                els.workflowStageBoard.scrollTop = 0;
            }
            const viewport = document.querySelector("[data-workflow-graph-viewport]");
            if (viewport) {
                viewport.scrollLeft = 0;
                viewport.scrollTop = 0;
            }
            state.workflowGraphNeedsReset = false;
        }

        function renderWorkflows() {
            renderWorkflowViewToggle();
            if (els.workflowSelect) {
                const workflowSelectHTML = [
                    optionHTML("", state.selectedWorkflowID ? "Select workflow" : "New workflow draft", !state.selectedWorkflowID),
                ].concat(
                    state.workflows.map((workflow) => optionHTML(workflow.id, workflow.name, workflow.id === state.selectedWorkflowID)),
                ).join("");
                setInnerHTMLIfChanged(els.workflowSelect, workflowSelectHTML);
                els.workflowSelect.disabled = !state.workflows.length && !state.selectedWorkflowID;
            }

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

            const stageGridHTML = state.workflowStageViewMode === "graph"
                ? renderWorkflowGraph(workflow)
                : workflow.stages.map((stage, index) => renderWorkflowStageCard(stage, workflow, index)).join("");
            setInnerHTMLIfChanged(els.stageGrid, stageGridHTML);
            resetWorkflowGraphViewportIfNeeded();
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
                const roles = splitAgentRoles(agent.agent_role);
                const roleChips = roles.length
                    ? roles.map((r) => "<span class=\"chip\">" + escapeHTML(r) + "</span>").join(" ") + " "
                    : "";
                const isRefiner = roles.some((r) => r.toLowerCase() === "refiner");
                let roleDesc;
                if (!roles.length) {
                    roleDesc = "claims any idle ticket";
                } else if (isRefiner && roles.length === 1) {
                    roleDesc = "refines draft tickets";
                } else {
                    roleDesc = "claims idle tickets whose current role is " + roles.join(" or ");
                }
                const statusChip = agent.enabled
                    ? "<span class=\"chip chip-success\">enabled</span>"
                    : "<span class=\"chip chip-danger\">disabled</span>";
                const name = escapeHTML(agent.username || agent.id);
                return "<div class=\"entity-card" + active + "\" data-agent-id=\"" + escapeHTML(agent.id) + "\">" +
                    "<h4>" + name + "</h4>" +
                    "<p>" + roleChips + statusChip + "</p>" +
                    "<small>" + escapeHTML(roleDesc) + "</small>" +
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

        function renderUsers() {
            if (!els.userList) return;
            if (!state.users || !state.users.length) {
                els.userList.innerHTML = "<div class=\"empty\">No users.</div>";
                return;
            }
            const rows = state.users.map((u) => {
                const roleChip = "<span class=\"chip\">" + escapeHTML(u.role || "user") + "</span>";
                const statusChip = u.enabled === false
                    ? "<span class=\"chip chip-danger\">disabled</span>"
                    : "<span class=\"chip chip-success\">active</span>";
                return "<tr class=\"ticket-list-row\" data-user-id=\"" + escapeHTML(String(u.id || u.username)) + "\">" +
                    "<td>" + escapeHTML(u.username) + "</td>" +
                    "<td>" + escapeHTML(u.display_name || "—") + "</td>" +
                    "<td>" + escapeHTML(u.email || "—") + "</td>" +
                    "<td>" + roleChip + "</td>" +
                    "<td>" + statusChip + "</td>" +
                    "</tr>";
            }).join("");
            els.userList.innerHTML = "<div class=\"table-wrap\"><table class=\"ticket-list-table\">" +
                "<thead><tr><th>Username</th><th>Display name</th><th>Email</th><th>Role</th><th>Status</th></tr></thead>" +
                "<tbody>" + rows + "</tbody>" +
                "</table></div>";
        }

        async function fetchUsers() {
            if (!isAdmin()) return;
            try {
                state.users = await apiClient.listUsers();
                renderUsers();
            } catch (e) {
                console.error("fetchUsers:", e);
            }
        }

        async function loadOrg() {
            if (!isAdmin()) return;
            try {
                state.org = await apiClient.getOrg();
            } catch (e) {
                console.error("loadOrg:", e);
            }
        }

        function renderOrg() {
            if (!els.orgForm) return;
            const o = state.org || {};
            if (els.orgName) els.orgName.value = o.name || "";
            if (els.orgDomain) els.orgDomain.value = o.domain || "";
            if (els.orgDescription) els.orgDescription.value = o.description || "";
            if (els.orgLogo) els.orgLogo.value = o.logo_url || "";
        }

        async function loadProgrammes() {
            if (!isAdmin()) return;
            try {
                const result = await apiClient.listProgrammes();
                state.programmes = Array.isArray(result) ? result : [];
            } catch (e) {
                console.error("loadProgrammes:", e);
            }
        }

        function renderProgrammeList() {
            if (!els.programmeList) return;
            if (!state.programmes || !state.programmes.length) {
                els.programmeList.innerHTML = "<div class=\"empty\">No programmes.</div>";
                return;
            }
            els.programmeList.innerHTML = state.programmes.map((p) => {
                const active = p.id === state.selectedProgrammeID ? " active" : "";
                return "<div class=\"entity-card" + active + "\" data-programme-id=\"" + p.id + "\">" +
                    "<h4>" + escapeHTML(p.name) + "</h4>" +
                    "<p>" + escapeHTML(p.description || "") + "</p>" +
                    "</div>";
            }).join("");
        }

        function renderProgrammeProjects() {
            if (!els.programmeProjectsList) return;
            const programmeID = state.selectedProgrammeID;
            const projects = state.projects || [];
            if (!projects.length) {
                els.programmeProjectsList.innerHTML = "<div class=\"empty\">No projects.</div>";
                return;
            }
            els.programmeProjectsList.innerHTML = projects.map((p) => {
                const inProgramme = p.programme_id === programmeID;
                const chipClass = inProgramme ? "chip chip-success" : "chip";
                const label = inProgramme ? "remove" : "add";
                return "<div style=\"display:flex;align-items:center;justify-content:space-between;padding:4px 8px\">" +
                    "<span>" + escapeHTML(p.prefix || p.title) + " — " + escapeHTML(p.title) + "</span>" +
                    "<button class=\"btn btn-sm " + chipClass + "\" type=\"button\" data-programme-project-id=\"" + p.id + "\" data-programme-project-in=\"" + inProgramme + "\">" + label + "</button>" +
                    "</div>";
            }).join("");
        }

        function renderProgrammeEditor() {
            if (!els.programmeForm) return;
            const prog = state.programmes.find((p) => p.id === state.selectedProgrammeID) || null;
            if (els.programmeEditorTitle) {
                els.programmeEditorTitle.textContent = prog ? "Edit programme" : "New programme";
            }
            if (els.programmeName) els.programmeName.value = prog ? prog.name : "";
            if (els.programmeDescription) els.programmeDescription.value = prog ? prog.description : "";
            if (els.deleteProgrammeButton) {
                els.deleteProgrammeButton.style.display = prog ? "" : "none";
            }
            renderProgrammeProjects();
        }

        function renderTeamMembers() {
            if (!els.teamMemberList) return;
            if (!state.teamMembers || !state.teamMembers.length) {
                els.teamMemberList.innerHTML = "<div class=\"empty\">No members yet.</div>";
                return;
            }
            els.teamMemberList.innerHTML = state.teamMembers.map((m) => {
                return "<div class=\"entity-card\" data-team-member-id=\"" + escapeHTML(String(m.user_id)) + "\">" +
                    "<div style=\"display:flex;align-items:center;justify-content:space-between\">" +
                    "<div><h4>" + escapeHTML(m.username || String(m.user_id)) + "</h4>" +
                    "<p>" + escapeHTML(m.job_title || "") + "</p></div>" +
                    "<div class=\"tag-row\"><span class=\"chip\">" + escapeHTML(m.role) + "</span>" +
                    "<button class=\"btn btn-danger btn-sm\" type=\"button\" data-remove-team-member=\"" + escapeHTML(String(m.user_id)) + "\">Remove</button>" +
                    "</div></div>" +
                    "</div>";
            }).join("");
        }

        async function fetchTeamMembers(teamID) {
            try {
                state.teamMembers = await apiClient.getTeamMembers(teamID);
                renderTeamMembers();
                populateTeamInviteUsers();
            } catch (e) {
                console.error("fetchTeamMembers:", e);
            }
        }

        function populateTeamInviteUsers() {
            if (!els.teamInviteUserSelect) return;
            const existing = new Set((state.teamMembers || []).map((m) => String(m.user_id)));
            const available = (state.users || []).filter((u) => !existing.has(String(u.id || u.username)));
            els.teamInviteUserSelect.innerHTML = available.length
                ? available.map((u) => "<option value=\"" + escapeHTML(String(u.id || u.username)) + "\">" + escapeHTML(u.username) + "</option>").join("")
                : "<option value=\"\">No users available</option>";
        }

        function releaseFilterTickets(tickets) {
            const sel = state.selectedReleaseID;
            if (sel === "" || sel === null || sel === undefined) {
                return tickets; // All
            }
            if (sel === "backlog") {
                return tickets.filter((t) => t.release_id === null || t.release_id === undefined);
            }
            const numID = Number(sel);
            return tickets.filter((t) => t.release_id === numID);
        }

        function agentStatusIcon(agent) {
            const status = agent.status || "idle";
            const role = agent.agent_role || "";
            const roleIcon = role === "refiner" ? "🔍" : role === "developer" ? "💻" : role === "tester" ? "🧪" : "🤖";
            let badge = "", badgeClass = "";
            if (!agent.enabled) {
                badge = "✕"; badgeClass = "agent-badge-offline";
            } else if (status === "working") {
                badge = "▶"; badgeClass = "agent-badge-working";
            } else if (status === "disabled" || status === "offline") {
                badge = "!"; badgeClass = "agent-badge-offline";
            } else if (status === "idle" || status === "soliciting") {
                badge = "z"; badgeClass = "agent-badge-idle";
            }
            const badgeHTML = badge ? "<span class=\"agent-icon-badge " + badgeClass + "\">" + badge + "</span>" : "";
            const label = escapeHTML(agent.username || agent.user_id || "agent");
            const ticketKey = agent.ticket_key || "";
            const ticketLine = ticketKey
                ? "<div class=\"agent-popup-row\"><span class=\"agent-popup-key\">Ticket</span><span class=\"agent-popup-val\">" + escapeHTML(ticketKey) + "</span></div>"
                : "";
            const lastSeen = escapeHTML(agent.last_seen || "never");
            const popupHTML = "<div class=\"agent-popup\">" +
                "<div class=\"agent-popup-name\">" + roleIcon + " " + label + "</div>" +
                "<div class=\"agent-popup-row\"><span class=\"agent-popup-key\">Role</span><span class=\"agent-popup-val\">" + escapeHTML(role || "—") + "</span></div>" +
                "<div class=\"agent-popup-row\"><span class=\"agent-popup-key\">Status</span><span class=\"agent-popup-val agent-popup-status\" data-status=\"" + escapeHTML(status) + "\">" + escapeHTML(status) + "</span></div>" +
                ticketLine +
                "<div class=\"agent-popup-row\"><span class=\"agent-popup-key\">Last seen</span><span class=\"agent-popup-val\">" + lastSeen + "</span></div>" +
                "</div>";
            return "<div class=\"agent-icon\" data-status=\"" + status + "\" data-agent-id=\"" + escapeHTML(agent.user_id || "") + "\"" + (ticketKey ? " data-ticket-key=\"" + escapeHTML(ticketKey) + "\"" : "") + ">" +
                roleIcon + badgeHTML + popupHTML + "</div>";
        }

        function renderProjectAgentBar() {
            const bar = els.projectAgentBar;
            if (!bar) return;
            const agents = state.projectAgents || [];
            if (!agents.length) {
                bar.classList.add("hidden");
                return;
            }
            bar.classList.remove("hidden");
            bar.innerHTML = "<span class=\"agent-bar-label\">Agents</span>" +
                agents.map((s) => agentStatusIcon(s.agent || s)).join("");
        }

        function renderReleaseSelect() {
            if (!els.boardReleaseSelect) {
                return;
            }
            const allReleases = state.releases || [];
            const sel = state.selectedReleaseID;

            // Order: most recently created first.
            const ordered = allReleases.slice().sort((a, b) => (b.id || 0) - (a.id || 0));

            const releaseOption = (r) => {
                const label = (r.title || "Release") + " (" + (r.status || "in_design") + ")";
                const selected = String(r.id) === String(sel) ? " selected" : "";
                return "<option value=\"" + r.id + "\"" + selected + ">" + escapeHTML(label) + "</option>";
            };

            const backlogSelected = sel === "backlog" ? " selected" : "";
            const options = [
                "<option value=\"backlog\"" + backlogSelected + ">Backlog</option>",
            ].concat(ordered.map(releaseOption));

            els.boardReleaseSelect.innerHTML = options.join("");
        }

        function renderTicketBoard() {
            const lanes = getBoardLaneDescriptors();
            const searchText = (els.boardSearch && els.boardSearch.value ? els.boardSearch.value : "").trim().toLowerCase();
            const hideDone = Boolean(els.boardHideDone && els.boardHideDone.checked);
            const perspective = state.boardPerspective || "board";

            // Toggle board/list/plan visibility
            if (els.ticketBoard) {
                els.ticketBoard.classList.toggle("hidden", perspective !== "board");
            }
            if (els.ticketListView) {
                els.ticketListView.classList.toggle("hidden", perspective !== "list");
            }
            if (els.ticketPlanView) {
                els.ticketPlanView.classList.toggle("hidden", perspective !== "plan");
            }

            // The release dropdown is a board-only filter — hide it in list/plan views.
            if (els.boardReleaseSelect) {
                els.boardReleaseSelect.classList.toggle("hidden", perspective !== "board");
            }
            // Reflect the active perspective on the segmented toggle buttons.
            document.querySelectorAll("[data-perspective]").forEach((btn) => {
                btn.classList.toggle("active", btn.dataset.perspective === perspective);
            });

            if (perspective !== "board") {
                return;
            }

            // "hide done" removes the done column entirely, not just its cards.
            const visibleLanes = hideDone
                ? lanes.filter((lane) => String(lane.name || "").toLowerCase() !== "done")
                : lanes;

            els.ticketBoard.innerHTML = visibleLanes.map((lane) => {
                const fallbackLane = visibleLanes.length ? visibleLanes[0].name : "";
                const cards = releaseFilterTickets(state.tickets)
                    .filter((ticket) => (ticket.stage || fallbackLane) === lane.name)
                    .filter((ticket) => !searchText || String(ticket.title || "").toLowerCase().includes(searchText) || String(ticket.key || ticket.id || "").toLowerCase().includes(searchText))
                    // Stories being refined (or refined and awaiting promotion) float
                    // to the top of their lane.
                    .sort((a, b) => {
                        const ra = refinementSortRank(a);
                        const rb = refinementSortRank(b);
                        if (ra !== rb) return ra - rb;
                        return (a.order || 0) - (b.order || 0);
                    })
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

        // List-view expand/collapse state persists across re-renders and perspective
        // switches. Keyed per group (release/feature/epic/backlog); an explicit entry
        // overrides the default open state.
        function loadListOpenState() {
            if (state._listOpenState) return state._listOpenState;
            let m = {};
            try { m = JSON.parse(localStorage.getItem("site2.listOpen") || "{}") || {}; } catch (_) { m = {}; }
            state._listOpenState = m;
            return m;
        }
        function listGroupOpen(key, def) {
            const m = loadListOpenState();
            return Object.prototype.hasOwnProperty.call(m, key) ? Boolean(m[key]) : def;
        }
        function setListGroupOpen(key, open) {
            const m = loadListOpenState();
            m[key] = Boolean(open);
            try { localStorage.setItem("site2.listOpen", JSON.stringify(m)); } catch (_) { /* ignore */ }
        }
        // openAttrFor returns ' open' / '' plus the data-ck key attribute for a group.
        function openAttrFor(key, def) {
            return " data-ck=\"" + escapeHTML(key) + "\"" + (listGroupOpen(key, def) ? " open" : "");
        }

        function renderTicketListView() {
            if (!els.ticketListView) {
                return;
            }
            const perspective = state.boardPerspective || "board";
            if (perspective !== "list") {
                return;
            }
            const releases = state.releases || [];
            // "hide done" hides finished work: done-stage stories.
            const hideDone = Boolean(els.boardHideDone && els.boardHideDone.checked);
            // The list view is a full Release → Feature → Epic → Story breakdown and
            // has no release dropdown of its own, so it shows every ticket (the board
            // view is the one filtered by the selected release).
            let tickets = state.tickets || [];
            if (hideDone) {
                tickets = tickets.filter((t) => String(t.stage || "").toLowerCase() !== "done");
            }

            // Index tickets by parent_id so we can walk the hierarchy, and collect the
            // feature tickets keyed by release_id.
            const childrenByParent = {};
            tickets.forEach((t) => {
                const pid = t.parent_id;
                if (pid !== null && pid !== undefined) {
                    if (!childrenByParent[pid]) { childrenByParent[pid] = []; }
                    childrenByParent[pid].push(t);
                }
            });
            const features = tickets.filter((t) => t.type === "feature");
            const featuresByRelease = {};
            const backlogFeatures = [];
            features.forEach((f) => {
                if (f.release_id !== null && f.release_id !== undefined) {
                    if (!featuresByRelease[f.release_id]) { featuresByRelease[f.release_id] = []; }
                    featuresByRelease[f.release_id].push(f);
                } else {
                    backlogFeatures.push(f);
                }
            });

            const sortByOrder = (a, b) => (a.order || 0) - (b.order || 0);
            // Features render newest-first (reverse order).
            const sortFeatures = (list) => list.slice().sort(sortByOrder).reverse();

            // Render a feature and its epic/story subtree as nested <details>.
            function featureBlock(feature) {
                const epics = (childrenByParent[feature.id] || []).slice().sort(sortByOrder);
                const epicsHtml = epics.map((epic) => {
                    const leaves = (childrenByParent[epic.id] || []).slice().sort(sortByOrder);
                    return "<details class=\"release-group release-group-epic\"" + openAttrFor("e:" + epic.id, true) + ">" +
                        "<summary><strong>" + escapeHTML(epic.key || String(epic.id)) + "</strong> " + escapeHTML(epic.title || "(untitled)") +
                        " <span class=\"chip\">" + leaves.length + "</span></summary>" +
                        renderTicketListRows(leaves, false) +
                        "</details>";
                }).join("");
                return "<details class=\"release-group release-group-feature\"" + openAttrFor("f:" + feature.id, true) + ">" +
                    "<summary><strong>" + escapeHTML(feature.key || String(feature.id)) + "</strong> " + escapeHTML(feature.title || "(untitled)") +
                    " <span class=\"chip\">" + epics.length + " epics</span></summary>" +
                    (epicsHtml || "<div class=\"empty\">No epics.</div>") +
                    "</details>";
            }

            function releaseBlock(release, releaseFeatures, extraClass, def) {
                const featuresHtml = sortFeatures(releaseFeatures).map(featureBlock).join("");
                const statusLabel = release.status || "in_design";
                return "<details class=\"release-group" + (extraClass ? " " + extraClass : "") + "\"" + openAttrFor("r:" + release.id, def) + ">" +
                    "<summary><strong>" + escapeHTML(release.title || "Release") + "</strong> " +
                    "<span class=\"chip release-status-" + escapeHTML(statusLabel) + "\">" + escapeHTML(statusLabel) + "</span> " +
                    "<span class=\"chip\">" + (release.feature_count != null ? release.feature_count : releaseFeatures.length) + " features</span> " +
                    "<span class=\"chip\">" + (release.story_count != null ? release.story_count : "") + " stories</span></summary>" +
                    (featuresHtml || "<div class=\"empty\">No features.</div>") +
                    "</details>";
            }

            const selectedReleaseNumID = state.selectedReleaseID && state.selectedReleaseID !== "backlog" ? Number(state.selectedReleaseID) : null;
            // Backlog at the top, then releases in reverse (newest first).
            const ordered = releases.slice().sort((a, b) => (b.id || 0) - (a.id || 0));

            let html = "";
            // Backlog group: features not in any release.
            if (backlogFeatures.length > 0 || state.selectedReleaseID === "backlog" || state.selectedReleaseID === "") {
                const featuresHtml = sortFeatures(backlogFeatures).map(featureBlock).join("");
                html += "<details class=\"release-group release-group-backlog\"" + openAttrFor("r:backlog", true) + ">" +
                    "<summary><strong>Backlog</strong> <span class=\"chip\">" + backlogFeatures.length + " features</span></summary>" +
                    (featuresHtml || "<div class=\"empty\">No features.</div>") +
                    "</details>";
            }
            ordered.forEach((release) => {
                const isSelected = selectedReleaseNumID === release.id;
                const def = isSelected || release.status !== "complete";
                html += releaseBlock(release, featuresByRelease[release.id] || [], "release-group-release", def);
            });

            if (!html) {
                html = "<div class=\"empty\">No tickets.</div>";
            }
            setInnerHTMLIfChanged(els.ticketListView, html);
        }

        function renderTicketPlanView() {
            if (!els.ticketPlanView) {
                return;
            }
            const perspective = state.boardPerspective || "board";
            if (perspective !== "plan") {
                return;
            }
            // "hide done" hides finished work: complete releases and done stories.
            const hideDone = Boolean(els.boardHideDone && els.boardHideDone.checked);
            const releases = (state.releases || []).slice()
                .filter((r) => !hideDone || r.status !== "complete")
                .sort((a, b) => (a.id || 0) - (b.id || 0));
            let allTickets = state.tickets || [];
            if (hideDone) {
                allTickets = allTickets.filter((t) => String(t.stage || "").toLowerCase() !== "done");
            }

            // Only features carry a release; the plan view drags features between
            // releases and the backlog.
            const features = allTickets.filter((t) => t.type === "feature");
            const featuresByRelease = {};
            const backlogFeatures = [];
            features.forEach((f) => {
                if (f.release_id !== null && f.release_id !== undefined) {
                    if (!featuresByRelease[f.release_id]) {
                        featuresByRelease[f.release_id] = [];
                    }
                    featuresByRelease[f.release_id].push(f);
                } else {
                    backlogFeatures.push(f);
                }
            });

            const releasesHtml = releases.map((release) => {
                const feats = featuresByRelease[release.id] || [];
                const isLocked = release.status !== "in_design";
                const draggable = isLocked ? "false" : "true";
                const lockedAttr = isLocked ? " data-release-locked=\"true\"" : "";
                const rowsHtml = feats.map((t) =>
                    "<div class=\"plan-ticket-row" + (isLocked ? " plan-ticket-locked" : "") + "\" draggable=\"" + draggable + "\" data-ticket-id=\"" + escapeHTML(String(t.id)) + "\" data-release-id=\"" + escapeHTML(String(release.id)) + "\">" +
                    "<span class=\"plan-ticket-key\">" + escapeHTML(t.key || String(t.id)) + "</span>" +
                    "<span>" + escapeHTML(t.title || "(untitled)") + "</span>" +
                    "</div>"
                ).join("");
                return "<details class=\"plan-release-group\" data-release-id=\"" + escapeHTML(String(release.id)) + "\"" + lockedAttr + " open>" +
                    "<summary><strong>" + escapeHTML(release.title || "Release") + "</strong> <span class=\"chip release-status-" + escapeHTML(release.status || "in_design") + "\">" + escapeHTML(release.status || "in_design") + "</span> <span class=\"chip\">" + feats.length + "</span></summary>" +
                    "<div class=\"plan-drop-zone\"" + lockedAttr + ">" +
                    (rowsHtml || "<div class=\"plan-empty\">No features</div>") +
                    "</div>" +
                    "</details>";
            }).join("");

            const backlogRowsHtml = backlogFeatures.map((t) =>
                "<div class=\"plan-ticket-row\" draggable=\"true\" data-ticket-id=\"" + escapeHTML(String(t.id)) + "\" data-release-id=\"\">" +
                "<span class=\"plan-ticket-key\">" + escapeHTML(t.key || String(t.id)) + "</span>" +
                "<span>" + escapeHTML(t.title || "(untitled)") + "</span>" +
                "</div>"
            ).join("");

            const html = "<div class=\"plan-pane plan-releases-pane\">" +
                "<div class=\"plan-pane-header\">Releases</div>" +
                (releasesHtml || "<div class=\"plan-empty\">No releases</div>") +
                "</div>" +
                "<div class=\"plan-pane plan-backlog-pane\" data-release-id=\"\">" +
                "<div class=\"plan-pane-header\">Backlog</div>" +
                (backlogRowsHtml || "<div class=\"plan-empty\">No backlog features</div>") +
                "</div>";

            setInnerHTMLIfChanged(els.ticketPlanView, html);
        }

        function renderAdminSummary() {
            if (!els.adminSummaryContent) {
                return;
            }
            if (!isAdmin()) {
                return;
            }

            const users = state.users || [];
            const projects = state.projects || [];
            const teams = state.teams || [];

            const usersHtml = "<div class=\"card admin-summary-card\">" +
                "<div class=\"card-header\"><h2>Users <span class=\"chip\">" + users.length + "</span></h2></div>" +
                "<div class=\"item-list\">" +
                users.map((u) => {
                    const enabled = u.enabled !== false;
                    return "<div class=\"item-row\">" +
                        "<span class=\"item-name\">" + escapeHTML(u.display_name || u.username) + "</span>" +
                        "<span class=\"item-meta\">" + escapeHTML(u.username) + " · " + escapeHTML(u.role || "user") + "</span>" +
                        "<button class=\"btn btn-sm\" data-admin-toggle-user=\"" + escapeHTML(u.username) + "\" data-enabled=\"" + (enabled ? "true" : "false") + "\">" + (enabled ? "Disable" : "Enable") + "</button>" +
                        "</div>";
                }).join("") +
                "</div></div>";

            const projectsHtml = "<div class=\"card admin-summary-card\">" +
                "<div class=\"card-header\"><h2>Projects <span class=\"chip\">" + projects.length + "</span></h2></div>" +
                "<div class=\"item-list\">" +
                projects.map((p) => {
                    const active = (p.status || "active") !== "disabled";
                    const ticketCount = (state.tickets || []).filter((t) => t.project_id === p.id).length;
                    return "<div class=\"item-row\">" +
                        "<span class=\"item-name\">" + escapeHTML(p.title) + " <span class=\"chip\">" + escapeHTML(p.prefix) + "</span></span>" +
                        "<span class=\"item-meta\">" + escapeHTML(p.visibility || "public") + " · " + ticketCount + " tickets</span>" +
                        "<button class=\"btn btn-sm\" data-admin-toggle-project=\"" + escapeHTML(String(p.id)) + "\" data-active=\"" + (active ? "true" : "false") + "\">" + (active ? "Disable" : "Enable") + "</button>" +
                        "</div>";
                }).join("") +
                "</div></div>";

            const teamsHtml = "<div class=\"card admin-summary-card\">" +
                "<div class=\"card-header\"><h2>Teams <span class=\"chip\">" + teams.length + "</span></h2></div>" +
                "<div class=\"item-list\">" +
                teams.map((t) => {
                    return "<div class=\"item-row\">" +
                        "<span class=\"item-name\">" + escapeHTML(t.name) + "</span>" +
                        "<span class=\"item-meta\">" + (t.member_count !== undefined ? t.member_count + " members" : "") + "</span>" +
                        "</div>";
                }).join("") +
                "</div></div>";

            setInnerHTMLIfChanged(els.adminSummaryContent, usersHtml + projectsHtml + teamsHtml);
        }

        function renderTicketListRows(tickets, draggable) {
            if (!tickets.length) {
                return "<div class=\"empty\">No tickets.</div>";
            }
            const dragAttr = draggable ? " draggable=\"true\"" : "";
            // Refining/refined stories float to the top of the list.
            const ordered = tickets.slice().sort((a, b) => {
                const ra = refinementSortRank(a);
                const rb = refinementSortRank(b);
                if (ra !== rb) return ra - rb;
                return (a.order || 0) - (b.order || 0);
            });
            return "<table class=\"ticket-list-table\"><thead><tr><th>ID</th><th>Title</th><th>Stage</th><th>State</th><th>Priority</th><th>Type</th><th>Assignee</th></tr></thead><tbody>" +
                ordered.map((t) => "<tr class=\"ticket-list-row\" data-ticket-id=\"" + escapeHTML(t.id) + "\"" + dragAttr + ">" +
                    "<td>" + ticketAgentDot(t) + escapeHTML(t.key || t.id || "") + "</td>" +
                    "<td>" + escapeHTML(t.title || "(untitled)") + (refinementBadgeHTML(t) ? " " + refinementBadgeHTML(t) : "") + "</td>" +
                    "<td>" + escapeHTML(t.stage || "") + "</td>" +
                    "<td><span class=\"chip chip-state-" + escapeHTML(t.state || "idle") + "\">" + escapeHTML(t.state || "idle") + "</span></td>" +
                    "<td><span class=\"editable-cell\" data-edit-field=\"priority\" title=\"Click to change priority\">p" + escapeHTML(String(t.priority || 0)) + "</span></td>" +
                    "<td><span class=\"editable-cell\" data-edit-field=\"type\" title=\"Click to change type\">" + escapeHTML(t.type || "") + "</span></td>" +
                    "<td>" + escapeHTML(t.assignee || "—") + "</td>" +
                    "</tr>").join("") +
                "</tbody></table>";
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
        }

        // ── Board right-click context menu: assign an idle story to an agent ──
        let boardContextMenuEl = null;

        function onBoardContextMenuKey(event) {
            if (event.key === "Escape") {
                dismissBoardContextMenu();
            }
        }

        function dismissBoardContextMenu() {
            if (boardContextMenuEl) {
                boardContextMenuEl.remove();
                boardContextMenuEl = null;
            }
            document.removeEventListener("click", dismissBoardContextMenu);
            document.removeEventListener("keydown", onBoardContextMenuKey);
            document.removeEventListener("scroll", dismissBoardContextMenu, true);
            window.removeEventListener("blur", dismissBoardContextMenu);
            window.removeEventListener("resize", dismissBoardContextMenu);
        }

        // ticketCurrentRoleName resolves a ticket's current role_id to its title.
        function ticketCurrentRoleName(ticket) {
            if (!ticket || !ticket.role_id) return "";
            const role = (state.roles || []).find((r) => Number(r.id) === Number(ticket.role_id));
            return role ? String(role.title || "") : "";
        }

        function agentPerformsRole(agent, roleName) {
            if (!roleName) return false;
            const target = roleName.toLowerCase();
            return splitAgentRoles(agent.agent_role).some((r) => r.toLowerCase() === target);
        }

        // inDesignReleases returns the releases still in the in_design status, which
        // are the only releases a feature can be added to.
        function inDesignReleases() {
            return (state.releases || []).filter((r) => r.status === "in_design");
        }

        // contextMenuExtraItemsHTML builds the trailing context-menu items shared by
        // every menu variant. For features it offers release add/remove and clone;
        // every variant ends with Delete.
        function contextMenuExtraItemsHTML(ticket) {
            let html = "<div class=\"context-menu-sep\"></div>";
            if (ticket.type === "feature") {
                const currentReleaseID = ticket.release_id;
                // Offer each in_design release the feature is not already in.
                inDesignReleases().forEach((r) => {
                    if (Number(currentReleaseID) === Number(r.id)) { return; }
                    html += "<button type=\"button\" class=\"context-menu-item\" data-move-release=\"" + r.id + "\">" +
                        "<span class=\"context-menu-check\">→</span>" +
                        "<span class=\"context-menu-label\">Add to " + escapeHTML(r.title || "Release") + "<small>add this feature to the release</small></span>" +
                        "</button>";
                });
                if (currentReleaseID !== null && currentReleaseID !== undefined) {
                    html += "<button type=\"button\" class=\"context-menu-item\" data-move-backlog=\"1\">" +
                        "<span class=\"context-menu-check\">←</span>" +
                        "<span class=\"context-menu-label\">Remove from release<small>move this feature to the backlog</small></span>" +
                        "</button>";
                }
                html += "<button type=\"button\" class=\"context-menu-item\" data-clone-feature=\"1\">" +
                    "<span class=\"context-menu-check\">⧉</span>" +
                    "<span class=\"context-menu-label\">Clone feature<small>deep-copy this feature subtree</small></span>" +
                    "</button>";
                html += "<div class=\"context-menu-sep\"></div>";
            }
            html += "<button type=\"button\" class=\"context-menu-item context-menu-danger\" data-delete-ticket=\"1\">" +
                "<span class=\"context-menu-check\">🗑</span>" +
                "<span class=\"context-menu-label\">Delete<small>permanently remove this ticket</small></span>" +
                "</button>";
            return html;
        }

        // handleContextMenuExtraClick dispatches the shared trailing items. Returns
        // true if it handled the click.
        function handleContextMenuExtraClick(btn, ticket) {
            if (btn.dataset.moveRelease) {
                moveTicketToRelease(ticket, Number(btn.dataset.moveRelease));
                return true;
            }
            if (btn.dataset.moveBacklog) {
                moveTicketToRelease(ticket, null);
                return true;
            }
            if (btn.dataset.cloneFeature) {
                cloneFeatureTicket(ticket);
                return true;
            }
            if (btn.dataset.deleteTicket) {
                deleteTicketWithConfirm(ticket);
                return true;
            }
            return false;
        }

        async function moveTicketToRelease(ticket, releaseID) {
            try {
                await apiClient.setTicketRelease(ticket.id, releaseID);
                await loadTickets();
                // Follow the feature to its destination: the board/list views filter by
                // the selected release, so without this the feature would vanish from
                // the current view instead of appearing where it landed.
                setSelectedReleaseFilter(releaseID === null ? "backlog" : String(releaseID));
                renderTicketBoard();
                renderTicketListView();
                renderTicketPlanView();
                setNotice(releaseID === null
                    ? "Removed " + (ticket.key || ticket.id) + " from its release."
                    : "Added " + (ticket.key || ticket.id) + " to the release.");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        async function cloneFeatureTicket(ticket) {
            try {
                const clone = await apiClient.cloneFeature(ticket.id);
                await loadTickets();
                renderTicketBoard();
                renderTicketListView();
                renderTicketPlanView();
                setNotice("Cloned " + (ticket.key || ticket.id) + (clone && clone.key ? " to " + clone.key : "") + ".");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        // setSelectedReleaseFilter changes the active release filter and keeps the
        // release <select> and persisted preference in sync.
        function setSelectedReleaseFilter(value) {
            state.selectedReleaseID = value;
            if (els.boardReleaseSelect) {
                els.boardReleaseSelect.value = value;
            }
            if (state.selectedProjectID) {
                try {
                    localStorage.setItem("site2.release." + state.selectedProjectID, value);
                } catch (_) { /* ignore storage failures */ }
            }
        }

        async function deleteTicketWithConfirm(ticket) {
            const confirmed = await uiConfirm(
                "Delete ticket " + (ticket.key || ticket.id) + "? This cannot be undone.", "Delete");
            if (!confirmed) return;
            try {
                await api("/api/tickets/" + ticket.id, { method: "DELETE" });
                if (state.activeTicket && String(state.activeTicket.id) === String(ticket.id)) {
                    closeTicketModal();
                }
                await loadTickets();
                renderTicketBoard();
                renderTicketListView();
                renderTicketPlanView();
                setNotice("Deleted " + (ticket.key || ticket.id) + ".");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        // openFieldPopup shows a small floating menu anchored to a list-view cell so
        // priority/type can be changed in place. Reuses the context-menu dismiss
        // machinery (boardContextMenuEl + arm/dismiss).
        function openFieldPopup(anchorEl, ticket, field) {
            dismissBoardContextMenu();
            const opts = field === "type"
                ? TICKET_TYPES.map((v) => ({ value: v, label: v }))
                : [0, 1, 2, 3, 4, 5].map((v) => ({ value: v, label: "p" + v }));
            const current = field === "type" ? String(ticket.type || "task") : String(Number(ticket.priority || 0));
            const menu = document.createElement("div");
            menu.className = "context-menu value-popup";
            menu.setAttribute("role", "menu");
            menu.innerHTML = "<div class=\"context-menu-list\">" + opts.map((o) => {
                const sel = String(o.value) === current;
                return "<button type=\"button\" class=\"context-menu-item" + (sel ? " is-match" : "") + "\" data-value=\"" + escapeHTML(String(o.value)) + "\">" +
                    "<span class=\"context-menu-check\">" + (sel ? "✓" : "") + "</span>" +
                    "<span class=\"context-menu-label\">" + escapeHTML(o.label) + "</span>" +
                    "</button>";
            }).join("") + "</div>";
            document.body.appendChild(menu);
            boardContextMenuEl = menu;
            const rect = anchorEl.getBoundingClientRect();
            positionBoardContextMenu(menu, { clientX: rect.left, clientY: rect.bottom + 2 });
            menu.addEventListener("click", (ev) => {
                const btn = ev.target.closest("[data-value]");
                if (!btn) return;
                ev.stopPropagation();
                dismissBoardContextMenu();
                const value = field === "type" ? btn.dataset.value : Number(btn.dataset.value);
                updateTicketField(ticket, field, value);
            });
            armBoardContextMenuDismiss();
        }

        // updateTicketField PUTs a single changed field on a ticket, preserving the rest.
        async function updateTicketField(ticket, field, value) {
            const payload = {
                project_id: ticket.project_id,
                type: ticket.type,
                title: ticket.title,
                description: ticket.description || "",
                acceptance_criteria: ticket.acceptance_criteria || "",
                parent_id: ticket.parent_id || null,
                stage: ticket.stage,
                state: ticket.state,
                assignee: ticket.assignee || "",
                priority: Number(ticket.priority || 0),
                order: Number(ticket.order || 0),
                estimate_effort: Number(ticket.estimate_effort || 0),
                health: Number(ticket.health || 0),
            };
            payload[field] = value;
            try {
                await api("/api/tickets/" + ticket.id, { method: "PUT", body: JSON.stringify(payload) });
                await loadTickets();
                renderTicketBoard();
                renderTicketListView();
                renderTicketPlanView();
                setNotice("Updated " + (ticket.key || ticket.id) + " " + field + ".");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        function openBoardContextMenu(event, ticket) {
            dismissBoardContextMenu();
            const roleName = ticketCurrentRoleName(ticket);

            // A story in a backlog stage is refined, not assigned. Offer "Refine this
            // story" (opens the refinement chat in place), plus promotion to develop:
            // "Move to develop" once it's refined, and an always-available "Force ready
            // for development and move to develop" escape hatch.
            if (ticketStageIsBacklog(ticket)) {
                const refined = ticketIsRefined(ticket);
                const developStage = findDevelopStageName();
                let items =
                    "<button type=\"button\" class=\"context-menu-item is-match\" data-refine-ticket=\"1\">" +
                    "<span class=\"context-menu-check\">✦</span>" +
                    "<span class=\"context-menu-label\">Refine this story<small>open the refinement chat</small></span>" +
                    "</button>";
                if (refined && developStage) {
                    items +=
                        "<button type=\"button\" class=\"context-menu-item is-match\" data-move-develop=\"1\">" +
                        "<span class=\"context-menu-check\">✓</span>" +
                        "<span class=\"context-menu-label\">Move to " + escapeHTML(developStage) + "<small>this story is refined</small></span>" +
                        "</button>";
                }
                if (developStage) {
                    items +=
                        "<button type=\"button\" class=\"context-menu-item\" data-force-develop=\"1\">" +
                        "<span class=\"context-menu-check\">»</span>" +
                        "<span class=\"context-menu-label\">Force ready for development and move to " + escapeHTML(developStage) + "<small>skip refinement</small></span>" +
                        "</button>";
                }
                const menu = document.createElement("div");
                menu.className = "context-menu";
                menu.setAttribute("role", "menu");
                menu.innerHTML = "<div class=\"context-menu-header\">" + escapeHTML(ticket.key || ticket.id) +
                    " <span class=\"context-menu-role\">" + escapeHTML(ticket.stage || "backlog") + "</span></div>" +
                    "<div class=\"context-menu-list\">" + items + contextMenuExtraItemsHTML(ticket) + "</div>";
                document.body.appendChild(menu);
                boardContextMenuEl = menu;
                positionBoardContextMenu(menu, event);
                menu.addEventListener("click", (clickEvent) => {
                    const btn = clickEvent.target.closest("[data-refine-ticket],[data-move-develop],[data-force-develop],[data-move-release],[data-move-backlog],[data-clone-feature],[data-delete-ticket]");
                    if (!btn) return;
                    clickEvent.stopPropagation();
                    dismissBoardContextMenu();
                    if (handleContextMenuExtraClick(btn, ticket)) return;
                    if (btn.dataset.refineTicket) {
                        refineStory(ticket);
                    } else if (btn.dataset.moveDevelop || btn.dataset.forceDevelop) {
                        moveTicketToDevelop(ticket);
                    }
                });
                armBoardContextMenuDismiss();
                return;
            }

            // Readiness gate: if the workflow requires stories to be ready before
            // assignment and this one is not ready yet, don't offer assignment — it
            // must be refined/readied first.
            const projectWorkflow = getCurrentProjectWorkflow();
            if (workflowRequiresReady(projectWorkflow) && !ticketIsReadyForAssignment(ticket)) {
                const menu = document.createElement("div");
                menu.className = "context-menu";
                menu.setAttribute("role", "menu");
                menu.innerHTML = "<div class=\"context-menu-header\">" + escapeHTML(ticket.key || ticket.id) + "</div>" +
                    "<div class=\"context-menu-empty\">Not ready for assignment — refine this story first.</div>" +
                    "<div class=\"context-menu-list\">" + contextMenuExtraItemsHTML(ticket) + "</div>";
                document.body.appendChild(menu);
                boardContextMenuEl = menu;
                positionBoardContextMenu(menu, event);
                menu.addEventListener("click", (clickEvent) => {
                    const btn = clickEvent.target.closest("[data-move-release],[data-move-backlog],[data-clone-feature],[data-delete-ticket]");
                    if (!btn) return;
                    clickEvent.stopPropagation();
                    dismissBoardContextMenu();
                    handleContextMenuExtraClick(btn, ticket);
                });
                armBoardContextMenuDismiss();
                return;
            }

            // Distinct enabled agents available on this project (includes globals).
            const seen = new Set();
            const agents = [];
            (state.projectAgents || []).map((s) => s.agent || s).forEach((a) => {
                if (!a || !a.username) return;
                const key = String(a.username).toLowerCase();
                if (seen.has(key) || a.enabled === false) return;
                seen.add(key);
                agents.push(a);
            });
            // Agents whose role matches the ticket's current role come first.
            agents.sort((a, b) => {
                const ma = agentPerformsRole(a, roleName) ? 0 : 1;
                const mb = agentPerformsRole(b, roleName) ? 0 : 1;
                if (ma !== mb) return ma - mb;
                return String(a.username).localeCompare(String(b.username));
            });

            const header = "<div class=\"context-menu-header\">Assign " + escapeHTML(ticket.key || ticket.id) +
                (roleName ? " <span class=\"context-menu-role\">" + escapeHTML(roleName) + "</span>" : "") + "</div>";
            let items;
            if (!agents.length) {
                items = "<div class=\"context-menu-empty\">No agents available</div>";
            } else {
                items = agents.map((a) => {
                    const isMatch = agentPerformsRole(a, roleName);
                    const roles = splitAgentRoles(a.agent_role);
                    const sub = roles.length ? roles.join(", ") : "any role";
                    return "<button type=\"button\" class=\"context-menu-item" + (isMatch ? " is-match" : "") + "\" data-assign-agent=\"" + escapeHTML(a.username) + "\">" +
                        "<span class=\"context-menu-check\">" + (isMatch ? "✓" : "") + "</span>" +
                        "<span class=\"context-menu-label\">" + escapeHTML(a.username) + "<small>" + escapeHTML(sub) + "</small></span>" +
                        "</button>";
                }).join("");
            }
            const footer = ticket.assignee
                ? "<button type=\"button\" class=\"context-menu-item context-menu-unassign\" data-assign-agent=\"\">Unassign (" + escapeHTML(ticket.assignee) + ")</button>"
                : "";

            const menu = document.createElement("div");
            menu.className = "context-menu";
            menu.setAttribute("role", "menu");
            menu.innerHTML = header + "<div class=\"context-menu-list\">" + items + "</div>" + footer +
                "<div class=\"context-menu-list\">" + contextMenuExtraItemsHTML(ticket) + "</div>";
            document.body.appendChild(menu);
            boardContextMenuEl = menu;
            positionBoardContextMenu(menu, event);

            menu.addEventListener("click", (clickEvent) => {
                const btn = clickEvent.target.closest("[data-assign-agent],[data-move-release],[data-move-backlog],[data-clone-feature],[data-delete-ticket]");
                if (!btn) return;
                clickEvent.stopPropagation();
                dismissBoardContextMenu();
                if (handleContextMenuExtraClick(btn, ticket)) return;
                const username = btn.dataset.assignAgent || "";
                assignTicketToAgent(ticket, username);
            });
            armBoardContextMenuDismiss();
        }

        // positionBoardContextMenu places the menu at the cursor, kept within the viewport.
        function positionBoardContextMenu(menu, event) {
            let x = event.clientX;
            let y = event.clientY;
            if (x + menu.offsetWidth > window.innerWidth - 8) {
                x = window.innerWidth - menu.offsetWidth - 8;
            }
            if (y + menu.offsetHeight > window.innerHeight - 8) {
                y = window.innerHeight - menu.offsetHeight - 8;
            }
            menu.style.left = Math.max(8, x) + "px";
            menu.style.top = Math.max(8, y) + "px";
        }

        // armBoardContextMenuDismiss wires the outside-click/escape/scroll dismissers,
        // deferred so the opening right-click doesn't immediately close the menu.
        function armBoardContextMenuDismiss() {
            setTimeout(() => {
                document.addEventListener("click", dismissBoardContextMenu);
                document.addEventListener("keydown", onBoardContextMenuKey);
                document.addEventListener("scroll", dismissBoardContextMenu, true);
                window.addEventListener("blur", dismissBoardContextMenu);
                window.addEventListener("resize", dismissBoardContextMenu);
            }, 0);
        }

        // refineStory puts a backlog story into refinement IN PLACE — it marks the
        // ticket a draft (the refinement signal) without changing its stage — and
        // opens the refinement chat. No move to a literal "refine" stage.
        async function refineStory(ticket) {
            try {
                if (!ticket.draft) {
                    await api("/api/tickets/" + ticket.id + "/draft", { method: "POST" });
                }
                await loadTickets();
                renderTicketBoard();
                renderTicketListView();
                const fresh = state.tickets.find((t) => String(t.id) === String(ticket.id)) || Object.assign({}, ticket, { draft: true });
                openTicketModal(fresh);
                setNotice("Refining " + (ticket.key || ticket.id) + " — chat with the refiner below.");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        // moveTicketToDevelop promotes a refined (or force-readied) backlog story into
        // the develop stage: it clears the draft flag (readiness) and moves the stage,
        // leaving it idle and unassigned so it enters the development claim pool.
        async function moveTicketToDevelop(ticket) {
            const developStage = findDevelopStageName();
            if (!developStage) {
                setNotice("This workflow has no development stage to move into.", true);
                return;
            }
            try {
                if (ticket.draft) {
                    await api("/api/tickets/" + ticket.id + "/undraft", { method: "POST" });
                }
                const payload = {
                    project_id: ticket.project_id,
                    type: ticket.type,
                    title: ticket.title,
                    description: ticket.description || "",
                    acceptance_criteria: ticket.acceptance_criteria || "",
                    parent_id: ticket.parent_id || null,
                    stage: developStage,
                    state: "idle",
                    assignee: "",
                    priority: Number(ticket.priority || 0),
                    order: Number(ticket.order || 0),
                    estimate_effort: Number(ticket.estimate_effort || 0),
                    health: Number(ticket.health || 0),
                };
                await api("/api/tickets/" + ticket.id, { method: "PUT", body: JSON.stringify(payload) });
                await loadTickets();
                renderTicketBoard();
                renderTicketListView();
                renderTicketPlanView();
                setNotice("Moved " + (ticket.key || ticket.id) + " to " + developStage + ".");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        // assignTicketToAgent assigns (username set) or unassigns (empty) an idle
        // ticket. Assigning sets state=active so the agent resumes it on next poll;
        // unassigning returns it to idle so it re-enters the claim pool.
        async function assignTicketToAgent(ticket, agentUsername) {
            const assigning = Boolean(agentUsername);
            const payload = {
                project_id: ticket.project_id,
                type: ticket.type,
                title: ticket.title,
                description: ticket.description || "",
                acceptance_criteria: ticket.acceptance_criteria || "",
                parent_id: ticket.parent_id || null,
                stage: ticket.stage,
                state: assigning ? "active" : "idle",
                assignee: agentUsername,
                priority: Number(ticket.priority || 0),
                order: Number(ticket.order || 0),
                estimate_effort: Number(ticket.estimate_effort || 0),
                health: Number(ticket.health || 0),
            };
            try {
                await api("/api/tickets/" + ticket.id, { method: "PUT", body: JSON.stringify(payload) });
                await loadTickets();
                renderTicketBoard();
                renderTicketListView();
                renderTicketPlanView();
                setNotice(assigning
                    ? "Assigned " + (ticket.key || ticket.id) + " to " + agentUsername + "."
                    : "Unassigned " + (ticket.key || ticket.id) + ".");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        function ticketAgentDot(ticket) {
            if (!ticket.assignee) return "";
            const agents = state.projectAgents || [];
            const agentStatus = agents.find((s) => (s.agent || s).username === ticket.assignee);
            if (!agentStatus) return "";
            const a = agentStatus.agent || agentStatus;
            const isWorking = a.status === "working";
            if (!isWorking && a.user_type !== "agent") return "";
            return "<span class=\"ticket-agent-dot " + (isWorking ? "working" : "idle") + "\" title=\"" + escapeHTML(a.username) + " (" + a.status + ")\">🤖</span>";
        }

        function renderTicketCard(ticket) {
            const agentDot = ticketAgentDot(ticket);
            // The accent outline marks a story the refiner is ACTIVELY working right
            // now — not every backlog draft. Passive refinement state is conveyed by
            // the chip badge alone.
            const refinerWorking = ticketInRefinement(ticket) && ticket.state === "active" &&
                String(ticket.assignee || "").trim() !== "";
            const cls = "ticket-card" + (refinerWorking ? " ticket-card-refining" : "");
            return "<div class=\"" + cls + "\" draggable=\"true\" data-ticket-id=\"" + ticket.id + "\">" +
                "<div class=\"panel-head panel-head-tight\">" + agentDot + "<h4>" + escapeHTML(ticket.key || ticket.id || "New") + "</h4><span class=\"chip\">" + escapeHTML(ticket.type || "task") + "</span></div>" +
                "<p>" + escapeHTML(ticket.title || "(untitled)") + "</p>" +
                "<div class=\"tag-row\">" +
                "<span class=\"chip\">p" + escapeHTML(ticket.priority || 0) + "</span>" +
                refinementBadgeHTML(ticket) +
                "</div>" +
                "</div>";
        }

        function renderEditors() {
            renderProjectEditor();
            renderDocumentEditor();
            renderWorkflowEditor();
            renderRoleEditor();
            renderAgentEditor();
            renderTeamEditor();
            // Populates the Settings → AI Providers panel (provider list + the
            // system/project agent-model selectors). Without this it only rendered
            // on a select-change event, so a freshly opened panel was empty.
            renderAgentHarnessEditor();
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
            document.getElementById("project-accepts-new-members").value = String(Boolean(project.accepts_new_members));
            document.getElementById("project-default-draft").value = String(Boolean(project.default_draft));
            document.getElementById("project-workflow").value = project.workflow_id || "";
            const orchestratorSelect = document.getElementById("project-orchestrator-enabled");
            if (orchestratorSelect) {
                orchestratorSelect.value = "true";
                if (project.id) {
                    const orchestratorProjectID = project.id;
                    (async () => {
                        try {
                            const result = await api("/api/projects/" + orchestratorProjectID + "/orchestrator");
                            // Ignore if the user switched projects while this was in flight.
                            const current = state.selectedProjectDraft || emptyProject();
                            if (current.id === orchestratorProjectID) {
                                orchestratorSelect.value = String(Boolean(result && result.enabled));
                            }
                        } catch (error) {
                            /* default to "true" on error */
                        }
                    })();
                }
            }
            document.getElementById("delete-project-button").disabled = !project.id;
            renderProjectAccessRequestsPanel();
            renderProjectHistoryPanel();
            renderProjectRequestAccessPanel();
            renderMyProjectAccessRequestsPanel();
            renderMyNotificationsPanel();
        }

        function renderProjectRequestAccessPanel() {
            if (!els.projectRequestAccessPanel || !els.projectRequestAccessRef) {
                return;
            }
            const project = getCurrentProject();
            if (!String(els.projectRequestAccessRef.value || "").trim() && project && (project.prefix || project.id)) {
                els.projectRequestAccessRef.value = String(project.prefix || project.id);
            }
        }

        function renderProjectAccessRequestsPanel() {
            if (!els.projectAccessRequestsPanel || !els.projectAccessRequestsList) {
                return;
            }
            const project = getCurrentProject();
            const visible = Boolean(project && project.id && state.projectAccessReviewEnabled);
            els.projectAccessRequestsPanel.classList.toggle("hidden", !visible);
            if (!visible) {
                return;
            }
            if (els.projectAccessRequestsSummary) {
                const count = Array.isArray(state.projectAccessRequests) ? state.projectAccessRequests.length : 0;
                els.projectAccessRequestsSummary.textContent =
                    "Membership requests are " + (project.accepts_new_members ? "open" : "closed") +
                    " · " + (count ? String(count) + " request(s)" : "no requests");
            }
            if (!state.projectAccessRequests.length) {
                els.projectAccessRequestsList.innerHTML = "<div class=\"empty\">No access requests for this project.</div>";
                return;
            }
            els.projectAccessRequestsList.innerHTML = state.projectAccessRequests.map((request) => {
                const actions = request.status === "pending"
                    ? "<div class=\"entity-actions\">" +
                        "<button type=\"button\" class=\"btn-primary\" data-project-access-request-action=\"approve\" data-project-access-request-id=\"" + request.id + "\">Approve</button>" +
                        "<button type=\"button\" class=\"btn-danger\" data-project-access-request-action=\"reject\" data-project-access-request-id=\"" + request.id + "\">Reject</button>" +
                        "</div>"
                    : "";
                return "<div class=\"history-item\">" +
                    "<div><strong>" + escapeHTML(request.username || request.user_id || "user") + "</strong></div>" +
                    "<div class=\"meta\">request #" + escapeHTML(String(request.id || "")) + " · " + escapeHTML(request.status || "pending") + " · " + escapeHTML(request.created_at || "") + "</div>" +
                    "<div>" + escapeHTML(request.message || "(no message)") + "</div>" +
                    (request.decision_message
                        ? "<div class=\"meta\">Decision: " + escapeHTML(request.decision_message) +
                            (request.decided_by ? " · by " + escapeHTML(request.decided_by) : "") +
                            (request.decided_at ? " · " + escapeHTML(request.decided_at) : "") +
                            "</div>"
                        : "") +
                    actions +
                    "</div>";
            }).join("");
        }

        function renderProjectHistoryPanel() {
            if (!els.projectHistoryPanel || !els.projectHistoryList) {
                return;
            }
            const project = getCurrentProject();
            const visible = Boolean(project && project.id);
            els.projectHistoryPanel.classList.toggle("hidden", !visible);
            if (!visible) {
                return;
            }
            if (els.projectHistorySummary) {
                if (state.projectHistoryError) {
                    els.projectHistorySummary.textContent = state.projectHistoryError;
                } else {
                    const count = Array.isArray(state.projectHistory) ? state.projectHistory.length : 0;
                    els.projectHistorySummary.textContent = count
                        ? "Showing the latest " + String(count) + " project event(s)."
                        : "No project history yet.";
                }
            }
            if (state.projectHistoryError) {
                els.projectHistoryList.innerHTML = "<div class=\"empty\">" + escapeHTML(state.projectHistoryError) + "</div>";
                return;
            }
            if (!state.projectHistory.length) {
                els.projectHistoryList.innerHTML = "<div class=\"empty\">No history yet for this project.</div>";
                return;
            }
            els.projectHistoryList.innerHTML = state.projectHistory.map((event) => {
                const label = event.ticket_key || event.ticket_id || "project";
                const actor = event.created_by || "system";
                return "<div class=\"history-item\">" +
                    "<div><strong>" + escapeHTML(label) + "</strong> — " + escapeHTML(formatProjectHistorySummary(event)) + "</div>" +
                    "<div class=\"meta\">" + escapeHTML(actor) + " · " + escapeHTML(event.created_at || "") + "</div>" +
                    "</div>";
            }).join("");
        }

        function renderMyProjectAccessRequestsPanel() {
            if (!els.projectMyAccessRequestsPanel || !els.projectMyAccessRequestsList) {
                return;
            }
            if (els.projectMyAccessRequestsSummary) {
                const count = Array.isArray(state.myProjectAccessRequests) ? state.myProjectAccessRequests.length : 0;
                els.projectMyAccessRequestsSummary.textContent = count
                    ? String(count) + " request(s) across your pending and decided project access history."
                    : "You have not submitted any project access requests yet.";
            }
            if (!state.myProjectAccessRequests.length) {
                els.projectMyAccessRequestsList.innerHTML = "<div class=\"empty\">No access requests yet.</div>";
                return;
            }
            els.projectMyAccessRequestsList.innerHTML = state.myProjectAccessRequests.map((request) => {
                const projectRef = request.project_prefix || String(request.project_id || "");
                const projectLabel = projectRef + (request.project_title ? " (" + request.project_title + ")" : "");
                return "<div class=\"history-item\">" +
                    "<div><strong>" + escapeHTML(projectLabel) + "</strong></div>" +
                    "<div class=\"meta\">request #" + escapeHTML(String(request.id || "")) + " · " + escapeHTML(request.status || "pending") + " · updated " + escapeHTML(request.updated_at || "") + "</div>" +
                    "<div>" + escapeHTML(request.message || "(no message)") + "</div>" +
                    (request.decision_message
                        ? "<div class=\"meta\">Decision: " + escapeHTML(request.decision_message) +
                            (request.decided_by ? " · by " + escapeHTML(request.decided_by) : "") +
                            (request.decided_at ? " · " + escapeHTML(request.decided_at) : "") +
                            "</div>"
                        : "") +
                    "</div>";
            }).join("");
        }

        function renderMyNotificationsPanel() {
            if (!els.projectNotificationsPanel || !els.projectNotificationsList) {
                return;
            }
            const unreadCount = state.myNotifications.filter((notification) => notification.status !== "read").length;
            if (els.projectNotificationsSummary) {
                els.projectNotificationsSummary.textContent = state.myNotifications.length
                    ? String(unreadCount) + " unread notification(s) across your latest " + String(state.myNotifications.length) + " update(s)."
                    : "No notifications yet.";
            }
            if (!state.myNotifications.length) {
                els.projectNotificationsList.innerHTML = "<div class=\"empty\">No notifications yet.</div>";
                return;
            }
            els.projectNotificationsList.innerHTML = state.myNotifications.map((notification) => {
                const notificationID = notification.notification_id || notification.id || "";
                const action = notification.status === "read"
                    ? ""
                    : "<div class=\"entity-actions\"><button type=\"button\" data-notification-read=\"" + escapeHTML(String(notificationID)) + "\">Mark read</button></div>";
                const isPR = notification.kind === "pr_ready_for_review";
                let payload = {};
                try { payload = JSON.parse(notification.payload || "{}"); } catch (_) {}
                const prLink = isPR && payload.pr_url
                    ? "<div><a href=\"" + escapeHTML(payload.pr_url) + "\" target=\"_blank\" rel=\"noopener\">Open PR ↗</a></div>"
                    : "";
                return "<div class=\"history-item" + (isPR ? " notif-pr-ready" : "") + "\">" +
                    "<div><strong>" + (isPR ? "🔀 " : "") + escapeHTML(notification.title || notification.kind || "notification") + "</strong></div>" +
                    "<div class=\"meta\">" + escapeHTML(notification.status || "unread") + " · " + escapeHTML(notification.created_at || "") + "</div>" +
                    "<div>" + escapeHTML(notification.message || "") + "</div>" +
                    prLink +
                    action +
                    "</div>";
            }).join("");
        }

        function renderWorkflowEditor() {
            const workflow = state.selectedWorkflowDraft || emptyWorkflow();
            document.getElementById("workflow-editor-title").textContent = workflow.id ? "Workflow board: " + workflow.name : "Workflow board";
            document.getElementById("workflow-name").value = workflow.name || "";
            document.getElementById("workflow-description").value = workflow.description || "";
            const approval = document.querySelector("input[name=\"workflow-approval-policy\"][value=\"" + (workflow.approval_policy || "single_role") + "\"]");
            const progression = document.querySelector("input[name=\"workflow-progression-mode\"][value=\"" + (workflow.progression_mode || "linear") + "\"]");
            if (approval) {
                approval.checked = true;
            }
            if (progression) {
                progression.checked = true;
            }
            document.getElementById("delete-workflow-button").disabled = !workflow.id;
            document.getElementById("new-stage-name").disabled = !workflow.id;
            document.getElementById("new-stage-wow").disabled = !workflow.id;
            document.getElementById("new-stage-dor").disabled = !workflow.id;
            document.getElementById("new-stage-dod").disabled = !workflow.id;
            document.getElementById("save-stage-button").disabled = !workflow.id;
            if (els.workflowSettings && !workflow.id) {
                els.workflowSettings.open = true;
            }
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

        function isTextUploadableFile(file) {
            if (!file) {
                return false;
            }
            const contentType = String(file.type || "").toLowerCase();
            if (contentType.startsWith("text/")) {
                return true;
            }
            const name = String(file.name || "").toLowerCase();
            return /\.(txt|md|markdown|json|ya?ml|csv|tsv|xml|html?|css|js|jsx|ts|tsx|go|py|java|rb|php|sh|sql|log)$/.test(name);
        }

        function setDocumentDropState(nextState) {
            const view = els.documentsView;
            if (!view) {
                return;
            }
            view.classList.remove("document-drop-active", "document-drop-uploading", "document-drop-success");
            if (nextState) {
                view.classList.add("document-drop-" + nextState);
            }
        }

        function clearDocumentDropState() {
            setDocumentDropState("");
            documentDropDepth = 0;
            if (documentDropSuccessTimer) {
                clearTimeout(documentDropSuccessTimer);
                documentDropSuccessTimer = null;
            }
        }

        async function uploadFileToCurrentDocument(selectedFile, overrideName) {
            const draft = state.selectedDocumentDraft || emptyDocument(state.selectedProjectID);
            if (!draft.id) {
                setNotice("Save the document before uploading files.", true);
                return false;
            }
            if (!selectedFile) {
                setNotice("Choose a file first.", true);
                return false;
            }
            if (!isTextUploadableFile(selectedFile)) {
                setNotice("Only text files can be uploaded here.", true);
                return false;
            }
            const buffer = await selectedFile.arrayBuffer();
            const payload = {
                file_name: String(overrideName || "").trim() || selectedFile.name,
                content_type: selectedFile.type || "text/plain",
                content: arrayBufferToBase64(buffer),
            };
            await api("/api/documents/" + draft.id + "/files", {
                method: "POST",
                body: JSON.stringify(payload),
            });
            await loadDocumentFiles(draft.id);
            renderEditors();
            return true;
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
            document.getElementById("agent-editor-title").textContent = agent ? "Agent: " + (agent.username || agent.id) : "Agent editor";
            document.getElementById("agent-id").value = agent ? agent.id : "";
            document.getElementById("agent-username").value = agent ? (agent.username || "") : "";
            document.getElementById("agent-username").disabled = !agent;
            document.getElementById("agent-enabled").value = agent ? String(Boolean(agent.enabled)) : "";
            populateAgentRoleOptions(agent ? agent.agent_role : "");
            document.getElementById("agent-new-password").value = "";
            document.getElementById("save-agent-button").disabled = !agent;
            document.getElementById("toggle-agent-button").disabled = !agent;
            document.getElementById("delete-agent-button").disabled = !agent;
        }

        // populateAgentRoleOptions fills the agent-role multi-select with the
        // distinct role titles defined across all workflows (matched by name),
        // plus a "Refiner" pseudo-role, and selects the agent's current roles.
        function populateAgentRoleOptions(agentRole) {
            const select = document.getElementById("agent-role");
            if (!select) return;
            const selected = splitAgentRoles(agentRole);
            const selectedLower = new Set(selected.map((r) => r.toLowerCase()));
            // Distinct role titles (case-insensitive), preserving first-seen casing.
            const seen = new Map();
            (state.roles || []).forEach((role) => {
                const title = String(role.title || "").trim();
                if (title && !seen.has(title.toLowerCase())) {
                    seen.set(title.toLowerCase(), title);
                }
            });
            // Always offer the Refiner pseudo-role.
            if (!seen.has("refiner")) {
                seen.set("refiner", "Refiner");
            }
            // Include any currently-assigned role not otherwise present (custom).
            selected.forEach((r) => {
                if (!seen.has(r.toLowerCase())) {
                    seen.set(r.toLowerCase(), r);
                }
            });
            const options = Array.from(seen.values()).sort((a, b) => a.localeCompare(b));
            select.innerHTML = options.map((title) => {
                const isSel = selectedLower.has(title.toLowerCase()) ? " selected" : "";
                return "<option value=\"" + escapeHTML(title) + "\"" + isSel + ">" + escapeHTML(title) + "</option>";
            }).join("");
            select.disabled = !getCurrentAgent();
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
                await finalizeAuthenticatedSession({
                    username: (authBody.user && authBody.user.username) || username,
                    token: authBody.token,
                });
            } catch (error) {
                resetAuthFailure(error.message);
            }
        }

        async function handlePasskeyLogin() {
            const username = String(document.getElementById("login-username").value || "").trim();
            if (!username) {
                els.loginError.textContent = "Enter your username before using a passkey.";
                focusLoginUsername();
                return;
            }
            if (!browserSupportsPasskeys()) {
                els.loginError.textContent = "This browser does not support passkey sign-in.";
                return;
            }
            try {
                els.loginError.textContent = "";
                const auth = await completePasskeyLogin(username);
                await finalizeAuthenticatedSession(auth);
            } catch (error) {
                resetAuthFailure(error.message);
            }
        }

        async function handlePasskeyEnrollment() {
            if (!browserSupportsPasskeyEnrollment()) {
                setPasskeyStatus("This browser does not support passkey enrollment.", true);
                renderAccountModal();
                return;
            }
            const label = String(els.accountPasskeyName && els.accountPasskeyName.value || "").trim();
            state.passkeyBusy = true;
            setPasskeyStatus("", false);
            renderAccountModal();
            try {
                const start = await apiClient.startPasskeyRegistration(label);
                const challenge = await apiClient.getPasskeyChallenge(start.code);
                if (!challenge || challenge.kind !== "registration") {
                    throw new Error("passkey enrollment challenge was not available");
                }
                const credential = await navigator.credentials.create({
                    publicKey: normalizePasskeyCreationOptions(challenge.public_key),
                });
                await apiClient.finishPasskeyFlow(start.code, serializePasskeyCredential(credential));
                await loadPasskeys();
                if (els.accountPasskeyName) {
                    els.accountPasskeyName.value = "";
                }
                setPasskeyStatus("Passkey enrolled.", false);
            } catch (error) {
                setPasskeyStatus(error.message, true);
            } finally {
                state.passkeyBusy = false;
                renderAccountModal();
            }
        }

        async function handlePasskeyRename(credentialID) {
            const current = state.passkeys.find((item) => item.id === credentialID);
            const nextName = await uiPrompt("Rename passkey", current && current.name ? current.name : "", "Save");
            if (nextName === null) {
                return;
            }
            const name = String(nextName || "").trim();
            if (!name) {
                await uiAlert("Passkey name is required.");
                return;
            }
            state.passkeyBusy = true;
            setPasskeyStatus("", false);
            renderAccountModal();
            try {
                await apiClient.renameMyPasskey(credentialID, name);
                await loadPasskeys();
                setPasskeyStatus("Passkey renamed.", false);
            } catch (error) {
                setPasskeyStatus(error.message, true);
            } finally {
                state.passkeyBusy = false;
                renderAccountModal();
            }
        }

        async function handlePasskeyDelete(credentialID) {
            const current = state.passkeys.find((item) => item.id === credentialID);
            const confirmed = await uiConfirm("Delete passkey " + (current && current.name ? "\"" + current.name + "\"" : "\"" + credentialID + "\"") + "?", "Delete");
            if (!confirmed) {
                return;
            }
            state.passkeyBusy = true;
            setPasskeyStatus("", false);
            renderAccountModal();
            try {
                await apiClient.deleteMyPasskey(credentialID);
                await loadPasskeys();
                setPasskeyStatus("Passkey deleted.", false);
            } catch (error) {
                setPasskeyStatus(error.message, true);
            } finally {
                state.passkeyBusy = false;
                renderAccountModal();
            }
        }

        async function openAccountModal() {
            state.accountModalOpen = true;
            setPasskeyStatus("", false);
            renderAccountModal();
            await loadPasskeys();
            renderAccountModal();
        }

        function closeAccountModal() {
            state.accountModalOpen = false;
            renderAccountModal();
        }

        async function handleRegister(event) {
            event.preventDefault();
            const formData = new FormData(els.registerForm);
            const username = String(formData.get("username") || "").trim();
            const email = String(formData.get("email") || "").trim();
            const password = String(formData.get("password") || "");
            try {
                const response = await apiClient.register(username, password, email);
                const generatedPassword = String(response.password || "");
                document.getElementById("login-username").value = username;
                document.getElementById("login-password").value = generatedPassword || password;
                els.registerForm.reset();
                showLoginForm();
                if (response.approved === false) {
                    if (generatedPassword) {
                        els.loginError.textContent = "Registered. Save the generated password and wait for an admin to approve your account.";
                    } else {
                        els.loginError.textContent = "Registered. Wait for an admin to approve your account before signing in.";
                    }
                } else if (generatedPassword) {
                    els.loginError.textContent = "Registered. A generated password has been filled into the sign-in form.";
                } else {
                    els.loginError.textContent = "Registered. You can now sign in.";
                }
            } catch (error) {
                els.loginError.textContent = error.message;
            }
        }

        function bindProjectHandlers() {
            els.projectList.addEventListener("click", (event) => {
                const defaultButton = event.target.closest("[data-project-default-id]");
                if (defaultButton) {
                    event.stopPropagation();
                    const projectID = Number(defaultButton.dataset.projectDefaultId);
                    const isDefault = currentDefaultProjectID() === projectID;
                    const request = isDefault
                        ? api("/api/users/me/default-project", { method: "DELETE" })
                        : api("/api/users/me/default-project", { method: "PUT", body: JSON.stringify({ project_ref: String(projectID) }) });
                    request.then(() => {
                        if (!state.status) {
                            state.status = {};
                        }
                        if (!state.status.user) {
                            state.status.user = {};
                        }
                        state.status.user.default_project_id = isDefault ? null : projectID;
                        renderAll();
                        setNotice(isDefault ? "Default project cleared." : "Default project updated.");
                    }).catch((error) => setNotice(error.message, true));
                    return;
                }
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
                Promise.all([loadProjectAgentModelConfig(), loadTickets(), loadReleases(), loadDocuments(), loadProjectAccessRequests(), loadProjectHistory(), loadMyProjectAccessRequests(), loadMyNotifications(), loadProjectAgents()]).then(renderAll).catch((error) => setNotice(error.message, true));
            });

            document.getElementById("new-project-button").addEventListener("click", () => {
                state.selectedProjectID = null;
                storeSelectedProjectID(state.selectedProjectID);
                state.selectedProjectDraft = emptyProject();
                state.projectAgentModelConfig = emptyAgentModelConfig();
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
                    accepts_new_members: normalizeBool(document.getElementById("project-accepts-new-members").value),
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
                    const orchestratorSelect = document.getElementById("project-orchestrator-enabled");
                    if (orchestratorSelect) {
                        try {
                            await api("/api/projects/" + project.id + "/orchestrator", {
                                method: "POST",
                                body: JSON.stringify({ enabled: orchestratorSelect.value === "true" }),
                            });
                        } catch (error) {
                            setNotice("Project saved, but orchestrator setting failed: " + error.message, true);
                        }
                    }
                    state.selectedProjectID = project.id;
                    storeSelectedProjectID(state.selectedProjectID);
                    await Promise.all([loadProjects(), loadWorkflows()]);
                    await Promise.all([loadTickets(), loadReleases(), loadDocuments(), loadProjectAccessRequests(), loadProjectHistory(), loadMyProjectAccessRequests(), loadMyNotifications(), loadProjectAgents()]);
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
                const confirmed = await uiConfirm("Delete project " + (draft.title ? "\"" + draft.title + "\"" : "#" + draft.id) + "?", "Delete");
                if (!confirmed) {
                    return;
                }
                try {
                    await api("/api/projects/" + draft.id, { method: "DELETE" });
                    state.selectedProjectID = null;
                    storeSelectedProjectID(state.selectedProjectID);
                    state.selectedProjectDraft = emptyProject();
                    state.projectAccessRequests = [];
                    state.projectAccessReviewEnabled = false;
                    state.projectAgentModelConfig = emptyAgentModelConfig();
                    await loadProjects();
                    await Promise.all([loadTickets(), loadReleases(), loadDocuments(), loadProjectAccessRequests(), loadProjectHistory(), loadMyProjectAccessRequests(), loadMyNotifications(), loadProjectAgents()]);
                    renderAll();
                    setNotice("Project deleted.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            if (els.projectRequestAccessForm) {
                els.projectRequestAccessForm.addEventListener("submit", async (event) => {
                    event.preventDefault();
                    const projectRef = String(els.projectRequestAccessRef && els.projectRequestAccessRef.value || "").trim();
                    const message = String(els.projectRequestAccessMessage && els.projectRequestAccessMessage.value || "").trim();
                    if (!projectRef) {
                        setNotice("Enter a project ref first.", true);
                        return;
                    }
                    try {
                        await apiClient.createProjectAccessRequest(projectRef, message);
                        await Promise.all([loadMyProjectAccessRequests(), loadProjectHistory(), loadMyNotifications()]);
                        if (els.projectRequestAccessMessage) {
                            els.projectRequestAccessMessage.value = "";
                        }
                        renderProjectHistoryPanel();
                        renderMyProjectAccessRequestsPanel();
                        renderMyNotificationsPanel();
                        setNotice("Access request submitted.");
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                });
            }

            if (els.projectAccessRequestsList) {
                els.projectAccessRequestsList.addEventListener("click", async (event) => {
                    const button = event.target.closest("[data-project-access-request-action]");
                    if (!button) {
                        return;
                    }
                    const project = getCurrentProject();
                    if (!project || !project.id) {
                        return;
                    }
                    const requestID = Number(button.dataset.projectAccessRequestId || 0);
                    const action = String(button.dataset.projectAccessRequestAction || "");
                    const status = action === "reject" ? "rejected" : "approved";
                    const decisionMessage = (await uiPrompt("Optional decision message", "", "Submit")) || "";
                    if (!requestID) {
                        return;
                    }
                    try {
                        await apiClient.setProjectAccessRequestStatus(project.prefix || project.id, requestID, status, decisionMessage);
                        await Promise.all([loadProjectAccessRequests(), loadProjectHistory()]);
                        renderProjectAccessRequestsPanel();
                        renderProjectHistoryPanel();
                        setNotice("Access request " + (status === "approved" ? "approved." : "rejected."));
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                });
            }
            if (els.projectNotificationsList) {
                els.projectNotificationsList.addEventListener("click", async (event) => {
                    const button = event.target.closest("[data-notification-read]");
                    if (!button) {
                        return;
                    }
                    const notificationID = Number(button.dataset.notificationRead || 0);
                    if (!notificationID) {
                        return;
                    }
                    try {
                        await apiClient.markNotificationRead(notificationID);
                        await loadMyNotifications();
                        renderMyNotificationsPanel();
                        setNotice("Notification marked as read.");
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                });
            }
        }

        function buildPlanPayloadFromForm() {
            return {
                slug: (els.planSlug.value || "").trim(),
                name: (els.planName.value || "").trim(),
                description: (els.planDescription.value || "").trim(),
                max_projects: Number(els.planMaxProjects.value || 0),
                max_private_projects: Number(els.planMaxPrivateProjects.value || 0),
                max_tickets: Number(els.planMaxTickets.value || 0),
                max_tickets_per_project: Number(els.planMaxTicketsPerProject.value || 0),
                max_team_memberships: Number(els.planMaxTeamMemberships.value || 0),
                max_api_calls_per_day: Number(els.planMaxAPICallsPerDay.value || 0),
                default_project_alias: els.planDefaultProjectAlias.value || "public",
                registration_actions: {
                    auto_assign_public_team: normalizeBool(els.planAutoAssignPublicTeam.value),
                    auto_create_private_project: normalizeBool(els.planAutoCreatePrivateProject.value),
                    auto_create_private_team: normalizeBool(els.planAutoCreatePrivateTeam.value),
                    teams: Array.isArray(state.selectedPlanDraft && state.selectedPlanDraft.registration_actions && state.selectedPlanDraft.registration_actions.teams)
                        ? state.selectedPlanDraft.registration_actions.teams
                        : [],
                    projects: Array.isArray(state.selectedPlanDraft && state.selectedPlanDraft.registration_actions && state.selectedPlanDraft.registration_actions.projects)
                        ? state.selectedPlanDraft.registration_actions.projects
                        : [],
                },
            };
        }

        function bindPlanHandlers() {
            if (els.planList) {
                els.planList.addEventListener("click", (event) => {
                    const card = event.target.closest("[data-plan-slug]");
                    if (!card) {
                        return;
                    }
                    state.selectedPlanSlug = card.dataset.planSlug || "";
                    state.selectedPlanDraft = getCurrentPlan() ? structuredClone(getCurrentPlan()) : emptyPlan();
                    renderPlans();
                });
            }

            const newPlanButton = document.getElementById("new-plan-button");
            if (newPlanButton) {
                newPlanButton.addEventListener("click", () => {
                    state.selectedPlanSlug = "";
                    state.selectedPlanDraft = emptyPlan();
                    renderPlanEditor();
                    renderPlans();
                });
            }

            if (els.savePlanAdminButton) {
                els.savePlanAdminButton.addEventListener("click", async () => {
                    try {
                        await apiClient.setRegistrationPolicy(
                            normalizeBool(els.registrationEnabledSelect.value),
                            normalizeBool(els.registrationAutoApproveSelect.value),
                        );
                        if (els.defaultPlanSelect.value) {
                            await apiClient.setDefaultPlan(els.defaultPlanSelect.value);
                        }
                        await Promise.all([loadStatus(), loadPlans()]);
                        syncRegistrationUI();
                        renderPlans();
                        setNotice("Onboarding policy saved.");
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                });
            }

            const resetPlanButton = document.getElementById("reset-plan-button");
            if (resetPlanButton) {
                resetPlanButton.addEventListener("click", () => {
                    state.selectedPlanDraft = getCurrentPlan() ? structuredClone(getCurrentPlan()) : emptyPlan();
                    renderPlanEditor();
                });
            }

            const planForm = document.getElementById("plan-form");
            if (planForm) {
                planForm.addEventListener("submit", async (event) => {
                    event.preventDefault();
                    const payload = buildPlanPayloadFromForm();
                    if (!payload.slug || !payload.name) {
                        setNotice("Plan slug and name are required.", true);
                        return;
                    }
                    try {
                        const existing = state.selectedPlanDraft && state.selectedPlanDraft.plan_id ? state.selectedPlanDraft : null;
                        const saved = normalizePlan(existing
                            ? await apiClient.updatePlan(existing.slug, payload)
                            : await apiClient.createPlan(payload));
                        await loadPlans();
                        state.selectedPlanSlug = saved.slug;
                        state.selectedPlanDraft = getCurrentPlan() ? structuredClone(getCurrentPlan()) : saved;
                        renderPlans();
                        setNotice("Plan saved.");
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                });
            }

            const deletePlanButton = document.getElementById("delete-plan-button");
            if (deletePlanButton) {
                deletePlanButton.addEventListener("click", async () => {
                    const plan = state.selectedPlanDraft;
                    if (!plan || !plan.plan_id) {
                        return;
                    }
                    const confirmed = await uiConfirm("Delete plan " + (plan.name ? "\"" + plan.name + "\"" : "\"" + plan.slug + "\"") + "?", "Delete");
                    if (!confirmed) {
                        return;
                    }
                    try {
                        await apiClient.deletePlan(plan.slug);
                        await loadPlans();
                        renderPlans();
                        setNotice("Plan deleted.");
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                });
            }
        }

        function bindAgentModelHandlers() {
            function buildAgentModelPayload(scope) {
                if (scope === "system") {
                    return {
                        provider: String(els.systemAgentProvider ? els.systemAgentProvider.value : "").trim(),
                        model: String(els.systemAgentModel ? els.systemAgentModel.value : "").trim(),
                        url: String(els.systemAgentURL ? els.systemAgentURL.value : "").trim(),
                        api_key: String(els.systemAgentAPIKey ? els.systemAgentAPIKey.value : "").trim(),
                        providers: Array.isArray(state.systemAgentModelConfig.providers) ? state.systemAgentModelConfig.providers : [],
                    };
                }
                return {
                    provider: String(els.projectAgentProvider ? els.projectAgentProvider.value : "").trim(),
                    model: String(els.projectAgentModel ? els.projectAgentModel.value : "").trim(),
                    url: String(els.projectAgentURL ? els.projectAgentURL.value : "").trim(),
                    api_key: String(els.projectAgentAPIKey ? els.projectAgentAPIKey.value : "").trim(),
                };
            }

            if (els.systemAgentProvider) {
                els.systemAgentProvider.addEventListener("change", () => {
                    applyProviderSelectionDefaults("system");
                    renderAgentHarnessEditor();
                });
            }
            if (els.projectAgentProvider) {
                els.projectAgentProvider.addEventListener("change", () => {
                    applyProviderSelectionDefaults("project");
                    renderAgentHarnessEditor();
                });
            }
            if (els.providerConfigSelect) {
                els.providerConfigSelect.addEventListener("change", () => {
                    state.selectedProviderConfigID = String(els.providerConfigSelect.value || "").trim();
                    renderProviderConfigPanel();
                });
            }
            if (els.configSettingsList) {
                els.configSettingsList.addEventListener("click", (event) => {
                    const card = event.target.closest("[data-config-setting-key]");
                    if (!card) {
                        return;
                    }
                    state.selectedConfigSettingKey = String(card.dataset.configSettingKey || "").trim();
                    renderConfigSettingsPanel();
                });
            }
            const newConfigSettingButton = document.getElementById("new-config-setting-button");
            if (newConfigSettingButton) {
                newConfigSettingButton.addEventListener("click", () => {
                    state.selectedConfigSettingKey = "";
                    setConfigSettingEditor(null);
                    if (els.configSettingKey) {
                        els.configSettingKey.focus();
                    }
                });
            }
            if (els.configSettingForm) {
                els.configSettingForm.addEventListener("submit", async (event) => {
                    event.preventDefault();
                    const originalKey = String(state.selectedConfigSettingKey || "").trim();
                    const nextKey = String(els.configSettingKey ? els.configSettingKey.value : "").trim();
                    const nextValue = String(els.configSettingValue ? els.configSettingValue.value : "");
                    if (!nextKey) {
                        setNotice("Configuration key is required.", true);
                        return;
                    }
                    try {
                        const path = originalKey ? ("/api/config/settings/" + encodeURIComponent(originalKey)) : "/api/config/settings";
                        const method = originalKey ? "PUT" : "POST";
                        await api(path, {
                            method,
                            body: JSON.stringify({ key: nextKey, value: nextValue }),
                        });
                        state.selectedConfigSettingKey = nextKey;
                        await loadConfigSettings();
                        renderConfigSettingsPanel();
                        setNotice("Configuration saved.");
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                });
            }
            const deleteConfigSettingButton = document.getElementById("delete-config-setting-button");
            if (deleteConfigSettingButton) {
                deleteConfigSettingButton.addEventListener("click", async () => {
                    const targetKey = String(state.selectedConfigSettingKey || "").trim();
                    if (!targetKey) {
                        return;
                    }
                    const confirmed = await uiConfirm("Delete configuration setting \"" + targetKey + "\"?", "Delete");
                    if (!confirmed) {
                        return;
                    }
                    try {
                        await api("/api/config/settings/" + encodeURIComponent(targetKey), { method: "DELETE" });
                        state.selectedConfigSettingKey = "";
                        await loadConfigSettings();
                        renderConfigSettingsPanel();
                        setNotice("Configuration deleted.");
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                });
            }
            const resetConfigSettingButton = document.getElementById("reset-config-setting-button");
            if (resetConfigSettingButton) {
                resetConfigSettingButton.addEventListener("click", () => {
                    state.selectedConfigSettingKey = "";
                    setConfigSettingEditor(null);
                });
            }
            const orchestratorConfigForm = document.getElementById("orchestrator-config-form");
            if (orchestratorConfigForm) {
                orchestratorConfigForm.addEventListener("submit", async (event) => {
                    event.preventDefault();
                    const idleEl = document.getElementById("orchestrator-refinement-idle");
                    const payload = {
                        interval_seconds: Number.parseInt(document.getElementById("orchestrator-interval").value, 10) || 0,
                        heartbeat_timeout_seconds: Number.parseInt(document.getElementById("orchestrator-heartbeat-timeout").value, 10) || 0,
                        refinement_idle_minutes: idleEl ? (Number.parseInt(idleEl.value, 10) || 0) : 0,
                    };
                    try {
                        await api("/api/config/orchestrator", { method: "POST", body: JSON.stringify(payload) });
                        setNotice("Orchestrator settings saved.");
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                });
            }
            const newProviderConfigButton = document.getElementById("new-provider-config-button");
            if (newProviderConfigButton) {
                newProviderConfigButton.addEventListener("click", () => {
                    state.selectedProviderConfigID = "";
                    if (els.providerConfigID) els.providerConfigID.value = "";
                    if (els.providerConfigLabel) els.providerConfigLabel.value = "";
                    if (els.providerConfigModel) els.providerConfigModel.value = "";
                    if (els.providerConfigURL) els.providerConfigURL.value = "";
                    if (els.providerConfigAuthType) els.providerConfigAuthType.value = "api_key";
                    if (els.providerConfigRequiresURL) els.providerConfigRequiresURL.value = "false";
                    if (els.providerConfigAPIKey) els.providerConfigAPIKey.value = "";
                    if (els.providerConfigModels) els.providerConfigModels.value = "";
                });
            }
            if (els.providerConfigForm) {
                els.providerConfigForm.addEventListener("submit", async (event) => {
                    event.preventDefault();
                    const id = String(els.providerConfigID ? els.providerConfigID.value : "").trim();
                    if (!id) {
                        setNotice("Configuration name is required.", true);
                        return;
                    }
                    const draft = normalizedProviderConfig({
                        id: id,
                        label: String(els.providerConfigLabel ? els.providerConfigLabel.value : "").trim(),
                        default_model: String(els.providerConfigModel ? els.providerConfigModel.value : "").trim(),
                        base_url: String(els.providerConfigURL ? els.providerConfigURL.value : "").trim(),
                        auth_type: String(els.providerConfigAuthType ? els.providerConfigAuthType.value : "api_key").trim() || "api_key",
                        requires_url: String(els.providerConfigRequiresURL ? els.providerConfigRequiresURL.value : "false") === "true",
                        api_key: String(els.providerConfigAPIKey ? els.providerConfigAPIKey.value : "").trim(),
                        models: String(els.providerConfigModels ? els.providerConfigModels.value : "")
                            .split(/\r?\n/)
                            .map((line) => line.trim())
                            .filter(Boolean),
                    });
                    const providers = providerConfigs().filter((provider) => provider.id !== draft.id).concat([draft]);
                    try {
                        await api("/api/config/agent-model", {
                            method: "PUT",
                            body: JSON.stringify({
                                provider: String(els.systemAgentProvider ? els.systemAgentProvider.value : "").trim(),
                                model: "",
                                url: "",
                                api_key: "",
                                providers: providers,
                            }),
                        });
                        state.selectedProviderConfigID = draft.id;
                        await Promise.all([loadSystemAgentModelConfig(), loadProjectAgentModelConfig()]);
                        renderAll();
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                });
            }
            const deleteProviderConfigButton = document.getElementById("delete-provider-config-button");
            if (deleteProviderConfigButton) {
                deleteProviderConfigButton.addEventListener("click", async () => {
                    const targetID = String(state.selectedProviderConfigID || "").trim();
                    if (!targetID) {
                        return;
                    }
                    const confirmed = await uiConfirm("Delete provider configuration \"" + targetID + "\"?", "Delete");
                    if (!confirmed) {
                        return;
                    }
                    const providers = providerConfigs().filter((provider) => provider.id !== targetID);
                    if (!providers.length) {
                        setNotice("At least one configuration is required.", true);
                        return;
                    }
                    try {
                        const systemProvider = String(els.systemAgentProvider ? els.systemAgentProvider.value : "").trim();
                        const nextSystemProvider = systemProvider === targetID ? providers[0].id : systemProvider;
                        await api("/api/config/agent-model", {
                            method: "PUT",
                            body: JSON.stringify({
                                provider: nextSystemProvider,
                                model: "",
                                url: "",
                                api_key: "",
                                providers: providers,
                            }),
                        });
                        if (state.selectedProjectID && String(els.projectAgentProvider ? els.projectAgentProvider.value : "").trim() === targetID) {
                            await api("/api/projects/" + state.selectedProjectID + "/agent-model", {
                                method: "PUT",
                                body: JSON.stringify({ provider: "", model: "", url: "", api_key: "" }),
                            });
                        }
                        state.selectedProviderConfigID = providers[0].id;
                        await Promise.all([loadSystemAgentModelConfig(), loadProjectAgentModelConfig(), loadProjects()]);
                        renderAll();
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                });
            }

            const saveSystemAgentModelButton = document.getElementById("save-system-agent-model");
            if (saveSystemAgentModelButton) {
                saveSystemAgentModelButton.addEventListener("click", async () => {
                    try {
                        await api("/api/config/agent-model", {
                            method: "PUT",
                            body: JSON.stringify(buildAgentModelPayload("system")),
                        });
                        await Promise.all([loadSystemAgentModelConfig()]);
                        renderAll();
                        setNotice("System agent model configuration saved.");
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                });
            }

            const saveProjectAgentModelButton = document.getElementById("save-project-agent-model");
            if (saveProjectAgentModelButton) {
                saveProjectAgentModelButton.addEventListener("click", async () => {
                    if (!state.selectedProjectID) {
                        setNotice("Select a project first.", true);
                        return;
                    }
                    try {
                        await api("/api/projects/" + state.selectedProjectID + "/agent-model", {
                            method: "PUT",
                            body: JSON.stringify(buildAgentModelPayload("project")),
                        });
                        await Promise.all([loadProjects(), loadProjectAgentModelConfig()]);
                        renderAll();
                        setNotice("Project agent model override saved.");
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                });
            }

            const clearProjectAgentModelButton = document.getElementById("clear-project-agent-model");
            if (clearProjectAgentModelButton) {
                clearProjectAgentModelButton.addEventListener("click", async () => {
                    if (!state.selectedProjectID) {
                        return;
                    }
                    try {
                        await api("/api/projects/" + state.selectedProjectID + "/agent-model", {
                            method: "PUT",
                            body: JSON.stringify({ provider: "", model: "", url: "", api_key: "" }),
                        });
                        await Promise.all([loadProjects(), loadProjectAgentModelConfig()]);
                        renderAll();
                        setNotice("Project agent model override cleared.");
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                });
            }

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
                const confirmed = await uiConfirm("Delete document " + (draft.title ? "\"" + draft.title + "\"" : "#" + draft.id) + "?", "Delete");
                if (!confirmed) {
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
                const fileInput = els.documentUploadFile;
                const selectedFile = fileInput && fileInput.files && fileInput.files[0] ? fileInput.files[0] : null;
                try {
                    const uploaded = await uploadFileToCurrentDocument(selectedFile, els.documentUploadName.value || "");
                    if (!uploaded) {
                        return;
                    }
                    fileInput.value = "";
                    els.documentUploadName.value = "";
                    setNotice("File uploaded.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });

            const hasFilesInEvent = (event) => {
                const types = event && event.dataTransfer && event.dataTransfer.types ? Array.from(event.dataTransfer.types) : [];
                return types.includes("Files");
            };

            const withDocumentsDropGuard = (event) => {
                if (state.currentView !== "documents" || !hasFilesInEvent(event)) {
                    return false;
                }
                event.preventDefault();
                return true;
            };

            if (els.documentsView) {
                els.documentsView.addEventListener("dragenter", (event) => {
                    if (!withDocumentsDropGuard(event)) {
                        return;
                    }
                    documentDropDepth += 1;
                    if (!els.documentsView.classList.contains("document-drop-uploading")) {
                        setDocumentDropState("active");
                    }
                });

                els.documentsView.addEventListener("dragover", (event) => {
                    if (!withDocumentsDropGuard(event)) {
                        return;
                    }
                    if (event.dataTransfer) {
                        event.dataTransfer.dropEffect = "copy";
                    }
                });

                els.documentsView.addEventListener("dragleave", (event) => {
                    if (!withDocumentsDropGuard(event)) {
                        return;
                    }
                    documentDropDepth = Math.max(0, documentDropDepth - 1);
                    if (!documentDropDepth && !els.documentsView.classList.contains("document-drop-uploading")) {
                        setDocumentDropState("");
                    }
                });

                els.documentsView.addEventListener("drop", async (event) => {
                    if (!withDocumentsDropGuard(event)) {
                        return;
                    }
                    documentDropDepth = 0;
                    const droppedFiles = event.dataTransfer && event.dataTransfer.files ? Array.from(event.dataTransfer.files) : [];
                    const selectedFile = droppedFiles.find((file) => isTextUploadableFile(file));
                    if (!selectedFile) {
                        clearDocumentDropState();
                        setNotice("Drop a text file to upload.", true);
                        return;
                    }
                    setDocumentDropState("uploading");
                    try {
                        const uploaded = await uploadFileToCurrentDocument(selectedFile, "");
                        if (!uploaded) {
                            clearDocumentDropState();
                            return;
                        }
                        setDocumentDropState("success");
                        if (documentDropSuccessTimer) {
                            clearTimeout(documentDropSuccessTimer);
                        }
                        documentDropSuccessTimer = setTimeout(() => {
                            if (state.currentView === "documents") {
                                setDocumentDropState("");
                            }
                        }, 520);
                        setNotice("File uploaded.");
                    } catch (error) {
                        clearDocumentDropState();
                        setNotice(error.message, true);
                    }
                });
            }

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
                const file = state.documentFiles.find((item) => Number(item.id) === fileID);
                const confirmed = await uiConfirm("Delete file " + ((file && file.file_name) ? "\"" + file.file_name + "\"" : "#" + fileID) + "?", "Delete");
                if (!confirmed) {
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

            if (els.workflowSelect) {
                els.workflowSelect.addEventListener("change", () => {
                    cancelWorkflowAutosave();
                    const nextID = Number(els.workflowSelect.value || 0);
                    state.selectedWorkflowID = nextID || null;
                    state.selectedWorkflowDraft = getCurrentWorkflow() ? structuredClone(getCurrentWorkflow()) : emptyWorkflow();
                    state.workflowGraphNeedsReset = state.workflowStageViewMode === "graph";
                    if (!state.selectedWorkflowID) {
                        renderAll();
                        return;
                    }
                    loadWorkflowValidation(state.selectedWorkflowID).then(renderAll).catch(() => renderAll());
                });
            }

            document.getElementById("new-workflow-button").addEventListener("click", () => {
                cancelWorkflowAutosave();
                state.selectedWorkflowID = null;
                state.selectedWorkflowDraft = emptyWorkflow();
                renderEditors();
                renderWorkflows();
            });

            if (els.workflowViewBoardButton) {
                els.workflowViewBoardButton.addEventListener("click", () => {
                    state.workflowStageViewMode = "board";
                    state.workflowGraphNeedsReset = false;
                    renderWorkflows();
                });
            }
            if (els.workflowViewGraphButton) {
                els.workflowViewGraphButton.addEventListener("click", () => {
                    state.workflowStageViewMode = "graph";
                    state.workflowGraphNeedsReset = true;
                    renderWorkflows();
                });
            }

            document.getElementById("workflow-form").addEventListener("submit", async (event) => {
                event.preventDefault();
                await persistWorkflowSettings();
            });
            document.getElementById("workflow-name").addEventListener("input", scheduleWorkflowAutosave);
            document.getElementById("workflow-description").addEventListener("input", scheduleWorkflowAutosave);
            document.querySelectorAll("input[name=\"workflow-approval-policy\"], input[name=\"workflow-progression-mode\"]").forEach((input) => {
                input.addEventListener("change", scheduleWorkflowAutosave);
            });

            document.getElementById("delete-workflow-button").addEventListener("click", async () => {
                const draft = state.selectedWorkflowDraft;
                if (!draft.id) {
                    return;
                }
                const confirmed = await uiConfirm("Delete workflow " + (draft.name ? "\"" + draft.name + "\"" : "#" + draft.id) + "?", "Delete");
                if (!confirmed) {
                    return;
                }
                try {
                    await api("/api/workflows/" + draft.id, { method: "DELETE" });
                    cancelWorkflowAutosave();
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
                    setNotice("Select a workflow first.", true);
                    return;
                }
                const stageName = document.getElementById("new-stage-name").value.trim();
                if (!stageName) {
                    setNotice("Stage name is required.", true);
                    return;
                }
                if (workflowHasDuplicateStageName(workflow, stageName)) {
                    setNotice("Stage names must be unique within a workflow.", true);
                    return;
                }
                try {
                    await api("/api/workflows/" + workflow.id + "/stages", {
                        method: "POST",
                        body: JSON.stringify({
                            stage_name: stageName,
                            wow: document.getElementById("new-stage-wow").value.trim(),
                            dor: document.getElementById("new-stage-dor").value.trim(),
                            dod: document.getElementById("new-stage-dod").value.trim(),
                            sort_order: Array.isArray(workflow.stages) ? workflow.stages.length : 0,
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
                const renameButton = event.target.closest("[data-rename-stage]");
                if (renameButton) {
                    const stageID = Number(renameButton.dataset.renameStage);
                    const workflow = getCurrentWorkflow();
                    const stage = workflow && Array.isArray(workflow.stages)
                        ? workflow.stages.find((item) => Number(item.id) === stageID)
                        : null;
                    if (!workflow || !stage) {
                        return;
                    }
                    const nextName = await uiPrompt("Rename stage", stage.name || "", "Rename");
                    if (nextName === null) {
                        return;
                    }
                    const trimmedName = String(nextName || "").trim();
                    if (!trimmedName) {
                        setNotice("Stage name is required.", true);
                        return;
                    }
                    if (workflowHasDuplicateStageName(workflow, trimmedName, stageID)) {
                        setNotice("Stage names must be unique within a workflow.", true);
                        return;
                    }
                    if (trimmedName === String(stage.name || "").trim()) {
                        return;
                    }
                    try {
                        await api("/api/workflows/stages/" + stageID, {
                            method: "PUT",
                            body: JSON.stringify({
                                stage_name: trimmedName,
                                wow: stage.wow || stage.description || "",
                                dor: stage.dor || "",
                                dod: stage.dod || "",
                            }),
                        });
                        await loadWorkflows();
                        renderAll();
                        setNotice("Stage renamed.");
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                    return;
                }
                const moveButton = event.target.closest("[data-move-stage]");
                if (moveButton) {
                    const stageID = Number(moveButton.dataset.moveStage);
                    const direction = moveButton.dataset.moveDirection;
                    const workflow = getCurrentWorkflow();
                    if (!workflow || !Array.isArray(workflow.stages) || workflow.stages.length < 2) {
                        return;
                    }
                    const ordered = workflow.stages.map((stage) => Number(stage.id));
                    const currentIndex = ordered.indexOf(stageID);
                    const targetIndex = direction === "left" ? currentIndex - 1 : currentIndex + 1;
                    if (currentIndex < 0 || targetIndex < 0 || targetIndex >= ordered.length) {
                        return;
                    }
                    ordered.splice(currentIndex, 1);
                    ordered.splice(targetIndex, 0, stageID);
                    try {
                        await persistWorkflowStageOrder(workflow.id, ordered);
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                    return;
                }
                const saveButton = event.target.closest("[data-save-stage]");
                if (saveButton) {
                    const stageID = Number(saveButton.dataset.saveStage);
                    await saveStage(stageID);
                    return;
                }
                const deleteButton = event.target.closest("[data-delete-stage]");
                if (deleteButton) {
                    const stageID = Number(deleteButton.dataset.deleteStage);
                    const workflow = getCurrentWorkflow();
                    const stage = workflow && Array.isArray(workflow.stages)
                        ? workflow.stages.find((item) => Number(item.id) === stageID)
                        : null;
                    const confirmed = await uiConfirm("Delete stage " + ((stage && stage.name) ? "\"" + stage.name + "\"" : "#" + stageID) + "?", "Delete");
                    if (!confirmed) {
                        return;
                    }
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
            bindWorkflowGraphPan();
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
                        stage_name: (getCurrentWorkflow() && getCurrentWorkflow().stages.find((stage) => Number(stage.id) === Number(stageID)) || {}).name || "",
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
                const confirmed = await uiConfirm("Delete role " + (draft.title ? "\"" + draft.title + "\"" : "#" + draft.id) + "?", "Delete");
                if (!confirmed) {
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
                const password = await uiPrompt("Optional password for the new agent (leave blank to auto-generate)", "", "Create");
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
                    const roleSelect = document.getElementById("agent-role");
                    const roleValue = Array.from(roleSelect.selectedOptions).map((o) => o.value).join(",");
                    const passwordValue = document.getElementById("agent-new-password").value;
                    const usernameValue = String(document.getElementById("agent-username").value || "").trim();
                    const body = {};
                    if (passwordValue) body.password = passwordValue;
                    body.agent_role = roleValue;
                    if (usernameValue && usernameValue !== agent.username) body.username = usernameValue;
                    await api("/api/agents/" + agent.id, {
                        method: "PUT",
                        body: JSON.stringify(body),
                    });
                    await loadAgents();
                    renderAll();
                    setNotice("Agent updated.");
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
                const confirmed = await uiConfirm("Delete agent " + (agent.id ? "\"" + agent.id + "\"" : "") + "?", "Delete");
                if (!confirmed) {
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

        function bindUsersHandlers() {
            let editingUsername = null;

            function openUserModal(user) {
                editingUsername = user ? user.username : null;
                els.userModalTitle.textContent = user ? "Edit user: " + user.username : "Create user";
                els.userModalUsername.value = user ? user.username : "";
                els.userModalUsername.readOnly = !!user;
                els.userModalEmail.value = user ? (user.email || "") : "";
                els.userModalPassword.value = "";
                els.userModalPassword.placeholder = user ? "leave blank to keep existing" : "leave blank to generate";
                els.userModalRole.value = user ? (user.role || "user") : "user";
                els.userModalStatusRow.classList.toggle("hidden", !user);
                if (user) { els.userModalEnabled.checked = user.enabled !== false; }
                els.userModalSave.textContent = user ? "Save" : "Create user";
                els.userModalResetPw.classList.toggle("hidden", !user);
                els.userModalDelete.classList.toggle("hidden", !user);
                els.userModalError.textContent = "";
                els.userModalGeneratedPw.classList.add("hidden");
                els.userModalGeneratedPw.textContent = "";
                els.userModal.classList.add("open");
                els.userModalUsername.focus();
            }

            function closeUserModal() {
                els.userModal.classList.remove("open");
                editingUsername = null;
            }

            document.getElementById("close-user-modal").addEventListener("click", closeUserModal);
            els.userModal.querySelector(".modal-backdrop").addEventListener("click", closeUserModal);

            els.newUserButton.addEventListener("click", () => openUserModal(null));

            els.userList.addEventListener("click", (event) => {
                const card = event.target.closest("[data-user-id]");
                if (!card) return;
                const user = (state.users || []).find((u) => String(u.id || u.username) === card.dataset.userId);
                if (user) openUserModal(user);
            });

            els.userForm.addEventListener("submit", async (event) => {
                event.preventDefault();
                els.userModalError.textContent = "";
                const username = els.userModalUsername.value.trim();
                const email = els.userModalEmail.value.trim();
                const password = els.userModalPassword.value;
                const role = els.userModalRole.value;
                try {
                    if (!editingUsername) {
                        const result = await apiClient.createUser(username, password, email, role);
                        if (result.password) {
                            els.userModalGeneratedPw.textContent = "Generated password: " + result.password;
                            els.userModalGeneratedPw.classList.remove("hidden");
                        }
                        closeUserModal();
                    } else {
                        const current = (state.users || []).find((u) => u.username === editingUsername);
                        if (current && (current.enabled !== false) !== els.userModalEnabled.checked) {
                            if (els.userModalEnabled.checked) {
                                await apiClient.enableUser(editingUsername);
                            } else {
                                await apiClient.disableUser(editingUsername);
                            }
                        }
                        closeUserModal();
                    }
                    await fetchUsers();
                    setNotice(editingUsername ? "User updated." : "User created.");
                } catch (error) {
                    els.userModalError.textContent = error.message || "An error occurred.";
                }
            });

            els.userModalResetPw.addEventListener("click", async () => {
                if (!editingUsername) return;
                const pw = await openDialog("New password for " + editingUsername + ":", { confirm: true, input: true, okText: "Reset", cancelText: "Cancel" });
                if (pw === null || !pw.trim()) return;
                try {
                    await apiClient.resetUserPassword(editingUsername, pw.trim());
                    setNotice("Password reset.");
                } catch (error) {
                    els.userModalError.textContent = error.message || "Failed to reset password.";
                }
            });

            els.userModalDelete.addEventListener("click", async () => {
                if (!editingUsername) return;
                const confirmed = await uiConfirm("Delete user \"" + editingUsername + "\"? This cannot be undone.", "Delete");
                if (!confirmed) return;
                try {
                    await apiClient.deleteUser(editingUsername);
                    closeUserModal();
                    await fetchUsers();
                    setNotice("User deleted.");
                } catch (error) {
                    els.userModalError.textContent = error.message || "Failed to delete user.";
                }
            });

            if (els.adminSummaryContent) {
                els.adminSummaryContent.addEventListener("click", async (event) => {
                    const userBtn = event.target.closest("[data-admin-toggle-user]");
                    const projBtn = event.target.closest("[data-admin-toggle-project]");
                    if (userBtn) {
                        const username = userBtn.dataset.adminToggleUser;
                        const enabled = userBtn.dataset.enabled === "true";
                        try {
                            if (enabled) { await apiClient.disableUser(username); } else { await apiClient.enableUser(username); }
                            await fetchUsers();
                            renderAdminSummary();
                        } catch (e) { setNotice(e.message, true); }
                    }
                    if (projBtn) {
                        const id = projBtn.dataset.adminToggleProject;
                        const active = projBtn.dataset.active === "true";
                        try {
                            if (active) {
                                await api("/api/projects/" + encodeURIComponent(id) + "/disable", { method: "POST", body: JSON.stringify({}) });
                            } else {
                                await api("/api/projects/" + encodeURIComponent(id) + "/enable", { method: "POST", body: JSON.stringify({}) });
                            }
                            await loadProjects();
                            renderAdminSummary();
                        } catch (e) { setNotice(e.message, true); }
                    }
                });
            }
        }

        function bindOrgHandlers() {
            if (!els.orgForm) return;
            els.orgForm.addEventListener("submit", async (event) => {
                event.preventDefault();
                try {
                    const name = els.orgName ? els.orgName.value.trim() : "";
                    const domain = els.orgDomain ? els.orgDomain.value.trim() : "";
                    const description = els.orgDescription ? els.orgDescription.value.trim() : "";
                    const logoURL = els.orgLogo ? els.orgLogo.value.trim() : "";
                    state.org = await apiClient.updateOrg(name, domain, description, logoURL);
                    setNotice("Organisation saved.");
                } catch (error) {
                    setNotice(error.message, true);
                }
            });
        }

        function bindProgrammeHandlers() {
            if (!els.programmeList) return;

            els.programmeList.addEventListener("click", (event) => {
                const card = event.target.closest("[data-programme-id]");
                if (!card) return;
                state.selectedProgrammeID = Number(card.dataset.programmeId);
                renderProgrammeList();
                renderProgrammeEditor();
            });

            if (document.getElementById("new-programme-button")) {
                document.getElementById("new-programme-button").addEventListener("click", () => {
                    state.selectedProgrammeID = null;
                    renderProgrammeList();
                    renderProgrammeEditor();
                });
            }

            if (els.resetProgrammeButton) {
                els.resetProgrammeButton.addEventListener("click", () => {
                    state.selectedProgrammeID = null;
                    renderProgrammeList();
                    renderProgrammeEditor();
                });
            }

            if (els.programmeForm) {
                els.programmeForm.addEventListener("submit", async (event) => {
                    event.preventDefault();
                    const name = els.programmeName ? els.programmeName.value.trim() : "";
                    const description = els.programmeDescription ? els.programmeDescription.value.trim() : "";
                    try {
                        if (state.selectedProgrammeID) {
                            await apiClient.updateProgramme(state.selectedProgrammeID, name, description);
                        } else {
                            const created = await apiClient.createProgramme(name, description);
                            state.selectedProgrammeID = created.id;
                        }
                        await loadProgrammes();
                        renderProgrammeList();
                        renderProgrammeEditor();
                        setNotice("Programme saved.");
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                });
            }

            if (els.deleteProgrammeButton) {
                els.deleteProgrammeButton.addEventListener("click", async () => {
                    if (!state.selectedProgrammeID) return;
                    const prog = state.programmes.find((p) => p.id === state.selectedProgrammeID);
                    const confirmed = await uiConfirm("Delete programme " + (prog ? "\"" + prog.name + "\"" : "#" + state.selectedProgrammeID) + "?", "Delete");
                    if (!confirmed) return;
                    try {
                        await apiClient.deleteProgramme(state.selectedProgrammeID);
                        state.selectedProgrammeID = null;
                        await loadProgrammes();
                        await loadProjects();
                        renderProgrammeList();
                        renderProgrammeEditor();
                        setNotice("Programme deleted.");
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                });
            }

            if (els.programmeProjectsList) {
                els.programmeProjectsList.addEventListener("click", async (event) => {
                    const btn = event.target.closest("[data-programme-project-id]");
                    if (!btn) return;
                    const projectID = Number(btn.dataset.programmeProjectId);
                    const inProgramme = btn.dataset.programmeProjectIn === "true";
                    const newProgrammeID = inProgramme ? null : state.selectedProgrammeID;
                    try {
                        await apiClient.setProjectProgramme(projectID, newProgrammeID);
                        await loadProjects();
                        renderProgrammeProjects();
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                });
            }
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
                if (state.selectedTeamID) {
                    fetchTeamMembers(state.selectedTeamID);
                }
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
                const confirmed = await uiConfirm("Delete team " + (draft.name ? "\"" + draft.name + "\"" : "#" + draft.id) + "?", "Delete");
                if (!confirmed) {
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

            if (els.teamInviteButton) {
                els.teamInviteButton.addEventListener("click", async () => {
                    const userID = els.teamInviteUserSelect && els.teamInviteUserSelect.value;
                    const role = (els.teamInviteRole && els.teamInviteRole.value) || "member";
                    if (!userID || !state.selectedTeamID) return;
                    try {
                        await apiClient.addTeamMember(state.selectedTeamID, userID, role);
                        await fetchTeamMembers(state.selectedTeamID);
                    } catch (e) {
                        setNotice(e.message, true);
                    }
                });
            }

            if (els.teamMemberList) {
                els.teamMemberList.addEventListener("click", async (e) => {
                    const btn = e.target.closest("[data-remove-team-member]");
                    if (!btn || !state.selectedTeamID) return;
                    const userID = btn.dataset.removeTeamMember;
                    try {
                        await apiClient.removeTeamMember(state.selectedTeamID, userID);
                        await fetchTeamMembers(state.selectedTeamID);
                    } catch (e) {
                        setNotice(e.message, true);
                    }
                });
            }
        }

        function bindTicketsHandlers() {
            // "New…" dropdown: create a ticket or a release from one menu.
            const newMenuButton = document.getElementById("new-menu-button");
            const newMenuDropdown = document.getElementById("new-menu-dropdown");
            function closeNewMenu() {
                if (newMenuDropdown) newMenuDropdown.classList.remove("open");
                if (newMenuButton) newMenuButton.setAttribute("aria-expanded", "false");
            }
            async function createReleaseFromMenu() {
                if (!state.selectedProjectID) return;
                try {
                    await apiClient.createRelease(state.selectedProjectID, { title: "New release" });
                    await loadReleases();
                    renderReleaseSelect();
                    renderTicketBoard();
                    renderTicketListView();
                    renderTicketPlanView();
                } catch (e) {
                    setNotice(e.message, true);
                }
            }
            if (newMenuButton && newMenuDropdown) {
                newMenuButton.addEventListener("click", (event) => {
                    event.stopPropagation();
                    const isOpen = newMenuDropdown.classList.toggle("open");
                    newMenuButton.setAttribute("aria-expanded", isOpen ? "true" : "false");
                });
                newMenuDropdown.addEventListener("click", (event) => {
                    const item = event.target.closest("[data-new-action]");
                    if (!item) return;
                    closeNewMenu();
                    if (item.dataset.newAction === "ticket") {
                        openTicketModal(emptyTicket());
                    } else if (item.dataset.newAction === "release") {
                        createReleaseFromMenu();
                    }
                });
                document.addEventListener("click", (event) => {
                    if (!event.target.closest("#new-menu-button") && !event.target.closest("#new-menu-dropdown")) {
                        closeNewMenu();
                    }
                });
            }
            if (els.boardSearch) {
                els.boardSearch.addEventListener("input", () => { renderTicketBoard(); renderTicketListView(); renderTicketPlanView(); });
            }
            if (els.boardHideDone) {
                els.boardHideDone.addEventListener("change", () => { renderTicketBoard(); renderTicketListView(); renderTicketPlanView(); });
            }
            if (els.boardReleaseSelect) {
                els.boardReleaseSelect.addEventListener("change", () => {
                    state.selectedReleaseID = els.boardReleaseSelect.value;
                    if (state.selectedProjectID) {
                        localStorage.setItem("site2.release." + state.selectedProjectID, state.selectedReleaseID);
                    }
                    renderTicketBoard();
                    renderTicketListView();
                    renderTicketPlanView();
                });
            }
            // Board perspective toggle buttons
            document.querySelectorAll("[data-perspective]").forEach((btn) => {
                btn.addEventListener("click", () => {
                    state.boardPerspective = btn.dataset.perspective;
                    localStorage.setItem("site2.board-view", state.boardPerspective);
                    renderTicketBoard();
                    renderTicketListView();
                    renderTicketPlanView();
                });
            });
            // Ticket list view row click
            if (els.ticketListView) {
                // Persist expand/collapse of release/feature/epic groups. `toggle`
                // doesn't bubble, so listen in the capture phase.
                els.ticketListView.addEventListener("toggle", (e) => {
                    const d = e.target;
                    if (d && d.tagName === "DETAILS" && d.dataset && d.dataset.ck) {
                        setListGroupOpen(d.dataset.ck, d.open);
                    }
                }, true);
                els.ticketListView.addEventListener("click", (e) => {
                    const row = e.target.closest(".ticket-list-row");
                    if (!row) {
                        return;
                    }
                    const ticketID = row.dataset.ticketId;
                    if (!ticketID) {
                        return;
                    }
                    const ticket = state.tickets.find((t) => t.id === ticketID);
                    if (!ticket) {
                        return;
                    }
                    // Clicking the priority/type cell edits it inline instead of
                    // opening the ticket.
                    const editCell = e.target.closest("[data-edit-field]");
                    if (editCell) {
                        e.stopPropagation();
                        openFieldPopup(editCell, ticket, editCell.dataset.editField);
                        return;
                    }
                    openTicketModal(ticket);
                });
            }
            document.getElementById("close-ticket-modal").addEventListener("click", closeTicketModal);
            els.ticketModal.addEventListener("click", (event) => {
                if (event.target === els.ticketModal) {
                    closeTicketModal();
                }
            });
            const markReadyBtn = document.getElementById("ticket-mark-ready-btn");
            if (markReadyBtn) {
                markReadyBtn.addEventListener("click", async () => {
                    const ticket = state.activeTicket;
                    if (!ticket || !ticket.id) return;
                    markReadyBtn.disabled = true;
                    try {
                        const updated = await apiClient.post("/api/tickets-action/mark-ready/" + ticket.id, {});
                        await Promise.all([loadTickets(), loadProjectAgents()]);
                        renderAll();
                        openTicketModal(updated);
                    } catch (e) {
                        setNotice(e.message || "Failed to mark ready", true);
                    } finally {
                        markReadyBtn.disabled = false;
                    }
                });
            }

            const refinementSendBtn = document.getElementById("refinement-send");
            const refinementInput = document.getElementById("refinement-input");
            if (refinementSendBtn && refinementInput) {
                const sendRefinementReply = async () => {
                    const ticket = state.activeTicket;
                    if (!ticket || !ticket.id) return;
                    const comment = String(refinementInput.value || "").trim();
                    if (!comment) {
                        setNotice("Reply is required.", true);
                        return;
                    }
                    // Prefer the streaming WebSocket; render the human bubble
                    // optimistically and let the server stream the reply back.
                    if (sendRefinementMessage(ticket.id, comment)) {
                        refinementInput.value = "";
                        return;
                    }
                    // Fall back to the existing REST POST + reload.
                    refinementSendBtn.disabled = true;
                    try {
                        await api("/api/tickets/" + ticket.id + "/comments", {
                            method: "POST",
                            body: JSON.stringify({ comment: comment }),
                        });
                        refinementInput.value = "";
                        await loadRefinementThread(ticket.id);
                    } catch (e) {
                        setNotice(e.message || "Failed to send reply", true);
                    } finally {
                        refinementSendBtn.disabled = false;
                    }
                };
                refinementSendBtn.addEventListener("click", () => {
                    sendRefinementReply().catch((e) => setNotice(e.message || "Failed to send reply", true));
                });
                refinementInput.addEventListener("keydown", (event) => {
                    // Enter sends; Shift+Enter inserts a newline.
                    if (event.key === "Enter" && !event.shiftKey) {
                        event.preventDefault();
                        sendRefinementReply().catch((e) => setNotice(e.message || "Failed to send reply", true));
                    }
                });
            }

            const refinementApproveBtn = document.getElementById("refinement-approve");
            if (refinementApproveBtn) {
                refinementApproveBtn.addEventListener("click", async () => {
                    const ticket = state.activeTicket;
                    if (!ticket || !ticket.id) return;
                    refinementApproveBtn.disabled = true;
                    try {
                        await api("/api/tickets/" + ticket.id + "/refinement/approve", {
                            method: "POST",
                            body: JSON.stringify({}),
                        });
                        await Promise.all([loadTickets(), loadProjectAgents()]);
                        renderAll();
                        closeTicketModal();
                        setNotice("Refinement approved.");
                    } catch (e) {
                        setNotice(e.message || "Failed to approve refinement", true);
                    } finally {
                        refinementApproveBtn.disabled = false;
                    }
                });
            }

            const refinementBreakdownEl = document.getElementById("refinement-breakdown");
            if (refinementBreakdownEl) {
                refinementBreakdownEl.addEventListener("click", (event) => {
                    const button = event.target.closest("[data-reorder]");
                    if (!button || button.disabled) return;
                    const ticket = state.activeTicket;
                    if (!ticket || !ticket.id) return;
                    reorderBreakdownChild(ticket, button.dataset.child, button.dataset.reorder)
                        .catch((e) => setNotice(e.message || "Failed to reorder breakdown", true));
                });
            }

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
                    state.projectAgentModelConfig = emptyAgentModelConfig();
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
            const onTicketContextMenu = (event) => {
                const card = event.target.closest("[data-ticket-id]");
                if (!card) {
                    return;
                }
                const ticket = state.tickets.find((item) => String(item.id) === card.dataset.ticketId);
                if (!ticket) {
                    return;
                }
                event.preventDefault();
                openBoardContextMenu(event, ticket);
            };
            els.ticketBoard.addEventListener("contextmenu", onTicketContextMenu);
            if (els.ticketListView) {
                els.ticketListView.addEventListener("contextmenu", onTicketContextMenu);
            }
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
                const confirmed = await uiConfirm("Delete ticket " + (state.activeTicket.title ? "\"" + state.activeTicket.title + "\"" : "#" + state.activeTicket.id) + "?", "Delete");
                if (!confirmed) {
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
            if (els.addTicketContextButton) {
                els.addTicketContextButton.addEventListener("click", () => {
                    addTicketContext();
                });
            }
            const ticketContextGraphButton = document.getElementById("ticket-context-graph-button");
            if (ticketContextGraphButton) {
                ticketContextGraphButton.addEventListener("click", () => {
                    const ticket = state.activeTicket;
                    if (!ticket || !ticket.id) return;
                    closeTicketModal();
                    focusTicketInContextGraph(ticket.id);
                });
            }
            const contextGraphEl = document.getElementById("context-graph");
            if (contextGraphEl) {
                contextGraphEl.addEventListener("click", (event) => {
                    const node = event.target.closest("[data-node-key]");
                    if (!node) return;
                    openContextNode(node.dataset.nodeType, node.dataset.nodeId);
                });
            }
            const contextClearFocusButton = document.getElementById("context-clear-focus");
            if (contextClearFocusButton) {
                contextClearFocusButton.addEventListener("click", () => {
                    state.contextFocusKey = null;
                    renderContextView();
                });
            }
            const contextSearchInput = document.getElementById("context-search-input");
            if (contextSearchInput) {
                let contextSearchTimer = null;
                contextSearchInput.addEventListener("input", () => {
                    clearTimeout(contextSearchTimer);
                    contextSearchTimer = setTimeout(() => {
                        runContextSearch(contextSearchInput.value)
                            .catch((e) => setNotice(e.message || "Context search failed", true));
                    }, 250);
                });
            }
            if (els.ticketContext) {
                els.ticketContext.addEventListener("click", (event) => {
                    const button = event.target.closest("[data-remove-ticket-context]");
                    if (!button) {
                        return;
                    }
                    removeTicketContext(button.dataset.removeTicketContext);
                });
            }
            els.addTicketTimeButton.addEventListener("click", () => {
                addTicketTimeEntry();
            });

            bindTicketBoardDragAndDrop();
            bindPlanViewHandlers();
            bindAgentBarHandlers();
        }

        function bindAgentBarHandlers() {
            const bar = els.projectAgentBar;
            if (!bar) return;
            bar.addEventListener("click", (event) => {
                const icon = event.target.closest(".agent-icon");
                if (!icon) return;
                const isOpen = icon.classList.contains("agent-popup-open");
                // Close any open popups
                bar.querySelectorAll(".agent-popup-open").forEach((el) => el.classList.remove("agent-popup-open"));
                if (!isOpen) {
                    icon.classList.add("agent-popup-open");
                }
                event.stopPropagation();
            });
            // Close popup when clicking outside
            document.addEventListener("click", () => {
                if (bar) bar.querySelectorAll(".agent-popup-open").forEach((el) => el.classList.remove("agent-popup-open"));
            });
        }

        function bindPlanViewHandlers() {
            if (!els.ticketPlanView) {
                return;
            }
            els.ticketPlanView.addEventListener("dragstart", (event) => {
                const row = event.target.closest("[data-ticket-id]");
                if (!row) {
                    return;
                }
                row.classList.add("dragging");
                event.dataTransfer.effectAllowed = "move";
                event.dataTransfer.setData("text/plain", row.dataset.ticketId);
            });
            els.ticketPlanView.addEventListener("dragend", () => {
                els.ticketPlanView.querySelectorAll(".dragging").forEach((el) => el.classList.remove("dragging"));
            });
            els.ticketPlanView.addEventListener("dragover", (event) => {
                const target = event.target.closest("[data-release-id]");
                if (!target || target.dataset.releaseLocked) {
                    return;
                }
                event.preventDefault();
                els.ticketPlanView.querySelectorAll(".plan-drop-target").forEach((el) => el.classList.remove("plan-drop-target"));
                target.classList.add("plan-drop-target");
            });
            els.ticketPlanView.addEventListener("dragleave", (event) => {
                if (!els.ticketPlanView.contains(event.relatedTarget)) {
                    els.ticketPlanView.querySelectorAll(".plan-drop-target").forEach((el) => el.classList.remove("plan-drop-target"));
                }
            });
            els.ticketPlanView.addEventListener("drop", async (event) => {
                event.preventDefault();
                els.ticketPlanView.querySelectorAll(".plan-drop-target").forEach((el) => el.classList.remove("plan-drop-target"));
                const ticketId = event.dataTransfer.getData("text/plain");
                if (!ticketId) {
                    return;
                }
                const target = event.target.closest("[data-release-id]");
                if (!target || target.dataset.releaseLocked) {
                    return;
                }
                const releaseId = target.dataset.releaseId;
                try {
                    await apiClient.setTicketRelease(ticketId, releaseId ? parseInt(releaseId, 10) : null);
                    await loadTickets();
                    renderTicketBoard();
                    renderTicketListView();
                    renderTicketPlanView();
                } catch (e) {
                    setNotice(e.message, true);
                }
            });
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
            // Populate release select. Only features carry a release, so the field is
            // shown for feature tickets and hidden otherwise. The options are the
            // in_design releases (plus the current release if it is no longer in_design,
            // so it still renders correctly).
            const ticketReleaseEl = document.getElementById("ticket-release");
            const ticketReleaseField = ticketReleaseEl ? ticketReleaseEl.closest(".field") : null;
            if (ticketReleaseEl) {
                const isFeature = ticket.type === "feature";
                if (ticketReleaseField) {
                    ticketReleaseField.classList.toggle("hidden", !isFeature);
                }
                if (isFeature) {
                    const releaseList = (state.releases || []).filter((r) =>
                        r.status === "in_design" || r.id === ticket.release_id);
                    const releaseOptions = ["<option value=\"\">None</option>"].concat(
                        releaseList.map((r) => {
                            const label = (r.title || "Release") + " (" + (r.status || "in_design") + ")";
                            const selected = ticket.release_id === r.id ? " selected" : "";
                            return "<option value=\"" + r.id + "\"" + selected + ">" + escapeHTML(label) + "</option>";
                        })
                    );
                    ticketReleaseEl.innerHTML = releaseOptions.join("");
                    if (!ticket.release_id) {
                        ticketReleaseEl.value = "";
                    }
                }
            }
            document.getElementById("ticket-draft").value = String(Boolean(ticket.draft));
            document.getElementById("ticket-priority").value = ticket.priority || 0;
            document.getElementById("ticket-order").value = ticket.order || 0;
            document.getElementById("ticket-estimate-effort").value = ticket.estimate_effort || 0;
            document.getElementById("ticket-health").value = ticket.health || 0;
            document.getElementById("delete-ticket-button").disabled = !ticket.id;
            els.ticketCommentInput.value = "";
            // Recommend-ready banner
            const rrBanner = document.getElementById("ticket-recommend-ready-banner");
            if (rrBanner) {
                if (ticket.recommended_ready && ticket.draft) {
                    rrBanner.classList.remove("hidden");
                } else {
                    rrBanner.classList.add("hidden");
                }
            }
            // In-progress badge: show how long an active ticket has been worked (TK-90).
            const inProgressEl = document.getElementById("ticket-in-progress");
            if (inProgressEl) {
                if (ticket.state === "active") {
                    let label = "In progress";
                    if (ticket.started_at) {
                        const elapsed = humanizeSince(ticket.started_at);
                        label += " · active since " + ticket.started_at + (elapsed ? " (" + elapsed + ")" : "");
                    }
                    inProgressEl.textContent = label;
                    inProgressEl.classList.remove("hidden");
                } else {
                    inProgressEl.classList.add("hidden");
                }
            }
            // PR URL display
            const prUrlEl = document.getElementById("ticket-pr-url");
            if (prUrlEl) {
                if (ticket.pr_url) {
                    prUrlEl.innerHTML = "PR: <a href=\"" + escapeHTML(ticket.pr_url) + "\" target=\"_blank\" rel=\"noopener\">" + escapeHTML(ticket.pr_url) + "</a>";
                    prUrlEl.classList.remove("hidden");
                } else {
                    prUrlEl.classList.add("hidden");
                }
            }
            els.ticketModal.classList.add("open");
            loadTicketPullRequests(ticket.id).catch((error) => {
                if (els.ticketPullRequests) els.ticketPullRequests.innerHTML = "<div class=\"empty\">" + escapeHTML(error.message) + "</div>";
            });
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
            loadTicketContext(ticket.id).catch((error) => {
                if (els.ticketContext) els.ticketContext.innerHTML = "<div class=\"empty\">" + escapeHTML(error.message) + "</div>";
            });
            loadTicketTime(ticket.id).catch((error) => {
                els.ticketTimeEntries.innerHTML = "<div class=\"empty\">" + escapeHTML(error.message) + "</div>";
            });
            renderRefinementPanel(state.activeTicket);
            loadTicketPrompt(ticket.id);
            // Always open on the Details panel first; the Refinement tab is one click
            // away for stories in refinement.
            switchTicketTab("details");
        }

        // loadTicketPullRequests fetches and renders a ticket's pull requests
        // (repo, branches, status, provider, url). The section hides when empty.
        async function loadTicketPullRequests(ticketID) {
            const section = document.getElementById("ticket-pull-requests-section");
            const list = els.ticketPullRequests;
            if (!list) return;
            if (!ticketID) {
                if (section) section.classList.add("hidden");
                list.innerHTML = "";
                return;
            }
            const prs = await api("/api/tickets/" + ticketID + "/pull-requests");
            if (!Array.isArray(prs) || !prs.length) {
                if (section) section.classList.add("hidden");
                list.innerHTML = "";
                return;
            }
            if (section) section.classList.remove("hidden");
            list.innerHTML = prs.map((pr) => {
                const branches = escapeHTML((pr.source_branch || "?") + " → " + (pr.target_branch || "?"));
                const meta = escapeHTML("#" + pr.id + " · " + (pr.status || "open") + " · " + (pr.provider || "none"));
                const repo = pr.repository ? "<div class=\"meta\">" + escapeHTML(pr.repository) + "</div>" : "";
                const link = pr.url
                    ? "<a href=\"" + escapeHTML(pr.url) + "\" target=\"_blank\" rel=\"noopener\">" + escapeHTML(pr.url) + "</a>"
                    : "";
                return "<div class=\"history-item\"><div>" + meta + " — " + branches + "</div>" + repo + (link ? "<div>" + link + "</div>" : "") + "</div>";
            }).join("");
        }

        // loadTicketPrompt fetches and renders the agent prompt preview for a ticket.
        async function loadTicketPrompt(ticketID) {
            const pre = document.getElementById("ticket-prompt-preview");
            if (!pre) return;
            if (!ticketID) { pre.textContent = "Save the ticket to preview its agent prompt."; return; }
            pre.textContent = "Loading…";
            try {
                const resp = await api("/api/tickets/" + ticketID + "/prompt");
                pre.textContent = (resp && resp.prompt) ? resp.prompt : "(empty)";
            } catch (error) {
                pre.textContent = "Failed to load prompt: " + (error.message || "error");
            }
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
            disconnectRefinementSocket();
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
                const rawType = item.event_type || item.action || item.type || "event";
                const label = humanizeHistoryEventType(rawType);
                const payload = parseHistoryPayload(item.payload);
                const detail = formatHistoryPayloadSummary(payload);
                return "<div class=\"history-item\"><strong>" + escapeHTML(label) + "</strong>" +
                    (detail ? "<div class=\"meta detail\">" + escapeHTML(detail) + "</div>" : "") +
                    "<div class=\"meta\">" + escapeHTML(item.created_by || "") + (item.created_by && item.created_at ? " · " : "") +
                    escapeHTML(item.created_at || item.timestamp || "") + "</div></div>";
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

        function agentUsernameSet() {
            const names = new Set();
            (state.projectAgents || []).map((s) => s.agent || s).forEach((a) => {
                if (a && a.username) names.add(String(a.username).toLowerCase());
            });
            return names;
        }

        function renderRefinementThread(comments, thinking) {
            const thread = document.getElementById("refinement-thread");
            if (!thread) return;
            const agents = agentUsernameSet();
            let html = "";
            if (Array.isArray(comments) && comments.length) {
                // ListComments returns newest-first; the refinement chat reads oldest
                // at the top to newest at the bottom (like a chat transcript).
                html = comments.slice().reverse().map((item) => {
                    const author = item.author || "user";
                    const isAgent = agents.has(String(author).toLowerCase()) || /refin/i.test(author);
                    const side = isAgent ? "agent" : "human";
                    return "<div class=\"refinement-bubble refinement-bubble-" + side + "\">" +
                        "<div class=\"refinement-author\">" + escapeHTML(author) + "</div>" +
                        "<div class=\"refinement-text\">" + escapeHTML(item.text || item.comment || "") + "</div>" +
                        (item.date ? "<div class=\"refinement-date\">" + escapeHTML(item.date) + "</div>" : "") +
                        "</div>";
                }).join("");
            } else if (!thinking) {
                html = "<div class=\"empty\">No refinement dialogue yet.</div>";
            }
            if (thinking) {
                html += "<div class=\"refinement-bubble refinement-bubble-agent refinement-thinking\">" +
                    "<div class=\"refinement-author\">refiner</div>" +
                    "<div class=\"refinement-text\"><span class=\"refinement-typing\"><span></span><span></span><span></span></span> thinking…</div>" +
                    "</div>";
            }
            thread.innerHTML = html;
            thread.scrollTop = thread.scrollHeight;
        }

        // refinementIsThinking reports whether the refiner agent is actively working
        // this refinement ticket (assigned + active) — drives the "thinking…" indicator.
        function refinementIsThinking(ticket) {
            return Boolean(ticket && ticketInRefinement(ticket) && ticket.state === "active" &&
                String(ticket.assignee || "").trim() !== "");
        }

        async function loadRefinementThread(ticketID, thinking) {
            if (!ticketID) return;
            const comments = await api("/api/tickets/" + ticketID + "/comments");
            renderRefinementThread(comments, thinking);
        }

        // setRefinementStatus drives the always-on status line at the top of the
        // refinement panel so there is a clear cue about what (if anything) is
        // happening: connecting, refiner working, awaiting your reply, idle, errors.
        function setRefinementStatus(text, busy) {
            const el = document.getElementById("refinement-status");
            if (!el) return;
            if (!text) {
                el.classList.add("hidden");
                el.innerHTML = "";
                return;
            }
            el.classList.remove("hidden", "refinement-status-warn");
            el.classList.toggle("refinement-status-busy", Boolean(busy));
            const dot = busy ? "<span class=\"refining-pulse\"></span> " : "";
            el.innerHTML = dot + escapeHTML(text);
        }

        // showRefinementNoLLM surfaces a clear, persistent warning (status line + a
        // note in the thread) when the server has no refiner LLM wired up, so the
        // human knows their message was saved but won't get an AI reply — and how to
        // fix it — instead of staring at a "thinking" animation that never resolves.
        function showRefinementNoLLM(advice, command) {
            const el = document.getElementById("refinement-status");
            const text = advice ||
                "No refiner LLM is configured on the server, so this message won't get an automated reply.";
            if (el) {
                el.classList.remove("hidden", "refinement-status-busy");
                el.classList.add("refinement-status-warn");
                el.innerHTML = "<span class=\"refinement-warn-icon\">⚠</span> " + escapeHTML(text);
            }
            const thread = refinementThreadEl();
            if (thread && !thread.querySelector(".refinement-no-llm-note")) {
                const note = document.createElement("div");
                note.className = "refinement-no-llm-note";
                note.innerHTML = "<strong>⚠ No AI refiner is running.</strong> " + escapeHTML(text) +
                    (command ? "<div class=\"meta\">Command tried: <code>" + escapeHTML(command) + "</code></div>" : "");
                thread.appendChild(note);
                refinementScrollToBottom();
            }
            setNotice(text, true);
        }

        // refinementBreakdownChildren returns the proposed breakdown stories for an
        // idea in their user-set priority order (sort_order, id as tie-break).
        function refinementBreakdownChildren(ticket) {
            return (state.tickets || [])
                .filter((t) => t.parent_id === ticket.id)
                .sort((a, b) => (a.order || 0) - (b.order || 0) ||
                    String(a.id).localeCompare(String(b.id)));
        }

        // reorderBreakdownChild moves one proposed story up or down in the breakdown
        // and persists the full order so the human can reprioritize before sign-off.
        async function reorderBreakdownChild(ticket, childID, direction) {
            const ids = refinementBreakdownChildren(ticket).map((c) => c.id);
            const index = ids.indexOf(childID);
            const swapWith = direction === "up" ? index - 1 : index + 1;
            if (index < 0 || swapWith < 0 || swapWith >= ids.length) return;
            const tmp = ids[index];
            ids[index] = ids[swapWith];
            ids[swapWith] = tmp;
            const updated = await api("/api/tickets/" + ticket.id + "/children/reorder", {
                method: "POST",
                body: JSON.stringify({ order: ids }),
            });
            (updated || []).map(normalizeTicket).forEach((fresh) => {
                const existing = (state.tickets || []).find((t) => t.id === fresh.id);
                if (existing) existing.order = fresh.order;
            });
            renderRefinementPanel(ticket);
        }

        function renderRefinementPanel(ticket) {
            const panel = document.getElementById("refinement-panel");
            if (!panel) return;
            if (!ticketInRefinement(ticket)) {
                disconnectRefinementSocket();
                setRefinementStatus("This story isn't being refined. Right-click it on the board → Refine this story, or it enters refinement automatically as a backlog draft.", false);
                const approveBox = document.getElementById("refinement-approve-box");
                if (approveBox) approveBox.classList.add("hidden");
                const compose = panel.querySelector(".refinement-compose");
                if (compose) compose.classList.add("hidden");
                const thread = document.getElementById("refinement-thread");
                if (thread) thread.innerHTML = "";
                return;
            }
            const compose = panel.querySelector(".refinement-compose");
            if (compose) compose.classList.remove("hidden");

            // Open (or keep) a streaming WebSocket for this refine ticket.
            connectRefinementSocket(ticket.id);

            const approveBox = document.getElementById("refinement-approve-box");
            const breakdown = document.getElementById("refinement-breakdown");
            if (approveBox && breakdown) {
                if (ticket.recommended_ready) {
                    approveBox.classList.remove("hidden");
                    const children = refinementBreakdownChildren(ticket);
                    if (children.length) {
                        breakdown.innerHTML = "<div class=\"refinement-subheading\">Proposed breakdown — reorder before approving</div>" +
                            children.map((c, i) =>
                                "<div class=\"refinement-child\" data-child-id=\"" + escapeHTML(c.id) + "\">" +
                                "<span class=\"refinement-child-reorder\">" +
                                "<button type=\"button\" class=\"refinement-reorder-btn\" data-reorder=\"up\" data-child=\"" + escapeHTML(c.id) + "\"" + (i === 0 ? " disabled" : "") + " title=\"Move up\" aria-label=\"Move up\">&#9650;</button>" +
                                "<button type=\"button\" class=\"refinement-reorder-btn\" data-reorder=\"down\" data-child=\"" + escapeHTML(c.id) + "\"" + (i === children.length - 1 ? " disabled" : "") + " title=\"Move down\" aria-label=\"Move down\">&#9660;</button>" +
                                "</span>" +
                                "<span class=\"refinement-child-body\"><strong>" + (i + 1) + ". " + escapeHTML(c.title || "(untitled)") + "</strong>" +
                                (c.description ? "<div class=\"meta\">" + escapeHTML(c.description) + "</div>" : "") +
                                "</span></div>"
                            ).join("");
                    } else {
                        breakdown.innerHTML = "";
                    }
                } else {
                    approveBox.classList.add("hidden");
                    breakdown.innerHTML = "";
                }
            }

            // A refiner reply is in flight for this ticket (we sent a turn, or the WS
            // is streaming/thinking). Don't reset the status to "Your turn" or reload
            // the thread — that would stomp the thinking animation and wipe the live
            // streaming bubble. The WS events drive the UI until message_done.
            const awaitingRefiner = String(state.refinementBusyTicketId || "") === String(ticket.id);
            const thinking = refinementIsThinking(ticket) || awaitingRefiner;

            if (thinking) {
                setRefinementStatus("Refiner is thinking…", true);
            } else if (ticket.recommended_ready) {
                setRefinementStatus("Refiner proposed a refinement — review & approve", false);
            } else {
                setRefinementStatus("Your turn — send a message to refine this story", false);
            }

            if (awaitingRefiner) {
                // Live stream owns the thread right now; leave it alone.
                return;
            }
            loadRefinementThread(ticket.id, thinking).catch((error) => {
                const thread = document.getElementById("refinement-thread");
                if (thread) thread.innerHTML = "<div class=\"empty\">" + escapeHTML(error.message) + "</div>";
            });
        }

        // refreshOpenRefinement re-syncs the open ticket modal's refinement panel from
        // a live WebSocket event so the dialogue updates in near real time.
        async function refreshOpenRefinement(ticketID) {
            if (!state.activeTicket || String(state.activeTicket.id) !== String(ticketID)) return;
            try {
                const fresh = normalizeTicket(await api("/api/tickets/" + ticketID));
                state.activeTicket = Object.assign({}, state.activeTicket, fresh);
                renderRefinementPanel(state.activeTicket);
            } catch (error) {
                // Best-effort live refresh; ignore transient errors.
            }
        }

        // disconnectRefinementSocket closes any open refinement WebSocket and clears
        // the associated streaming state.
        function disconnectRefinementSocket() {
            const socket = state.refinementSocket;
            state.refinementSocket = null;
            state.refinementTicketId = null;
            state.refinementPendingSend = null;
            state.refinementLastHumanText = null;
            state.refinementBusyTicketId = null;
            if (socket) {
                try {
                    socket.onclose = null;
                    socket.close();
                } catch (_) { /* ignore */ }
            }
        }

        // connectRefinementSocket opens a streaming WebSocket for the given refine
        // ticket. Any previously open refinement socket is closed first. Failures are
        // swallowed so the UI silently falls back to REST + polling.
        function connectRefinementSocket(ticketId) {
            if (!ticketId) return;
            if (window.__site2MockFetch || typeof WebSocket === "undefined") return;
            if (state.refinementSocket && String(state.refinementTicketId) === String(ticketId) &&
                (state.refinementSocket.readyState === WebSocket.OPEN ||
                 state.refinementSocket.readyState === WebSocket.CONNECTING)) {
                return;
            }
            disconnectRefinementSocket();

            let socket;
            try {
                const scheme = window.location.protocol === "https:" ? "wss:" : "ws:";
                socket = new WebSocket(scheme + "//" + window.location.host +
                    "/api/refinement/ws?ticket=" + encodeURIComponent(ticketId));
            } catch (_) {
                return;
            }
            state.refinementSocket = socket;
            state.refinementTicketId = ticketId;
            setRefinementStatus("Connecting to the refiner…", true);

            socket.addEventListener("open", () => {
                if (state.refinementSocket !== socket) return;
                setRefinementStatus("Connected — your turn", false);
                if (state.refinementPendingSend != null) {
                    const text = state.refinementPendingSend;
                    state.refinementPendingSend = null;
                    try {
                        socket.send(JSON.stringify({ type: "message", text: text }));
                    } catch (_) { /* ignore */ }
                }
            });

            socket.addEventListener("message", (event) => {
                if (state.refinementSocket !== socket) return;
                let payload;
                try {
                    payload = JSON.parse(event.data);
                } catch (_) {
                    return;
                }
                handleRefinementMessage(ticketId, payload);
            });

            socket.addEventListener("close", () => {
                if (state.refinementSocket === socket) {
                    state.refinementSocket = null;
                }
            });

            socket.addEventListener("error", () => {
                // Non-fatal; the REST fallback in the send handler covers this.
                if (state.refinementSocket === socket) {
                    setRefinementStatus("Live connection unavailable — replies may be delayed", false);
                }
            });
        }

        // refinementThreadEl returns the live thread container, or null.
        function refinementThreadEl() {
            return document.getElementById("refinement-thread");
        }

        function refinementScrollToBottom() {
            const thread = refinementThreadEl();
            if (thread) thread.scrollTop = thread.scrollHeight;
        }

        // appendRefinementHumanBubble optimistically renders a human turn so the
        // sender sees it immediately, before the server echo arrives.
        function appendRefinementHumanBubble(author, text) {
            const thread = refinementThreadEl();
            if (!thread) return;
            const empty = thread.querySelector(".empty");
            if (empty) empty.remove();
            const bubble = document.createElement("div");
            bubble.className = "refinement-bubble refinement-bubble-human";
            bubble.innerHTML = "<div class=\"refinement-author\">" + escapeHTML(author || "you") + "</div>" +
                "<div class=\"refinement-text\"></div>";
            bubble.querySelector(".refinement-text").textContent = text;
            thread.appendChild(bubble);
            refinementScrollToBottom();
        }

        // removeRefinementStreamingBubble clears any in-progress streaming/thinking
        // agent bubble.
        function removeRefinementStreamingBubble() {
            const thread = refinementThreadEl();
            if (!thread) return;
            const streaming = thread.querySelector(".refinement-streaming");
            if (streaming) streaming.remove();
            const thinking = thread.querySelector(".refinement-thinking");
            if (thinking) thinking.remove();
        }

        // ensureRefinementStreamingBubble returns the live text node of the streaming
        // agent bubble, creating the bubble on first use.
        function ensureRefinementStreamingBubble() {
            const thread = refinementThreadEl();
            if (!thread) return null;
            let bubble = thread.querySelector(".refinement-streaming");
            if (!bubble) {
                const empty = thread.querySelector(".empty");
                if (empty) empty.remove();
                const thinking = thread.querySelector(".refinement-thinking");
                if (thinking) thinking.remove();
                bubble = document.createElement("div");
                bubble.className = "refinement-bubble refinement-bubble-agent refinement-streaming";
                bubble.innerHTML = "<div class=\"refinement-author\">refiner</div>" +
                    "<div class=\"refinement-text\"></div>";
                thread.appendChild(bubble);
            }
            return bubble.querySelector(".refinement-text");
        }

        // handleRefinementMessage applies a server → client streaming protocol message
        // to the open refinement thread.
        function handleRefinementMessage(ticketId, payload) {
            if (!payload || !payload.type) return;
            switch (payload.type) {
                case "refinement_connected":
                    if (payload.llm_available === false) {
                        showRefinementNoLLM(payload.llm_advice, payload.llm_command);
                    } else if (String(state.refinementBusyTicketId || "") === String(ticketId)) {
                        // A reply is in flight (e.g. reconnect after idle to resend) —
                        // keep the thinking cue rather than flashing "your turn".
                        setRefinementStatus("Refiner is thinking…", true);
                    } else {
                        setRefinementStatus("Connected — your turn", false);
                    }
                    return;
                case "message": {
                    // Skip echoes of the local sender's own human turn to avoid a
                    // duplicate bubble (we already rendered it optimistically).
                    if (payload.side === "human") {
                        const text = String(payload.text || "");
                        if (state.refinementLastHumanText != null && text === state.refinementLastHumanText) {
                            state.refinementLastHumanText = null;
                            return;
                        }
                        appendRefinementHumanBubble(payload.author, text);
                    }
                    return;
                }
                case "refinement_thinking": {
                    state.refinementBusyTicketId = String(ticketId);
                    setRefinementStatus("Refiner is thinking…", true);
                    const thread = refinementThreadEl();
                    if (!thread) return;
                    removeRefinementStreamingBubble();
                    const bubble = document.createElement("div");
                    // Mark as thinking so the first chunk knows to clear the
                    // animated dots before appending real text.
                    bubble.className = "refinement-bubble refinement-bubble-agent refinement-streaming refinement-thinking";
                    bubble.innerHTML = "<div class=\"refinement-author\">refiner</div>" +
                        "<div class=\"refinement-text\"><span class=\"refinement-typing\" aria-label=\"thinking\">" +
                        "<span></span><span></span><span></span></span></div>";
                    thread.appendChild(bubble);
                    refinementScrollToBottom();
                    return;
                }
                case "chunk": {
                    state.refinementBusyTicketId = String(ticketId);
                    setRefinementStatus("Refiner is responding…", true);
                    const node = ensureRefinementStreamingBubble();
                    if (node) {
                        // Clear the "thinking" dots once real output begins.
                        const bubble = node.closest(".refinement-bubble");
                        if (bubble && bubble.classList.contains("refinement-thinking")) {
                            bubble.classList.remove("refinement-thinking");
                            node.textContent = "";
                        }
                        node.textContent += String(payload.text || "");
                        refinementScrollToBottom();
                    }
                    return;
                }
                case "message_done": {
                    state.refinementBusyTicketId = null;
                    removeRefinementStreamingBubble();
                    setRefinementStatus("Refiner replied — your turn", false);
                    refreshOpenRefinement(ticketId);
                    return;
                }
                case "refinement_busy":
                    state.refinementBusyTicketId = String(ticketId);
                    setRefinementStatus("Refiner is still responding…", true);
                    setNotice("Refiner is still responding…");
                    return;
                case "refinement_no_llm":
                    state.refinementBusyTicketId = null;
                    removeRefinementStreamingBubble();
                    showRefinementNoLLM(payload.advice, payload.command);
                    return;
                case "refinement_error":
                    state.refinementBusyTicketId = null;
                    removeRefinementStreamingBubble();
                    setRefinementStatus("Error: " + (payload.error || "refinement failed"), false);
                    setNotice(payload.error || "Refinement error", true);
                    return;
                case "refinement_idle_closed": {
                    state.refinementBusyTicketId = null;
                    const thread = refinementThreadEl();
                    if (thread) {
                        const note = document.createElement("div");
                        note.className = "empty refinement-idle-note";
                        note.textContent = "Session idle — send a message to resume.";
                        thread.appendChild(note);
                        refinementScrollToBottom();
                    }
                    setRefinementStatus("Session idle — send a message to resume", false);
                    if (state.refinementSocket && String(state.refinementTicketId) === String(ticketId)) {
                        state.refinementSocket = null;
                    }
                    return;
                }
                default:
                    return;
            }
        }

        // sendRefinementMessage streams a human turn over the refinement WebSocket,
        // reconnecting first if the session went idle. Returns true if handled over
        // the socket; false if the caller should fall back to REST.
        function sendRefinementMessage(ticketId, text) {
            const socket = state.refinementSocket;
            const sameTicket = String(state.refinementTicketId) === String(ticketId);
            if (socket && sameTicket && socket.readyState === WebSocket.OPEN) {
                state.refinementLastHumanText = text;
                try {
                    socket.send(JSON.stringify({ type: "message", text: text }));
                } catch (_) {
                    return false;
                }
                state.refinementBusyTicketId = String(ticketId);
                appendRefinementHumanBubble("you", text);
                setRefinementStatus("Refiner is thinking…", true);
                return true;
            }
            // Socket closed (e.g. idle) or absent: reconnect and queue the send.
            if (typeof WebSocket !== "undefined" && !window.__site2MockFetch) {
                state.refinementLastHumanText = text;
                state.refinementPendingSend = text;
                connectRefinementSocket(ticketId);
                if (state.refinementSocket) {
                    state.refinementBusyTicketId = String(ticketId);
                    appendRefinementHumanBubble("you", text);
                    setRefinementStatus("Refiner is thinking…", true);
                    return true;
                }
                state.refinementPendingSend = null;
            }
            return false;
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
            const label = state.ticketLabels.find((item) => String(item.label_id || item.id) === String(labelID));
            const confirmed = await uiConfirm("Remove label " + ((label && label.name) ? "\"" + label.name + "\"" : "#" + labelID) + " from this ticket?", "Remove");
            if (!confirmed) {
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
            const confirmed = await uiConfirm("Remove dependency on " + String(dependsOn) + "?", "Remove");
            if (!confirmed) {
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

        // Context links (FACTORY.md §5.7 context graph): documents, URLs, and other tickets
        // attached to this ticket as supporting context.
        function contextEdgeLabel(edge) {
            const isSource = state.activeTicket && edge.source_type === "ticket" &&
                edge.source_id === state.activeTicket.id;
            const type = isSource ? edge.target_type : edge.source_type;
            const id = isSource ? edge.target_id : edge.source_id;
            if (type === "url") {
                return { type: type, text: edge.title || id, href: id };
            }
            return { type: type, text: (type === "document" ? "doc " : "") + id, href: "" };
        }

        function renderTicketContext() {
            if (!els.ticketContext) return;
            if (!Array.isArray(state.ticketContext) || !state.ticketContext.length) {
                els.ticketContext.innerHTML = "<div class=\"empty\">No context attached yet.</div>";
                return;
            }
            els.ticketContext.innerHTML = state.ticketContext.map((edge) => {
                const label = contextEdgeLabel(edge);
                const body = label.href
                    ? "<a href=\"" + escapeHTML(label.href) + "\" target=\"_blank\" rel=\"noopener noreferrer\">" + escapeHTML(label.text) + "</a>"
                    : escapeHTML(label.text);
                return "<div class=\"history-item\"><strong>" + body + "</strong><div class=\"meta\">" +
                    escapeHTML(label.type + " · " + (edge.relation || "references")) +
                    "</div><button type=\"button\" data-remove-ticket-context=\"" + String(edge.edge_id) + "\">Remove</button></div>";
            }).join("");
        }

        async function loadTicketContext(ticketID) {
            if (!els.ticketContext) return;
            if (!ticketID) {
                state.ticketContext = [];
                renderTicketContext();
                return;
            }
            state.ticketContext = await api("/api/tickets/" + ticketID + "/context");
            renderTicketContext();
        }

        async function addTicketContext() {
            if (!state.activeTicket || !state.activeTicket.id) {
                return;
            }
            const targetType = els.ticketContextType.value;
            const targetID = els.ticketContextTarget.value.trim();
            if (!targetID) {
                setNotice("Context target is required.", true);
                return;
            }
            try {
                await api("/api/tickets/" + state.activeTicket.id + "/context", {
                    method: "POST",
                    body: JSON.stringify({ target_type: targetType, target_id: targetID }),
                });
                els.ticketContextTarget.value = "";
                await loadTicketContext(state.activeTicket.id);
                setNotice("Context attached.");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        async function removeTicketContext(edgeID) {
            if (!state.activeTicket || !state.activeTicket.id) {
                return;
            }
            try {
                await api("/api/tickets/" + state.activeTicket.id + "/context/" + edgeID, { method: "DELETE" });
                await loadTicketContext(state.activeTicket.id);
                setNotice("Context removed.");
            } catch (error) {
                setNotice(error.message, true);
            }
        }

        // ── Context view ────────────────────────────────────────────────────
        // Renders the project context graph (GET /api/projects/{ref}/context) as
        // an SVG node-link map. Nodes are tickets, documents, and URLs; clicking
        // one opens it. A focused story shows its direct context, rest dimmed.

        function contextNodeKey(type, id) {
            return String(type) + ":" + String(id);
        }

        // layoutContextGraph computes node positions with a small deterministic
        // force simulation: nodes seeded on a circle, pairwise repulsion, springs
        // along edges, gentle centering. Deterministic so renders are stable.
        function layoutContextGraph(graph, width, height) {
            const keys = graph.nodes.map((n) => contextNodeKey(n.type, n.id));
            const index = new Map(keys.map((key, i) => [key, i]));
            const cx = width / 2;
            const cy = height / 2;
            const seedRadius = Math.min(width, height) * 0.38;
            const count = Math.max(1, graph.nodes.length);
            const pts = graph.nodes.map((n, i) => {
                const angle = (2 * Math.PI * i) / count;
                const jitter = 0.6 + 0.4 * (((i * 7919) % 97) / 97);
                return { x: cx + seedRadius * jitter * Math.cos(angle), y: cy + seedRadius * jitter * Math.sin(angle) };
            });
            const links = graph.edges.map((e) => ({
                a: index.get(contextNodeKey(e.source_type, e.source_id)),
                b: index.get(contextNodeKey(e.target_type, e.target_id)),
            })).filter((l) => l.a !== undefined && l.b !== undefined && l.a !== l.b);

            const iterations = 180;
            for (let iter = 0; iter < iterations; iter++) {
                const heat = 1 - iter / iterations;
                for (let i = 0; i < pts.length; i++) {
                    for (let j = i + 1; j < pts.length; j++) {
                        let dx = pts[j].x - pts[i].x;
                        let dy = pts[j].y - pts[i].y;
                        const d2 = dx * dx + dy * dy || 1;
                        const d = Math.sqrt(d2);
                        const push = Math.min(10, 2600 / d2) * heat;
                        dx /= d;
                        dy /= d;
                        pts[i].x -= dx * push;
                        pts[i].y -= dy * push;
                        pts[j].x += dx * push;
                        pts[j].y += dy * push;
                    }
                }
                links.forEach((l) => {
                    const a = pts[l.a];
                    const b = pts[l.b];
                    const dx = b.x - a.x;
                    const dy = b.y - a.y;
                    const d = Math.sqrt(dx * dx + dy * dy) || 1;
                    const pull = ((d - 120) / d) * 0.18 * heat;
                    a.x += dx * pull;
                    a.y += dy * pull;
                    b.x -= dx * pull;
                    b.y -= dy * pull;
                });
                pts.forEach((p) => {
                    p.x += (cx - p.x) * 0.012 * heat;
                    p.y += (cy - p.y) * 0.012 * heat;
                });
            }
            pts.forEach((p) => {
                p.x = Math.max(60, Math.min(width - 60, p.x));
                p.y = Math.max(36, Math.min(height - 36, p.y));
            });
            return { index, pts };
        }

        async function loadContextGraph() {
            if (!state.selectedProjectID) {
                state.contextGraph = { nodes: [], edges: [] };
                return;
            }
            state.contextGraph = await api("/api/projects/" + state.selectedProjectID + "/context");
        }

        function contextFocusNeighbors(graph, focusKey) {
            const neighbors = new Set();
            if (!focusKey) return neighbors;
            neighbors.add(focusKey);
            (graph.edges || []).forEach((e) => {
                const a = contextNodeKey(e.source_type, e.source_id);
                const b = contextNodeKey(e.target_type, e.target_id);
                if (a === focusKey) neighbors.add(b);
                if (b === focusKey) neighbors.add(a);
            });
            return neighbors;
        }

        function renderContextView() {
            const svg = document.getElementById("context-graph");
            if (!svg) return;
            const emptyEl = document.getElementById("context-empty");
            const clearBtn = document.getElementById("context-clear-focus");
            if (clearBtn) clearBtn.classList.toggle("hidden", !state.contextFocusKey);
            const graph = state.contextGraph;
            if (!graph || !Array.isArray(graph.nodes) || !graph.nodes.length) {
                svg.innerHTML = "";
                if (emptyEl) emptyEl.classList.remove("hidden");
                return;
            }
            if (emptyEl) emptyEl.classList.add("hidden");

            const width = 960;
            const height = 620;
            svg.setAttribute("viewBox", "0 0 " + width + " " + height);
            const layout = layoutContextGraph(graph, width, height);
            const focusKey = state.contextFocusKey;
            const neighbors = contextFocusNeighbors(graph, focusKey);
            const matches = state.contextMatches;

            const edgesHtml = (graph.edges || []).map((e) => {
                const aKey = contextNodeKey(e.source_type, e.source_id);
                const bKey = contextNodeKey(e.target_type, e.target_id);
                const ai = layout.index.get(aKey);
                const bi = layout.index.get(bKey);
                if (ai === undefined || bi === undefined) return "";
                const a = layout.pts[ai];
                const b = layout.pts[bi];
                const inFocus = !focusKey || aKey === focusKey || bKey === focusKey;
                return "<line class=\"context-edge" + (inFocus ? "" : " dimmed") + "\" x1=\"" + a.x.toFixed(1) +
                    "\" y1=\"" + a.y.toFixed(1) + "\" x2=\"" + b.x.toFixed(1) + "\" y2=\"" + b.y.toFixed(1) + "\"></line>";
            }).join("");

            const nodesHtml = graph.nodes.map((node, i) => {
                const key = contextNodeKey(node.type, node.id);
                const p = layout.pts[i];
                const inFocus = !focusKey || neighbors.has(key);
                const matched = matches instanceof Set && matches.has(key);
                let label = String(node.title || node.id || "");
                if (label.length > 26) label = label.slice(0, 25) + "…";
                const classes = ["context-node", "context-node-" + node.type];
                if (!inFocus) classes.push("dimmed");
                if (matched) classes.push("matched");
                if (focusKey === key) classes.push("focus");
                return "<g class=\"" + classes.join(" ") + "\" data-node-key=\"" + escapeHTML(key) +
                    "\" data-node-type=\"" + escapeHTML(node.type) + "\" data-node-id=\"" + escapeHTML(node.id) +
                    "\" transform=\"translate(" + p.x.toFixed(1) + "," + p.y.toFixed(1) + ")\" role=\"button\" tabindex=\"0\">" +
                    "<title>" + escapeHTML(node.type + ": " + (node.title || node.id)) + "</title>" +
                    "<circle r=\"" + (focusKey === key ? 13 : 9) + "\"></circle>" +
                    "<text y=\"" + (focusKey === key ? 28 : 22) + "\" text-anchor=\"middle\">" + escapeHTML(label) + "</text>" +
                    "</g>";
            }).join("");

            svg.innerHTML = edgesHtml + nodesHtml;
        }

        async function refreshContextView() {
            try {
                await loadContextGraph();
            } catch (error) {
                state.contextGraph = { nodes: [], edges: [] };
                setNotice(error.message || "Failed to load context graph", true);
            }
            renderContextView();
        }

        // openContextNode opens whatever the node represents: ticket modal,
        // document editor, or the external URL in a new tab.
        function openContextNode(nodeType, nodeID) {
            if (nodeType === "ticket") {
                const ticket = (state.tickets || []).find((t) => String(t.id) === String(nodeID));
                if (ticket) openTicketModal(ticket);
                return;
            }
            if (nodeType === "document") {
                state.selectedDocumentID = Number(nodeID);
                switchView("documents");
                renderAll();
                return;
            }
            if (nodeType === "url") {
                window.open(String(nodeID), "_blank", "noopener,noreferrer");
            }
        }

        // focusTicketInContextGraph is the "View in graph" entry point from the
        // ticket modal: jump to the Context view centered on that story.
        function focusTicketInContextGraph(ticketID) {
            state.contextFocusKey = contextNodeKey("ticket", ticketID);
            state.contextMatches = null;
            const searchInput = document.getElementById("context-search-input");
            if (searchInput) searchInput.value = "";
            switchView("context");
        }

        async function runContextSearch(query) {
            const trimmed = String(query || "").trim();
            if (!trimmed) {
                state.contextMatches = null;
                renderContextView();
                return;
            }
            const nodes = await api("/api/projects/" + state.selectedProjectID + "/context/search?q=" + encodeURIComponent(trimmed));
            state.contextMatches = new Set((nodes || []).map((n) => contextNodeKey(n.type, n.id)));
            renderContextView();
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

                // Handle release assignment (features only).
                const ticketReleaseEl = document.getElementById("ticket-release");
                if (ticketReleaseEl && ticket.type === "feature") {
                    const releaseVal = ticketReleaseEl.value;
                    const newReleaseID = releaseVal ? Number(releaseVal) : null;
                    const currentReleaseID = ticket.release_id || null;
                    if (newReleaseID !== currentReleaseID) {
                        await apiClient.setTicketRelease(ticket.id, newReleaseID);
                    }
                }

                closeTicketModal();
                await loadTickets();
                renderTicketBoard();
                renderTicketListView();
                renderTicketPlanView();
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

        function bindWorkflowGraphPan() {
            let panState = null;

            function stopPan(pointerID) {
                if (!panState || (pointerID !== undefined && panState.pointerID !== pointerID)) {
                    return;
                }
                panState.viewport.classList.remove("is-panning");
                panState = null;
            }

            els.stageGrid.addEventListener("pointerdown", (event) => {
                const viewport = event.target.closest("[data-workflow-graph-viewport]");
                if (!viewport || state.workflowStageViewMode !== "graph" || event.button !== 0) {
                    return;
                }
                if (event.target.closest(".workflow-graph-node, button, input, textarea, select, label, summary")) {
                    return;
                }
                panState = {
                    pointerID: event.pointerId,
                    viewport,
                    startX: event.clientX,
                    startY: event.clientY,
                    scrollLeft: viewport.scrollLeft,
                    scrollTop: viewport.scrollTop,
                };
                viewport.classList.add("is-panning");
                viewport.setPointerCapture(event.pointerId);
                event.preventDefault();
            });

            els.stageGrid.addEventListener("pointermove", (event) => {
                if (!panState || event.pointerId !== panState.pointerID) {
                    return;
                }
                panState.viewport.scrollLeft = panState.scrollLeft - (event.clientX - panState.startX);
                panState.viewport.scrollTop = panState.scrollTop - (event.clientY - panState.startY);
            });

            els.stageGrid.addEventListener("pointerup", (event) => {
                stopPan(event.pointerId);
            });
            els.stageGrid.addEventListener("pointercancel", (event) => {
                stopPan(event.pointerId);
            });
        }

        function bindStageDragAndDrop() {
            const stageCreateForm = document.getElementById("new-stage-form");

            function clearStageDragTargets() {
                document.querySelectorAll("[data-stage-role-row]").forEach((row) => row.classList.remove("drag-target"));
                document.querySelectorAll(".workflow-role-card").forEach((card) => card.classList.remove("drag-target"));
                document.querySelectorAll(".stage-card").forEach((card) => {
                    card.classList.remove("drag-target");
                    card.classList.remove("is-dragging");
                });
                if (stageCreateForm) {
                    stageCreateForm.classList.remove("drag-target");
                }
            }

            function stageRoleIDs(stage) {
                return (stage && Array.isArray(stage.roles) ? stage.roles : [])
                    .map((role) => Number(role.id))
                    .filter((roleID) => !Number.isNaN(roleID) && roleID > 0);
            }

            function insertRoleID(roleIDs, roleID, beforeRoleID) {
                const ordered = roleIDs.filter((currentID) => Number(currentID) !== Number(roleID));
                const insertIndex = beforeRoleID ? ordered.indexOf(Number(beforeRoleID)) : -1;
                ordered.splice(insertIndex >= 0 ? insertIndex : ordered.length, 0, Number(roleID));
                return ordered;
            }

            async function persistStageRoleOrder(workflowID, stageID, roleIDs) {
                await api("/api/workflows/stages/roles/" + workflowID + "/" + stageID, {
                    method: "PUT",
                    body: JSON.stringify({ role_ids: roleIDs }),
                });
            }

            if (els.workflowRoleBank) {
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
                    event.dataTransfer.setData("text/plain", "stage-role");
                });

                els.workflowRoleBank.addEventListener("dragend", () => {
                    clearStageDragTargets();
                    state.drag = null;
                });
            }

            els.stageGrid.addEventListener("dragstart", (event) => {
                const role = event.target.closest("[data-role-id]");
                if (role) {
                    state.drag = {
                        type: "stage-role",
                        stageID: Number(role.dataset.stageId),
                        roleID: Number(role.dataset.roleId),
                    };
                    event.dataTransfer.effectAllowed = "move";
                    event.dataTransfer.setData("text/plain", "stage-role");
                    return;
                }
                const stage = event.target.closest("[data-stage-id]");
                if (!stage || state.workflowStageViewMode !== "board") {
                    return;
                }
                state.drag = { type: "stage", stageID: Number(stage.dataset.stageId) };
                event.dataTransfer.effectAllowed = "move";
                event.dataTransfer.setData("text/plain", "stage");
                stage.classList.add("is-dragging");
            });

            els.stageGrid.addEventListener("dragend", () => {
                clearStageDragTargets();
                state.drag = null;
            });

            els.stageGrid.addEventListener("dragover", (event) => {
                if (!state.drag) {
                    return;
                }
                const roleCard = event.target.closest(".workflow-role-card");
                const roleRow = event.target.closest("[data-stage-role-row]");
                const stageCard = event.target.closest(".stage-card");
                if (state.drag.type === "stage-role" && roleCard) {
                    event.preventDefault();
                    clearStageDragTargets();
                    roleCard.classList.add("drag-target");
                    return;
                }
                if (state.drag.type === "stage-role" && roleRow) {
                    event.preventDefault();
                    clearStageDragTargets();
                    roleRow.classList.add("drag-target");
                    return;
                }
                if (state.drag.type === "stage" && stageCard) {
                    if (state.workflowStageViewMode !== "board") {
                        return;
                    }
                    event.preventDefault();
                    clearStageDragTargets();
                    stageCard.classList.add("drag-target");
                }
            });

            if (stageCreateForm) {
                stageCreateForm.addEventListener("dragover", (event) => {
                    if (!state.drag || state.drag.type !== "stage") {
                        return;
                    }
                    event.preventDefault();
                    clearStageDragTargets();
                    stageCreateForm.classList.add("drag-target");
                });
            }

            els.stageGrid.addEventListener("drop", async (event) => {
                if (!state.drag) {
                    return;
                }
                const workflow = getCurrentWorkflow();
                if (!workflow) {
                    state.drag = null;
                    clearStageDragTargets();
                    return;
                }
                if (state.drag.type === "stage-role") {
                    const targetRoleCard = event.target.closest(".workflow-role-card");
                    const roleRow = event.target.closest("[data-stage-role-row]");
                    if (!roleRow && !targetRoleCard) {
                        state.drag = null;
                        clearStageDragTargets();
                        return;
                    }
                    event.preventDefault();
                    clearStageDragTargets();
                    const targetStageID = targetRoleCard ? Number(targetRoleCard.dataset.stageId) : Number(roleRow.dataset.stageRoleRow);
                    const targetRoleID = targetRoleCard ? Number(targetRoleCard.dataset.roleId) : null;
                    if (targetRoleID && Number(targetRoleID) === Number(state.drag.roleID) && Number(targetStageID) === Number(state.drag.stageID)) {
                        state.drag = null;
                        return;
                    }
                    const targetStage = workflow.stages.find((item) => item.id === targetStageID);
                    if (!targetStage) {
                        state.drag = null;
                        return;
                    }
                    try {
                        if (state.drag.stageID && state.drag.stageID !== targetStageID) {
                            await api("/api/workflows/stages/roles/" + workflow.id + "/" + state.drag.stageID + "/" + state.drag.roleID, { method: "DELETE" });
                            await api("/api/workflows/stages/roles/" + workflow.id + "/" + targetStageID, { method: "POST", body: JSON.stringify({ role_id: state.drag.roleID }) });
                            const orderedRoleIDs = insertRoleID(stageRoleIDs(targetStage), state.drag.roleID, targetRoleID);
                            if (orderedRoleIDs.length > 1 || targetRoleID) {
                                await persistStageRoleOrder(workflow.id, targetStageID, orderedRoleIDs);
                            }
                        } else if (!state.drag.stageID) {
                            await api("/api/workflows/stages/roles/" + workflow.id + "/" + targetStageID, { method: "POST", body: JSON.stringify({ role_id: state.drag.roleID }) });
                            const orderedRoleIDs = insertRoleID(stageRoleIDs(targetStage), state.drag.roleID, targetRoleID);
                            if (orderedRoleIDs.length > 1 || targetRoleID) {
                                await persistStageRoleOrder(workflow.id, targetStageID, orderedRoleIDs);
                            }
                        } else {
                            const orderedRoleIDs = insertRoleID(stageRoleIDs(targetStage), state.drag.roleID, targetRoleID);
                            await persistStageRoleOrder(workflow.id, targetStageID, orderedRoleIDs);
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
                if (state.workflowStageViewMode !== "board") {
                    state.drag = null;
                    clearStageDragTargets();
                    return;
                }
                if (!targetStage) {
                    state.drag = null;
                    clearStageDragTargets();
                    return;
                }
                event.preventDefault();
                clearStageDragTargets();
                const ordered = Array.from(els.stageGrid.querySelectorAll(".stage-card"))
                    .map((card) => Number(card.dataset.stageId))
                    .filter((stageID) => stageID !== state.drag.stageID);
                const targetIndex = ordered.indexOf(Number(targetStage.dataset.stageId));
                ordered.splice(targetIndex >= 0 ? targetIndex : ordered.length, 0, state.drag.stageID);
                try {
                    await persistWorkflowStageOrder(workflow.id, ordered);
                } catch (error) {
                    setNotice(error.message, true);
                }
                state.drag = null;
            });

            if (stageCreateForm) {
                stageCreateForm.addEventListener("drop", async (event) => {
                    if (!state.drag || state.drag.type !== "stage") {
                        return;
                    }
                    const workflow = getCurrentWorkflow();
                    if (!workflow) {
                        state.drag = null;
                        clearStageDragTargets();
                        return;
                    }
                    event.preventDefault();
                    clearStageDragTargets();
                    const ordered = Array.from(els.stageGrid.querySelectorAll(".stage-card"))
                        .map((card) => Number(card.dataset.stageId))
                        .filter((stageID) => stageID !== state.drag.stageID);
                    ordered.push(state.drag.stageID);
                    try {
                        await persistWorkflowStageOrder(workflow.id, ordered);
                    } catch (error) {
                        setNotice(error.message, true);
                    }
                    state.drag = null;
                });
            }
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
                    openAccountModal(item.dataset.accountAction).catch((error) => {
                        setPasskeyStatus(error.message, true);
                        renderAccountModal();
                    });
                    closeAccountMenu();
                }
            });

            if (els.closeAccountModal) {
                els.closeAccountModal.addEventListener("click", closeAccountModal);
            }
            if (els.accountModal) {
                els.accountModal.addEventListener("click", (event) => {
                    if (event.target === els.accountModal) {
                        closeAccountModal();
                        return;
                    }
                    const button = event.target.closest("[data-passkey-action]");
                    if (!button || state.passkeyBusy) {
                        return;
                    }
                    const credentialID = String(button.dataset.passkeyId || "");
                    if (!credentialID) {
                        return;
                    }
                    if (button.dataset.passkeyAction === "rename") {
                        handlePasskeyRename(credentialID).catch((error) => {
                            setPasskeyStatus(error.message, true);
                            renderAccountModal();
                        });
                        return;
                    }
                    if (button.dataset.passkeyAction === "delete") {
                        handlePasskeyDelete(credentialID).catch((error) => {
                            setPasskeyStatus(error.message, true);
                            renderAccountModal();
                        });
                    }
                });
            }
            if (els.accountPasskeyEnrollButton) {
                els.accountPasskeyEnrollButton.addEventListener("click", () => {
                    handlePasskeyEnrollment().catch((error) => {
                        setPasskeyStatus(error.message, true);
                        state.passkeyBusy = false;
                        renderAccountModal();
                    });
                });
            }
            if (els.accountOpenConfigButton) {
                els.accountOpenConfigButton.addEventListener("click", () => {
                    closeAccountModal();
                    switchView("settings");
                });
            }

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
                state.auth = null;
                state.passkeys = [];
                state.passkeyError = "";
                setPasskeyStatus("", false);
                clearStoredAuth();
                showLoginScreen();
                els.loginForm.reset();
            });
        }

        els.loginForm.addEventListener("submit", handleLogin);
        if (els.loginPasskeyButton) {
            els.loginPasskeyButton.addEventListener("click", handlePasskeyLogin);
        }
        els.registerForm.addEventListener("submit", handleRegister);
        els.showRegisterButton.addEventListener("click", showRegisterForm);
        els.hideRegisterButton.addEventListener("click", showLoginForm);
        state.navOrder = loadStoredNavOrder();
        renderMainNav();
        bindViewNavigation();
        bindProjectHandlers();
        bindPlanHandlers();
        bindAgentModelHandlers();
        bindDocumentsHandlers();
        bindWorkflowHandlers();
        bindRolesHandlers();
        bindAgentsHandlers();
        bindUsersHandlers();
        bindOrgHandlers();
        bindProgrammeHandlers();
        bindTeamsHandlers();
        bindTicketsHandlers();
        bindMiscHandlers();
        if (els.dialogOK) {
            els.dialogOK.addEventListener("click", () => closeDialog(true));
        }
        if (els.dialogCancel) {
            els.dialogCancel.addEventListener("click", () => closeDialog(false));
        }
        if (els.dialogInput) {
            els.dialogInput.addEventListener("keydown", (event) => {
                if (event.key === "Enter") {
                    event.preventDefault();
                    closeDialog(true);
                }
            });
        }
        if (els.dialogOverlay) {
            els.dialogOverlay.addEventListener("click", (event) => {
                if (event.target === els.dialogOverlay) {
                    closeDialog(false);
                }
            });
        }
        // ── Command palette: Shift Shift quick-switcher (TK-127) ──────────
        // Friendly + single-letter aliases so the site is keyboard-driveable
        // (e.g. /c = /chat, /b = /backlog). Aliases pointing at not-yet-registered
        // views (like chat) simply don't appear until that view exists.
        const PALETTE_ALIASES = {
            c: "chat", chat: "chat",
            b: "tickets", board: "tickets", backlog: "tickets",
            p: "projects",
            w: "workflows",
            d: "documents",
            m: "interventions", mailbox: "interventions", inbox: "interventions",
            r: "roles",
            t: "teams",
            g: "context",
            x: "agents", a: "agents",
            u: "users",
            s: "settings",
        };
        const TICKET_KEY_RE = /^[a-z]{1,6}-\d+$/i;
        async function openTicketByKey(rawKey) {
            const key = String(rawKey || "").trim().toUpperCase();
            let ticket = (state.tickets || []).find((t) => String(t.id || "").toUpperCase() === key);
            if (!ticket) {
                try { ticket = await api("/api/tickets/" + encodeURIComponent(key)); } catch (err) { ticket = null; }
            }
            if (ticket && (ticket.id || ticket.ticket_id)) { openTicketModal(ticket); }
            else { setNotice("Ticket " + key + " not found.", true); }
        }
        const paletteEls = {
            overlay: document.getElementById("command-palette-overlay"),
            input: document.getElementById("command-palette-input"),
            list: document.getElementById("command-palette-list"),
        };
        let paletteActiveIndex = 0;
        let lastShiftAt = 0;
        function buildPaletteCommands() {
            const known = new Set(availableViewNames());
            const labelByView = {};
            NAV_ITEMS.forEach((item) => { labelByView[item.view] = item.label; });
            const commands = [];
            const seen = new Set();
            const add = (slash, view) => {
                if (!known.has(view) || seen.has(slash)) { return; }
                seen.add(slash);
                commands.push({ slash: "/" + slash, key: slash, view: view, label: labelByView[view] || view });
            };
            visibleNavItems().forEach((item) => add(item.view, item.view));
            Object.keys(PALETTE_ALIASES).forEach((alias) => add(alias, PALETTE_ALIASES[alias]));
            return commands;
        }
        function isPaletteOpen() { return paletteEls.overlay && !paletteEls.overlay.classList.contains("hidden"); }
        // The palette is a STACK of frames (TK-130): the command list, and entity
        // action menus pushed on top (e.g. a ticket's actions). Esc pops one frame;
        // popping past the root closes the palette.
        let paletteStack = [{ kind: "commands" }];
        // After navigating to a view, optionally focus an element (e.g. /chat
        // focuses the chat composer once the rooms view registers it, TK-118 S5).
        const PALETTE_FOCUS = { chat: "#chat-composer-input" };
        function paletteFocusView(view) {
            const sel = PALETTE_FOCUS[view];
            if (!sel) { return; }
            setTimeout(() => { const el = document.querySelector(sel); if (el) { el.focus(); } }, 60);
        }
        function paletteTopFrame() { return paletteStack[paletteStack.length - 1]; }
        function paletteQuery() { return String(paletteEls.input.value || "").replace(/^\//, "").trim().toLowerCase(); }
        function promptTicketComment(key) {
            return Promise.resolve(uiPrompt("Comment on " + key, "", "Comment")).then((text) => {
                const body = String(text || "").trim();
                if (!body) { return; }
                return api("/api/tickets/" + encodeURIComponent(key) + "/comments", { method: "POST", body: { comment: body } })
                    .then(() => setNotice("Commented on " + key + "."))
                    .catch((err) => setNotice("Comment failed: " + err.message, true));
            });
        }
        function ticketActionItems(key) {
            return [
                { label: "Open detail", run: () => openTicketByKey(key) },
                { label: "Add comment…", run: () => promptTicketComment(key) },
                { label: "Copy key", run: () => { try { if (navigator.clipboard) { navigator.clipboard.writeText(key); } } catch (e) {} setNotice("Copied " + key + "."); } },
            ];
        }
        function pushTicketActions(key) {
            paletteStack.push({ kind: "actions", title: key, items: ticketActionItems(key) });
            paletteActiveIndex = 0;
            paletteEls.input.value = "";
            renderPalette();
        }
        function popPaletteFrame() {
            if (paletteStack.length <= 1) { return false; }
            paletteStack.pop();
            paletteActiveIndex = 0;
            paletteEls.input.value = "";
            renderPalette();
            return true;
        }
        function paletteFrameItems() {
            const frame = paletteTopFrame();
            const query = paletteQuery();
            if (frame.kind === "actions") {
                return frame.items
                    .filter((it) => !query || it.label.toLowerCase().includes(query))
                    .map((it, i) => ({ left: String(i + 1), label: it.label, run: () => { closeCommandPalette(); it.run(); } }));
            }
            const rank = (cmd) => (cmd.key === query ? 0 : cmd.key.startsWith(query) ? 1 : cmd.label.toLowerCase().startsWith(query) ? 2 : 3);
            const items = buildPaletteCommands()
                .filter((cmd) => !query || cmd.key.includes(query) || cmd.label.toLowerCase().includes(query))
                .sort((a, b) => rank(a) - rank(b))
                .map((cmd) => ({ left: cmd.slash, label: cmd.label, run: () => { closeCommandPalette(); switchView(cmd.view); paletteFocusView(cmd.view); } }));
            if (TICKET_KEY_RE.test(query)) {
                const key = query.toUpperCase();
                items.unshift({ left: "/" + query, label: "Ticket " + key + " — actions…", run: () => pushTicketActions(key) });
            }
            return items;
        }
        function renderPalette() {
            const frame = paletteTopFrame();
            const items = paletteFrameItems();
            paletteEls.input.placeholder = frame.kind === "actions"
                ? (frame.title + " — pick an action (number / ↑↓)")
                : "Type a / command — e.g. /chat, /backlog, /tk-23";
            if (paletteActiveIndex >= items.length) { paletteActiveIndex = Math.max(0, items.length - 1); }
            paletteEls.list._items = items;
            if (!items.length) {
                paletteEls.list.innerHTML = "<li class=\"command-palette-empty\">No matching commands</li>";
                return;
            }
            paletteEls.list.innerHTML = items.map((it, i) =>
                "<li class=\"command-palette-item" + (i === paletteActiveIndex ? " active" : "") + "\" role=\"option\" data-cp-index=\"" + i + "\">" +
                "<span class=\"cp-slash\">" + escapeHTML(it.left) + "</span><span class=\"cp-label\">" + escapeHTML(it.label) + "</span></li>").join("");
        }
        function runPaletteActive() {
            const items = paletteEls.list._items || [];
            const it = items[paletteActiveIndex];
            if (it) { it.run(); }
        }
        function openCommandPalette() {
            if (!paletteEls.overlay || isPaletteOpen()) { return; }
            paletteStack = [{ kind: "commands" }];
            paletteActiveIndex = 0;
            paletteEls.input.value = "";
            paletteEls.overlay.classList.remove("hidden");
            renderPalette();
            setTimeout(() => { paletteEls.input.focus(); }, 0);
        }
        function closeCommandPalette() {
            if (paletteEls.overlay) { paletteEls.overlay.classList.add("hidden"); }
        }
        function setupCommandPalette() {
            if (!paletteEls.overlay) { return; }
            paletteEls.input.addEventListener("input", () => { paletteActiveIndex = 0; renderPalette(); });
            paletteEls.input.addEventListener("keydown", (event) => {
                const items = paletteEls.list._items || [];
                if (event.key === "ArrowDown") { event.preventDefault(); paletteActiveIndex = Math.min(items.length - 1, paletteActiveIndex + 1); renderPalette(); }
                else if (event.key === "ArrowUp") { event.preventDefault(); paletteActiveIndex = Math.max(0, paletteActiveIndex - 1); renderPalette(); }
                else if (event.key === "Enter") { event.preventDefault(); runPaletteActive(); }
                else if (event.key === "Escape") { event.preventDefault(); event.stopPropagation(); if (!popPaletteFrame()) { closeCommandPalette(); } }
                else if (/^[1-9]$/.test(event.key) && paletteTopFrame().kind === "actions") {
                    const idx = Number(event.key) - 1;
                    if (idx < items.length) { event.preventDefault(); paletteActiveIndex = idx; runPaletteActive(); }
                }
            });
            paletteEls.list.addEventListener("click", (event) => {
                const item = event.target.closest("[data-cp-index]");
                if (!item) { return; }
                paletteActiveIndex = Number(item.dataset.cpIndex) || 0;
                runPaletteActive();
            });
            paletteEls.overlay.addEventListener("click", (event) => { if (event.target === paletteEls.overlay) { closeCommandPalette(); } });
            document.addEventListener("keydown", (event) => {
                if (event.key === "Shift" && !event.repeat) {
                    const now = Date.now();
                    if (now - lastShiftAt < 400) { lastShiftAt = 0; if (isPaletteOpen()) { closeCommandPalette(); } else { openCommandPalette(); } }
                    else { lastShiftAt = now; }
                    return;
                }
                lastShiftAt = 0;
            });
        }
        document.addEventListener("keydown", (event) => {
            if (event.key !== "Escape") {
                return;
            }
            if (paletteEls.overlay && !paletteEls.overlay.classList.contains("hidden")) {
                closeCommandPalette();
                return;
            }
            if (els.dialogOverlay && !els.dialogOverlay.classList.contains("hidden")) {
                closeDialog(false);
            } else if (state.accountModalOpen) {
                closeAccountModal();
            } else if (els.ticketModal && els.ticketModal.classList.contains("open")) {
                closeTicketModal();
            }
        });
        setupCommandPalette();
        setupChat();
        setupBoardKeyboardNav();
        setupAccessView();
        setupEmailSettings();
        state.viewScrollByPanel = loadStoredViewScrollByPanel();
        state.currentView = loadStoredSelectedView() || state.currentView;
        switchView(state.currentView, { restoreScroll: false });
        window.addEventListener("scroll", storeCurrentViewScroll, { passive: true });

        (async function restoreSession() {
            const auth = loadStoredAuth();
            if (!auth) {
                await loadPublicStatus();
                if (state.status && state.status.authenticated) {
                    state.auth = {
                        username: (state.status.user && state.status.user.username) || "",
                        token: "",
                    };
                    apiClient.setToken("");
                    localStorage.setItem("tk-authed", "1");
                    document.getElementById("login-username").value = state.auth.username;
                    showAuthenticatedShell();
                    try {
                        await refreshAll();
                        connectLiveUpdates();
                        startAgentBarPoller();
                    } catch (error) {
                        disconnectLiveUpdates();
                        if (isAuthError(error)) {
                            state.auth = null;
                            clearStoredAuth();
                            els.loginError.textContent = error.message;
                            showLoginScreen();
                            return;
                        }
                        setNotice(error.message, true);
                        showAuthenticatedShell();
                    }
                    return;
                }
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
                if (isAuthError(error)) {
                    state.auth = null;
                    clearStoredAuth();
                    els.loginError.textContent = error.message;
                    showLoginScreen();
                    return;
                }
                setNotice(error.message, true);
                showAuthenticatedShell();
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
