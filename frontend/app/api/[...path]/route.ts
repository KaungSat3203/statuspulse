import { proxyAPI } from "@/lib/api-proxy";
import { serverConfig } from "@/lib/server-config";

export const runtime = "nodejs";
export const dynamic = "force-dynamic";

async function handle(request: Request, context: { params: Promise<{ path: string[] }> }) {
  const { path } = await context.params;
  return proxyAPI(request, path.join("/"), serverConfig());
}

export { handle as GET, handle as POST, handle as PATCH, handle as DELETE };
