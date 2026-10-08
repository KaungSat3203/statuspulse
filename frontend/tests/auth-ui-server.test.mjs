import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { createRequire } from "node:module";
import { pathToFileURL } from "node:url";
import test from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import ts from "typescript";

const require = createRequire(import.meta.url);
const reactURL = pathToFileURL(require.resolve("react")).href;
const moduleURL = (source) => "data:text/javascript;base64," + Buffer.from(source).toString("base64");
async function compile(path) {
  return ts.transpileModule(await readFile(new URL(path, import.meta.url), "utf8"), {
    compilerOptions: { module: ts.ModuleKind.ESNext, target: ts.ScriptTarget.ES2022, jsx: ts.JsxEmit.React },
  }).outputText;
}
const apiURL = moduleURL(await compile("../lib/api.ts"));
const api = await import(apiURL);
const linkURL = moduleURL('import React from "' + reactURL + '"; export default function Link(props) { return React.createElement("a", props); }');
const formURL = moduleURL('import React from "' + reactURL + '";\n' + (await compile("../components/auth/auth-form.tsx"))
  .replace('from "react"', 'from "' + reactURL + '"')
  .replace('from "next/link"', 'from "' + linkURL + '"')
  .replace('from "@/lib/api"', 'from "' + apiURL + '"'));
const AuthForm = (await import(formURL)).default;

test("auth forms render accessible labels, appropriate autocomplete, password visibility and navigation", () => {
  for (const mode of ["login", "signup"]) {
    const html = renderToStaticMarkup(React.createElement(AuthForm, { mode }));
    assert.match(html, /for="email"/);
    assert.match(html, /for="password"/);
    assert.match(html, /type="email"/);
    assert.match(html, /type="password"/);
    assert.match(html, /aria-label="Show password"/);
    assert.match(html, /aria-pressed="false"/);
    assert.match(html, mode === "signup" ? /autoComplete="new-password"/ : /autoComplete="current-password"/);
    assert.match(html, mode === "signup" ? /href="\/login"/ : /href="\/signup"/);
  }
});

const runtimeKey = "__statuspulseServerAuthTest";
const cookiesURL = moduleURL('export const cookies = async () => ({ get: () => ({ value: globalThis.' + runtimeKey + '.token }) });');
const configURL = moduleURL('export const serverConfig = () => ({ upstream: "http://api.internal", cookieName: "statuspulse_session" });');
const navigationURL = moduleURL('export function redirect(path) { const error = new Error("redirect"); error.path = path; throw error; }');
const serverURL = moduleURL((await compile("../lib/server-api.ts"))
  .replace('import "server-only";', "")
  .replace('from "next/headers"', 'from "' + cookiesURL + '"')
  .replace('from "next/navigation"', 'from "' + navigationURL + '"')
  .replace('from "./api"', 'from "' + apiURL + '"')
  .replace('from "./server-config"', 'from "' + configURL + '"'));
const server = await import(serverURL);

test("SSR clients forward only their own request cookie without global user state or caching", async (t) => {
  globalThis[runtimeKey] = { token: "a".repeat(43) };
  t.after(() => { delete globalThis[runtimeKey]; });
  const first = await server.serverAPI();
  globalThis[runtimeKey].token = "b".repeat(43);
  const second = await server.serverAPI();
  const calls = [];
  t.mock.method(globalThis, "fetch", async (url, init) => {
    calls.push({ url, init });
    return Response.json({ id: "user", email: "test@example.com" });
  });
  await first("/auth/me");
  await second("/auth/me");
  assert.equal(calls[0].init.headers.get("Cookie"), "statuspulse_session=" + "a".repeat(43));
  assert.equal(calls[1].init.headers.get("Cookie"), "statuspulse_session=" + "b".repeat(43));
  assert.equal(calls[0].url, "http://api.internal/auth/me");
  assert.equal(calls[0].init.cache, "no-store");
  assert.equal(calls[0].init.redirect, "error");
  await assert.rejects(first("//evil.example"));
  await assert.rejects(first("/monitors", { method: "POST" }));
});

test("server authentication distinguishes expiry from outages and redirects only unauthorized requests", async () => {
  assert.equal(await server.currentUser(async () => { throw new api.ApiError("expired", 401); }), null);
  await assert.rejects(server.currentUser(async () => new Response(null, { status: 503 })), /temporarily unavailable/);
  assert.throws(() => server.handleServerAuthError(new api.ApiError("expired", 401)), (error) => error.path === "/login");
  assert.throws(() => server.handleServerAuthError(new Error("offline")), /offline/);
});
