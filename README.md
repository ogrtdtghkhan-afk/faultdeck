# FaultDeck

**Break it here. Ship it stronger.**

An HTTP proxy with a control panel for slow responses, errors, timeouts, and dropped connections. Run it locally or deploy an authenticated service with saved rules. Point your app at FaultDeck, choose a fault, and see whether recovery actually works.

[中文文档](README.zh-CN.md) · [Download](https://github.com/ogrtdtghkhan-afk/faultdeck/releases/latest) · [Report an issue](https://github.com/ogrtdtghkhan-afk/faultdeck/issues)

[Deploy the working service](docs/deployment.md) · [Interactive overview](https://ogrtdtghkhan-afk.github.io/faultdeck/) (browser simulation)

[![CI](https://github.com/ogrtdtghkhan-afk/faultdeck/actions/workflows/ci.yml/badge.svg)](https://github.com/ogrtdtghkhan-afk/faultdeck/actions/workflows/ci.yml)

```text
Your app ──► localhost:7332 ──► Your backend
                   ▲
          FaultDeck control panel
            localhost:7331
```

- **Click to break a request.** Add latency, return 429/503, simulate a timeout, or close the connection.
- **Fail twice, then recover.** Apply faults every Nth matching request and stop after a chosen number of injections.
- **See what happened.** Inspect request metadata, applied rules, response status, and duration.
- **Keep scenarios in your repo.** Export JSON and replay it with the CLI or control panel.
- **Try it immediately.** An included demo API provides orders, products, health, and an SSE stream. No API keys or datasets.
- **One executable.** Embedded UI, Go standard library, no runtime services, account, or frontend build step.

![FaultDeck after two injected failures: the rule is complete and the next request returns 200.](docs/assets/fail-twice.png)

## Deploy with Docker

Deploy the real control panel and proxy with authentication and saved rules:

```sh
git clone https://github.com/ogrtdtghkhan-afk/faultdeck.git
cd faultdeck
cp .env.example .env
```

In PowerShell, use `Copy-Item .env.example .env`. Set `FAULTDECK_ADMIN_PASSWORD` in `.env` to your own random password of at least 12 characters, then run:

```sh
docker compose up --build -d
docker compose ps
```

When healthy, open **http://127.0.0.1:7331** and sign in as `admin` with that password. Set your real upstream in the UI. Applications call **http://127.0.0.1:7332** with `X-FaultDeck-Token` (the admin password unless a separate token is configured). Rules and configuration survive container replacement through the named data volume.

See the [deployment guide](docs/deployment.md) for host/container networking, remote access, credentials, persistence, and the real container acceptance test. Use the current source checkout; the older v0.1.0 archive does not contain this deployment setup.

## Try the local executable

Download and extract the archive for your platform from [Releases](https://github.com/ogrtdtghkhan-afk/faultdeck/releases/latest). Open a terminal inside the extracted `faultdeck_<version>_<os>_<arch>` folder (the folder containing the executable), then run:

```sh
# macOS / Linux
./faultdeck
```

```powershell
# Windows PowerShell
.\faultdeck.exe
```

Open **http://127.0.0.1:7331**. On a fresh workspace with no target configured, FaultDeck uses its built-in demo API. Subsequent runs restore the saved target and rules from `./data`.

1. Send a request to `/api/orders` in the playground: it succeeds.
2. Add the **Fail twice** preset and send the request three times.
3. Inspect the activity: `503`, `503`, then the real upstream response.

The playground sends real requests to the configured target. The built-in demo is the place to try the presets first. After downloading, the demo works offline. Native execution defaults to loopback access; authentication can be enabled with the environment variables below. Use the Docker instructions for a deployed service.

### Run from source

Requires Go 1.24 or later.

```sh
git clone https://github.com/ogrtdtghkhan-afk/faultdeck.git
cd faultdeck
go run ./cmd/faultdeck
```

## Connect your app

If your backend listens on port 8000:

```sh
./faultdeck --target http://127.0.0.1:8000
```

Set your application's development API base URL to `http://127.0.0.1:7332`. When proxy authentication is enabled, include `X-FaultDeck-Token` with the configured token; business `Authorization` is preserved. Open the control panel, create a rule for the endpoint you want to test, and exercise your application normally.

Want a complete example? Follow the [Node.js retry client and Vite integration guide](docs/integrations.md) to see automatic `503 → 503 → 200` recovery and connect a browser app through its development proxy.

The target can be HTTP or HTTPS and may include a base path. The local proxy itself listens over HTTP. An HTTPS web page may therefore need your development server's own proxy to reach it without browser mixed-content restrictions. FaultDeck forwards upstream CORS behavior; it does not automatically make a cross-origin API accessible.

Native control and proxy listeners default to `127.0.0.1`; `--listen` can change that. Remote binding requires authentication. The built-in demo remains on loopback. Docker publishes its control and proxy ports only on host loopback by default; see the [deployment guide](docs/deployment.md) for networking and remote access.

## Faults that mean what they say

| Fault | Behavior | Useful for |
| --- | --- | --- |
| Latency | Wait, then forward the request normally | Loading states, slow networks |
| Status | Return a chosen 400–599 status without contacting the upstream | Auth errors, rate limits, retry handling |
| Timeout | Wait for the configured duration, then return 504 if the client is still connected | Client deadlines, timeout UI |
| Disconnect | Close the client connection without an HTTP response or upstream request | Network failure handling |

Timeout is a bounded simulation, not an indefinitely hanging socket. Set its duration above your client's timeout to exercise that deadline. The playground has its own 10-second deadline.

### Matching and ordering

- Match an HTTP method or `*`, and an exact path, `*`, or trailing-prefix pattern such as `/api/*`.
- Rules are evaluated in display order. The **first eligible rule wins**; faults do not stack.
- `every: 1` injects on every matching request; `every: 3` injects on the third, sixth, ninth, and so on.
- `limit: 2` injects twice, then lets later rules or the upstream handle subsequent requests. `limit: 0` is unlimited.
- A rule's matching counter advances only when evaluation reaches that enabled rule and its method/path match. Requests consumed by an earlier rule do not advance later rules.
- Editing a rule resets its counters. Reset clears all counters, totals, and request history while retaining rules and configuration.

For example, `every: 1, limit: 2, type: "status", statusCode: 503` means **fail the first two matching requests, then recover**. `every: 2, limit: 2` injects on matching requests two and four.

## Save a scenario

Use the control panel to import or export JSON, or start with an included example:

```sh
./faultdeck --scenario examples/fail-twice.json
```

```json
{
  "version": 1,
  "name": "Fail twice, then recover",
  "upstream": "http://127.0.0.1:7333",
  "enabled": true,
  "rules": [
    {
      "name": "Orders recover after two failures",
      "enabled": true,
      "method": "GET",
      "path": "/api/orders",
      "type": "status",
      "statusCode": 503,
      "every": 1,
      "limit": 2
    }
  ]
}
```

Examples use the built-in demo's default port, 7333. See also [slow orders](examples/slow-orders.json), [every third request rate-limited](examples/rate-limit.json), [timeout](examples/timeout.json), and [disconnect once](examples/disconnect-once.json). Runtime counters are not exported, so a newly imported scenario starts fresh.

Scenario files include their upstream URL and rule paths. Review them for private hostnames and identifiers before sharing. A scenario import can change the target; check the displayed upstream before sending requests.

## CLI

| Flag | Default | Purpose |
| --- | --- | --- |
| `--target` | Saved target, otherwise demo | Explicitly override the upstream HTTP(S) URL |
| `--listen` | `127.0.0.1` | Control and proxy bind address |
| `--port` | `7332` | Proxy port |
| `--ui-port` | `7331` | Control panel port |
| `--demo-port` | `7333` | Built-in demo port |
| `--data-dir` | `./data` | Save configuration and rules; `--data-dir=` disables persistence |
| `--ui-origin` | Local control origin | Advertised browser origin, including scheme and port |
| `--proxy-url` | Local proxy origin | Advertised application proxy origin |
| `--scenario` | None | Import a scenario JSON file on startup |
| `--healthcheck` | — | Probe the internal control listener and exit 0/1 |
| `--version` | — | Print the version and exit |

`--target` and `--scenario` are mutually exclusive because the scenario includes its target. Use `--help` for the executable's flag reference. The [HTTP API contract](docs/api-contract.md) describes programmatic control.

Authentication uses environment variables: `FAULTDECK_ADMIN_USER` defaults to `admin`; `FAULTDECK_ADMIN_PASSWORD` enables Basic Auth and must be at least 12 characters; `FAULTDECK_PROXY_TOKEN` can supply a separate token of at least 12 characters and otherwise defaults to the admin password. Remote access requires an admin password. The supplied Compose deployment requires it even though the published host ports are local.

## Scope and stored data

FaultDeck v0.2 supports native and container deployments for development/test HTTP request faults. SSE is passed through; individual event corruption or event-level delays are not implemented. WebSocket behavior is not guaranteed. It is not a TCP traffic shaper, HTTPS interception proxy, or load-testing service.

The upstream, master switch, and rules are saved in the configured data directory. Native execution uses `./data`; Docker uses the `/data` volume. Runtime counters and activity are not saved and restart from zero after the service restarts.

The last 200 requests are kept in memory as metadata only: method, path, status, duration, and fault details. Request/response bodies, headers, and query strings are not recorded in activity logs. The playground displays up to 64 KiB of the requested response. Paths may still contain sensitive identifiers.

The control API checks the configured origin and requires `X-FaultDeck: 1` for mutations. With authentication enabled, the UI and control API require Basic Auth, and application requests require `X-FaultDeck-Token`; that token is stripped before forwarding. `/healthz` is the minimal unauthenticated health endpoint. Use the [deployment guide](docs/deployment.md) for SSH access or an authenticated HTTPS reverse proxy. For the trust boundary and reporting process, see [SECURITY.md](SECURITY.md).

## Develop

```sh
go test ./...
go vet ./...
go build ./cmd/faultdeck
```

On a machine with a supported C compiler, also run `go test -race ./...`. The repository includes CI for Go 1.24 and stable on Linux, Windows, and macOS, plus a Linux race check. Tagged releases build archives for `amd64` and `arm64` on each platform with SHA-256 checksums.

Bug reports with a small reproduction, practical fault scenarios, and documentation improvements are welcome. See [CONTRIBUTING.md](CONTRIBUTING.md).

See the [roadmap](docs/roadmap.md) for current scope and candidate improvements.

## License

[MIT](LICENSE).
