import { createConnectTransport } from "@connectrpc/connect-web";
import { authClient } from "./auth-client";
import { impersonationToken } from "./impersonation";

// authClient.getSession() is a real network round trip to
// /api/auth/get-session. Attaching it as a per-RPC interceptor meant every
// single Connect call the app makes — and a single page load easily fires
// several in parallel (season, operator, groups, pilgrims, ...) — paid that
// latency on its own, which is why navigation felt heavy. Cache the resolved
// token in memory for a few seconds so a burst of RPC calls shares one
// lookup instead of each doing its own. This is the only layer of caching
// here — lib/auth.ts deliberately has no server-side session.cookieCache,
// so every cache miss below is a real, current DB check, not a stale one.
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

/** For the rare non-Connect call (e.g. the multipart document upload endpoint, which Connect's unary protocol can't express) that still needs the same Bearer token every RPC carries. Shares the same cache as the interceptor above instead of re-parsing cookies. */
export async function getBearerToken(): Promise<string | undefined> {
  return resolveToken();
}

export const transport = createConnectTransport({
  baseUrl: process.env.NEXT_PUBLIC_API_URL!,
  interceptors: [
    (next) => async (request) => {
      const token = await resolveToken();
      if (token) request.header.set("Authorization", `Bearer ${token}`);
      // Sent alongside the admin's own session, never instead of it: the
      // server needs both to know who is looking and whose screen they are
      // looking at. The server refuses this header on anything that writes, so
      // attaching it to every tenant-facing call is safe — the client never
      // decides what counts as a read.
      //
      // The platform's own surface is the one exception, and not because it is
      // a read: talking to the platform panel is the admin acting as
      // themselves, not as the customer. It is also what makes the session
      // closable — the button that ends an impersonation is a PlatformService
      // call, and a header the server refuses there would trap the admin
      // inside the session they are trying to leave.
      const impersonation = impersonationToken();
      if (impersonation && request.service.typeName !== "hajj.v1.PlatformService") {
        request.header.set("X-Impersonation-Token", impersonation);
      }
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
