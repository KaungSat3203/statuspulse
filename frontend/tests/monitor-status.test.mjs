import assert from "node:assert/strict";
import test from "node:test";

import { readFile } from "node:fs/promises";
import ts from "typescript";

// Use the compiler already installed by Next.js; no additional test framework.
async function loadTypeScript(relativePath) {
  const source = await readFile(new URL(relativePath, import.meta.url), "utf8");
  const { outputText } = ts.transpileModule(source, {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2020 },
  });
  return import(`data:text/javascript;base64,${Buffer.from(outputText).toString("base64")}`);
}

const { getMonitorsWithLatestChecks } = await loadTypeScript("../lib/api.ts");
const {
  countMonitorStatuses,
  formatCheckLatency,
  formatCheckStatus,
  getMonitorStatus,
} = await loadTypeScript("../lib/monitor-status.ts");

const monitor = {
  id: "11111111-1111-1111-1111-111111111111",
  name: "Example",
  url: "https://example.com",
  expected_status_code: 200,
  interval_seconds: 30,
  timeout_seconds: 5,
  active: true,
  consecutive_failures: 0,
  consecutive_successes: 0,
  created_at: "2026-10-08T00:00:00Z",
  updated_at: "2026-10-08T00:00:00Z",
};
const check = {
  monitor_id: monitor.id,
  checked_at: "2026-10-08T01:00:00Z",
  status_code: 200,
  latency_ms: 25,
  reachable: true,
  error: "",
};

test("status requires a real check and ignores counters", () => {
  assert.equal(getMonitorStatus(monitor), "AWAITING CHECK");
  assert.equal(getMonitorStatus({ ...monitor, consecutive_failures: 5 }), "AWAITING CHECK");
  assert.equal(getMonitorStatus({ ...monitor, consecutive_failures: 5, latestCheck: check }), "UP");
  assert.equal(getMonitorStatus({ ...monitor, latestCheck: { ...check, reachable: false, status_code: 500 } }), "DOWN");
});

test("pause takes precedence and resume retains the last actual check", () => {
  const known = { ...monitor, latestCheck: check };
  assert.equal(getMonitorStatus({ ...known, active: false }), "PAUSED");
  assert.equal(getMonitorStatus({ ...known, active: true }), "UP");
  assert.equal(getMonitorStatus({ ...monitor, active: false }), "PAUSED");
  assert.equal(getMonitorStatus({ ...monitor, active: true }), "AWAITING CHECK");
});

test("overview partitions monitors into four independent states", () => {
  assert.deepEqual(countMonitorStatuses([
    { ...monitor, latestCheck: check },
    { ...monitor, latestCheck: { ...check, reachable: false } },
    { ...monitor, active: false, latestCheck: check },
    monitor,
  ]), { UP: 1, DOWN: 1, PAUSED: 1, "AWAITING CHECK": 1 });
  assert.deepEqual(countMonitorStatuses([]), { UP: 0, DOWN: 0, PAUSED: 0, "AWAITING CHECK": 0 });
});

test("missing and zero readings use a dash, HTTP failures retain their code", () => {
  assert.equal(formatCheckStatus(), "—");
  assert.equal(formatCheckLatency(), "—");
  const unreachable = { ...check, reachable: false, status_code: 0, latency_ms: 0, error: "network error" };
  assert.equal(formatCheckStatus(unreachable), "—");
  assert.equal(formatCheckLatency(unreachable), "—");
  assert.equal(formatCheckStatus({ ...check, reachable: false, status_code: 500 }), "500");
  assert.equal(formatCheckLatency(check), "25 ms");
});

test("initial loading fetches one persisted check per monitor on every refresh", async (t) => {
  const requests = [];
  let reachable = false;
  t.mock.method(globalThis, "fetch", async (url, options) => {
    requests.push({ url: String(url), options });
    if (String(url).endsWith("/monitors")) return Response.json([monitor]);
    assert.ok(String(url).endsWith("/checks?limit=1"));
    return Response.json([{ ...check, reachable }]);
  });

  const initial = await getMonitorsWithLatestChecks();
  assert.equal(getMonitorStatus(initial[0]), "DOWN");
  reachable = true;
  const refreshed = await getMonitorsWithLatestChecks();
  assert.equal(getMonitorStatus(refreshed[0]), "UP");
  assert.equal(requests.length, 4);
  assert.ok(requests.every(({ options }) => options.cache === "no-store"));
});

test("empty history and failed history loading never imply UP", async (t) => {
  const paused = { ...monitor, id: "22222222-2222-2222-2222-222222222222", active: false };
  t.mock.method(globalThis, "fetch", async (url) => {
    if (String(url).endsWith("/monitors")) return Response.json([monitor, paused]);
    if (String(url).includes(paused.id)) return new Response("unavailable", { status: 503 });
    return Response.json([]);
  });

  const results = await getMonitorsWithLatestChecks();
  assert.equal(getMonitorStatus(results[0]), "AWAITING CHECK");
  assert.equal(results[0].latestCheckError, undefined);
  assert.equal(getMonitorStatus(results[1]), "PAUSED");
  assert.match(results[1].latestCheckError, /Unable to load/);
});
