# Security

FaultDeck is a development and test tool. Its control panel changes where requests are forwarded and which requests fail, so administrator access is a trusted capability.

## Trust boundary

- The standalone binary binds to `127.0.0.1` by default. Without credentials configured, local users and processes can use it. The built-in demo remains on loopback for every deployment.
- Non-loopback binding and a public control origin require `FAULTDECK_ADMIN_PASSWORD` (at least 12 bytes). The administrator username defaults to `admin`. The whole control interface uses HTTP Basic Auth; `/healthz` is an empty, unauthenticated liveness response.
- Authenticated deployments also require `X-FaultDeck-Token` on proxy traffic. Set a separate `FAULTDECK_PROXY_TOKEN` for clients; if absent it defaults to the administrator password. This header is stripped before upstream forwarding. Business `Authorization` headers pass through unchanged.
- Client libraries must not forward the proxy token when following an upstream redirect to another origin. The generated Node.js snippet and bundled retry client disable automatic redirect following. Use a separate proxy token and configure other clients accordingly.
- Browser control API mutations additionally require a trusted origin and `X-FaultDeck: 1`. Host checks accept loopback names and the explicit `--ui-origin`; forwarded headers do not grant trust.
- Compose publishes ports on the host's loopback only. Use an SSH tunnel or an HTTPS reverse proxy for remote access; Basic Auth and proxy tokens require encrypted transport across untrusted networks. See [deployment](docs/deployment.md). This is one administrator credential, not a multi-user account/RBAC system.
- Upstreams can intentionally be local or private HTTP(S) services. Imported scenarios can change the upstream. Inspect files from other people and verify the displayed target before sending requests.
- The playground sends real GET/HEAD requests to the configured target. Use the included demo for experimentation; some upstreams may have side effects even for GET requests.
- Activity logs retain metadata in memory, including request paths. Bodies, headers, and query strings are not recorded there, but a path may contain sensitive identifiers. The playground displays a bounded response body in the browser.
- Exported scenarios include the upstream URL and rule paths. Review them before posting publicly.
- The data directory saves only workspace configuration, never deployment credentials or request logs. Configuration files use mode 0600 on Unix. Do not share a data directory between running instances. Back up the exported scenario or workspace file before replacing a volume.

Use FaultDeck with systems you are authorized to test. It is not a production gateway, a network isolation boundary, or a credential vault.

## Supported versions

Security fixes target the latest released version. During the initial 0.x series, upgrade to the latest release rather than expecting maintenance of older minor releases.

## Report a vulnerability

Use the repository's **Security → Advisories → Report a vulnerability** option when private reporting is available. Include the affected version, a minimal local reproduction, and the impact. Please avoid real credentials or private production data.

If private reporting is unavailable, open a public issue asking for a private reporting channel without disclosing exploit details. An acknowledgement time or resolution deadline is not guaranteed.
