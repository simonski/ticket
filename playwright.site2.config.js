const { defineConfig } = require("@playwright/test");
const { resolvePort } = require("./playwright.port");

const port = resolvePort("PLAYWRIGHT_SITE2_PORT", 4174);

module.exports = defineConfig({
  testDir: "./tests/playwright",
  testMatch: "site2.spec.js",
  timeout: 30000,
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    headless: true,
  },
  webServer: {
    command: `python3 -m http.server ${port} -d web`,
    url: `http://127.0.0.1:${port}/site2`,
    reuseExistingServer: true,
    timeout: 30000,
  },
});
