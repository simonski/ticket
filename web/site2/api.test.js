const test = require("node:test");
const assert = require("node:assert/strict");
const { createClient } = require("./api.js");

function jsonResponse(status, body, headers) {
    const payload = body === undefined ? "" : JSON.stringify(body);
    return {
        ok: status >= 200 && status < 300,
        status,
        statusText: status === 200 ? "OK" : "ERR",
        headers: {
            get(name) {
                const key = Object.keys(headers || {}).find((item) => item.toLowerCase() === String(name).toLowerCase());
                return key ? headers[key] : null;
            },
        },
        async text() {
            return payload;
        },
        async blob() {
            return new Blob([payload], { type: "application/json" });
        },
    };
}

test("request includes auth header when token is set", async () => {
    let captured;
    const client = createClient({
        fetch: async (url, options) => {
            captured = { url, options };
            return jsonResponse(200, { ok: true });
        },
    });
    client.setToken("secret-token");
    const body = await client.get("/api/status");
    assert.deepEqual(body, { ok: true });
    assert.equal(captured.url, "/api/status");
    assert.equal(captured.options.headers.Authorization, "Bearer secret-token");
});

test("request includes csrf header from host cookie when present", async () => {
    let captured;
    globalThis.document = { cookie: "__Host-_csrf=host-token; other=1" };
    const client = createClient({
        fetch: async (url, options) => {
            captured = { url, options };
            return jsonResponse(200, { ok: true });
        },
    });
    await client.post("/api/logout", {});
    assert.equal(captured.options.headers["X-CSRF-Token"], "host-token");
    delete globalThis.document;
});

test("request falls back to legacy csrf cookie when present", async () => {
    let captured;
    globalThis.document = { cookie: "_csrf=legacy-token; other=1" };
    const client = createClient({
        fetch: async (url, options) => {
            captured = { url, options };
            return jsonResponse(200, { ok: true });
        },
    });
    await client.post("/api/logout", {});
    assert.equal(captured.options.headers["X-CSRF-Token"], "legacy-token");
    delete globalThis.document;
});

test("login does not send auth header", async () => {
    let captured;
    const client = createClient({
        fetch: async (url, options) => {
            captured = { url, options };
            return jsonResponse(200, { token: "abc" });
        },
    });
    await client.login("user", "pass");
    assert.equal(captured.url, "/api/login");
    assert.equal(captured.options.headers.Authorization, undefined);
});

test("startPasskeyLogin does not send auth header", async () => {
    let captured;
    const client = createClient({
        fetch: async (url, options) => {
            captured = { url, options };
            return jsonResponse(200, { code: "abc123" });
        },
    });
    await client.startPasskeyLogin("alice");
    assert.equal(captured.url, "/api/auth/passkey/login/start");
    assert.equal(captured.options.headers.Authorization, undefined);
    assert.deepEqual(JSON.parse(captured.options.body), { username: "alice" });
});

test("passkey helpers encode challenge and finish payload paths", async () => {
    const calls = [];
    const client = createClient({
        fetch: async (url, options) => {
            calls.push({ url, options });
            return jsonResponse(200, { status: "complete", token: "abc" });
        },
    });
    await client.getPasskeyChallenge("code 1");
    await client.finishPasskeyFlow("code 1", { id: "cred" });
    await client.pollPasskey("code 1");
    assert.equal(calls[0].url, "/api/auth/passkey/challenge?code=code%201");
    assert.equal(calls[0].options.method, "GET");
    assert.equal(calls[1].url, "/api/auth/passkey/finish?code=code%201");
    assert.deepEqual(JSON.parse(calls[1].options.body), { id: "cred" });
    assert.equal(calls[2].url, "/api/auth/passkey/poll");
    assert.deepEqual(JSON.parse(calls[2].options.body), { code: "code 1" });
});

test("passkey management methods use authenticated user passkey paths", async () => {
    const calls = [];
    const client = createClient({
        fetch: async (url, options) => {
            calls.push({ url, options });
            return jsonResponse(200, { ok: true });
        },
    });
    client.setToken("secret-token");
    await client.startPasskeyRegistration("Laptop");
    await client.listMyPasskeys();
    await client.renameMyPasskey("cred/1", "Desk key");
    await client.deleteMyPasskey("cred/1");
    assert.equal(calls[0].url, "/api/auth/passkey/register/start");
    assert.equal(calls[0].options.headers.Authorization, "Bearer secret-token");
    assert.deepEqual(JSON.parse(calls[0].options.body), { name: "Laptop" });
    assert.equal(calls[1].url, "/api/users/me/passkeys");
    assert.equal(calls[2].url, "/api/users/me/passkeys/cred%2F1");
    assert.deepEqual(JSON.parse(calls[2].options.body), { name: "Desk key" });
    assert.equal(calls[3].url, "/api/users/me/passkeys/cred%2F1");
    assert.equal(calls[3].options.method, "DELETE");
});

test("register does not send auth header and omits empty optional fields", async () => {
    let captured;
    const client = createClient({
        fetch: async (url, options) => {
            captured = { url, options };
            return jsonResponse(201, { user: { username: "newuser" }, password: "generated-pass" });
        },
    });
    const body = await client.register("newuser", "", "");
    assert.equal(captured.url, "/api/register");
    assert.equal(captured.options.headers.Authorization, undefined);
    assert.deepEqual(JSON.parse(captured.options.body), { username: "newuser" });
    assert.equal(body.password, "generated-pass");
});

test("requestWithFallback returns fallback on error", async () => {
    const client = createClient({
        fetch: async () => jsonResponse(500, { error: "boom" }),
    });
    const value = await client.requestWithFallback("/api/projects", []);
    assert.deepEqual(value, []);
});

test("request reports API error message", async () => {
    const client = createClient({
        fetch: async () => jsonResponse(400, { error: "invalid payload" }),
    });
    await assert.rejects(() => client.post("/api/projects", {}), /invalid payload/);
});

test("fetchDocumentFile returns parsed filename + blob", async () => {
    const client = createClient({
        fetch: async () => jsonResponse(200, { file: true }, { "Content-Disposition": "attachment; filename=\"plan.txt\"" }),
    });
    const out = await client.fetchDocumentFile(1, 2);
    assert.equal(out.fileName, "plan.txt");
    assert.ok(out.blob instanceof Blob);
});

test("updatePlan sends put payload", async () => {
    let captured;
    const client = createClient({
        fetch: async (url, options) => {
            captured = { url, options };
            return jsonResponse(200, { ok: true });
        },
    });
    await client.updatePlan("free", {
        default_project_alias: "private",
        registration_actions: {
            auto_assign_public_team: false,
            auto_create_private_project: true,
            auto_create_private_team: false,
        },
    });
    assert.equal(captured.url, "/api/plans/free");
    assert.equal(captured.options.method, "PUT");
    assert.deepEqual(JSON.parse(captured.options.body), {
        default_project_alias: "private",
        registration_actions: {
            auto_assign_public_team: false,
            auto_create_private_project: true,
            auto_create_private_team: false,
        },
    });
});

test("listProjectAccessRequests appends optional status query", async () => {
    let captured;
    const client = createClient({
        fetch: async (url, options) => {
            captured = { url, options };
            return jsonResponse(200, []);
        },
    });
    await client.listProjectAccessRequests("GATE", "pending");
    assert.equal(captured.url, "/api/projects/GATE/access-requests?status=pending");
    assert.equal(captured.options.method, "GET");
});

test("createProjectAccessRequest posts message payload", async () => {
    let captured;
    const client = createClient({
        fetch: async (url, options) => {
            captured = { url, options };
            return jsonResponse(201, { request_id: 7, status: "pending" });
        },
    });
    const body = await client.createProjectAccessRequest("GATE", "please add me");
    assert.equal(captured.url, "/api/projects/GATE/access-requests");
    assert.equal(captured.options.method, "POST");
    assert.deepEqual(JSON.parse(captured.options.body), { message: "please add me" });
    assert.equal(body.status, "pending");
});

test("listMyProjectAccessRequests appends optional status query", async () => {
    let captured;
    const client = createClient({
        fetch: async (url, options) => {
            captured = { url, options };
            return jsonResponse(200, []);
        },
    });
    await client.listMyProjectAccessRequests("approved");
    assert.equal(captured.url, "/api/users/me/access-requests?status=approved");
    assert.equal(captured.options.method, "GET");
});

test("listProjectHistory encodes project ref and filters", async () => {
    let captured;
    const client = createClient({
        fetch: async (url, options) => {
            captured = { url, options };
            return jsonResponse(200, []);
        },
    });
    await client.listProjectHistory("private", { limit: 5, userID: "alice", teamID: 21 });
    assert.equal(captured.url, "/api/projects/private/history?limit=5&user_id=alice&team_id=21");
    assert.equal(captured.options.method, "GET");
});

test("listMyNotifications appends status and limit query", async () => {
    let captured;
    const client = createClient({
        fetch: async (url, options) => {
            captured = { url, options };
            return jsonResponse(200, []);
        },
    });
    await client.listMyNotifications("unread", 5);
    assert.equal(captured.url, "/api/users/me/notifications?status=unread&limit=5");
    assert.equal(captured.options.method, "GET");
});

test("markNotificationRead posts read action", async () => {
    let captured;
    const client = createClient({
        fetch: async (url, options) => {
            captured = { url, options };
            return jsonResponse(200, { notification_id: 9, status: "read" });
        },
    });
    const body = await client.markNotificationRead(9);
    assert.equal(captured.url, "/api/users/me/notifications/9/read");
    assert.equal(captured.options.method, "POST");
    assert.deepEqual(JSON.parse(captured.options.body), {});
    assert.equal(body.status, "read");
});

test("setProjectAccessRequestStatus posts approve action", async () => {
    let captured;
    const client = createClient({
        fetch: async (url, options) => {
            captured = { url, options };
            return jsonResponse(200, { request_id: 7, status: "approved" });
        },
    });
    const body = await client.setProjectAccessRequestStatus("GATE", 7, "approved", "Looks good");
    assert.equal(captured.url, "/api/projects/GATE/access-requests/7/approve");
    assert.equal(captured.options.method, "POST");
    assert.deepEqual(JSON.parse(captured.options.body), { message: "Looks good" });
    assert.equal(body.status, "approved");
});
