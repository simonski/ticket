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
                throw new Error(toErrorMessage(response, body));
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
            fetchDocumentFile: fetchDocumentFile,
        };
    }

    return {
        createClient: createClient,
    };
}));
