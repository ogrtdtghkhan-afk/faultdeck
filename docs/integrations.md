# Use FaultDeck with a real client

These examples connect an application to the proxy on port **7332**. The control panel uses **7331**; it is not the API endpoint for your application. Everything below is for local development.

## Node.js: automatically fail twice, then recover

The [retry client on GitHub main](https://github.com/ogrtdtghkhan-afk/faultdeck/blob/main/examples/clients/retry.mjs) requires Node.js 22 or later and has no package dependencies. It was added after v0.1.0: the currently published v0.1.0 archives contain the scenario, but **do not contain this client**. Use a current source checkout as recommended below, or follow the archive instructions to save the script separately.

### Recommended: run from the main checkout

Requires Go 1.24 or later as well as Node.js. In terminal one, clone the repository and start FaultDeck from its root:

```sh
git clone --branch main https://github.com/ogrtdtghkhan-afk/faultdeck.git
cd faultdeck
go run ./cmd/faultdeck --scenario examples/fail-twice.json
```

In terminal two, open the same cloned `faultdeck` directory, then run:

```sh
node examples/clients/retry.mjs
```

The client makes the retries automatically. Expected output from a fresh scenario:

```text
GET http://127.0.0.1:7332/api/orders
Attempt 1/3 -> HTTP 503
Waiting 250 ms before retry...
Attempt 2/3 -> HTTP 503
Waiting 250 ms before retry...
Attempt 3/3 -> HTTP 200
Success after 3 attempts.
```

Open `http://127.0.0.1:7331` to see the three requests. Each run uses the scenario's shared counters: playground requests and other clients can consume its two injected failures first. For the same sequence again, restart FaultDeck with the scenario or deliberately use **Reset** in the control panel. The example never imports scenarios, changes rules, or calls the control API.

### Alternative: use the published v0.1.0 binary

This option needs Node.js but does not need Go.

1. Extract the v0.1.0 archive and enter its `faultdeck_<version>_<os>_<arch>` directory, which contains the executable and `examples/fail-twice.json`.
2. Create a `clients` folder inside `examples`.
3. Open the [raw retry.mjs from main](https://raw.githubusercontent.com/ogrtdtghkhan-afk/faultdeck/main/examples/clients/retry.mjs) and save it as `examples/clients/retry.mjs` inside that extracted directory. Save the raw JavaScript, and make sure your browser does not add a `.txt` extension.

In terminal one, from the extracted directory:

```sh
# macOS / Linux
./faultdeck --scenario examples/fail-twice.json
```

```powershell
# Windows PowerShell
.\faultdeck.exe --scenario examples/fail-twice.json
```

In terminal two, from the same extracted directory, run `node examples/clients/retry.mjs`. The expected output is the same sequence shown above. The separately downloaded script is from main, not part of the original v0.1.0 archive.

### Retry policy and limits

- Only `GET /api/orders` is sent; there are at most **three application attempts**.
- Only HTTP **429** and **503** are retried. Other errors, redirects, timeouts, and disconnects stop with exit code 1. Redirects are not followed.
- Each attempt has a **3-second deadline**. Missing or invalid `Retry-After` uses a **250 ms** delay. Valid nonnegative integer seconds and HTTP dates are honored.
- The maximum retry wait is **2 seconds**. If the server requests more, the client stops rather than retrying before the server's deadline. There is no wait after the final failed attempt.
- A 2xx response prints success and exits 0; exhaustion prints the last status and exits 1. Response bodies are not buffered or printed.

This is a small demonstration policy, not a universal production retry strategy. Only apply retries to operations whose effects and idempotency you understand; real applications may also need jitter, a total time budget, and service-specific handling. The example intentionally keeps its attempts predictable.

To use a different proxy port, pass its origin explicitly:

```sh
node examples/clients/retry.mjs --target http://127.0.0.1:17332
```

`--target` accepts an HTTP(S) origin without credentials, an endpoint path, a query, or a fragment; the request path remains `/api/orders`. The default is `http://127.0.0.1:7332`. Point it only at a development service you intend to call. No bearer tokens or API keys are supplied.

From a current main checkout, run the local mock-server tests with:

```sh
node --test examples/clients/retry.test.mjs
```

The tests use temporary loopback ports. They cover successful recovery, the three-attempt cap and exit code, rate-limit timing decisions, non-retryable responses, redirects, timeout, and target validation. The archive instructions above download only the client; the test file is available in the main checkout.

## Vite: keep browser API requests on the same origin

For an existing Vite project, merge this into its `vite.config.js` or `vite.config.ts`, preserving your existing plugins and other options:

```js
import { defineConfig } from "vite";

export default defineConfig({
  server: {
    host: "127.0.0.1",
    proxy: {
      "/api": {
        target: "http://127.0.0.1:7332",
        changeOrigin: true,
      },
    },
  },
});
```

Restart your existing Vite dev server with its usual command. Keep the browser-side base URL relative:

```js
const response = await fetch("/api/orders");
if (!response.ok) throw new Error(`Orders request failed: ${response.status}`);
const orders = await response.json();
```

This snippet performs one request. Use your application's retry logic separately if you want automatic recovery.

```text
Browser /api/orders -> Vite dev server -> FaultDeck :7332 -> configured upstream
```

There is no path rewrite: FaultDeck sees `/api/orders`, so the included rule matches. The browser contacts its own Vite origin; Vite performs the cross-origin hop on the server. This allows the browser to observe injected error statuses even when an injected response has no CORS headers. An absolute browser URL such as `http://127.0.0.1:7332/api/orders` bypasses the Vite proxy and may encounter CORS restrictions. `changeOrigin` changes the forwarded Host header; it does not disable browser security. See [Vite's dev proxy documentation](https://vite.dev/config/server-options.html#server-proxy).

To test your existing backend, start FaultDeck with `--target http://127.0.0.1:8000`, then add rules in its control panel. `--target` and `--scenario` are mutually exclusive. Use the included scenario for the demo API, or explicitly configure a scenario with your development upstream.

Keep this routing in the **development server only**. Do not point a production API base URL, deployed reverse proxy, or production build at FaultDeck. Vite's `server.proxy` does not configure production hosting. Keep both local services on loopback; no wildcard CORS setting or disabled TLS verification is needed for this example.

## 中文速览

**当前 v0.1.0 压缩包尚未包含这个客户端。** 推荐克隆 main，在仓库根目录运行 `go run ./cmd/faultdeck --scenario examples/fail-twice.json`，再在同一目录打开第二个终端，运行 `node examples/clients/retry.mjs`。使用 v0.1.0 二进制包时，先按上文把原始脚本另存到解压目录的 `examples/clients/retry.mjs`，再用该目录中的 `./faultdeck` 或 `.\faultdeck.exe` 加载场景。

客户端会自动请求三次，得到 **503、503、200**。它只重试 429/503，最多三次，不会修改任何故障规则。重新演示前重启场景或在控制台手动 Reset，避免旧计数影响结果。

Vite 项目在开发配置中把 `/api` 代理到 `http://127.0.0.1:7332`，前端继续调用 `fetch("/api/orders")`，即可通过同源请求观察错误响应。**7331 是控制面板，7332 才是业务请求代理。** 这套配置只用于本地开发，不能用于生产流量。
