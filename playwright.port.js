const crypto = require("crypto");
const fs = require("fs");
const os = require("os");
const path = require("path");
const { execFileSync } = require("child_process");
const resolvedPorts = new Map();

function cachePath(envName) {
  const key = crypto
    .createHash("sha1")
    .update(`${process.cwd()}:${envName}`)
    .digest("hex");
  return path.join(os.tmpdir(), `ticket-playwright-port-${key}.txt`);
}

function canBindPort(port) {
  if (!Number.isInteger(port) || port <= 0) {
    return false;
  }
  try {
    execFileSync(
      "python3",
      [
        "-c",
        "import socket,sys; p=int(sys.argv[1]); s=socket.socket(); s.bind(('127.0.0.1', p)); s.close()",
        String(port),
      ],
      { stdio: "ignore" },
    );
    return true;
  } catch {
    return false;
  }
}

function probeEphemeralPort() {
  try {
    const output = execFileSync(
      "python3",
      [
        "-c",
        "import socket; s=socket.socket(); s.bind(('127.0.0.1', 0)); print(s.getsockname()[1]); s.close()",
      ],
      { encoding: "utf8" },
    ).trim();
    const parsed = Number(output);
    if (Number.isInteger(parsed) && parsed > 0) {
      return parsed;
    }
  } catch {
    // Fall back below.
  }
  return 0;
}

function resolvePort(envName, fallbackPort) {
  const memoized = resolvedPorts.get(envName);
  if (Number.isInteger(memoized) && memoized > 0) {
    return memoized;
  }
  const raw = process.env[envName];
  if (raw) {
    const parsed = Number(raw);
    if (Number.isInteger(parsed) && parsed > 0) {
      resolvedPorts.set(envName, parsed);
      return parsed;
    }
  }

  const cachedPath = cachePath(envName);
  try {
    const cached = fs.readFileSync(cachedPath, "utf8").trim();
    const parsed = Number(cached);
    if (canBindPort(parsed)) {
      resolvedPorts.set(envName, parsed);
      return parsed;
    }
    const stat = fs.statSync(cachedPath);
    if (Date.now()-stat.mtimeMs < 5 * 60 * 1000 && Number.isInteger(parsed) && parsed > 0) {
      // A sibling Playwright process likely just started the web server on this port.
      resolvedPorts.set(envName, parsed);
      return parsed;
    }
  } catch {
    // Cache miss, probe a fresh port below.
  }

  const ephemeralPort = probeEphemeralPort();
  if (canBindPort(ephemeralPort)) {
    fs.writeFileSync(cachedPath, `${ephemeralPort}\n`);
    resolvedPorts.set(envName, ephemeralPort);
    return ephemeralPort;
  }

  if (canBindPort(fallbackPort)) {
    resolvedPorts.set(envName, fallbackPort);
    return fallbackPort;
  }

  const fallbackEphemeralPort = probeEphemeralPort();
  if (canBindPort(fallbackEphemeralPort)) {
    fs.writeFileSync(cachedPath, `${fallbackEphemeralPort}\n`);
    resolvedPorts.set(envName, fallbackEphemeralPort);
    return fallbackEphemeralPort;
  }

  resolvedPorts.set(envName, fallbackPort);
  return fallbackPort;
}

module.exports = { resolvePort };
