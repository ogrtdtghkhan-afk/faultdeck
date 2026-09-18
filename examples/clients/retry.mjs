#!/usr/bin/env node

// Node.js 22+, no packages required. This client never changes FaultDeck rules.
import { setTimeout as sleep } from "node:timers/promises";
import { pathToFileURL } from "node:url";

export const DEFAULT_TARGET = "http://127.0.0.1:7332";
export const MAX_ATTEMPTS = 3;
export const MAX_WAIT_MS = 2000;
const DEFAULT_WAIT_MS = 250;

export function parseTarget(value) {
  let target;
  try {
    target = new URL(value);
  } catch {
    throw new Error(
      "--target must be an HTTP(S) origin, for example http://127.0.0.1:7332",
    );
  }
  if (
    !["http:", "https:"].includes(target.protocol) ||
    !target.hostname ||
    target.username ||
    target.password ||
    target.pathname !== "/" ||
    target.href.includes("?") ||
    target.href.includes("#") ||
    target.port === "0"
  ) {
    throw new Error(
      "--target must be an HTTP(S) origin without credentials, path, query, or fragment",
    );
  }
  return target;
}

export function retryDelayMs(retryAfter, now = Date.now()) {
  if (retryAfter === null || retryAfter.trim() === "") return DEFAULT_WAIT_MS;
  const value = retryAfter.trim();
  let delay;
  if (/^\d+$/.test(value)) {
    delay = Number(value) * 1000;
  } else {
    // Retry-After also accepts HTTP dates. Avoid treating invalid numeric values
    // such as "-1" or "1.5" as dates accepted by JavaScript's loose parser.
    const date = /[A-Za-z]/.test(value) ? Date.parse(value) : NaN;
    if (!Number.isFinite(date)) return DEFAULT_WAIT_MS;
    delay = Math.max(0, date - now);
  }
  if (delay > MAX_WAIT_MS) {
    throw new Error(
      `Retry-After exceeds this demo's ${MAX_WAIT_MS} ms wait cap; stopping without retrying early`,
    );
  }
  return delay;
}

export async function runRetryDemo({
  target = DEFAULT_TARGET,
  log = console.log,
  wait = sleep,
  timeoutMs = 3000,
} = {}) {
  if (!Number.isInteger(timeoutMs) || timeoutMs < 1 || timeoutMs > 60000) {
    throw new Error("timeoutMs must be an integer between 1 and 60000");
  }
  const url = new URL("/api/orders", parseTarget(target));
  log(`GET ${url.href}`);
  for (let attempt = 1; attempt <= MAX_ATTEMPTS; attempt++) {
    let response;
    try {
      response = await fetch(url, {
        method: "GET",
        headers: { Accept: "application/json" },
        redirect: "manual",
        signal: AbortSignal.timeout(timeoutMs),
      });
    } catch (error) {
      log(`Attempt ${attempt}/${MAX_ATTEMPTS} -> no HTTP response`);
      const detail =
        error.name === "TimeoutError"
          ? `deadline exceeded (${timeoutMs} ms)`
          : "connection failed";
      throw new Error(
        `${detail}; this demo retries only HTTP 429/503. Check that FaultDeck is running at the target.`,
      );
    }

    log(`Attempt ${attempt}/${MAX_ATTEMPTS} -> HTTP ${response.status}`);
    // Only status and Retry-After are needed. Do not buffer or print API bodies.
    await response.body?.cancel();
    if (response.ok) {
      log(`Success after ${attempt} attempt${attempt === 1 ? "" : "s"}.`);
      return { status: response.status, attempts: attempt };
    }
    if (response.status !== 429 && response.status !== 503) {
      throw new Error(
        `HTTP ${response.status} is not retryable in this demo (only 429 and 503)`,
      );
    }
    if (attempt === MAX_ATTEMPTS) {
      throw new Error(
        `Retries exhausted after ${MAX_ATTEMPTS} attempts; last status was HTTP ${response.status}`,
      );
    }
    const delay = retryDelayMs(response.headers.get("retry-after"));
    log(`Waiting ${delay} ms before retry...`);
    await wait(delay);
  }
}

function cliTarget(args) {
  if (args.length === 0) return DEFAULT_TARGET;
  if (args.length === 2 && args[0] === "--target") return args[1];
  throw new Error(
    "Usage: node examples/clients/retry.mjs [--target http://127.0.0.1:7332]",
  );
}

if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(process.argv[1]).href
) {
  try {
    if (
      process.argv.length === 3 &&
      ["--help", "-h"].includes(process.argv[2])
    ) {
      console.log(
        "Usage: node examples/clients/retry.mjs [--target http://127.0.0.1:7332]",
      );
      console.log(
        "GET /api/orders, at most 3 attempts; retries only 429/503. No control API calls.",
      );
    } else {
      await runRetryDemo({ target: cliTarget(process.argv.slice(2)) });
    }
  } catch (error) {
    console.error(`Retry demo: ${error.message}`);
    process.exitCode = 1;
  }
}
