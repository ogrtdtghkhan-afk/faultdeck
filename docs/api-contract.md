# FaultDeck v0.2 API contract

The control UI defaults to `http://127.0.0.1:7331`, the HTTP reverse proxy to port 7332, and the built-in demo upstream to 7333. The binary defaults to loopback; `--listen` enables authenticated remote/container use. A new workspace starts with the demo upstream, the fault switch enabled, and no rules. The embedded UI has no external assets or runtime dependencies.

Every API mutation must include `X-FaultDeck: 1`. JSON requests use `Content-Type: application/json`. Same-origin only. Errors are JSON `{ "error": "..." }` with an appropriate non-2xx status.

When `FAULTDECK_ADMIN_PASSWORD` is set, all control resources require HTTP Basic Auth (`FAULTDECK_ADMIN_USER`, default `admin`). The proxy separately requires `X-FaultDeck-Token`, configured by `FAULTDECK_PROXY_TOKEN` or falling back to the admin password. The token is removed before forwarding; business `Authorization` headers are preserved. Credentials never appear in state or scenario exports. `GET /healthz` returns an empty 200 without authentication.

Configuration and rule changes are persisted when `--data-dir` is nonempty (default `./data`). A failed save returns 500 and leaves active configuration unchanged. Invalid input returns 400 and missing rule IDs return 404. Restart restores configuration and assigns runtime IDs, while clearing logs and counters.

## State

`GET /api/state` returns:

```json
{"version":"0.2.1","proxyUrl":"http://127.0.0.1:7332","demoUrl":"http://127.0.0.1:7333","upstream":"http://127.0.0.1:7333","enabled":true,"persistent":true,"proxyAuth":false,"startedAt":"2026-09-18T00:00:00Z","rules":[],"stats":{"requests":0,"injected":0,"errors":0,"avgDurationMs":0},"logs":[]}
```

Poll state every 1200ms, preserving current editing state. Logs are newest first, bounded to 200. A log contains `id, time, method, path, status, durationMs, ruleId, ruleName, fault, error`. Status 0 means connection closed without HTTP response. Logs contain metadata only, never bodies, headers, or query strings. UI must render all dynamic data with textContent / safe DOM methods.

`PUT /api/config` accepts `{ "upstream": "http://localhost:8000", "enabled": true }` (either optional). Returns state. User-supplied targets must be http(s), no credentials/query/fragment. It may include a base path. Targets pointing to FaultDeck's own control/proxy port are rejected.

## Rules

```json
{"id":"r-123","name":"Slow orders","enabled":true,"method":"GET","path":"/api/orders","type":"latency","delayMs":2000,"statusCode":503,"every":1,"limit":0,"matched":0,"hits":0}
```

- type: `latency` (delay then forward), `status` (return configured status before forwarding), `timeout` (wait delayMs, then 504 if client still connected), `disconnect` (close the client connection before forwarding).
- method: `*`, `GET`, `POST`, `PUT`, `PATCH`, `DELETE`, `HEAD`, `OPTIONS`.
- path: exact path, `*` for all, or a trailing `*` prefix, e.g. `/api/*`. No other wildcard syntax.
- delayMs: integer 0–60000 (timeout requires at least 1). statusCode: integer 400–599. every: integer 1–100000. limit: integer 0–100000; zero means unlimited.
- first eligible rule wins; every applies to the matching request counter; limit caps injected requests. A rule with `every:1, limit:2` fails twice, then falls through. Each rule's matched counter increments when considered and matched. Updating a rule resets its counters.

`POST /api/rules` adds a rule and returns it (201). Omit id and counters. `PUT /api/rules/{id}` replaces a rule and returns it. `DELETE /api/rules/{id}` deletes it (204). `POST /api/reset` clears logs, totals and all rule counters, retaining configuration and rules; returns state. `DELETE /api/logs` clears only logs (204).

## Scenarios

`GET /api/scenario` exports `{ "version":1, "name":"My scenario", "upstream":"...", "enabled":true, "rules":[...] }`, without runtime counters. `POST /api/scenario` imports this object atomically and returns state. Rule IDs may be omitted; a fresh unique ID is assigned on import. Validate the whole scenario before replacing active settings. UI downloads a JSON file / imports a local file without uploading to any external service. Maximum 100 rules and 1 MiB request body.

## Playground

`POST /api/play` accepts `{ "method":"GET", "path":"/api/orders" }` and sends a single request through the proxy to the currently configured target. Only GET/HEAD are accepted to avoid accidental writes. Returns `{ "status":200, "durationMs":1.4, "body":"JSON response text", "error":"" }`. On disconnect status is 0 and error is populated. The response is capped at 64 KiB and the playground deadline is 10 seconds. This is a real proxy request and appears in the activity log. Surface that playground requests go to the configured upstream; use built-in demo for one-click presets.

Demo endpoints: `GET /api/orders`, `GET /api/products`, `GET /api/health`, `GET /api/events` (SSE passthrough smoke check only, not fault-aware event manipulation). A root path returns an informational JSON response. The dashboard playground defaults to `/api/orders`.

## UI direction

The playground uses an internal proxy URL and supplies its access token server-side, so it works behind an HTTPS reverse proxy without exposing the token to the browser. It does not supply business credentials; use your own client for upstream APIs requiring custom headers or request bodies.

The control panel starts with the real backend connection workflow: set the upstream, point the client at the proxy, supply the proxy token if required, and create a fault rule. It shows persistence status and the effective target. Presets create backend rules. Dynamic content uses safe DOM methods, and client snippets contain credential placeholders only. The layout is keyboard accessible and responsive, with no external fonts, icons, or CDN dependencies.
