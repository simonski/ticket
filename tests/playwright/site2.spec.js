const { test, expect } = require("@playwright/test");

function installSite2Mock(page, seed = {}) {
  return page.addInitScript((mockSeed) => {
    const defaultProjects = [
      {
        project_id: 1,
        prefix: "OPS",
        title: "Operations",
        description: "Keep things running",
        acceptance_criteria: "",
        git_repository: "acme/ops",
        visibility: "public",
        workflow_id: 1,
        default_draft: false,
      },
    ];
    const db = {
      status: Object.assign({
        authenticated: false,
        mode: "local",
        version: "dev",
        user: null,
        registration_enabled: true,
        registration_auto_approve: true,
      }, mockSeed.status || {}),
      configSettings: Array.isArray(mockSeed.configSettings)
        ? mockSeed.configSettings.map((item) => ({ ...item }))
        : [
            { key: "chat_enabled", value: "1" },
            { key: "registration_enabled", value: "1" },
          ],
      nextProjectID: Number(mockSeed.nextProjectID || 2),
      projects: Array.isArray(mockSeed.projects) && mockSeed.projects.length ? mockSeed.projects : defaultProjects,
      tickets: [
        {
          ticket_id: "OPS-101",
          project_id: 1,
          type: "task",
          title: "Move me",
          description: "",
          acceptance_criteria: "",
          status: "open",
          stage: "design",
          priority: 1,
          order: 1,
          estimate_effort: 3,
          health_score: 5,
          draft: false,
          archived: false,
          workflow_id: null,
        },
      ],
      commentsByTicket: {
        "OPS-101": [{ author: "admin", text: "Initial note", date: "now" }],
      },
      labels: [{ label_id: 51, project_id: 1, name: "backend", color: "#ff6600", created_at: "now" }],
      ticketLabelIDs: { "OPS-101": [51] },
      dependenciesByTicket: {
        "OPS-101": [{ id: 71, project_id: 1, ticket_id: "OPS-101", depends_on: "OPS-100", created_by: "admin", created_at: "now" }],
      },
      nextDependencyID: 72,
      timeEntriesByTicket: {
        "OPS-101": [{ time_entry_id: 81, ticket_id: "OPS-101", user_id: "admin", minutes: 30, note: "Initial effort", created_at: "now" }],
      },
      nextTimeEntryID: 82,
      workflows: [
        {
          workflow_id: 1,
          name: "Delivery",
          description: "Default flow",
          stages: [
            { workflow_stage_id: 11, workflow_id: 1, stage_name: "backlog", description: "", definition_of_ready: "", definition_of_done: "", roles: [{ role_id: 5, title: "Engineer" }], next_stage_ids: [12] },
            { workflow_stage_id: 12, workflow_id: 1, stage_name: "todo", description: "", definition_of_ready: "", definition_of_done: "", roles: [], next_stage_ids: [11, 13, 14] },
            { workflow_stage_id: 13, workflow_id: 1, stage_name: "doing", description: "", definition_of_ready: "", definition_of_done: "", roles: [], next_stage_ids: [14] },
            { workflow_stage_id: 14, workflow_id: 1, stage_name: "done", description: "", definition_of_ready: "", definition_of_done: "", roles: [], next_stage_ids: [] },
          ],
        },
      ],
      roles: [
        { role_id: 5, title: "Engineer", description: "Build", acceptance_criteria: "", workflow_id: 1 },
        { role_id: 6, title: "QA", description: "Verify", acceptance_criteria: "", workflow_id: 1 },
      ],
      nextPlanID: Number(mockSeed.nextPlanID || 3),
      plans: Array.isArray(mockSeed.plans) && mockSeed.plans.length
        ? mockSeed.plans.map((plan) => ({ ...plan, registration_actions: { ...(plan.registration_actions || {}) } }))
        : [
            {
              plan_id: 1,
              slug: "starter",
              name: "Starter",
              description: "Entry plan",
              max_projects: 1,
              max_private_projects: 1,
              max_tickets: 100,
              max_tickets_per_project: 100,
              max_team_memberships: 10,
              max_api_calls_per_day: 1000,
              default_project_alias: "public",
              registration_actions: {
                auto_assign_public_team: true,
                auto_create_private_project: false,
                auto_create_private_team: false,
                teams: [],
                projects: [],
              },
            },
            {
              plan_id: 2,
              slug: "pro",
              name: "Pro",
              description: "Higher limits",
              max_projects: 10,
              max_private_projects: 5,
              max_tickets: 2000,
              max_tickets_per_project: 500,
              max_team_memberships: 50,
              max_api_calls_per_day: 20000,
              default_project_alias: "private",
              registration_actions: {
                auto_assign_public_team: true,
                auto_create_private_project: true,
                auto_create_private_team: true,
                teams: [],
                projects: [],
              },
            },
          ],
      defaultPlanSlug: String(mockSeed.defaultPlanSlug || "starter"),
      agents: [{ user_id: "agent-1", enabled: true }],
      teams: [{ team_id: 21, name: "Platform", parent_team_id: null }],
      nextGoalID: Number(mockSeed.nextGoalID || 2),
      goals: Array.isArray(mockSeed.goals)
        ? mockSeed.goals.map((goal) => ({ ...goal }))
        : [
            {
              goal_id: 1,
              project_id: 1,
              title: "Ship MVP",
              description: "Initial release goal",
              notes: "",
              eta: "",
              priority: 1,
              status: "draft",
              refined_goal: "",
              decomposition: "",
            },
      ],
      myProjectAccessRequests: Array.isArray(mockSeed.myProjectAccessRequests)
        ? mockSeed.myProjectAccessRequests.map((request) => ({ ...request }))
        : [],
      myNotifications: Array.isArray(mockSeed.myNotifications)
        ? mockSeed.myNotifications.map((notification) => ({ ...notification }))
        : [],
      projectHistoryByProject: Object.assign({}, mockSeed.projectHistoryByProject || {}),
      goalChatByGoal: {},
      nextDocumentID: 2,
      nextDocumentFileID: 2,
      documents: [
        {
          document_id: 1,
          project_id: 1,
          title: "Runbook",
          description: "Primary ops runbook",
          notes: "",
          content: "Initial content",
          created_at: "now",
          updated_at: "now",
        },
      ],
      documentFilesByDocument: {
        "1": [
          {
            file_id: 1,
            document_id: 1,
            file_name: "runbook.txt",
            content_type: "text/plain",
            size_bytes: 7,
            content: "UkVOREFNQQ==",
            created_at: "now",
          },
        ],
      },
      passkeys: Array.isArray(mockSeed.passkeys)
        ? mockSeed.passkeys.map((credential) => ({ ...credential }))
        : [],
      nextPasskeyIndex: Number(mockSeed.nextPasskeyIndex || 2),
    };

    window.__site2Requests = [];

    function json(body, status = 200) {
      return new Response(JSON.stringify(body), {
        status,
        headers: { "Content-Type": "application/json" },
      });
    }

    function parseBody(body) {
      if (!body) {
        return {};
      }
      return JSON.parse(body);
    }

    function last(pathParts) {
      return pathParts[pathParts.length - 1];
    }

    window.__site2MockFetch = async (input, init = {}) => {
      const method = (init.method || "GET").toUpperCase();
      const url = new URL(typeof input === "string" ? input : input.url, window.location.origin);
      const path = url.pathname;
      const body = parseBody(init.body);

      window.__site2Requests.push({ method, path, body });

      if (path === "/api/status") {
        return json(db.status);
      }
      if (path === "/api/config/settings" && method === "GET") {
        return json(db.configSettings);
      }
      if (path === "/api/config/registration" && method === "POST") {
        db.status.registration_enabled = Boolean(body.enabled);
        db.status.registration_auto_approve = Boolean(body.auto_approve);
        return json({
          enabled: db.status.registration_enabled,
          auto_approve: db.status.registration_auto_approve,
        });
      }
      if (path === "/api/plans" && method === "GET") {
        if (window.sessionStorage.getItem("site2.failPlansOnBoot") === "1") {
          return json({ error: "plans unavailable" }, 500);
        }
        return json(db.plans);
      }
      if (path === "/api/plans/default" && method === "GET") {
        return json(db.plans.find((plan) => plan.slug === db.defaultPlanSlug) || null);
      }
      if (path === "/api/plans/default" && method === "POST") {
        db.defaultPlanSlug = String(body.slug || "");
        return json(db.plans.find((plan) => plan.slug === db.defaultPlanSlug) || null);
      }
      if (path === "/api/plans" && method === "POST") {
        const plan = {
          plan_id: db.nextPlanID++,
          ...body,
          registration_actions: {
            auto_assign_public_team: Boolean(body.registration_actions?.auto_assign_public_team),
            auto_create_private_project: Boolean(body.registration_actions?.auto_create_private_project),
            auto_create_private_team: Boolean(body.registration_actions?.auto_create_private_team),
            teams: Array.isArray(body.registration_actions?.teams) ? body.registration_actions.teams : [],
            projects: Array.isArray(body.registration_actions?.projects) ? body.registration_actions.projects : [],
          },
        };
        db.plans.push(plan);
        return json(plan, 201);
      }
      if (path.match(/^\/api\/plans\/[^/]+$/) && method === "PUT") {
        const ref = decodeURIComponent(path.split("/")[3]);
        const plan = db.plans.find((item) => item.slug === ref);
        if (!plan) {
          return json({ error: "not found" }, 404);
        }
        Object.assign(plan, body, {
          registration_actions: {
            auto_assign_public_team: Boolean(body.registration_actions?.auto_assign_public_team),
            auto_create_private_project: Boolean(body.registration_actions?.auto_create_private_project),
            auto_create_private_team: Boolean(body.registration_actions?.auto_create_private_team),
            teams: Array.isArray(body.registration_actions?.teams) ? body.registration_actions.teams : [],
            projects: Array.isArray(body.registration_actions?.projects) ? body.registration_actions.projects : [],
          },
        });
        return json(plan);
      }
      if (path.match(/^\/api\/plans\/[^/]+$/) && method === "DELETE") {
        const ref = decodeURIComponent(path.split("/")[3]);
        db.plans = db.plans.filter((item) => item.slug !== ref);
        if (db.defaultPlanSlug === ref) {
          db.defaultPlanSlug = db.plans[0]?.slug || "";
        }
        return new Response(null, { status: 204 });
      }
      if (path === "/api/config/settings" && method === "POST") {
        const item = { key: String(body.key || "").trim(), value: String(body.value || "") };
        db.configSettings = db.configSettings.filter((entry) => entry.key !== item.key).concat([item]).sort((a, b) => a.key.localeCompare(b.key));
        return json(item);
      }
      if (path.match(/^\/api\/config\/settings\/.+$/) && method === "PUT") {
        const currentKey = decodeURIComponent(path.split("/").slice(4).join("/"));
        const nextKey = String(body.key || currentKey).trim();
        db.configSettings = db.configSettings.filter((entry) => entry.key !== currentKey && entry.key !== nextKey)
          .concat([{ key: nextKey, value: String(body.value || "") }])
          .sort((a, b) => a.key.localeCompare(b.key));
        return json({ key: nextKey, value: String(body.value || "") });
      }
      if (path.match(/^\/api\/config\/settings\/.+$/) && method === "DELETE") {
        const currentKey = decodeURIComponent(path.split("/").slice(4).join("/"));
        db.configSettings = db.configSettings.filter((entry) => entry.key !== currentKey);
        return new Response(null, { status: 204 });
      }
      if (path === "/api/login" && method === "POST") {
        db.status.authenticated = true;
        db.status.user = { username: body.username || "admin", role: "admin", email: "admin@example.com" };
        return json({ token: "test-token", user: { username: body.username || "admin", role: "admin", email: "admin@example.com" } });
      }
      if (path === "/api/users/me/passkeys" && method === "GET") {
        return json(db.passkeys);
      }
      if (path.match(/^\/api\/users\/me\/passkeys\/.+$/) && method === "PUT") {
        const credentialID = decodeURIComponent(path.split("/").slice(5).join("/"));
        const credential = db.passkeys.find((item) => item.credential_id === credentialID);
        if (!credential) {
          return json({ error: "passkey not found" }, 404);
        }
        credential.name = String(body.name || "");
        credential.updated_at = "now";
        return json(credential);
      }
      if (path.match(/^\/api\/users\/me\/passkeys\/.+$/) && method === "DELETE") {
        const credentialID = decodeURIComponent(path.split("/").slice(5).join("/"));
        const before = db.passkeys.length;
        db.passkeys = db.passkeys.filter((item) => item.credential_id !== credentialID);
        if (db.passkeys.length === before) {
          return json({ error: "passkey not found" }, 404);
        }
        return json({ status: "deleted" });
      }
      if (path === "/api/auth/passkey/login/start" && method === "POST") {
        db.passkeyLogin = {
          code: "passkey-1",
          username: body.username || "admin",
          finished: false,
        };
        return json({
          verification_url: window.location.origin + "/passkey?code=passkey-1",
          code: "passkey-1",
          expires_at: "2099-01-01T00:00:00Z",
        });
      }
      if (path === "/api/auth/passkey/register/start" && method === "POST") {
        const code = `passkey-register-${db.nextPasskeyIndex}`;
        db.passkeyRegistration = {
          code,
          name: String(body.name || ""),
        };
        return json({
          verification_url: window.location.origin + `/passkey?code=${encodeURIComponent(code)}`,
          code,
          expires_at: "2099-01-01T00:00:00Z",
        });
      }
      if (path === "/api/auth/passkey/challenge" && method === "GET") {
        const code = url.searchParams.get("code");
        if (db.passkeyLogin && code === db.passkeyLogin.code) {
          return json({
            kind: "login",
            public_key: {
              challenge: "Y2hhbGxlbmdl",
              rpId: window.location.hostname,
              timeout: 60000,
              userVerification: "required",
              allowCredentials: [
                { id: "Y3JlZC0x", type: "public-key" },
              ],
            },
          });
        }
        if (db.passkeyRegistration && code === db.passkeyRegistration.code) {
          return json({
            kind: "registration",
            public_key: {
              challenge: "Y3JlYXRlLWNoYWxsZW5nZQ",
              rp: { name: "Ticket", id: window.location.hostname },
              user: {
                id: "YWRtaW4",
                name: "admin",
                displayName: "admin",
              },
              pubKeyCredParams: [{ type: "public-key", alg: -7 }],
              timeout: 60000,
              attestation: "none",
            },
          });
        }
        return json({ error: "not found" }, 404);
      }
      if (path === "/api/auth/passkey/finish" && method === "POST") {
        const code = url.searchParams.get("code");
        if (db.passkeyLogin && code === db.passkeyLogin.code) {
          db.passkeyLogin.finished = true;
          db.passkeyLogin.assertion = body;
          return json({ status: "ok" });
        }
        if (db.passkeyRegistration && code === db.passkeyRegistration.code) {
          const credentialID = body.id || `cred-${db.nextPasskeyIndex}`;
          db.passkeys.push({
            credential_id: credentialID,
            name: db.passkeyRegistration.name || `Passkey ${db.nextPasskeyIndex}`,
            created_at: "now",
            updated_at: "now",
            last_used_at: "",
          });
          db.nextPasskeyIndex += 1;
          db.passkeyRegistration = null;
          return json({ status: "ok" });
        }
        return json({ error: "not found" }, 404);
      }
      if (path === "/api/auth/passkey/poll" && method === "POST") {
        if (!db.passkeyLogin || body.code !== db.passkeyLogin.code) {
          return json({ error: "not found" }, 404);
        }
        if (!db.passkeyLogin.finished) {
          return json({ status: "pending" }, 202);
        }
        db.status.authenticated = true;
        db.status.user = { username: db.passkeyLogin.username, role: "admin", email: "admin@example.com" };
        return json({
          status: "complete",
          token: "passkey-token",
          user: { username: db.passkeyLogin.username, role: "admin", email: "admin@example.com" },
        });
      }
      if (path === "/api/logout" && method === "POST") {
        db.status.authenticated = false;
        db.status.user = null;
        return json({ status: "ok" });
      }
      if (path === "/api/projects" && method === "GET") {
        return json(db.projects);
      }
      if (path === "/api/projects" && method === "POST") {
        const project = {
          project_id: db.nextProjectID++,
          prefix: body.prefix,
          title: body.title,
          description: body.description || "",
          acceptance_criteria: body.acceptance_criteria || "",
          git_repository: body.git_repository || "",
          visibility: body.visibility || "public",
          workflow_id: body.workflow_id || null,
          default_draft: false,
        };
        db.projects.push(project);
        return json(project, 201);
      }
      if (path.match(/^\/api\/projects\/\d+$/) && method === "PUT") {
        const id = Number(last(path.split("/")));
        const project = db.projects.find((item) => item.project_id === id);
        Object.assign(project, body);
        return json(project);
      }
      if (path.match(/^\/api\/projects\/\d+$/) && method === "DELETE") {
        const id = Number(last(path.split("/")));
        db.projects = db.projects.filter((item) => item.project_id !== id);
        return json({ status: "deleted" });
      }
      if (path.match(/^\/api\/projects\/\d+\/set-draft$/) && method === "PUT") {
        const id = Number(path.split("/")[3]);
        const project = db.projects.find((item) => item.project_id === id);
        project.default_draft = Boolean(body.draft);
        return json(project);
      }
      if (path.match(/^\/api\/projects\/[^/]+\/access-requests$/) && method === "POST") {
        const ref = path.split("/")[3];
        const request = {
          request_id: db.myProjectAccessRequests.length + 1,
          project_id: 0,
          project_prefix: ref,
          project_title: "",
          user_id: "user-1",
          username: "admin",
          message: body.message || "",
          status: "pending",
          created_at: "now",
          updated_at: "now",
        };
        db.myProjectAccessRequests.unshift(request);
        return json(request, 201);
      }
      if (path === "/api/users/me/access-requests" && method === "GET") {
        return json(db.myProjectAccessRequests);
      }
      if (path === "/api/users/me/notifications" && method === "GET") {
        return json(db.myNotifications);
      }
      if (path.match(/^\/api\/users\/me\/notifications\/\d+\/read$/) && method === "POST") {
        const notificationID = Number(path.split("/")[5]);
        const notification = db.myNotifications.find((item) => item.notification_id === notificationID);
        if (!notification) {
          return json({ error: "not found" }, 404);
        }
        notification.status = "read";
        return json(notification);
      }
      if (path.match(/^\/api\/projects\/[^/]+\/history$/) && method === "GET") {
        const ref = path.split("/")[3];
        return json(db.projectHistoryByProject[String(ref)] || []);
      }
      if (path.match(/^\/api\/projects\/\d+\/goals$/) && method === "GET") {
        const id = Number(path.split("/")[3]);
        return json(db.goals.filter((goal) => goal.project_id === id));
      }
      if (path.match(/^\/api\/projects\/\d+\/goal-inbox$/) && method === "GET") {
        const id = Number(path.split("/")[3]);
        const statusFilter = String(url.searchParams.get("status") || "").trim();
        const sort = String(url.searchParams.get("sort") || "updated_desc").trim();
        let goals = db.goals.filter((goal) => goal.project_id === id);
        if (statusFilter) {
          goals = goals.filter((goal) => goal.status === statusFilter);
        }
        if (sort === "priority_asc") {
          goals = goals.slice().sort((a, b) => Number(a.priority || 0) - Number(b.priority || 0));
        } else if (sort === "status") {
          const rank = { refining: 0, draft: 1, ready: 2 };
          goals = goals.slice().sort((a, b) => (rank[a.status] ?? 9) - (rank[b.status] ?? 9));
        } else {
          goals = goals.slice().reverse();
        }
        return json(goals.map((goal) => ({
          goal_id: goal.goal_id,
          project_id: goal.project_id,
          title: goal.title,
          status: goal.status,
          priority: goal.priority,
          updated_at: "now",
          refinement_confirmed: Boolean(goal.refinement_confirmed),
          decomposition_depth: String(goal.decomposition || "").split(/\r?\n/).filter((line) => String(line).trim()).length,
          unresolved_clarifications: 0,
        })));
      }
      if (path.match(/^\/api\/projects\/\d+\/goals$/) && method === "POST") {
        const projectID = Number(path.split("/")[3]);
        const goal = {
          goal_id: db.nextGoalID++,
          project_id: projectID,
          title: body.title || "",
          description: body.description || "",
          notes: body.notes || "",
          eta: body.eta || "",
          priority: Number(body.priority || 1),
          status: "draft",
          refined_goal: "",
          decomposition: "",
        };
        db.goals.push(goal);
        return json(goal, 201);
      }
      if (path.match(/^\/api\/projects\/\d+\/documents$/) && method === "GET") {
        const id = Number(path.split("/")[3]);
        return json(db.documents.filter((documentItem) => documentItem.project_id === id));
      }
      if (path.match(/^\/api\/projects\/\d+\/documents$/) && method === "POST") {
        const projectID = Number(path.split("/")[3]);
        const documentItem = {
          document_id: db.nextDocumentID++,
          project_id: projectID,
          title: body.title || "",
          description: body.description || "",
          notes: body.notes || "",
          content: body.content || "",
          created_at: "now",
          updated_at: "now",
        };
        db.documents.push(documentItem);
        return json(documentItem, 201);
      }
      if (path.match(/^\/api\/documents\/\d+$/) && method === "GET") {
        const documentID = Number(path.split("/")[3]);
        const documentItem = db.documents.find((item) => item.document_id === documentID);
        if (!documentItem) {
          return json({ error: "not found" }, 404);
        }
        return json(documentItem);
      }
      if (path.match(/^\/api\/documents\/\d+$/) && method === "PUT") {
        const documentID = Number(path.split("/")[3]);
        const documentItem = db.documents.find((item) => item.document_id === documentID);
        if (!documentItem) {
          return json({ error: "not found" }, 404);
        }
        Object.assign(documentItem, body, { updated_at: "now" });
        return json(documentItem);
      }
      if (path.match(/^\/api\/documents\/\d+$/) && method === "DELETE") {
        const documentID = Number(path.split("/")[3]);
        db.documents = db.documents.filter((item) => item.document_id !== documentID);
        delete db.documentFilesByDocument[String(documentID)];
        return new Response(null, { status: 204 });
      }
      if (path.match(/^\/api\/documents\/\d+\/files$/) && method === "GET") {
        const documentID = Number(path.split("/")[3]);
        const files = db.documentFilesByDocument[String(documentID)] || [];
        return json(files.map(({ content, ...rest }) => rest));
      }
      if (path.match(/^\/api\/documents\/\d+\/files$/) && method === "POST") {
        const documentID = Number(path.split("/")[3]);
        const file = {
          file_id: db.nextDocumentFileID++,
          document_id: documentID,
          file_name: body.file_name || "upload.bin",
          content_type: body.content_type || "application/octet-stream",
          size_bytes: body.content ? body.content.length : 0,
          content: body.content || "",
          created_at: "now",
        };
        if (!db.documentFilesByDocument[String(documentID)]) {
          db.documentFilesByDocument[String(documentID)] = [];
        }
        db.documentFilesByDocument[String(documentID)].push(file);
        const { content, ...response } = file;
        return json(response, 201);
      }
      if (path.match(/^\/api\/documents\/\d+\/files\/\d+$/) && method === "GET") {
        const parts = path.split("/");
        const documentID = Number(parts[3]);
        const fileID = Number(parts[5]);
        const file = (db.documentFilesByDocument[String(documentID)] || []).find((item) => item.file_id === fileID);
        if (!file) {
          return json({ error: "not found" }, 404);
        }
        const binary = atob(file.content || "");
        return new Response(binary, {
          status: 200,
          headers: {
            "Content-Type": file.content_type || "application/octet-stream",
            "Content-Disposition": `attachment; filename="${file.file_name}"`,
          },
        });
      }
      if (path.match(/^\/api\/documents\/\d+\/files\/\d+$/) && method === "DELETE") {
        const parts = path.split("/");
        const documentID = Number(parts[3]);
        const fileID = Number(parts[5]);
        db.documentFilesByDocument[String(documentID)] = (db.documentFilesByDocument[String(documentID)] || []).filter((item) => item.file_id !== fileID);
        return new Response(null, { status: 204 });
      }
      if (path.match(/^\/api\/goals\/\d+$/) && method === "GET") {
        const goalID = Number(path.split("/")[3]);
        const goal = db.goals.find((item) => item.goal_id === goalID);
        if (!goal) {
          return json({ error: "not found" }, 404);
        }
        return json(goal);
      }
      if (path.match(/^\/api\/goals\/\d+$/) && method === "PUT") {
        const goalID = Number(path.split("/")[3]);
        const goal = db.goals.find((item) => item.goal_id === goalID);
        if (!goal) {
          return json({ error: "not found" }, 404);
        }
        Object.assign(goal, body);
        return json(goal);
      }
      if (path.match(/^\/api\/goals\/\d+$/) && method === "DELETE") {
        const goalID = Number(path.split("/")[3]);
        db.goals = db.goals.filter((item) => item.goal_id !== goalID);
        delete db.goalChatByGoal[String(goalID)];
        return json({ status: "deleted" });
      }
      if (path.match(/^\/api\/goals\/\d+\/refine$/) && method === "POST") {
        const goalID = Number(path.split("/")[3]);
        const goal = db.goals.find((item) => item.goal_id === goalID);
        if (!goal) {
          return json({ error: "not found" }, 404);
        }
        goal.status = "refining";
        return json(goal);
      }
      if (path.match(/^\/api\/goals\/\d+\/ready$/) && method === "POST") {
        const goalID = Number(path.split("/")[3]);
        const goal = db.goals.find((item) => item.goal_id === goalID);
        if (!goal) {
          return json({ error: "not found" }, 404);
        }
        if (!body.confirm_refinement) {
          return json({ error: "confirm_refinement must be true before setting ready" }, 400);
        }
        if (!String(goal.refined_goal || "").trim()) {
          return json({ error: "goal refined_goal is required before setting ready" }, 400);
        }
        if (!String(goal.decomposition || "").trim()) {
          return json({ error: "goal decomposition is required before setting ready" }, 400);
        }
        goal.status = "ready";
        return json(goal);
      }
      if (path.match(/^\/api\/goals\/\d+\/refinement$/) && method === "GET") {
        const goalID = Number(path.split("/")[3]);
        const goal = db.goals.find((item) => item.goal_id === goalID);
        if (!goal) {
          return json({ error: "not found" }, 404);
        }
        return json({ refined_goal: goal.refined_goal || "", decomposition: goal.decomposition || "" });
      }
      if (path.match(/^\/api\/goals\/\d+\/refinement$/) && method === "PUT") {
        const goalID = Number(path.split("/")[3]);
        const goal = db.goals.find((item) => item.goal_id === goalID);
        if (!goal) {
          return json({ error: "not found" }, 404);
        }
        goal.refined_goal = body.refined_goal || "";
        goal.decomposition = body.decomposition || "";
        return json(goal);
      }
      if (path.match(/^\/api\/goals\/\d+\/chat\/messages$/) && method === "GET") {
        const goalID = Number(path.split("/")[3]);
        return json((db.goalChatByGoal[String(goalID)] || []).map((message, index) => ({
          message_id: index + 1,
          goal_id: goalID,
          author: message.author,
          text: message.text,
        })));
      }
      if (path.match(/^\/api\/goals\/\d+\/chat\/messages$/) && method === "POST") {
        const goalID = Number(path.split("/")[3]);
        const key = String(goalID);
        if (!db.goalChatByGoal[key]) {
          db.goalChatByGoal[key] = [];
        }
        db.goalChatByGoal[key].push({ author: body.author || "user", text: body.text || "" });
        return json({ message_id: db.goalChatByGoal[key].length, goal_id: goalID, author: body.author || "user", text: body.text || "" }, 201);
      }
      if (path.match(/^\/api\/projects\/\d+\/tickets$/) && method === "GET") {
        const id = Number(path.split("/")[3]);
        return json(db.tickets.filter((ticket) => ticket.project_id === id));
      }
      if (path.match(/^\/api\/projects\/\d+\/interventions$/) && method === "GET") {
        const id = Number(path.split("/")[3]);
        return json(db.tickets.filter((ticket) => ticket.project_id === id && String(ticket.state || "").toLowerCase() === "fail"));
      }
      if (path.match(/^\/api\/projects\/\d+\/labels$/) && method === "GET") {
        const id = Number(path.split("/")[3]);
        return json(db.labels.filter((label) => label.project_id === id));
      }
      if (path === "/api/workflows" && method === "GET") {
        return json(db.workflows.map(({ stages, ...workflow }) => workflow));
      }
      if (path === "/api/workflows" && method === "POST") {
        const workflow = {
          workflow_id: db.workflows.reduce((max, item) => Math.max(max, Number(item.workflow_id || 0)), 0) + 1,
          name: body.name || "",
          description: body.description || "",
          approval_policy: body.approval_policy || "single_role",
          progression_mode: body.progression_mode || "linear",
          stages: [],
        };
        db.workflows.push(workflow);
        return json(workflow, 201);
      }
      if (path.match(/^\/api\/workflows\/\d+$/) && method === "GET") {
        const id = Number(last(path.split("/")));
        return json(db.workflows.find((item) => item.workflow_id === id));
      }
      if (path.match(/^\/api\/workflows\/\d+$/) && method === "PUT") {
        const id = Number(last(path.split("/")));
        const workflow = db.workflows.find((item) => item.workflow_id === id);
        if (!workflow) {
          return json({ error: "not found" }, 404);
        }
        workflow.name = body.name || workflow.name;
        workflow.description = body.description || "";
        workflow.approval_policy = body.approval_policy || "single_role";
        workflow.progression_mode = body.progression_mode || "linear";
        return json(workflow);
      }
      if (path.match(/^\/api\/workflows\/\d+\/stages$/) && method === "POST") {
        const workflowID = Number(path.split("/")[3]);
        const workflow = db.workflows.find((item) => item.workflow_id === workflowID);
        const nextStageID = workflow.stages.reduce((max, stage) => Math.max(max, Number(stage.workflow_stage_id || 0)), 0) + 1;
        const stage = {
          workflow_stage_id: nextStageID,
          workflow_id: workflowID,
          stage_name: body.stage_name || "",
          description: body.wow || "",
          definition_of_ready: body.dor || "",
          definition_of_done: body.dod || "",
          roles: [],
          next_stage_ids: [],
          sort_order: Number(body.sort_order || workflow.stages.length),
        };
        workflow.stages.push(stage);
        return json(stage, 201);
      }
      if (path.match(/^\/api\/workflows\/stages\/\d+$/) && method === "PUT") {
        const stageID = Number(path.split("/")[4]);
        const workflow = db.workflows.find((item) => item.stages.some((stage) => stage.workflow_stage_id === stageID));
        const stage = workflow?.stages.find((item) => item.workflow_stage_id === stageID);
        if (!stage) {
          return json({ error: "not found" }, 404);
        }
        stage.stage_name = body.stage_name || stage.stage_name;
        stage.description = body.wow || "";
        stage.definition_of_ready = body.dor || "";
        stage.definition_of_done = body.dod || "";
        return json(stage);
      }
      if (path.match(/^\/api\/workflows\/stages\/\d+\/transitions$/) && method === "PUT") {
        const stageID = Number(path.split("/")[4]);
        const workflow = db.workflows.find((item) => item.stages.some((stage) => stage.workflow_stage_id === stageID));
        const stage = workflow?.stages.find((item) => item.workflow_stage_id === stageID);
        if (!stage) {
          return json({ error: "not found" }, 404);
        }
        stage.next_stage_ids = Array.isArray(body.to_stage_ids) ? body.to_stage_ids.map(Number) : [];
        return json(stage);
      }
      if (path === "/api/roles" && method === "GET") {
        return json(db.roles);
      }
      if (path === "/api/agents" && method === "GET") {
        return json(db.agents);
      }
      if (path === "/api/teams" && method === "GET") {
        return json(db.teams);
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/history$/) && method === "GET") {
        return json([{ action: "created", created_at: "now", comment: "" }]);
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/comments$/) && method === "GET") {
        const id = path.split("/")[3];
        const comments = db.commentsByTicket[id] || [];
        return json([...comments].reverse());
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/comments$/) && method === "POST") {
        const id = path.split("/")[3];
        if (!db.commentsByTicket[id]) {
          db.commentsByTicket[id] = [];
        }
        const comment = {
          author: db.status.username,
          text: body.comment || "",
          date: "now",
        };
        db.commentsByTicket[id].push(comment);
        return json(comment, 201);
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/labels$/) && method === "GET") {
        const id = path.split("/")[3];
        const ids = db.ticketLabelIDs[id] || [];
        const labelsByID = new Map(db.labels.map((label) => [label.label_id, label]));
        return json(ids.map((labelID) => labelsByID.get(labelID)).filter(Boolean));
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/labels$/) && method === "POST") {
        const id = path.split("/")[3];
        const labelID = Number(body.label_id);
        if (!db.ticketLabelIDs[id]) {
          db.ticketLabelIDs[id] = [];
        }
        if (!db.ticketLabelIDs[id].includes(labelID)) {
          db.ticketLabelIDs[id].push(labelID);
        }
        return json({ status: "added" });
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/labels\/\d+$/) && method === "DELETE") {
        const parts = path.split("/");
        const id = parts[3];
        const labelID = Number(parts[5]);
        db.ticketLabelIDs[id] = (db.ticketLabelIDs[id] || []).filter((item) => item !== labelID);
        return json({ status: "removed" });
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/dependencies$/) && method === "GET") {
        const id = path.split("/")[3];
        return json(db.dependenciesByTicket[id] || []);
      }
      if (path === "/api/dependencies" && method === "POST") {
        if (!db.dependenciesByTicket[body.ticket_id]) {
          db.dependenciesByTicket[body.ticket_id] = [];
        }
        const dependency = {
          id: db.nextDependencyID++,
          project_id: Number(body.project_id),
          ticket_id: body.ticket_id,
          depends_on: body.depends_on,
          created_by: db.status.username,
          created_at: "now",
        };
        db.dependenciesByTicket[body.ticket_id].push(dependency);
        return json(dependency, 201);
      }
      if (path === "/api/dependencies" && method === "DELETE") {
        const ticketID = url.searchParams.get("ticket_id");
        const dependsOn = url.searchParams.get("depends_on");
        db.dependenciesByTicket[ticketID] = (db.dependenciesByTicket[ticketID] || []).filter((item) => item.depends_on !== dependsOn);
        return json({ status: "deleted" });
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/time$/) && method === "GET") {
        const id = path.split("/")[3];
        return json(db.timeEntriesByTicket[id] || []);
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/time\/total$/) && method === "GET") {
        const id = path.split("/")[3];
        const total = (db.timeEntriesByTicket[id] || []).reduce((sum, entry) => sum + Number(entry.minutes || 0), 0);
        return json({ total });
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/time$/) && method === "POST") {
        const id = path.split("/")[3];
        if (!db.timeEntriesByTicket[id]) {
          db.timeEntriesByTicket[id] = [];
        }
        const entry = {
          time_entry_id: db.nextTimeEntryID++,
          ticket_id: id,
          user_id: db.status.username,
          minutes: Number(body.minutes || 0),
          note: body.note || "",
          created_at: "now",
        };
        db.timeEntriesByTicket[id].push(entry);
        return json(entry, 201);
      }
      if (path.match(/^\/api\/tickets\/[^/]+$/) && method === "PUT") {
        const id = last(path.split("/"));
        const ticket = db.tickets.find((item) => item.ticket_id === id);
        if (!ticket) {
          return json({ error: "not found" }, 404);
        }
        Object.assign(ticket, body);
        return json(ticket);
      }
      if (path === "/api/tickets" && method === "POST") {
        const ticket = Object.assign(
          {
            ticket_id: "OPS-999",
            archived: false,
            draft: false,
            workflow_id: null,
          },
          body,
        );
        db.tickets.push(ticket);
        return json(ticket, 201);
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/(draft|undraft|open|close|archive|unarchive)$/) && method === "POST") {
        return json({ status: "ok" });
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/intervene$/) && method === "POST") {
        const id = path.split("/")[3];
        const ticket = db.tickets.find((item) => item.ticket_id === id);
        if (!ticket) {
          return json({ error: "not found" }, 404);
        }
        const outcome = String(body.outcome || "").toLowerCase();
        if (outcome === "cancel") {
          ticket.archived = true;
        } else {
          ticket.state = "idle";
        }
        let followUp = null;
        if (outcome === "split-work") {
          followUp = {
            ticket_id: "OPS-" + String(1000 + db.tickets.length),
            project_id: ticket.project_id,
            type: "task",
            title: "Follow-up: " + (ticket.title || "work"),
            description: body.message || "",
            acceptance_criteria: "",
            status: "open",
            stage: ticket.stage || "design",
            state: "idle",
            priority: ticket.priority || 1,
            order: 0,
            estimate_effort: 0,
            health_score: 0,
            draft: false,
            archived: false,
            workflow_id: ticket.workflow_id || null,
          };
          db.tickets.push(followUp);
        }
        return json({ ticket, follow_up: followUp, decision: outcome, intervention: true });
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/workflow$/) && (method === "POST" || method === "DELETE")) {
        return json({ status: "ok" });
      }
      if (path.match(/^\/api\/workflows\/\d+\/reorder$/) && method === "PUT") {
        const workflow = db.workflows.find((item) => item.workflow_id === Number(path.split("/")[3]));
        const stagesByID = new Map(workflow.stages.map((stage) => [stage.workflow_stage_id, stage]));
        workflow.stages = body.stage_ids.map((id) => stagesByID.get(id)).filter(Boolean);
        return json({ status: "reordered" });
      }
      if (path.match(/^\/api\/workflows\/stages\/roles\/\d+\/\d+$/) && method === "POST") {
        const parts = path.split("/");
        const workflow = db.workflows.find((item) => item.workflow_id === Number(parts[5]));
        const stage = workflow.stages.find((item) => item.workflow_stage_id === Number(parts[6]));
        const role = db.roles.find((item) => item.role_id === Number(body.role_id));
        stage.roles.push({ role_id: role.role_id, title: role.title });
        return json({ status: "created" }, 201);
      }
      if (path.match(/^\/api\/workflows\/stages\/roles\/\d+\/\d+$/) && method === "PUT") {
        const parts = path.split("/");
        const workflow = db.workflows.find((item) => item.workflow_id === Number(parts[5]));
        const stage = workflow.stages.find((item) => item.workflow_stage_id === Number(parts[6]));
        const rolesByID = new Map(db.roles.map((item) => [item.role_id, item]));
        stage.roles = body.role_ids.map((id) => {
          const role = rolesByID.get(id);
          return { role_id: role.role_id, title: role.title };
        });
        return json({ status: "reordered" });
      }
      if (path.match(/^\/api\/workflows\/stages\/roles\/\d+\/\d+\/\d+$/) && method === "DELETE") {
        const parts = path.split("/");
        const workflow = db.workflows.find((item) => item.workflow_id === Number(parts[5]));
        const stage = workflow.stages.find((item) => item.workflow_stage_id === Number(parts[6]));
        const roleID = Number(parts[7]);
        stage.roles = (stage.roles || []).filter((item) => Number(item.role_id) !== roleID);
        return json({ status: "removed" });
      }

      return json({ error: `Unhandled ${method} ${path}` }, 500);
    };
  }, seed);
}

test("focuses the username field on first load", async ({ page }) => {
  await installSite2Mock(page);
  await page.goto("/site2/");
  await page.evaluate(() => {
    sessionStorage.clear();
    localStorage.clear();
    window.location.reload();
  });
  await expect(page.locator("#login-username")).toBeVisible();
  await expect.poll(() => page.evaluate(() => document.activeElement && document.activeElement.id)).toBe("login-username");
});

test("shows the server version on the login screen", async ({ page }) => {
  await installSite2Mock(page, { status: { authenticated: false, server_version: "1.2.3" } });
  await page.goto("/site2/");
  await page.evaluate(() => {
    sessionStorage.clear();
    localStorage.clear();
    window.location.reload();
  });

  await expect(page.locator("#version-overlay")).toBeVisible();
  await expect(page.locator("#version-overlay")).toHaveText("server: 1.2.3");
});

test("login and register forms do not fall back to raw auth endpoint posts", async ({ page }) => {
  await installSite2Mock(page, { status: { authenticated: false } });
  await page.goto("/site2/");
  await page.evaluate(() => {
    sessionStorage.clear();
    localStorage.clear();
    window.location.reload();
  });

  await expect(page.locator("#login-form")).not.toHaveAttribute("action", /\/api\/login$/);
  await expect(page.locator("#register-form")).not.toHaveAttribute("action", /\/api\/register$/);
});

test("admin settings view supports config key CRUD", async ({ page }) => {
  await installSite2Mock(page);
  await page.goto("/site2/");
  await page.evaluate(() => {
    sessionStorage.clear();
    localStorage.clear();
    window.location.reload();
  });

  await page.locator("#login-username").fill("admin");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: "Sign in" }).click();

  await page.getByRole("button", { name: "Config" }).click();
  await expect(page.getByRole("heading", { name: "Configuration registry" })).toBeVisible();
  await expect(page.locator("#config-settings-list")).toContainText("chat_enabled");

  await page.getByRole("button", { name: "New key" }).click();
  await page.locator("#config-setting-key").fill("feature.flag");
  await page.locator("#config-setting-value").fill("on");
  await page.getByRole("button", { name: "Save setting" }).click();
  await expect(page.locator("#config-settings-list")).toContainText("feature.flag");

  await page.locator("[data-config-setting-key='feature.flag']").click();
  await page.locator("#config-setting-key").fill("feature.flag.renamed");
  await page.locator("#config-setting-value").fill("off");
  await page.getByRole("button", { name: "Save setting" }).click();
  await expect(page.locator("#config-settings-list")).toContainText("feature.flag.renamed");

  await page.evaluate(() => { window._origUiConfirm = window.uiConfirm; window.uiConfirm = async () => true; });
  await page.getByRole("button", { name: "Delete" }).click();
  await page.evaluate(() => { if (window._origUiConfirm) window.uiConfirm = window._origUiConfirm; });
  await expect(page.locator("#config-settings-list")).not.toContainText("feature.flag.renamed");
});

test("does not emit CSP inline-style violations after login", async ({ page }) => {
  const messages = [];
  page.on("console", (message) => messages.push(message.text()));

  await installSite2Mock(page);
  await page.goto("/site2/");
  await page.evaluate(() => {
    sessionStorage.clear();
    localStorage.clear();
    window.location.reload();
  });
  await page.locator("#login-username").fill("admin");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();

  const cspMessages = messages.filter((text) => text.includes("Applying inline style violates the following Content Security Policy directive"));
  expect(cspMessages).toEqual([]);
});

test("logs in without leaking credentials into URL query parameters", async ({ page }) => {
  await installSite2Mock(page);
  await page.goto("/site2/");
  await page.evaluate(() => {
    sessionStorage.clear();
    localStorage.clear();
    window.location.reload();
  });

  await page.locator("#login-username").fill("admin");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: "Sign in" }).click();

  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();
  await expect(page).toHaveURL(/\/site2\/?$/);
  await expect(page).not.toHaveURL(/username=|password=/);

  const requests = await page.evaluate(() => window.__site2Requests || []);
  const loginRequest = requests.find((request) => request.path === "/api/login");
  expect(loginRequest).toBeTruthy();
  expect(loginRequest.method).toBe("POST");
  expect(loginRequest.body).toEqual(expect.objectContaining({
    username: "admin",
    password: "secret",
  }));
});

test("passkey login signs in from the website without posting a password", async ({ page }) => {
  await installSite2Mock(page);
  await page.addInitScript(() => {
    window.PublicKeyCredential = window.PublicKeyCredential || function PublicKeyCredential() {};
    window.PublicKeyCredential.parseRequestOptionsFromJSON = (value) => value;
    Object.defineProperty(navigator, "credentials", {
      configurable: true,
      value: {
        get: async () => ({
          id: "cred-1",
          rawId: Uint8Array.from([99, 114, 101, 100, 45, 49]).buffer,
          type: "public-key",
          authenticatorAttachment: "platform",
          response: {
            clientDataJSON: Uint8Array.from([1, 2, 3]).buffer,
            authenticatorData: Uint8Array.from([4, 5, 6]).buffer,
            signature: Uint8Array.from([7, 8, 9]).buffer,
          },
          getClientExtensionResults: () => ({}),
        }),
      },
    });
  });
  await page.goto("/site2/");

  await page.locator("#login-username").fill("admin");
  await page.getByRole("button", { name: "Use passkey" }).click();

  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();
  const requests = await page.evaluate(() => window.__site2Requests || []);
  expect(requests.find((request) => request.path === "/api/login")).toBeFalsy();
  expect(requests.find((request) => request.path === "/api/auth/passkey/login/start")).toBeTruthy();
  expect(requests.find((request) => request.path === "/api/auth/passkey/challenge")).toBeTruthy();
  expect(requests.find((request) => request.path === "/api/auth/passkey/finish")).toBeTruthy();
  expect(requests.find((request) => request.path === "/api/auth/passkey/poll")).toBeTruthy();
});

test("account settings modal manages website passkeys", async ({ page }) => {
  await installSite2Mock(page, {
    passkeys: [
      { credential_id: "cred-old", name: "Laptop", created_at: "now", updated_at: "now", last_used_at: "" },
    ],
  });
  await page.addInitScript(() => {
    window.PublicKeyCredential = window.PublicKeyCredential || function PublicKeyCredential() {};
    window.PublicKeyCredential.parseCreationOptionsFromJSON = (value) => value;
    Object.defineProperty(navigator, "credentials", {
      configurable: true,
      value: {
        create: async () => ({
          id: "cred-new",
          rawId: Uint8Array.from([99, 114, 101, 100, 45, 110, 101, 119]).buffer,
          type: "public-key",
          authenticatorAttachment: "platform",
          response: {
            clientDataJSON: Uint8Array.from([1, 2, 3]).buffer,
            attestationObject: Uint8Array.from([4, 5, 6]).buffer,
          },
          getClientExtensionResults: () => ({}),
        }),
      },
    });
  });
  await page.goto("/site2/");
  await page.locator("#login-username").fill("admin");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();

  await page.locator("#account-menu-button").click();
  await page.getByRole("button", { name: "Account settings" }).click();
  await expect(page.getByRole("heading", { name: "Account settings" })).toBeVisible();
  await expect(page.getByRole("button", { name: "Open app settings" })).toBeVisible();
  await expect(page.locator("#account-passkey-list")).toContainText("Laptop");

  await page.locator("[data-passkey-id='cred-old'] [data-passkey-action='rename']").click();
  await page.locator("#dialog-input").fill("Desk key");
  await page.getByRole("button", { name: "Save" }).click();
  await expect(page.locator("#account-passkey-list")).toContainText("Desk key");

  await page.locator("#account-passkey-name").fill("Phone");
  await page.getByRole("button", { name: "Enroll passkey" }).click();
  await expect(page.locator("#account-passkey-list")).toContainText("Phone");

  await page.locator("[data-passkey-id='cred-old'] [data-passkey-action='delete']").click();
  await page.getByRole("button", { name: "Delete" }).click();
  await expect(page.locator("#account-passkey-list")).not.toContainText("Desk key");

  const requests = await page.evaluate(() => window.__site2Requests || []);
  expect(requests.find((request) => request.path === "/api/users/me/passkeys" && request.method === "GET")).toBeTruthy();
  expect(requests.find((request) => request.path === "/api/users/me/passkeys/cred-old" && request.method === "PUT")).toBeTruthy();
  expect(requests.find((request) => request.path === "/api/users/me/passkeys/cred-old" && request.method === "DELETE")).toBeTruthy();
  expect(requests.find((request) => request.path === "/api/auth/passkey/register/start")).toBeTruthy();
  expect(requests.find((request) => request.path === "/api/auth/passkey/finish")).toBeTruthy();
});

test("continues to board when goals API returns null", async ({ page }) => {
  await installSite2Mock(page);
  await page.goto("/site2/");
  await page.evaluate(() => {
    const original = window.__site2MockFetch;
    window.__site2MockFetch = async (input, init = {}) => {
      const method = (init.method || "GET").toUpperCase();
      const url = new URL(typeof input === "string" ? input : input.url, window.location.origin);
      if (method === "GET" && /^\/api\/projects\/\d+\/goals$/.test(url.pathname)) {
        return new Response("null", { status: 200, headers: { "Content-Type": "application/json" } });
      }
      return original(input, init);
    };
    sessionStorage.clear();
    localStorage.clear();
    window.location.reload();
  });

  await page.locator("#login-username").fill("admin");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();
});

test("refresh restores the app from the server session when client auth storage is missing", async ({ page }) => {
  await installSite2Mock(page);
  await page.goto("/site2/");
  await page.evaluate(() => {
    sessionStorage.clear();
    localStorage.clear();
    window.location.reload();
  });

  await page.locator("#login-username").fill("admin");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();

  await page.evaluate(() => {
    sessionStorage.removeItem("site2.auth");
    window.location.reload();
  });

  await expect(page.locator("#app-shell")).not.toHaveClass(/hidden/);
  await expect(page.locator("#login-screen")).toHaveClass(/hidden/);
  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();
});

test("refresh keeps the app shell visible when a non-auth boot request fails", async ({ page }) => {
  await installSite2Mock(page);
  await page.goto("/site2/");
  await page.evaluate(() => {
    sessionStorage.clear();
    localStorage.clear();
    window.location.reload();
  });

  await page.locator("#login-username").fill("admin");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();

  await page.evaluate(() => {
    sessionStorage.setItem("site2.failPlansOnBoot", "1");
    window.location.reload();
  });

  await expect(page.locator("#app-shell")).not.toHaveClass(/hidden/);
  await expect(page.locator("#login-screen")).toHaveClass(/hidden/);
  await expect(page.locator("#app-notice")).toContainText("plans unavailable");
});

test("goal refine -> decomposition reorder -> ready flow works in site2", async ({ page }) => {
  await installSite2Mock(page);
  await page.goto("/site2/");
  await page.evaluate(() => {
    sessionStorage.clear();
    localStorage.clear();
    window.location.reload();
  });

  await page.locator("#login-username").fill("admin");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();

  await page.getByRole("button", { name: "Goals" }).click();
  await expect(page.getByRole("heading", { name: "Goals" })).toBeVisible();

  await page.locator("#new-goal-button").click();
  await page.locator("#goal-title").fill("Realtime todo app");
  await page.locator("#save-goal-button").click();
  await expect(page.locator("#goal-status")).toHaveValue("draft");

  await page.locator("#refine-goal-button").click();
  await expect(page.locator("#goal-status")).toHaveValue("refining");

  await page.locator("#goal-refined-goal").fill("Build a multiplayer todo app with login.");
  await page.locator("#goal-decomposition").fill("1. Epic: Auth\n2. Epic: Realtime sync\n3. Story: Presence");
  await page.locator('[data-decomposition-up="1"]').click();
  await page.locator("#save-goal-refinement-button").click();
  await page.locator("#ready-goal-button").click();
  await expect(page.locator("#goal-status")).toHaveValue("ready");

  const requests = await page.evaluate(() => window.__site2Requests || []);
  const refinementRequest = requests
    .filter((request) => request.method === "PUT" && /\/api\/goals\/\d+\/refinement$/.test(request.path))
    .slice(-1)[0];
  expect(refinementRequest).toBeTruthy();
  expect(refinementRequest.body.decomposition.startsWith("1. Epic: Realtime sync")).toBe(true);
});

test("keeps the session and visible tickets across refresh", async ({ page }) => {
  await installSite2Mock(page);
  await page.goto("/site2/");
  await page.evaluate(() => {
    sessionStorage.clear();
    localStorage.clear();
    window.location.reload();
  });
  await page.locator("#login-username").fill("admin");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.locator("#ticket-board")).toContainText("Move me");

  await page.reload();

  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();
  await expect(page.locator("#ticket-board")).toContainText("Move me");
  await expect(page.locator("#login-screen")).toHaveClass(/hidden/);
});

test("remembers the selected project from localStorage after reload", async ({ page }) => {
  await installSite2Mock(page, {
    nextProjectID: 3,
    projects: [
      {
        project_id: 1,
        prefix: "OPS",
        title: "Operations",
        description: "Keep things running",
        acceptance_criteria: "",
        git_repository: "acme/ops",
        visibility: "public",
        workflow_id: 1,
        default_draft: false,
      },
      {
        project_id: 2,
        prefix: "WEB",
        title: "Website",
        description: "Customer web experience",
        acceptance_criteria: "",
        git_repository: "acme/web",
        visibility: "public",
        workflow_id: 1,
        default_draft: false,
      },
    ],
  });
  await page.goto("/site2/");
  await page.evaluate(() => {
    sessionStorage.clear();
    localStorage.clear();
    window.location.reload();
  });
  await page.locator("#login-username").fill("admin");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();

  await page.locator("#project-menu-button").click();
  await page.locator('[data-project-switch="2"]').click();
  await expect(page.locator("#project-menu-button")).toHaveText("Website (WEB)");
  await expect.poll(() => page.evaluate(() => localStorage.getItem("site2.selectedProjectID"))).toBe("2");

  await page.reload();

  await expect(page.locator("#project-menu-button")).toHaveText("Website (WEB)");
});

test("remembers active panel and scroll position after refresh", async ({ page }) => {
  await installSite2Mock(page);
  await page.goto("/site2/");
  await page.evaluate(() => {
    sessionStorage.clear();
    localStorage.clear();
    window.location.reload();
  });
  await page.locator("#login-username").fill("admin");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();

  await page.getByRole("button", { name: "Projects" }).click();
  await page.evaluate(() => {
    document.body.style.minHeight = "3000px";
    window.scrollTo(0, 420);
    window.dispatchEvent(new Event("scroll"));
  });

  await expect.poll(() => page.evaluate(() => localStorage.getItem("site2.selectedView"))).toBe("projects");
  await expect.poll(() => page.evaluate(() => {
    const raw = localStorage.getItem("site2.viewScroll");
    if (!raw) {
      return 0;
    }
    const parsed = JSON.parse(raw);
    return Number(parsed.projects || 0);
  })).toBeGreaterThan(300);

  await page.reload();

  await expect(page.locator('.nav button[data-view="projects"]')).toHaveClass(/active/);
});

test.beforeEach(async ({ page }) => {
  await installSite2Mock(page);
  await page.goto("/site2/");
  await page.evaluate(() => {
    localStorage.clear();
  });
  await page.locator("#login-username").fill("admin");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: "Sign in", exact: true }).click();
  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();
});

test("creates a project and persists default draft settings through the existing API", async ({ page }) => {
  await page.getByRole("button", { name: "Projects" }).click();
  await page.getByRole("button", { name: "New project" }).click();
  await expect(page.locator("#project-prefix")).toHaveAttribute("maxlength", "5");
  await expect(page.locator("#project-prefix")).toHaveAttribute("pattern", "[A-Z]{1,5}");
  await page.locator("#project-prefix").fill("WEB");
  await page.locator("#project-title").fill("Website");
  await page.locator("#project-default-draft").selectOption("true");
  await page.getByRole("button", { name: "Save project" }).click();

  await expect(page.locator("#project-list")).toContainText("Website");

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ method: "POST", path: "/api/projects", body: expect.objectContaining({ prefix: "WEB", title: "Website" }) }),
      expect.objectContaining({ method: "PUT", path: "/api/projects/2/set-draft", body: { draft: true } }),
    ]),
  );
});

test("submits a project access request from the Projects view", async ({ page }) => {
  await page.getByRole("button", { name: "Projects" }).click();
  await page.locator("#project-request-access-ref").fill("GATE");
  await page.locator("#project-request-access-message").fill("please add me");
  await page.getByRole("button", { name: "Request access" }).click();
  await expect(page.locator("#project-my-access-requests-list")).toContainText("GATE");
  await expect(page.locator("#project-my-access-requests-list")).toContainText("please add me");
  await expect(page.locator("#project-my-access-requests-list")).toContainText("pending");

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({
        method: "POST",
        path: "/api/projects/GATE/access-requests",
        body: { message: "please add me" },
      }),
      expect.objectContaining({
        method: "GET",
        path: "/api/users/me/access-requests",
      }),
    ]),
  );
});

test("shows recent project history in the Projects view", async ({ page }) => {
  await installSite2Mock(page, {
    projectHistoryByProject: {
      OPS: [
        {
          id: 11,
          project_id: 1,
          ticket_id: "",
          ticket_key: "",
          event_type: "project_access_request_created",
          payload: JSON.stringify({
            request_id: 7,
            username: "alice",
            project_prefix: "OPS",
            message: "Need access to help with support",
          }),
          created_by: "alice-id",
          created_at: "2026-05-15T21:00:00Z",
        },
        {
          id: 12,
          project_id: 1,
          ticket_id: "",
          ticket_key: "",
          event_type: "project_access_request_approved",
          payload: JSON.stringify({
            request_id: 7,
            username: "alice",
            project_prefix: "OPS",
            decided_by: "admin",
          }),
          created_by: "admin-id",
          created_at: "2026-05-15T21:05:00Z",
        },
      ],
    },
  });
  await page.goto("/site2/");
  await page.evaluate(() => {
    sessionStorage.clear();
    localStorage.clear();
    window.location.reload();
  });
  await page.locator("#login-username").fill("admin");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();
  await page.getByRole("button", { name: "Projects" }).click();

  await expect(page.locator("#project-history-list")).toContainText("alice requested access to OPS");
  await expect(page.locator("#project-history-list")).toContainText("approved access request #7 for alice on OPS");
  await expect(page.locator("#project-history-list")).toContainText("project");

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({
        method: "GET",
        path: "/api/projects/OPS/history",
      }),
    ]),
  );
});

test("marks notifications as read from the Projects view", async ({ page }) => {
  await installSite2Mock(page, {
    myNotifications: [
      {
        notification_id: 9,
        status: "unread",
        kind: "project_access_approved",
        title: "Project access approved",
        message: "Your request for OPS (Operations) was approved by admin.",
        created_at: "2026-05-15T21:05:00Z",
      },
    ],
  });
  await page.goto("/site2/");
  await page.evaluate(() => {
    sessionStorage.clear();
    localStorage.clear();
    window.location.reload();
  });
  await page.locator("#login-username").fill("admin");
  await page.locator("#login-password").fill("secret");
  await page.getByRole("button", { name: "Sign in" }).click();
  await expect(page.getByRole("heading", { name: "Board" })).toBeVisible();
  await page.getByRole("button", { name: "Projects" }).click();

  await expect(page.locator("#project-notifications-list")).toContainText("Project access approved");
  await page.getByRole("button", { name: "Mark read" }).click();
  await expect(page.locator("#project-notifications-list")).toContainText("read");

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ method: "GET", path: "/api/users/me/notifications" }),
      expect.objectContaining({ method: "POST", path: "/api/users/me/notifications/9/read" }),
    ]),
  );
});

test("moves a ticket across the board with drag and drop", async ({ page }) => {
  await expect(page.locator('[data-lane-stage="design"]')).toContainText("Move me");
  await page.dragAndDrop('[data-ticket-id="OPS-101"]', '[data-lane-stage="done"]');
  await expect(page.locator('[data-lane-stage="done"]')).toContainText("Move me");

  const requests = await page.evaluate(() => window.__site2Requests.filter((request) => request.path === "/api/tickets/OPS-101"));
  expect(requests.some((request) => request.body.stage === "done")).toBeTruthy();
});

test("creates, updates, and deletes plans from the plans panel", async ({ page }) => {
  await page.getByRole("button", { name: "Plans" }).click();

  await expect(page.locator("#plan-list")).toContainText("Starter");

  await page.getByRole("button", { name: "New plan" }).click();
  await page.locator("#plan-slug").fill("enterprise");
  await page.locator("#plan-name").fill("Enterprise");
  await page.locator("#plan-description").fill("Expanded limits");
  await page.locator("#plan-max-projects").fill("25");
  await page.locator("#plan-auto-create-private-project").selectOption("true");
  await page.locator("#plan-form").getByRole("button", { name: "Save plan" }).click();

  await expect(page.locator("#plan-list")).toContainText("Enterprise");

  await page.locator('[data-plan-slug="enterprise"]').click();
  await page.locator("#plan-name").fill("Enterprise Plus");
  await page.locator("#plan-form").getByRole("button", { name: "Save plan" }).click();

  await expect(page.locator("#plan-list")).toContainText("Enterprise Plus");

  await page.locator("#default-plan-select").selectOption("enterprise");
  await page.getByRole("button", { name: "Save onboarding policy" }).click();

  await page.getByRole("button", { name: "Delete plan" }).click();
  await page.getByRole("button", { name: "Delete" }).click();

  await expect(page.locator("#plan-list")).not.toContainText("Enterprise Plus");

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(expect.arrayContaining([
    expect.objectContaining({ method: "POST", path: "/api/plans", body: expect.objectContaining({ slug: "enterprise", name: "Enterprise" }) }),
    expect.objectContaining({ method: "PUT", path: "/api/plans/enterprise", body: expect.objectContaining({ name: "Enterprise Plus" }) }),
    expect.objectContaining({ method: "POST", path: "/api/plans/default", body: expect.objectContaining({ slug: "enterprise" }) }),
    expect.objectContaining({ method: "DELETE", path: "/api/plans/enterprise" }),
  ]));
});

test("reorders board stages through the Workflow reorder endpoint", async ({ page }) => {
  await page.dragAndDrop('[data-workflow-stage-id="11"]', '[data-workflow-stage-id="14"]');

  const requests = await page.evaluate(() => window.__site2Requests.filter((request) => request.path === "/api/workflows/1/reorder"));
  expect(requests.length).toBeGreaterThan(0);
});

test("adds a stage from the workflows board", async ({ page }) => {
  await page.getByRole("button", { name: "Workflows" }).click();
  await page.locator("#new-stage-name").fill("review");
  await page.locator("#new-stage-wow").fill("Peer review before QA");
  await page.locator("#save-stage-button").click();

  await expect(page.locator('#stage-grid [data-stage-title]').last()).toContainText("review");

  const requests = await page.evaluate(() => window.__site2Requests.filter((request) => request.path === "/api/workflows/1/stages"));
  expect(requests.at(-1)).toEqual(expect.objectContaining({
    method: "POST",
    path: "/api/workflows/1/stages",
    body: expect.objectContaining({
      stage_name: "review",
      wow: "Peer review before QA",
      sort_order: 4,
    }),
  }));
});

test("reorders stages inside the workflows board", async ({ page }) => {
  await page.getByRole("button", { name: "Workflows" }).click();
  await page.dragAndDrop('[data-stage-id="11"]', '[data-stage-id="13"]');

  await expect.poll(async () => page.evaluate(() => (
    Array.from(document.querySelectorAll("#stage-grid [data-stage-id]"))
      .map((item) => Number(item.dataset.stageId))
  ))).toEqual([12, 11, 13, 14]);

  const requests = await page.evaluate(() => window.__site2Requests.filter((request) => request.path === "/api/workflows/1/reorder"));
  expect(requests.at(-1)?.body?.stage_ids).toEqual([12, 11, 13, 14]);
});

test("reorders stages inside the workflows board with lane buttons", async ({ page }) => {
  await page.getByRole("button", { name: "Workflows" }).click();
  await page.locator('[data-move-stage="12"][data-move-direction="left"]').click();

  await expect.poll(async () => page.evaluate(() => (
    Array.from(document.querySelectorAll("#stage-grid [data-stage-id]"))
      .map((item) => Number(item.dataset.stageId))
  ))).toEqual([12, 11, 13, 14]);

  const requests = await page.evaluate(() => window.__site2Requests.filter((request) => request.path === "/api/workflows/1/reorder"));
  expect(requests.at(-1)?.body?.stage_ids).toEqual([12, 11, 13, 14]);
});

test("workflow settings autosave changes", async ({ page }) => {
  await page.getByRole("button", { name: "Workflows" }).click();
  await page.locator("#workflow-settings summary").click();
  await page.locator("#workflow-description").fill("Ship safer changes");
  await page.locator('label[for="workflow-approval-policy-all"]').click();

  await expect.poll(async () => page.evaluate(() => window.__site2Requests.filter((request) => request.method === "PUT" && request.path === "/api/workflows/1"))).toHaveLength(1);

  const requests = await page.evaluate(() => window.__site2Requests.filter((request) => request.method === "PUT" && request.path === "/api/workflows/1"));
  expect(requests.at(-1)?.body).toEqual(expect.objectContaining({
    description: "Ship safer changes",
    approval_policy: "all_roles",
  }));
});

test("renders the workflow graph as compact titled nodes and still allows stage creation", async ({ page }) => {
  await page.getByRole("button", { name: "Workflows" }).click();
  await page.evaluate(() => {
    const board = document.querySelector('#workflow-stage-board');
    if (board) {
      board.scrollLeft = 9999;
    }
  });
  await page.getByRole("button", { name: "Graph" }).click();

  await expect(page.locator('[data-workflow-graph-viewport]')).toBeVisible();
  await expect(page.locator('[data-workflow-graph-edge]')).toHaveCount(4);
  await expect(page.locator('[data-workflow-graph-edge][data-edge-direction="both"]')).toHaveCount(1);
  await expect(page.locator('.workflow-graph-node')).toHaveCount(4);
  await expect(page.locator('.workflow-graph-node-button')).toHaveCount(4);
  await expect(page.locator('.workflow-graph-node-button')).toContainText(["backlog", "todo", "doing", "done"]);
  const graphGeometry = await page.evaluate(() => {
    const viewport = document.querySelector('[data-workflow-graph-viewport]');
    const viewportRect = viewport.getBoundingClientRect();
    const nodes = Array.from(document.querySelectorAll('.workflow-graph-node')).map((node) => {
      const rect = node.getBoundingClientRect();
      return {
        text: node.textContent.trim(),
        left: rect.left,
        right: rect.right,
        top: rect.top,
        bottom: rect.bottom,
        visible: rect.right > viewportRect.left && rect.left < viewportRect.right && rect.bottom > viewportRect.top && rect.top < viewportRect.bottom,
      };
    });
    const overlaps = [];
    for (let i = 0; i < nodes.length; i += 1) {
      for (let j = i + 1; j < nodes.length; j += 1) {
        const a = nodes[i];
        const b = nodes[j];
        const overlap = !(a.right <= b.left || b.right <= a.left || a.bottom <= b.top || b.bottom <= a.top);
        if (overlap) {
          overlaps.push([a.text, b.text]);
        }
      }
    }
    return { nodes, overlaps };
  });
  expect(graphGeometry.nodes.every((node) => node.visible)).toBe(true);
  expect(graphGeometry.overlaps).toEqual([]);
  await expect(page.locator('.workflow-graph-node')).not.toContainText("Add role");
  await expect(page.locator('.workflow-graph-node')).not.toContainText("Save stage");

  await page.locator("#new-stage-name").fill("review");
  await page.locator("#save-stage-button").click();
  await expect(page.locator('.workflow-graph-node')).toHaveCount(5);

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ method: "POST", path: "/api/workflows/1/stages", body: expect.objectContaining({ stage_name: "review" }) }),
    ]),
  );
});

test("adds a role inside the Workflow editor using the existing stage-role API", async ({ page }) => {
  await page.getByRole("button", { name: "Workflows" }).click();
  await expect(page.locator('[data-stage-title="11"]')).toContainText("backlog");
  await page.locator('[data-add-role-select="12"]').selectOption("6");
  await page.locator('[data-add-role="12"]').click();

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ method: "POST", path: "/api/workflows/stages/roles/1/12", body: { role_id: 6 } }),
    ]),
  );
});

test("reorders roles inside a workflow lane", async ({ page }) => {
  await page.getByRole("button", { name: "Workflows" }).click();
  await page.locator('[data-add-role-select="11"]').selectOption("6");
  await page.locator('[data-add-role="11"]').click();
  await expect(page.locator('.workflow-role-card[data-stage-id="11"][data-role-id="6"]')).toBeVisible();

  await page.dragAndDrop(
    '.workflow-role-card[data-stage-id="11"][data-role-id="6"]',
    '.workflow-role-card[data-stage-id="11"][data-role-id="5"]',
  );

  await expect.poll(async () => page.evaluate(() => (
    Array.from(document.querySelectorAll('.workflow-role-card[data-stage-id="11"]'))
      .map((item) => Number(item.dataset.roleId))
  ))).toEqual([6, 5]);

  const requests = await page.evaluate(() => window.__site2Requests.filter((request) => request.path === "/api/workflows/stages/roles/1/11"));
  expect(requests.at(-1)).toEqual(expect.objectContaining({
    method: "PUT",
    body: { role_ids: [6, 5] },
  }));
});

test("renames a stage from the workflow lane title", async ({ page }) => {
  await page.getByRole("button", { name: "Workflows" }).click();
  await page.evaluate(() => {
    window._origUiPrompt = window.uiPrompt;
    window.uiPrompt = async () => "intake";
  });
  await page.locator('[data-rename-stage="11"]').click();
  await page.evaluate(() => {
    if (window._origUiPrompt) {
      window.uiPrompt = window._origUiPrompt;
    }
  });

  await expect(page.locator('[data-stage-title="11"]')).toContainText("intake");

  const requests = await page.evaluate(() => window.__site2Requests.filter((request) => request.path === "/api/workflows/stages/11"));
  expect(requests.at(-1)).toEqual(expect.objectContaining({
    method: "PUT",
    body: expect.objectContaining({ stage_name: "intake" }),
  }));
});

test("adds a comment from the ticket modal using the ticket comments endpoint", async ({ page }) => {
  await page.getByText("Move me").click();
  await expect(page.getByRole("heading", { name: "Comments" })).toBeVisible();
  await expect(page.locator("#ticket-comments")).toContainText("Initial note");
  await page.locator("#ticket-comment-input").fill("Looks good to me");
  await page.getByRole("button", { name: "Add comment" }).click();
  await expect(page.locator("#ticket-comments")).toContainText("Looks good to me");

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ method: "POST", path: "/api/tickets/OPS-101/comments", body: { comment: "Looks good to me" } }),
    ]),
  );
});

test("manages labels, dependencies, and time from the ticket modal", async ({ page }) => {
  await page.getByText("Move me").click();
  await expect(page.getByRole("heading", { name: "Labels" })).toBeVisible();
  await expect(page.locator("#ticket-labels")).toContainText("backend");
  await page.locator("#ticket-label-select").selectOption("51");
  await page.getByRole("button", { name: "Add label" }).click();

  await expect(page.getByRole("heading", { name: "Dependencies" })).toBeVisible();
  await expect(page.locator("#ticket-dependencies")).toContainText("OPS-100");
  await page.locator("#ticket-dependency-input").fill("OPS-102");
  await page.getByRole("button", { name: "Add dependency" }).click();
  await expect(page.locator("#ticket-dependencies")).toContainText("OPS-102");

  await expect(page.getByRole("heading", { name: "Time tracking" })).toBeVisible();
  await expect(page.locator("#ticket-time-total")).toContainText("30");
  await page.locator("#ticket-time-minutes").fill("15");
  await page.locator("#ticket-time-note").fill("Refactor");
  await page.getByRole("button", { name: "Log time" }).click();
  await expect(page.locator("#ticket-time-total")).toContainText("45");

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ method: "POST", path: "/api/tickets/OPS-101/labels", body: { label_id: 51 } }),
      expect.objectContaining({ method: "POST", path: "/api/dependencies", body: expect.objectContaining({ ticket_id: "OPS-101", depends_on: "OPS-102" }) }),
      expect.objectContaining({ method: "POST", path: "/api/tickets/OPS-101/time", body: { minutes: 15, note: "Refactor" } }),
    ]),
  );
});

test("creates, updates, uploads, and deletes documents from the Documents view", async ({ page }) => {
  await page.getByRole("button", { name: "Documents" }).click();
  await expect(page.getByRole("heading", { name: "Documents" })).toBeVisible();

  await page.getByRole("button", { name: "New document" }).click();
  await page.locator("#document-title").fill("Incident SOP");
  await page.locator("#document-description").fill("Runbook for incidents");
  await page.locator("#document-content").fill("step 1");
  await page.getByRole("button", { name: "Save document" }).click();
  await expect(page.locator("#document-list")).toContainText("Incident SOP");

  await page.locator("#document-title").fill("Incident SOP v2");
  await page.getByRole("button", { name: "Save document" }).click();
  await expect(page.locator("#document-list")).toContainText("Incident SOP v2");

  await page.setInputFiles("#document-upload-file", {
    name: "sop.txt",
    mimeType: "text/plain",
    buffer: Buffer.from("steps"),
  });
  await page.getByRole("button", { name: "Upload file" }).click();
  await expect(page.locator("#document-files-list")).toContainText("sop.txt");

  await page.evaluate(() => { window._origUiConfirm = window.uiConfirm; window.uiConfirm = async () => true; });
  await page.getByRole("button", { name: "Delete document" }).click();
  await page.evaluate(() => { if (window._origUiConfirm) window.uiConfirm = window._origUiConfirm; });
  await expect(page.locator("#document-list")).not.toContainText("Incident SOP v2");

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ method: "POST", path: "/api/projects/1/documents", body: expect.objectContaining({ title: "Incident SOP" }) }),
      expect.objectContaining({ method: "PUT", path: "/api/documents/2", body: expect.objectContaining({ title: "Incident SOP v2" }) }),
      expect.objectContaining({ method: "POST", path: "/api/documents/2/files", body: expect.objectContaining({ file_name: "sop.txt" }) }),
      expect.objectContaining({ method: "DELETE", path: "/api/documents/2" }),
    ]),
  );
});
