import assert from "node:assert/strict";
import { once } from "node:events";
import { readFileSync } from "node:fs";
import { createServer } from "node:http";
import test from "node:test";
import { runInNewContext } from "node:vm";

const source = readFileSync(
  new URL("../internal/web/static/app.js", import.meta.url),
  "utf8",
);
const start = source.indexOf("  function renderIntegrationExample()");
const end = source.indexOf("  function updateState(", start);
assert(start >= 0 && end > start, "locate the actual UI snippet renderer");
const renderer = source.slice(start, end);
const AsyncFunction = Object.getPrototypeOf(async function () {}).constructor;

function nodeSnippet(proxyUrl, proxyAuth) {
  // Run the production renderer against only its text fields. No copy of the
  // snippet-generation logic or expected fetch options lives in this test.
  const elements = {
    "integration-path": { value: "/api/orders" },
    "integration-code": { textContent: "" },
    "integration-auth-help": { textContent: "" },
  };
  runInNewContext(`${renderer}\nrenderIntegrationExample();`, {
    state: { proxyUrl, proxyAuth },
    exampleLanguage: "node",
    lastExample: "",
    elements,
  });
  return elements["integration-code"].textContent;
}

async function mockServer(t, handler) {
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

for (const authenticated of [true, false]) {
  test(
    `generated ${authenticated ? "authenticated" : "anonymous"} Node snippet does not follow a cross-origin redirect`,
    { timeout: 5000 },
    async (t) => {
      const token = "synthetic-ui-snippet-test-token";
      let redirectRequests = 0;
      const redirectOrigin = await mockServer(t, (_req, res) => {
        redirectRequests++;
        res.end("this origin must not receive the request or proxy token");
      });
      const received = [];
      const proxyOrigin = await mockServer(t, (req, res) => {
        received.push({
          path: req.url,
          token: req.headers["x-faultdeck-token"],
        });
        res.writeHead(302, { Location: `${redirectOrigin}/external-login` });
        res.end("redirect returned to the caller");
      });

      // Compile and execute exactly what a user copies from the UI, using real
      // Node fetch and two ephemeral loopback origins, with synthetic credentials.
      const execute = new AsyncFunction(
        "fetch",
        "process",
        "console",
        nodeSnippet(proxyOrigin, authenticated),
      );
      const output = [];
      await execute(
        fetch,
        { env: { FAULTDECK_PROXY_TOKEN: token } },
        { log: (...parts) => output.push(parts) },
      );

      assert.deepEqual(received, [
        { path: "/api/orders", token: authenticated ? token : undefined },
      ]);
      assert.equal(redirectRequests, 0, "redirect destination was contacted");
      assert.deepEqual(output, [[302, "redirect returned to the caller"]]);
    },
  );
}
