import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import test from "node:test";
import { pathToFileURL } from "node:url";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import ts from "typescript";

// No DOM/testing dependencies: render shared states with React and test the
// dialogs' request lifecycle using a minimal, isolated hook stub.
const require = createRequire(import.meta.url);
const reactURL = pathToFileURL(require.resolve("react")).href;
const runtimeKey = "__statuspulseDialogTestRuntime";
function moduleURL(source) {
  return `data:text/javascript;base64,${Buffer.from(source).toString("base64")}`;
}
async function compile(path) {
  const source = await readFile(new URL(path, import.meta.url), "utf8");
  return ts.transpileModule(source, {
    compilerOptions: {
      module: ts.ModuleKind.ESNext,
      target: ts.ScriptTarget.ES2020,
      jsx: ts.JsxEmit.React,
    },
  }).outputText;
}
const sharedURL = moduleURL(
  `import React from "${reactURL}";\n` +
  await compile("../components/ui/dialog-data-state.tsx"),
);
const states = await import(sharedURL);
const analyticsURL = moduleURL(await compile("../lib/monitor-analytics.ts"));
const hooksURL = moduleURL(`
  export const useState = (...args) => globalThis.${runtimeKey}.useState(...args);
  export const useEffect = (...args) => globalThis.${runtimeKey}.useEffect(...args);
  export const useMemo = (callback) => callback();
`);
const apiURL = moduleURL(`
  export const getMonitorChecks = (...args) => globalThis.${runtimeKey}.request(...args);
  export const getMonitorMetrics = (...args) => globalThis.${runtimeKey}.request(...args);
`);
const chartURL = moduleURL(`
  export const CartesianGrid = () => null;
  export const Line = () => null;
  export const LineChart = () => null;
  export const ResponsiveContainer = () => null;
  export const Tooltip = () => null;
  export const XAxis = () => null;
  export const YAxis = () => null;
`);
async function loadDialog(filename) {
  const source = await compile(`../components/monitor/${filename}`);
  return (await import(moduleURL(
    `import React from "${reactURL}";\n` +
    source
      .replace('from "react"', `from "${hooksURL}"`)
      .replace('from "@/lib/api"', `from "${apiURL}"`)
      .replace('from "@/lib/monitor-analytics"', `from "${analyticsURL}"`)
      .replace('from "@/components/ui/dialog-data-state"', `from "${sharedURL}"`)
      .replace('from "recharts"', `from "${chartURL}"`),
  ))).default;
}
const History = await loadDialog("monitor-history-dialog.tsx");
const Analytics = await loadDialog("monitor-analytics-dialog.tsx");

function findElement(node, type) {
  if (!node || typeof node !== "object") return undefined;
  if (Array.isArray(node)) {
    for (const child of node) {
      const found = findElement(child, type);
      if (found) return found;
    }
    return undefined;
  }
  if (node.type === type) return node;
  return findElement(node.props?.children, type);
}
const flush = () => new Promise(resolve => setImmediate(resolve));
function deferred() {
  let resolve, reject;
  const promise = new Promise((res, rej) => { resolve = res; reject = rej; });
  return { promise, resolve, reject };
}
function createHarness(t, Component) {
  const values = [];
  const requests = [];
  let index = 0;
  let dependencies;
  let nextEffect;
  let cleanup;
  let updates = 0;
  let props = { monitor: { id: "monitor-1", name: "Website", active: true }, onClose() {} };
  globalThis[runtimeKey] = {
    useState(initial) {
      const slot = index++;
      if (slot === values.length) values.push(initial);
      return [values[slot], value => {
        updates++;
        values[slot] = typeof value === "function" ? value(values[slot]) : value;
      }];
    },
    useEffect(effect, deps) {
      if (!dependencies || deps.some((value, i) => !Object.is(value, dependencies[i]))) {
        dependencies = deps;
        nextEffect = effect;
      }
    },
    request(id, limit) {
      const pending = deferred();
      requests.push({ id, limit, ...pending });
      return pending.promise;
    },
  };
  const harness = {
    requests,
    values,
    get updates() { return updates; },
    render(nextProps) {
      if (nextProps) props = { ...props, ...nextProps };
      index = 0;
      const tree = Component(props);
      if (nextEffect) {
        cleanup?.();
        const effect = nextEffect;
        nextEffect = undefined;
        cleanup = effect();
      }
      return tree;
    },
    unmount() { cleanup?.(); },
  };
  t.after(() => {
    harness.unmount();
    delete globalThis[runtimeKey];
  });
  return harness;
}

test("shared loading states provide matching placeholders without fake readings", () => {
  for (const variant of ["history", "analytics"]) {
    const html = renderToStaticMarkup(React.createElement(states.DialogLoadingState, {
      label: "Loading data…", variant,
    }));
    assert.match(html, /role="status"/);
    assert.match(html, /aria-busy="true"/);
    assert.match(html, /aria-hidden="true"/);
    assert.match(html, /motion-safe:animate-pulse/);
    assert.doesNotMatch(html, /HTTP 200|0 ms|No .*yet/);
  }
});

test("error state preserves and escapes the error and offers a manual Retry", () => {
  let retries = 0;
  const props = { title: "Unable to load", message: "<request failed>", onRetry: () => retries++ };
  const tree = states.DialogErrorState(props);
  const html = renderToStaticMarkup(React.createElement(states.DialogErrorState, props));
  assert.match(html, /role="alert"/);
  assert.match(html, /&lt;request failed&gt;/);
  assert.match(html, />Retry<\/button>/);
  findElement(tree, "button").props.onClick();
  assert.equal(retries, 1);
});

test("empty state has explanatory copy and no error or Retry", () => {
  const html = renderToStaticMarkup(React.createElement(states.DialogEmptyState, {
    title: "No data yet", description: "Data is available after the first check.",
  }));
  assert.match(html, /No data yet/);
  assert.match(html, /after the first check/);
  assert.doesNotMatch(html, /role="alert"|Retry/);
});

for (const [name, Component, limit, result] of [
  ["History", History, 50, { checked_at: "2026-10-08T00:00:00Z", reachable: false, status_code: 0, latency_ms: 0, error: "network error" }],
  ["Analytics", Analytics, 30, { metric_date: "2026-10-08T00:00:00Z", total_checks: 10, successful_checks: 9 }],
]) {
  test(`${name}: retry clears errors, reloads the same monitor, and preserves successful results`, async t => {
    const h = createHarness(t, Component);
    assert.ok(findElement(h.render(), states.DialogLoadingState));
    assert.equal(h.requests.length, 1);
    assert.equal(h.requests[0].limit, limit);
    h.requests[0].reject(new Error("API temporarily unavailable"));
    await flush();
    const failure = findElement(h.render(), states.DialogErrorState);
    assert.equal(failure.props.message, "API temporarily unavailable");
    failure.props.onRetry();
    assert.equal(h.values[0], null);
    assert.equal(h.values[1], null);
    assert.ok(findElement(h.render(), states.DialogLoadingState));
    assert.equal(h.requests.length, 2);
    assert.equal(h.requests[1].id, "monitor-1");
    assert.equal(h.requests[1].limit, limit);
    h.requests[1].resolve([result]);
    await flush();
    const loaded = h.render();
    assert.deepEqual(h.values[0], [result]);
    assert.equal(findElement(loaded, states.DialogErrorState), undefined);
    assert.equal(findElement(loaded, states.DialogLoadingState), undefined);
    h.render();
    assert.equal(h.requests.length, 2, "ordinary renders must not refetch");
  });

  test(`${name}: repeated failures remain retryable and empty success is not an error`, async t => {
    const h = createHarness(t, Component);
    h.render();
    for (const message of ["first failure", "second failure"]) {
      h.requests.at(-1).reject(new Error(message));
      await flush();
      const error = findElement(h.render(), states.DialogErrorState);
      assert.equal(error.props.message, message);
      error.props.onRetry();
      h.render();
    }
    h.requests.at(-1).resolve([]);
    await flush();
    const tree = h.render();
    const empty = findElement(tree, states.DialogEmptyState);
    assert.ok(empty);
    assert.equal(findElement(tree, states.DialogErrorState), undefined);
    if (name === "Analytics") assert.match(empty.props.description, /completed UTC days/);
  });

  test(`${name}: pending responses are ignored after unmount`, async t => {
    const h = createHarness(t, Component);
    h.render();
    h.unmount();
    const updates = h.updates;
    h.requests[0].resolve([result]);
    await flush();
    assert.equal(h.updates, updates);
  });

  test(`${name}: changing monitors ignores the old request and resets loading`, async t => {
    const h = createHarness(t, Component);
    h.render();
    h.render({ monitor: { id: "monitor-2", name: "Second", active: true } });
    assert.ok(findElement(h.render(), states.DialogLoadingState));
    assert.equal(h.requests.length, 2);
    h.requests[1].resolve([result]);
    await flush();
    h.requests[0].reject(new Error("stale monitor failure"));
    await flush();
    h.render();
    assert.deepEqual(h.values[0], [result]);
    assert.equal(h.values[1], null);
    assert.equal(h.requests[1].id, "monitor-2");
  });
}

test("History: paused monitor empty state explains how checking can start", async t => {
  const h = createHarness(t, History);
  h.render({ monitor: { id: "monitor-1", name: "Paused", active: false } });
  h.requests[0].resolve([]);
  await flush();
  assert.match(findElement(h.render(), states.DialogEmptyState).props.description, /Resume/);
});
