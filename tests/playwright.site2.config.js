const { defineConfig } = require("@playwright/test");
const { resolvePort } = require("./playwright.port");

const port = resolvePort("PLAYWRIGHT_SITE2_PORT", 4174);
const retries = Number(process.env.PLAYWRIGHT_RETRIES || 1);

module.exports = defineConfig({
  testDir: "./playwright",
  testMatch: "site2.spec.js",
  fullyParallel: true,
  workers: 4,
  timeout: 30000,
  retries: Number.isInteger(retries) && retries >= 0 ? retries : 1,
  use: {
    baseURL: `http://127.0.0.1:${port}`,
    headless: true,
  },
  webServer: {
    command: `python3 serve-site.py ${port}`,
    url: `http://127.0.0.1:${port}/`,
    reuseExistingServer: true,
    timeout: 30000,
  },
});
