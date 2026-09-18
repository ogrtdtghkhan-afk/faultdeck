#!/usr/bin/env python3
"""Build and verify the real Compose deployment; uses only Python's stdlib.

Creates an isolated Compose project, random credentials/ports, and an independent
upstream container. Only this test project's containers and volume are removed.
"""

import argparse
import base64
import http.client
import json
import os
from pathlib import Path
import secrets
import socket
import subprocess
import sys
import tempfile
import time
import urllib.error
import urllib.request


class NoRedirect(urllib.request.HTTPRedirectHandler):
    def redirect_request(self, req, fp, code, msg, headers, newurl):
        return None


def request(method, url, headers=None, body=None):
    payload = body.encode("utf-8") if isinstance(body, str) else body
    req = urllib.request.Request(url, data=payload, headers=headers or {}, method=method)
    opener = urllib.request.build_opener(urllib.request.ProxyHandler({}), NoRedirect())
    try:
        response = opener.open(req, timeout=10)
    except urllib.error.HTTPError as error:
        response = error
    with response:
        return response.status, response.read()


def require(condition, message):
    if not condition:
        raise RuntimeError(message)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--report", type=Path, help="Write a JSON result without credentials")
    args = parser.parse_args()
    root = Path(__file__).resolve().parent.parent
    project = "faultdeck-smoke-" + secrets.token_hex(5)
    password = secrets.token_urlsafe(30)
    token = secrets.token_urlsafe(30)
    report = {"success": False, "checks": []}
    started = time.monotonic()

    def passed(label):
        report["checks"].append(label)
        print("PASS " + label, flush=True)

    def command(argv, env=None, capture=True, timeout=600):
        result = subprocess.run(argv, cwd=root, env=env, text=True, capture_output=capture, timeout=timeout)
        if result.returncode:
            detail = ((result.stdout or "") + (result.stderr or ""))[-6000:]
            detail = detail.replace(password, "[redacted]").replace(token, "[redacted]")
            raise RuntimeError("Command failed: " + " ".join(argv[:4]) + "\n" + detail)
        return result.stdout or ""

    try:
        command(["docker", "info", "--format", "{{.ServerVersion}}"])
        command(["docker", "compose", "version"])
        reservations = []
        try:
            for _ in range(2):
                listener = socket.socket()
                listener.bind(("127.0.0.1", 0))
                reservations.append(listener)
            ui_port, proxy_port = [listener.getsockname()[1] for listener in reservations]
        finally:
            for listener in reservations:
                listener.close()
        admin_url = f"http://127.0.0.1:{ui_port}"
        proxy_url = f"http://127.0.0.1:{proxy_port}"
        env = dict(os.environ, FAULTDECK_ADMIN_USER="admin", FAULTDECK_ADMIN_PASSWORD=password,
                   FAULTDECK_PROXY_TOKEN=token, FAULTDECK_UI_PORT=str(ui_port),
                   FAULTDECK_PROXY_PORT=str(proxy_port), FAULTDECK_UI_ORIGIN=admin_url,
                   FAULTDECK_PROXY_URL=proxy_url)
        base = ["docker", "compose", "--project-directory", str(root), "-f", str(root / "compose.yaml")]
        config = json.loads(command(base + ["config", "--format", "json"], env=env))
        config.pop("name", None)
        # The rendered base config contains default project names; replace those
        # so a developer's real stack/volume cannot be reused by this test.
        config["volumes"] = {"faultdeck-data": {}}
        config["networks"] = {"default": {}}
        config["services"]["faultdeck"]["depends_on"] = {"upstream": {"condition": "service_healthy"}}
        config["services"]["upstream"] = {
            "image": "python:3.13.15-slim-trixie",
            "user": "65532:65532",
            "command": ["python", "-B", "/test/upstream.py"],
            "volumes": [{"type": "bind", "source": str(root / "scripts" / "deployment-upstream.py"),
                         "target": "/test/upstream.py", "read_only": True}],
            "healthcheck": {"test": ["CMD", "python", "-c", "import urllib.request; urllib.request.urlopen('http://127.0.0.1:8080/healthz', timeout=1).read()"],
                            "interval": "1s", "timeout": "2s", "retries": 30},
            "read_only": True,
            "cap_drop": ["ALL"],
            "security_opt": ["no-new-privileges:true"],
        }
        auth = "Basic " + base64.b64encode(("admin:" + password).encode()).decode()
        admin_headers = {"Authorization": auth, "X-FaultDeck": "1", "Content-Type": "application/json"}

        def admin(method, path, data=None):
            body = None if data is None else json.dumps(data).encode()
            code, raw = request(method, admin_url + path, admin_headers, body)
            require(200 <= code < 300, f"Admin {method} {path} returned {code}")
            return json.loads(raw) if raw else None

        def proxy(path="/api/orders", access_token=token, method="GET", body=None, headers=None):
            outgoing = dict(headers or {})
            if access_token is not None:
                outgoing["X-FaultDeck-Token"] = access_token
            return request(method, proxy_url + path, outgoing, body)

        with tempfile.TemporaryDirectory(prefix=project + "-") as directory:
            temporary = Path(directory).resolve()
            require(temporary.parent == Path(tempfile.gettempdir()).resolve(), "Unexpected temporary directory")
            compose_file = temporary / "compose.json"
            compose_file.write_text(json.dumps(config), encoding="utf-8")
            os.chmod(compose_file, 0o600)
            compose = ["docker", "compose", "--project-name", project, "--project-directory", str(root), "-f", str(compose_file)]
            try:
                print("Building and starting the isolated deployment...", flush=True)
                command(compose + ["up", "--build", "-d", "--wait", "--wait-timeout", "120"], env=env, capture=False)
                passed("Compose image builds and both real containers become healthy")
                container = command(compose + ["ps", "-q", "faultdeck"], env=env).strip()
                identity = command(["docker", "inspect", "--format", "{{.Config.User}}", container]).strip()
                require(identity in ("65532", "65532:65532", "nonroot"), "FaultDeck container is not nonroot")
                command(compose + ["exec", "-T", "faultdeck", "/faultdeck", "--healthcheck"], env=env)
                passed("Nonroot container and executable healthcheck")
                code, raw = request("GET", admin_url + "/healthz")
                require(code == 200 and len(raw) <= 128 and b"upstream" not in raw.lower(), "Unauthenticated health endpoint failed or disclosed state")
                for path in ("/", "/api/state", "/app.js"):
                    require(request("GET", admin_url + path)[0] == 401, "Admin resource accessible without authentication: " + path)
                require(proxy(access_token=None)[0] == 401, "Proxy accepted missing token")
                require(proxy(access_token="wrong-token")[0] == 401, "Proxy accepted wrong token")
                require(proxy(access_token=password)[0] == 401, "Separate proxy token did not isolate admin password")
                passed("Admin Basic Auth and proxy token reject unauthenticated access")

                upstream = "http://upstream:8080"
                admin("PUT", "/api/config", {"upstream": upstream, "enabled": True})
                state = admin("GET", "/api/state")
                require(state.get("persistent") is True, "Data volume persistence is not enabled")
                body = '{"order":"deployment-check","note":"测试 body"}'
                headers = {"Authorization": "Bearer test-business-credential", "Content-Type": "application/json"}
                code, raw = proxy("/echo?encoded=a%2Fb&count=2", method="POST", body=body, headers=headers)
                require(code == 200, "Real POST did not reach independent upstream")
                echo = json.loads(raw)
                require(echo["service"] == "independent-deployment-upstream", "Request used the built-in demo")
                require(echo["method"] == "POST" and echo["path"] == "/echo" and echo["body"] == body, "POST method/path/body changed")
                require(echo["query"] == "encoded=a%2Fb&count=2", "Query changed")
                require(echo["authorization"] == headers["Authorization"], "Business Authorization was overwritten")
                require(echo["proxyTokenPresent"] is False, "Proxy access token leaked to upstream")
                passed("Real upstream preserves POST body, query and business auth while stripping proxy token")

                rule = admin("POST", "/api/rules", {"name": "Deployment fail twice", "enabled": True, "method": "GET",
                             "path": "/api/orders", "type": "status", "statusCode": 503, "every": 1, "limit": 2})
                require([proxy()[0] for _ in range(3)] == [503, 503, 200], "Fail twice sequence was incorrect")
                passed("Real client requests return 503, 503, then upstream 200")

                for fault in ("latency", "timeout", "disconnect"):
                    path = "/fault/" + fault
                    temporary_rule = admin("POST", "/api/rules", {"name": "Deployment " + fault,
                        "enabled": True, "method": "GET", "path": path, "type": fault,
                        "delayMs": 80, "every": 1, "limit": 0})
                    if fault == "disconnect":
                        connection = http.client.HTTPConnection("127.0.0.1", proxy_port, timeout=3)
                        try:
                            connection.request("GET", path, headers={"X-FaultDeck-Token": token})
                            try:
                                connection.getresponse()
                            except (http.client.RemoteDisconnected, ConnectionResetError):
                                pass
                            else:
                                raise RuntimeError("Disconnect returned an HTTP response")
                        finally:
                            connection.close()
                    else:
                        before = time.monotonic()
                        code, raw = proxy(path)
                        require(time.monotonic() - before >= 0.080, fault + " returned before its configured delay")
                        require(code == (200 if fault == "latency" else 504), fault + " returned an unexpected status")
                        if fault == "latency":
                            require(json.loads(raw)["service"] == "independent-deployment-upstream", "Latency did not forward upstream")
                    admin("DELETE", "/api/rules/" + temporary_rule["id"])
                passed("Real latency, bounded timeout 504, and TCP disconnect without an HTTP response")

                admin("PUT", "/api/config", {"enabled": False})
                command(compose + ["up", "-d", "--no-deps", "--force-recreate", "--wait", "--wait-timeout", "120", "faultdeck"], env=env)
                recreated = command(compose + ["ps", "-q", "faultdeck"], env=env).strip()
                require(recreated != container, "Persistence test did not replace the container")
                state = admin("GET", "/api/state")
                require(state["upstream"] == upstream and state["enabled"] is False, "Upstream or master switch did not persist")
                require(len(state["rules"]) == 1, "Saved rule count changed")
                restored_rule = state["rules"][0]
                for field in ("name", "enabled", "method", "path", "type", "statusCode", "every", "limit"):
                    require(restored_rule[field] == rule[field], "Saved rule field changed: " + field)
                # IDs are runtime identifiers and may be reassigned on reload.
                rule = restored_rule
                require(state["rules"][0].get("hits", 0) == 0 and state["stats"]["requests"] == 0 and state["logs"] == [], "Runtime counters/logs persisted unexpectedly")
                require(proxy()[0] == 200, "Persisted disabled switch did not bypass rules")
                admin("PUT", "/api/config", {"enabled": True})
                require([proxy()[0] for _ in range(3)] == [503, 503, 200], "Restored rule could not replay")
                passed("Container recreation preserves target/rules/switch and resets runtime counters/logs")

                config["services"]["faultdeck"]["environment"]["FAULTDECK_PROXY_TOKEN"] = ""
                compose_file.write_text(json.dumps(config), encoding="utf-8")
                command(compose + ["up", "-d", "--no-deps", "--force-recreate", "--wait", "--wait-timeout", "120", "faultdeck"], env=env)
                require(proxy("/echo", access_token=password)[0] == 200, "Empty proxy token did not fall back to admin password")
                require(proxy("/echo", access_token=token)[0] == 401, "Old proxy token remained valid after reconfiguration")
                passed("Default proxy credential fallback works after container recreation")
                report["success"] = True
            finally:
                if not report["success"]:
                    try:
                        logs = command(compose + ["logs", "--no-color", "--tail", "80"], env=env)
                        print(logs.replace(password, "[redacted]").replace(token, "[redacted]"), file=sys.stderr)
                    except Exception:
                        pass
                command(compose + ["down", "--volumes", "--remove-orphans"], env=env)
    except Exception as error:
        report["error"] = str(error).replace(password, "[redacted]").replace(token, "[redacted]")
        print("FAIL " + report["error"], file=sys.stderr)
        report["success"] = False
    finally:
        report["durationSeconds"] = round(time.monotonic() - started, 2)
        if args.report:
            args.report.parent.mkdir(parents=True, exist_ok=True)
            args.report.write_text(json.dumps(report, indent=2) + "\n", encoding="utf-8")
    return 0 if report["success"] else 1


if __name__ == "__main__":
    sys.exit(main())
