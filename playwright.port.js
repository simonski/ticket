const crypto = require("crypto");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { execFileSync } = require("child_process");

function cachePath(envName) {
  const key = crypto
    .createHash("sha1")
    .update(`${process.cwd()}:${envName}`)
    .digest("hex");
  return path.join(os.tmpdir(), `ticket-playwright-port-${key}.txt`);
}

function resolvePort(envName, fallbackPort) {
  const raw = process.env[envName];
  if (raw) {
    const parsed = Number(raw);
    if (Number.isInteger(parsed) && parsed > 0) {
      return parsed;
    }
  }

  const cachedPath = cachePath(envName);
  try {
    const cached = fs.readFileSync(cachedPath, "utf8").trim();
    const parsed = Number(cached);
    if (Number.isInteger(parsed) && parsed > 0) {
      return parsed;
    }
  } catch {
    // Cache miss, probe a fresh port below.
  }

  try {
    const output = execFileSync("python3", [
      "-c",
      "import socket; s=socket.socket(); s.bind(('127.0.0.1', 0)); print(s.getsockname()[1]); s.close()",
    ], { encoding: "utf8" }).trim();
    const parsed = Number(output);
    if (Number.isInteger(parsed) && parsed > 0) {
      fs.writeFileSync(cachedPath, `${parsed}\n`);
      return parsed;
    }
  } catch {
    // Fall back to the historical port if probing fails.
  }

  return fallbackPort;
}

module.exports = { resolvePort };
