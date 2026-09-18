import assert from "node:assert/strict";
import { createServer } from "node:http";
import { once } from "node:events";
import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import test from "node:test";
import { parseTarget, retryDelayMs, runRetryDemo } from "./retry.mjs";

async function mockAPI(t, handler) {
  const server = createServer(handler);
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  t.after(
    () =>
      new Promise((resolve) => {
        server.close(resolve);
        server.closeAllConnections();
      }),
  );
  return `http://127.0.0.1:${server.address().port}`;
}

test("automatically retries 503, 503, 200 and reports all attempts", async (t) => {
  const statuses = [503, 503, 200];
  let calls = 0;
  const target = await mockAPI(t, (req, res) => {
    assert.equal(req.method, "GET");
    assert.equal(req.url, "/api/orders");
    assert.equal(req.headers["x-faultdeck-token"], undefined);
    res.writeHead(statuses[calls++]);
    res.end();
  });
  const lines = [];
  const waits = [];
  const result = await runRetryDemo({
    target,
    log: (line) => lines.push(line),
    wait: async (ms) => waits.push(ms),
  });
  assert.deepEqual(result, { status: 200, attempts: 3 });
  assert.equal(calls, 3);
  assert.deepEqual(waits, [250, 250]);
  assert.deepEqual(
    lines.filter((line) => line.startsWith("Attempt")),
    [
      "Attempt 1/3 -> HTTP 503",
      "Attempt 2/3 -> HTTP 503",
      "Attempt 3/3 -> HTTP 200",
    ],
  );
});

test("429 honors Retry-After before the next attempt", async (t) => {
  let calls = 0;
  const target = await mockAPI(t, (req, res) => {
    res.writeHead(++calls === 1 ? 429 : 200, { "Retry-After": "1" });
    res.end();
  });
  const waits = [];
  await runRetryDemo({ target, log() {}, wait: async (ms) => waits.push(ms) });
  assert.deepEqual(waits, [1000]);
  assert.equal(calls, 2);
});

test("CLI exhaustion stops after three requests and exits nonzero", async (t) => {
  let calls = 0;
  const target = await mockAPI(t, (req, res) => {
    calls++;
    res.writeHead(503, { "Retry-After": "0" });
    res.end();
  });
  const child = spawn(
    process.execPath,
    [
      fileURLToPath(new URL("./retry.mjs", import.meta.url)),
      "--target",
      target,
    ],
    {
      windowsHide: true,
      env: { ...process.env, FAULTDECK_PROXY_TOKEN: "" },
    },
  );
  let stdout = "";
  let stderr = "";
  child.stdout.on("data", (chunk) => {
    stdout += chunk;
  });
  child.stderr.on("data", (chunk) => {
    stderr += chunk;
  });
  const [code] = await once(child, "close");
  assert.equal(code, 1);
  assert.equal(calls, 3);
  assert.match(stdout, /Attempt 3\/3 -> HTTP 503/);
  assert.match(stderr, /Retries exhausted after 3 attempts/);
});

test("CLI reads the proxy token from its environment on every retry without logging it", async (t) => {
  const token = "test-only-proxy-token-123";
  let calls = 0;
  const target = await mockAPI(t, (req, res) => {
    assert.equal(req.url, "/api/orders");
    assert.equal(req.headers.authorization, undefined);
    if (req.headers["x-faultdeck-token"] !== token) {
      res.writeHead(401);
      res.end();
      return;
    }
    calls++;
    res.writeHead(calls < 3 ? 503 : 200, { "Retry-After": "0" });
    res.end();
  });
  const child = spawn(
    process.execPath,
    [fileURLToPath(new URL("./retry.mjs", import.meta.url)), "--target", target],
    {
      windowsHide: true,
      env: { ...process.env, FAULTDECK_PROXY_TOKEN: token },
    },
  );
  let output = "";
  child.stdout.on("data", (chunk) => {
    output += chunk;
  });
  child.stderr.on("data", (chunk) => {
    output += chunk;
  });
  const [code] = await once(child, "close");
  assert.equal(code, 0);
  assert.equal(calls, 3);
  assert.match(output, /Success after 3 attempts/);
  assert.equal(output.includes(token), false);
});

test("missing or incorrect proxy tokens stop on 401 without retrying", async (t) => {
  const expected = "test-only-expected-proxy-token";
  for (const proxyToken of ["", "test-only-incorrect-proxy-token"]) {
    let calls = 0;
    const target = await mockAPI(t, (req, res) => {
      calls++;
      assert.equal(req.url, "/api/orders");
      assert.equal(req.headers.authorization, undefined);
      res.writeHead(req.headers["x-faultdeck-token"] === expected ? 200 : 401);
      res.end();
    });
    const lines = [];
    await assert.rejects(
      runRetryDemo({
        target,
        proxyToken,
        log: (line) => lines.push(line),
        wait: async () => assert.fail("401 must not be retried"),
      }),
      /HTTP 401 is not retryable/,
    );
    assert.equal(calls, 1);
    if (proxyToken) assert.equal(lines.join("\n").includes(proxyToken), false);
  }
});

test("invalid proxy header values fail before any request or log disclosure", async () => {
  const token = "test-only-secret\r\nInjected: yes";
  const lines = [];
  await assert.rejects(
    runRetryDemo({
      proxyToken: token,
      log: (line) => lines.push(line),
    }),
    (error) => {
      assert.match(error.message, /proxyToken must be a string without newlines/);
      assert.equal(error.message.includes(token), false);
      return true;
    },
  );
  assert.deepEqual(lines, []);
});

test("non-transient errors and redirects do not cause another request", async (t) => {
  for (const status of [400, 401, 404, 500, 504, 302]) {
    let calls = 0;
    const target = await mockAPI(t, (req, res) => {
      calls++;
      res.writeHead(status, { Location: "/redirect-target" });
      res.end();
    });
    await assert.rejects(
      runRetryDemo({ target, log() {} }),
      new RegExp(`HTTP ${status} is not retryable`),
    );
    assert.equal(calls, 1, `status ${status} was retried or followed`);
  }
});

test("Retry-After supports seconds, HTTP dates, fallback, and a hard wait cap", () => {
  const now = Date.UTC(2026, 0, 1, 0, 0, 0);
  assert.equal(retryDelayMs("2", now), 2000);
  assert.equal(retryDelayMs("Thu, 01 Jan 2026 00:00:01 GMT", now), 1000);
  assert.equal(retryDelayMs("Wed, 31 Dec 2025 23:59:59 GMT", now), 0);
  for (const value of [null, "", "invalid", "-1", "1.5"])
    assert.equal(retryDelayMs(value, now), 250);
  for (const value of [
    "3",
    "99999999999999999999999999999999999999",
    "Thu, 01 Jan 2026 00:00:03 GMT",
  ]) {
    assert.throws(() => retryDelayMs(value, now), /wait cap/);
  }
});

test("a Retry-After above the cap stops without a second request", async (t) => {
  let calls = 0;
  const target = await mockAPI(t, (req, res) => {
    calls++;
    res.writeHead(429, { "Retry-After": "60" });
    res.end();
  });
  await assert.rejects(
    runRetryDemo({ target, log() {} }),
    /stopping without retrying early/,
  );
  assert.equal(calls, 1);
});

test("request deadline aborts a stalled upstream without retrying", async (t) => {
  let calls = 0;
  const target = await mockAPI(t, () => {
    calls++;
  });
  await assert.rejects(
    runRetryDemo({ target, timeoutMs: 100, log() {} }),
    /deadline exceeded/,
  );
  assert.equal(calls, 1);
});

test("target accepts an origin and rejects credentials or ambiguous endpoint configuration", () => {
  assert.equal(
    parseTarget("http://127.0.0.1:7332").origin,
    "http://127.0.0.1:7332",
  );
  assert.equal(parseTarget("https://localhost:8443/").protocol, "https:");
  for (const target of [
    "localhost:7332",
    "file:///tmp/api",
    "http://user:secret@localhost",
    "http://localhost/api",
    "http://localhost?key=x",
    "http://localhost?",
    "http://localhost#part",
    "http://localhost:0",
  ]) {
    assert.throws(() => parseTarget(target), /HTTP\(S\) origin/);
  }
});
