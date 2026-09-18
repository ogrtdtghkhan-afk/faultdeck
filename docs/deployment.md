# Deploy FaultDeck with Docker

[中文部署指南](deployment.zh-CN.md)

This deploys the working proxy and control panel, with authentication and a persistent workspace. It is separate from the static website's browser simulation. Use the current source checkout for this deployment; older v0.1.0 release archives do not include the deployment files.

## Start the service

Install Docker with the Compose v2 plugin. Docker Desktop should use Linux containers. You do not need Go or Node.js on the host.

```sh
git clone https://github.com/ogrtdtghkhan-afk/faultdeck.git
cd faultdeck
cp .env.example .env
```

In Windows PowerShell, replace the last command with `Copy-Item .env.example .env`.

Open `.env` and set **`FAULTDECK_ADMIN_PASSWORD`** to your own random password of at least 12 characters. There is no working default password. The username defaults to `admin`. Optionally set a different `FAULTDECK_PROXY_TOKEN`, also at least 12 characters; otherwise the proxy uses the admin password as its token. Keep `.env` private; it is excluded from Git and the Docker build context.

```sh
docker compose up --build -d
docker compose ps
```

Wait for `faultdeck` to become healthy, then open **http://127.0.0.1:7331**. Enter `admin` and your password in the browser's HTTP Basic Auth prompt. The proxy is **http://127.0.0.1:7332**.

The image uses an [official Go builder](https://hub.docker.com/_/golang) and a [Distroless nonroot runtime](https://github.com/GoogleContainerTools/distroless) with CA certificates. The process runs as UID/GID 65532, the root filesystem is read-only, and `/data` is writable through a named volume. Only the two host loopback ports are published; the built-in demo remains internal.

## Point it at a real API

In the control panel, change **Upstream** to an API that the container can reach and save it. The initial built-in demo is useful for checking installation, but you can now forward real requests to your own service.

| Where your backend runs | Example upstream |
| --- | --- |
| Another container on the same Docker network | `http://your-api-service:8000` |
| On the Docker host | `http://host.docker.internal:8000` |
| A reachable development server | `https://dev-api.example.com` |

Inside the FaultDeck container, `127.0.0.1` refers to that container. It does not refer to your host or another container. Compose provides a `host.docker.internal` host-gateway mapping; on Linux, your host backend must listen on an address reachable from the Docker bridge. A backend bound only to host `127.0.0.1` may not be reachable. Containers must share a Docker network before service-name addresses work.

Each application request to port 7332 needs **`X-FaultDeck-Token`**. This credential is removed before forwarding. Your application's existing **`Authorization`** header remains available to the backend.

Send the proxy token only to FaultDeck. Disable automatic redirect following in clients that would retain this custom header: an upstream redirect can otherwise send it to a different server. Use `redirect: "manual"` with Node.js `fetch`; the included retry client already rejects redirects. A separate proxy token keeps client credentials distinct from the administrator password.

For a Bash terminal, enter the proxy token at the prompt below (use the admin password if you left the separate token empty):

```sh
read -r -s -p "Proxy token: " proxy_token
printf '\n'
curl -i -H "X-FaultDeck-Token: $proxy_token" http://127.0.0.1:7332/api/orders
unset proxy_token
```

PowerShell equivalent:

```powershell
$credential = [System.Net.NetworkCredential]::new('', (Read-Host 'Proxy token' -AsSecureString))
Invoke-WebRequest http://127.0.0.1:7332/api/orders -MaximumRedirection 0 -Headers @{'X-FaultDeck-Token' = $credential.Password}
Remove-Variable credential
```

Use a path your configured backend actually serves. Add a **Fail twice** rule for that path, make three requests, and observe two injected 503 responses followed by the real upstream response. The dashboard playground already supplies the proxy credential internally.

For browser applications, use your development server or BFF as the same-origin hop and add the proxy token there. Do not embed an admin password or proxy token in public browser code. FaultDeck does not provide a general cross-origin browser API or automatically add CORS headers to injected responses.

## What survives a restart

The named `faultdeck-data` volume stores the upstream, master switch, and rules. Configuration changes through the UI/API are saved. Logs, totals, and rule execution counters remain in memory, so a rule that fails twice starts its count again after restarting.

```sh
docker compose restart faultdeck
```

Stopping with `docker compose down` retains the named volume. Start again with the same Compose project name/directory to reuse it. Removing the volume erases the workspace; do not add `--volumes` to a normal shutdown. Prefer the supplied named volume over a host bind mount. If you choose a bind mount, its directory must be writable by UID/GID 65532.

Credentials come from `.env`, not the saved workspace. After changing a password, token, or public URL, apply the environment with `docker compose up -d --force-recreate`. Restarting alone does not replace an existing container's environment.

## Access a deployment on another machine

The default host ports remain private. For personal use on a remote Docker host, forward both ports with SSH:

```sh
ssh -L 7331:127.0.0.1:7331 -L 7332:127.0.0.1:7332 user@your-server
```

Then use the same local URLs and credentials. Keep the tunnel open while using FaultDeck.

For a shared HTTPS deployment, place your TLS reverse proxy on the Docker host and forward separate hostnames to `127.0.0.1:7331` and `127.0.0.1:7332`. Set these values in `.env` to your real public origins and recreate the container:

```dotenv
FAULTDECK_UI_ORIGIN=https://faultdeck.example.com
FAULTDECK_PROXY_URL=https://faults.example.com
```

These settings describe external addresses; they do not provision DNS or TLS. Preserve the request Host and authentication headers through your reverse proxy. Route each hostname to its corresponding port, without a URL path prefix. `--ui-origin` must match the URL used by the browser. Keep Basic Auth and the proxy token enabled, and use HTTPS for access outside an SSH tunnel or the local host.

## Operations and diagnosis

```sh
docker compose logs --tail=100 faultdeck
docker compose exec -T faultdeck /faultdeck --healthcheck
```

`/healthz` is a minimal unauthenticated health endpoint on the control port. The executable's healthcheck probes its internal listener and exits 0 on success or 1 on failure; it does not require browser credentials or the public hostname. When using custom internal ports outside this Compose file, pass the matching `--ui-port` to the healthcheck too.

| Symptom | Check |
| --- | --- |
| Compose asks for a password | Fill the empty password in `.env`; use at least 12 characters |
| Browser login fails | Username/password in `.env`, then recreate the container after changes |
| Proxy returns 401 | Supply `X-FaultDeck-Token`; it is separate from business `Authorization` |
| Proxy returns 502 | Verify the upstream is reachable from the container and its TLS certificate is trusted |
| UI reports an origin/host error | Match `FAULTDECK_UI_ORIGIN` to the browser's exact scheme, hostname, and port |
| Workspace cannot be saved | Check volume permissions and disk space; the process uses UID 65532 |
| Host port is occupied | Change `FAULTDECK_UI_PORT`/`FAULTDECK_PROXY_PORT` and the corresponding advertised URLs in `.env` |

For an update, review the new source, rebuild with `docker compose up --build -d`, and retain the volume. FaultDeck is a tool for development/test environments; it deliberately introduces failures into the traffic you send through it.

## Deployment acceptance test

With Docker and Python 3 installed, run from the repository root:

```sh
python3 scripts/deployment-smoke.py
```

On Windows, use `python` instead of `python3`. The test builds the real image, starts an isolated Compose project with an independent upstream container, and verifies authentication, real POST/body/query/auth forwarding, token removal, 503/503/200 behavior, latency, timeout, TCP disconnect, nonroot healthchecks, and persistence across container recreation. It uses random credentials and host ports, then removes only its own containers and volume.

The [Deployment workflow](https://github.com/ogrtdtghkhan-afk/faultdeck/actions/workflows/deployment.yml) runs this same test and retains a JSON evidence artifact. Use that run's result to assess the tested commit; a native Go test or static website preview alone does not establish that a container deployment works.
