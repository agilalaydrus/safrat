import { createConnectTransport } from "@connectrpc/connect-web";
import { authClient } from "./auth-client";

// authClient.getSession() is a real network round trip to
// /api/auth/get-session. Attaching it as a per-RPC interceptor meant every
// single Connect call the app makes — and a single page load easily fires
// several in parallel (season, operator, groups, pilgrims, ...) — paid that
// latency on its own, which is why navigation felt heavy. Cache the resolved
// token in memory for a few seconds so a burst of RPC calls shares one
// lookup instead of each doing its own; the server-side cookie cache
// (lib/auth.ts) backs this up for calls that land just after it expires.
const TOKEN_TTL_MS = 10_000;
let cachedToken: { value: string; expiresAt: number } | undefined;
let pending: Promise<string | undefined> | undefined;

async function resolveToken(): Promise<string | undefined> {
  const now = Date.now();
  if (cachedToken && cachedToken.expiresAt > now) return cachedToken.value;
  if (!pending) {
    pending = authClient
      .getSession({ fetchOptions: { cache: "no-store" } })
      .then((session) => {
        const token = session.data?.session?.token;
        cachedToken = token ? { value: token, expiresAt: Date.now() + TOKEN_TTL_MS } : undefined;
        return token;
      })
      .catch(() => undefined)
      .finally(() => { pending = undefined; });
  }
  return pending;
}

/** Call whenever the underlying Better Auth session changes (sign-in, sign-out, role change) — otherwise an RPC fired in the TOKEN_TTL_MS window right before/after can still carry the previous session's token and get rejected as unauthenticated. */
export function invalidateTokenCache() {
  cachedToken = undefined;
}

export const transport = createConnectTransport({
  baseUrl: process.env.NEXT_PUBLIC_API_URL!,
  interceptors: [
    (next) => async (request) => {
      const token = await resolveToken();
      if (token) request.header.set("Authorization", `Bearer ${token}`);
      try {
        return await next(request);
      } catch (error) {
        // The token may be stale or rotated — drop the cache so the next
        // call re-fetches instead of retrying the same bad token.
        cachedToken = undefined;
        throw error;
      }
    },
  ],
});
