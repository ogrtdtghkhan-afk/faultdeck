# Security

FaultDeck is a local development tool. Its control panel changes where requests are forwarded and which requests fail, so access to that panel is a trusted capability.

## Trust boundary

- All listeners bind to `127.0.0.1`. There is no account system or authentication for local users and processes.
- Browser control API mutations require a same-origin request and `X-FaultDeck: 1`. These are protections against unintended browser requests, not credentials or an authorization system.
- Do not expose the control port through a public reverse proxy, tunnel, or port-forwarding service.
- Upstreams can intentionally be local or private HTTP(S) services. Imported scenarios can change the upstream. Inspect files from other people and verify the displayed target before sending requests.
- The playground sends real GET/HEAD requests to the configured target. Use the included demo for experimentation; some upstreams may have side effects even for GET requests.
- Activity logs retain metadata in memory, including request paths. Bodies, headers, and query strings are not recorded there, but a path may contain sensitive identifiers. The playground displays a bounded response body in the browser.
- Exported scenarios include the upstream URL and rule paths. Review them before posting publicly.

Use FaultDeck with systems you are authorized to test. It is not a production gateway, a network isolation boundary, or a credential vault.

## Supported versions

Security fixes target the latest released version. During the initial 0.x series, upgrade to the latest release rather than expecting maintenance of older minor releases.

## Report a vulnerability

Use the repository's **Security → Advisories → Report a vulnerability** option when private reporting is available. Include the affected version, a minimal local reproduction, and the impact. Please avoid real credentials or private production data.

If private reporting is unavailable, open a public issue asking for a private reporting channel without disclosing exploit details. An acknowledgement time or resolution deadline is not guaranteed.
