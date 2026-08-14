import { createConnectTransport } from "@connectrpc/connect-web";
import { authClient } from "./auth-client";

export const transport = createConnectTransport({
  baseUrl: process.env.NEXT_PUBLIC_API_URL!,
  interceptors: [
    (next) => async (request) => {
      try {
        const session = await authClient.getSession({
          fetchOptions: { cache: "no-store" },
        });
        const token = session.data?.session?.token;
        if (!token) return next(request);

        // A rotated session can temporarily lose its active organization.
        // The API falls back to membership when this happens.
        const activeOrg = (session.data as any)?.session?.activeOrganizationId;
        if (!activeOrg) {
          // The server-side fallback remains authoritative.
        }
        request.header.set("Authorization", `Bearer ${token}`);
      } catch {
        // Let the server return unauthenticated so auth-client can redirect.
      }
      return next(request);
    },
  ],
});
