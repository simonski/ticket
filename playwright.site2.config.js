const { defineConfig } = require("@playwright/test");

module.exports = defineConfig({
  testDir: "./tests/playwright",
  testMatch: "site2.spec.js",
  timeout: 30000,
  use: {
    baseURL: "http://127.0.0.1:4174",
    headless: true,
  },
  webServer: {
    command: "python3 -m http.server 4174 -d web",
    url: "http://127.0.0.1:4174/site2",
    reuseExistingServer: true,
    timeout: 30000,
  },
});
