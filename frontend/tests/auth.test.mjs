import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";
import ts from "typescript";

async function load(path) {
  const source = await readFile(new URL(path, import.meta.url), "utf8");
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022 },
  });
  return import("data:text/javascript;base64," + Buffer.from(outputText).toString("base64"));
}
const { allowedProxyPath, sessionCookie, proxyAPI } = await load("../lib/api-proxy.ts");
const api = await load("../lib/api.ts");
const config = {
  upstream: "http://api.internal:8080", origins: ["http://localhost:3000"],
  cookieName: "statuspulse_session", trustProxyIP: false,
};
const token = "a".repeat(43);
const cookie = config.cookieName + "=" + token;

test("proxy allowlists paths/methods and forwards only a single valid session cookie", () => {
  for (const [path, method] of [["auth/login", "POST"], ["monitors", "GET"], ["monitors/abc/checks", "GET"], ["events", "GET"]]) {
    assert.equal(allowedProxyPath(path, method), true);
  }
  for (const path of ["https://evil.example", "../auth/me", "monitors/a/../../auth/me", "auth/reset", "//evil"]) {
    assert.equal(allowedProxyPath(path, "GET"), false);
  }
  assert.equal(allowedProxyPath("auth/logout", "GET"), false);
  assert.equal(sessionCookie("other=secret; " + cookie, config.cookieName), cookie);
  assert.equal(sessionCookie(cookie + "; " + cookie, config.cookieName), undefined);
  assert.equal(sessionCookie(config.cookieName + "=malformed", config.cookieName), undefined);
});

test("proxy blocks untrusted mutations without making an upstream request", async (t) => {
  t.mock.method(globalThis, "fetch", () => { throw new Error("must not fetch"); });
  for (const headers of [{}, { Origin: "null", "X-StatusPulse-CSRF": "1" },
    { Origin: "https://evil.example", "X-StatusPulse-CSRF": "1" }, { Origin: "http://localhost:3000" }]) {
    const result = await proxyAPI(new Request("http://localhost:3000/api/monitors", { method: "POST", headers, body: "{}" }), "monitors", config);
    assert.equal(result.status, 403);
  }
  assert.equal((await proxyAPI(new Request("http://localhost:3000/api/events?token=secret"), "events", config)).status, 400);
});

test("proxy preserves cookies/CSRF, does not trust user identity or spoofed IP headers, and disables caching", async (t) => {
  const calls = [];
  t.mock.method(globalThis, "fetch", async (url, init) => {
    calls.push({ url, init });
    return Response.json({ id: "owned" }, { status: 201, headers: {
      "Set-Cookie": cookie + "; Path=/; HttpOnly; SameSite=Lax",
    } });
  });
  const request = new Request("http://localhost:3000/api/monitors", {
    method: "POST", body: '{"name":"Example"}', headers: {
      Origin: "http://localhost:3000", "Content-Type": "application/json",
      "X-StatusPulse-CSRF": "1", Cookie: "unrelated=secret; " + cookie,
      "X-User-ID": "another-user", "X-Forwarded-For": "1.2.3.4",
    },
  });
  const result = await proxyAPI(request, "monitors", config);
  assert.equal(result.status, 201);
  assert.equal(result.headers.get("Cache-Control"), "no-store");
  assert.match(result.headers.get("Set-Cookie"), /HttpOnly/);
  assert.deepEqual(await result.json(), { id: "owned" });
  assert.equal(calls[0].url, "http://api.internal:8080/monitors");
  assert.equal(calls[0].init.headers.get("Cookie"), cookie);
  assert.equal(calls[0].init.headers.get("Origin"), "http://localhost:3000");
  assert.equal(calls[0].init.headers.get("X-StatusPulse-CSRF"), "1");
  assert.equal(calls[0].init.headers.get("X-User-ID"), null);
  assert.equal(calls[0].init.headers.get("X-Forwarded-For"), null);
  assert.equal(calls[0].init.redirect, "manual");
  assert.equal(calls[0].init.cache, "no-store");
});

test("proxy never follows upstream redirects and returns safe network errors", async (t) => {
  t.mock.method(globalThis, "fetch", async () => new Response(null, { status: 302, headers: { Location: "https://evil.example" } }));
  const request = new Request("http://localhost:3000/api/auth/me");
  assert.equal((await proxyAPI(request, "auth/me", config)).status, 502);
  t.mock.method(globalThis, "fetch", async () => { throw new Error("internal password=secret"); });
  const result = await proxyAPI(request, "auth/me", config);
  assert.equal(result.status, 502);
  assert.doesNotMatch(await result.text(), /secret/);
});

test("SSE proxy streams immediately and cancels upstream when the client disconnects", async (t) => {
  let signal;
  let cancelled = false;
  t.mock.method(globalThis, "fetch", async (_url, init) => {
    signal = init.signal;
    return new Response(new ReadableStream({
      start(controller) { controller.enqueue(new TextEncoder().encode('event: check\ndata: {"monitor_id":"a"}\n\n')); },
      cancel() { cancelled = true; },
    }), { headers: { "Content-Type": "text/event-stream" } });
  });
  const result = await proxyAPI(new Request("http://localhost:3000/api/events", { headers: { Cookie: cookie } }), "events", config);
  assert.equal(result.headers.get("Content-Type"), "text/event-stream");
  const reader = result.body.getReader();
  const { value } = await reader.read();
  assert.match(new TextDecoder().decode(value), /event: check/);
  await reader.cancel();
  assert.equal(signal.aborted, true);
  assert.equal(cancelled, true);
});

test("browser mutation helpers use the same-origin API and CSRF header without a JS session token", async (t) => {
  const calls = [];
  t.mock.method(globalThis, "fetch", async (url, init) => {
    calls.push({ url, init });
    return Response.json({ id: "monitor" });
  });
  await api.createMonitor({ name: "Example", url: "https://example.com", expected_status_code: 200, interval_seconds: 30, timeout_seconds: 5 });
  await api.updateMonitorActive("monitor", false);
  await api.deleteMonitor("monitor");
  for (const { url, init } of calls) {
    assert.ok(url.startsWith("/api/monitors"));
    assert.equal(init.headers.get("X-StatusPulse-CSRF"), "1");
    assert.equal(init.headers.get("Cookie"), null);
    assert.equal(init.credentials, "same-origin");
    assert.equal(init.cache, "no-store");
  }
});

test("SSR hydration uses the supplied authenticated reader for monitors and every latest check", async () => {
  const calls = [];
  const request = async (path) => {
    calls.push(path);
    if (path === "/monitors") return Response.json([{ id: "owned-monitor" }]);
    return Response.json([{ monitor_id: "owned-monitor", reachable: true }]);
  };
  const result = await api.getMonitorsWithLatestChecks(request);
  assert.equal(result[0].latestCheck.reachable, true);
  assert.deepEqual(calls, ["/monitors", "/monitors/owned-monitor/checks?limit=1"]);
  await assert.rejects(api.getMonitorsWithLatestChecks(async (path) => {
    if (path === "/monitors") return Response.json([{ id: "owned-monitor" }]);
    throw new api.ApiError("Authentication required", 401);
  }), (error) => error.status === 401);
});

test("login errors stay on the form, but protected 401 responses signal session termination", async (t) => {
  const events = [];
  globalThis.window = { dispatchEvent: (event) => events.push(event.type) };
  t.after(() => { delete globalThis.window; });
  t.mock.method(globalThis, "fetch", async () => Response.json({ error: "invalid email or password" }, { status: 401 }));
  await assert.rejects(api.authenticate("login", "a@example.com", "wrong"), /invalid email or password/);
  assert.deepEqual(events, []);
  await assert.rejects(api.getSessionUser(), (error) => error.status === 401);
  assert.deepEqual(events, ["statuspulse:session-ended"]);
});
