import "server-only";

export function serverConfig() {
  const production = process.env.APP_ENV === "production";
  const rawOrigins = process.env.FRONTEND_ORIGINS ?? (production ? "" : "http://localhost:3000");
  const origins = rawOrigins.split(",").map((value) => value.trim()).filter(Boolean);
  if (!origins.length || origins.some((origin) => {
    try {
      const url = new URL(origin);
      return url.origin !== origin || !["http:", "https:"].includes(url.protocol) ||
        (production && url.protocol !== "https:");
    } catch { return true; }
  })) throw new Error("Configure exact FRONTEND_ORIGINS (HTTPS in production).");

  const upstream = new URL(process.env.STATUSPULSE_API_URL ?? "http://localhost:8080");
  if (!["http:", "https:"].includes(upstream.protocol) || upstream.username || upstream.password ||
    upstream.pathname !== "/" || upstream.search || upstream.hash) {
    throw new Error("STATUSPULSE_API_URL must be a fixed HTTP origin.");
  }
  return {
    upstream: upstream.origin,
    origins,
    cookieName: production ? "__Host-statuspulse_session" : "statuspulse_session",
    // Enable ONLY behind a trusted edge that overwrites X-Forwarded-For.
    trustProxyIP: process.env.STATUSPULSE_TRUST_PROXY_IP === "true",
  };
}
