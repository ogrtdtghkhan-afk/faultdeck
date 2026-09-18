# Use FaultDeck with a real client

These examples connect an application to the proxy on port **7332**. The control panel uses **7331**; it is not the API endpoint for your application. Use a local or self-hosted development instance; these examples are not for production traffic.

## Node.js: automatically fail twice, then recover

The [retry client](https://github.com/ogrtdtghkhan-afk/faultdeck/blob/main/examples/clients/retry.mjs) requires Node.js 22 or later and has no package dependencies. It is included in the v0.2.0 release archives and the source checkout.

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

### Alternative: use the v0.2.0 binary

This option needs Node.js but does not need Go.

Extract the v0.2.0 archive and enter its `faultdeck_<version>_<os>_<arch>` directory. It contains the executable, `examples/fail-twice.json`, and `examples/clients/retry.mjs`.

In terminal one, from the extracted directory:

```sh
# macOS / Linux
./faultdeck --scenario examples/fail-twice.json
```

```powershell
# Windows PowerShell
.\faultdeck.exe --scenario examples/fail-twice.json
```

In terminal two, from the same extracted directory, run `node examples/clients/retry.mjs`. The expected output is the same sequence shown above. Older v0.1.0 archives do not include the client; upgrade or use the current source checkout.

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

`--target` accepts an HTTP(S) origin without credentials, an endpoint path, a query, or a fragment; the request path remains `/api/orders`. The default is `http://127.0.0.1:7332`. Point it only at a development service you intend to call. The client does not supply upstream bearer tokens or API keys. Optional FaultDeck proxy authentication uses a separate header, described below.

### Connect to a proxy that requires a token

The current client optionally reads `FAULTDECK_PROXY_TOKEN` from its environment and sends it as `X-FaultDeck-Token` on each request. It never places the token in the URL, prints it, or uses the application's `Authorization` header for proxy authentication. With the variable unset or empty, no proxy token header is sent, so the local unauthenticated demo and v0.1.0 still work.

For the Docker deployment, leaving the server's `FAULTDECK_PROXY_TOKEN` empty makes its proxy token equal to `FAULTDECK_ADMIN_PASSWORD`. The client does not read the admin password automatically: in that configuration, its `FAULTDECK_PROXY_TOKEN` must contain that same password. Prefer setting a **separate random proxy token** on the server and client so client access does not also grant control-panel access.

When running the client from the deployment checkout, put that separate token in the ignored `.env` file along with the deployment settings, then use Node.js 22 or later's environment-file support:

```sh
node --env-file=.env examples/clients/retry.mjs
```

For a different host or published port, keep the token in the environment and pass only the origin:

```sh
node --env-file=.env examples/clients/retry.mjs --target https://faultdeck-api.example.test
```

Use the proxy's HTTPS origin when connecting across machines. This client does not disable TLS certificate verification. A missing or wrong token returns 401 and stops immediately under the existing retry policy; it is not retried. The example still only requests `/api/orders` and never calls the control API.

From a current main checkout, run the local mock-server tests with:

```sh
node --test examples/clients/retry.test.mjs
```

The tests use temporary loopback ports. They cover successful recovery with and without a proxy token, environment-variable authentication against a real mock HTTP server, credential-free logs and URLs, the three-attempt cap and exit code, rate-limit timing decisions, non-retryable responses, redirects, timeout, and target validation.

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

**v0.2.0 压缩包已经包含客户端和场景。** 解压后进入可执行文件所在目录，运行 `./faultdeck --scenario examples/fail-twice.json`（Windows 使用 `.\faultdeck.exe`），再打开第二个终端运行 `node examples/clients/retry.mjs`。也可以克隆源码，用 `go run ./cmd/faultdeck --scenario examples/fail-twice.json` 启动。旧版 v0.1.0 不含客户端，建议升级。

客户端会自动请求三次，得到 **503、503、200**。它只重试 429/503，最多三次，不会修改任何故障规则。重新演示前重启场景或在控制台手动 Reset，避免旧计数影响结果。

连接需要认证的部署实例时，为客户端设置 `FAULTDECK_PROXY_TOKEN`；脚本只通过 `X-FaultDeck-Token` 请求头发送，不写进 URL、不打印，也不占用业务接口的 `Authorization`。Docker 服务端未设置独立 token 时，代理 token 默认等于管理员密码；建议给服务端和客户端配置相同的独立随机 token。使用部署目录的 `.env` 时，可运行 `node --env-file=.env examples/clients/retry.mjs`。未设置 token 的本地演示仍可使用；token 错误导致 401，会立即停止。

Vite 项目在开发配置中把 `/api` 代理到 `http://127.0.0.1:7332`，前端继续调用 `fetch("/api/orders")`，即可通过同源请求观察错误响应。**7331 是控制面板，7332 才是业务请求代理。** 这套配置只用于本地开发，不能用于生产流量。
