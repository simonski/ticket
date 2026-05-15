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
      status: { username: "admin", mode: "local", version: "dev" },
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
            { workflow_stage_id: 11, workflow_id: 1, stage_name: "backlog", description: "", definition_of_ready: "", definition_of_done: "", roles: [{ role_id: 5, title: "Engineer" }] },
            { workflow_stage_id: 12, workflow_id: 1, stage_name: "todo", description: "", definition_of_ready: "", definition_of_done: "", roles: [] },
            { workflow_stage_id: 13, workflow_id: 1, stage_name: "doing", description: "", definition_of_ready: "", definition_of_done: "", roles: [] },
            { workflow_stage_id: 14, workflow_id: 1, stage_name: "done", description: "", definition_of_ready: "", definition_of_done: "", roles: [] },
          ],
        },
      ],
      roles: [
        { role_id: 5, title: "Engineer", description: "Build", acceptance_criteria: "", workflow_id: 1 },
        { role_id: 6, title: "QA", description: "Verify", acceptance_criteria: "", workflow_id: 1 },
      ],
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
      if (path === "/api/login" && method === "POST") {
        return json({ token: "test-token", user: { username: body.username || "admin" } });
      }
      if (path === "/api/logout" && method === "POST") {
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
      if (path.match(/^\/api\/workflows\/\d+$/) && method === "GET") {
        const id = Number(last(path.split("/")));
        return json(db.workflows.find((item) => item.workflow_id === id));
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
  await page.getByRole("button", { name: "Sign in" }).click();
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

test("shows predicted next work entries on the board", async ({ page }) => {
  await expect(page.locator("#predicted-work-list")).toContainText("No forecastable tickets.");
});

test("reorders board stages through the Workflow reorder endpoint", async ({ page }) => {
  await page.dragAndDrop('[data-workflow-stage-id="11"]', '[data-workflow-stage-id="14"]');

  const requests = await page.evaluate(() => window.__site2Requests.filter((request) => request.path === "/api/workflows/1/reorder"));
  expect(requests.length).toBeGreaterThan(0);
});

test("adds a role inside the Workflow editor using the existing stage-role API", async ({ page }) => {
  await page.getByRole("button", { name: "Workflows" }).click();
  await expect(page.locator("#stage-grid")).toContainText("backlog");
  await expect(page.locator("#workflow-role-bank")).toContainText("Engineer");
  await page.locator('[data-add-role-select="12"]').selectOption("6");
  await page.locator('[data-add-role="12"]').click();

  const requests = await page.evaluate(() => window.__site2Requests);
  expect(requests).toEqual(
    expect.arrayContaining([
      expect.objectContaining({ method: "POST", path: "/api/workflows/stages/roles/1/12", body: { role_id: 6 } }),
    ]),
  );
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

  await page.getByRole("button", { name: "Delete document" }).click();
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
