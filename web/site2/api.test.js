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
