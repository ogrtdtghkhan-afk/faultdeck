# FaultDeck roadmap

FaultDeck helps developers reproduce HTTP failures locally while checking loading states, error handling, and retries against an existing API.

## Available in v0.1

- A loopback HTTP reverse proxy and embedded browser control panel in one Go executable.
- Latency, configured 400–599 statuses, bounded timeouts, and dropped connections.
- Method/path matching, ordered rules, every-Nth-request injection, and injection limits.
- In-memory request metadata, a GET/HEAD playground, reset, and JSON scenarios.
- A built-in demo API and five examples that work without external services or datasets.

The main branch also includes a [Node.js retry client and Vite development-proxy guide](integrations.md), plus a [browser simulation](https://ogrtdtghkhan-afk.github.io/faultdeck/). The client was added after the v0.1.0 archives; see the guide for both installation paths.

SSE is passed through. Event-level stream faults and guaranteed WebSocket support are outside the current scope. Activity does not capture bodies, headers, or query strings.

## Next priorities

1. **Dependable first use.** Address confirmed startup, scenario, port-conflict, or platform defects. Improve troubleshooting from actual reports.
2. **Clear integration examples.** Improve the existing client and browser development-proxy guide from installation and integration feedback.
3. **Reproducible requests.** Consider a “Copy curl” action for playground GET/HEAD requests, with clear target/path handling.
4. **Reusable scenarios.** Consider a deliberate CLI target override while retaining visible effective configuration and testing precedence.

These are candidate improvements, not a delivery schedule. Confirm a recurring problem before adding a feature, and prefer the smallest change that resolves it.

## Feedback that helps

- Which step prevented your first successful request?
- How does your development client connect to its backend?
- Which failure is difficult to reproduce using the existing rules?
- What would make you use the tool again?

A small synthetic reproduction is sufficient. No proprietary data, model APIs, or telemetry are required. Please use [issues](https://github.com/ogrtdtghkhan-afk/faultdeck/issues) for concrete bugs and feature proposals.

Hosted accounts, a plugin marketplace, TCP shaping, and new AI dependencies are outside the next iteration. We will keep the scope small enough to maintain reliable local behavior.
