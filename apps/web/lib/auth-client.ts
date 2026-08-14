import { createAuthClient } from "better-auth/react";
import { organizationClient } from "better-auth/client/plugins";

export const authClient = createAuthClient({
  baseURL: process.env.NEXT_PUBLIC_APP_URL!,
  plugins: [organizationClient()],
  fetchOptions: {
    cache: "no-store",
    onError: async (ctx) => {
      if (ctx.response.status === 401) {
        window.location.href = "/sign-in";
      }
    },
  },
});
