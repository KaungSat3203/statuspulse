import { isIP } from "node:net";

type ProxyConfig = {
  upstream: string;
  origins: string[];
  cookieName: string;
  trustProxyIP: boolean;
};

export function allowedProxyPath(path: string, method: string): boolean {
  if (/^auth\/(login|signup|logout)$/.test(path)) return method === "POST";
  if (path === "auth/me" || path === "events" || path === "incidents") return method === "GET";
  if (path === "monitors") return ["GET", "POST"].includes(method);
  if (/^monitors\/[a-zA-Z0-9-]{1,64}$/.test(path)) return ["GET", "PATCH", "DELETE"].includes(method);
  return /^monitors\/[a-zA-Z0-9-]{1,64}\/(checks|metrics)$/.test(path) && method === "GET";
}

// Forward only the session cookie, never the entire browser Cookie header.
export function sessionCookie(header: string | null, name: string): string | undefined {
  const matches = (header ?? "").split(";").map((part) => part.trim()).filter((part) => part.startsWith(name + "="));
  if (matches.length !== 1 || !new RegExp("^" + name + "=[A-Za-z0-9_-]{43}$").test(matches[0])) return undefined;
  return matches[0];
}

function problem(status: number, error: string) {
  return Response.json({ error }, { status, headers: { "Cache-Control": "no-store" } });
}

async function boundedBody(request: Request): Promise<Uint8Array | undefined> {
  if (!request.body) return undefined;
  const reader = request.body.getReader();
  const chunks: Uint8Array[] = [];
  let size = 0;
  try {
    for (;;) {
      const { done, value } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > 16 * 1024) { await reader.cancel(); throw new Error("body too large"); }
      chunks.push(value);
    }
  } finally { reader.releaseLock(); }
  const body = new Uint8Array(size);
  let offset = 0;
  for (const chunk of chunks) { body.set(chunk, offset); offset += chunk.length; }
  return body;
}

export async function proxyAPI(request: Request, path: string, config: ProxyConfig): Promise<Response> {
  if (!allowedProxyPath(path, request.method)) return problem(404, "not found");
  const origin = request.headers.get("origin");
  const mutation = !["GET", "HEAD"].includes(request.method);
  if ((origin && !config.origins.includes(origin)) ||
    (mutation && (!origin || request.headers.get("X-StatusPulse-CSRF") !== "1"))) {
    return problem(403, "invalid request origin or CSRF header");
  }
  const query = new URL(request.url).searchParams;
  if ([...query.keys()].some((key) => key !== "limit") || query.getAll("limit").length > 1 ||
    (query.get("limit")?.length ?? 0) > 32) return problem(400, "invalid query");

  const headers = new Headers();
  for (const key of ["origin", "content-type", "x-statuspulse-csrf", "sec-fetch-site"]) {
    const value = request.headers.get(key);
    if (value) headers.set(key, value);
  }
  const cookie = sessionCookie(request.headers.get("cookie"), config.cookieName);
  if (cookie) headers.set("Cookie", cookie);
  if (config.trustProxyIP) {
    const ip = request.headers.get("x-forwarded-for")?.split(",")[0].trim();
    if (ip && isIP(ip)) headers.set("X-Forwarded-For", ip);
  }
  let body: Uint8Array | undefined;
  try { if (mutation) body = await boundedBody(request); }
  catch { return problem(413, "request body too large"); }
  const abort = new AbortController();
  const onAbort = () => abort.abort();
  request.signal.addEventListener("abort", onAbort, { once: true });
  if (request.signal.aborted) abort.abort();
  // Bound the connection/header wait; SSE has no whole-stream timeout.
  const timeout = setTimeout(() => abort.abort(), 15_000);
  const cleanup = () => {
    clearTimeout(timeout);
    request.signal.removeEventListener("abort", onAbort);
  };
  try {
    const suffix = query.size ? "?" + query.toString() : "";
    const upstream = await fetch(config.upstream + "/" + path + suffix, {
      method: request.method, headers, body: body as BodyInit | undefined,
      cache: "no-store", redirect: "manual", signal: abort.signal,
    });
    if (path === "events") clearTimeout(timeout);
    // Redirects could leak cookies to a different upstream. Never follow them.
    if (upstream.status >= 300 && upstream.status < 400) {
      await upstream.body?.cancel(); abort.abort(); cleanup();
      return problem(502, "unexpected API redirect");
    }
    const responseHeaders = new Headers({ "Cache-Control": "no-store", "X-Content-Type-Options": "nosniff" });
    for (const key of ["content-type", "retry-after", "x-accel-buffering"]) {
      const value = upstream.headers.get(key);
      if (value) responseHeaders.set(key, value);
    }
    for (const value of upstream.headers.getSetCookie()) responseHeaders.append("Set-Cookie", value);
    if (!upstream.body) { cleanup(); return new Response(null, { status: upstream.status, headers: responseHeaders }); }
    const reader = upstream.body.getReader();
    const stream = new ReadableStream({
      async pull(controller) {
        try {
          const { done, value } = await reader.read();
          if (done) { cleanup(); controller.close(); return; }
          controller.enqueue(value);
        } catch { cleanup(); abort.abort(); controller.error(new Error("API connection closed")); }
      },
      async cancel() {
        cleanup(); abort.abort();
        await reader.cancel().catch(() => {});
      },
    });
    return new Response(stream, { status: upstream.status, headers: responseHeaders });
  } catch {
    cleanup(); abort.abort();
    return problem(502, "API temporarily unavailable");
  }
}
