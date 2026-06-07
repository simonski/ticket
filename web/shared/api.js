(function (root, factory) {
    if (typeof module === "object" && module.exports) {
        module.exports = factory();
        return;
    }
    root.TicketAPI = factory();
}(typeof globalThis !== "undefined" ? globalThis : this, function () {
    "use strict";

    function toErrorMessage(response, body) {
        if (body && typeof body === "object") {
            if (body.error) {
                return String(body.error);
            }
            if (body.message) {
                return String(body.message);
            }
        }
        return response.status + " " + response.statusText;
    }

    function extractFileName(contentDisposition, fallback) {
        const fileNameMatch = /filename="?([^"]+)"?/i.exec(contentDisposition || "");
        if (fileNameMatch && fileNameMatch[1]) {
            return fileNameMatch[1];
        }
        return fallback;
    }

    function readCookie(name) {
        if (typeof document === "undefined" || !document || typeof document.cookie !== "string") {
            return "";
        }
        const escaped = String(name || "").replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
        const match = document.cookie.match(new RegExp("(?:^|;\\s*)" + escaped + "=([^;]*)"));
        return match ? decodeURIComponent(match[1]) : "";
    }

    function getCsrfToken() {
        return readCookie("__Host-_csrf") || readCookie("_csrf");
    }

    function createClient(options) {
        const opts = options || {};
        const fetchImpl = opts.fetch || (typeof fetch === "function" ? fetch.bind(globalThis) : null);
        if (!fetchImpl) {
            throw new Error("fetch implementation is required");
        }
        const timeoutMs = Number(opts.timeoutMs || 10000);
        const baseURL = String(opts.baseURL || "");
        const credentials = opts.credentials || "same-origin";
        let token = "";

        function withBase(path) {
            return baseURL + path;
        }

        function setToken(next) {
            token = String(next || "");
        }

        function getToken() {
            return token;
        }

        function authHeaders(extraHeaders, includeJSON, includeAuth) {
            const headers = Object.assign({}, extraHeaders || {});
            if (includeJSON && !headers["Content-Type"]) {
                headers["Content-Type"] = "application/json";
            }
            const csrf = getCsrfToken();
            if (csrf && !headers["X-CSRF-Token"]) {
                headers["X-CSRF-Token"] = csrf;
            }
            if (includeAuth && token) {
                headers.Authorization = "Bearer " + token;
            }
            return headers;
        }

        async function requestRaw(path, options) {
            const requestOptions = options || {};
            const controller = new AbortController();
            const timeoutID = setTimeout(function () { controller.abort(); }, timeoutMs);
            try {
                return await fetchImpl(withBase(path), Object.assign({}, requestOptions, {
                    credentials: credentials,
                    headers: authHeaders(requestOptions.headers, requestOptions.json !== false, requestOptions.auth !== false),
                    signal: controller.signal,
                }));
            } catch (error) {
                if (error && error.name === "AbortError") {
                    throw new Error("request timed out");
                }
                throw error;
            } finally {
                clearTimeout(timeoutID);
            }
        }

        async function request(path, options) {
            const response = await requestRaw(path, options);
            const text = await response.text();
            let body = null;
            if (text) {
                try {
                    body = JSON.parse(text);
                } catch (error) {
                    body = text;
                }
            }
            if (!response.ok) {
                const error = new Error(toErrorMessage(response, body));
                error.status = response.status;
                error.responseBody = body;
                throw error;
            }
            return body;
        }

        function requestWithFallback(path, fallbackValue, options) {
            return request(path, options).catch(function () {
                return fallbackValue;
            });
        }

        function get(path) {
            return request(path, { method: "GET" });
        }

        function post(path, payload) {
            return request(path, { method: "POST", body: JSON.stringify(payload || {}) });
        }

        function put(path, payload) {
            return request(path, { method: "PUT", body: JSON.stringify(payload || {}) });
        }

        function del(path) {
            return request(path, { method: "DELETE" });
        }

        async function login(username, password) {
            return request("/api/login", {
                method: "POST",
                body: JSON.stringify({ username: username, password: password }),
                auth: false,
            });
        }

        async function startPasskeyLogin(username) {
            return request("/api/auth/passkey/login/start", {
                method: "POST",
                body: JSON.stringify({ username: username }),
                auth: false,
            });
        }

        function getPasskeyChallenge(code) {
            return request("/api/auth/passkey/challenge?code=" + encodeURIComponent(String(code || "")), {
                method: "GET",
                auth: false,
            });
        }

        async function finishPasskeyFlow(code, credential) {
            return request("/api/auth/passkey/finish?code=" + encodeURIComponent(String(code || "")), {
                method: "POST",
                body: JSON.stringify(credential || {}),
                auth: false,
            });
        }

        async function pollPasskey(code) {
            return request("/api/auth/passkey/poll", {
                method: "POST",
                body: JSON.stringify({ code: String(code || "") }),
                auth: false,
            });
        }

        async function startPasskeyRegistration(name) {
            return request("/api/auth/passkey/register/start", {
                method: "POST",
                body: JSON.stringify({ name: String(name || "") }),
            });
        }

        function listMyPasskeys() {
            return get("/api/users/me/passkeys");
        }

        function renameMyPasskey(credentialID, name) {
            return put("/api/users/me/passkeys/" + encodeURIComponent(credentialID), {
                name: String(name || ""),
            });
        }

        function deleteMyPasskey(credentialID) {
            return del("/api/users/me/passkeys/" + encodeURIComponent(credentialID));
        }

        async function register(username, password, email) {
            const payload = { username: username };
            if (password) {
                payload.password = password;
            }
            if (email) {
                payload.email = email;
            }
            return request("/api/register", {
                method: "POST",
                body: JSON.stringify(payload),
                auth: false,
            });
        }

        function listPlans() {
            return get("/api/plans");
        }

        function getDefaultPlan() {
            return get("/api/plans/default");
        }

        function setDefaultPlan(slug) {
            return post("/api/plans/default", { slug: slug });
        }

        function createPlan(payload) {
            return post("/api/plans", payload);
        }

        function updatePlan(planRef, payload) {
            return put("/api/plans/" + encodeURIComponent(planRef), payload);
        }

        function deletePlan(planRef) {
            return del("/api/plans/" + encodeURIComponent(planRef));
        }

        function listProjectAccessRequests(projectRef, status) {
            let path = "/api/projects/" + encodeURIComponent(projectRef) + "/access-requests";
            if (status) {
                path += "?status=" + encodeURIComponent(status);
            }
            return get(path);
        }

        function createProjectAccessRequest(projectRef, message) {
            return post("/api/projects/" + encodeURIComponent(projectRef) + "/access-requests", {
                message: String(message || ""),
            });
        }

        function listMyProjectAccessRequests(status) {
            let path = "/api/users/me/access-requests";
            if (status) {
                path += "?status=" + encodeURIComponent(status);
            }
            return get(path);
        }

        function listMyNotifications(status, limit) {
            let path = "/api/users/me/notifications";
            const query = [];
            if (status) {
                query.push("status=" + encodeURIComponent(status));
            }
            if (limit !== undefined && limit !== null && limit !== "") {
                query.push("limit=" + encodeURIComponent(limit));
            }
            if (query.length) {
                path += "?" + query.join("&");
            }
            return get(path);
        }

        function markNotificationRead(notificationID) {
            return post("/api/users/me/notifications/" + encodeURIComponent(notificationID) + "/read", {});
        }

        function listProjectHistory(projectRef, options) {
            const opts = options || {};
            const query = [];
            if (opts.limit !== undefined && opts.limit !== null && opts.limit !== "") {
                query.push("limit=" + encodeURIComponent(opts.limit));
            }
            if (opts.userID) {
                query.push("user_id=" + encodeURIComponent(opts.userID));
            }
            if (opts.agentID) {
                query.push("agent_id=" + encodeURIComponent(opts.agentID));
            }
            if (opts.teamID !== undefined && opts.teamID !== null && opts.teamID !== "") {
                query.push("team_id=" + encodeURIComponent(opts.teamID));
            }
            let path = "/api/projects/" + encodeURIComponent(projectRef) + "/history";
            if (query.length) {
                path += "?" + query.join("&");
            }
            return get(path);
        }

        function setProjectAccessRequestStatus(projectRef, requestID, status, message) {
            const action = status === "rejected" ? "reject" : "approve";
            return post("/api/projects/" + encodeURIComponent(projectRef) + "/access-requests/" + encodeURIComponent(requestID) + "/" + action, {
                message: String(message || ""),
            });
        }

        function setRegistrationPolicy(enabled, autoApprove) {
            return post("/api/config/registration", {
                enabled: Boolean(enabled),
                auto_approve: Boolean(autoApprove),
            });
        }

        async function fetchDocumentFile(documentID, fileID) {
            const response = await requestRaw("/api/documents/" + documentID + "/files/" + fileID, {
                method: "GET",
                json: false,
            });
            if (!response.ok) {
                throw new Error(response.status + " " + response.statusText);
            }
            return {
                blob: await response.blob(),
                fileName: extractFileName(response.headers.get("Content-Disposition"), "file-" + fileID),
                contentType: response.headers.get("Content-Type") || "",
            };
        }

        function listUsers() {
            return get("/api/users");
        }

        function createUser(username, password, email, role) {
            return post("/api/users", { username, password: password || "", email: email || "", role: role || "user" });
        }

        function enableUser(username) {
            return post("/api/users/" + encodeURIComponent(username) + "/enable", {});
        }

        function disableUser(username) {
            return post("/api/users/" + encodeURIComponent(username) + "/disable", {});
        }

        function deleteUser(username) {
            return del("/api/users/" + encodeURIComponent(username));
        }

        function resetUserPassword(username, password) {
            return post("/api/users/" + encodeURIComponent(username) + "/reset-password", { password });
        }

        function getTeamMembers(teamID) {
            return get("/api/teams/" + encodeURIComponent(teamID) + "/users");
        }

        function addTeamMember(teamID, userID, role, jobTitle) {
            return post("/api/teams/" + encodeURIComponent(teamID) + "/users", { user_id: userID, role: role, job_title: jobTitle || "" });
        }

        function removeTeamMember(teamID, userID) {
            return del("/api/teams/" + encodeURIComponent(teamID) + "/users/" + encodeURIComponent(userID));
        }

        return {
            setToken: setToken,
            getToken: getToken,
            request: request,
            requestWithFallback: requestWithFallback,
            get: get,
            post: post,
            put: put,
            del: del,
            login: login,
            startPasskeyLogin: startPasskeyLogin,
            startPasskeyRegistration: startPasskeyRegistration,
            getPasskeyChallenge: getPasskeyChallenge,
            finishPasskeyFlow: finishPasskeyFlow,
            pollPasskey: pollPasskey,
            listMyPasskeys: listMyPasskeys,
            renameMyPasskey: renameMyPasskey,
            deleteMyPasskey: deleteMyPasskey,
            register: register,
            listPlans: listPlans,
            getDefaultPlan: getDefaultPlan,
            setDefaultPlan: setDefaultPlan,
            createPlan: createPlan,
            updatePlan: updatePlan,
            deletePlan: deletePlan,
            listProjectAccessRequests: listProjectAccessRequests,
            createProjectAccessRequest: createProjectAccessRequest,
            listMyProjectAccessRequests: listMyProjectAccessRequests,
            listMyNotifications: listMyNotifications,
            markNotificationRead: markNotificationRead,
            listProjectHistory: listProjectHistory,
            setProjectAccessRequestStatus: setProjectAccessRequestStatus,
            setRegistrationPolicy: setRegistrationPolicy,
            fetchDocumentFile: fetchDocumentFile,
            listUsers: listUsers,
            createUser: createUser,
            enableUser: enableUser,
            disableUser: disableUser,
            deleteUser: deleteUser,
            resetUserPassword: resetUserPassword,
            getTeamMembers: getTeamMembers,
            addTeamMember: addTeamMember,
            removeTeamMember: removeTeamMember,
        };
    }

    return {
        createClient: createClient,
    };
}));
