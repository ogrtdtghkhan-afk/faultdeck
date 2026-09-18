# Changelog

## 0.2.1

First published deployment release. The v0.2.0 source tag is retained, but its binary publication was stopped for the public-port correction below.

- Deploy with Docker Compose: nonroot container, healthcheck, persistent volume, and restart policy.
- Persistent upstream, master switch, and rules, restored after a restart. Changes are saved before they become active; failed writes preserve the previous configuration.
- Authenticated server deployment: administrator Basic Auth and a separate proxy access token, without replacing upstream business credentials.
- Configurable listen address and public URLs, with an internal route for real playground requests behind a reverse proxy.
- Public host ports are kept separate from internal listeners, so port remapping does not incorrectly reject the built-in upstream or other backend ports.
- A backend connection guide in the control panel, client examples, and visible persistence/authentication status.
- End-to-end container acceptance against an independent upstream, including forwarding, fault injection, authentication, and container replacement.
- Docker deployment acceptance now gates binary releases alongside the cross-platform test suite.

The binary now saves `./data/workspace.json` by default. Use `--data-dir ""` for the previous in-memory behavior. An explicit `--target` replaces the saved target while retaining rules; an explicit `--scenario` imports that file on each startup. Logs and injection counters still reset on restart.

## 0.1.0

Initial release.

- Local HTTP reverse proxy with an embedded browser control panel.
- Latency, HTTP error, bounded timeout, and connection-disconnect rules.
- Method/path matching, every-Nth request, and injection limits.
- Scenario import/export with validation before replacement.
- Bounded in-memory request metadata and a real proxy playground.
- Built-in synthetic orders, products, health, and SSE demo endpoints.
- Loopback listeners and cross-origin protection for the control API.
