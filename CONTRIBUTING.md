# Contributing to FaultDeck

FaultDeck helps developers reproduce HTTP failure behavior locally. Small, practical improvements to that workflow are welcome.

## Run locally

Install Go 1.24 or later, clone the repository, and run:

```sh
go run ./cmd/faultdeck
```

Open `http://127.0.0.1:7331`. The default target is an included demo API; you do not need external services or API keys. The UI is embedded in the executable, so restart `go run` after editing its assets.

## Propose a change

For a bug, include your operating system, FaultDeck version, a minimal scenario, the request you sent, and expected versus actual behavior. Remove private URLs and identifiers before attaching a scenario or log.

For a feature, describe the development failure you need to reproduce and the simplest useful behavior. Open an issue before a substantial redesign so the scope can be discussed. Documentation fixes and small bug fixes can go straight to a pull request.

## Before submitting

```sh
gofmt -w ./cmd ./internal
go test ./...
go vet ./...
go build ./cmd/faultdeck
```

Run `go test -race ./...` if a supported C compiler is available. For backend behavior changes, add a focused regression test that uses local test servers, not public endpoints. For UI changes, check the demo flow, keyboard interaction, a narrow window, and browser errors. Include a screenshot when it helps review a visible change.

Keep runtime dependencies to the Go standard library and bundled UI assets unless a dependency has been discussed first. Preserve loopback-only listeners, metadata-only request logs, safe rendering of untrusted data, and validation of control API mutations.

If API behavior changes, update [the contract](docs/api-contract.md), examples, and both READMEs as needed. Explain what your pull request changes and which checks you actually ran; mention checks you could not run.

## Releases

Maintainers publish version tags such as `v0.1.0`. The release workflow first runs the CI suite, builds six platform/architecture archives, and attaches them with SHA-256 checksums to a GitHub release. This requires GitHub Actions to be enabled and the release job to have permission to write repository contents.

## Security reports

Use the process in [SECURITY.md](SECURITY.md) for vulnerabilities. Do not post secrets or an exploit against another person's system in a public issue.

By contributing, you agree that your contributions are licensed under the repository's MIT license.
