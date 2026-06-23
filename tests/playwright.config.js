const path = require("path");
const { defineConfig } = require("@playwright/test");
const { resolvePort } = require("./playwright.port");

// Repo root, so the static-server `-d web/static` resolves regardless of where
// playwright is invoked from (its webServer cwd otherwise defaults to this
// config's directory, tests/, where web/static does not exist).
const repoRoot = path.resolve(__dirname, "..");

const port = resolvePort("PLAYWRIGHT_PORT", 4173);
const workers = Number(process.env.PLAYWRIGHT_WORKERS || 4);
const retries = Number(process.env.PLAYWRIGHT_RETRIES || 1);

module.exports = defineConfig({
  testDir: "./playwright",
  testIgnore: "site2.spec.js",
  timeout: 30000,
  retries: Number.isInteger(retries) && retries >= 0 ? retries : 1,
  workers: Number.isInteger(workers) && workers > 0 ? workers : 1,
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    headless: true,
  },
  webServer: {
    command: `python3 -m http.server ${port} -d web/static`,
    cwd: repoRoot,
    url: `http://127.0.0.1:${port}`,
    reuseExistingServer: true,
    timeout: 30000,
  },
});
