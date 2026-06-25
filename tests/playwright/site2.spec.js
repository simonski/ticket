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
    // The mock server session persists across page reloads (the test db is
    // re-seeded on every addInitScript run, so server-side auth lives in
    // localStorage). This lets refresh/restore tests model "client auth storage
    // cleared but server session still valid".
    const serverAuthed = window.localStorage.getItem("site2.serverAuth") === "1";
    const db = {
      status: Object.assign({
        authenticated: serverAuthed,
        mode: "local",
        version: "dev",
        user: serverAuthed ? { username: "admin", role: "admin", email: "admin@example.com" } : null,
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
        {
          ticket_id: "OPS-200",
          project_id: 1,
          type: "story",
          title: "Refine me",
          description: "An idea being broken down",
          acceptance_criteria: "",
          status: "open",
          stage: "refine",
          state: "idle",
          priority: 1,
          order: 2,
          estimate_effort: 0,
          health_score: 0,
          draft: true,
          recommended_ready: true,
          archived: false,
          workflow_id: null,
        },
        {
          ticket_id: "OPS-201",
          project_id: 1,
          parent_id: "OPS-200",
          type: "story",
          title: "Breakdown story A",
          description: "",
          acceptance_criteria: "",
          status: "open",
          stage: "refine",
          state: "idle",
          priority: 1,
          order: 1,
          estimate_effort: 0,
          health_score: 0,
          draft: true,
          archived: false,
          workflow_id: null,
        },
        {
          ticket_id: "OPS-202",
          project_id: 1,
          parent_id: "OPS-200",
          type: "story",
          title: "Breakdown story B",
          description: "",
          acceptance_criteria: "",
          status: "open",
          stage: "refine",
          state: "idle",
          priority: 1,
          order: 2,
          estimate_effort: 0,
          health_score: 0,
          draft: true,
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
      contextByTicket: {
        "OPS-101": [{ edge_id: 91, project_id: 1, source_type: "ticket", source_id: "OPS-101", target_type: "url", target_id: "https://example.com/runbook", relation: "references", title: "Runbook", created_by: "admin", created_at: "now" }],
      },
      nextContextEdgeID: 92,
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
      rooms: [{ room_id: 1, slug: "general", name: "General", topic: "Everything", visibility: "public", project_id: null, ticket_id: "", archived: 0, created_by: "admin" }],
      roomMembers: [{ room_id: 1, member_id: "admin", role: "owner", joined_at: "now", last_read_at: "" }],
      roomMessages: [{ message_id: 1, room_id: 1, sender_id: "u-9f3a", sender_name: "admin", kind: "text", body: "welcome to the room", created_at: "now" }],
      nextRoomID: 2,
      nextRoomMessageID: 2,
      agents: [{ user_id: "agent-1", enabled: true }],
      teams: [{ team_id: 21, name: "Platform", parent_team_id: null }],
      myProjectAccessRequests: Array.isArray(mockSeed.myProjectAccessRequests)
        ? mockSeed.myProjectAccessRequests.map((request) => ({ ...request }))
        : [],
      myNotifications: Array.isArray(mockSeed.myNotifications)
        ? mockSeed.myNotifications.map((notification) => ({ ...notification }))
        : [],
      projectHistoryByProject: Object.assign({}, mockSeed.projectHistoryByProject || {}),
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
        window.localStorage.setItem("site2.serverAuth", "1");
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
        window.localStorage.removeItem("site2.serverAuth");
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
      if (path.match(/^\/api\/projects\/\d+\/context$/) && method === "GET") {
        const edges = Object.keys(db.contextByTicket).reduce((all, key) => all.concat(db.contextByTicket[key]), []);
        const nodes = [];
        const seen = new Set();
        const addNode = (type, id, title) => {
          const nodeKey = type + ":" + id;
          if (seen.has(nodeKey)) return;
          seen.add(nodeKey);
          nodes.push({ type, id: String(id), title });
        };
        db.documents.forEach((documentItem) => addNode("document", documentItem.document_id, documentItem.title));
        edges.forEach((edge) => {
          [[edge.source_type, edge.source_id], [edge.target_type, edge.target_id]].forEach(([type, id]) => {
            if (type === "ticket") {
              const ticket = db.tickets.find((item) => item.ticket_id === id);
              addNode("ticket", id, ticket ? ticket.title : id);
            } else if (type === "url") {
              addNode("url", id, edge.title || id);
            } else if (type === "document") {
              const documentItem = db.documents.find((item) => item.document_id === Number(id));
              addNode("document", id, documentItem ? documentItem.title : id);
            }
          });
        });
        return json({ nodes, edges });
      }
      if (path.match(/^\/api\/projects\/\d+\/context\/search$/) && method === "GET") {
        const q = String(url.searchParams.get("q") || "").toLowerCase();
        const nodes = [];
        if (q) {
          db.documents.forEach((documentItem) => {
            const haystack = [documentItem.title, documentItem.description, documentItem.notes, documentItem.content].join(" ").toLowerCase();
            if (haystack.includes(q)) {
              nodes.push({ type: "document", id: String(documentItem.document_id), title: documentItem.title });
            }
          });
          db.tickets.forEach((ticket) => {
            const haystack = [ticket.ticket_id, ticket.title, ticket.description || ""].join(" ").toLowerCase();
            if (haystack.includes(q)) {
              nodes.push({ type: "ticket", id: ticket.ticket_id, title: ticket.title });
            }
          });
        }
        return json(nodes);
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
        if (mockSeed.nonAdmin) {
          return json({ error: "user is not an admin" }, 403);
        }
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
      if (path.match(/^\/api\/tickets\/[^/]+\/context$/) && method === "GET") {
        const id = path.split("/")[3];
        return json(db.contextByTicket[id] || []);
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/context$/) && method === "POST") {
        const id = path.split("/")[3];
        if (!db.contextByTicket[id]) {
          db.contextByTicket[id] = [];
        }
        const edge = {
          edge_id: db.nextContextEdgeID++,
          project_id: 1,
          source_type: "ticket",
          source_id: id,
          target_type: String(body.target_type || ""),
          target_id: String(body.target_id || ""),
          relation: String(body.relation || "") || "references",
          title: String(body.title || ""),
          created_by: "admin",
          created_at: "now",
        };
        db.contextByTicket[id].push(edge);
        return json(edge, 201);
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/context\/\d+$/) && method === "DELETE") {
        const parts = path.split("/");
        const id = parts[3];
        const edgeID = Number(parts[5]);
        db.contextByTicket[id] = (db.contextByTicket[id] || []).filter((edge) => edge.edge_id !== edgeID);
        return json({ status: "removed" });
      }
      if (path.match(/^\/api\/tickets\/[^/]+\/children\/reorder$/) && method === "POST") {
        const parentID = path.split("/")[3];
        const order = Array.isArray(body.order) ? body.order : [];
        const children = db.tickets.filter((item) => item.parent_id === parentID);
        order.forEach((childID, index) => {
          const child = children.find((item) => item.ticket_id === childID);
          if (child) {
            child.order = index + 1;
          }
        });
        children.sort((a, b) => (a.order || 0) - (b.order || 0));
        return json(children);
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

      if (path === "/api/rooms" && method === "GET") {
        return json(db.rooms.filter((r) => !r.archived));
      }
      if (path === "/api/rooms" && method === "POST") {
        const room = {
          room_id: db.nextRoomID++, slug: String(body.name || "room").toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-|-$/g, ""),
          name: body.name || "room", topic: body.topic || "", visibility: body.visibility === "private" ? "private" : "public",
          project_id: body.project_id || null, ticket_id: body.ticket_id || "", archived: 0, created_by: "admin",
        };
        db.rooms.push(room);
        db.roomMembers.push({ room_id: room.room_id, member_id: "admin", role: "owner", joined_at: "now", last_read_at: "" });
        return json(room, 201);
      }
      {
        const roomMatch = path.match(/^\/api\/rooms\/(\d+)(\/(join|leave|members|messages))?$/);
        if (roomMatch) {
          const roomID = Number(roomMatch[1]);
          const sub = roomMatch[3] || "";
          const room = db.rooms.find((r) => r.room_id === roomID);
          if (!room) { return json({ error: "room not found" }, 404); }
          if (sub === "" && method === "GET") { return json(room); }
          if (sub === "" && method === "DELETE") { room.archived = 1; return json({ status: "archived" }); }
          if (sub === "join" && method === "POST") {
            if (!db.roomMembers.some((m) => m.room_id === roomID && m.member_id === "admin")) {
              db.roomMembers.push({ room_id: roomID, member_id: "admin", role: "member", joined_at: "now", last_read_at: "" });
            }
            return json(room);
          }
          if (sub === "leave" && method === "POST") {
            db.roomMembers = db.roomMembers.filter((m) => !(m.room_id === roomID && m.member_id === "admin"));
            return json({ status: "left" });
          }
          if (sub === "members" && method === "GET") {
            return json(db.roomMembers.filter((m) => m.room_id === roomID));
          }
          if (sub === "messages" && method === "GET") {
            return json(db.roomMessages.filter((m) => m.room_id === roomID));
          }
          if (sub === "messages" && method === "POST") {
            const msg = { message_id: db.nextRoomMessageID++, room_id: roomID, sender_id: "u-9f3a", sender_name: "admin", kind: body.kind || "text", body: body.body || "", created_at: "now" };
            db.roomMessages.push(msg);
            return json(msg, 201);
          }
        }
      }

      return json({ error: `Unhandled ${method} ${path}` }, 500);
    };
  }, seed);
}

// Settings sub-areas (config, plans, providers, organisation) are now tabs inside
// the Settings view rather than standalone nav buttons.
async function openSettingsTab(page, tab) {
  await page.locator('button[data-view="settings"]').click();
  await page.locator(`[data-settings-tab="${tab}"]`).click();
}

// The board/workflow lanes use native HTML5 drag-and-drop. Playwright's
// mouse-based dragAndDrop does not trigger native DnD handlers, so dispatch the
// drag events directly with a shared DataTransfer.
async function htmlDragDrop(page, sourceSel, targetSel) {
  await page.locator(sourceSel).first().waitFor({ state: "attached" });
  await page.locator(targetSel).first().waitFor({ state: "attached" });
  await page.evaluate(({ sourceSel, targetSel }) => {
    const src = document.querySelector(sourceSel);
    const tgt = document.querySelector(targetSel);
    const dataTransfer = new DataTransfer();
    const ev = (type) => new DragEvent(type, { bubbles: true, cancelable: true, composed: true, dataTransfer });
    src.dispatchEvent(ev("dragstart"));
    tgt.dispatchEvent(ev("dragenter"));
    tgt.dispatchEvent(ev("dragover"));
    tgt.dispatchEvent(ev("drop"));
    src.dispatchEvent(ev("dragend"));
  }, { sourceSel, targetSel });
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

  await openSettingsTab(page, "settings");
  await expect(page.getByRole("heading", { name: "Configuration" })).toBeVisible();
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
  await page.evaluate(() => {
    sessionStorage.clear();
    localStorage.clear();
    window.location.reload();
  });

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
  await page.evaluate(() => {
    sessionStorage.clear();
    localStorage.clear();
    window.location.reload();
  });
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
  // Wait for the prompt dialog to finish opening (it sets the input to the current
  // name) before overwriting it, otherwise the fill races with the dialog setup.
  await expect(page.locator("#dialog-input")).toBeVisible();
  await page.locator("#dialog-input").fill("Desk key");
  await page.locator("#dialog-ok").click();
  await expect(page.locator("#account-passkey-list")).toContainText("Desk key");

  await page.locator("#account-passkey-name").fill("Phone");
  await page.getByRole("button", { name: "Enroll passkey" }).click();
  await expect(page.locator("#account-passkey-list")).toContainText("Phone");

  await page.locator("[data-passkey-id='cred-old'] [data-passkey-action='delete']").click();
  await page.locator("#dialog-ok").click();
  await expect(page.locator("#account-passkey-list")).not.toContainText("Desk key");

  const requests = await page.evaluate(() => window.__site2Requests || []);
  expect(requests.find((request) => request.path === "/api/users/me/passkeys" && request.method === "GET")).toBeTruthy();
  expect(requests.find((request) => request.path === "/api/users/me/passkeys/cred-old" && request.method === "PUT")).toBeTruthy();
  expect(requests.find((request) => request.path === "/api/users/me/passkeys/cred-old" && request.method === "DELETE")).toBeTruthy();
  expect(requests.find((request) => request.path === "/api/auth/passkey/register/start")).toBeTruthy();
  expect(requests.find((request) => request.path === "/api/auth/passkey/finish")).toBeTruthy();
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
  await expect(page.locator("#project-menu-button")).toContainText("Website (WEB)");
  await expect.poll(() => page.evaluate(() => localStorage.getItem("site2.selectedProjectID"))).toBe("2");

  await page.reload();

  await expect(page.locator("#project-menu-button")).toContainText("Website (WEB)");
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

  await page.locator('#main-nav button[data-view="projects"]').click();
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

  await expect(page.locator('#main-nav button[data-view="projects"]')).toHaveClass(/active/);
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
  await page.locator('#main-nav button[data-view="projects"]').click();
  await page.getByRole("button", { name: "New project" }).click();
  await expect(page.locator("#project-prefix")).toHaveAttribute("maxlength", "5");
  await page.locator("#project-prefix").fill("WEB");
  await page.locator("#project-title").fill("Website");
  await page.locator("#project-default-draft").selectOption("true");
  await page.getByRole("button", { name: "Save project" }).click();

  await expect(page.locator("#project-title")).toHaveValue("Website");

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ method: "POST", path: "/api/projects", body: expect.objectContaining({ prefix: "WEB", title: "Website" }) }),
      expect.objectContaining({ method: "PUT", path: "/api/projects/2/set-draft", body: { draft: true } }),
    ]),
  );
});

test("submits a project access request from the Projects view", async ({ page }) => {
  await page.locator('#main-nav button[data-view="projects"]').click();
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
  await page.locator('#main-nav button[data-view="projects"]').click();

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
  await page.locator('#main-nav button[data-view="projects"]').click();

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
  await htmlDragDrop(page, '#ticket-board [data-ticket-id="OPS-101"]', '[data-lane-stage="ready"]');
  await expect(page.locator('[data-lane-stage="ready"]')).toContainText("Move me");

  const requests = await page.evaluate(() => window.__site2Requests.filter((request) => request.path === "/api/tickets/OPS-101"));
  expect(requests.some((request) => request.body.stage === "ready")).toBeTruthy();
});

test("creates, updates, and deletes plans from the plans panel", async ({ page }) => {
  await openSettingsTab(page, "plans");

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

  await page.locator("#delete-plan-button").click();
  await page.locator("#dialog-ok").click();

  await expect(page.locator("#plan-list")).not.toContainText("Enterprise Plus");

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(expect.arrayContaining([
    expect.objectContaining({ method: "POST", path: "/api/plans", body: expect.objectContaining({ slug: "enterprise", name: "Enterprise" }) }),
    expect.objectContaining({ method: "PUT", path: "/api/plans/enterprise", body: expect.objectContaining({ name: "Enterprise Plus" }) }),
    expect.objectContaining({ method: "POST", path: "/api/plans/default", body: expect.objectContaining({ slug: "enterprise" }) }),
    expect.objectContaining({ method: "DELETE", path: "/api/plans/enterprise" }),
  ]));
});

// Removed: "reorders board stages through the Workflow reorder endpoint" — the main
// ticket board no longer exposes workflow-stage lanes (no [data-workflow-stage-id]);
// stage reordering now lives in the Workflows view and is covered by the two
// "reorders stages inside the workflows board" tests below.

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
  await htmlDragDrop(page, '#stage-grid .stage-card[data-stage-id="11"]', '#stage-grid .stage-card[data-stage-id="13"]');

  await expect.poll(async () => page.evaluate(() => (
    Array.from(document.querySelectorAll("#stage-grid .stage-card[data-stage-id]"))
      .map((item) => Number(item.dataset.stageId))
  ))).toEqual([12, 11, 13, 14]);

  const requests = await page.evaluate(() => window.__site2Requests.filter((request) => request.path === "/api/workflows/1/reorder"));
  expect(requests.at(-1)?.body?.stage_ids).toEqual([12, 11, 13, 14]);
});

test("reorders stages inside the workflows board with lane buttons", async ({ page }) => {
  await page.getByRole("button", { name: "Workflows" }).click();
  await page.locator('[data-move-stage="12"][data-move-direction="left"]').click();

  await expect.poll(async () => page.evaluate(() => (
    Array.from(document.querySelectorAll("#stage-grid .stage-card[data-stage-id]"))
      .map((item) => Number(item.dataset.stageId))
  ))).toEqual([12, 11, 13, 14]);

  const requests = await page.evaluate(() => window.__site2Requests.filter((request) => request.path === "/api/workflows/1/reorder"));
  expect(requests.at(-1)?.body?.stage_ids).toEqual([12, 11, 13, 14]);
});

test("workflow settings autosave changes", async ({ page }) => {
  await page.getByRole("button", { name: "Workflows" }).click();
  await page.locator("#workflow-settings summary").click();
  await page.locator("#workflow-description").fill("Ship safer changes");
  await page.locator('#workflow-approval-policy-all').check();

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
  await page.locator("#workflow-view-graph").click();

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
  await expect(page.locator('[data-workflow-graph-viewport]')).not.toContainText("Add role");
  await expect(page.locator('[data-workflow-graph-viewport]')).not.toContainText("Save stage");

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

  await htmlDragDrop(
    page,
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
  await page.locator("[data-ticket-tab='activity']").click();
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
  await page.locator("[data-ticket-tab='properties']").click();
  await expect(page.locator("#ticket-labels")).toContainText("backend");
  await page.locator("#ticket-label-select").selectOption("51");
  await page.locator("#add-ticket-label-button").click();

  await expect(page.locator("#ticket-dependencies")).toContainText("OPS-100");
  await page.locator("#ticket-dependency-input").fill("OPS-102");
  await page.getByRole("button", { name: "Add dependency" }).click();
  await expect(page.locator("#ticket-dependencies")).toContainText("OPS-102");

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

test("attaches and removes context from the ticket modal", async ({ page }) => {
  await page.getByText("Move me").click();
  await page.locator("[data-ticket-tab='properties']").click();
  await expect(page.locator("#ticket-context")).toBeVisible();
  await expect(page.locator("#ticket-context")).toContainText("Runbook");

  await page.locator("#ticket-context-type").selectOption("url");
  await page.locator("#ticket-context-target").fill("https://example.com/spec");
  await page.getByRole("button", { name: "Attach" }).click();
  await expect(page.locator("#ticket-context")).toContainText("https://example.com/spec");

  await page.locator("#ticket-context [data-remove-ticket-context='91']").click();
  await expect(page.locator("#ticket-context")).not.toContainText("Runbook");

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({
        method: "POST",
        path: "/api/tickets/OPS-101/context",
        body: expect.objectContaining({ target_type: "url", target_id: "https://example.com/spec" }),
      }),
      expect.objectContaining({ method: "DELETE", path: "/api/tickets/OPS-101/context/91" }),
    ]),
  );
});

test("renders the context graph and focuses a story from the ticket modal", async ({ page }) => {
  // The Context view draws the project graph from the context endpoint.
  await page.getByRole("button", { name: "Context" }).click();
  await expect(page.locator("#context-graph [data-node-key='document:1']")).toBeVisible();
  await expect(page.locator("#context-graph [data-node-key='ticket:OPS-101']")).toBeVisible();
  await expect(page.locator("#context-graph [data-node-key='url:https://example.com/runbook']")).toBeVisible();

  // Search highlights matching nodes via the context search endpoint.
  await page.locator("#context-search-input").fill("Runbook");
  await expect(page.locator("#context-graph .context-node.matched")).toHaveCount(1);
  await expect(page.locator("#context-graph .context-node.matched")).toHaveAttribute("data-node-key", "document:1");

  // "View in graph" from a story's Context section focuses that story.
  await page.getByRole("button", { name: "Board" }).click();
  await page.locator("#view-tickets").getByText("Move me").click();
  await page.locator("[data-ticket-tab='properties']").click();
  await page.getByRole("button", { name: "View in graph" }).click();
  await expect(page.locator("#view-context")).toHaveClass(/active/);
  await expect(page.locator("#context-graph [data-node-key='ticket:OPS-101']")).toHaveClass(/focus/);

  // Non-neighbors are dimmed while focused; clearing focus restores them.
  await expect(page.locator("#context-graph .context-node.dimmed")).not.toHaveCount(0);
  await page.locator("#context-clear-focus").click();
  await expect(page.locator("#context-graph .context-node.dimmed")).toHaveCount(0);
});

test("reorders the proposed breakdown from the refinement panel", async ({ page }) => {
  await page.getByText("Refine me").click();
  await page.locator("[data-ticket-tab='refinement']").click();
  await expect(page.locator("#refinement-breakdown")).toContainText("Breakdown story A");

  // Story A is first; move it down so the order becomes B, A.
  await page.locator("#refinement-breakdown [data-child-id='OPS-201'] [data-reorder='down']").click();
  await expect(page.locator("#refinement-breakdown .refinement-child").first()).toContainText("Breakdown story B");

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({
        method: "POST",
        path: "/api/tickets/OPS-200/children/reorder",
        body: { order: ["OPS-202", "OPS-201"] },
      }),
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
  await page.locator("#upload-document-file-button").click();
  await expect(page.locator("#document-files-list")).toContainText("sop.txt");

  await page.evaluate(() => { window._origUiConfirm = window.uiConfirm; window.uiConfirm = async () => true; });
  await page.locator("#delete-document-button").click();
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

test("Shift Shift command palette navigates between windows", async ({ page }) => {
  // Double-Shift opens the palette.
  await page.keyboard.press("Shift");
  await page.keyboard.press("Shift");
  await expect(page.locator("#command-palette-overlay")).toBeVisible();

  // Filter to a slash-shortcut and navigate with Enter.
  await page.locator("#command-palette-input").fill("proj");
  await expect(page.locator("#command-palette-list")).toContainText("/projects");
  await page.locator("#command-palette-input").press("Enter");
  await expect(page.locator('#main-nav button[data-view="projects"]')).toHaveClass(/active/);
  await expect(page.locator("#command-palette-overlay")).toBeHidden();

  // /backlog is an alias for the board (tickets) window.
  await page.keyboard.press("Shift");
  await page.keyboard.press("Shift");
  await expect(page.locator("#command-palette-overlay")).toBeVisible();
  await page.locator("#command-palette-input").fill("backlog");
  await expect(page.locator("#command-palette-list")).toContainText("/backlog");
  await page.locator("#command-palette-input").press("Enter");
  await expect(page.locator('#main-nav button[data-view="tickets"]')).toHaveClass(/active/);

  // Esc closes the palette without navigating.
  await page.keyboard.press("Shift");
  await page.keyboard.press("Shift");
  await expect(page.locator("#command-palette-overlay")).toBeVisible();
  await page.locator("#command-palette-input").press("Escape");
  await expect(page.locator("#command-palette-overlay")).toBeHidden();
});

test("command palette: single-letter alias and /ticket-key quick-open", async ({ page }) => {
  // Single-letter alias: /b -> board (tickets).
  await page.keyboard.press("Shift");
  await page.keyboard.press("Shift");
  await expect(page.locator("#command-palette-overlay")).toBeVisible();
  await page.locator("#command-palette-input").fill("p");
  await expect(page.locator(".command-palette-item").first()).toContainText("/p");
  await page.locator("#command-palette-input").press("Enter");
  await expect(page.locator('#main-nav button[data-view="projects"]')).toHaveClass(/active/);

  // /<ticket-key> pushes a numbered action menu (command stack, TK-130).
  await page.keyboard.press("Shift");
  await page.keyboard.press("Shift");
  await page.locator("#command-palette-input").fill("ops-101");
  await expect(page.locator("#command-palette-list")).toContainText("OPS-101");
  await page.locator("#command-palette-input").press("Enter");
  await expect(page.locator("#command-palette-list")).toContainText("Open detail");

  // Esc pops one frame back to the command list without closing the palette.
  await page.locator("#command-palette-input").press("Escape");
  await expect(page.locator("#command-palette-overlay")).toBeVisible();
  await expect(page.locator("#command-palette-list")).not.toContainText("Open detail");

  // Re-enter the ticket actions and pick "Open detail" by number key.
  await page.locator("#command-palette-input").fill("ops-101");
  await page.locator("#command-palette-input").press("Enter");
  await page.locator("#command-palette-input").press("1");
  await expect(page.locator("#ticket-modal")).toHaveClass(/open/);
});

test("chat: rooms list, messages, send, and create a room", async ({ page }) => {
  // /chat (now registered) navigates to the chat view.
  await page.keyboard.press("Shift");
  await page.keyboard.press("Shift");
  await page.locator("#command-palette-input").fill("chat");
  await page.locator("#command-palette-input").press("Enter");
  await expect(page.locator('#main-nav button[data-view="chat"]')).toHaveClass(/active/);

  // Seeded room appears; selecting it shows its messages.
  await expect(page.locator("#chat-rooms-list")).toContainText("General");
  await page.locator('.chat-room-item[data-room-id="1"]').click();
  await expect(page.locator("#chat-room-title")).toHaveText("General");
  await expect(page.locator("#chat-messages")).toContainText("welcome to the room");
  // Shows the username, not the raw user id.
  await expect(page.locator("#chat-messages")).toContainText("admin");
  await expect(page.locator("#chat-messages")).not.toContainText("u-9f3a");

  // Send a message.
  await page.locator("#chat-composer-input").fill("hello there");
  await page.locator("#chat-send-button").click();
  await expect(page.locator("#chat-messages")).toContainText("hello there");

  // Create a new room from the prompt.
  await page.locator("#new-room-button").click();
  await expect(page.locator("#dialog-input")).toBeVisible();
  await page.locator("#dialog-input").fill("Engineering");
  await page.locator("#dialog-ok").click();
  await expect(page.locator("#chat-rooms-list")).toContainText("Engineering");
});

test("chat: breakout room from a ticket", async ({ page }) => {
  // Open a ticket and start a breakout room scoped to it.
  await page.getByText("Move me").click();
  await expect(page.locator("#ticket-modal")).toHaveClass(/open/);
  await page.locator("#ticket-breakout-button").click();
  await expect(page.locator('#main-nav button[data-view="chat"]')).toHaveClass(/active/);
  await expect(page.locator("#chat-room-title")).toContainText("Breakout");
  await expect(page.locator("#chat-rooms-list")).toContainText("Breakout");
});

test("login boots even when an admin-only load (roles) returns 403", async ({ page }) => {
  // Regression: loadRoles hit /api/roles (admin-only) unconditionally; a 403
  // aborted the whole boot for non-admins ("user is not an admin").
  await installSite2Mock(page, { nonAdmin: true });
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
  await expect(page.locator("#app-shell")).not.toHaveClass(/hidden/);
});

test("board: arrow/wasd move card focus and Enter opens the ticket", async ({ page }) => {
  // Seeded board has OPS-101 ("Move me") in the design lane.
  await expect(page.locator('[data-lane-stage="design"] [data-ticket-id="OPS-101"]')).toBeVisible();
  // No card focused yet; the first nav keypress focuses the first card.
  await page.locator("body").press("ArrowDown");
  await expect(page.locator(".ticket-card.kbd-focus")).toHaveCount(1);
  // wasd also navigates (focus stays on a card, page does not lose the focus ring).
  await page.locator("body").press("d");
  await page.locator("body").press("s");
  await expect(page.locator(".ticket-card.kbd-focus")).toHaveCount(1);
  // Enter opens the focused ticket's modal.
  await page.locator("body").press("Enter");
  await expect(page.locator("#ticket-modal")).toHaveClass(/open/);
});

test("Escape closes the ticket modal and the account modal (TK-49)", async ({ page }) => {
  // Ticket modal: open via a board card, Escape dismisses it.
  await page.locator('[data-ticket-id="OPS-101"]').click();
  await expect(page.locator("#ticket-modal")).toHaveClass(/open/);
  await page.locator("body").press("Escape");
  await expect(page.locator("#ticket-modal")).not.toHaveClass(/open/);
  // Account modal: open via the single account-menu item, Escape dismisses it.
  await page.locator("#account-menu-button").click();
  await page.locator('[data-account-action="account"]').click();
  await expect(page.locator("#account-modal")).toHaveClass(/open/);
  await page.locator("body").press("Escape");
  await expect(page.locator("#account-modal")).not.toHaveClass(/open/);
});

test("account menu is a single 'Account settings' entry (TK-50)", async ({ page }) => {
  await page.locator("#account-menu-button").click();
  const items = page.locator("#account-menu-dropdown [data-account-action]");
  await expect(items).toHaveCount(1);
  await expect(items.first()).toHaveText("Account settings");
  await items.first().click();
  await expect(page.locator("#account-modal")).toHaveClass(/open/);
  await expect(page.locator("#account-modal-title")).toHaveText("Account settings");
});

test("admin sees the Access (access-roles) nav entry (TK-135)", async ({ page }) => {
  await expect(page.locator('#admin-nav button[data-view="access"]')).toBeVisible();
});

test("chat: opening the panel auto-selects a room and focuses the composer", async ({ page }) => {
  await page.locator('#main-nav button[data-view="chat"]').click();
  // A room is selected automatically — the user can chat without clicking one.
  await expect(page.locator("#chat-room-title")).toHaveText("General");
  await expect(page.locator("#chat-composer-input")).toBeEnabled();
  await expect(page.locator("#chat-composer-input")).toBeFocused();
});

test("composer power transforms apply sed-style substitutions safely (TK-167)", async ({ page }) => {
  // The shared beforeEach already booted and logged in; the transform helpers
  // are exposed on window at app boot.
  const result = await page.evaluate(() => {
    const tt = window.__site2TextTransform;
    const apply = (target, text) => {
      const sub = tt.parseSubstitution(text);
      if (!sub) { return { parsed: false }; }
      return { parsed: true, ...tt.applySubstitution(target, sub) };
    };
    return {
      firstOnly: apply("teh cat sat on teh mat", "s/teh/the/"),
      global: apply("teh cat sat on teh mat", "s/teh/the/g"),
      caseInsensitive: apply("Hello HELLO hello", "s/hello/hi/gi"),
      altDelimiter: apply("a/b/c", "s|/|-|g"),
      invalid: apply("anything", "s/(unclosed/x/"),
      notACommand: tt.parseSubstitution("such a nice day"),
      bareWordS: tt.parseSubstitution("so this is fine"),
    };
  });
  expect(result.firstOnly.text).toBe("the cat sat on teh mat");
  expect(result.global.text).toBe("the cat sat on the mat");
  expect(result.caseInsensitive.text).toBe("hi hi hi");
  expect(result.altDelimiter.text).toBe("a-b-c");
  expect(result.invalid.error).toBeTruthy(); // invalid regex reported, not thrown
  expect(result.notACommand).toBeNull();
  expect(result.bareWordS).toBeNull();
});

test("typing a substitution rewrites the previous message in the composer (TK-167)", async ({ page }) => {
  // The shared beforeEach already logged in and landed on the Board.
  await page.locator('#main-nav button[data-view="chat"]').click();
  const input = page.locator("#chat-composer-input");
  await expect(input).toBeEnabled();
  // Send a message with a typo, then correct it with a substitution.
  await input.fill("hello wrold");
  await input.press("Enter");
  await expect.poll(() => page.evaluate(() => document.getElementById("chat-composer-input").value)).toBe("");
  await input.fill("s/wrold/world/");
  await input.press("Enter");
  // The substitution loads the corrected previous message back into the composer.
  await expect(input).toHaveValue("hello world");
});
