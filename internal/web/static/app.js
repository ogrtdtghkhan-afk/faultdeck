"use strict";

(() => {
  const $ = (selector, root = document) => root.querySelector(selector);
  const $$ = (selector, root = document) =>
    Array.from(root.querySelectorAll(selector));
  const elements = {};
  $$("[id]").forEach((element) => {
    elements[element.id] = element;
  });
  let state = null;
  let loading = false;
  let lastRuleSignature = "";
  let lastLogSignature = "";
  let logFilter = "all";
  let editingRuleId = null;
  let targetDirty = false;
  let confirmResolve = null;
  let playing = false;
  let exampleLanguage = "curl";
  let lastExample = "";

  const icons = {
    latency: '<circle cx="10" cy="11" r="6"/><path d="M10 8v3l2 1M8 2h4"/>',
    status: '<path d="M10 3 18 17H2L10 3zM10 8v4m0 2h.01"/>',
    timeout:
      '<circle cx="10" cy="10" r="7"/><path d="M10 6v4l3 2M4 3 2 5m14-2 2 2"/>',
    disconnect:
      '<path d="m3 3 14 14M6 6l-3 3 3 3 3-3m2 2 3 3 3-3-3-3M4 11l-2 2m14-6 2-2"/>',
    edit: '<path d="m13 3 4 4-10 10H3v-4L13 3zM11 5l4 4"/>',
    delete: '<path d="M4 6h12M8 3h4l1 3H7l1-3zM5 6l1 11h8l1-11M8 9v5m4-5v5"/>',
    bolt: '<path d="m12 2-8 10h6l-2 6 8-10h-6l2-6z"/>',
  };

  function node(tag, className, text) {
    const element = document.createElement(tag);
    if (className) element.className = className;
    if (text !== undefined && text !== null) element.textContent = String(text);
    return element;
  }

  function icon(name) {
    // Only hard-coded, application-owned SVG markup enters this helper.
    const svg = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    svg.setAttribute("viewBox", "0 0 20 20");
    svg.setAttribute("aria-hidden", "true");
    svg.innerHTML = icons[name] || icons.bolt;
    return svg;
  }

  function toast(message, error = false) {
    const item = node("div", `toast${error ? " error" : ""}`, message);
    elements["toast-stack"].append(item);
    window.setTimeout(
      () => {
        item.classList.add("removing");
        window.setTimeout(() => item.remove(), 220);
      },
      error ? 6500 : 3500,
    );
  }

  async function api(path, method = "GET", body) {
    const options = { method, headers: {} };
    if (method !== "GET") options.headers["X-FaultDeck"] = "1";
    if (body !== undefined) {
      options.headers["Content-Type"] = "application/json";
      options.body = JSON.stringify(body);
    }
    const response = await fetch(path, options);
    if (response.status === 204) return null;
    let data;
    try {
      data = await response.json();
    } catch {
      throw new Error(`Unexpected response (${response.status}).`);
    }
    if (!response.ok)
      throw new Error(data.error || `Request failed (${response.status}).`);
    return data;
  }

  function setConnection(connected) {
    elements["connection-dot"].classList.toggle("offline", !connected);
    elements["connection-label"].textContent = connected
      ? "Instance connected"
      : "Reconnecting…";
    elements["master-toggle"].disabled = !connected;
  }

  function formatNumber(value) {
    return Number(value || 0).toLocaleString();
  }
  function formatDuration(value) {
    const number = Number(value || 0);
    if (number >= 1000) return `${(number / 1000).toFixed(2)}s`;
    return `${number < 10 ? number.toFixed(1) : Math.round(number)}ms`;
  }

  async function refresh() {
    if (loading) return;
    loading = true;
    try {
      updateState(await api("/api/state"));
      setConnection(true);
    } catch {
      setConnection(false);
    } finally {
      loading = false;
    }
  }

  function shellQuote(value) {
    return "'" + String(value).replace(/'/g, "'\\''") + "'";
  }

  function renderIntegrationExample() {
    if (!state) return;
    const inputPath = elements["integration-path"].value.trim() || "/your-endpoint";
    const requestPath = inputPath.startsWith("/") ? inputPath : "/" + inputPath;
    const url = String(state.proxyUrl).replace(/\/+$/, "") + requestPath;
    let example;
    if (exampleLanguage === "node") {
      const authSetup = state.proxyAuth
        ? 'const token = process.env.FAULTDECK_PROXY_TOKEN;\nif (!token) throw new Error("Set FAULTDECK_PROXY_TOKEN from deployment config");\n\n'
        : "";
      const options = state.proxyAuth
        ? ', {\n  headers: { "X-FaultDeck-Token": token },\n}'
        : "";
      example = `${authSetup}const response = await fetch(${JSON.stringify(url)}${options});\nconsole.log(response.status, await response.text());`;
    } else {
      const auth = state.proxyAuth
        ? " --header " + shellQuote("X-FaultDeck-Token: <token-from-your-deployment>")
        : "";
      example = "curl --include" + auth + " --url " + shellQuote(url);
    }
    // Generated snippets are text only. Quote URLs for their target language;
    // never request, read or embed deployment secrets in browser state.
    if (lastExample !== example) {
      elements["integration-code"].textContent = example;
      lastExample = example;
    }
    elements["integration-auth-help"].textContent = state.proxyAuth
      ? "Replace the curl token placeholder, or set FAULTDECK_PROXY_TOKEN in your Node.js process. Use your configured proxy token; if none was set, use the admin password. Keep this value in server-side configuration. curl examples use POSIX shell quoting."
      : "Replace /your-endpoint with an existing backend route. Node.js examples run server-side with built-in fetch. curl examples use POSIX shell quoting. No proxy token is required by this instance.";
  }

  function updateState(next) {
    state = next;
    const rules = state.rules || [];
    const logs = state.logs || [];
    elements["version"].textContent = `v${state.version || "0.2.0"}`;
    elements["master-toggle"].setAttribute(
      "aria-checked",
      String(Boolean(state.enabled)),
    );
    elements["master-toggle"].setAttribute(
      "aria-label",
      state.enabled ? "Pause fault injection" : "Enable fault injection",
    );
    elements["master-status"].textContent = state.enabled
      ? "Injection active"
      : "Injection paused";
    $(".master-control").classList.toggle("is-paused", !state.enabled);
    if (!targetDirty && document.activeElement !== elements.upstream)
      elements.upstream.value = state.upstream;
    elements["upstream-kind"].textContent =
      state.upstream === state.demoUrl ? "BUILT-IN DEMO" : "YOUR BACKEND";
    elements["proxy-url"].textContent = state.proxyUrl;
    elements["play-target"].textContent = state.upstream;
    elements["play-target"].title = state.upstream;
    elements["use-demo"].hidden = state.upstream === state.demoUrl;
    const usingDemo = Boolean(state.demoUrl) && state.upstream === state.demoUrl;
    elements["integration-target-note"].textContent = usingDemo
      ? "The built-in demo is currently selected. Replace the backend URL above to test your own service."
      : "Your backend is selected. Client requests to the proxy are forwarded to the target shown above.";
    elements["integration-target-note"].classList.toggle("demo-target-note", usingDemo);
    elements["persistence-status"].textContent = state.persistent ? "Persistence enabled" : "Persistence not enabled";
    elements["persistence-status"].classList.toggle("is-persistent", Boolean(state.persistent));
    elements["persistence-note"].textContent = state.persistent
      ? "Backend URL, rules and injection switch are saved across restarts. Request activity and counters are session-only. JSON export remains available."
      : "Settings are in memory and will be lost on restart. Export your scenario, or start FaultDeck with persistent storage enabled.";
    elements["proxy-auth-note"].textContent = state.proxyAuth
      ? "Add the X-FaultDeck-Token header to requests from your server-side client. Use the token from your deployment configuration."
      : "This instance does not require a proxy token. Send requests to the client base URL above.";
    renderIntegrationExample();
    const stats = state.stats || {};
    elements["stat-requests"].textContent = formatNumber(stats.requests);
    elements["stat-injected"].textContent = formatNumber(stats.injected);
    elements["stat-errors"].textContent = formatNumber(stats.errors);
    elements["injected-percent"].textContent =
      `${stats.requests ? Math.round((stats.injected / stats.requests) * 100) : 0}% of requests`;
    const duration = Number(stats.avgDurationMs || 0);
    elements["stat-duration"].replaceChildren(
      document.createTextNode(
        duration >= 1000
          ? (duration / 1000).toFixed(1)
          : String(Math.round(duration)),
      ),
      node("span", "metric-unit", duration >= 1000 ? "s" : "ms"),
    );
    elements["rule-count"].textContent = rules.length;
    const signature = JSON.stringify(rules);
    if (signature !== lastRuleSignature) {
      renderRules(rules);
      lastRuleSignature = signature;
    }
    const logSignature = JSON.stringify(logs);
    if (logSignature !== lastLogSignature) {
      renderLogs(logs);
      lastLogSignature = logSignature;
    }
  }

  function ruleSummary(rule) {
    switch (rule.type) {
      case "latency":
        return `Add ${formatDuration(rule.delayMs)} latency`;
      case "status":
        return `Return HTTP ${rule.statusCode}`;
      case "timeout":
        return `Timeout after ${formatDuration(rule.delayMs)}`;
      case "disconnect":
        return "Close the connection";
      default:
        return rule.type;
    }
  }

  function renderRules(rules) {
    // Preserve focus across polling updates when rule counters change.
    const focused = document.activeElement;
    const focusKey =
      focused && focused.dataset ? focused.dataset.focusKey : null;
    const list = elements["rules-list"];
    const scroll = list.scrollTop;
    const fragment = document.createDocumentFragment();
    if (!rules.length) {
      const empty = node("div", "empty-rules");
      const graphic = node("div", "empty-graphic");
      graphic.append(icon("bolt"));
      empty.append(
        graphic,
        node("h3", "", "A good place to break things."),
        node(
          "p",
          "",
          "Create a rule for your backend’s endpoint, then send a real request to test the unexpected.",
        ),
      );
      const create = node("button", "text-button", "Create your first rule →");
      create.addEventListener("click", () => openRule());
      empty.append(create);
      fragment.append(empty);
    } else {
      rules.forEach((rule, index) => {
        const card = node(
          "article",
          `rule-card${!rule.enabled ? " disabled" : ""}`,
        );
        const top = node("div", "rule-top");
        const faultIcon = node("span", "fault-icon");
        faultIcon.append(icon(rule.type));
        const titleGroup = node("div", "rule-title-group");
        const title = node("h3", "rule-title", rule.name);
        title.title = rule.name;
        titleGroup.append(title, node("div", "rule-kind", ruleSummary(rule)));
        const controls = node("div", "rule-controls");
        const edit = node("button", "icon-button");
        edit.append(icon("edit"));
        edit.setAttribute("aria-label", `Edit ${rule.name}`);
        edit.title = "Edit rule";
        edit.dataset.focusKey = `edit-${rule.id}`;
        edit.addEventListener("click", () => openRule(rule));
        const remove = node("button", "icon-button");
        remove.append(icon("delete"));
        remove.setAttribute("aria-label", `Delete ${rule.name}`);
        remove.title = "Delete rule";
        remove.dataset.focusKey = `delete-${rule.id}`;
        remove.addEventListener("click", () => deleteRule(rule, remove));
        const toggle = node("button", "toggle");
        toggle.setAttribute("role", "switch");
        toggle.setAttribute("aria-checked", String(rule.enabled));
        toggle.setAttribute(
          "aria-label",
          `${rule.enabled ? "Disable" : "Enable"} ${rule.name}`,
        );
        toggle.dataset.focusKey = `toggle-${rule.id}`;
        toggle.append(node("span"));
        toggle.addEventListener("click", () => toggleRule(rule, toggle));
        controls.append(edit, remove, toggle);
        top.append(faultIcon, titleGroup, controls);
        const pathLine = node("div", "rule-path-line");
        const path = node("code", "rule-path", rule.path);
        path.title = rule.path;
        pathLine.append(
          node("span", "method-tag", rule.method === "*" ? "ANY" : rule.method),
          path,
        );
        const footer = node("div", "rule-footer");
        let schedule =
          rule.every > 1
            ? `Every ${rule.every} matching requests`
            : "Every matching request";
        schedule = `${index + 1}. ${schedule}`;
        const exhausted = rule.limit > 0 && rule.hits >= rule.limit;
        const hits = node("span", `rule-hits${exhausted ? " exhausted" : ""}`);
        hits.append(
          node("span", "status-dot"),
          document.createTextNode(
            exhausted
              ? `Completed · ${rule.hits}/${rule.limit} hits`
              : `${formatNumber(rule.hits)}${rule.limit ? `/${rule.limit}` : ""} injected`,
          ),
        );
        hits.title = `${formatNumber(rule.matched)} matching requests considered${exhausted ? ". Limit reached; later rules or the upstream handle new requests." : ""}`;
        footer.append(node("span", "", schedule), hits);
        card.append(top, pathLine, footer);
        fragment.append(card);
      });
    }
    list.replaceChildren(fragment);
    list.scrollTop = scroll;
    if (focusKey) {
      const match = $$("[data-focus-key]", list).find(
        (element) => element.dataset.focusKey === focusKey,
      );
      if (match) match.focus({ preventScroll: true });
    }
  }

  function renderLogs(logs) {
    const filtered =
      logFilter === "faults"
        ? logs.filter((log) => log.ruleId || log.fault)
        : logs;
    const fragment = document.createDocumentFragment();
    filtered.forEach((log) => {
      const row = node("tr");
      const date = new Date(log.time);
      const time = Number.isNaN(date.getTime())
        ? "—"
        : date.toLocaleTimeString([], { hour12: false });
      const timeCell = node("td", "", time);
      timeCell.title = log.time;
      const request = node("td");
      const requestContent = node("div", "activity-request");
      const path = node("span", "", log.path);
      path.title = `${log.method} ${log.path}`;
      requestContent.append(node("span", "method-tag", log.method), path);
      request.append(requestContent);
      const status = node("td");
      status.append(
        node(
          "span",
          `status-pill${log.status === 0 ? " closed" : log.status >= 400 ? " error" : ""}`,
          log.status === 0 ? "CLOSED" : log.status,
        ),
      );
      if (log.error) status.title = log.error;
      const duration = node("td", "", formatDuration(log.durationMs));
      const fault = node("td");
      const faultText = node(
        "span",
        `log-fault${!log.fault && !log.ruleId ? " passthrough" : ""}`,
        log.ruleName || log.fault || "Passed through",
      );
      faultText.title = log.error || log.ruleName || "Forwarded to upstream";
      fault.append(faultText);
      row.append(timeCell, request, status, duration, fault);
      fragment.append(row);
    });
    elements["activity-rows"].replaceChildren(fragment);
    elements["activity-empty"].hidden = filtered.length > 0;
    $("strong", elements["activity-empty"]).textContent =
      logFilter === "faults"
        ? "No injected requests yet"
        : "Listening for requests";
    $("span", elements["activity-empty"]).textContent =
      logFilter === "faults"
        ? "Enable a rule and send a matching request to see it here."
        : "Send a request from the playground or point your app at the proxy.";
    elements["activity-summary"].textContent = filtered.length
      ? `Showing ${filtered.length} ${logFilter === "faults" ? "injected" : "recent"} request${filtered.length === 1 ? "" : "s"} · latest first · up to 200 retained`
      : "No requests yet";
  }

  function cleanRule(rule) {
    const {
      name,
      enabled,
      method,
      path,
      type,
      delayMs,
      statusCode,
      every,
      limit,
    } = rule;
    return {
      name,
      enabled,
      method,
      path,
      type,
      delayMs,
      statusCode,
      every,
      limit,
    };
  }

  async function toggleRule(rule, button) {
    button.disabled = true;
    try {
      await api(`/api/rules/${encodeURIComponent(rule.id)}`, "PUT", {
        ...cleanRule(rule),
        enabled: !rule.enabled,
      });
      await refresh();
    } catch (error) {
      toast(error.message, true);
    } finally {
      button.disabled = false;
    }
  }

  async function deleteRule(rule, button) {
    button.disabled = true;
    try {
      await api(`/api/rules/${encodeURIComponent(rule.id)}`, "DELETE");
      toast(`Deleted “${rule.name}”.`);
      await refresh();
    } catch (error) {
      toast(error.message, true);
      button.disabled = false;
    }
  }

  function updateTypeFields() {
    const type = elements["rule-type"].value;
    const needsDelay = type === "latency" || type === "timeout";
    elements["delay-field"].hidden = !needsDelay;
    elements["rule-delay"].disabled = !needsDelay;
    elements["rule-delay"].required = needsDelay;
    elements["rule-delay"].min = type === "timeout" ? "1" : "0";
    elements["status-field"].hidden = type !== "status";
    elements["rule-status"].disabled = type !== "status";
    elements["rule-status"].required = type === "status";
  }

  function openRule(rule) {
    editingRuleId = rule ? rule.id : null;
    elements["rule-form"].reset();
    elements["rule-name"].value = rule ? rule.name : "";
    elements["rule-method"].value = rule ? rule.method : "*";
    elements["rule-path"].value = rule ? rule.path : "/api/*";
    elements["rule-type"].value = rule ? rule.type : "latency";
    elements["rule-delay"].value = rule ? rule.delayMs : 1500;
    elements["rule-status"].value =
      rule && rule.statusCode >= 400 ? rule.statusCode : 503;
    elements["rule-every"].value = rule ? rule.every : 1;
    elements["rule-limit"].value = rule ? rule.limit : 0;
    elements["rule-enabled"].checked = rule ? rule.enabled : true;
    elements["rule-dialog-title"].textContent = rule
      ? "Edit fault rule"
      : "Create fault rule";
    elements["save-rule"].textContent = rule ? "Save changes" : "Create rule";
    elements["rule-error"].hidden = true;
    updateTypeFields();
    elements["rule-dialog"].showModal();
    elements["rule-name"].focus();
  }

  function confirmAction(title, message, accept = "Continue") {
    if (confirmResolve) confirmResolve(false);
    elements["confirm-title"].textContent = title;
    elements["confirm-message"].textContent = message;
    elements["confirm-accept"].textContent = accept;
    elements["confirm-dialog"].showModal();
    elements["confirm-cancel"].focus();
    return new Promise((resolve) => {
      confirmResolve = resolve;
    });
  }

  function finishConfirmation(accepted) {
    elements["confirm-dialog"].close();
    const resolve = confirmResolve;
    confirmResolve = null;
    if (resolve) resolve(accepted);
  }

  elements["confirm-cancel"].addEventListener("click", () =>
    finishConfirmation(false),
  );
  elements["confirm-accept"].addEventListener("click", () =>
    finishConfirmation(true),
  );
  elements["confirm-dialog"].addEventListener("cancel", (event) => {
    event.preventDefault();
    finishConfirmation(false);
  });
  elements["add-rule"].addEventListener("click", () => openRule());
  elements["integration-add-rule"].addEventListener("click", () => openRule());
  elements["integration-path"].addEventListener("input", renderIntegrationExample);
  $$("[data-example]").forEach((button) => button.addEventListener("click", () => {
    exampleLanguage = button.dataset.example;
    $$("[data-example]").forEach((item) => {
      const selected = item === button;
      item.classList.toggle("selected", selected);
      item.setAttribute("aria-pressed", String(selected));
    });
    renderIntegrationExample();
  }));
  elements["copy-example"].addEventListener("click", async () => {
    if (!state) return;
    try {
      await navigator.clipboard.writeText(elements["integration-code"].textContent);
      toast("Connection example copied. Replace the endpoint and any token placeholder before running.");
    } catch {
      const range = document.createRange();
      range.selectNodeContents(elements["integration-code"]);
      const selection = window.getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      toast("Example selected. Press Ctrl+C or ⌘C to copy.");
    }
  });
  $$("[data-close-rule]").forEach((button) =>
    button.addEventListener("click", () => elements["rule-dialog"].close()),
  );
  elements["rule-type"].addEventListener("change", updateTypeFields);

  elements["rule-form"].addEventListener("submit", async (event) => {
    event.preventDefault();
    const type = elements["rule-type"].value;
    const rule = {
      name: elements["rule-name"].value.trim(),
      enabled: elements["rule-enabled"].checked,
      method: elements["rule-method"].value,
      path: elements["rule-path"].value.trim(),
      type,
      delayMs:
        type === "latency" || type === "timeout"
          ? Number(elements["rule-delay"].value)
          : 0,
      statusCode:
        type === "status" ? Number(elements["rule-status"].value) : 503,
      every: Number(elements["rule-every"].value),
      limit: Number(elements["rule-limit"].value),
    };
    elements["save-rule"].disabled = true;
    elements["rule-error"].hidden = true;
    try {
      await api(
        editingRuleId
          ? `/api/rules/${encodeURIComponent(editingRuleId)}`
          : "/api/rules",
        editingRuleId ? "PUT" : "POST",
        rule,
      );
      elements["rule-dialog"].close();
      toast(
        editingRuleId
          ? "Rule updated. Its counters have been reset."
          : "Fault rule created. Ready to inject.",
      );
      await refresh();
    } catch (error) {
      elements["rule-error"].textContent = error.message;
      elements["rule-error"].hidden = false;
    } finally {
      elements["save-rule"].disabled = false;
    }
  });

  elements.upstream.addEventListener("input", () => {
    targetDirty = true;
  });
  elements["upstream-form"].addEventListener("submit", async (event) => {
    event.preventDefault();
    elements["save-target"].disabled = true;
    try {
      const next = await api("/api/config", "PUT", {
        upstream: elements.upstream.value.trim(),
      });
      targetDirty = false;
      updateState(next);
      toast("Upstream target updated.");
    } catch (error) {
      toast(error.message, true);
    } finally {
      elements["save-target"].disabled = false;
    }
  });

  elements["master-toggle"].addEventListener("click", async () => {
    if (!state) return;
    elements["master-toggle"].disabled = true;
    try {
      updateState(await api("/api/config", "PUT", { enabled: !state.enabled }));
    } catch (error) {
      toast(error.message, true);
    } finally {
      elements["master-toggle"].disabled = false;
    }
  });

  elements["copy-proxy"].addEventListener("click", async () => {
    if (!state) return;
    try {
      await navigator.clipboard.writeText(state.proxyUrl);
      toast("Proxy address copied.");
    } catch {
      const range = document.createRange();
      range.selectNodeContents(elements["proxy-url"]);
      const selection = window.getSelection();
      selection.removeAllRanges();
      selection.addRange(range);
      toast("Address selected. Press Ctrl+C or ⌘C to copy.");
    }
  });

  async function switchToDemo() {
    if (!state) return false;
    if (state.upstream !== state.demoUrl) {
      const accepted = await confirmAction(
        "Switch to the demo upstream?",
        "New proxy requests will go to the built-in demo service. Your existing fault rules will stay in place.",
        "Use demo",
      );
      if (!accepted) return false;
      const next = await api("/api/config", "PUT", { upstream: state.demoUrl });
      targetDirty = false;
      updateState(next);
      elements.upstream.value = next.upstream;
    }
    return true;
  }

  elements["use-demo"].addEventListener("click", async () => {
    try {
      if (await switchToDemo()) toast("Built-in demo connected.");
    } catch (error) {
      toast(error.message, true);
    }
  });

  const presets = {
    slow: { name: "Slow response", type: "latency", delayMs: 1500 },
    rate: { name: "Rate limited", type: "status", statusCode: 429 },
    twice: {
      name: "Fail twice, then recover",
      type: "status",
      statusCode: 503,
      limit: 2,
    },
    disconnect: { name: "Connection interrupted", type: "disconnect" },
  };
  $$("[data-preset]").forEach((button) =>
    button.addEventListener("click", async () => {
      button.disabled = true;
      try {
        if (!(await switchToDemo())) return;
        await api("/api/rules", "POST", {
          name: "",
          enabled: true,
          method: "GET",
          path: "/api/orders",
          type: "latency",
          delayMs: 0,
          statusCode: 503,
          every: 1,
          limit: 0,
          ...presets[button.dataset.preset],
        });
        elements["play-path"].value = "/api/orders";
        elements["play-method"].value = "GET";
        toast("Demo rule added. Earlier matching rules run first.");
        await refresh();
      } catch (error) {
        toast(error.message, true);
      } finally {
        button.disabled = false;
      }
    }),
  );

  elements["play-form"].addEventListener("submit", async (event) => {
    event.preventDefault();
    if (playing) return;
    playing = true;
    elements["send-request"].disabled = true;
    $("span", elements["send-request"]).textContent = "Sending…";
    elements["response-meta"].replaceChildren(
      node("span", "muted", "Request in progress…"),
    );
    elements["response-body"].textContent =
      "// Waiting for the proxy response…";
    try {
      const response = await api("/api/play", "POST", {
        method: elements["play-method"].value,
        path: elements["play-path"].value,
      });
      const status = node(
        "span",
        `response-status${response.status === 0 || response.status >= 400 ? " error" : ""}`,
        response.status === 0
          ? "CONNECTION CLOSED"
          : `${response.status} ${statusName(response.status)}`.trim(),
      );
      elements["response-meta"].replaceChildren(
        status,
        node("span", "muted", formatDuration(response.durationMs)),
      );
      let body = response.body || "";
      try {
        body = JSON.stringify(JSON.parse(body), null, 2);
      } catch {
        /* Plain text remains plain text. */
      }
      elements["response-body"].textContent = response.error
        ? `${response.error}${body ? `\n\n${body}` : ""}`
        : body || "// Empty response body";
    } catch (error) {
      elements["response-meta"].replaceChildren(
        node("span", "response-status error", "REQUEST FAILED"),
      );
      elements["response-body"].textContent = error.message;
    } finally {
      playing = false;
      elements["send-request"].disabled = false;
      $("span", elements["send-request"]).textContent = "Send request";
      await refresh();
    }
  });

  function statusName(code) {
    return (
      {
        200: "OK",
        201: "Created",
        204: "No Content",
        301: "Moved Permanently",
        302: "Found",
        400: "Bad Request",
        401: "Unauthorized",
        403: "Forbidden",
        404: "Not Found",
        408: "Request Timeout",
        429: "Too Many Requests",
        500: "Internal Server Error",
        502: "Bad Gateway",
        503: "Service Unavailable",
        504: "Gateway Timeout",
      }[code] || ""
    );
  }

  $$("[data-log-filter]").forEach((button) =>
    button.addEventListener("click", () => {
      logFilter = button.dataset.logFilter;
      $$("[data-log-filter]").forEach((item) => {
        const selected = item === button;
        item.classList.toggle("selected", selected);
        item.setAttribute("aria-pressed", String(selected));
      });
      renderLogs(state ? state.logs || [] : []);
    }),
  );

  elements["clear-logs"].addEventListener("click", async () => {
    try {
      await api("/api/logs", "DELETE");
      toast("Request activity cleared. Session totals are unchanged.");
      await refresh();
    } catch (error) {
      toast(error.message, true);
    }
  });

  elements["reset-session"].addEventListener("click", async () => {
    if (
      !(await confirmAction(
        "Reset this session?",
        "Clear request activity, session totals and rule counters. Your target and rules will be retained. Rules that reached their limit will inject again.",
        "Reset session",
      ))
    )
      return;
    try {
      updateState(await api("/api/reset", "POST"));
      toast("Fresh session. Rules and target retained.");
    } catch (error) {
      toast(error.message, true);
    }
  });

  elements["export-button"].addEventListener("click", async () => {
    try {
      const scenario = await api("/api/scenario");
      const blob = new Blob([`${JSON.stringify(scenario, null, 2)}\n`], {
        type: "application/json",
      });
      const url = URL.createObjectURL(blob);
      const link = node("a");
      link.href = url;
      link.download = "faultdeck-scenario.json";
      document.body.append(link);
      link.click();
      link.remove();
      window.setTimeout(() => URL.revokeObjectURL(url), 1000);
      toast("Scenario exported. Keep it with your project.");
    } catch (error) {
      toast(error.message, true);
    }
  });

  elements["import-button"].addEventListener("click", () =>
    elements["import-file"].click(),
  );
  elements["import-file"].addEventListener("change", async () => {
    const file = elements["import-file"].files[0];
    elements["import-file"].value = "";
    if (!file) return;
    if (file.size > 1024 * 1024) {
      toast("Scenario files must be 1 MiB or smaller.", true);
      return;
    }
    try {
      const scenario = JSON.parse(await file.text());
      if (
        !scenario ||
        typeof scenario !== "object" ||
        !Array.isArray(scenario.rules)
      )
        throw new Error(
          "Choose a valid FaultDeck scenario with a rules array.",
        );
      if (
        !(await confirmAction(
          "Replace the current scenario?",
          `Import “${file.name}” with ${scenario.rules.length} rule${scenario.rules.length === 1 ? "" : "s"}? This replaces your target, injection switch and rules. The file stays on this computer.`,
          "Import scenario",
        ))
      )
        return;
      const next = await api("/api/scenario", "POST", scenario);
      targetDirty = false;
      updateState(next);
      elements.upstream.value = next.upstream;
      toast(
        `Imported ${next.rules.length} fault rule${next.rules.length === 1 ? "" : "s"}.`,
      );
    } catch (error) {
      toast(
        error instanceof SyntaxError
          ? "This file is not valid JSON."
          : error.message,
        true,
      );
    }
  });

  renderRules([]);
  renderLogs([]);
  refresh();
  window.setInterval(() => {
    if (!document.hidden) refresh();
  }, 1200);
  document.addEventListener("visibilitychange", () => {
    if (!document.hidden) refresh();
  });
})();
