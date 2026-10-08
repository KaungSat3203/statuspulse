import "server-only";
import { cookies } from "next/headers";
import { redirect } from "next/navigation";

import { ApiError, type ApiFetch, type User } from "./api";
import { serverConfig } from "./server-config";

// A per-request closure: never keep a cookie or authenticated client globally.
export async function serverAPI(): Promise<ApiFetch> {
  const config = serverConfig();
  const token = (await cookies()).get(config.cookieName)?.value;
  return async (path, init = {}) => {
    if (!path.startsWith("/") || path.startsWith("//") || (init.method && init.method !== "GET")) {
      throw new Error("Invalid server API read.");
    }
    const headers = new Headers();
    if (token && /^[A-Za-z0-9_-]{43}$/.test(token)) headers.set("Cookie", config.cookieName + "=" + token);
    const response = await fetch(config.upstream + path, {
      ...init, headers, cache: "no-store", redirect: "error", signal: AbortSignal.timeout(15_000),
    });
    if (response.status === 401) throw new ApiError("Authentication required.", 401);
    return response;
  };
}

export async function currentUser(request: ApiFetch): Promise<User | null> {
  try {
    const response = await request("/auth/me");
    if (!response.ok) throw new Error("Authentication is temporarily unavailable.");
    return response.json();
  } catch (error) {
    if (error instanceof ApiError && error.status === 401) return null;
    throw error; // An unavailable API is not evidence that the user is logged out.
  }
}

export function handleServerAuthError(error: unknown): never {
  if (error instanceof ApiError && error.status === 401) redirect("/login");
  throw error;
}
