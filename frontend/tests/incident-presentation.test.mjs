import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import test from "node:test";
import { pathToFileURL } from "node:url";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import ts from "typescript";

// Reuse the installed compiler, React renderer, and Node's built-in test runner.
const require = createRequire(import.meta.url);
async function compile(relativePath) {
  const source = await readFile(new URL(relativePath, import.meta.url), "utf8");
  return ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2020,
      jsx: ts.JsxEmit.React,
    },
  }).outputText;
}
function moduleURL(source) {
  return `data:text/javascript;base64,${Buffer.from(source).toString("base64")}`;
}
const helperURL = moduleURL(await compile("../lib/incident-presentation.ts"));
const { formatDowntime, formatIncidentTime } = await import(helperURL);
const componentSource = await compile("../components/incident/incident-list.tsx");
const { default: IncidentList } = await import(moduleURL(
  `import React from "${pathToFileURL(require.resolve("react")).href}";\n` +
  componentSource.replace("@/lib/incident-presentation", helperURL),
));

const openIncident = {
  id: "incident-1",
  monitor_id: "monitor-1",
  started_at: "2026-10-08T10:32:15Z",
  created_at: "2026-10-08T10:32:15Z",
  reason: "Unexpected HTTP status: 500",
};
const recoveredIncident = {
  ...openIncident,
  id: "incident-2",
  recovered_at: "2026-10-08T10:34:29Z",
  downtime_seconds: 134,
};
function render(props = {}) {
  return renderToStaticMarkup(React.createElement(IncidentList, {
    incidents: [openIncident, recoveredIncident],
    monitors: [{ id: "monitor-1", name: "Website" }],
    error: null,
    ...props,
  }));
}

test("incident timestamps use explicit UTC consistently across browser timezones", () => {
  const originalTimezone = process.env.TZ;
  try {
    process.env.TZ = "America/Los_Angeles";
    assert.equal(formatIncidentTime(openIncident.started_at), "Oct 8, 2026, 10:32:15 UTC");
    assert.equal(formatIncidentTime("2026-10-08T17:32:15+07:00"), "Oct 8, 2026, 10:32:15 UTC");
    assert.equal(formatIncidentTime("invalid"), "—");
  } finally {
    if (originalTimezone === undefined) delete process.env.TZ;
    else process.env.TZ = originalTimezone;
  }
});

test("downtime formats seconds, minutes, hours and days without losing zero", () => {
  assert.equal(formatDowntime(0), "0s");
  assert.equal(formatDowntime(59), "59s");
  assert.equal(formatDowntime(60), "1m");
  assert.equal(formatDowntime(134), "2m 14s");
  assert.equal(formatDowntime(3661), "1h 1m 1s");
  assert.equal(formatDowntime(90061), "1d 1h 1m 1s");
  assert.equal(formatDowntime(1.9), "1s");
  assert.equal(formatDowntime(-1), "—");
  assert.equal(formatDowntime(Infinity), "—");
});

test("open and recovered incidents have distinct emphasis and retain API ordering", () => {
  const articles = [...render().matchAll(/<article\b[^>]*>.*?<\/article>/g)].map(match => match[0]);
  assert.equal(articles.length, 2);
  assert.match(articles[0], /border-l-red-400/);
  assert.match(articles[0], />Open<\/span>/);
  assert.doesNotMatch(articles[0], /<dt[^>]*>Recovered/);
  assert.match(articles[1], /border-l-zinc-700/);
  assert.match(articles[1], />Recovered<\/span>/);
  assert.match(articles[1], /2m 14s/);
  assert.match(articles[1], /dateTime="2026-10-08T10:34:29Z"/i);
  assert.ok(articles.every(article => article.includes("Website")));
  assert.ok(articles.every(article => article.includes("Unexpected HTTP status: 500")));
});

test("reason text wraps and is escaped, with sensible missing-name and reason labels", () => {
  const html = render({
    incidents: [{ ...openIncident, monitor_id: "missing", reason: "<script>error</script>" }],
  });
  assert.match(html, /Unknown monitor/);
  assert.match(html, /overflow-wrap:anywhere/);
  assert.match(html, /whitespace-pre-wrap/);
  assert.match(html, /&lt;script&gt;error&lt;\/script&gt;/);
  assert.doesNotMatch(html, /<script>/);
  assert.match(render({ incidents: [{ ...openIncident, reason: "" }] }), /Monitor check failed/);
});

test("empty state explains incident history without claiming all monitors are healthy", () => {
  const html = render({ incidents: [] });
  assert.match(html, /No recent incidents/);
  assert.match(html, /reaches its failure threshold/);
  assert.doesNotMatch(html, /<article/);
  assert.doesNotMatch(html, /all monitors.*healthy/i);
});

test("fetch errors remain visible instead of showing misleading empty or stale data", () => {
  const html = render({ error: "Unable to load recent incidents right now." });
  assert.match(html, /role="alert"/);
  assert.match(html, /Unable to load incidents/);
  assert.match(html, /Unable to load recent incidents right now/);
  assert.doesNotMatch(html, /<article/);
  assert.doesNotMatch(html, /No recent incidents/);
});

test("recovered zero-second downtime remains visible, missing downtime stays omitted", () => {
  assert.match(render({
    incidents: [{ ...recoveredIncident, downtime_seconds: 0 }],
  }), />0s<\/dd>/);
  assert.doesNotMatch(render({
    incidents: [openIncident],
  }), /<dt[^>]*>Downtime/);
});
